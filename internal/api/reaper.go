package api

import (
	"context"
	"time"
)

// StartNodeReaper runs a periodic loop to mark stale nodes inactive.
func (a *API) StartNodeReaper(ctx context.Context) {
	interval := a.cfg.Scheduler.ReaperInterval
	timeout := a.cfg.Scheduler.HeartbeatTimeout

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-timeout)
				count, err := a.store.MarkStaleNodes(ctx, cutoff)
				if err != nil {
					a.logger.Error("node reaper failed", "error", err)
					continue
				}
				if count > 0 {
					a.logger.Warn("nodes marked stale", "count", count, "cutoff", cutoff)
				}
			}
		}
	}()
}
