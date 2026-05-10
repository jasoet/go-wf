package chunk

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

// heartbeatSleeperInterval is the inner-tick frequency for HeartbeatSleeper.
// 10s leaves comfortable margin under the typical 1-minute HeartbeatTimeout.
const heartbeatSleeperInterval = 10 * time.Second

// HeartbeatSleeper sleeps d while emitting Temporal activity heartbeats every
// 10s so the activity does not exceed its HeartbeatTimeout. Returns early
// when ctx is canceled.
//
// Use as the default Sleeper in RateLimitOpts. Panics if called outside an
// activity context (where activity.RecordHeartbeat is invalid).
func HeartbeatSleeper(ctx context.Context, d time.Duration) {
	deadline := time.Now().Add(d)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		step := heartbeatSleeperInterval
		if remaining < step {
			step = remaining
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(step):
			activity.RecordHeartbeat(ctx, "chunk sleep")
		}
	}
}
