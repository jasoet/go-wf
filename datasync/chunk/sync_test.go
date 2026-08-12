package chunk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v2/datasync"
	"github.com/jasoet/go-wf/v2/datasync/payload"
)

type stubPartitioner struct {
	parts []Partition[int64]
}

func (s *stubPartitioner) Partitions(_ context.Context) ([]Partition[int64], error) {
	return s.parts, nil
}

func TestChunkedSync_Build_RequiresPartitioner(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_, _ = NewChunkedSync[string, string, int64]("job-x").
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresFetcher(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_, _ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresMapper(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_, _ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresSink(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_, _ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Build()
}

func TestChunkedSync_Build_PopulatesRegistration(t *testing.T) {
	def, err := NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		ScheduleEvery(15 * time.Minute).
		Disabled(true).
		Build()
	require.NoError(t, err)
	assert.Equal(t, "job-x", def.Name)
	assert.Equal(t, "sync-job-x", def.TaskQueue)
	require.NotNil(t, def.Schedule)
	assert.Equal(t, 15*time.Minute, def.Schedule.Interval)
	assert.True(t, def.Schedule.Paused)
}

func TestChunkedSync_Workflow_NoTracker_AllPartitionsProcessed(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			return PartitionResult[int64]{
				Start:    in.Partition.Start,
				End:      in.Partition.End,
				Fetched:  3,
				Inserted: 3,
			}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                  "job-x",
		partitionsActivityName:   "job-x.Partitions",
		runPartitionActivityName: "job-x.RunPartition",
		partitionActivityOptions: workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:    workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})

	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result SyncResult[int64]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalPartitions)
	assert.Equal(t, 6, result.TotalFetched)
	assert.Equal(t, 6, result.TotalInserted)
	assert.Len(t, result.Partitions, 2)
}

func TestChunkedSync_Workflow_Tracker_NoCursor_ProcessesAll(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResult[int64]{Cursor: 0, Exists: false}, nil)
	advanced := []int64{}
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).
		Return(func(_ context.Context, c int64) error {
			advanced = append(advanced, c)
			return nil
		})
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End, Fetched: 1, Inserted: 1}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	assert.Equal(t, []int64{10, 20}, advanced)
}

func TestChunkedSync_Workflow_Tracker_MidRangeCursor_FiltersBefore(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
	}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResult[int64]{Cursor: 10, Exists: true}, nil)
	processed := []int64{}
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			processed = append(processed, in.Partition.Start)
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
		})
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).Return(nil)

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Partitions starting at 0 (< cursor 10) are skipped; 10 and 20 are processed.
	assert.Equal(t, []int64{10, 20}, processed)
}

func TestChunkedSync_Workflow_Tracker_CursorPastEnd_ProcessesNothing(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResult[int64]{Cursor: 100, Exists: true}, nil)

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result SyncResult[int64]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalPartitions)
}

func TestChunkedSync_Workflow_Tracker_PartitionFails_CursorNotAdvanced(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResult[int64]{Cursor: 0, Exists: false}, nil)
	advanced := []int64{}
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).
		Return(func(_ context.Context, c int64) error {
			advanced = append(advanced, c)
			return nil
		})
	calls := 0
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			calls++
			if calls == 1 {
				return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
			}
			return PartitionResult[int64]{}, errors.New("boom")
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())

	// Cursor advanced for the first (successful) partition only.
	assert.Equal(t, []int64{10}, advanced)
}

func TestChunkedSync_Workflow_MaxPerExecution_TriggersContinueAsNew(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
		{Start: 30, End: 40},
	}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResult[int64]{Cursor: 0, Exists: false}, nil)
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).Return(nil)
	processed := []int64{}
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			processed = append(processed, in.Partition.Start)
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
		maxPerExec:                2,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())

	// ContinueAsNew is signaled by the workflow returning a *workflow.ContinueAsNewError;
	// the test environment surfaces it as the workflow error.
	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.True(t, workflow.IsContinueAsNewError(err), "expected ContinueAsNewError, got %T: %v", err, err)

	// Only the first 2 partitions processed in this execution.
	assert.Equal(t, []int64{0, 10}, processed)
}

func TestChunkedSync_Build_RequiresTrackerWhenMaxPerExecSet(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_, _ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		MaxPartitionsPerExecution(50).
		Build()
}

func TestChunkedSync_Workflow_PartitionSleep_DelaysBetweenPartitions(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()
	registerStubActivities(env)

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                  "job-x",
		partitionsActivityName:   "job-x.Partitions",
		runPartitionActivityName: "job-x.RunPartition",
		partitionActivityOptions: workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:    workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionSleep:           5 * time.Second,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})

	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result SyncResult[int64]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalPartitions)
}

// registerStubActivities registers stubs for the four activities referenced by
// chunkedSyncWorkflow.run, satisfying Temporal's test environment requirement
// that activities be registered before OnActivity mocks them.
func registerStubActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivityWithOptions(
		func(_ context.Context) ([]Partition[int64], error) { return nil, nil },
		activity.RegisterOptions{Name: "job-x.Partitions"})
	env.RegisterActivityWithOptions(
		func(_ context.Context, _ runPartitionInput[int64]) (PartitionResult[int64], error) {
			return PartitionResult[int64]{}, nil
		},
		activity.RegisterOptions{Name: "job-x.RunPartition"})
	env.RegisterActivityWithOptions(
		func(_ context.Context, _ string) (cursorResult[int64], error) {
			return cursorResult[int64]{}, nil
		},
		activity.RegisterOptions{Name: "job-x.ReadCursor"})
	env.RegisterActivityWithOptions(
		func(_ context.Context, _ int64) error { return nil },
		activity.RegisterOptions{Name: "job-x.AdvanceCursor"})
}
