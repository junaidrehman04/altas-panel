package panelview

import (
	"math"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/analytics"
	"github.com/junaidrehman04/altas/apps/api/internal/auth"
	"github.com/junaidrehman04/altas/apps/api/internal/inspect"
	"github.com/junaidrehman04/altas/apps/api/internal/metrics"
	"github.com/junaidrehman04/altas/apps/api/internal/stacks"
)

// Sources is the real data a page can see. Nil pointers and empty slices stay empty.
type Sources struct {
	Now        time.Time
	Range      string
	Snap       metrics.Snapshot
	Series     metrics.Series
	Detail     []metrics.DetailPoint
	Cores      metrics.CoreHeat
	Summary    analytics.Summary
	Events     []auth.AuthEvent
	Host       inspect.Report
	Containers []stacks.RuntimeContainer
	AccessLog  bool
	Geo        bool
	Project    string
}

// Label is one chart axis tick.
type Label struct {
	Axis string `json:"axis"`
	Full string `json:"full"`
}

// Gap is a section the host cannot fill. The UI keeps the component and explains why.
type Gap struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// Edge is the traffic page.
type Edge struct {
	Range       string  `json:"range"`
	CollectedAt string  `json:"collected_at"`
	Network     Network `json:"network"`
	Traffic     Traffic `json:"traffic"`
	Classes     Gap     `json:"classes"`
	Protocols   Gap     `json:"protocols"`
	Forecast    Gap     `json:"forecast"`
	Clients     Gap     `json:"clients"`
	Events      []Event `json:"events"`
}

// Network is interface throughput.
type Network struct {
	Available    bool       `json:"available"`
	Iface        string     `json:"iface"`
	IngressMbps  *float64   `json:"ingress_mbps"`
	EgressMbps   *float64   `json:"egress_mbps"`
	PeakIngress  *float64   `json:"peak_ingress_mbps"`
	PeakEgress   *float64   `json:"peak_egress_mbps"`
	IngressDelta *float64   `json:"ingress_delta"`
	EgressDelta  *float64   `json:"egress_delta"`
	Labels       []Label    `json:"labels"`
	Ingress      []*float64 `json:"ingress"`
	Egress       []*float64 `json:"egress"`
}

// Traffic is request rollups. Class splits are not included.
type Traffic struct {
	Available       bool       `json:"available"`
	Source          string     `json:"source"`
	Requests        int64      `json:"requests"`
	ReqPerSec       *float64   `json:"req_per_s"`
	Visitors        int64      `json:"visitors"`
	Bytes           int64      `json:"bytes"`
	Errors          int64      `json:"errors"`
	ErrorRate       float64    `json:"error_rate"`
	P95MS           *float64   `json:"p95_ms"`
	GeoConfigured   bool       `json:"geo_configured"`
	Note            string     `json:"note"`
	Labels          []Label    `json:"labels"`
	ReqPerSecSeries []*float64 `json:"req_per_s_series"`
	Paths           []PathRow  `json:"paths"`
	Countries       []Country  `json:"countries"`
}

// PathRow is one normalised request path.
type PathRow struct {
	Path     string `json:"path"`
	Requests int64  `json:"requests"`
	Bytes    int64  `json:"bytes"`
}

// Country is a request count. Coordinates are nil when the centroid is unknown.
type Country struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Requests int64    `json:"requests"`
	Lat      *float64 `json:"lat"`
	Lon      *float64 `json:"lon"`
}

// Event is one real alert row for the security list.
type Event struct {
	At     string `json:"at"`
	Sev    string `json:"sev"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Loc    string `json:"loc"`
}

// Hardware is the diagnostics page.
type Hardware struct {
	Range       string      `json:"range"`
	CollectedAt string      `json:"collected_at"`
	Identity    Identity    `json:"identity"`
	Memory      MemoryView  `json:"memory"`
	Heat        HeatView    `json:"heat"`
	IOWait      SeriesView  `json:"iowait"`
	Steal       StealView   `json:"steal"`
	Filesystems []FSView    `json:"filesystems"`
	RAID        RAIDView    `json:"raid"`
	NVMe        NVMeView    `json:"nvme"`
	IPMI        Gap         `json:"ipmi"`
	Listeners   ListenView  `json:"listeners"`
	Deleted     DeletedView `json:"deleted"`
	Kernel      KernelView  `json:"kernel"`
	SSH         SSHView     `json:"ssh"`
}

// Identity is the fact strip.
type Identity struct {
	ID            string  `json:"id"`
	FQDN          string  `json:"fqdn"`
	Kind          string  `json:"kind"`
	Virt          string  `json:"virt"`
	Hypervisor    bool    `json:"hypervisor"`
	Provider      string  `json:"provider"`
	IP            string  `json:"ip"`
	OS            string  `json:"os"`
	Kernel        string  `json:"kernel"`
	CPU           string  `json:"cpu"`
	Cores         int     `json:"cores"`
	Threads       int     `json:"threads"`
	RAMBytes      uint64  `json:"ram_bytes"`
	Uptime        string  `json:"uptime"`
	Load1         float64 `json:"load1"`
	Load5         float64 `json:"load5"`
	Load15        float64 `json:"load15"`
	RebootPending bool    `json:"reboot_pending"`
	Status        string  `json:"status"`
}

// MemoryView is the composition panel.
type MemoryView struct {
	TotalBytes     uint64      `json:"total_bytes"`
	RSSBytes       uint64      `json:"rss_bytes"`
	BuffersBytes   uint64      `json:"buffers_bytes"`
	CacheBytes     uint64      `json:"cache_bytes"`
	FreeBytes      uint64      `json:"free_bytes"`
	AvailBytes     uint64      `json:"available_bytes"`
	AvailPercent   float64     `json:"available_percent"`
	MinAvailPct    *float64    `json:"min_available_percent"`
	SwapUsedBytes  uint64      `json:"swap_used_bytes"`
	SwapTotalBytes uint64      `json:"swap_total_bytes"`
	PSI            inspect.PSI `json:"psi"`
	Labels         []Label     `json:"labels"`
	RSS            []*float64  `json:"rss"`
	Buffers        []*float64  `json:"buffers"`
	Cache          []*float64  `json:"cache"`
	Available      []*float64  `json:"available"`
}

// HeatView is the per-core matrix. Values are empty until two samples exist.
type HeatView struct {
	Labels []Label     `json:"labels"`
	Cores  []string    `json:"cores"`
	Values [][]float64 `json:"values"`
}

// SeriesView is one line.
type SeriesView struct {
	Current *float64   `json:"current"`
	Peak    *float64   `json:"peak"`
	Labels  []Label    `json:"labels"`
	Values  []*float64 `json:"values"`
}

// StealView is hypervisor steal. Applicable is false on bare metal.
type StealView struct {
	Applicable bool       `json:"applicable"`
	Reason     string     `json:"reason"`
	Current    *float64   `json:"current"`
	Labels     []Label    `json:"labels"`
	Values     []*float64 `json:"values"`
}

// FSView is one mount from statfs.
type FSView struct {
	Mount      string   `json:"mount"`
	Device     string   `json:"device"`
	FS         string   `json:"fs"`
	UsedBytes  uint64   `json:"used_bytes"`
	TotalBytes uint64   `json:"total_bytes"`
	UsedPct    float64  `json:"used_percent"`
	InodePct   *float64 `json:"inode_percent"`
}

// RAIDView is mdstat or an explanation.
type RAIDView struct {
	Gap
	Arrays []inspect.RAID `json:"arrays"`
}

// NVMeView is sysfs or an explanation.
type NVMeView struct {
	Gap
	Devices []inspect.NVMe `json:"devices"`
}

// ListenView is bound sockets.
type ListenView struct {
	Note  string           `json:"note"`
	Ports []inspect.Listen `json:"ports"`
}

// DeletedView is unlinked open files.
type DeletedView struct {
	Note  string            `json:"note"`
	Files []inspect.Deleted `json:"files"`
}

// KernelView is readable kernel log lines.
type KernelView struct {
	Gap
	Events []KernelEvent `json:"events"`
	Labels []Label       `json:"labels"`
}

// KernelEvent is one log line placed on a lane.
type KernelEvent struct {
	Lane  string `json:"lane"`
	Tone  string `json:"tone"`
	Label string `json:"label"`
	At    string `json:"at"`
}

// SSHView is a readable auth log summary.
type SSHView struct {
	Gap
	Failed  int                 `json:"failed"`
	Sources []inspect.SSHSource `json:"sources"`
}

// APM is the projects page.
type APM struct {
	Range    string    `json:"range"`
	Host     APMHost   `json:"host"`
	Projects []Project `json:"projects"`
	Selected string    `json:"selected"`
	HTTP     HTTPView  `json:"http"`
	Queues   Gap       `json:"queues"`
	SQL      Gap       `json:"sql"`
	Cache    Gap       `json:"cache"`
	Cron     CronView  `json:"cron"`
	Mail     MailView  `json:"mail"`
}

// APMHost is capacity for the attribution bar.
type APMHost struct {
	Cores        int      `json:"cores"`
	Threads      int      `json:"threads"`
	RAMBytes     uint64   `json:"ram_bytes"`
	CPUPercent   *float64 `json:"cpu_percent"`
	MemUsedBytes uint64   `json:"mem_used_bytes"`
}

// Project is the panel process or a container.
type Project struct {
	ID       string   `json:"id"`
	Domain   string   `json:"domain"`
	Stack    string   `json:"stack"`
	Color    string   `json:"color"`
	Source   string   `json:"source"`
	CPU      *float64 `json:"cpu"`
	RAMBytes *uint64  `json:"ram_bytes"`
	RPS      *float64 `json:"rps"`
	P95MS    *float64 `json:"p95_ms"`
	Err5xx   *float64 `json:"err5xx"`
	HTTP     bool     `json:"http"`
	Status   string   `json:"status"`
}

// HTTPView is request timing for the selected project when it is the panel API.
type HTTPView struct {
	Gap
	Labels  []Label    `json:"labels"`
	RPS     []*float64 `json:"rps"`
	P95     []*float64 `json:"p95"`
	ErrRate []*float64 `json:"error_rate"`
	OK      int64      `json:"ok"`
	Err     int64      `json:"errors"`
	Paths   []PathRow  `json:"paths"`
}

// CronView is readable crontab lines. Run history is not known.
type CronView struct {
	Gap
	Jobs []inspect.Cron `json:"jobs"`
}

// MailView is postfix status counts when a log is readable.
type MailView struct {
	Gap
	Sent     int `json:"sent"`
	Deferred int `json:"deferred"`
	Bounced  int `json:"bounced"`
}

// Alert is one notification.
type Alert struct {
	ID     string `json:"id"`
	Sev    string `json:"sev"`
	Icon   string `json:"icon"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Host   string `json:"host"`
	Time   string `json:"time"`
	Screen string `json:"screen"`
	Tab    string `json:"tab"`
	Read   bool   `json:"read"`
}

// BuildEdge assembles the traffic page from measured series.
func BuildEdge(src Sources) Edge {
	out := Edge{
		Range:       src.Range,
		CollectedAt: stamp(src),
		Classes:     Gap{Reason: "Human, trusted-bot, AI crawler, and malicious labels are not measured. No classifier or WAF verdict is configured."},
		Protocols:   Gap{Reason: "Per-protocol throughput is not measured on this host."},
		Forecast:    Gap{Reason: "No monthly egress plan is configured, so a forecast against a cap is not shown."},
		Clients:     Gap{Reason: "Client addresses are not stored. Visitor counts use a daily truncated hash."},
		Events:      []Event{},
	}
	out.Network = networkView(src)
	out.Traffic = trafficView(src)
	out.Events = edgeEvents(src)
	return out
}

// BuildHardware assembles the diagnostics page.
func BuildHardware(src Sources) Hardware {
	id := identity(src)
	out := Hardware{
		Range:       src.Range,
		CollectedAt: stamp(src),
		Identity:    id,
		Memory:      memoryView(src),
		Heat:        heatView(src),
		IOWait:      waitView(src, true),
		Steal:       stealView(src),
		Filesystems: filesystems(src.Snap),
		RAID:        RAIDView{Gap: gapNote(len(src.Host.RAID) > 0, src.Host.RAIDNote), Arrays: nonemptyRAID(src.Host.RAID)},
		NVMe:        NVMeView{Gap: gapNote(len(src.Host.NVMe) > 0, src.Host.NVMeNote), Devices: nonemptyNVMe(src.Host.NVMe)},
		IPMI:        Gap{Reason: src.Host.IPMINote},
		Listeners:   ListenView{Note: src.Host.ListenNote, Ports: src.Host.Listeners},
		Deleted:     DeletedView{Note: src.Host.DeletedNote, Files: src.Host.Deleted},
		Kernel:      kernelView(src),
		SSH: SSHView{
			Gap:     gapNote(src.Host.SSH.Available, src.Host.SSH.Note),
			Failed:  src.Host.SSH.Failed,
			Sources: src.Host.SSH.Sources,
		},
	}
	if out.SSH.Sources == nil {
		out.SSH.Sources = []inspect.SSHSource{}
	}
	if out.Listeners.Ports == nil {
		out.Listeners.Ports = []inspect.Listen{}
	}
	if out.Deleted.Files == nil {
		out.Deleted.Files = []inspect.Deleted{}
	}
	return out
}

// BuildAPM assembles the projects page.
func BuildAPM(src Sources) APM {
	projects := projectsOf(src)
	selected := src.Project
	if selected == "" && len(projects) > 0 {
		selected = projects[0].ID
	}
	found := false
	for _, project := range projects {
		if project.ID == selected {
			found = true
			break
		}
	}
	if !found && len(projects) > 0 {
		selected = projects[0].ID
	}
	out := APM{
		Range:    src.Range,
		Host:     apmHost(src),
		Projects: projects,
		Selected: selected,
		Queues:   Gap{Reason: "Queue depth is not measured. No Redis, database, or worker inspector is configured."},
		SQL:      Gap{Reason: "SQL statements are not collected. No database statistics source is configured."},
		Cache:    Gap{Reason: "Object-cache hit ratio is not measured for this host."},
		Cron: CronView{
			Gap:  gapNote(src.Host.CronNote == "" || len(src.Host.Cron) > 0, emptyNote(src.Host.CronNote, "No crontab entries were readable.")),
			Jobs: src.Host.Cron,
		},
		Mail: MailView{
			Gap:      gapNote(src.Host.Mail.Available, src.Host.Mail.Note),
			Sent:     src.Host.Mail.Sent,
			Deferred: src.Host.Mail.Deferred,
			Bounced:  src.Host.Mail.Bounced,
		},
	}
	if out.Cron.Jobs == nil {
		out.Cron.Jobs = []inspect.Cron{}
	}
	if selected == "altas-api" {
		out.HTTP = httpView(src)
	} else {
		out.HTTP = HTTPView{Gap: Gap{Reason: "No request log is attributed to this project. Panel traffic is counted only for the API process."}, Paths: []PathRow{}}
	}
	return out
}

// BuildAlerts returns current threshold and exposure alerts. It does not invent incidents.
func BuildAlerts(src Sources) []Alert {
	host := src.Host.ID
	if host == "" {
		host = "host"
	}
	var out []Alert
	if src.Snap.Memory.TotalBytes > 0 {
		avail := 100 - src.Snap.Memory.UsedPercent
		if src.Snap.Memory.AvailableBytes > 0 {
			avail = float64(src.Snap.Memory.AvailableBytes) / float64(src.Snap.Memory.TotalBytes) * 100
		}
		switch {
		case avail < 10:
			out = append(out, Alert{ID: "mem-available", Sev: "critical", Icon: "memory", Title: "MemAvailable below 10%", Detail: "The kernel reports little reclaimable memory.", Host: host, Time: "now", Screen: "hardware", Tab: "overview"})
		case avail < 20:
			out = append(out, Alert{ID: "mem-available", Sev: "warning", Icon: "memory", Title: "MemAvailable below 20%", Detail: "Memory pressure is elevated.", Host: host, Time: "now", Screen: "hardware", Tab: "overview"})
		}
	}
	for _, disk := range src.Snap.Disks {
		switch {
		case disk.UsedPercent >= 90:
			out = append(out, Alert{ID: "disk:" + disk.Mount, Sev: "critical", Icon: "disk", Title: disk.Mount + " is " + format1(disk.UsedPercent) + "% full", Detail: "statfs used space is at or above 90%.", Host: host, Time: "now", Screen: "hardware", Tab: "storage"})
		case disk.UsedPercent >= 80:
			out = append(out, Alert{ID: "disk:" + disk.Mount, Sev: "warning", Icon: "disk", Title: disk.Mount + " is " + format1(disk.UsedPercent) + "% full", Detail: "statfs used space is at or above 80%.", Host: host, Time: "now", Screen: "hardware", Tab: "storage"})
		}
		if disk.InodeUsedPercent != nil && *disk.InodeUsedPercent >= 90 {
			out = append(out, Alert{ID: "inode:" + disk.Mount, Sev: "critical", Icon: "disk", Title: disk.Mount + " inodes at " + format1(*disk.InodeUsedPercent) + "%", Detail: "Inode usage is at or above 90%.", Host: host, Time: "now", Screen: "hardware", Tab: "storage"})
		}
	}
	cores := src.Snap.CPU.Cores
	if cores < 1 {
		cores = src.Host.Cores
	}
	if cores > 0 && src.Snap.Load.Load1 > float64(cores*2) {
		out = append(out, Alert{ID: "load", Sev: "warning", Icon: "cpu", Title: "Load average above 2× cores", Detail: "1-minute load is high for the visible CPU count.", Host: host, Time: "now", Screen: "hardware", Tab: "overview"})
	}
	if src.Host.Hypervisor && src.Snap.CPU.StealPercent != nil && *src.Snap.CPU.StealPercent >= 15 {
		out = append(out, Alert{ID: "steal", Sev: "warning", Icon: "cpu", Title: "CPU steal above 15%", Detail: "The hypervisor is running other work while this vCPU is runnable.", Host: host, Time: "now", Screen: "hardware", Tab: "overview"})
	}
	if src.Host.RebootPending {
		out = append(out, Alert{ID: "reboot", Sev: "info", Icon: "refresh", Title: "Reboot pending", Detail: "The host reports a pending reboot.", Host: host, Time: "now", Screen: "hardware", Tab: "kernel"})
	}
	for _, port := range src.Host.Listeners {
		if port.Exposure != "public" || !sensitivePort(port.Addr) {
			continue
		}
		out = append(out, Alert{ID: "port:" + port.Addr, Sev: "critical", Icon: "network", Title: port.Process + " " + port.Addr + " is public", Detail: "The socket is bound to a non-loopback address. Firewall rules are not visible to the panel.", Host: host, Time: "now", Screen: "hardware", Tab: "kernel"})
	}
	if src.Summary.Totals.Requests >= 20 && src.Summary.Totals.ErrorRate >= 1 {
		out = append(out, Alert{ID: "panel-5xx", Sev: "warning", Icon: "activity", Title: "Panel 5xx rate is elevated", Detail: "API responses at status 500 or higher are at least 1% of requests in this range.", Host: host, Time: "now", Screen: "apm", Tab: "performance"})
	}
	for _, ev := range src.Events {
		if ev.Kind != "sign_in_locked" && ev.Kind != "totp_failed" {
			continue
		}
		out = append(out, Alert{ID: "auth:" + ev.Kind + ":" + ev.At, Sev: "warning", Icon: "shield", Title: "Sign-in protection event", Detail: ev.Detail, Host: host, Time: ev.At, Screen: "edge", Tab: "security"})
		if len(out) > 24 {
			break
		}
	}
	if out == nil {
		out = []Alert{}
	}
	return out
}

func networkView(src Sources) Network {
	netw := Network{Labels: []Label{}, Ingress: []*float64{}, Egress: []*float64{}}
	var rx, tx float64
	var have bool
	iface := ""
	var best float64
	for _, item := range src.Snap.Network {
		if item.RxBps == nil || item.TxBps == nil {
			continue
		}
		have = true
		rx += *item.RxBps
		tx += *item.TxBps
		if *item.RxBps+*item.TxBps >= best {
			best = *item.RxBps + *item.TxBps
			iface = item.Name
		}
	}
	if have {
		netw.Available = true
		netw.Iface = iface
		in := bytesToMbps(rx)
		eg := bytesToMbps(tx)
		netw.IngressMbps = &in
		netw.EgressMbps = &eg
	}
	for _, point := range src.Series.Points {
		if point.RxBps == nil || point.TxBps == nil {
			continue
		}
		netw.Labels = append(netw.Labels, labelAt(point.T, src.Range))
		in := bytesToMbps(*point.RxBps)
		eg := bytesToMbps(*point.TxBps)
		netw.Ingress = append(netw.Ingress, &in)
		netw.Egress = append(netw.Egress, &eg)
	}
	netw.PeakIngress = peak(netw.Ingress)
	netw.PeakEgress = peak(netw.Egress)
	netw.IngressDelta = deltaPct(netw.Ingress)
	netw.EgressDelta = deltaPct(netw.Egress)
	if len(netw.Labels) > 0 {
		netw.Available = true
	}
	return netw
}

func trafficView(src Sources) Traffic {
	tr := Traffic{
		GeoConfigured:   src.Geo,
		Labels:          []Label{},
		ReqPerSecSeries: []*float64{},
		Paths:           []PathRow{},
		Countries:       []Country{},
	}
	if src.AccessLog {
		tr.Source = "access log"
		tr.Note = "Combined access log tail, plus requests the panel API handled."
	} else {
		tr.Source = "panel"
		tr.Note = "Counts are requests the panel API handled. No external access log is configured."
	}
	tr.Requests = src.Summary.Totals.Requests
	tr.Visitors = src.Summary.Totals.Visitors
	tr.Bytes = src.Summary.Totals.Bytes
	tr.Errors = src.Summary.Totals.Errors
	tr.ErrorRate = src.Summary.Totals.ErrorRate
	tr.P95MS = src.Summary.Totals.P95MS
	windowSec := src.Now.Sub(spanStart(src.Now, src.Range)).Seconds()
	if windowSec > 0 && tr.Requests > 0 {
		rate := float64(tr.Requests) / windowSec
		tr.ReqPerSec = &rate
	}
	step := bucketSeconds(src.Summary.Resolution)
	for _, point := range src.Summary.Series {
		tr.Labels = append(tr.Labels, labelAt(point.T, src.Range))
		if step <= 0 {
			tr.ReqPerSecSeries = append(tr.ReqPerSecSeries, nil)
			continue
		}
		rate := float64(point.Requests) / step
		tr.ReqPerSecSeries = append(tr.ReqPerSecSeries, &rate)
	}
	for _, path := range src.Summary.TopPaths {
		tr.Paths = append(tr.Paths, PathRow{Path: path.Name, Requests: path.Requests, Bytes: path.Bytes})
	}
	var total int64
	for _, country := range src.Summary.Countries {
		total += country.Requests
		name, lat, lon, ok := countryCentroid(country.Name)
		row := Country{Code: country.Name, Name: name, Requests: country.Requests}
		if ok {
			row.Lat = &lat
			row.Lon = &lon
		}
		tr.Countries = append(tr.Countries, row)
	}
	tr.Available = tr.Requests > 0 || len(tr.Labels) > 0
	if !src.Geo {
		tr.Note += " GeoIP database is not configured, so countries stay empty."
	}
	_ = total
	return tr
}

func edgeEvents(src Sources) []Event {
	var out []Event
	for _, ev := range src.Events {
		sev := "info"
		title := "Sign-in event"
		switch ev.Kind {
		case "sign_in_failed", "totp_failed":
			sev = "warning"
			title = "Failed sign-in"
		case "sign_in_locked":
			sev = "critical"
			title = "Sign-in locked"
		case "recovery_used":
			sev = "warning"
			title = "Recovery code used"
		default:
			continue
		}
		out = append(out, Event{At: ev.At, Sev: sev, Title: title, Detail: ev.Detail, Loc: "Panel"})
	}
	if out == nil {
		out = []Event{}
	}
	return out
}

func identity(src Sources) Identity {
	ram := src.Snap.Memory.TotalBytes
	cores := src.Snap.CPU.Cores
	if cores == 0 {
		cores = src.Host.Cores
	}
	threads := src.Host.Threads
	if threads == 0 {
		threads = cores
	}
	id := src.Host.ID
	if id == "" {
		id = "host"
	}
	return Identity{
		ID: id, FQDN: fallback(src.Host.FQDN, id), Kind: fallback(src.Host.Kind, "unknown"),
		Virt: fallback(src.Host.Virt, "Unavailable"), Hypervisor: src.Host.Hypervisor,
		Provider: fallback(src.Host.Provider, src.Host.Virt), IP: src.Host.IP,
		OS: src.Host.OS, Kernel: src.Host.Kernel, CPU: src.Host.CPUModel,
		Cores: cores, Threads: threads, RAMBytes: ram,
		Uptime: formatUptime(src.Snap.UptimeSeconds),
		Load1:  src.Snap.Load.Load1, Load5: src.Snap.Load.Load5, Load15: src.Snap.Load.Load15,
		RebootPending: src.Host.RebootPending,
		Status:        hostStatus(src),
	}
}

func memoryView(src Sources) MemoryView {
	mem := src.Snap.Memory
	view := MemoryView{
		TotalBytes: mem.TotalBytes, RSSBytes: mem.AnonBytes, BuffersBytes: mem.BuffersBytes,
		CacheBytes: mem.CachedBytes, FreeBytes: mem.FreeBytes, AvailBytes: mem.AvailableBytes,
		SwapUsedBytes: mem.SwapUsedBytes, SwapTotalBytes: mem.SwapTotalBytes, PSI: src.Host.PSI,
		Labels: []Label{}, RSS: []*float64{}, Buffers: []*float64{}, Cache: []*float64{}, Available: []*float64{},
	}
	if mem.TotalBytes > 0 {
		view.AvailPercent = float64(mem.AvailableBytes) / float64(mem.TotalBytes) * 100
	}
	var minPct *float64
	for _, point := range src.Detail {
		if point.RSS == nil || point.Avail == nil || mem.TotalBytes == 0 {
			continue
		}
		view.Labels = append(view.Labels, labelAt(point.T, src.Range))
		view.RSS = append(view.RSS, gb(point.RSS))
		view.Buffers = append(view.Buffers, gb(point.Buffers))
		view.Cache = append(view.Cache, gb(point.Cache))
		view.Available = append(view.Available, gb(point.Avail))
		pct := float64(*point.Avail) / float64(mem.TotalBytes) * 100
		if minPct == nil || pct < *minPct {
			minPct = &pct
		}
	}
	view.MinAvailPct = minPct
	return view
}

func heatView(src Sources) HeatView {
	view := HeatView{Labels: []Label{}, Cores: []string{}, Values: [][]float64{}}
	if len(src.Cores.Cores) == 0 || len(src.Cores.Buckets) < 2 {
		return view
	}
	view.Cores = src.Cores.Cores
	view.Values = src.Cores.Values
	for _, bucket := range src.Cores.Buckets {
		view.Labels = append(view.Labels, labelAt(bucket, src.Range))
	}
	return view
}

func waitView(src Sources, iowait bool) SeriesView {
	view := SeriesView{Labels: []Label{}, Values: []*float64{}}
	var peakV *float64
	for _, point := range src.Detail {
		value := point.IOWait
		if !iowait {
			value = point.Steal
		}
		if value == nil {
			continue
		}
		view.Labels = append(view.Labels, labelAt(point.T, src.Range))
		view.Values = append(view.Values, value)
		if peakV == nil || *value > *peakV {
			copied := *value
			peakV = &copied
		}
	}
	view.Peak = peakV
	if iowait {
		view.Current = src.Snap.CPU.IOWaitPercent
	}
	return view
}

func stealView(src Sources) StealView {
	if !src.Host.Hypervisor {
		return StealView{Reason: "Steal time is only reported inside virtual machines. This host has no hypervisor flag.", Labels: []Label{}, Values: []*float64{}}
	}
	series := waitView(src, false)
	return StealView{Applicable: true, Current: src.Snap.CPU.StealPercent, Labels: series.Labels, Values: series.Values}
}

func filesystems(snap metrics.Snapshot) []FSView {
	out := []FSView{}
	for _, disk := range snap.Disks {
		out = append(out, FSView{
			Mount: disk.Mount, Device: disk.Device, FS: disk.Filesystem,
			UsedBytes: disk.UsedBytes, TotalBytes: disk.TotalBytes, UsedPct: disk.UsedPercent,
			InodePct: disk.InodeUsedPercent,
		})
	}
	return out
}

func kernelView(src Sources) KernelView {
	view := KernelView{Gap: gapNote(len(src.Host.KernelEvents) > 0 || src.Host.KernelNote == "", src.Host.KernelNote), Events: []KernelEvent{}, Labels: []Label{}}
	if src.Host.KernelNote != "" && len(src.Host.KernelEvents) == 0 {
		view.Available = false
		view.Reason = src.Host.KernelNote
	}
	for _, ev := range src.Host.KernelEvents {
		view.Events = append(view.Events, KernelEvent{Lane: ev.Lane, Tone: ev.Tone, Label: ev.Label, At: ev.At.UTC().Format(time.RFC3339)})
	}
	if len(view.Events) > 0 {
		view.Available = true
		view.Reason = ""
	}
	return view
}

func projectsOf(src Sources) []Project {
	colors := []string{"var(--p1)", "var(--p2)", "var(--p3)", "var(--p4)", "var(--p5)", "var(--p6)", "var(--p7)"}
	hostName := fallback(src.Host.FQDN, src.Host.ID)
	var cpu *float64
	if src.Host.AgentCPU != nil {
		cpu = src.Host.AgentCPU
	}
	var ram *uint64
	if src.Host.AgentRSS > 0 {
		v := src.Host.AgentRSS
		ram = &v
	}
	panel := Project{
		ID: "altas-api", Domain: hostName, Stack: "Go · this panel", Color: colors[0], Source: "process",
		CPU: cpu, RAMBytes: ram, HTTP: true, Status: "ok",
	}
	if src.Summary.Totals.Requests > 0 {
		windowSec := src.Now.Sub(spanStart(src.Now, src.Range)).Seconds()
		if windowSec > 0 {
			rate := float64(src.Summary.Totals.Requests) / windowSec
			panel.RPS = &rate
		}
		panel.P95MS = src.Summary.Totals.P95MS
		errRate := src.Summary.Totals.ErrorRate
		panel.Err5xx = &errRate
		if errRate >= 1 {
			panel.Status = "critical"
		}
	}
	out := []Project{panel}
	for i, container := range src.Containers {
		status := "ok"
		if container.State != "" && container.State != "running" {
			status = "warning"
		}
		out = append(out, Project{
			ID: container.Name, Domain: container.Name, Stack: container.Image, Color: colors[(i+1)%len(colors)],
			Source: "container", CPU: container.CPUPercent, RAMBytes: container.MemoryBytes, Status: status,
		})
	}
	return out
}

func apmHost(src Sources) APMHost {
	cores := src.Snap.CPU.Cores
	if cores == 0 {
		cores = src.Host.Cores
	}
	threads := src.Host.Threads
	if threads == 0 {
		threads = cores
	}
	return APMHost{
		Cores: cores, Threads: threads, RAMBytes: src.Snap.Memory.TotalBytes,
		CPUPercent: src.Snap.CPU.UsagePercent, MemUsedBytes: src.Snap.Memory.UsedBytes,
	}
}

func httpView(src Sources) HTTPView {
	view := HTTPView{Labels: []Label{}, RPS: []*float64{}, P95: []*float64{}, ErrRate: []*float64{}, Paths: []PathRow{}}
	step := bucketSeconds(src.Summary.Resolution)
	for _, point := range src.Summary.Series {
		view.Labels = append(view.Labels, labelAt(point.T, src.Range))
		if step > 0 {
			rate := float64(point.Requests) / step
			view.RPS = append(view.RPS, &rate)
		}
		if point.P95MS != nil {
			view.P95 = append(view.P95, point.P95MS)
		} else {
			view.P95 = append(view.P95, nil)
		}
		rate := point.ErrorRate
		view.ErrRate = append(view.ErrRate, &rate)
	}
	view.OK = src.Summary.Totals.Requests - src.Summary.Totals.Errors
	if view.OK < 0 {
		view.OK = 0
	}
	view.Err = src.Summary.Totals.Errors
	for _, path := range src.Summary.TopPaths {
		view.Paths = append(view.Paths, PathRow{Path: path.Name, Requests: path.Requests, Bytes: path.Bytes})
	}
	if src.Summary.Totals.Requests == 0 && len(view.Labels) == 0 {
		view.Reason = "No panel API requests in this range yet."
		return view
	}
	view.Available = true
	return view
}

func hostStatus(src Sources) string {
	for _, alert := range BuildAlerts(src) {
		if alert.Sev == "critical" {
			return "critical"
		}
	}
	for _, alert := range BuildAlerts(src) {
		if alert.Sev == "warning" {
			return "warning"
		}
	}
	return "ok"
}

func gapNote(ok bool, note string) Gap {
	if ok {
		return Gap{Available: true}
	}
	if note == "" {
		note = "This source is not available on this host."
	}
	return Gap{Reason: note}
}

func emptyNote(note, fallback string) string {
	if note != "" {
		return note
	}
	return fallback
}

func nonemptyRAID(items []inspect.RAID) []inspect.RAID {
	if items == nil {
		return []inspect.RAID{}
	}
	return items
}

func nonemptyNVMe(items []inspect.NVMe) []inspect.NVMe {
	if items == nil {
		return []inspect.NVMe{}
	}
	return items
}

func stamp(src Sources) string {
	if !src.Snap.CollectedAt.IsZero() {
		return src.Snap.CollectedAt.UTC().Format(time.RFC3339)
	}
	if src.Now.IsZero() {
		return time.Now().UTC().Format(time.RFC3339)
	}
	return src.Now.UTC().Format(time.RFC3339)
}

func labelAt(rfc, rangeName string) Label {
	t, err := time.Parse(time.RFC3339, rfc)
	if err != nil {
		return Label{Axis: rfc, Full: rfc}
	}
	local := t.Local()
	full := local.Format("Jan 2, 15:04")
	axis := local.Format("15:04")
	if rangeName == "7d" || rangeName == "30d" {
		axis = local.Format("Jan 2")
	}
	return Label{Axis: axis, Full: full}
}

func spanStart(now time.Time, name string) time.Time {
	switch name {
	case "1h":
		return now.Add(-time.Hour)
	case "6h":
		return now.Add(-6 * time.Hour)
	case "7d":
		return now.Add(-7 * 24 * time.Hour)
	case "30d":
		return now.Add(-30 * 24 * time.Hour)
	default:
		return now.Add(-24 * time.Hour)
	}
}

func bucketSeconds(resolution string) float64 {
	switch resolution {
	case "15s":
		return 15
	case "1m":
		return 60
	case "1h":
		return 3600
	case "1d", "1mo":
		return 86400
	default:
		return 60
	}
}

func bytesToMbps(bytesPerSec float64) float64 {
	return math.Round(bytesPerSec*8/1e6*10) / 10
}

func gb(v *uint64) *float64 {
	if v == nil {
		return nil
	}
	g := float64(*v) / (1024 * 1024 * 1024)
	return &g
}

func peak(vals []*float64) *float64 {
	var best *float64
	for _, v := range vals {
		if v == nil {
			continue
		}
		if best == nil || *v > *best {
			copied := *v
			best = &copied
		}
	}
	return best
}

func deltaPct(vals []*float64) *float64 {
	var first, last *float64
	for _, v := range vals {
		if v == nil {
			continue
		}
		if first == nil {
			first = v
		}
		last = v
	}
	if first == nil || last == nil || math.Abs(*first) < 0.05 {
		return nil
	}
	d := (*last - *first) / math.Abs(*first) * 100
	d = math.Round(d*10) / 10
	return &d
}

func format1(v float64) string {
	return strconvFormat(math.Round(v*10) / 10)
}

func strconvFormat(v float64) string {
	s := strconvTrim(v)
	return s
}

func strconvTrim(v float64) string {
	text := stringsTrimFloat(v)
	return text
}

func stringsTrimFloat(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	whole := int64(v)
	frac := int64(math.Round((v - float64(whole)) * 10))
	if frac == 10 {
		whole++
		frac = 0
	}
	out := itoa(whole) + "." + itoa(frac)
	if neg {
		return "-" + out
	}
	return out
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [32]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func formatUptime(seconds float64) string {
	if seconds <= 0 {
		return ""
	}
	total := int64(seconds)
	days := total / 86400
	hours := (total % 86400) / 3600
	mins := (total % 3600) / 60
	if days > 0 {
		return itoa(days) + "d " + two(hours) + ":" + two(mins)
	}
	return two(hours) + ":" + two(mins)
}

func two(v int64) string {
	if v < 10 {
		return "0" + itoa(v)
	}
	return itoa(v)
}

func fallback(value, alt string) string {
	if value == "" {
		return alt
	}
	return value
}

func sensitivePort(addr string) bool {
	_, port, ok := splitHostPort(addr)
	if !ok {
		return false
	}
	switch port {
	case "6379", "5432", "3306", "27017", "9200", "11211", "2375", "2376", "5900", "3389", "5672", "15672":
		return true
	default:
		return false
	}
}

func splitHostPort(addr string) (string, string, bool) {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i], addr[i+1:], true
		}
	}
	return "", "", false
}
