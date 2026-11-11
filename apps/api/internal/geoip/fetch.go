// Package geoip downloads the GeoLite2-Country database.
// The license key is read from the process environment by config and passed in.
// This package never reads the key from a request and never logs it.
package geoip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

const (
	edition          = "GeoLite2-Country"
	defaultBase      = "https://download.maxmind.com"
	refreshEvery     = 7 * 24 * time.Hour
	maxArchive       = 32 << 20
	maxDatabase      = 64 << 20
	maxChecksumBytes = 4096
)

// Options configures the downloader.
// Key is the MaxMind license key. An empty key disables downloads.
// BaseURL and HTTP are for tests. Production leaves both empty.
type Options struct {
	Path     string
	Key      string
	BaseURL  string
	Interval time.Duration
	Logger   *log.Logger
	HTTP     *http.Client
	Reload   func(path string) error
}

// Run downloads a missing database, then refreshes it once a week until ctx ends.
// A failed refresh keeps the file already on disk.
func Run(ctx context.Context, opt Options) {
	m := newManager(opt)
	m.run(ctx)
}

type manager struct {
	path     string
	key      string
	base     string
	interval time.Duration
	logger   *log.Logger
	http     *http.Client
	reload   func(path string) error
}

func newManager(opt Options) *manager {
	interval := opt.Interval
	if interval <= 0 {
		interval = refreshEvery
	}
	return &manager{
		path:     strings.TrimSpace(opt.Path),
		key:      strings.TrimSpace(opt.Key),
		base:     strings.TrimSpace(opt.BaseURL),
		interval: interval,
		logger:   opt.Logger,
		http:     opt.HTTP,
		reload:   opt.Reload,
	}
}

func (m *manager) run(ctx context.Context) {
	if m.key == "" {
		return
	}
	if _, err := m.sync(ctx, false); err != nil && ctx.Err() == nil {
		m.logf("geoip: download failed: %s", err.Error())
	}
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := m.sync(ctx, true); err != nil && ctx.Err() == nil {
				m.logf("geoip: refresh failed, keeping the previous database: %s", err.Error())
			}
		}
	}
}

// sync downloads GeoLite2-Country. refresh forces a download even when the file exists.
// The returned error is already redacted.
func (m *manager) sync(ctx context.Context, refresh bool) (bool, error) {
	if m.key == "" {
		return false, nil
	}
	if err := validDest(m.path); err != nil {
		return false, err
	}
	if !refresh {
		if _, err := os.Stat(m.path); err == nil {
			return false, nil
		} else if !os.IsNotExist(err) {
			return false, redactErr(fmt.Errorf("database path: %w", err), m.key)
		}
	}
	archive, err := m.get(ctx, "tar.gz", maxArchive)
	if err != nil {
		return false, redactErr(err, m.key)
	}
	sum, err := m.get(ctx, "tar.gz.sha256", maxChecksumBytes)
	if errors.Is(err, errNotFound) {
		m.logf("geoip: checksum file unavailable, checking the database type")
	} else if err != nil {
		return false, redactErr(err, m.key)
	} else if err := verifySHA256(archive, sum); err != nil {
		return false, err
	}
	db, err := extractMMDB(archive)
	if err != nil {
		return false, err
	}
	if err := validateCountry(db); err != nil {
		return false, err
	}
	if err := writeAtomic(m.path, db); err != nil {
		return false, redactErr(err, m.key)
	}
	if m.reload != nil {
		if err := m.reload(m.path); err != nil {
			return false, redactErr(fmt.Errorf("reload: %w", err), m.key)
		}
	}
	if refresh {
		m.logf("geoip: country database refreshed")
	} else {
		m.logf("geoip: country database ready")
	}
	return true, nil
}

func validDest(path string) error {
	if path == "" || !filepath.IsAbs(path) || strings.Contains(path, "..") {
		return errors.New("database path must be absolute")
	}
	return nil
}

var errNotFound = errors.New("not found")

func (m *manager) get(ctx context.Context, suffix string, limit int64) ([]byte, error) {
	raw, err := m.endpoint(suffix)
	if err != nil {
		return nil, errors.New("download url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, errors.New("download request")
	}
	req.Header.Set("User-Agent", "Altas-Panel")
	resp, err := m.client().Do(req)
	if err != nil {
		return nil, errors.New("download request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, errors.New("download read failed")
	}
	if int64(len(body)) > limit {
		return nil, errors.New("download too large")
	}
	return body, nil
}

func (m *manager) client() *http.Client {
	if m.http != nil {
		return m.http
	}
	return defaultClient
}

// defaultClient does not use HTTP_PROXY. The license key is in the query string,
// and a proxy would record the full URL.
var defaultClient = newDefaultClient()

func newDefaultClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &http.Client{
		Timeout:   2 * time.Minute,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if req.URL == nil || (req.URL.Scheme != "https" && req.URL.Scheme != "http") {
				return errors.New("redirect scheme")
			}
			return nil
		},
	}
}

func (m *manager) endpoint(suffix string) (string, error) {
	base := m.base
	if base == "" {
		base = defaultBase
	}
	u, err := url.Parse(base)
	if err != nil || u.Host == "" {
		return "", errors.New("download url")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && m.base != "") {
		return "", errors.New("download url")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/app/geoip_download"
	q := u.Query()
	q.Set("edition_id", edition)
	q.Set("license_key", m.key)
	q.Set("suffix", suffix)
	u.RawQuery = q.Encode()
	u.Fragment = ""
	return u.String(), nil
}

func verifySHA256(archive, checksumFile []byte) error {
	text := strings.TrimSpace(string(checksumFile))
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return errors.New("checksum file is empty")
	}
	want := strings.ToLower(strings.TrimPrefix(fields[0], "sha256:"))
	if len(want) != sha256.Size*2 {
		return errors.New("checksum file is malformed")
	}
	if _, err := hex.DecodeString(want); err != nil {
		return errors.New("checksum file is malformed")
	}
	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	if subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return errors.New("checksum mismatch")
	}
	return nil
}

func extractMMDB(gz []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(gz))
	if err != nil {
		return nil, errors.New("archive is not gzip")
	}
	defer zr.Close()
	tr := tar.NewReader(&io.LimitedReader{R: zr, N: maxDatabase + 1})
	var picked []byte
	var pickedName string
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errors.New("archive is not a tar")
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if strings.Contains(hdr.Name, "..") {
			return nil, errors.New("archive path")
		}
		name := path.Base(path.Clean("/" + hdr.Name))
		if !strings.HasSuffix(strings.ToLower(name), ".mmdb") {
			continue
		}
		if hdr.Size < 0 || hdr.Size > maxDatabase {
			return nil, errors.New("database file is too large")
		}
		buf, err := io.ReadAll(io.LimitReader(tr, hdr.Size+1))
		if err != nil || int64(len(buf)) != hdr.Size {
			return nil, errors.New("database file is truncated")
		}
		if picked == nil || (strings.Contains(strings.ToLower(name), "country") && !strings.Contains(pickedName, "country")) {
			picked = buf
			pickedName = strings.ToLower(name)
		}
	}
	if picked == nil {
		return nil, errors.New("archive has no database")
	}
	return picked, nil
}

func validateCountry(data []byte) error {
	reader, err := maxminddb.FromBytes(data)
	if err != nil {
		return errors.New("database is not a valid mmdb")
	}
	defer reader.Close()
	kind := strings.ToLower(reader.Metadata.DatabaseType)
	if !strings.Contains(kind, "country") {
		return errors.New("database is not a country database")
	}
	return nil
}

func writeAtomic(dest string, data []byte) error {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("data directory: %w", err)
	}
	f, err := os.CreateTemp(dir, ".geoip-*")
	if err != nil {
		return fmt.Errorf("temporary file: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("temporary write: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("temporary sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("temporary close: %w", err)
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return fmt.Errorf("temporary mode: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		return fmt.Errorf("replace database: %w", err)
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

func (m *manager) logf(format string, args ...any) {
	if m.logger == nil {
		return
	}
	m.logger.Print(redact(fmt.Sprintf(format, args...), m.key))
}

func redactErr(err error, key string) error {
	if err == nil {
		return nil
	}
	return errors.New(redact(err.Error(), key))
}

func redact(msg, key string) string {
	if key != "" {
		msg = strings.ReplaceAll(msg, key, "redacted")
		escaped := url.QueryEscape(key)
		if escaped != key {
			msg = strings.ReplaceAll(msg, escaped, "redacted")
		}
	}
	return urlPattern.ReplaceAllString(msg, "redacted-url")
}

var urlPattern = regexp.MustCompile(`https?://\S+`)
