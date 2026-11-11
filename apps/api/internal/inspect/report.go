package inspect

import (
	"sync"
	"time"
)

// Report is a point-in-time look at the machine. Empty sections mean the
// source was not readable. Callers must not invent values for those sections.
type Report struct {
	CollectedAt   time.Time  `json:"collected_at"`
	Hostname      string     `json:"hostname"`
	FQDN          string     `json:"fqdn"`
	ID            string     `json:"id"`
	Kind          string     `json:"kind"`
	Virt          string     `json:"virt"`
	Hypervisor    bool       `json:"hypervisor"`
	Provider      string     `json:"provider"`
	IP            string     `json:"ip"`
	OS            string     `json:"os"`
	Kernel        string     `json:"kernel"`
	CPUModel      string     `json:"cpu_model"`
	Cores         int        `json:"cores"`
	Threads       int        `json:"threads"`
	UptimeSeconds float64    `json:"uptime_seconds"`
	Load1         float64    `json:"load1"`
	Load5         float64    `json:"load5"`
	Load15        float64    `json:"load15"`
	RebootPending bool       `json:"reboot_pending"`
	AgentCPU      *float64   `json:"agent_cpu_percent"`
	AgentRSS      uint64     `json:"agent_rss_bytes"`
	PSI           PSI        `json:"psi"`
	Listeners     []Listen   `json:"listeners"`
	ListenNote    string     `json:"listen_note,omitempty"`
	Deleted       []Deleted  `json:"deleted"`
	DeletedNote   string     `json:"deleted_note,omitempty"`
	RAID          []RAID     `json:"raid"`
	RAIDNote      string     `json:"raid_note,omitempty"`
	NVMe          []NVMe     `json:"nvme"`
	NVMeNote      string     `json:"nvme_note,omitempty"`
	IPMINote      string     `json:"ipmi_note,omitempty"`
	KernelEvents  []KEvent   `json:"kernel_events"`
	KernelNote    string     `json:"kernel_note,omitempty"`
	SSH           SSHReport  `json:"ssh"`
	Cron          []Cron     `json:"cron"`
	CronNote      string     `json:"cron_note,omitempty"`
	Mail          MailReport `json:"mail"`
}

// PSI is pressure-stall information. Available is false when /proc/pressure is missing.
type PSI struct {
	Available  bool    `json:"available"`
	MemorySome float64 `json:"memory_some"`
	MemoryFull float64 `json:"memory_full"`
	IOSome     float64 `json:"io_some"`
}

// Listen is one bound socket the panel process could see.
type Listen struct {
	Proto    string `json:"proto"`
	Addr     string `json:"addr"`
	Process  string `json:"process"`
	PID      int    `json:"pid"`
	User     string `json:"user"`
	Exposure string `json:"exposure"`
}

// Deleted is an unlinked file still held open.
type Deleted struct {
	PID     int    `json:"pid"`
	Process string `json:"process"`
	Path    string `json:"path"`
	Bytes   uint64 `json:"bytes"`
}

// RAID is one md array read from /proc/mdstat.
type RAID struct {
	Name    string `json:"name"`
	Level   string `json:"level"`
	State   string `json:"state"`
	Members string `json:"members"`
}

// NVMe is a device node the kernel exposes under /sys. Wear is omitted when sysfs has no figure.
type NVMe struct {
	Name  string   `json:"name"`
	Model string   `json:"model"`
	TempC *float64 `json:"temp_c"`
}

// KEvent is one kernel or package log line.
type KEvent struct {
	At    time.Time `json:"at"`
	Lane  string    `json:"lane"`
	Tone  string    `json:"tone"`
	Label string    `json:"label"`
}

// SSHReport summarises a readable auth log. Available is false when the log cannot be read.
type SSHReport struct {
	Available bool        `json:"available"`
	Note      string      `json:"note,omitempty"`
	Failed    int         `json:"failed"`
	Sources   []SSHSource `json:"sources"`
}

// SSHSource is one address already present in the system auth log.
type SSHSource struct {
	IP    string `json:"ip"`
	Fails int    `json:"fails"`
	Users string `json:"users"`
}

// Cron is one scheduled command read from a crontab file. Last run is not known.
type Cron struct {
	Command  string `json:"command"`
	Schedule string `json:"schedule"`
	Source   string `json:"source"`
}

// MailReport counts postfix status tokens in a readable mail log.
type MailReport struct {
	Available bool   `json:"available"`
	Note      string `json:"note,omitempty"`
	Sent      int    `json:"sent"`
	Deferred  int    `json:"deferred"`
	Bounced   int    `json:"bounced"`
}

// Reader caches a host report so page polls do not rescan /proc every time.
type Reader struct {
	proc string
	root string

	mu      sync.Mutex
	at      time.Time
	rep     Report
	procCPU prevCPU
}

type prevCPU struct {
	at    time.Time
	ticks uint64
}

// NewReader reads procRoot and hostRoot. Empty values mean /proc and /.
func NewReader(procRoot, hostRoot string) *Reader {
	return &Reader{proc: cleanRoot(procRoot, "/proc"), root: cleanRoot(hostRoot, "/")}
}

// Report returns a cached snapshot. The cache is a few seconds, not a substitute for history.
func (r *Reader) Report() Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.at.IsZero() && time.Since(r.at) < 8*time.Second {
		return r.rep
	}
	rep := collect(r.proc, r.root, &r.procCPU)
	r.rep = rep
	r.at = time.Now()
	return rep
}
