package metrics

import "time"

const (
	// SourceProc means the snapshot was read from a Linux proc filesystem.
	SourceProc = "proc"
	// SourceFallback means this build cannot read Linux /proc.
	SourceFallback = "fallback"
)

// Snapshot is a true-state summary of the host the process can see.
// CPU usage and network rates stay null until a second sample exists.
// Memory used is MemTotal minus MemAvailable, so page cache is not reported as full.
type Snapshot struct {
	CollectedAt   time.Time   `json:"collected_at"`
	Hostname      string      `json:"hostname"`
	Source        string      `json:"source"`
	UptimeSeconds float64     `json:"uptime_seconds"`
	CPU           CPUStats    `json:"cpu"`
	Memory        MemoryStats `json:"memory"`
	Load          LoadStats   `json:"load"`
	Disks         []DiskStats `json:"disks"`
	Network       []NetStats  `json:"network"`
	Note          string      `json:"note,omitempty"`
}

// CPUStats is the aggregate CPU. UsagePercent is nil on the first sample.
type CPUStats struct {
	UsagePercent *float64 `json:"usage_percent"`
	Cores        int      `json:"cores"`
}

// MemoryStats separates cache from memory that is actually unavailable.
type MemoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	CacheBytes     uint64  `json:"cache_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// LoadStats is the 1, 5, and 15 minute load average.
type LoadStats struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

// DiskStats describes one real filesystem mount.
type DiskStats struct {
	Mount          string  `json:"mount"`
	Filesystem     string  `json:"filesystem"`
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

// NetStats is one interface. Rates are nil until a second sample exists.
type NetStats struct {
	Name      string   `json:"name"`
	RxBytes   uint64   `json:"rx_bytes"`
	TxBytes   uint64   `json:"tx_bytes"`
	RxPackets uint64   `json:"rx_packets"`
	TxPackets uint64   `json:"tx_packets"`
	RxErrors  uint64   `json:"rx_errors"`
	TxErrors  uint64   `json:"tx_errors"`
	RxBps     *float64 `json:"rx_bps"`
	TxBps     *float64 `json:"tx_bps"`
}
