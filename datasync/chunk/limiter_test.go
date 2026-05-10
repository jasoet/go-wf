package chunk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRateLimit = errors.New("rate limit hit")

func rateLimitDetector(err error) bool {
	return errors.Is(err, errRateLimit)
}

func TestRateLimitRetry_FirstCallSucceeds(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return []string{"ok"}, nil
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector: rateLimitDetector,
		Sleeper:  func(_ context.Context, _ time.Duration) {},
	})
	got, err := wrapped(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, got)
	assert.Equal(t, 1, calls)
}

func TestRateLimitRetry_RetriesOnDetectedError(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		if calls < 3 {
			return nil, errRateLimit
		}
		return []string{"ok"}, nil
	}
	sleeps := 0
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 5,
		Sleep:       1 * time.Millisecond,
		Sleeper: func(_ context.Context, _ time.Duration) {
			sleeps++
		},
	})
	got, err := wrapped(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, got)
	assert.Equal(t, 3, calls)
	assert.Equal(t, 2, sleeps, "sleep called between attempts but not after success")
}

func TestRateLimitRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return nil, errRateLimit
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 3,
		Sleeper:     func(_ context.Context, _ time.Duration) {},
	})
	_, err := wrapped(context.Background(), 0, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errRateLimit))
	assert.Equal(t, 3, calls)
}

func TestRateLimitRetry_NonRateLimitErrorReturnsImmediately(t *testing.T) {
	other := errors.New("other failure")
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return nil, other
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 5,
		Sleeper:     func(_ context.Context, _ time.Duration) {},
	})
	_, err := wrapped(context.Background(), 0, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, other))
	assert.Equal(t, 1, calls)
}

func TestRateLimitRetry_ContextCancelStopsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		return nil, errRateLimit
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 5,
		Sleeper: func(_ context.Context, _ time.Duration) {
			cancel()
		},
	})
	_, err := wrapped(ctx, 0, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRateLimitRetry_NilDetectorTreatsAllErrorsAsNonRateLimit(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return nil, errRateLimit
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    nil,
		MaxAttempts: 5,
		Sleeper:     func(_ context.Context, _ time.Duration) {},
	})
	_, err := wrapped(context.Background(), 0, 10)
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}
