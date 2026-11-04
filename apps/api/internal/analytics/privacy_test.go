package analytics

import (
	"net"
	"strings"
	"testing"
)

func TestTruncateIP(t *testing.T) {
	v4, ok := TruncateIP("203.0.113.55")
	if !ok || v4 != "203.0.113.0" {
		t.Fatalf("v4 = %q ok=%v", v4, ok)
	}
	v6, ok := TruncateIP("2001:db8:abcd:12::1")
	if !ok {
		t.Fatal("v6 not ok")
	}
	parsed := net.ParseIP(v6)
	if parsed == nil || parsed.To4() != nil {
		t.Fatalf("truncated v6 = %q", v6)
	}
	raw := parsed.To16()
	for i := 6; i < 16; i++ {
		if raw[i] != 0 {
			t.Fatalf("bit past /48 set in %q", v6)
		}
	}
	if !strings.HasPrefix(strings.ToLower(v6), "2001:db8:abcd:") {
		t.Fatalf("prefix lost: %q", v6)
	}
	mapped, ok := TruncateIP("::ffff:203.0.113.9")
	if !ok || mapped != "203.0.113.0" {
		t.Fatalf("mapped = %q ok=%v", mapped, ok)
	}
	for _, bad := range []string{"", "not-an-ip", "999.1.1.1", "203.0.113.55\n1.2.3.4"} {
		if _, ok := TruncateIP(bad); ok {
			t.Fatalf("accepted %q", bad)
		}
	}
}

func TestVisitorHashHidesAddress(t *testing.T) {
	day := mustDay(t, "2026-11-04")
	next := mustDay(t, "2026-11-05")
	a := VisitorHash("salt-a", day, "203.0.113.0")
	b := VisitorHash("salt-a", day, "203.0.113.0")
	c := VisitorHash("salt-a", next, "203.0.113.0")
	d := VisitorHash("salt-a", day, "198.51.100.0")
	e := VisitorHash("salt-b", day, "203.0.113.0")
	if a != b || len(a) != 16 {
		t.Fatalf("hash = %q", a)
	}
	if a == c || a == d || a == e {
		t.Fatalf("hashes did not change: %s %s %s %s", a, c, d, e)
	}
	if strings.Contains(a, "203") || strings.Contains(a, "113") {
		t.Fatalf("hash looks like the address: %s", a)
	}
	for _, ch := range a {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			t.Fatalf("non-hex hash %q", a)
		}
	}
}

func TestSanitizePathDropsQueryAndDotDot(t *testing.T) {
	got := SanitizePath("/api/../../etc/passwd?token=secret#frag")
	if got != "/api/etc/passwd" {
		t.Fatalf("path = %q", got)
	}
	if strings.Contains(got, "secret") || strings.Contains(got, "..") {
		t.Fatalf("path leaked: %q", got)
	}
	if SanitizePath("metrics") != "/metrics" {
		t.Fatal("relative path")
	}
	if SanitizePath("/ok") != "/ok" {
		t.Fatal("clean path changed")
	}
	if strings.Contains(SanitizePath("/a\nb"), "\n") {
		t.Fatal("newline kept")
	}
}

func TestSanitizeHostAndProject(t *testing.T) {
	if got := SanitizeHost("Example.COM:8080"); got != "example.com" {
		t.Fatalf("host = %q", got)
	}
	if SanitizeHost("bad host") != "other" || SanitizeHost("evil\r\n.com") != "other" {
		t.Fatal("invalid host accepted")
	}
	if SanitizeProject("Panel") != "panel" {
		t.Fatal("project case")
	}
	if SanitizeProject("../x") != "" || SanitizeProject("has space") != "" {
		t.Fatal("invalid project accepted")
	}
}

func TestDeviceType(t *testing.T) {
	if DeviceType("Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X)") != "mobile" {
		t.Fatal("iphone")
	}
	if DeviceType("Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X)") != "tablet" {
		t.Fatal("ipad")
	}
	if DeviceType("Mozilla/5.0 (compatible; Googlebot/2.1)") != "bot" {
		t.Fatal("bot")
	}
	if DeviceType("Mozilla/5.0 (X11; Linux x86_64)") != "desktop" {
		t.Fatal("desktop")
	}
	if DeviceType("") != "unknown" {
		t.Fatal("empty")
	}
}

func mustDay(t *testing.T, value string) string {
	t.Helper()
	if len(value) != 10 {
		t.Fatal(value)
	}
	return value
}
