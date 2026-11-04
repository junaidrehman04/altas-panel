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
