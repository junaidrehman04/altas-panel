package analytics

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/window"
)

// Event is one request observation. RemoteIP is truncated before it is stored.
// HasLatency is false for sources that do not know how long the request took.
type Event struct {
	At         time.Time
	Project    string
	Host       string
	App        string
	Path       string
	Status     int
	Bytes      int64
	Latency    time.Duration
	HasLatency bool
	RemoteIP   string
	UserAgent  string
}

// Totals are exact sums for a window, not a sum of downsampled chart points.
type Totals struct {
	Requests  int64    `json:"requests"`
	Visitors  int64    `json:"visitors"`
	Bytes     int64    `json:"bytes"`
	Errors    int64    `json:"errors"`
	ErrorRate float64  `json:"error_rate"`
	P95MS     *float64 `json:"p95_ms"`
}

// SeriesPoint is one chart bucket.
type SeriesPoint struct {
	T         string   `json:"t"`
	Requests  int64    `json:"requests"`
	Errors    int64    `json:"errors"`
	Bytes     int64    `json:"bytes"`
	ErrorRate float64  `json:"error_rate"`
	P95MS     *float64 `json:"p95_ms"`
}

// Top is a ranked label such as a host, application, path, or device.
type Top struct {
	Name     string `json:"name"`
	Requests int64  `json:"requests"`
	Errors   int64  `json:"errors"`
	Bytes    int64  `json:"bytes"`
}

// Place is a country or city count. Country is set on city rows.
type Place struct {
	Name     string `json:"name"`
	Country  string `json:"country,omitempty"`
	Requests int64  `json:"requests"`
}

// Summary is the analytics panel payload.
type Summary struct {
	Range         string        `json:"range"`
	Resolution    string        `json:"resolution"`
	From          string        `json:"from"`
	To            string        `json:"to"`
	Project       string        `json:"project"`
	GeoConfigured bool          `json:"geo_configured"`
	Totals        Totals        `json:"totals"`
	Series        []SeriesPoint `json:"series"`
	TopHosts      []Top         `json:"top_hosts"`
	TopApps       []Top         `json:"top_apps"`
	TopPaths      []Top         `json:"top_paths"`
	Countries     []Place       `json:"countries"`
	Cities        []Place       `json:"cities"`
	Devices       []Top         `json:"devices"`
}

// Recorder writes request rollups into SQLite.
type Recorder struct {
	db         *sql.DB
	loc        Locator
	now        func() time.Time
	locate     func(truncated string) (country, city string)
	mu         sync.Mutex
	cachedSalt string
	lastPrune  time.Time
}

var bucketUpper = [12]float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 10000}

var histNames = func() string {
	parts := make([]string, 12)
	for i := range parts {
		parts[i] = fmt.Sprintf("b%d", i)
	}
	return strings.Join(parts, ", ")
}()

var histSums = func() string {
	parts := make([]string, 12)
	for i := range parts {
		parts[i] = fmt.Sprintf("SUM(b%d)", i)
	}
	return strings.Join(parts, ", ")
}()

var histUpdates = func() string {
	parts := make([]string, 12)
	for i := range parts {
		parts[i] = fmt.Sprintf("b%d = b%d + excluded.b%d", i, i, i)
	}
	return strings.Join(parts, ", ")
}()

// NewRecorder stores rollups in db. A nil locator leaves geo empty.
func NewRecorder(db *sql.DB, loc Locator) *Recorder {
	if loc == nil {
		loc = disabledLocator{}
	}
	return &Recorder{db: db, loc: loc, now: time.Now}
}

// Record folds one event into the minute, hour, and day buckets.
// The raw client address is not written. A daily hash is written instead.
func (r *Recorder) Record(ctx context.Context, ev Event) error {
	if ev.At.IsZero() {
		ev.At = r.now().UTC()
	} else {
		ev.At = ev.At.UTC()
	}
	if ev.Status == 0 {
		ev.Status = 200
	}
	if ev.Status < 100 || ev.Status > 599 {
		return nil
	}
	if ev.Bytes < 0 {
		ev.Bytes = 0
	}
	project := SanitizeProject(ev.Project)
	host := SanitizeHost(ev.Host)
	app := SanitizeProject(ev.App)
	if app == "" {
		app = "other"
	}
	path := SanitizePath(ev.Path)
	device := DeviceType(ev.UserAgent)

	var country, city string
	var visitor string
	if truncated, ok := TruncateIP(ev.RemoteIP); ok {
		if r.locate != nil {
			country, city = r.locate(truncated)
		} else if r.loc != nil && r.loc.Enabled() {
			country, city = r.loc.Lookup(truncated)
		}
		country = sanitizeCountry(country)
		city = sanitizeCity(city)
		if country == "" {
			city = ""
		}
		salt, err := r.salt(ctx)
		if err != nil {
			return err
		}
		visitor = VisitorHash(salt, ev.At.Format("2006-01-02"), truncated)
	}

	errorN := int64(0)
	if ev.Status >= 500 {
		errorN = 1
	}
	var hist [12]int64
	if ev.HasLatency {
		hist[latencyBucket(ev.Latency)] = 1
	}

	dayBucket := window.BucketStart(ev.At, "1d")
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	path, err = r.limitLabel(ctx, tx, dayBucket, project, "path", path, 200)
	if err != nil {
		return err
	}
	if host != "" {
		host, err = r.limitLabel(ctx, tx, dayBucket, project, "host", host, 100)
		if err != nil {
			return err
		}
	}
	app, err = r.limitLabel(ctx, tx, dayBucket, project, "app", app, 100)
	if err != nil {
		return err
	}

	dims := []struct{ kind, name string }{
		{"app", app},
		{"path", path},
		{"device", device},
	}
	if host != "" {
		dims = append(dims, struct{ kind, name string }{"host", host})
	}
	if country != "" {
		dims = append(dims, struct{ kind, name string }{"country", country})
		if city != "" {
			dims = append(dims, struct{ kind, name string }{"city", country + "|" + city})
		}
	}

	for _, resolution := range []string{"1m", "1h", "1d"} {
		bucket := window.BucketStart(ev.At, resolution)
		if err := upsertBucket(ctx, tx, resolution, bucket, project, errorN, ev.Bytes, hist); err != nil {
			return err
		}
		for _, dim := range dims {
			if err := upsertDim(ctx, tx, resolution, bucket, project, dim.kind, dim.name, errorN, ev.Bytes); err != nil {
				return err
			}
		}
	}
	if visitor != "" {
		for _, scope := range []struct {
			name string
			res  string
		}{{"h", "1h"}, {"d", "1d"}} {
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO analytics_visitors (scope, bucket_start, project, visitor_hash)
				VALUES (?, ?, ?, ?)
			`, scope.name, window.BucketStart(ev.At, scope.res), project, visitor); err != nil {
				return err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.maybePrune(ctx, r.now().UTC())
	return nil
}

func upsertBucket(ctx context.Context, tx *sql.Tx, resolution string, bucket int64, project string, errorsN, bytes int64, hist [12]int64) error {
	args := []any{resolution, bucket, project, int64(1), errorsN, bytes}
	for _, n := range hist {
		args = append(args, n)
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO analytics_buckets (
			resolution, bucket_start, project, requests, errors, bytes, `+histNames+`
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(resolution, bucket_start, project) DO UPDATE SET
			requests = requests + excluded.requests,
			errors = errors + excluded.errors,
			bytes = bytes + excluded.bytes,
			`+histUpdates, args...)
	return err
}

func upsertDim(ctx context.Context, tx *sql.Tx, resolution string, bucket int64, project, kind, name string, errorsN, bytes int64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO analytics_dims (
			resolution, bucket_start, project, kind, name, requests, errors, bytes
		) VALUES (?, ?, ?, ?, ?, 1, ?, ?)
		ON CONFLICT(resolution, bucket_start, project, kind, name) DO UPDATE SET
			requests = requests + excluded.requests,
			errors = errors + excluded.errors,
			bytes = bytes + excluded.bytes
	`, resolution, bucket, project, kind, name, errorsN, bytes)
	return err
}

func (r *Recorder) limitLabel(ctx context.Context, tx *sql.Tx, bucket int64, project, kind, name string, limit int) (string, error) {
	if name == "" || name == "other" {
		return name, nil
	}
	var existing int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM analytics_dims
		WHERE resolution = '1d' AND bucket_start = ? AND project = ? AND kind = ? AND name = ?
	`, bucket, project, kind, name).Scan(&existing); err != nil {
		return "", err
	}
	if existing > 0 {
		return name, nil
	}
	var count int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM analytics_dims
		WHERE resolution = '1d' AND bucket_start = ? AND project = ? AND kind = ?
	`, bucket, project, kind).Scan(&count); err != nil {
		return "", err
	}
	if count >= limit {
		return "other", nil
	}
	return name, nil
}

// Summary reads rollups for span. project is empty for every project.
func (r *Recorder) Summary(ctx context.Context, span window.Span, project string) (Summary, error) {
	project = SanitizeProject(project)
	resolution := span.AnalyticsResolution
	out := Summary{
		Range:         span.Name,
		Resolution:    resolution,
		From:          span.Start.UTC().Format(time.RFC3339),
		To:            span.End.UTC().Format(time.RFC3339),
		Project:       project,
		GeoConfigured: r.loc != nil && r.loc.Enabled(),
		Series:        []SeriesPoint{},
		TopHosts:      []Top{},
		TopApps:       []Top{},
		TopPaths:      []Top{},
		Countries:     []Place{},
		Cities:        []Place{},
		Devices:       []Top{},
	}
	if span.Monthly {
		out.Resolution = "1mo"
	}
	rows, err := r.buckets(ctx, resolution, span, project)
	if err != nil {
		return Summary{}, err
	}
	if span.Monthly {
		rows = foldMonths(rows)
	}
	var totals bucketRow
	for _, row := range rows {
		totals.requests += row.requests
		totals.errors += row.errors
		totals.bytes += row.bytes
		for i := range totals.hist {
			totals.hist[i] += row.hist[i]
		}
		out.Series = append(out.Series, SeriesPoint{
			T:         time.Unix(row.start, 0).UTC().Format(time.RFC3339),
			Requests:  row.requests,
			Errors:    row.errors,
			Bytes:     row.bytes,
			ErrorRate: errorRate(row.errors, row.requests),
			P95MS:     percentile(row.hist, 0.95),
		})
	}
	out.Totals = Totals{
		Requests:  totals.requests,
		Bytes:     totals.bytes,
		Errors:    totals.errors,
		ErrorRate: errorRate(totals.errors, totals.requests),
		P95MS:     percentile(totals.hist, 0.95),
	}
	visitors, err := r.visitors(ctx, span, project)
	if err != nil {
		return Summary{}, err
	}
	out.Totals.Visitors = visitors
	out.TopHosts, err = r.tops(ctx, resolution, span, project, "host")
	if err != nil {
		return Summary{}, err
	}
	out.TopApps, err = r.tops(ctx, resolution, span, project, "app")
	if err != nil {
		return Summary{}, err
	}
	out.TopPaths, err = r.tops(ctx, resolution, span, project, "path")
	if err != nil {
		return Summary{}, err
	}
	out.Devices, err = r.tops(ctx, resolution, span, project, "device")
	if err != nil {
		return Summary{}, err
	}
	out.Countries, err = r.places(ctx, resolution, span, project, "country")
	if err != nil {
		return Summary{}, err
	}
	out.Cities, err = r.places(ctx, resolution, span, project, "city")
	if err != nil {
		return Summary{}, err
	}
	return out, nil
}

type bucketRow struct {
	start    int64
	requests int64
	errors   int64
	bytes    int64
	hist     [12]int64
}

func (r *Recorder) buckets(ctx context.Context, resolution string, span window.Span, project string) ([]bucketRow, error) {
	query := `
		SELECT bucket_start, SUM(requests), SUM(errors), SUM(bytes), ` + histSums + `
		FROM analytics_buckets
		WHERE resolution = ? AND bucket_start >= ? AND bucket_start <= ?
		  AND (? = '' OR project = ?)
		GROUP BY bucket_start
		ORDER BY bucket_start
	`
	rows, err := r.db.QueryContext(ctx, query, resolution, span.Start.Unix(), span.End.Unix(), project, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []bucketRow
	for rows.Next() {
		var row bucketRow
		dest := []any{&row.start, &row.requests, &row.errors, &row.bytes}
		for i := range row.hist {
			dest = append(dest, &row.hist[i])
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func foldMonths(rows []bucketRow) []bucketRow {
	var folded []bucketRow
	index := map[string]int{}
	for _, row := range rows {
		at := time.Unix(row.start, 0).UTC()
		key := at.Format("2006-01")
		i, ok := index[key]
		if !ok {
			i = len(folded)
			index[key] = i
			start := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC).Unix()
			folded = append(folded, bucketRow{start: start})
		}
		folded[i].requests += row.requests
		folded[i].errors += row.errors
		folded[i].bytes += row.bytes
		for b := range folded[i].hist {
			folded[i].hist[b] += row.hist[b]
		}
	}
	return folded
}

func (r *Recorder) visitors(ctx context.Context, span window.Span, project string) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT visitor_hash) FROM analytics_visitors
		WHERE scope = ? AND bucket_start >= ? AND bucket_start <= ?
		  AND (? = '' OR project = ?)
	`, span.VisitorScope, span.Start.Unix(), span.End.Unix(), project, project).Scan(&n)
	return n, err
}

func (r *Recorder) tops(ctx context.Context, resolution string, span window.Span, project, kind string) ([]Top, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name, SUM(requests), SUM(errors), SUM(bytes)
		FROM analytics_dims
		WHERE resolution = ? AND bucket_start >= ? AND bucket_start <= ? AND kind = ?
		  AND (? = '' OR project = ?)
		GROUP BY name
		ORDER BY SUM(requests) DESC, name ASC
		LIMIT 8
	`, resolution, span.Start.Unix(), span.End.Unix(), kind, project, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Top{}
	for rows.Next() {
		var item Top
		if err := rows.Scan(&item.Name, &item.Requests, &item.Errors, &item.Bytes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Recorder) places(ctx context.Context, resolution string, span window.Span, project, kind string) ([]Place, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name, SUM(requests)
		FROM analytics_dims
		WHERE resolution = ? AND bucket_start >= ? AND bucket_start <= ? AND kind = ?
		  AND (? = '' OR project = ?)
		GROUP BY name
		ORDER BY SUM(requests) DESC, name ASC
		LIMIT 8
	`, resolution, span.Start.Unix(), span.End.Unix(), kind, project, project)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Place{}
	for rows.Next() {
		var name string
		var requests int64
		if err := rows.Scan(&name, &requests); err != nil {
			return nil, err
		}
		item := Place{Name: name, Requests: requests}
		if kind == "city" {
			country, city, ok := strings.Cut(name, "|")
			if ok {
				item.Country = country
				item.Name = city
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// Prune deletes buckets older than the fixed retention windows.
func (r *Recorder) Prune(ctx context.Context, now time.Time) error {
	now = now.UTC()
	cuts := []struct {
		res string
		age time.Duration
	}{
		{"1m", window.RetainMinute},
		{"1h", window.RetainHour},
		{"1d", window.RetainDay},
	}
	for _, cut := range cuts {
		before := now.Add(-cut.age).Unix()
		if _, err := r.db.ExecContext(ctx, `DELETE FROM analytics_buckets WHERE resolution = ? AND bucket_start < ?`, cut.res, before); err != nil {
			return err
		}
		if _, err := r.db.ExecContext(ctx, `DELETE FROM analytics_dims WHERE resolution = ? AND bucket_start < ?`, cut.res, before); err != nil {
			return err
		}
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM analytics_visitors WHERE scope = 'h' AND bucket_start < ?`, now.Add(-window.RetainMinute).Unix()); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM analytics_visitors WHERE scope = 'd' AND bucket_start < ?`, now.Add(-window.RetainDay).Unix())
	return err
}

func (r *Recorder) maybePrune(ctx context.Context, now time.Time) {
	r.mu.Lock()
	if !r.lastPrune.IsZero() && now.Sub(r.lastPrune) < time.Minute {
		r.mu.Unlock()
		return
	}
	r.lastPrune = now
	r.mu.Unlock()
	_ = r.Prune(ctx, now)
}

func (r *Recorder) salt(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cachedSalt != "" {
		return r.cachedSalt, nil
	}
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM analytics_meta WHERE key = 'visitor_salt'`).Scan(&value)
	if err == sql.ErrNoRows {
		buf := make([]byte, 16)
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		value = hex.EncodeToString(buf)
		if _, err := r.db.ExecContext(ctx, `INSERT INTO analytics_meta (key, value) VALUES ('visitor_salt', ?)`, value); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	r.cachedSalt = value
	return value, nil
}

func latencyBucket(d time.Duration) int {
	ms := d.Milliseconds()
	if ms < 0 {
		ms = 0
	}
	edges := [11]int64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}
	for i, edge := range edges {
		if ms <= edge {
			return i
		}
	}
	return 11
}

func percentile(counts [12]int64, p float64) *float64 {
	var total int64
	for _, count := range counts {
		total += count
	}
	if total == 0 {
		return nil
	}
	target := int64(math.Ceil(p * float64(total)))
	if target < 1 {
		target = 1
	}
	var acc int64
	for i, count := range counts {
		acc += count
		if acc >= target {
			value := bucketUpper[i]
			return &value
		}
	}
	return nil
}

func errorRate(errorsN, requests int64) float64 {
	if requests <= 0 {
		return 0
	}
	return float64(errorsN) / float64(requests)
}

func sanitizeCountry(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if len(raw) != 2 {
		return ""
	}
	for _, c := range raw {
		if c < 'A' || c > 'Z' {
			return ""
		}
	}
	return raw
}

func sanitizeCity(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\x00|") {
		return ""
	}
	if len([]rune(raw)) > 80 {
		raw = string([]rune(raw)[:80])
	}
	return raw
}
