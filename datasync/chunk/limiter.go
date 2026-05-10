package chunk

import (
	"cmp"
	"context"
	"time"
)

// RateLimitDetector returns true if err indicates a rate-limit response from
// the upstream API. Different APIs use different signaling — implementers
// supply this function for their specific API.
type RateLimitDetector func(err error) bool

// RateLimitOpts configures the WithRateLimitRetry decorator.
type RateLimitOpts struct {
	// Detector identifies rate-limit errors. If nil, the decorator treats
	// every error as non-rate-limit (no retries) — useful for callers that
	// want a no-op default and provide a real detector at runtime.
	Detector RateLimitDetector

	// MaxAttempts is the total number of calls (sleeps = MaxAttempts - 1).
	// If zero, defaults to 3.
	MaxAttempts int

	// Sleep is the duration between attempts. If zero, defaults to 60s.
	Sleep time.Duration

	// Sleeper performs the inter-attempt sleep. If nil, defaults to
	// HeartbeatSleeper which emits Temporal heartbeats during the sleep.
	// Tests typically pass a recording fake.
	Sleeper func(ctx context.Context, d time.Duration)
}

const (
	defaultMaxAttempts = 3
	defaultSleep       = 60 * time.Second
)

// WithRateLimitRetry decorates a PartitionFetcher with retry on detected
// rate-limit errors. The decorator calls inner up to opts.MaxAttempts times,
// sleeping opts.Sleep between attempts, only when the previous attempt
// returned an error matched by opts.Detector. Other errors return
// immediately without retry.
//
// Returns ctx.Err() if ctx is canceled during a sleep.
func WithRateLimitRetry[T any, K cmp.Ordered](
	inner PartitionFetcher[T, K],
	opts RateLimitOpts,
) PartitionFetcher[T, K] {
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	sleep := opts.Sleep
	if sleep <= 0 {
		sleep = defaultSleep
	}
	sleeper := opts.Sleeper
	if sleeper == nil {
		sleeper = HeartbeatSleeper
	}
	detector := opts.Detector

	return func(ctx context.Context, start, end K) ([]T, error) {
		var lastErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			records, err := inner(ctx, start, end)
			if err == nil {
				return records, nil
			}
			lastErr = err
			isRateLimit := detector != nil && detector(err)
			if !isRateLimit {
				return nil, err
			}
			if attempt < maxAttempts {
				sleeper(ctx, sleep)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
			}
		}
		return nil, lastErr
	}
}
