package chunk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/pkg/v2/temporal/job"
	"go.temporal.io/sdk/temporal"
)

func TestChunkedSync_OptionSetters(t *testing.T) {
	retry := temporal.RetryPolicy{MaximumAttempts: 7}
	rateLimit := RateLimitOpts{MaxAttempts: 4}
	rawSpec := &job.ScheduleSpec{Cron: "@hourly"}

	c := NewChunkedSync[int, int, int64]("opts").
		PartitionSleep(2*time.Second).
		ScheduleCron("@daily").
		RateLimitRetry(rateLimit).
		ActivityRetry(retry).
		ActivityTimeouts(time.Minute, 10*time.Second).
		MaxPartitionsPerExecution(25).
		Disabled(true)

	assert.Equal(t, 2*time.Second, c.partitionSleep)
	assert.Equal(t, "@daily", c.schedule.Cron)
	require.NotNil(t, c.rateLimitOpts)
	assert.Equal(t, 4, c.rateLimitOpts.MaxAttempts)
	assert.Equal(t, int32(7), c.activityRetry.MaximumAttempts)
	assert.Equal(t, time.Minute, c.startToClose)
	assert.Equal(t, 10*time.Second, c.heartbeat)
	assert.Equal(t, 25, c.maxPerExec)
	assert.True(t, c.disabled)

	c.ScheduleRaw(rawSpec)
	assert.Same(t, rawSpec, c.schedule)
}

func TestDateChunkedSync_OptionSetters(t *testing.T) {
	retry := temporal.RetryPolicy{MaximumAttempts: 2}
	rateLimit := RateLimitOpts{MaxAttempts: 5}
	rawSpec := &job.ScheduleSpec{Interval: time.Hour}

	d := NewDateChunkedSync[int, int]("date-opts").
		PartitionSleep(time.Second).
		ScheduleCron("@weekly").
		RateLimitRetry(rateLimit).
		ActivityRetry(retry).
		ActivityTimeouts(2*time.Minute, 30*time.Second).
		MaxPartitionsPerExecution(10).
		Disabled(true)

	assert.Equal(t, time.Second, d.inner.partitionSleep)
	assert.Equal(t, "@weekly", d.inner.schedule.Cron)
	assert.Equal(t, 5, d.inner.rateLimitOpts.MaxAttempts)
	assert.Equal(t, int32(2), d.inner.activityRetry.MaximumAttempts)
	assert.Equal(t, 2*time.Minute, d.inner.startToClose)
	assert.Equal(t, 30*time.Second, d.inner.heartbeat)
	assert.Equal(t, 10, d.inner.maxPerExec)
	assert.True(t, d.inner.disabled)

	d.ScheduleRaw(rawSpec)
	assert.Same(t, rawSpec, d.inner.schedule)
}
