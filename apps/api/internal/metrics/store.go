package metrics

import (
	"context"
	"database/sql"
	"math"
	"sync"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/window"
)

// Point is one averaged host bucket.
type Point struct {
	T               string   `json:"t"`
	CPUPercent      *float64 `json:"cpu_percent"`
	MemUsedPercent  float64  `json:"mem_used_percent"`
	MemUsedBytes    uint64   `json:"mem_used_bytes"`
	DiskUsedPercent float64  `json:"disk_used_percent"`
	DiskUsedBytes   uint64   `json:"disk_used_bytes"`
	RxBps           *float64 `json:"rx_bps"`
	TxBps           *float64 `json:"tx_bps"`
}

// Series is the metrics chart payload.
type Series struct {
	Range      string  `json:"range"`
	Resolution string  `json:"resolution"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Points     []Point `json:"points"`
}

// Store keeps host samples as fixed-size rollups.
type Store struct {
	db        *sql.DB
	mu        sync.Mutex
	lastPrune time.Time
}

// NewStore writes into the panel SQLite database.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// RecordSnapshot averages one true-state sample into 15s, 1m, 1h, and 1d buckets.
// The disk series follows the largest mount. Network rates are the sum of interfaces.
func (s *Store) RecordSnapshot(ctx context.Context, snap Snapshot) error {
	at := snap.CollectedAt.UTC()
	if snap.CollectedAt.IsZero() {
		at = time.Now().UTC()
	}
	var cpu float64
	cpuSamples := 0
	if snap.CPU.UsagePercent != nil {
		cpu = *snap.CPU.UsagePercent
		cpuSamples = 1
	}
	diskSamples := 0
	var diskPct float64
	var diskUsed, diskTotal uint64
	if len(snap.Disks) > 0 {
		best := snap.Disks[0]
		for _, disk := range snap.Disks[1:] {
			if disk.TotalBytes > best.TotalBytes {
				best = disk
			}
		}
		diskPct = best.UsedPercent
		diskUsed = best.UsedBytes
		diskTotal = best.TotalBytes
		diskSamples = 1
	}
	netSamples := 0
	var rx, tx float64
	for _, iface := range snap.Network {
		if iface.RxBps == nil || iface.TxBps == nil {
			continue
		}
		rx += *iface.RxBps
		tx += *iface.TxBps
		netSamples = 1
	}
	for _, resolution := range []string{"15s", "1m", "1h", "1d"} {
		bucket := window.BucketStart(at, resolution)
		_, err := s.db.ExecContext(ctx, `
			INSERT INTO host_metric_buckets (
				resolution, bucket_start, cpu_sum, cpu_samples,
				mem_pct_sum, mem_used_sum, mem_total,
				disk_pct_sum, disk_used_sum, disk_total, disk_samples,
				rx_sum, tx_sum, net_samples, samples
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
			ON CONFLICT(resolution, bucket_start) DO UPDATE SET
				cpu_sum = cpu_sum + excluded.cpu_sum,
				cpu_samples = cpu_samples + excluded.cpu_samples,
				mem_pct_sum = mem_pct_sum + excluded.mem_pct_sum,
				mem_used_sum = mem_used_sum + excluded.mem_used_sum,
				mem_total = excluded.mem_total,
				disk_pct_sum = disk_pct_sum + excluded.disk_pct_sum,
				disk_used_sum = disk_used_sum + excluded.disk_used_sum,
				disk_total = CASE WHEN excluded.disk_samples > 0 THEN excluded.disk_total ELSE disk_total END,
				disk_samples = disk_samples + excluded.disk_samples,
				rx_sum = rx_sum + excluded.rx_sum,
				tx_sum = tx_sum + excluded.tx_sum,
				net_samples = net_samples + excluded.net_samples,
				samples = samples + 1
		`, resolution, bucket, cpu, cpuSamples,
			snap.Memory.UsedPercent, snap.Memory.UsedBytes, snap.Memory.TotalBytes,
			diskPct, diskUsed, diskTotal, diskSamples,
			rx, tx, netSamples)
		if err != nil {
			return err
		}
	}
	s.maybePrune(ctx, time.Now().UTC())
	return nil
}

type hostRow struct {
	start       int64
	cpuSum      float64
	cpuSamples  int64
	memPctSum   float64
	memUsedSum  int64
	diskPctSum  float64
	diskUsedSum int64
	diskSamples int64
	rxSum       float64
	txSum       float64
	netSamples  int64
	samples     int64
}

// Series returns averaged buckets for span. An empty database returns an empty point list.
func (s *Store) Series(ctx context.Context, span window.Span) (Series, error) {
	resolution := span.HostResolution
	out := Series{
		Range:      span.Name,
		Resolution: resolution,
		From:       span.Start.UTC().Format(time.RFC3339),
		To:         span.End.UTC().Format(time.RFC3339),
		Points:     []Point{},
	}
	if span.Monthly {
		out.Resolution = "1mo"
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT bucket_start, cpu_sum, cpu_samples, mem_pct_sum, mem_used_sum,
		       disk_pct_sum, disk_used_sum, disk_samples, rx_sum, tx_sum, net_samples, samples
		FROM host_metric_buckets
		WHERE resolution = ? AND bucket_start >= ? AND bucket_start <= ?
		ORDER BY bucket_start
	`, resolution, span.Start.Unix(), span.End.Unix())
	if err != nil {
		return Series{}, err
	}
	defer rows.Close()
	var loaded []hostRow
	for rows.Next() {
		var row hostRow
		if err := rows.Scan(
			&row.start, &row.cpuSum, &row.cpuSamples, &row.memPctSum, &row.memUsedSum,
			&row.diskPctSum, &row.diskUsedSum, &row.diskSamples, &row.rxSum, &row.txSum, &row.netSamples, &row.samples,
		); err != nil {
			return Series{}, err
		}
		loaded = append(loaded, row)
	}
	if err := rows.Err(); err != nil {
		return Series{}, err
	}
	if span.Monthly {
		loaded = foldHostMonths(loaded)
	}
	for _, row := range loaded {
		if row.samples <= 0 {
			continue
		}
		point := Point{
			T:              time.Unix(row.start, 0).UTC().Format(time.RFC3339),
			MemUsedPercent: round2(row.memPctSum / float64(row.samples)),
			MemUsedBytes:   uint64(row.memUsedSum / row.samples),
		}
		if row.cpuSamples > 0 {
			value := round2(row.cpuSum / float64(row.cpuSamples))
			point.CPUPercent = &value
		}
		if row.diskSamples > 0 {
			point.DiskUsedPercent = round2(row.diskPctSum / float64(row.diskSamples))
			point.DiskUsedBytes = uint64(row.diskUsedSum / row.diskSamples)
		}
		if row.netSamples > 0 {
			rx := round2(row.rxSum / float64(row.netSamples))
			tx := round2(row.txSum / float64(row.netSamples))
			point.RxBps = &rx
			point.TxBps = &tx
		}
		out.Points = append(out.Points, point)
	}
	return out, nil
}

func foldHostMonths(rows []hostRow) []hostRow {
	var folded []hostRow
	index := map[string]int{}
	for _, row := range rows {
		at := time.Unix(row.start, 0).UTC()
		key := at.Format("2006-01")
		i, ok := index[key]
		if !ok {
			i = len(folded)
			index[key] = i
			folded = append(folded, hostRow{
				start: time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC).Unix(),
			})
		}
		folded[i].cpuSum += row.cpuSum
		folded[i].cpuSamples += row.cpuSamples
		folded[i].memPctSum += row.memPctSum
		folded[i].memUsedSum += row.memUsedSum
		folded[i].diskPctSum += row.diskPctSum
		folded[i].diskUsedSum += row.diskUsedSum
		folded[i].diskSamples += row.diskSamples
		folded[i].rxSum += row.rxSum
		folded[i].txSum += row.txSum
		folded[i].netSamples += row.netSamples
		folded[i].samples += row.samples
	}
	return folded
}

// Prune drops host buckets outside the fixed retention windows.
func (s *Store) Prune(ctx context.Context, now time.Time) error {
	now = now.UTC()
	cuts := []struct {
		res string
		age time.Duration
	}{
		{"15s", window.RetainFine},
		{"1m", window.RetainMinute},
		{"1h", window.RetainHour},
		{"1d", window.RetainDay},
	}
	for _, cut := range cuts {
		if _, err := s.db.ExecContext(ctx, `
			DELETE FROM host_metric_buckets WHERE resolution = ? AND bucket_start < ?
		`, cut.res, now.Add(-cut.age).Unix()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) maybePrune(ctx context.Context, now time.Time) {
	s.mu.Lock()
	if !s.lastPrune.IsZero() && now.Sub(s.lastPrune) < time.Minute {
		s.mu.Unlock()
		return
	}
	s.lastPrune = now
	s.mu.Unlock()
	_ = s.Prune(ctx, now)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
