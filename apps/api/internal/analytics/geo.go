package analytics

import (
	"io"
	"log"
	"net"
	"path/filepath"
	"strings"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// Locator resolves a truncated address to a country and city.
// Enabled is false when no database is configured. Callers must leave geo fields empty then.
type Locator interface {
	Lookup(truncatedIP string) (country, city string)
	Enabled() bool
}

// FakeLocator is a test double. It does not read a database.
type FakeLocator struct {
	Country string
	City    string
}

func (f FakeLocator) Lookup(string) (string, string) { return f.Country, f.City }
func (f FakeLocator) Enabled() bool                  { return true }

type disabledLocator struct{}

func (disabledLocator) Lookup(string) (string, string) { return "", "" }
func (disabledLocator) Enabled() bool                  { return false }

type mmdbLocator struct {
	db *maxminddb.Reader
}

type cityRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
}

// OpenLocator opens a MaxMind City or Country database.
// An empty path, a relative path, or a missing file returns a disabled locator.
// It never invents a place.
func OpenLocator(path string, logger *log.Logger) Locator {
	path = strings.TrimSpace(path)
	if path == "" {
		return disabledLocator{}
	}
	if strings.Contains(path, "..") || !filepath.IsAbs(path) {
		if logger != nil {
			logger.Printf("geoip: database path must be absolute")
		}
		return disabledLocator{}
	}
	reader, err := maxminddb.Open(filepath.Clean(path))
	if err != nil {
		if logger != nil {
			logger.Printf("geoip: database unavailable: %v", err)
		}
		return disabledLocator{}
	}
	return &mmdbLocator{db: reader}
}

func (m *mmdbLocator) Close() error {
	if m == nil || m.db == nil {
		return nil
	}
	err := m.db.Close()
	m.db = nil
	return err
}

func (m *mmdbLocator) Enabled() bool { return m != nil && m.db != nil }

func (m *mmdbLocator) Lookup(truncated string) (string, string) {
	if m == nil || m.db == nil {
		return "", ""
	}
	ip := net.ParseIP(truncated)
	if ip == nil {
		return "", ""
	}
	var rec cityRecord
	if err := m.db.Lookup(ip, &rec); err != nil {
		return "", ""
	}
	return rec.Country.ISOCode, rec.City.Names["en"]
}

// Reloader swaps the open database when a new file is ready.
// Lookups keep using the previous reader until the new file opens.
type Reloader struct {
	mu     sync.RWMutex
	inner  Locator
	logger *log.Logger
}

// NewReloader opens path when it is present. A missing file stays disabled.
func NewReloader(path string, logger *log.Logger) *Reloader {
	return &Reloader{inner: OpenLocator(path, logger), logger: logger}
}

func (r *Reloader) Lookup(truncated string) (string, string) {
	if r == nil {
		return "", ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.inner == nil {
		return "", ""
	}
	return r.inner.Lookup(truncated)
}

func (r *Reloader) Enabled() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.inner != nil && r.inner.Enabled()
}

// Reload opens path and replaces the current reader.
// A path that does not open leaves the previous reader in place.
func (r *Reloader) Reload(path string) error {
	if r == nil {
		return errDatabaseClosed
	}
	next := OpenLocator(path, r.logger)
	if !next.Enabled() {
		return errDatabaseClosed
	}
	r.mu.Lock()
	prev := r.inner
	r.inner = next
	r.mu.Unlock()
	closeLocator(prev)
	return nil
}

var errDatabaseClosed = errString("geoip: database did not open")

type errString string

func (e errString) Error() string { return string(e) }

func closeLocator(loc Locator) {
	c, ok := loc.(io.Closer)
	if !ok {
		return
	}
	_ = c.Close()
}
