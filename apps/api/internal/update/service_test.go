package update

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/db"
)

func TestAutoUpdateDefaultsOffAndPersists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "altas.db")
	sqlDB := openPath(t, dbPath)
	svc := New(sqlDB, Options{
		Version:    "v0.2.0-dev",
		Restart:    "none",
		BinaryPath: filepath.Join(t.TempDir(), "altas-panel"),
	})

	st, err := svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.AutoUpdate || st.CurrentVersion != "v0.2.0-dev" {
		t.Fatalf("status = %+v", st)
	}

	if err := svc.SetAutoUpdate(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openPath(t, dbPath)
	again := New(reopened, Options{Version: "v0.2.0-dev"})
	st, err = again.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.AutoUpdate {
		t.Fatal("auto-update did not persist")
	}
	if err := again.SetAutoUpdate(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	st, err = again.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.AutoUpdate {
		t.Fatal("toggle did not turn off")
	}
}

func TestApplyRefusesBadDigestAndDoesNotReplace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	current := fakeELF("current")
	if err := os.WriteFile(path, current, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := fakeELF("tampered")
	sum := sha256.Sum256(fakeELF("honest"))
	srv := releaseServer(t, "v0.3.0", payload, hex.EncodeToString(sum[:]), nil)

	svc := New(openPath(t, filepath.Join(dir, "altas.db")), Options{
		Version:    "v0.2.0-dev",
		BaseURL:    srv.URL,
		BinaryPath: path,
		Restart:    "none",
		OS:         "linux",
		Arch:       "amd64",
	})
	_, err := svc.Apply(context.Background())
	if !errors.Is(err, ErrChecksumMismatch) {
		t.Fatalf("err = %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, current) {
		t.Fatal("binary changed after a bad digest")
	}
	if HasBackup(path) {
		t.Fatal("backup written after a bad digest")
	}
}

func TestCheckNowAppliesOnlyWhenEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	if err := os.WriteFile(path, fakeELF("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	next := fakeELF("new")
	sum := sha256.Sum256(next)
	srv := releaseServer(t, "v0.3.0", next, hex.EncodeToString(sum[:]), nil)
	restarts := 0
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	svc := New(openPath(t, filepath.Join(dir, "altas.db")), Options{
		Version:    "v0.2.0-dev",
		BaseURL:    srv.URL,
		BinaryPath: path,
		Restart:    "none",
		OS:         "linux",
		Arch:       "amd64",
		ManualMin:  time.Hour,
		Now:        func() time.Time { return now },
	})
	svc.Restarter = restarterFunc(func(context.Context) error {
		restarts++
		return nil
	})

	off, err := svc.Check(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if off.Applied || !off.UpdateAvailable || off.Latest != "v0.3.0" {
		t.Fatalf("check while off = %+v", off)
	}
	if restarts != 0 {
		t.Fatal("restarted while auto-update was off")
	}

	now = now.Add(2 * time.Hour)
	if err := svc.SetAutoUpdate(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	on, err := svc.Check(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if !on.Applied || restarts != 1 {
		t.Fatalf("applied=%v restarts=%d", on.Applied, restarts)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, next) {
		t.Fatal("enabled check did not install the release")
	}

	now = now.Add(30 * time.Second)
	if _, err := svc.Check(context.Background(), true); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limit err = %v", err)
	}
}

func TestScheduledCheckSkipsUntilInterval(t *testing.T) {
	calls := 0
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	bin := fakeELF("same")
	sum := sha256.Sum256(bin)
	svc := New(openPath(t, filepath.Join(t.TempDir(), "altas.db")), Options{
		Version:  "v0.2.0-dev",
		Interval: 6 * time.Hour,
		Now:      func() time.Time { return now },
	})
	if err := svc.SetAutoUpdate(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	svc.Source = sourceFunc(func(context.Context) (Release, error) {
		calls++
		return Release{
			Tag:       "v0.2.0-dev",
			Name:      "altas-panel-linux-amd64",
			Binary:    bin,
			Checksums: []byte(hex.EncodeToString(sum[:]) + "  altas-panel-linux-amd64\n"),
		}, nil
	})
	if _, err := svc.Check(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	res, err := svc.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Skipped || calls != 1 {
		t.Fatalf("skipped=%v calls=%d result=%+v", res.Skipped, calls, res)
	}
}

func TestSignatureRequiredWhenKeyConfigured(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	if err := os.WriteFile(path, fakeELF("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	next := fakeELF("signed-release")
	sum := sha256.Sum256(next)
	sig := hex.EncodeToString(ed25519.Sign(priv, next))
	srv := releaseServer(t, "v0.4.0", next, hex.EncodeToString(sum[:]), []byte(sig))
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	svc := New(openPath(t, filepath.Join(dir, "altas.db")), Options{
		Version:    "v0.2.0-dev",
		BaseURL:    srv.URL,
		BinaryPath: path,
		Restart:    "none",
		OS:         "linux",
		Arch:       "amd64",
		PublicKey:  hex.EncodeToString(pub),
		ManualMin:  time.Minute,
		Now:        func() time.Time { return now },
	})
	if _, err := svc.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute)
	unsigned := releaseServer(t, "v0.5.0", next, hex.EncodeToString(sum[:]), nil)
	svc.Source = &Client{
		BaseURL: unsigned.URL,
		Owner:   "junaidrehman04",
		Repo:    "altas-panel",
		OS:      "linux",
		Arch:    "amd64",
	}
	if _, err := svc.Apply(context.Background()); !errors.Is(err, ErrSignature) {
		t.Fatalf("missing signature err = %v", err)
	}
}

func TestPreviewCheckDoesNotBlockScheduledInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	if err := os.WriteFile(path, fakeELF("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	next := fakeELF("scheduled")
	sum := sha256.Sum256(next)
	srv := releaseServer(t, "v0.3.0", next, hex.EncodeToString(sum[:]), nil)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	restarts := 0
	svc := New(openPath(t, filepath.Join(dir, "altas.db")), Options{
		Version:    "v0.2.1",
		BaseURL:    srv.URL,
		BinaryPath: path,
		Restart:    "none",
		OS:         "linux",
		Arch:       "amd64",
		Interval:   6 * time.Hour,
		ManualMin:  time.Minute,
		Now:        func() time.Time { return now },
	})
	svc.Restarter = restarterFunc(func(context.Context) error {
		restarts++
		return nil
	})

	preview, err := svc.Check(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Applied || !preview.UpdateAvailable {
		t.Fatalf("preview = %+v", preview)
	}
	if err := svc.SetAutoUpdate(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	scheduled, err := svc.Check(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !scheduled.Applied || restarts != 1 {
		t.Fatalf("scheduled applied=%v restarts=%d result=%+v", scheduled.Applied, restarts, scheduled)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, next) {
		t.Fatal("scheduled check did not install the release")
	}
}

func TestApplyRollsBackWhenRestartFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "altas-panel")
	current := fakeELF("current")
	if err := os.WriteFile(path, current, 0o755); err != nil {
		t.Fatal(err)
	}
	next := fakeELF("next")
	sum := sha256.Sum256(next)
	srv := releaseServer(t, "v0.3.0", next, hex.EncodeToString(sum[:]), nil)
	svc := New(openPath(t, filepath.Join(dir, "altas.db")), Options{
		Version:    "v0.2.1",
		BaseURL:    srv.URL,
		BinaryPath: path,
		Restart:    "none",
		OS:         "linux",
		Arch:       "amd64",
		ManualMin:  time.Minute,
	})
	svc.Restarter = restarterFunc(func(context.Context) error {
		return ErrRestart
	})
	if _, err := svc.Apply(context.Background()); !errors.Is(err, ErrRestart) {
		t.Fatalf("err = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, current) {
		t.Fatal("failed restart left the new binary in place")
	}
	if !HasBackup(path) {
		t.Fatal("backup missing after a failed restart")
	}
}

func openPath(t *testing.T, path string) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func releaseServer(t *testing.T, tag string, bin []byte, digest string, sig []byte) *httptest.Server {
	t.Helper()
	sums := digest + "  altas-panel-linux-amd64\n"
	assets := []map[string]any{
		{"id": 10, "name": "altas-panel-linux-amd64", "size": len(bin), "browser_download_url": "https://evil.example/bin"},
		{"id": 11, "name": "SHA256SUMS", "size": len(sums), "browser_download_url": "https://evil.example/sums"},
	}
	files := map[string][]byte{
		"10": bin,
		"11": []byte(sums),
	}
	if sig != nil {
		assets = append(assets, map[string]any{
			"id": 12, "name": "altas-panel-linux-amd64.ed25519", "size": len(sig),
			"browser_download_url": "https://evil.example/sig",
		})
		files["12"] = sig
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.Host, "evil.example") {
			t.Errorf("request host %s", r.Host)
		}
		switch {
		case r.URL.Path == "/repos/junaidrehman04/altas-panel/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": tag, "assets": assets})
		case strings.HasPrefix(r.URL.Path, "/repos/junaidrehman04/altas-panel/releases/assets/"):
			if r.Header.Get("Accept") != "application/octet-stream" {
				http.Error(w, "accept", http.StatusNotAcceptable)
				return
			}
			id := strings.TrimPrefix(r.URL.Path, "/repos/junaidrehman04/altas-panel/releases/assets/")
			body, ok := files[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

type restarterFunc func(context.Context) error

func (f restarterFunc) Restart(ctx context.Context) error { return f(ctx) }

type sourceFunc func(context.Context) (Release, error)

func (f sourceFunc) FetchLatest(ctx context.Context) (Release, error) { return f(ctx) }
