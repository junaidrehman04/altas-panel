package stacks

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

// RuntimeContainer is one container visible on the local socket.
// CPU and memory stay nil when the stats call does not return them.
type RuntimeContainer struct {
	Name        string   `json:"name"`
	Image       string   `json:"image"`
	State       string   `json:"state"`
	CPUPercent  *float64 `json:"cpu_percent"`
	MemoryBytes *uint64  `json:"memory_bytes"`
}

type listedContainer struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
}

// Containers lists local containers when a Docker socket is present.
// It does not create, start, stop, or delete anything. A missing socket returns an error
// for the log and an empty slice.
func (p Probe) Containers(ctx context.Context) ([]RuntimeContainer, error) {
	sock, err := p.findSocket()
	if err != nil {
		return []RuntimeContainer{}, err
	}
	body, err := fetchContainers(ctx, sock)
	if err != nil {
		return []RuntimeContainer{}, err
	}
	var listed []listedContainer
	if err := json.Unmarshal(body, &listed); err != nil {
		return []RuntimeContainer{}, err
	}
	out := make([]RuntimeContainer, 0, len(listed))
	statsLeft := 4
	for _, item := range listed {
		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		if !safeLabel(name) {
			name = "container"
		}
		image := item.Image
		if len(image) > 120 {
			image = image[:120]
		}
		row := RuntimeContainer{Name: name, Image: image, State: strings.ToLower(item.State)}
		if statsLeft > 0 && strings.EqualFold(item.State, "running") && len(item.ID) >= 12 {
			statsLeft--
			if cpu, mem, ok := containerStats(ctx, sock, item.ID); ok {
				row.CPUPercent = cpu
				row.MemoryBytes = mem
			}
		}
		out = append(out, row)
		if len(out) == 12 {
			break
		}
	}
	return out, nil
}

func containerStats(ctx context.Context, socketPath, id string) (*float64, *uint64, bool) {
	ctx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
	defer cancel()
	client := &http.Client{
		Timeout: 800 * time.Millisecond,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", socketPath)
			},
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/containers/"+id+"/stats?stream=false", nil)
	if err != nil {
		return nil, nil, false
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, false
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, nil, false
	}
	var body struct {
		CPU struct {
			CPUUsage struct {
				Total uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			System uint64 `json:"system_cpu_usage"`
		} `json:"cpu_stats"`
		Pre struct {
			CPUUsage struct {
				Total uint64 `json:"total_usage"`
			} `json:"cpu_usage"`
			System uint64 `json:"system_cpu_usage"`
		} `json:"precpu_stats"`
		Memory struct {
			Usage uint64 `json:"usage"`
			Stats struct {
				Cache        uint64 `json:"cache"`
				InactiveFile uint64 `json:"inactive_file"`
			} `json:"stats"`
		} `json:"memory_stats"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, nil, false
	}
	var cpu *float64
	if body.CPU.CPUUsage.Total > body.Pre.CPUUsage.Total && body.CPU.System > body.Pre.System {
		pct := float64(body.CPU.CPUUsage.Total-body.Pre.CPUUsage.Total) / float64(body.CPU.System-body.Pre.System) * 100
		if pct < 0 {
			pct = 0
		}
		if pct > 100 {
			pct = 100
		}
		cpu = &pct
	}
	mem := body.Memory.Usage
	cache := body.Memory.Stats.InactiveFile
	if cache == 0 {
		cache = body.Memory.Stats.Cache
	}
	if mem >= cache {
		mem -= cache
	}
	if mem == 0 && body.Memory.Usage == 0 {
		return cpu, nil, cpu != nil
	}
	return cpu, &mem, true
}
