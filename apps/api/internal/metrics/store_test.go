package metrics

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/db"
	"github.com/junaidrehman04/altas/apps/api/internal/window"
)

func TestHostSeriesAveragesAndEmpty(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "altas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)

	now := time.Date(2026, 11, 4, 12, 0, 30, 0, time.UTC)
	span, err := window.Parse("24h", now)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := store.Series(ctx, span)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Points == nil || len(empty.Points) != 0 || empty.Resolution != "1m" {
		t.Fatalf("empty = %+v", empty)
	}

	cpu := func(v float64) *float64 { return &v }
	first := Snapshot{
		CollectedAt: now.Add(-2 * time.Minute),
		CPU:         CPUStats{UsagePercent: cpu(10), Cores: 2},
		Memory:      MemoryStats{UsedPercent: 40, UsedBytes: 400, TotalBytes: 1000},
		Disks: []DiskStats{
			{Mount: "/boot", TotalBytes: 10, UsedBytes: 5, UsedPercent: 50},
			{Mount: "/", TotalBytes: 1000, UsedBytes: 250, UsedPercent: 25},
		},
		Network: []NetStats{{Name: "eth0", RxBps: cpu(100), TxBps: cpu(40)}},
	}
	second := first
	second.CollectedAt = now.Add(-2 * time.Minute).Add(20 * time.Second)
	second.CPU.UsagePercent = cpu(30)
	second.Memory.UsedPercent = 50
	second.Memory.UsedBytes = 500
	second.Network = []NetStats{{Name: "eth0", RxBps: cpu(300), TxBps: cpu(60)}}
	if err := store.RecordSnapshot(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordSnapshot(ctx, second); err != nil {
		t.Fatal(err)
	}

	series, err := store.Series(ctx, span)
	if err != nil {
		t.Fatal(err)
	}
	if len(series.Points) != 1 {
		t.Fatalf("points = %+v", series.Points)
	}
	pt := series.Points[0]
	if pt.CPUPercent == nil || *pt.CPUPercent != 20 {
		t.Fatalf("cpu = %v", pt.CPUPercent)
	}
	if pt.MemUsedPercent != 45 || pt.MemUsedBytes != 450 {
		t.Fatalf("mem = %+v", pt)
	}
	if pt.DiskUsedPercent != 25 || pt.DiskUsedBytes != 250 {
		t.Fatalf("disk picked the wrong mount: %+v", pt)
	}
	if pt.RxBps == nil || *pt.RxBps != 200 || pt.TxBps == nil || *pt.TxBps != 50 {
		t.Fatalf("net = %+v", pt)
	}
}

func TestHostSeriesMonthGroupsDays(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "altas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)
	cpu := func(v float64) *float64 { return &v }
	for _, at := range []time.Time{
		time.Date(2026, 10, 2, 1, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC),
	} {
		snap := Snapshot{
			CollectedAt: at,
			CPU:         CPUStats{UsagePercent: cpu(10)},
			Memory:      MemoryStats{UsedPercent: 20, UsedBytes: 20, TotalBytes: 100},
		}
		if err := store.RecordSnapshot(ctx, snap); err != nil {
			t.Fatal(err)
		}
	}
	span, err := window.Parse("months", time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	series, err := store.Series(ctx, span)
	if err != nil {
		t.Fatal(err)
	}
	if series.Resolution != "1mo" || len(series.Points) != 2 {
		t.Fatalf("series = %+v", series)
	}
}

func TestHostPrune(t *testing.T) {
	ctx := context.Background()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "altas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	store := NewStore(sqlDB)
	cpu := func(v float64) *float64 { return &v }
	old := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := store.RecordSnapshot(ctx, Snapshot{
		CollectedAt: old,
		CPU:         CPUStats{UsagePercent: cpu(1)},
		Memory:      MemoryStats{UsedPercent: 1, UsedBytes: 1, TotalBytes: 2},
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.Prune(ctx, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM host_metric_buckets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("kept %d buckets", n)
	}
}
