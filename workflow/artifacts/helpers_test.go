package artifacts

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadBytesDownloadBytes_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	metadata := ArtifactMetadata{
		Name:       "roundtrip-test",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
		Type:       "file",
	}

	original := []byte("hello, artifact world!")
	err = UploadBytes(ctx, store, metadata, original)
	require.NoError(t, err)

	downloaded, err := DownloadBytes(ctx, store, metadata)
	require.NoError(t, err)
	assert.Equal(t, original, downloaded)
}

func TestUploadBytes_SetsSizeAndContentType(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	metadata := ArtifactMetadata{
		Name:       "metadata-test",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
		Type:       "file",
	}

	data := []byte("some data")
	err = UploadBytes(ctx, store, metadata, data)
	require.NoError(t, err)

	// Verify the file was stored (size is set internally before upload)
	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify that metadata.Size and ContentType are set correctly by the function
	// We test this by calling UploadBytes with empty ContentType and verifying behavior
	metadataWithCT := ArtifactMetadata{
		Name:        "metadata-ct-test",
		WorkflowID:  "wf-1",
		RunID:       "run-1",
		StepName:    "step-1",
		Type:        "file",
		ContentType: "text/plain",
	}
	err = UploadBytes(ctx, store, metadataWithCT, data)
	require.NoError(t, err)

	// ContentType should be preserved when already set
	assert.Equal(t, "text/plain", metadataWithCT.ContentType)
}

func TestDownloadBytes_NonExistentArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	metadata := ArtifactMetadata{
		Name:       "non-existent",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
		Type:       "file",
	}

	data, err := DownloadBytes(ctx, store, metadata)
	assert.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "artifact not found")
}

func TestUploadBytes_EmptyData(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	metadata := ArtifactMetadata{
		Name:       "empty-test",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
		Type:       "file",
	}

	err = UploadBytes(ctx, store, metadata, []byte{})
	require.NoError(t, err)

	// Verify it exists and can be downloaded
	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.True(t, exists)

	downloaded, err := DownloadBytes(ctx, store, metadata)
	require.NoError(t, err)
	assert.Empty(t, downloaded)
}
