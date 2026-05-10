package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/payload"
)

type stubPartitioner struct {
	parts []Partition[int64]
}

func (s *stubPartitioner) Partitions(_ context.Context) ([]Partition[int64], error) {
	return s.parts, nil
}

func TestChunkedSync_Build_RequiresPartitioner(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresFetcher(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresMapper(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresSink(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Build()
}

func TestChunkedSync_Build_PopulatesRegistration(t *testing.T) {
	reg := NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Schedule(15 * time.Minute).
		Disabled(true).
		Build()
	assert.Equal(t, "job-x", reg.Name)
	assert.Equal(t, "sync-job-x", reg.TaskQueue)
	assert.Equal(t, 15*time.Minute, reg.Schedule)
	assert.True(t, reg.Disabled)
}

func TestChunkedSync_Workflow_NoTracker_AllPartitionsProcessed(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	// Register stub activities so the test environment recognizes the names.
	stubPartitionsAct := func(_ context.Context) ([]Partition[int64], error) { return nil, nil }
	stubRunPartitionAct := func(_ context.Context, _ runPartitionInput[int64]) (PartitionResult[int64], error) {
		return PartitionResult[int64]{}, nil
	}
	env.RegisterActivityWithOptions(stubPartitionsAct, sdkactivity.RegisterOptions{Name: "job-x.Partitions"})
	env.RegisterActivityWithOptions(stubRunPartitionAct, sdkactivity.RegisterOptions{Name: "job-x.RunPartition"})

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
