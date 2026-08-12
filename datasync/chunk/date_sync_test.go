package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/v2/datasync"
)

func TestDateChunkedSync_Build_ConfiguresDatePartitioner(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err)

	def, buildErr := NewDateChunkedSync[string, string]("date-job").
		LookBack(48 * time.Hour).
		ChunkSize(24 * time.Hour).
		Timezone(loc).
		Fetcher(func(_ context.Context, _, _ time.Time) ([]string, error) {
			return []string{"x"}, nil
		}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		ScheduleEvery(15 * time.Minute).
		Build()

	require.NoError(t, buildErr)
	assert.Equal(t, "date-job", def.Name)
	assert.Equal(t, "sync-date-job", def.TaskQueue)
	require.NotNil(t, def.Schedule)
	assert.Equal(t, 15*time.Minute, def.Schedule.Interval)
}

func TestDateChunkedSync_Fetcher_ConvertsKeysToTime(t *testing.T) {
	got := struct {
		start, end time.Time
	}{}
	d := NewDateChunkedSync[string, string]("date-job").
		LookBack(time.Hour).
		ChunkSize(time.Hour).
		Fetcher(func(_ context.Context, s, e time.Time) ([]string, error) {
			got.start = s
			got.end = e
			return nil, nil
		}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"})

	// Drive the inner fetcher directly with int64 keys.
	startKey := TimeToKey(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	endKey := TimeToKey(time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC))
	_, err := d.inner.fetcher(context.Background(), startKey, endKey)
	require.NoError(t, err)
	assert.True(t, got.start.Equal(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)))
	assert.True(t, got.end.Equal(time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC)))
}

func TestDateChunkedSync_Tracker_ConvertsCursor(t *testing.T) {
	timeCursor := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	advancedTimes := []time.Time{}

	tracker := stubTimeTracker{
		cursor: timeCursor,
		exists: true,
		advance: func(t time.Time) error {
			advancedTimes = append(advancedTimes, t)
			return nil
		},
	}

	d := NewDateChunkedSync[string, string]("date-job").
		LookBack(time.Hour).
		ChunkSize(time.Hour).
		Fetcher(func(_ context.Context, _, _ time.Time) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		WithTracker(tracker)

	require.NotNil(t, d.inner.tracker, "tracker adapter should be installed")

	// Read cursor via inner adapter.
	gotKey, exists, err := d.inner.tracker.Cursor(context.Background(), "date-job")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, TimeToKey(timeCursor), gotKey)

	// Advance via inner adapter.
	advanceTime := time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC)
	require.NoError(t, d.inner.tracker.Advance(context.Background(), "date-job", TimeToKey(advanceTime)))
	require.Len(t, advancedTimes, 1)
	assert.True(t, advanceTime.Equal(advancedTimes[0]))
}

type stubTimeTracker struct {
	cursor  time.Time
	exists  bool
	advance func(time.Time) error
}

func (s stubTimeTracker) Cursor(_ context.Context, _ string) (time.Time, bool, error) {
	return s.cursor, s.exists, nil
}

func (s stubTimeTracker) Advance(_ context.Context, _ string, t time.Time) error {
	if s.advance != nil {
		return s.advance(t)
	}
	return nil
}
