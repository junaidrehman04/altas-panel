package geoip

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/analytics"
	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/maxminddb-golang"
)

const testKey = "Mm/Key+ZZZ-secret"

func TestDownloadInstallsACountryDatabase(t *testing.T) {
	db := buildDB(t, "GeoLite2-Country", "8.8.8.0/24", "US")
	archive := tarGz(t, "GeoLite2-Country_20241111/GeoLite2-Country.mmdb", db)
	var sawArchive, sawSum bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("license_key") != testKey {
			http.Error(w, "no", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("edition_id") != edition {
			http.Error(w, "edition", http.StatusBadRequest)
			return
		}
		switch r.URL.Query().Get("suffix") {
		case "tar.gz":
			sawArchive = true
			_, _ = w.Write(archive)
		case "tar.gz.sha256":
			sawSum = true
			sum := sha256.Sum256(archive)
			_, _ = fmt.Fprintf(w, "%s  GeoLite2-Country.tar.gz\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	dest := filepath.Join(dir, "GeoLite2-Country.mmdb")
	var logs bytes.Buffer
	reloader := analytics.NewReloader("", nil)
	m := newManager(Options{
		Path:    dest,
		Key:     testKey,
		BaseURL: srv.URL,
		Logger:  log.New(&logs, "", 0),
		HTTP:    srv.Client(),
		Reload:  reloader.Reload,
	})
	wrote, err := m.sync(context.Background(), false)
	if err != nil || !wrote {
		t.Fatalf("sync: wrote=%v err=%v logs=%s", wrote, err, logs.String())
	}
	if !sawArchive || !sawSum {
		t.Fatalf("archive=%v checksum=%v", sawArchive, sawSum)
	}
	if strings.Contains(logs.String(), testKey) || strings.Contains(logs.String(), "license_key") {
		t.Fatalf("log leaked the key: %s", logs.String())
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, db) {
		t.Fatal("written file is not the database from the archive")
	}
	reader, err := maxminddb.Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if !strings.Contains(strings.ToLower(reader.Metadata.DatabaseType), "country") {
		t.Fatalf("database type %q", reader.Metadata.DatabaseType)
	}
	if !reloader.Enabled() {
		t.Fatal("reader did not reload")
	}
	country, _ := reloader.Lookup("8.8.8.0")
	if country != "US" {
		t.Fatalf("country = %q", country)
	}
}

func TestMissingChecksumStillInstallsAValidCountryDatabase(t *testing.T) {
	db := buildDB(t, "GeoLite2-Country", "8.8.4.0/24", "BR")
	archive := tarGz(t, "GeoLite2-Country.mmdb", db)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("suffix") == "tar.gz.sha256" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archive)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	var logs bytes.Buffer
	m := newManager(Options{
		Path:    dest,
		Key:     testKey,
		BaseURL: srv.URL,
		Logger:  log.New(&logs, "", 0),
		HTTP:    srv.Client(),
	})
	wrote, err := m.sync(context.Background(), false)
	if err != nil || !wrote {
		t.Fatalf("sync: wrote=%v err=%v", wrote, err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || !bytes.Equal(got, db) {
		t.Fatal("database was not kept when the checksum file was absent")
	}
	if !strings.Contains(logs.String(), "checksum file unavailable") {
		t.Fatalf("logs = %s", logs.String())
	}
	assertNoKey(t, logs.String())
}

func TestChecksumOrValidationFailureDoesNotWrite(t *testing.T) {
	db := buildDB(t, "GeoLite2-Country", "1.2.3.0/24", "DE")
	good := tarGz(t, "GeoLite2-Country.mmdb", db)
	city := tarGz(t, "GeoLite2-City.mmdb", buildDB(t, "GeoLite2-City", "1.2.3.0/24", "DE"))
	junk := tarGz(t, "GeoLite2-Country.mmdb", []byte("not-a-database"))
	sum := sha256.Sum256(good)
	goodSum := hex.EncodeToString(sum[:])

	cases := []struct {
		name    string
		archive []byte
		sum     string
	}{
		{name: "checksum mismatch", archive: good, sum: strings.Repeat("ab", 32)},
		{name: "malformed checksum", archive: good, sum: "nope"},
		{name: "city database", archive: city, sum: hashOf(city)},
		{name: "invalid mmdb", archive: junk, sum: hashOf(junk)},
		{name: "not an archive", archive: []byte("hello"), sum: hashOf([]byte("hello"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Query().Get("suffix") {
				case "tar.gz":
					_, _ = w.Write(tc.archive)
				case "tar.gz.sha256":
					_, _ = io.WriteString(w, tc.sum+"\n")
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			dest := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
			m := newManager(Options{Path: dest, Key: testKey, BaseURL: srv.URL, HTTP: srv.Client()})
			if _, err := m.sync(context.Background(), false); err == nil {
				t.Fatal("invalid download was accepted")
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatal("invalid download wrote a database")
			}
		})
	}
	bare := hashOf(good)
	if err := verifySHA256(good, []byte(bare)); err != nil {
		t.Fatal(err)
	}
	if err := verifySHA256(good, []byte(goodSum+"  GeoLite2-Country.tar.gz")); err != nil {
		t.Fatal(err)
	}
}

func TestFailedRefreshKeepsThePreviousFile(t *testing.T) {
	db := buildDB(t, "GeoLite2-Country", "9.9.9.0/24", "JP")
	archive := tarGz(t, "GeoLite2-Country.mmdb", db)
	var fail atomic.Bool
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if fail.Load() {
			http.Error(w, "license_key="+testKey, http.StatusInternalServerError)
			return
		}
		switch r.URL.Query().Get("suffix") {
		case "tar.gz":
			_, _ = w.Write(archive)
		case "tar.gz.sha256":
			_, _ = io.WriteString(w, hashOf(archive)+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	var logs safeBuf
	var reloads atomic.Int32
	reloader := analytics.NewReloader("", nil)
	m := newManager(Options{
		Path:     dest,
		Key:      testKey,
		BaseURL:  srv.URL,
		Interval: 20 * time.Millisecond,
		Logger:   log.New(&logs, "", 0),
		HTTP:     srv.Client(),
		Reload: func(path string) error {
			reloads.Add(1)
			return reloader.Reload(path)
		},
	})
	if _, err := m.sync(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if reloads.Load() != 1 {
		t.Fatalf("reloads = %d", reloads.Load())
	}
	fail.Store(true)
	afterHits := hits.Load()
	if _, err := m.sync(context.Background(), true); err == nil {
		t.Fatal("failed refresh was treated as success")
	}
	if hits.Load() <= afterHits {
		t.Fatal("refresh did not contact the server")
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, got) {
		t.Fatal("failed refresh replaced the database")
	}
	if reloads.Load() != 1 {
		t.Fatalf("failed refresh reloaded: %d", reloads.Load())
	}
	country, _ := reloader.Lookup("9.9.9.0")
	if country != "JP" {
		t.Fatalf("country after failed refresh = %q", country)
	}
	assertNoKey(t, logs.String())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		m.run(ctx)
		close(done)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !strings.Contains(logs.String(), "keeping the previous database") {
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("refresh loop did not stop")
	}
	got, err = os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, got) {
		t.Fatal("background refresh replaced the database")
	}
	if !strings.Contains(logs.String(), "keeping the previous database") {
		t.Fatalf("refresh failure was not logged: %s", logs.String())
	}
	assertNoKey(t, logs.String())
}

func TestNoLicenseKeyDoesNotDownload(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	m := newManager(Options{Path: dest, Key: "", BaseURL: srv.URL, HTTP: srv.Client()})
	wrote, err := m.sync(context.Background(), true)
	if err != nil || wrote {
		t.Fatalf("no-key sync wrote=%v err=%v", wrote, err)
	}
	done := make(chan struct{})
	go func() {
		m.run(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("no-key path blocked")
	}
	if hits.Load() != 0 {
		t.Fatalf("no-key path contacted the server %d times", hits.Load())
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("no-key path created a database")
	}
	if analytics.NewReloader("", nil).Enabled() {
		t.Fatal("empty reader enabled geo")
	}
}

func TestLicenseKeyNeverAppearsInLogsOrErrors(t *testing.T) {
	var logs bytes.Buffer
	m := newManager(Options{
		Path:    filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb"),
		Key:     testKey,
		BaseURL: "http://127.0.0.1:9",
		Logger:  log.New(&logs, "", 0),
		HTTP: &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("Get %q: connection refused", req.URL.String())
		})},
	})
	_, err := m.sync(context.Background(), true)
	if err == nil {
		t.Fatal("expected a download error")
	}
	m.logf("geoip: refresh failed, keeping the previous database: %s", err.Error())
	assertNoKey(t, err.Error())
	assertNoKey(t, logs.String())

	msg := redact("Get \"http://download.maxmind.com/app/geoip_download?license_key="+url.QueryEscape(testKey)+"\": no", testKey)
	assertNoKey(t, msg)
	if strings.Contains(msg, "http://") || strings.Contains(msg, "https://") {
		t.Fatalf("url remained: %s", msg)
	}
}

func TestRelativePathIsRefused(t *testing.T) {
	m := newManager(Options{Path: "GeoLite2-Country.mmdb", Key: testKey})
	if _, err := m.sync(context.Background(), true); err == nil {
		t.Fatal("relative path was accepted")
	}
	m = newManager(Options{Path: "/tmp/../GeoLite2-Country.mmdb", Key: testKey})
	if _, err := m.sync(context.Background(), true); err == nil {
		t.Fatal("dot-dot path was accepted")
	}
}

func assertNoKey(t *testing.T, msg string) {
	t.Helper()
	if strings.Contains(msg, testKey) || strings.Contains(msg, url.QueryEscape(testKey)) || strings.Contains(msg, "license_key") {
		t.Fatalf("key leaked: %s", msg)
	}
}

type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func buildDB(t *testing.T, kind, cidr, iso string) []byte {
	t.Helper()
	w, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: kind, RecordSize: 24})
	if err != nil {
		t.Fatal(err)
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatal(err)
	}
	err = w.Insert(network, mmdbtype.Map{
		"country": mmdbtype.Map{
			"iso_code": mmdbtype.String(iso),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := w.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tarGz(t *testing.T, name string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	if err := tw.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o644,
		Size: int64(len(body)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRedactRemovesTheKeyBeforeAnythingIsLogged(t *testing.T) {
	raw := "license_key=" + testKey + " and " + url.QueryEscape(testKey)
	got := redact(raw, testKey)
	if strings.Contains(got, testKey) || strings.Contains(got, url.QueryEscape(testKey)) {
		t.Fatalf("redact left the key in %s", got)
	}
	if redactErr(errors.New(raw), testKey) == nil {
		t.Fatal("nil error")
	}
}
