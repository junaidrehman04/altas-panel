package httpapi_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/junaidrehman04/altas/apps/api/internal/config"
)

func TestHostMetricsAndStacksRequireSession(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	for _, path := range []string{"/api/metrics/host", "/api/stacks", "/api/stacks/compose"} {
		req := httptest.NewRequest(http.MethodGet, path+"?path=/etc/passwd", nil)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("%s status %d body %s", path, rr.Code, rr.Body.String())
		}
		if strings.Contains(rr.Body.String(), "/etc/passwd") {
			t.Fatalf("%s reflected query: %s", path, rr.Body.String())
		}
	}
}

func TestAuthenticatedHostMetricsAndStackCatalog(t *testing.T) {
	handler, _ := newTestApp(t, "development")
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := signedInClient(t, srv.URL)

	res, data := getJSON(t, client, srv.URL+"/api/metrics/host?path=/etc/passwd&cmd=$(id)")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("metrics status %d body %s", res.StatusCode, data)
	}
	if strings.Contains(string(data), "/etc/passwd") || strings.Contains(string(data), "$(id)") {
		t.Fatalf("metrics reflected query: %s", data)
	}
	var snap struct {
		Source string `json:"source"`
		CPU    struct {
			Cores int `json:"cores"`
		} `json:"cpu"`
		Memory struct {
			TotalBytes uint64 `json:"total_bytes"`
			CacheBytes uint64 `json:"cache_bytes"`
		} `json:"memory"`
		Disks   []any `json:"disks"`
		Network []any `json:"network"`
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		t.Fatalf("decode metrics: %v body %s", err, data)
	}
	if snap.CPU.Cores < 1 || snap.Disks == nil || snap.Network == nil {
		t.Fatalf("snapshot = %+v", snap)
	}
	if runtime.GOOS == "linux" {
		if snap.Source != "proc" || snap.Memory.TotalBytes == 0 {
			t.Fatalf("linux snapshot = %+v", snap)
		}
	}

	res, data = getJSON(t, client, srv.URL+"/api/stacks")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("stacks status %d body %s", res.StatusCode, data)
	}
	var catalog struct {
		ProjectsRoot string `json:"projects_root"`
		Stacks       []struct {
			Kind       string `json:"kind"`
			Deployable bool   `json:"deployable"`
		} `json:"stacks"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.ProjectsRoot != "/var/lib/altas/projects" || len(catalog.Stacks) != 3 {
		t.Fatalf("catalog = %+v", catalog)
	}
	kinds := map[string]bool{}
	for _, item := range catalog.Stacks {
		if item.Deployable {
			t.Fatalf("kind %s is deployable", item.Kind)
		}
		kinds[item.Kind] = true
	}
	for _, kind := range []string{"docker-compose", "laravel", "nextjs"} {
		if !kinds[kind] {
			t.Fatalf("missing kind %s in %+v", kind, catalog.Stacks)
		}
	}

	res, data = getJSON(t, client, srv.URL+"/api/stacks/compose")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("compose status %d body %s", res.StatusCode, data)
	}
	var report struct {
		Enabled   bool   `json:"enabled"`
		Available bool   `json:"available"`
		Message   string `json:"message"`
		Projects  []any  `json:"projects"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Enabled || report.Available || report.Projects == nil || !strings.Contains(report.Message, "ALTAS_DOCKER_PROBE=1") {
		t.Fatalf("compose report = %+v", report)
	}
}

func TestComposeProbeListsProjectsFromSocket(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	docker := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/containers/json" {
			http.Error(w, "no", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"State":"running","Labels":{"com.docker.compose.project":"web"}}]`))
	})}
	go func() { _ = docker.Serve(ln) }()
	t.Cleanup(func() { _ = docker.Close() })

	handler, logs := newConfiguredApp(t, func(cfg *config.Config) {
		cfg.DockerProbe = true
		cfg.DockerSocket = socket
	})
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client := signedInClient(t, srv.URL)

	res, data := getJSON(t, client, srv.URL+"/api/stacks/compose")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", res.StatusCode, data)
	}
	var report struct {
		Enabled   bool `json:"enabled"`
		Available bool `json:"available"`
		Projects  []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"projects"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if !report.Enabled || !report.Available || len(report.Projects) != 1 || report.Projects[0].Name != "web" || report.Projects[0].Status != "running" {
		t.Fatalf("report = %+v", report)
	}
	if strings.Contains(report.Message, socket) || strings.Contains(string(data), socket) {
		t.Fatalf("response leaked socket path: %s", data)
	}
	if strings.Contains(logs.String(), "docker probe:") {
		t.Fatalf("successful probe was logged as a failure: %s", logs.String())
	}
}

func signedInClient(t *testing.T, base string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	res, data := postJSON(t, client, base+"/api/auth/register", map[string]string{
		"email":    "metrics@altas.dev",
		"password": "correct-horse",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("register %d %s", res.StatusCode, data)
	}
	return client
}
