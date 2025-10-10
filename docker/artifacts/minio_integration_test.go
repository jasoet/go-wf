//go:build integration

package artifacts

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// setupMinioContainer starts a Minio container using testcontainers.
func setupMinioContainer(ctx context.Context, t *testing.T) (testcontainers.Container, MinioConfig) {
	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "9000")
	require.NoError(t, err)

	config := MinioConfig{
		Endpoint:  host + ":" + port.Port(),
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test-artifacts",
		Prefix:    "workflows/",
		UseSSL:    false,
		Region:    "us-east-1",
	}

	return container, config
}

func TestMinioStore_UploadDownload(t *testing.T) {
	ctx := context.Background()

	// Start Minio container
	container, config := setupMinioContainer(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Create Minio store
	store, err := NewMinioStore(ctx, config)
	require.NoError(t, err)
	defer store.Close()

	// Upload artifact
	metadata := ArtifactMetadata{
		Name:       "test-file",
		Path:       "/test/file.txt",
		Type:       "file",
		WorkflowID: "workflow-123",
		RunID:      "run-456",
		StepName:   "build",
	}

	content := []byte("test content for minio")
	err = store.Upload(ctx, metadata, bytes.NewReader(content))
	require.NoError(t, err)

	// Verify artifact exists
	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.True(t, exists)

	// Download artifact
	reader, err := store.Download(ctx, metadata)
	require.NoError(t, err)
	defer reader.Close()

	// Verify content
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, content, buf.Bytes())
}

func TestMinioStore_Delete(t *testing.T) {
	ctx := context.Background()

	// Start Minio container
	container, config := setupMinioContainer(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Create Minio store
	store, err := NewMinioStore(ctx, config)
	require.NoError(t, err)
	defer store.Close()

	metadata := ArtifactMetadata{
		Name:       "test-file",
		Path:       "/test/file.txt",
		Type:       "file",
		WorkflowID: "workflow-123",
		RunID:      "run-456",
		StepName:   "build",
	}

	// Upload artifact
	err = store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	// Delete artifact
	err = store.Delete(ctx, metadata)
	require.NoError(t, err)

	// Verify artifact doesn't exist
	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestMinioStore_List(t *testing.T) {
	ctx := context.Background()

	// Start Minio container
	container, config := setupMinioContainer(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Create Minio store
	store, err := NewMinioStore(ctx, config)
	require.NoError(t, err)
	defer store.Close()

	// Upload multiple artifacts
	artifacts := []ArtifactMetadata{
		{
			Name:       "file1",
			WorkflowID: "workflow-123",
			RunID:      "run-456",
			StepName:   "build",
			Type:       "file",
		},
		{
			Name:       "file2",
			WorkflowID: "workflow-123",
			RunID:      "run-456",
			StepName:   "test",
			Type:       "file",
		},
		{
			Name:       "file3",
			WorkflowID: "workflow-123",
			RunID:      "run-789",
			StepName:   "build",
			Type:       "file",
		},
	}

	for _, metadata := range artifacts {
		err := store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
		require.NoError(t, err)
	}

	// List artifacts for specific run
	listed, err := store.List(ctx, "workflow-123/run-456/")
	require.NoError(t, err)
	assert.Len(t, listed, 2)

	// List all artifacts for workflow
	listed, err = store.List(ctx, "workflow-123/")
	require.NoError(t, err)
	assert.Len(t, listed, 3)
}

func TestMinioStore_UploadDownloadActivities(t *testing.T) {
	ctx := context.Background()

	// Start Minio container
	container, config := setupMinioContainer(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Create Minio store
	store, err := NewMinioStore(ctx, config)
	require.NoError(t, err)
	defer store.Close()

	// Create a temporary file
	content := []byte("test file content for activities")
	tmpFile := t.TempDir() + "/test-file.txt"
	err = writeFile(tmpFile, content)
	require.NoError(t, err)

	// Upload using activity
	metadata := ArtifactMetadata{
		Name:       "activity-test",
		Path:       tmpFile,
		Type:       "file",
		WorkflowID: "workflow-activity",
		RunID:      "run-activity",
		StepName:   "upload-step",
	}

	uploadInput := UploadArtifactInput{
		Metadata:   metadata,
		SourcePath: tmpFile,
	}

	err = UploadArtifactActivity(ctx, store, uploadInput)
	require.NoError(t, err)

	// Download using activity
	destFile := t.TempDir() + "/downloaded-file.txt"
	downloadInput := DownloadArtifactInput{
		Metadata: metadata,
		DestPath: destFile,
	}

	err = DownloadArtifactActivity(ctx, store, downloadInput)
	require.NoError(t, err)

	// Verify content
	downloaded, err := readFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)
}

func TestMinioStore_CleanupWorkflow(t *testing.T) {
	ctx := context.Background()

	// Start Minio container
	container, config := setupMinioContainer(ctx, t)
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}()

	// Create Minio store
	store, err := NewMinioStore(ctx, config)
	require.NoError(t, err)
	defer store.Close()

	workflowID := "workflow-cleanup"
	runID := "run-cleanup"

	// Upload multiple artifacts
	artifacts := []ArtifactMetadata{
		{Name: "file1", WorkflowID: workflowID, RunID: runID, StepName: "step1", Type: "file"},
		{Name: "file2", WorkflowID: workflowID, RunID: runID, StepName: "step2", Type: "file"},
		{Name: "file3", WorkflowID: workflowID, RunID: runID, StepName: "step3", Type: "file"},
	}

	for _, metadata := range artifacts {
		err := store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
		require.NoError(t, err)
	}

	// Verify artifacts exist
	for _, metadata := range artifacts {
		exists, err := store.Exists(ctx, metadata)
		require.NoError(t, err)
		assert.True(t, exists)
	}

	// Cleanup
	err = CleanupWorkflowArtifacts(ctx, store, workflowID, runID)
	require.NoError(t, err)

	// Verify artifacts are deleted
	for _, metadata := range artifacts {
		exists, err := store.Exists(ctx, metadata)
		require.NoError(t, err)
		assert.False(t, exists)
	}
}

// Helper functions for file operations
func writeFile(path string, content []byte) error {
	return os.WriteFile(path, content, 0o644)
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
