package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/activity"
	"github.com/jasoet/go-wf/datasync/payload"
)

// stubSyncDataActivity is a no-op activity stub for workflow test registration.
func stubSyncDataActivity(_ context.Context, _ activity.ActivityInput) (*activity.ActivityOutput, error) {
	return nil, nil
}

type mockSource[T any] struct {
	name    string
	records []T
	err     error
}

func (m *mockSource[T]) Name() string                         { return m.name }
func (m *mockSource[T]) Fetch(_ context.Context) ([]T, error) { return m.records, m.err }

type mockSink[U any] struct {
	name   string
	result datasync.WriteResult
	err    error
}

func (m *mockSink[U]) Name() string { return m.name }
func (m *mockSink[U]) Write(_ context.Context, _ []U) (datasync.WriteResult, error) {
	return m.result, m.err
}

func TestTaskQueue(t *testing.T) {
	assert.Equal(t, "sync-attendee-sync", TaskQueue("attendee-sync"))
}

func TestBuildWorkflowInput(t *testing.T) {
	source := &mockSource[int]{name: "test-source"}
	sink := &mockSink[int]{name: "test-sink"}
	job := datasync.Job[int, int]{
		Name:     "test-job",
		Source:   source,
		Mapper:   datasync.IdentityMapper[int](),
		Sink:     sink,
		Metadata: "test-meta",
	}

	input := BuildWorkflowInput(job)
	assert.Equal(t, "test-job", input.JobName)
	assert.Equal(t, "test-source", input.SourceName)
	assert.Equal(t, "test-sink", input.SinkName)
	assert.Equal(t, "test-meta", input.Metadata)
}

func TestBuildJobRegistration(t *testing.T) {
	source := &mockSource[int]{name: "src"}
	sink := &mockSink[int]{name: "dst"}
	job := datasync.Job[int, int]{
		Name:     "test-job",
		Source:   source,
		Mapper:   datasync.IdentityMapper[int](),
		Sink:     sink,
		Schedule: 5 * time.Minute,
	}

	reg := BuildJobRegistration(job, false)
	assert.Equal(t, "test-job", reg.Name)
	assert.Equal(t, "sync-test-job", reg.TaskQueue)
	assert.Equal(t, 5*time.Minute, reg.Schedule)
	assert.False(t, reg.Disabled)
	assert.NotNil(t, reg.Register)
}

func TestSyncWorkflow_Success(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	jobName := "test-job"
	env.RegisterActivityWithOptions(stubSyncDataActivity, sdkactivity.RegisterOptions{Name: jobName + ".SyncData"})

	activityOutput := activity.ActivityOutput{
		TotalFetched: 10,
		Inserted:     5,
		Updated:      3,
		Skipped:      2,
	}

	env.OnActivity(jobName+".SyncData", mock.Anything, mock.Anything).
		Return(&activityOutput, nil)

	source := &mockSource[int]{name: "src"}
	sink := &mockSink[int]{name: "dst"}
	job := datasync.Job[int, int]{
		Name:   jobName,
		Source: source,
		Mapper: datasync.IdentityMapper[int](),
		Sink:   sink,
	}

	actInput := BuildActivityInput(job)
	wf := newSyncWorkflow(job, actInput)

	input := payload.SyncExecutionInput{JobName: jobName, SourceName: "src", SinkName: "dst"}
	env.ExecuteWorkflow(wf, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var output payload.SyncExecutionOutput
	require.NoError(t, env.GetWorkflowResult(&output))
	assert.True(t, output.Success)
	assert.Equal(t, 10, output.TotalFetched)
	assert.Equal(t, 5, output.Inserted)
	assert.Equal(t, 3, output.Updated)
	assert.Equal(t, 2, output.Skipped)
}

func TestSyncWorkflow_ActivityError(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	jobName := "test-job"
	env.RegisterActivityWithOptions(stubSyncDataActivity, sdkactivity.RegisterOptions{Name: jobName + ".SyncData"})

	env.OnActivity(jobName+".SyncData", mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	source := &mockSource[int]{name: "src"}
	sink := &mockSink[int]{name: "dst"}
	job := datasync.Job[int, int]{
		Name:   jobName,
		Source: source,
		Mapper: datasync.IdentityMapper[int](),
		Sink:   sink,
	}

	actInput := BuildActivityInput(job)
	wf := newSyncWorkflow(job, actInput)

	input := payload.SyncExecutionInput{JobName: jobName, SourceName: "src", SinkName: "dst"}
	env.ExecuteWorkflow(wf, input)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
