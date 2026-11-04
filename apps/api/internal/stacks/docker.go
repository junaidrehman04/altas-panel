package stacks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	dockerDisabledMessage = "Docker probe is disabled. Set ALTAS_DOCKER_PROBE=1 to list local Compose projects."
	dockerMissingMessage  = "Docker is not available on this host. The probe needs a Docker socket (rootless or the docker group) and does not run shell commands."
	composeProjectLabel   = "com.docker.compose.project"
	composeWorkdirLabel   = "com.docker.compose.project.working_dir"
	maxDockerBody         = 8 << 20
)

// Project is one Compose project seen on the local Docker socket.
type Project struct {
	Name       string `json:"name"`
	Workdir    string `json:"workdir,omitempty"`
	Containers int    `json:"containers"`
	Running    int    `json:"running"`
	Status     string `json:"status"`
}

// ComposeReport is the read-only inventory returned to a signed-in operator.
// Socket paths are not included.
type ComposeReport struct {
	Enabled   bool      `json:"enabled"`
	Available bool      `json:"available"`
	Projects  []Project `json:"projects"`
	Message   string    `json:"message,omitempty"`
}

// Probe lists Compose projects with a single HTTP GET against the Docker socket.
// It has no method that creates, starts, stops, or deletes containers.
type Probe struct {
	enabled bool
	socket  string
}

// NewProbe builds a probe. socket is an optional operator-configured path.
// An empty socket searches the default dockerd and rootless locations.
func NewProbe(enabled bool, socket string) Probe {
	return Probe{enabled: enabled, socket: strings.TrimSpace(socket)}
}

// Report returns a client-safe inventory. The error is for the server log and
// may contain a socket path; callers must not write it to the response.
func (p Probe) Report(ctx context.Context) (ComposeReport, error) {
	empty := ComposeReport{Enabled: p.enabled, Projects: []Project{}}
	if !p.enabled {
		empty.Message = dockerDisabledMessage
		return empty, nil
	}
	sock, err := p.findSocket()
	if err != nil {
		empty.Message = dockerMissingMessage
		return empty, err
	}
	body, err := fetchContainers(ctx, sock)
	if err != nil {
		empty.Message = dockerMissingMessage
		return empty, err
	}
	projects, err := ProjectsFromContainers(body)
	if err != nil {
		empty.Message = dockerMissingMessage
		return empty, err
	}
	return ComposeReport{
		Enabled:   true,
		Available: true,
		Projects:  projects,
	}, nil
}

func (p Probe) findSocket() (string, error) {
	candidates := p.candidates()
	var last error = errors.New("docker socket not found")
	for _, candidate := range candidates {
		if err := validateSocketPath(candidate); err != nil {
			last = err
			continue
		}
		info, err := os.Lstat(candidate)
		if err != nil {
			last = err
			continue
		}
		if info.Mode()&os.ModeSocket == 0 {
			last = errors.New("docker path is not a socket")
			continue
		}
		return candidate, nil
	}
	return "", last
}

func (p Probe) candidates() []string {
	if p.socket != "" {
		return []string{p.socket}
	}
	return []string{
		"/var/run/docker.sock",
		"/run/docker.sock",
		fmt.Sprintf("/run/user/%d/docker.sock", os.Getuid()),
	}
}

func validateSocketPath(path string) error {
	if path == "" || strings.ContainsRune(path, 0) || strings.ContainsAny(path, "\r\n") {
		return errors.New("docker socket path rejected")
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return errors.New("docker socket path rejected")
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return errors.New("docker socket path rejected")
		}
	}
	return nil
}

func fetchContainers(ctx context.Context, socketPath string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/json?all=1", nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker api status %d", res.StatusCode)
	}
	return io.ReadAll(io.LimitReader(res.Body, maxDockerBody))
}

type dockerContainer struct {
	State  string            `json:"State"`
	Labels map[string]string `json:"Labels"`
}

// ProjectsFromContainers groups containers by the Compose project label.
// Containers without that label are ignored. Names with control characters
// are skipped. The function does not touch the Docker socket.
func ProjectsFromContainers(body []byte) ([]Project, error) {
	if len(bytesTrim(body)) == 0 || string(bytesTrim(body)) == "null" {
		return []Project{}, nil
	}
	var containers []dockerContainer
	if err := json.Unmarshal(body, &containers); err != nil {
		return nil, err
	}
	grouped := map[string]*Project{}
	order := []string{}
	for _, container := range containers {
		name := strings.TrimSpace(container.Labels[composeProjectLabel])
		if !safeLabel(name) {
			continue
		}
		item, ok := grouped[name]
		if !ok {
			item = &Project{Name: name}
			if workdir := container.Labels[composeWorkdirLabel]; safeWorkdir(workdir) {
				item.Workdir = workdir
			}
			grouped[name] = item
			order = append(order, name)
		} else if item.Workdir == "" {
			if workdir := container.Labels[composeWorkdirLabel]; safeWorkdir(workdir) {
				item.Workdir = workdir
			}
		}
		item.Containers++
		if strings.EqualFold(container.State, "running") {
			item.Running++
		}
	}
	projects := make([]Project, 0, len(order))
	for _, name := range order {
		item := grouped[name]
		switch {
		case item.Containers > 0 && item.Running == item.Containers:
			item.Status = "running"
		case item.Running == 0:
			item.Status = "exited"
		default:
			item.Status = "partial"
		}
		projects = append(projects, *item)
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	return projects, nil
}

func safeLabel(name string) bool {
	if name == "" || len(name) > 128 {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}

func safeWorkdir(path string) bool {
	if path == "" || len(path) > 4096 {
		return false
	}
	for _, r := range path {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
