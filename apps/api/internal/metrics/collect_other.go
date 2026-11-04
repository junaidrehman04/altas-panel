//go:build !linux

package metrics

import "runtime"

func (c *Collector) collect() (Snapshot, error) {
	return Snapshot{
		CollectedAt: c.now().UTC(),
		Hostname:    hostname(),
		Source:      SourceFallback,
		CPU: CPUStats{
			Cores: runtime.NumCPU(),
		},
		Disks:   []DiskStats{},
		Network: []NetStats{},
		Note:    "Host /proc metrics are read on Linux. This build reports the CPU count only.",
	}, nil
}

func statPath(string) (uint64, uint64, uint64, error) {
	return 0, 0, 0, errNoStat
}

type noStatError struct{}

func (noStatError) Error() string { return "disk stats are unavailable" }

var errNoStat = noStatError{}
