//go:build !linux

package metrics

import "testing"

func TestFallbackSnapshot(t *testing.T) {
	snap, err := NewCollector("", "").Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snap.Source != SourceFallback || snap.CPU.Cores < 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if snap.Note == "" {
		t.Fatal("fallback note is empty")
	}
}
