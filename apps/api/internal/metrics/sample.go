package metrics

import (
	"context"
	"log"
	"time"
)

// Sample reads the host on a fixed interval and stores rollups.
// every outside 5s–5m is replaced with 15s. The loop stops when ctx is cancelled.
func Sample(ctx context.Context, collector *Collector, store *Store, every time.Duration, logger *log.Logger) {
	if every < 5*time.Second || every > 5*time.Minute {
		every = 15 * time.Second
	}
	record := func() {
		if collector == nil || store == nil {
			return
		}
		snap, err := collector.Snapshot()
		if err != nil {
			if logger != nil {
				logger.Printf("metrics sample: %v", err)
			}
			return
		}
		if err := store.RecordSnapshot(ctx, snap); err != nil && logger != nil {
			logger.Printf("metrics rollup: %v", err)
		}
	}
	record()
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			record()
		}
	}
}
