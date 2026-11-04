package window

import (
	"errors"
	"strings"
	"time"
)

// ErrBadRange is returned when a client asks for a window this package does not serve.
var ErrBadRange = errors.New("bad range")

// Retention limits for rollup buckets. Finer buckets are deleted first.
// A month view reads daily buckets, so those stay for a little over a year.
const (
	RetainFine   = 2 * time.Hour
	RetainMinute = 48 * time.Hour
	RetainHour   = 40 * 24 * time.Hour
	RetainDay    = 400 * 24 * time.Hour
)

// Span is one supported chart window.
// HostResolution and AnalyticsResolution are the bucket sizes to read.
// Monthly reports fold daily buckets into calendar months.
type Span struct {
	Name                string
	Start               time.Time
	End                 time.Time
	HostResolution      string
	AnalyticsResolution string
	VisitorScope        string
	Monthly             bool
}

// Parse accepts live, 24h, 7d, 30d, and month (or months).
// now is the inclusive end of the window. The caller supplies it so tests stay fixed.
func Parse(name string, now time.Time) (Span, error) {
	if strings.ContainsAny(name, "\r\n\x00") {
		return Span{}, ErrBadRange
	}
	now = now.UTC()
	span := Span{End: now}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "live":
		span.Name = "live"
		span.Start = now.Add(-15 * time.Minute)
		span.HostResolution = "15s"
		span.AnalyticsResolution = "1m"
		span.VisitorScope = "h"
	case "24h":
		span.Name = "24h"
		span.Start = now.Add(-24 * time.Hour)
		span.HostResolution = "1m"
		span.AnalyticsResolution = "1m"
		span.VisitorScope = "h"
	case "7d":
		span.Name = "7d"
		span.Start = now.Add(-7 * 24 * time.Hour)
		span.HostResolution = "1h"
		span.AnalyticsResolution = "1h"
		span.VisitorScope = "d"
	case "30d":
		span.Name = "30d"
		span.Start = now.Add(-30 * 24 * time.Hour)
		span.HostResolution = "1h"
		span.AnalyticsResolution = "1h"
		span.VisitorScope = "d"
	case "month", "months":
		span.Name = "month"
		span.Start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -11, 0)
		span.HostResolution = "1d"
		span.AnalyticsResolution = "1d"
		span.VisitorScope = "d"
		span.Monthly = true
	default:
		return Span{}, ErrBadRange
	}
	return span, nil
}

// BucketStart truncates t to the UTC start of a 15s, 1m, 1h, or 1d bucket.
func BucketStart(t time.Time, resolution string) int64 {
	t = t.UTC()
	switch resolution {
	case "15s":
		unix := t.Unix()
		return unix - unix%15
	case "1m":
		unix := t.Unix()
		return unix - unix%60
	case "1h":
		unix := t.Unix()
		return unix - unix%3600
	case "1d":
		year, month, day := t.Date()
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix()
	default:
		return t.Unix()
	}
}
