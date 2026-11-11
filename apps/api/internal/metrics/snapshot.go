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
// PerCore, IOWaitPercent, and StealPercent are also nil until a second sample exists.
type CPUStats struct {
	UsagePercent  *float64  `json:"usage_percent"`
	Cores         int       `json:"cores"`
	PerCore       []float64 `json:"per_core,omitempty"`
	IOWaitPercent *float64  `json:"iowait_percent,omitempty"`
	StealPercent  *float64  `json:"steal_percent,omitempty"`
}

// MemoryStats separates cache from memory that is actually unavailable.
type MemoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	CacheBytes     uint64  `json:"cache_bytes"`
	BuffersBytes   uint64  `json:"buffers_bytes,omitempty"`
	CachedBytes    uint64  `json:"cached_bytes,omitempty"`
	AnonBytes      uint64  `json:"anon_bytes,omitempty"`
	SwapTotalBytes uint64  `json:"swap_total_bytes,omitempty"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes,omitempty"`
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
	Mount            string   `json:"mount"`
	Filesystem       string   `json:"filesystem"`
	Device           string   `json:"device,omitempty"`
	TotalBytes       uint64   `json:"total_bytes"`
	UsedBytes        uint64   `json:"used_bytes"`
	AvailableBytes   uint64   `json:"available_bytes"`
	UsedPercent      float64  `json:"used_percent"`
	InodeUsedPercent *float64 `json:"inode_used_percent,omitempty"`
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
