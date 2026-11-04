//go:build linux

package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCollectorUsesFixtureProc(t *testing.T) {
	proc := t.TempDir()
	host := t.TempDir()
	if err := os.Mkdir(filepath.Join(host, "boot"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProc(t, proc, map[string]string{
		"stat":    "cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 50 0 50 400 0 0 0 0 0 0\ncpu1 50 0 50 400 0 0 0 0 0 0\n",
		"meminfo": "MemTotal: 1000000 kB\nMemFree: 100000 kB\nMemAvailable: 600000 kB\nBuffers: 50000 kB\nCached: 300000 kB\n",
		"loadavg": "0.20 0.10 0.05 1/10 1\n",
		"uptime":  "42.0 1.0\n",
		"net/dev": "Inter-|   Receive                                                |  Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n    lo: 1 1 0 0 0 0 0 0 1 1 0 0 0 0 0 0\n  eth0: 1000 10 1 0 0 0 0 0 2000 20 2 0 0 0 0 0\n",
		"mounts":  "/dev/sda1 / ext4 rw 0 0\ntmpfs /run tmpfs rw 0 0\n/dev/sda2 /boot ext4 rw 0 0\n",
	})

	c := NewCollector(proc, host)
	start := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	c.now = func() time.Time { return start }
	first, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != SourceProc || first.CPU.UsagePercent != nil {
		t.Fatalf("first cpu = %+v source %s", first.CPU, first.Source)
	}
	if first.CPU.Cores != 2 || first.Memory.UsedBytes != 400000*1024 || first.Memory.CacheBytes != 350000*1024 {
		t.Fatalf("first = %+v", first)
	}
	if first.Load.Load1 != 0.20 || first.UptimeSeconds != 42 {
		t.Fatalf("load/uptime = %+v %v", first.Load, first.UptimeSeconds)
	}
	if len(first.Network) != 1 || first.Network[0].Name != "eth0" || first.Network[0].RxBps != nil {
		t.Fatalf("network = %+v", first.Network)
	}
	if len(first.Disks) != 2 || first.Disks[0].Mount != "/" || first.Disks[0].TotalBytes == 0 {
		t.Fatalf("disks = %+v", first.Disks)
	}

	writeProc(t, proc, map[string]string{
		"stat":    "cpu  200 0 200 900 0 0 0 0 0 0\ncpu0 100 0 100 450 0 0 0 0 0 0\ncpu1 100 0 100 450 0 0 0 0 0 0\n",
		"net/dev": "Inter-|   Receive                                                |  Transmit\n face |bytes packets errs drop fifo frame compressed multicast|bytes packets errs drop fifo colls carrier compressed\n    lo: 1 1 0 0 0 0 0 0 1 1 0 0 0 0 0 0\n  eth0: 3000 10 1 0 0 0 0 0 4000 20 2 0 0 0 0 0\n",
	})
	c.now = func() time.Time { return start.Add(2 * time.Second) }
	second, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if second.CPU.UsagePercent == nil || *second.CPU.UsagePercent < 66 || *second.CPU.UsagePercent > 67 {
		t.Fatalf("usage = %v", second.CPU.UsagePercent)
	}
	if second.Network[0].RxBps == nil || *second.Network[0].RxBps != 1000 || *second.Network[0].TxBps != 1000 {
		t.Fatalf("rates = %+v", second.Network[0])
	}
}

func TestLiveProcSnapshot(t *testing.T) {
	if _, err := os.Stat("/proc/stat"); err != nil {
		t.Skip("/proc/stat is not available")
	}
	c := NewCollector("", "")
	first, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if first.Source != SourceProc || first.Memory.TotalBytes == 0 || first.CPU.Cores < 1 {
		t.Fatalf("live = %+v", first)
	}
	if first.CPU.UsagePercent != nil {
		t.Fatal("first live sample should not invent a CPU percent")
	}
	second, err := c.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if second.CPU.UsagePercent == nil || *second.CPU.UsagePercent < 0 || *second.CPU.UsagePercent > 100 {
		t.Fatalf("live usage = %v", second.CPU.UsagePercent)
	}
	if len(second.Disks) == 0 {
		t.Fatal("live snapshot has no disks")
	}
}

func writeProc(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
