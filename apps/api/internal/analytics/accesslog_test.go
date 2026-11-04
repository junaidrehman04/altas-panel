package analytics

import (
	"strings"
	"testing"
	"time"
)

func TestParseCombinedAndRejectsGarbage(t *testing.T) {
	line := `203.0.113.10 - - [04/Nov/2026:09:08:07 +0000] "GET /app/index?token=secret HTTP/1.1" 500 120 "-" "curl/8.0"`
	ev, ok := ParseCombined(line)
	if !ok {
		t.Fatal("expected parse")
	}
	if ev.RemoteIP != "203.0.113.10" || ev.Status != 500 || ev.Bytes != 120 || ev.HasLatency {
		t.Fatalf("event = %+v", ev)
	}
	if strings.Contains(ev.Path, "secret") || ev.Path != "/app/index" {
		t.Fatalf("path = %q", ev.Path)
	}
	if ev.App != "proxy" || ev.Project != "proxy" {
		t.Fatalf("source = %+v", ev)
	}
	want := time.Date(2026, 11, 4, 9, 8, 7, 0, time.UTC)
	if !ev.At.Equal(want) {
		t.Fatalf("time = %s", ev.At)
	}
	if _, ok := ParseCombined("not a log line"); ok {
		t.Fatal("accepted garbage")
	}
	if _, ok := ParseCombined(strings.Repeat("A", 9000)); ok {
		t.Fatal("accepted oversized line")
	}
}

func TestSafeLogPath(t *testing.T) {
	if _, err := SafeLogPath(""); err == nil {
		t.Fatal("empty path enabled the tail")
	}
	if _, err := SafeLogPath("access.log"); err == nil {
		t.Fatal("relative path")
	}
	if _, err := SafeLogPath("/tmp/../etc/passwd"); err == nil {
		t.Fatal("dot-dot path")
	}
	if _, err := SafeLogPath("/no/such/altas-access.log"); err == nil {
		t.Fatal("missing file")
	}
}
