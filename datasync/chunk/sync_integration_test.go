//go:build integration

package chunk

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/workflow/testutil"
)

// Order is a domain type used in chunk integration tests.
type Order struct {
	ID    int
	Value string
}

// orderSink is an in-memory Sink[Order] that records written records.
type orderSink struct {
	mu      sync.Mutex
	written []Order
}

func (s *orderSink) Name() string { return "order-sink" }

func (s *orderSink) Write(_ context.Context, records []Order) (datasync.WriteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.written = append(s.written, records...)
	return datasync.WriteResult{Inserted: len(records)}, nil
}

func (s *orderSink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.written)
}

// inMemoryTracker satisfies TimeProgressTracker using a mutex-guarded map.
type inMemoryTracker struct {
	mu      sync.Mutex
	cursors map[string]time.Time
}

func newInMemoryTracker() *inMemoryTracker {
	return &inMemoryTracker{cursors: make(map[string]time.Time)}
}

func (t *inMemoryTracker) Cursor(_ context.Context, jobName string) (time.Time, bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	ts, ok := t.cursors[jobName]
	if !ok {
		return time.Time{}, false, nil
	}
	return ts, true, nil
}

func (t *inMemoryTracker) Advance(_ context.Context, jobName string, completed time.Time) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.cursors[jobName] = completed
	return nil
}

func TestIntegration_ChunkedSync_EndToEnd_NoTracker(t *testing.T) {
	ctx := context.Background()

	// Compute the expected partition count from the same DatePartitioner logic
	// used by the workflow so the assertion is correct at any time of day.
	const lookBack = 72 * time.Hour
	const chunkSize = 24 * time.Hour
	expectedParts, err := (&DatePartitioner{
		Loc:       time.UTC,
		LookBack:  lookBack,
		ChunkSize: chunkSize,
	}).Partitions(ctx)
	require.NoError(t, err)
	expectedCount := len(expectedParts)
	const recordsPerPartition = 2

	tc, err := testutil.StartTemporalContainer(ctx)
	require.NoError(t, err)

	sink := &orderSink{}

	jobName := "integration-test-chunk-no-tracker"

	reg := NewDateChunkedSync[Order, Order](jobName).
		LookBack(lookBack).
		ChunkSize(chunkSize).
		Timezone(time.UTC).
		Fetcher(func(_ context.Context, _, _ time.Time) ([]Order, error) {
			return []Order{
				{ID: 1, Value: "order-1"},
				{ID: 2, Value: "order-2"},
			}, nil
		}).
		Mapper(datasync.IdentityMapper[Order]()).
		Sink(sink).
		Build()

	w := worker.New(tc.Client, reg.TaskQueue, worker.Options{})
	reg.Register(w)
	require.NoError(t, w.Start())
	defer w.Stop()
	defer tc.Cleanup(ctx)

	we, err := tc.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "test-" + reg.Name,
		TaskQueue: reg.TaskQueue,
	}, reg.Name, reg.WorkflowInput)
	require.NoError(t, err)

	var result SyncResult[int64]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, expectedCount, result.TotalPartitions, "partition count should match DatePartitioner output")
	assert.Equal(t, expectedCount*recordsPerPartition, result.TotalFetched, "total fetched = partitions × records per partition")
	assert.Equal(t, expectedCount*recordsPerPartition, result.TotalInserted, "total inserted = partitions × records per partition")
	assert.Len(t, result.Partitions, expectedCount, "Partitions slice length should match partition count")
}

func TestIntegration_ChunkedSync_EndToEnd_WithTracker(t *testing.T) {
	ctx := context.Background()

	// Compute the expected partition count from the same DatePartitioner logic
	// used by the workflow so the assertion is correct at any time of day.
	const lookBack = 72 * time.Hour
	const chunkSize = 24 * time.Hour
	expectedParts, err := (&DatePartitioner{
		Loc:       time.UTC,
		LookBack:  lookBack,
		ChunkSize: chunkSize,
	}).Partitions(ctx)
	require.NoError(t, err)
	expectedCount := len(expectedParts)
	const recordsPerPartition = 2

	tc, err := testutil.StartTemporalContainer(ctx)
	require.NoError(t, err)

	sink := &orderSink{}
	tracker := newInMemoryTracker()

	jobName := "integration-test-chunk-with-tracker"

	reg := NewDateChunkedSync[Order, Order](jobName).
		LookBack(lookBack).
		ChunkSize(chunkSize).
		Timezone(time.UTC).
		Fetcher(func(_ context.Context, _, _ time.Time) ([]Order, error) {
			return []Order{
				{ID: 1, Value: "order-1"},
				{ID: 2, Value: "order-2"},
			}, nil
		}).
		Mapper(datasync.IdentityMapper[Order]()).
		Sink(sink).
		WithTracker(tracker).
		Build()

	w := worker.New(tc.Client, reg.TaskQueue, worker.Options{})
	reg.Register(w)
	require.NoError(t, w.Start())
	defer w.Stop()
	defer tc.Cleanup(ctx)

	we, err := tc.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "test-" + reg.Name,
		TaskQueue: reg.TaskQueue,
	}, reg.Name, reg.WorkflowInput)
	require.NoError(t, err)

	var result SyncResult[int64]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, expectedCount, result.TotalPartitions, "partition count should match DatePartitioner output")
	assert.Equal(t, expectedCount*recordsPerPartition, result.TotalFetched, "total fetched = partitions × records per partition")
	assert.Equal(t, expectedCount*recordsPerPartition, result.TotalInserted, "total inserted = partitions × records per partition")
	assert.Len(t, result.Partitions, expectedCount, "Partitions slice length should match partition count")

	// Verify that the tracker cursor was advanced to the end of the last partition.
	cursorTime, exists, err := tracker.Cursor(ctx, jobName)
	require.NoError(t, err)
	assert.True(t, exists, "tracker should have a cursor after workflow completion")

	// The cursor should correspond to the End key of the last partition.
	if assert.True(t, len(result.Partitions) > 0) {
		lastPartition := result.Partitions[len(result.Partitions)-1]
		expectedCursorTime := KeyToTime(lastPartition.End)
		assert.True(t, cursorTime.Equal(expectedCursorTime),
			"tracker cursor %v should equal last partition end %v", cursorTime, expectedCursorTime)
	}
}
