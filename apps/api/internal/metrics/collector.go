package metrics

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Collector reads host metrics. On Linux it reads procRoot (default /proc).
// hostRoot is the prefix for mount points when the host root is bind-mounted
// somewhere else, for example /host/root inside a container. Both values come
// from operator configuration, never from a request.
type Collector struct {
	proc string
	host string
	now  func() time.Time

	mu      sync.Mutex
	prevCPU *cpuSample
	prevNet map[string]netCounters
	prevAt  time.Time
}

// NewCollector returns a collector. Empty paths use /proc and /.
func NewCollector(procRoot, hostRoot string) *Collector {
	procRoot = strings.TrimSpace(procRoot)
	hostRoot = strings.TrimSpace(hostRoot)
	if procRoot == "" {
		procRoot = "/proc"
	}
	if hostRoot == "" {
		hostRoot = "/"
	}
	return &Collector{
		proc:    filepath.Clean(procRoot),
		host:    filepath.Clean(hostRoot),
		now:     time.Now,
		prevNet: map[string]netCounters{},
	}
}

// Snapshot reads the host once and fills rates from the previous call.
func (c *Collector) Snapshot() (Snapshot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	snap, err := c.collect()
	if err != nil {
		return Snapshot{}, err
	}
	if snap.Disks == nil {
		snap.Disks = []DiskStats{}
	}
	if snap.Network == nil {
		snap.Network = []NetStats{}
	}
	return snap, nil
}

func (c *Collector) read(name string) (string, error) {
	b, err := os.ReadFile(filepath.Join(c.proc, name))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func underRoot(root, mount string) string {
	if root == "" || root == string(os.PathSeparator) {
		return mount
	}
	rel := strings.TrimPrefix(mount, string(os.PathSeparator))
	if rel == "" {
		return root
	}
	return filepath.Join(root, rel)
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
