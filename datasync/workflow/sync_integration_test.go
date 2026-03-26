//go:build integration

package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/payload"
	"github.com/jasoet/go-wf/workflow/testutil"
)

func TestIntegration_SyncWorkflow_EndToEnd(t *testing.T) {
	ctx := context.Background()

	tc, err := testutil.StartTemporalContainer(ctx)
	require.NoError(t, err)
	defer tc.Cleanup(ctx)

	c := tc.Client

	jobName := "integration-test-sync"
	source := &mockSource[string]{name: "src", records: []string{"a", "b", "c"}}
	sink := &mockSink[string]{name: "dst", result: datasync.WriteResult{Inserted: 3}}
	job := datasync.Job[string, string]{
		Name:   jobName,
		Source: source,
		Mapper: datasync.IdentityMapper[string](),
		Sink:   sink,
	}

	w := worker.New(c, TaskQueue(jobName), worker.Options{})
	RegisterJob(w, job)
	require.NoError(t, w.Start())
	defer w.Stop()

	input := payload.SyncExecutionInput{
		JobName:    jobName,
		SourceName: "src",
		SinkName:   "dst",
	}

	we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "test-" + jobName,
		TaskQueue: TaskQueue(jobName),
	}, jobName, input)
	require.NoError(t, err)

	var output payload.SyncExecutionOutput
	require.NoError(t, we.Get(ctx, &output))
	assert.True(t, output.Success)
	assert.Equal(t, 3, output.TotalFetched)
	assert.Equal(t, 3, output.Inserted)
}
