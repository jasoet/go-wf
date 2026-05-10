// Package heartbeat provides shared Temporal-activity heartbeat helpers used by
// datasync/activity and datasync/chunk. Living under internal/ keeps these
// helpers off the public API while still allowing reuse across sibling
// packages within datasync.
package heartbeat

import (
	"context"
	"sync/atomic"
	"time"

	"go.temporal.io/sdk/activity"
)

// Interval derives the periodic heartbeat tick from the configured
// HeartbeatTimeout. Returns max(1s, hbTimeout/3); falls back to 10s when
// hbTimeout is 0 (no timeout configured).
func Interval(hbTimeout time.Duration) time.Duration {
	if hbTimeout == 0 {
		return 10 * time.Second
	}
	interval := hbTimeout / 3
	if interval < 1*time.Second {
		return 1 * time.Second
	}
	return interval
}

// Loop ticks at the given interval until done is closed or ctx is canceled,
// calling activity.RecordHeartbeat with the message returned by message().
// message is invoked once per tick, so callers may compute payloads lazily
// (e.g., reading an atomic.Pointer for the current phase).
func Loop(
	ctx context.Context,
	interval time.Duration,
	message func() string,
	done <-chan struct{},
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			activity.RecordHeartbeat(ctx, message())
		}
	}
}

// PhaseMessage returns a message function suitable for Loop that formats a
// heartbeat as "<prefix>: <phase>" where phase is read atomically. If the
// phase pointer has never been set, "starting" is used.
func PhaseMessage(prefix string, phase *atomic.Pointer[string]) func() string {
	return func() string {
		p := "starting"
		if ptr := phase.Load(); ptr != nil {
			p = *ptr
		}
		return prefix + ": " + p
	}
}
