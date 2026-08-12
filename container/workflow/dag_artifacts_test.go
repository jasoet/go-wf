package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v2/container/payload"
	"github.com/jasoet/go-wf/v2/workflow/store"
)

// withContainerArtifactStore wraps DAGWorkflow to inject the artifact store
// after the test environment deserializes the input (ArtifactStore is json:"-").
func withContainerArtifactStore(raw store.RawStore) func(wf.Context, payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
	return func(ctx wf.Context, in payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
		in.ArtifactStore = raw
		return DAGWorkflow(ctx, in)
	}
}

func dagWithArtifacts(producerPath, consumerPath string, consumerOptional bool) payload.DAGWorkflowInput {
	return payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "producer",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"},
					OutputArtifacts: []payload.Artifact{
						{Name: "binary", Path: producerPath, Type: "file"},
					},
				},
			},
			{
				Name: "consumer",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{Image: "alpine:latest"},
					InputArtifacts: []payload.Artifact{
						{Name: "binary", Path: consumerPath, Type: "file", Optional: consumerOptional},
					},
				},
				Dependencies: []string{"producer"},
			},
		},
	}
}

func TestDAGWorkflow_Artifacts_EndToEnd(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	// Real store in a temp dir: artifact transfer runs as local activities
	// against this store, so bytes must actually travel through it.
	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	src := filepath.Join(t.TempDir(), "binary")
	require.NoError(t, os.WriteFile(src, []byte("binary-bytes"), 0o600))
	dest := filepath.Join(t.TempDir(), "binary")

	env.ExecuteWorkflow(withContainerArtifactStore(raw), dagWithArtifacts(src, dest, false))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)

	// The producer uploaded exactly one key under its step name.
	keys, err := raw.List(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	assert.Contains(t, keys[0], "producer")
	assert.Contains(t, keys[0], "binary")

	// The file content traveled through the store end to end.
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "binary-bytes", string(got))
}

func TestDAGWorkflow_ArtifactDownloadFailure_NotOptional(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	// Empty store — nothing was ever uploaded, so the download misses for real.
	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	env.ExecuteWorkflow(withContainerArtifactStore(raw), dagWithArtifacts(
		filepath.Join(t.TempDir(), "binary"), filepath.Join(t.TempDir(), "binary"), false))

	require.True(t, env.IsWorkflowCompleted())
	err = env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download artifact")
}

func TestDAGWorkflow_ArtifactDownloadFailure_Optional(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	// Empty store — the optional download misses and is skipped.
	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	env.ExecuteWorkflow(withContainerArtifactStore(raw), dagWithArtifacts(
		filepath.Join(t.TempDir(), "binary"), filepath.Join(t.TempDir(), "binary"), true))

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestDAGWorkflow_ArtifactUploadFailure_OnlyLogged(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	// The producer's source path does not exist, so the upload fails for real.
	env.ExecuteWorkflow(withContainerArtifactStore(raw), dagWithArtifacts(
		filepath.Join(t.TempDir(), "binary"), filepath.Join(t.TempDir(), "binary"), true))

	require.True(t, env.IsWorkflowCompleted())
	// Upload failures are logged, not propagated — the node still succeeds.
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)

	// Nothing reached the store.
	keys, err := raw.List(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, keys)
}
