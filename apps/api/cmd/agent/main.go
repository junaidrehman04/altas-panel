// Command agent prints one true-state host snapshot and exits.
// It is the small binary path for the host reader. The API uses the same
// package and keeps the previous sample so a poll can show rates.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/junaidrehman04/altas/apps/api/internal/metrics"
)

func main() {
	collector := metrics.NewCollector(os.Getenv("ALTAS_HOST_PROC"), os.Getenv("ALTAS_HOST_ROOT"))
	if _, err := collector.Snapshot(); err != nil {
		fmt.Fprintf(os.Stderr, "metrics: %v\n", err)
		os.Exit(1)
	}
	time.Sleep(200 * time.Millisecond)
	snap, err := collector.Snapshot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "metrics: %v\n", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(snap); err != nil {
		fmt.Fprintf(os.Stderr, "metrics: %v\n", err)
		os.Exit(1)
	}
}
