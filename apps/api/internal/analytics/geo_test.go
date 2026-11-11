package analytics

import "testing"

func TestOpenLocatorStaysDisabledWithoutADatabase(t *testing.T) {
	if OpenLocator("", nil).Enabled() {
		t.Fatal("empty path enabled geo")
	}
	if OpenLocator("GeoLite2-City.mmdb", nil).Enabled() {
		t.Fatal("relative path enabled geo")
	}
	if OpenLocator("/tmp/../GeoLite2-City.mmdb", nil).Enabled() {
		t.Fatal("dot-dot path enabled geo")
	}
}

func TestReloaderKeepsThePreviousReaderWhenTheNewFileDoesNotOpen(t *testing.T) {
	r := NewReloader("", nil)
	if r.Enabled() {
		t.Fatal("empty path enabled geo")
	}
	if err := r.Reload(t.TempDir() + "/missing.mmdb"); err == nil {
		t.Fatal("missing file reloaded")
	}
	if r.Enabled() {
		t.Fatal("failed reload enabled geo")
	}
	country, city := r.Lookup("203.0.113.8")
	if country != "" || city != "" {
		t.Fatalf("lookup invented a place: %s %s", country, city)
	}
}
