package analytics

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/db"
	"github.com/junaidrehman04/altas/apps/api/internal/window"
)

func TestRollupQueryAndEmptySummary(t *testing.T) {
	ctx := context.Background()
	rec := newTestRecorder(t, FakeLocator{})
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	rec.now = func() time.Time { return now }

	span, err := window.Parse("24h", now)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := rec.Summary(ctx, span, "")
	if err != nil {
		t.Fatal(err)
	}
	if empty.Totals.Requests != 0 || empty.Totals.Visitors != 0 || empty.Totals.Bytes != 0 || empty.Totals.P95MS != nil {
		t.Fatalf("empty totals = %+v", empty.Totals)
	}
	if empty.Series == nil || empty.TopHosts == nil || empty.TopPaths == nil || empty.Countries == nil || empty.Devices == nil {
		t.Fatalf("nil slices in %+v", empty)
	}
	if len(empty.Series) != 0 || len(empty.Countries) != 0 {
		t.Fatalf("empty summary has rows: %+v", empty)
	}

	base := Event{
		At:         now.Add(-time.Minute),
		Project:    "panel",
		Host:       "panel.example",
		App:        "panel",
		Path:       "/api/metrics/host?token=secret",
		Status:     200,
		Bytes:      100,
		Latency:    10 * time.Millisecond,
		HasLatency: true,
		RemoteIP:   "203.0.113.55",
		UserAgent:  "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)",
	}
	for i := 0; i < 94; i++ {
		ev := base
		if err := rec.Record(ctx, ev); err != nil {
			t.Fatal(err)
		}
	}
	slow := base
	slow.Status = 500
	slow.Latency = 1000 * time.Millisecond
	slow.Bytes = 50
	slow.Path = "/api/stacks"
	for i := 0; i < 6; i++ {
		if err := rec.Record(ctx, slow); err != nil {
			t.Fatal(err)
		}
	}
	other := base
	other.Project = "blog"
	other.RemoteIP = "198.51.100.20"
	other.UserAgent = "Mozilla/5.0 (X11; Linux x86_64)"
	other.HasLatency = false
	if err := rec.Record(ctx, other); err != nil {
		t.Fatal(err)
	}

	sum, err := rec.Summary(ctx, span, "")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Totals.Requests != 101 || sum.Totals.Errors != 6 || sum.Totals.Bytes != 94*100+6*50+100 {
		t.Fatalf("totals = %+v", sum.Totals)
	}
	if sum.Totals.Visitors != 2 {
		t.Fatalf("visitors = %d", sum.Totals.Visitors)
	}
	if sum.Totals.ErrorRate < 0.059 || sum.Totals.ErrorRate > 0.060 {
		t.Fatalf("error rate = %v", sum.Totals.ErrorRate)
	}
	if sum.Totals.P95MS == nil || *sum.Totals.P95MS != 1000 {
		t.Fatalf("p95 = %v", sum.Totals.P95MS)
	}
	if len(sum.TopPaths) < 2 || sum.TopPaths[0].Name != "/api/metrics/host" {
		t.Fatalf("paths = %+v", sum.TopPaths)
	}
	for _, path := range sum.TopPaths {
		if strings.Contains(path.Name, "secret") || strings.Contains(path.Name, "?") {
			t.Fatalf("query stored in path %q", path.Name)
		}
	}

	filtered, err := rec.Summary(ctx, span, "blog")
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Totals.Requests != 1 || filtered.Totals.Visitors != 1 {
		t.Fatalf("filtered = %+v", filtered.Totals)
	}

	assertNoAddress(t, rec.db, "203.0.113.55")
	assertNoAddress(t, rec.db, "203.0.113.0")
	assertNoAddress(t, rec.db, "198.51.100.20")
}

func TestGeoStaysEmptyWithoutDatabase(t *testing.T) {
	ctx := context.Background()
	rec := newTestRecorder(t, disabledLocator{})
	now := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	// Prune follows this clock. The default wall clock deletes minute buckets after 48 hours.
	rec.now = func() time.Time { return now }
	err := rec.Record(ctx, Event{
		At: now, Project: "panel", Host: "h.example", App: "panel", Path: "/",
		Status: 200, Bytes: 10, Latency: 5 * time.Millisecond, HasLatency: true,
		RemoteIP: "203.0.113.8", UserAgent: "Mozilla/5.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	span, err := window.Parse("live", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := rec.Summary(ctx, span, "")
	if err != nil {
		t.Fatal(err)
	}
	if sum.GeoConfigured || len(sum.Countries) != 0 || len(sum.Cities) != 0 {
		t.Fatalf("geo invented: %+v countries=%+v cities=%+v", sum.GeoConfigured, sum.Countries, sum.Cities)
	}
	if len(sum.Devices) != 1 || sum.Devices[0].Name != "desktop" {
		t.Fatalf("devices = %+v", sum.Devices)
	}
}

func TestGeoUsesLocatorAfterTruncation(t *testing.T) {
	ctx := context.Background()
	rec := newTestRecorder(t, FakeLocator{Country: "US", City: "Austin"})
	now := time.Date(2026, 6, 2, 8, 0, 0, 0, time.UTC)
	rec.now = func() time.Time { return now }
	seen := ""
	rec.locate = func(truncated string) (string, string) {
		seen = truncated
		return "US", "Austin"
	}
	if err := rec.Record(ctx, Event{
		At: now, Project: "panel", Host: "h.example", App: "panel", Path: "/a",
		Status: 404, Bytes: 4, HasLatency: true, Latency: 25 * time.Millisecond,
		RemoteIP: "203.0.113.55", UserAgent: "Mozilla/5.0",
	}); err != nil {
		t.Fatal(err)
	}
	if seen != "203.0.113.0" {
		t.Fatalf("lookup saw %q", seen)
	}
	span, err := window.Parse("7d", now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := rec.Summary(ctx, span, "panel")
	if err != nil {
		t.Fatal(err)
	}
	if !sum.GeoConfigured || len(sum.Countries) != 1 || sum.Countries[0].Name != "US" || sum.Countries[0].Requests != 1 {
		t.Fatalf("countries = %+v configured=%v", sum.Countries, sum.GeoConfigured)
	}
	if len(sum.Cities) != 1 || sum.Cities[0].Name != "Austin" || sum.Cities[0].Country != "US" {
		t.Fatalf("cities = %+v", sum.Cities)
	}
	if sum.Totals.Errors != 0 {
		t.Fatalf("404 counted as error: %+v", sum.Totals)
	}
}

func TestMonthRollupGroupsDays(t *testing.T) {
	ctx := context.Background()
	rec := newTestRecorder(t, disabledLocator{})
	nov := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	oct := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{nov, nov.Add(24 * time.Hour), oct} {
		if err := rec.Record(ctx, Event{
			At: at, Project: "panel", App: "panel", Host: "h.example", Path: "/a",
			Status: 200, Bytes: 8, HasLatency: true, Latency: 5 * time.Millisecond,
			RemoteIP: "203.0.113.4", UserAgent: "Mozilla/5.0",
		}); err != nil {
			t.Fatal(err)
		}
	}
	span, err := window.Parse("month", time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	sum, err := rec.Summary(ctx, span, "")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Resolution != "1mo" || sum.Totals.Requests != 3 || len(sum.Series) != 2 {
		t.Fatalf("month summary res=%s totals=%+v series=%d", sum.Resolution, sum.Totals, len(sum.Series))
	}
	if sum.Series[0].Requests != 1 || sum.Series[1].Requests != 2 {
		t.Fatalf("series = %+v", sum.Series)
	}
}

func TestPruneDropsOldAnalyticsBuckets(t *testing.T) {
	ctx := context.Background()
	rec := newTestRecorder(t, disabledLocator{})
	old := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := rec.Record(ctx, Event{
		At: old, Project: "panel", App: "panel", Host: "h.example", Path: "/old",
		Status: 200, Bytes: 1, RemoteIP: "203.0.113.4", UserAgent: "Mozilla/5.0",
	}); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	if err := rec.Prune(ctx, now); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := rec.db.QueryRow(`SELECT COUNT(*) FROM analytics_buckets`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("old buckets kept: %d", n)
	}
}

func newTestRecorder(t *testing.T, loc Locator) *Recorder {
	t.Helper()
	sqlDB := openAnalyticsDB(t)
	rec := NewRecorder(sqlDB, loc)
	rec.now = time.Now
	return rec
}

func openAnalyticsDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "altas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func assertNoAddress(t *testing.T, sqlDB *sql.DB, addr string) {
	t.Helper()
	tables := []struct {
		query string
		col   string
	}{
		{`SELECT name FROM analytics_dims`, "name"},
		{`SELECT visitor_hash FROM analytics_visitors`, "visitor_hash"},
		{`SELECT project FROM analytics_buckets`, "project"},
	}
	for _, table := range tables {
		rows, err := sqlDB.Query(table.query)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				_ = rows.Close()
				t.Fatal(err)
			}
			if strings.Contains(value, addr) {
				_ = rows.Close()
				t.Fatalf("%s contains %s (%s)", table.col, addr, value)
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			t.Fatal(err)
		}
		_ = rows.Close()
	}
}
