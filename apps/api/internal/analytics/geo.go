package analytics

import (
	"log"
	"net"
	"path/filepath"
	"strings"

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
