package metrics

import (
	"fmt"
	"strconv"
	"strings"
)

type cpuSample struct {
	User    uint64
	Nice    uint64
	System  uint64
	Idle    uint64
	IOWait  uint64
	IRQ     uint64
	SoftIRQ uint64
	Steal   uint64
	Cores   int
}

func (c cpuSample) total() uint64 {
	// guest and guest_nice are already included in user and nice.
	return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ + c.SoftIRQ + c.Steal
}

func cpuUsage(prev, curr cpuSample) float64 {
	prevIdle := prev.Idle + prev.IOWait
	currIdle := curr.Idle + curr.IOWait
	prevTotal := prev.total()
	currTotal := curr.total()
	if currTotal < prevTotal || currIdle < prevIdle {
		return 0
	}
	dt := currTotal - prevTotal
	if dt == 0 {
		return 0
	}
	used := float64(dt-(currIdle-prevIdle)) / float64(dt) * 100
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}

func parseProcStat(data string) (cpuSample, error) {
	var sample cpuSample
	found := false
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		name := fields[0]
		if name == "cpu" {
			values, err := parseU64s(fields[1:])
			if err != nil {
				return cpuSample{}, fmt.Errorf("proc stat: %w", err)
			}
			if len(values) < 4 {
				return cpuSample{}, fmt.Errorf("proc stat: short cpu line")
			}
			sample.User = values[0]
			sample.Nice = values[1]
			sample.System = values[2]
			sample.Idle = values[3]
			if len(values) > 4 {
				sample.IOWait = values[4]
			}
			if len(values) > 5 {
				sample.IRQ = values[5]
			}
			if len(values) > 6 {
				sample.SoftIRQ = values[6]
			}
			if len(values) > 7 {
				sample.Steal = values[7]
			}
			found = true
			continue
		}
		if len(name) > 3 && strings.HasPrefix(name, "cpu") && isDigits(name[3:]) {
			sample.Cores++
		}
	}
	if !found {
		return cpuSample{}, fmt.Errorf("proc stat: missing cpu line")
	}
	return sample, nil
}

type memInfo struct {
	Total     uint64
	Free      uint64
	Available uint64
	Buffers   uint64
	Cached    uint64
}

func parseMemInfo(data string) (memInfo, error) {
	values := map[string]uint64{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		n, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		mult := uint64(1)
		if len(fields) >= 3 && fields[2] == "kB" {
			mult = 1024
		}
		values[key] = n * mult
	}
	total, ok := values["MemTotal"]
	if !ok || total == 0 {
		return memInfo{}, fmt.Errorf("meminfo: missing MemTotal")
	}
	info := memInfo{
		Total:   total,
		Free:    values["MemFree"],
		Buffers: values["Buffers"],
		Cached:  values["Cached"],
	}
	if avail, ok := values["MemAvailable"]; ok {
		info.Available = avail
	} else {
		info.Available = info.Free + info.Buffers + info.Cached
		if info.Available > info.Total {
			info.Available = info.Total
		}
	}
	return info, nil
}

func parseLoadAvg(data string) (LoadStats, error) {
	fields := strings.Fields(data)
	if len(fields) < 3 {
		return LoadStats{}, fmt.Errorf("loadavg: short line")
	}
	var load LoadStats
	var err error
	if load.Load1, err = strconv.ParseFloat(fields[0], 64); err != nil {
		return LoadStats{}, fmt.Errorf("loadavg: %w", err)
	}
	if load.Load5, err = strconv.ParseFloat(fields[1], 64); err != nil {
		return LoadStats{}, fmt.Errorf("loadavg: %w", err)
	}
	if load.Load15, err = strconv.ParseFloat(fields[2], 64); err != nil {
		return LoadStats{}, fmt.Errorf("loadavg: %w", err)
	}
	return load, nil
}

func parseUptime(data string) (float64, error) {
	fields := strings.Fields(data)
	if len(fields) < 1 {
		return 0, fmt.Errorf("uptime: empty")
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, fmt.Errorf("uptime: %w", err)
	}
	return v, nil
}

type netCounters struct {
	Name      string
	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
}

func parseNetDev(data string) ([]netCounters, error) {
	var out []netCounters
	for _, line := range strings.Split(data, "\n") {
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		if name == "" || strings.Contains(name, "|") {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 16 {
			return nil, fmt.Errorf("net dev: short line for %s", name)
		}
		nums, err := parseU64s(fields[:16])
		if err != nil {
			return nil, fmt.Errorf("net dev: %w", err)
		}
		out = append(out, netCounters{
			Name:      name,
			RxBytes:   nums[0],
			RxPackets: nums[1],
			RxErrors:  nums[2],
			TxBytes:   nums[8],
			TxPackets: nums[9],
			TxErrors:  nums[10],
		})
	}
	return out, nil
}

type mountEntry struct {
	Device string
	Mount  string
	FSType string
}

func parseMounts(data string) []mountEntry {
	var out []mountEntry
	seen := map[string]struct{}{}
	for _, line := range strings.Split(data, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		device := unescapeMount(fields[0])
		mount := unescapeMount(fields[1])
		fs := fields[2]
		if !keepFilesystem(fs, device) {
			continue
		}
		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}
		out = append(out, mountEntry{Device: device, Mount: mount, FSType: fs})
		if len(out) == 16 {
			break
		}
	}
	return out
}

func keepFilesystem(fs, device string) bool {
	switch fs {
	case "ext2", "ext3", "ext4", "xfs", "btrfs", "zfs", "vfat", "exfat", "ntfs", "ntfs3", "f2fs", "overlay", "fuseblk":
		return true
	default:
		return strings.HasPrefix(device, "/dev/")
	}
}

func unescapeMount(value string) string {
	replacer := strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	)
	return replacer.Replace(value)
}

func parseU64s(fields []string) ([]uint64, error) {
	out := make([]uint64, len(fields))
	for i, field := range fields {
		n, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func usedPercent(used, avail uint64) float64 {
	denom := used + avail
	if denom == 0 {
		return 0
	}
	return float64(used) / float64(denom) * 100
}
