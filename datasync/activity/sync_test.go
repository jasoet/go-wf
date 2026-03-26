package activity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/datasync"
)

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

func TestActivities_SyncData_Success(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", records: []string{"a", "b", "c"}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst", result: datasync.WriteResult{Inserted: 3}}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	result, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	var output ActivityOutput
	require.NoError(t, result.Get(&output))
	assert.Equal(t, 3, output.TotalFetched)
	assert.Equal(t, 3, output.Inserted)
}

func TestActivities_SyncData_EmptySource(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", records: []string{}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst"}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	result, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	var output ActivityOutput
	require.NoError(t, result.Get(&output))
	assert.Equal(t, 0, output.TotalFetched)
}

func TestActivities_SyncData_FetchError(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", err: fmt.Errorf("api down")}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst"}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source src fetch failed")
}

func TestActivities_SyncData_WriteError(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", records: []string{"a"}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst", err: fmt.Errorf("db down")}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sink dst write failed")
}

func TestToSyncExecutionOutput_Success(t *testing.T) {
	ao := &ActivityOutput{TotalFetched: 10, Inserted: 5, Updated: 3, Skipped: 2}
	output := ToSyncExecutionOutput("test-job", ao, 100, nil)
	assert.True(t, output.Success)
	assert.Equal(t, "test-job", output.JobName)
	assert.Equal(t, 10, output.TotalFetched)
	assert.Equal(t, 5, output.Inserted)
}

func TestToSyncExecutionOutput_Error(t *testing.T) {
	output := ToSyncExecutionOutput("test-job", nil, 0, fmt.Errorf("something failed"))
	assert.False(t, output.Success)
	assert.Equal(t, "something failed", output.Error)
}
