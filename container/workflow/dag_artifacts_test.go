package workflow

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/workflow/store"
)

func TestArtifactActivities_DirectRoundTrip(t *testing.T) {
	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	ctx := context.Background()
	src := filepath.Join(t.TempDir(), "report.txt")
	require.NoError(t, os.WriteFile(src, []byte("artifact-content"), 0o600))

	err = uploadArtifactActivity(ctx, raw, "wf/run/step/report.txt", src, "file")
	require.NoError(t, err)

	dest := filepath.Join(t.TempDir(), "downloaded.txt")
	err = downloadArtifactActivity(ctx, raw, "wf/run/step/report.txt", dest, "file")
	require.NoError(t, err)

	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "artifact-content", string(got))
}

func TestDownloadArtifactActivity_NotFound(t *testing.T) {
	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	err = downloadArtifactActivity(context.Background(), raw, "missing/key", filepath.Join(t.TempDir(), "x"), "file")
	require.Error(t, err)
}

// withContainerArtifactStore wraps DAGWorkflow to inject the artifact store
// after the test environment deserializes the input (ArtifactStore is json:"-").
func withContainerArtifactStore(raw store.RawStore) func(wf.Context, payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
	return func(ctx wf.Context, in payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
		in.ArtifactStore = raw
		return DAGWorkflow(ctx, in)
	}
}

// artifactActivityAdapters registers downloadArtifactActivity /
// uploadArtifactActivity under the names the DAG workflow uses, backed by raw.
// The store argument is accepted as `any` because a RawStore cannot cross the
// activity serialization boundary; the real store is captured in the closure.
type artifactActivityAdapters struct {
	uploadErr    error
	downloadErr  error
	uploadKeys   []string
	downloadKeys []string
}

func (a *artifactActivityAdapters) register(env *testsuite.TestWorkflowEnvironment, raw store.RawStore) {
	env.RegisterActivityWithOptions(
		func(_ context.Context, _ any, key, destPath, typ string) error {
			a.downloadKeys = append(a.downloadKeys, key)
			if a.downloadErr != nil {
				return a.downloadErr
			}
			return store.DownloadFile(context.Background(), raw, key, destPath, typ)
		},
		activity.RegisterOptions{Name: "downloadArtifactActivity"},
	)
	env.RegisterActivityWithOptions(
		func(_ context.Context, _ any, key, sourcePath, typ string) error {
			a.uploadKeys = append(a.uploadKeys, key)
			if a.uploadErr != nil {
				return a.uploadErr
			}
			return store.UploadFile(context.Background(), raw, key, sourcePath, typ)
		},
		activity.RegisterOptions{Name: "uploadArtifactActivity"},
	)
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

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	adapters := &artifactActivityAdapters{}
	adapters.register(env, raw)

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

	// The producer uploaded and the consumer downloaded the same store key.
	require.Len(t, adapters.uploadKeys, 1)
	require.Len(t, adapters.downloadKeys, 1)
	assert.Equal(t, adapters.uploadKeys[0], adapters.downloadKeys[0])
	assert.Contains(t, adapters.uploadKeys[0], "producer")
	assert.Contains(t, adapters.uploadKeys[0], "binary")

	// The file content traveled through the store end to end.
	got, err := os.ReadFile(dest)
	require.NoError(t, err)
	assert.Equal(t, "binary-bytes", string(got))
}

func TestDAGWorkflow_ArtifactDownloadFailure_NotOptional(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerContainerActivity(env)

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	adapters := &artifactActivityAdapters{downloadErr: errors.New("object not found")}
	adapters.register(env, raw)

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

	raw, err := store.NewLocalStore(t.TempDir())
	require.NoError(t, err)
	defer raw.Close() //nolint:errcheck // test cleanup

	adapters := &artifactActivityAdapters{downloadErr: errors.New("object not found")}
	adapters.register(env, raw)

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

	adapters := &artifactActivityAdapters{uploadErr: errors.New("store unavailable")}
	adapters.register(env, raw)

	env.OnActivity("StartContainerActivity", mock.Anything, mock.Anything).Return(
		&payload.ContainerExecutionOutput{Success: true, ExitCode: 0, Duration: time.Second}, nil)

	env.ExecuteWorkflow(withContainerArtifactStore(raw), dagWithArtifacts(
		filepath.Join(t.TempDir(), "binary"), filepath.Join(t.TempDir(), "binary"), true))

	require.True(t, env.IsWorkflowCompleted())
	// Upload failures are logged, not propagated — the node still succeeds.
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
}
