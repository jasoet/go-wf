package workflow

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v2/function/payload"
	"github.com/jasoet/go-wf/v2/workflow/store"
)

// withArtifactStore wraps DAGWorkflow to inject the artifact store after the
// test environment deserializes the input (ArtifactStore is json:"-").
func withArtifactStore(raw store.RawStore) func(wf.Context, payload.DAGWorkflowInput) (*payload.FunctionDAGWorkflowOutput, error) {
	return func(ctx wf.Context, in payload.DAGWorkflowInput) (*payload.FunctionDAGWorkflowOutput, error) {
		in.ArtifactStore = raw
		return DAGWorkflow(ctx, in)
	}
}

func TestDAGWorkflow_ArtifactBytesRoundTrip(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	outputData := []byte("producer-artifact-bytes")

	var consumerData []byte
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			in, ok := args.Get(1).(payload.FunctionExecutionInput)
			require.True(t, ok)
			if in.Name == "consumer-func" {
				consumerData = in.Data
			}
		}).
		Return(&payload.FunctionExecutionOutput{
			Name:     "ok",
			Success:  true,
			Data:     outputData,
			Duration: time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "producer",
				Function: payload.FunctionExecutionInput{Name: "producer-func"},
				OutputArtifacts: []payload.ArtifactRef{
					{Name: "shared-data", Type: "bytes"},
				},
			},
			{
				Name:         "consumer",
				Function:     payload.FunctionExecutionInput{Name: "consumer-func"},
				Dependencies: []string{"producer"},
				InputArtifacts: []payload.ArtifactRef{
					{Name: "shared-data", Type: "bytes"},
				},
			},
		},
	}

	env.ExecuteWorkflow(withArtifactStore(raw), input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)

	// The consumer must have received the producer's bytes via the store.
	assert.Equal(t, outputData, consumerData)
}

func TestDAGWorkflow_ArtifactFileUpload(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Name: "ok", Success: true, Duration: time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "producer",
				Function: payload.FunctionExecutionInput{Name: "producer-func"},
				OutputArtifacts: []payload.ArtifactRef{
					// A directory path that exists in the test environment.
					{Name: "report", Type: "file", Path: "dag.go"},
				},
			},
		},
	}

	env.ExecuteWorkflow(withArtifactStore(raw), input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// The file artifact must have been uploaded under the producer step.
	keys, err := raw.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Contains(t, keys[0], "producer")
	assert.Contains(t, keys[0], "report")
}

func TestDAGWorkflow_ArtifactDownloadFailure_NotOptional(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	// Empty store — the required artifact does not exist.
	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Name: "ok", Success: true, Duration: time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "consumer",
				Function: payload.FunctionExecutionInput{Name: "consumer-func"},
				InputArtifacts: []payload.ArtifactRef{
					{Name: "missing-data", Type: "bytes", Optional: false},
				},
			},
		},
	}

	env.ExecuteWorkflow(withArtifactStore(raw), input)

	require.True(t, env.IsWorkflowCompleted())
	err = env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download artifact")
}

func TestDAGWorkflow_ArtifactUploadFailure_NotOptional(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Name: "ok", Success: true, Duration: time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.FunctionDAGNode{
			{
				Name:     "producer",
				Function: payload.FunctionExecutionInput{Name: "producer-func"},
				OutputArtifacts: []payload.ArtifactRef{
					// The path does not exist; upload fails but is only logged.
					{Name: "report", Type: "file", Path: "does/not/exist.txt", Optional: false},
				},
			},
		},
	}

	env.ExecuteWorkflow(withArtifactStore(raw), input)

	require.True(t, env.IsWorkflowCompleted())
	// Upload failures are logged, not propagated — the node still succeeds.
	require.NoError(t, env.GetWorkflowError())

	var result payload.FunctionDAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
}
