package window

import (
	"testing"
	"time"
)

func TestParseRanges(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 30, 45, 0, time.UTC)

	cases := []struct {
		in                 string
		hostRes            string
		analyticsRes       string
		visitors           string
		monthly            bool
		wantStart          time.Time
	}{
		{"live", "15s", "1m", "h", false, now.Add(-15 * time.Minute)},
		{" 24H ", "1m", "1m", "h", false, now.Add(-24 * time.Hour)},
		{"7d", "1h", "1h", "d", false, now.Add(-7 * 24 * time.Hour)},
		{"30d", "1h", "1h", "d", false, now.Add(-30 * 24 * time.Hour)},
		{"month", "1d", "1d", "d", true, time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
		{"months", "1d", "1d", "d", true, time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			span, err := Parse(tc.in, now)
			if err != nil {
				t.Fatal(err)
			}
			if span.HostResolution != tc.hostRes || span.AnalyticsResolution != tc.analyticsRes {
				t.Fatalf("resolutions = %s %s", span.HostResolution, span.AnalyticsResolution)
			}
			if span.VisitorScope != tc.visitors || span.Monthly != tc.monthly {
				t.Fatalf("span = %+v", span)
			}
			if !span.Start.Equal(tc.wantStart) || !span.End.Equal(now) {
				t.Fatalf("window %s – %s, want %s – %s", span.Start, span.End, tc.wantStart, now)
			}
			if span.Name == "" {
				t.Fatal("empty name")
			}
		})
	}
}

func TestParseRejectsUnknownRange(t *testing.T) {
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{"", "1h", "year", "live;drop", "../etc", "month\n"} {
		if _, err := Parse(in, now); err == nil {
			t.Fatalf("accepted %q", in)
		}
	}
}
