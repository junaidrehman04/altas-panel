package stacks

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectsFromContainersGroupsComposeLabels(t *testing.T) {
	t.Parallel()
	body := []byte(`[
		{"State":"running","Labels":{"com.docker.compose.project":"web","com.docker.compose.project.working_dir":"/var/lib/altas/projects/web"}},
		{"State":"exited","Labels":{"com.docker.compose.project":"web"}},
		{"State":"running","Labels":{"com.docker.compose.project":"api"}},
		{"State":"running","Labels":{}},
		{"State":"running","Labels":{"com.docker.compose.project":"bad\nname"}},
		{"State":"running","Labels":{"com.docker.compose.project":"mail","com.docker.compose.project.working_dir":"/tmp/mail\ninjected"}}
	]`)
	projects, err := ProjectsFromContainers(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 3 {
		t.Fatalf("projects = %+v", projects)
	}
	if projects[0].Name != "api" || projects[0].Status != "running" || projects[0].Containers != 1 {
		t.Fatalf("api = %+v", projects[0])
	}
	if projects[2].Name != "web" || projects[2].Status != "partial" || projects[2].Running != 1 || projects[2].Containers != 2 {
		t.Fatalf("web = %+v", projects[2])
	}
	if projects[2].Workdir != "/var/lib/altas/projects/web" {
		t.Fatalf("workdir = %q", projects[2].Workdir)
	}
	if projects[1].Name != "mail" || projects[1].Workdir != "" {
		t.Fatalf("mail = %+v", projects[1])
	}
}

func TestSocketPathRejectsTraversal(t *testing.T) {
	t.Parallel()
	for _, path := range []string{
		"",
		"relative.sock",
		"/var/run/../../tmp/docker.sock",
		"/tmp/docker.sock/../other.sock",
		"/tmp/docker.sock\n",
	} {
		if err := validateSocketPath(path); err == nil {
			t.Fatalf("validateSocketPath(%q) accepted", path)
		}
	}
	if err := validateSocketPath("/var/run/docker.sock"); err != nil {
		t.Fatal(err)
	}
}

func TestReportDisabledDoesNotTouchSocket(t *testing.T) {
	t.Parallel()
	report, err := NewProbe(false, "/var/run/docker.sock").Report(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Enabled || report.Available || len(report.Projects) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(report.Message, "ALTAS_DOCKER_PROBE=1") {
		t.Fatalf("message = %q", report.Message)
	}
}

func TestReportMissingSocketHidesPath(t *testing.T) {
	t.Parallel()
	socket := "/tmp/does-not-exist-altas.sock"
	report, err := NewProbe(true, socket).Report(context.Background())
	if err == nil {
		t.Fatal("expected a log error")
	}
	if report.Enabled != true || report.Available || len(report.Projects) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if strings.Contains(report.Message, socket) || !strings.Contains(report.Message, "Docker is not available") {
		t.Fatalf("message = %q", report.Message)
	}
}

func TestReportReadsUnixSocketReadOnly(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "docker.sock")
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/containers/json" || r.URL.RawQuery != "all=1" {
			t.Errorf("request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			http.Error(w, "no", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"State":"running","Labels":{"com.docker.compose.project":"web"}},
			{"State":"running","Labels":{"com.docker.compose.project":"web"}}
		]`))
	})}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	report, err := NewProbe(true, socket).Report(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Available || len(report.Projects) != 1 || report.Projects[0].Name != "web" || report.Projects[0].Status != "running" {
		t.Fatalf("report = %+v", report)
	}
	if strings.Contains(report.Message, socket) {
		t.Fatalf("message leaked socket path: %s", report.Message)
	}
}
