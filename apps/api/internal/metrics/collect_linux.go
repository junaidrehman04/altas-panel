//go:build linux

package metrics

import (
	"fmt"
	"runtime"
	"sort"
	"time"
)

func (c *Collector) collect() (Snapshot, error) {
	now := c.now()
	statRaw, err := c.read("stat")
	if err != nil {
		return Snapshot{}, fmt.Errorf("read proc stat: %w", err)
	}
	sample, err := parseProcStat(statRaw)
	if err != nil {
		return Snapshot{}, err
	}
	memRaw, err := c.read("meminfo")
	if err != nil {
		return Snapshot{}, fmt.Errorf("read proc meminfo: %w", err)
	}
	mem, err := parseMemInfo(memRaw)
	if err != nil {
		return Snapshot{}, err
	}

	snap := Snapshot{
		CollectedAt: now.UTC(),
		Hostname:    hostname(),
		Source:      SourceProc,
		CPU: CPUStats{
			Cores: sample.Cores,
		},
		Memory: MemoryStats{
			TotalBytes:     mem.Total,
			AvailableBytes: mem.Available,
			FreeBytes:      mem.Free,
			CacheBytes:     mem.Buffers + mem.Cached,
			BuffersBytes:   mem.Buffers,
			CachedBytes:    mem.Cached,
			AnonBytes:      anonBytes(mem),
			SwapTotalBytes: mem.SwapTotal,
			SwapUsedBytes:  swapUsed(mem),
		},
		Disks:   []DiskStats{},
		Network: []NetStats{},
	}
	if snap.CPU.Cores == 0 {
		snap.CPU.Cores = runtime.NumCPU()
	}
	if mem.Total >= mem.Available {
		snap.Memory.UsedBytes = mem.Total - mem.Available
	}
	if mem.Total > 0 {
		snap.Memory.UsedPercent = float64(snap.Memory.UsedBytes) / float64(mem.Total) * 100
	}
	if c.prevCPU != nil {
		pct := cpuUsage(*c.prevCPU, sample)
		snap.CPU.UsagePercent = &pct
		if len(c.prevCPU.PerCore) == len(sample.PerCore) && len(sample.PerCore) > 0 {
			cores := make([]float64, len(sample.PerCore))
			for i := range sample.PerCore {
				if busy, ok := coreBusyPercent(c.prevCPU.PerCore[i], sample.PerCore[i]); ok {
					cores[i] = busy
				}
			}
			snap.CPU.PerCore = cores
		}
		if wait, ok := cpuPartPercent(*c.prevCPU, sample, func(sample cpuSample) uint64 { return sample.IOWait }); ok {
			snap.CPU.IOWaitPercent = &wait
		}
		if steal, ok := cpuPartPercent(*c.prevCPU, sample, func(sample cpuSample) uint64 { return sample.Steal }); ok {
			snap.CPU.StealPercent = &steal
		}
	}
	c.prevCPU = &sample

	if raw, err := c.read("loadavg"); err == nil {
		if load, err := parseLoadAvg(raw); err == nil {
			snap.Load = load
		}
	}
	if raw, err := c.read("uptime"); err == nil {
		if up, err := parseUptime(raw); err == nil {
			snap.UptimeSeconds = up
		}
	}
	if raw, err := c.read("net/dev"); err == nil {
		if ifaces, err := parseNetDev(raw); err == nil {
			snap.Network = c.networkRates(ifaces, now)
		}
	}
	if raw, err := c.read("mounts"); err == nil {
		snap.Disks = c.diskStats(parseMounts(raw))
	}
	c.prevAt = now
	return snap, nil
}

func (c *Collector) networkRates(ifaces []netCounters, now time.Time) []NetStats {
	filtered := make([]netCounters, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.Name == "lo" {
			continue
		}
		filtered = append(filtered, iface)
	}
	if len(filtered) == 0 {
		filtered = ifaces
	}
	if len(filtered) > 16 {
		filtered = filtered[:16]
	}
	sec := now.Sub(c.prevAt).Seconds()
	out := make([]NetStats, 0, len(filtered))
	next := make(map[string]netCounters, len(filtered))
	for _, iface := range filtered {
		item := NetStats{
			Name:      iface.Name,
			RxBytes:   iface.RxBytes,
			TxBytes:   iface.TxBytes,
			RxPackets: iface.RxPackets,
			TxPackets: iface.TxPackets,
			RxErrors:  iface.RxErrors,
			TxErrors:  iface.TxErrors,
		}
		if prev, ok := c.prevNet[iface.Name]; ok && sec > 0 {
			rx := rate(prev.RxBytes, iface.RxBytes, sec)
			tx := rate(prev.TxBytes, iface.TxBytes, sec)
			item.RxBps = &rx
			item.TxBps = &tx
		}
		next[iface.Name] = iface
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	c.prevNet = next
	return out
}

func anonBytes(mem memInfo) uint64 {
	if mem.Anon > 0 {
		return mem.Anon
	}
	used := mem.Free + mem.Buffers + mem.Cached
	if mem.Total >= used {
		return mem.Total - used
	}
	return 0
}

func swapUsed(mem memInfo) uint64 {
	if mem.SwapTotal >= mem.SwapFree {
		return mem.SwapTotal - mem.SwapFree
	}
	return 0
}

func rate(prev, curr uint64, seconds float64) float64 {
	if curr < prev || seconds <= 0 {
		return 0
	}
	return float64(curr-prev) / seconds
}

func (c *Collector) diskStats(mounts []mountEntry) []DiskStats {
	out := make([]DiskStats, 0, len(mounts))
	for _, mnt := range mounts {
		path := underRoot(c.host, mnt.Mount)
		total, used, avail, inode, err := statPathFull(path)
		if err != nil || total == 0 {
			continue
		}
		out = append(out, DiskStats{
			Mount:            mnt.Mount,
			Filesystem:       mnt.FSType,
			Device:           mnt.Device,
			TotalBytes:       total,
			UsedBytes:        used,
			AvailableBytes:   avail,
			UsedPercent:      usedPercent(used, avail),
			InodeUsedPercent: inode,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Mount == "/" && out[j].Mount != "/" {
			return true
		}
		if out[j].Mount == "/" && out[i].Mount != "/" {
			return false
		}
		return out[i].Mount < out[j].Mount
	})
	return out
}
