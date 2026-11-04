package metrics

import (
	"math"
	"testing"
)

func TestParseProcStatAndUsage(t *testing.T) {
	t.Parallel()
	first, err := parseProcStat("cpu  100 0 100 800 0 0 0 0 0 0\ncpu0 100 0 100 800 0 0 0 0 0 0\ncpu1 0 0 0 0 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	if first.Cores != 2 {
		t.Fatalf("cores = %d", first.Cores)
	}
	second, err := parseProcStat("cpu  200 0 200 900 0 0 0 0 0 0\ncpu0 200 0 200 900 0 0 0 0 0 0\n")
	if err != nil {
		t.Fatal(err)
	}
	got := cpuUsage(first, second)
	if math.Abs(got-200.0/300.0*100) > 0.01 {
		t.Fatalf("usage = %v", got)
	}
	if _, err := parseProcStat("intr 1\n"); err == nil {
		t.Fatal("missing cpu line should fail")
	}
}

func TestParseMemInfoSeparatesCacheFromUsed(t *testing.T) {
	t.Parallel()
	raw := "" +
		"MemTotal:       1000000 kB\n" +
		"MemFree:         100000 kB\n" +
		"MemAvailable:    600000 kB\n" +
		"Buffers:          50000 kB\n" +
		"Cached:          300000 kB\n"
	info, err := parseMemInfo(raw)
	if err != nil {
		t.Fatal(err)
	}
	if info.Total != 1000000*1024 || info.Available != 600000*1024 {
		t.Fatalf("meminfo = %+v", info)
	}
	if info.Buffers+info.Cached != 350000*1024 {
		t.Fatalf("cache = %d", info.Buffers+info.Cached)
	}
	used := info.Total - info.Available
	if used != 400000*1024 {
		t.Fatalf("used = %d", used)
	}

	fallback, err := parseMemInfo("MemTotal: 1000 kB\nMemFree: 100 kB\nBuffers: 50 kB\nCached: 25 kB\n")
	if err != nil {
		t.Fatal(err)
	}
	if fallback.Available != 175*1024 {
		t.Fatalf("available fallback = %d", fallback.Available)
	}
}

func TestParseLoadUptimeNetAndMounts(t *testing.T) {
	t.Parallel()
	load, err := parseLoadAvg("0.15 0.10 0.05 1/123 4567\n")
	if err != nil || load.Load1 != 0.15 || load.Load5 != 0.10 || load.Load15 != 0.05 {
		t.Fatalf("load = %+v, %v", load, err)
	}
	up, err := parseUptime("12345.67 890.1\n")
	if err != nil || up != 12345.67 {
		t.Fatalf("uptime = %v, %v", up, err)
	}

	raw := "" +
		"Inter-|   Receive                                                |  Transmit\n" +
		" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n" +
		"    lo: 10 1 0 0 0 0 0 0 20 1 0 0 0 0 0 0\n" +
		"  eth0: 1000 10 1 0 0 0 0 0 2000 20 2 0 0 0 0 0\n"
	ifaces, err := parseNetDev(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(ifaces) != 2 || ifaces[1].Name != "eth0" || ifaces[1].RxBytes != 1000 || ifaces[1].TxBytes != 2000 || ifaces[1].RxErrors != 1 || ifaces[1].TxErrors != 2 {
		t.Fatalf("ifaces = %+v", ifaces)
	}

	mounts := parseMounts("" +
		"/dev/sda1 / ext4 rw,relatime 0 0\n" +
		"tmpfs /run tmpfs rw 0 0\n" +
		"/dev/sda2 /boot ext4 rw 0 0\n" +
		"overlay /var/lib/docker/overlay2 overlay rw 0 0\n" +
		"/dev/sda1 / ext4 rw 0 0\n")
	if len(mounts) != 3 {
		t.Fatalf("mounts = %+v", mounts)
	}
	if mounts[0].Mount != "/" || mounts[1].Mount != "/boot" || mounts[2].FSType != "overlay" {
		t.Fatalf("mounts = %+v", mounts)
	}
}

func TestUnescapeMountSpace(t *testing.T) {
	t.Parallel()
	mounts := parseMounts(`/dev/sda1 /media/My\040Disk ext4 rw 0 0` + "\n")
	if len(mounts) != 1 || mounts[0].Mount != "/media/My Disk" {
		t.Fatalf("mounts = %+v", mounts)
	}
}
