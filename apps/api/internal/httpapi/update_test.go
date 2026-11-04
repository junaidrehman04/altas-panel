package httpapi_test

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/config"
	"github.com/junaidrehman04/altas/apps/api/internal/db"
	"github.com/junaidrehman04/altas/apps/api/internal/httpapi"
)

func TestUpdateSettingsAreAdminOnly(t *testing.T) {
	handler, sqlDB := newUpdateApp(t, nil)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	anon := &http.Client{}
	res, data := getJSON(t, anon, srv.URL+"/api/settings")
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anon status %d body %s", res.StatusCode, data)
	}

	client := signedInClient(t, srv.URL)
	res, data = getJSON(t, client, srv.URL+"/api/settings")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("admin status %d body %s", res.StatusCode, data)
	}
	var body updateSettingsBody
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if body.AutoUpdate || body.CurrentVersion == "" {
		t.Fatalf("settings = %+v", body)
	}

	res, data = patchJSON(t, client, srv.URL+"/api/settings", map[string]any{"auto_update": true})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("patch status %d body %s", res.StatusCode, data)
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if !body.AutoUpdate {
		t.Fatalf("toggle = %+v", body)
	}

	if _, err := sqlDB.Exec(`UPDATE users SET is_admin = 0`); err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		method string
		path   string
		json   any
	}{
		{http.MethodGet, "/api/settings", nil},
		{http.MethodPatch, "/api/settings", map[string]any{"auto_update": false}},
		{http.MethodPost, "/api/settings/updates/check", map[string]any{}},
		{http.MethodPost, "/api/settings/updates/apply", map[string]any{}},
		{http.MethodPost, "/api/settings/updates/rollback", map[string]any{}},
	} {
		res, data = sendJSON(t, client, call.method, srv.URL+call.path, call.json)
		if res.StatusCode != http.StatusForbidden {
			t.Fatalf("%s %s status %d body %s", call.method, call.path, res.StatusCode, data)
		}
	}
}

func TestUpdateCheckApplyAndRollback(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "altas-panel")
	original := append([]byte{0x7f, 'E', 'L', 'F'}, []byte("installed")...)
	if err := os.WriteFile(binPath, original, 0o755); err != nil {
		t.Fatal(err)
	}
	next := append([]byte{0x7f, 'E', 'L', 'F'}, []byte("release")...)
	sum := sha256.Sum256(next)
	sums := hex.EncodeToString(sum[:]) + "  altas-panel-linux-amd64\n"
	release := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/junaidrehman04/altas-panel/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v9.9.9",
				"assets": []map[string]any{
					{"id": 10, "name": "altas-panel-linux-amd64", "size": len(next), "browser_download_url": "https://evil.example/bin"},
					{"id": 11, "name": "SHA256SUMS", "size": len(sums), "browser_download_url": "https://evil.example/sums"},
				},
			})
		case "/repos/junaidrehman04/altas-panel/releases/assets/10":
			if r.Header.Get("Accept") != "application/octet-stream" {
				http.Error(w, "accept", http.StatusNotAcceptable)
				return
			}
			_, _ = w.Write(next)
		case "/repos/junaidrehman04/altas-panel/releases/assets/11":
			if r.Header.Get("Accept") != "application/octet-stream" {
				http.Error(w, "accept", http.StatusNotAcceptable)
				return
			}
			_, _ = w.Write([]byte(sums))
		default:
			if strings.Contains(r.URL.Host, "evil.example") {
				t.Errorf("downloaded %s", r.URL)
			}
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(release.Close)

	handler, _ := newUpdateApp(t, func(cfg *config.Config) {
		cfg.UpdateAPIBase = release.URL
		cfg.UpdateBinary = binPath
		cfg.UpdateRestart = "none"
		cfg.UpdateOS = "linux"
		cfg.UpdateArch = "amd64"
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := signedInClient(t, srv.URL)

	res, data := postJSON(t, client, srv.URL+"/api/settings/updates/check", map[string]any{})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("check status %d body %s", res.StatusCode, data)
	}
	var body updateSettingsBody
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if body.AutoUpdate || body.Applied || body.LatestAvailable != "v9.9.9" || !body.UpdateAvailable {
		t.Fatalf("check = %+v", body)
	}
	got, err := os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("check replaced the binary while auto-update was off")
	}

	res, data = postJSON(t, client, srv.URL+"/api/settings/updates/apply", map[string]any{})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("apply status %d body %s", res.StatusCode, data)
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatal(err)
	}
	if !body.Applied || !body.RollbackAvailable {
		t.Fatalf("apply = %+v", body)
	}
	got, err = os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, next) {
		t.Fatal("apply did not install the release asset")
	}

	res, data = postJSON(t, client, srv.URL+"/api/settings/updates/rollback", map[string]any{})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("rollback status %d body %s", res.StatusCode, data)
	}
	got, err = os.ReadFile(binPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("rollback = %q", got)
	}
}

func TestVersionEndpoint(t *testing.T) {
	handler, _ := newUpdateApp(t, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "version") {
		t.Fatalf("version status %d body %s", rr.Code, rr.Body.String())
	}
}

type updateSettingsBody struct {
	AutoUpdate        bool   `json:"auto_update"`
	CurrentVersion    string `json:"current_version"`
	LatestAvailable   string `json:"latest_available"`
	RollbackAvailable bool   `json:"rollback_available"`
	Applied           bool   `json:"applied"`
	UpdateAvailable   bool   `json:"update_available"`
}

func patchJSON(t *testing.T, client *http.Client, url string, body any) (*http.Response, []byte) {
	t.Helper()
	return sendJSON(t, client, http.MethodPatch, url, body)
}

func sendJSON(t *testing.T, client *http.Client, method, url string, body any) (*http.Response, []byte) {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, data
}

func newUpdateApp(t *testing.T, mutate func(*config.Config)) (http.Handler, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	sqlDB, err := db.Open(filepath.Join(dir, "altas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	cfg := config.Config{
		Env:          "development",
		WebOrigin:    "http://127.0.0.1:5173",
		SessionTTL:   24 * time.Hour,
		ResetTTL:     time.Hour,
		ArgonMemory:  8 * 1024,
		ArgonTime:    1,
		ArgonThreads: 1,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return httpapi.New(cfg, sqlDB, log.New(&bytes.Buffer{}, "", 0)), sqlDB
}
