package artifacts

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalFileStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, store)
	assert.Equal(t, tmpDir, store.BasePath)

	// Verify directory was created
	_, err = os.Stat(tmpDir)
	assert.NoError(t, err)
}

func TestLocalFileStore_UploadDownload(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	metadata := ArtifactMetadata{
		Name:       "test-file",
		Path:       "/test/file.txt",
		Type:       "file",
		WorkflowID: "workflow-123",
		RunID:      "run-456",
		StepName:   "build",
	}

	// Upload artifact
	content := []byte("test content")
	err = store.Upload(ctx, metadata, bytes.NewReader(content))
	require.NoError(t, err)

	// Verify file exists
	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.True(t, exists)

	// Download artifact
	reader, err := store.Download(ctx, metadata)
	require.NoError(t, err)
	defer reader.Close()

	// Verify content
	downloaded := make([]byte, len(content))
	_, err = reader.Read(downloaded)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)
}

func TestLocalFileStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
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

	// Deleting non-existent artifact should not error
	err = store.Delete(ctx, metadata)
	assert.NoError(t, err)
}

func TestLocalFileStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Upload multiple artifacts
	artifacts := []ArtifactMetadata{
		{
			Name:       "file1",
			WorkflowID: "workflow-123",
			RunID:      "run-456",
			StepName:   "build",
		},
		{
			Name:       "file2",
			WorkflowID: "workflow-123",
			RunID:      "run-456",
			StepName:   "test",
		},
		{
			Name:       "file3",
			WorkflowID: "workflow-123",
			RunID:      "run-789",
			StepName:   "build",
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

func TestArtifactMetadata_StorageKey(t *testing.T) {
	metadata := ArtifactMetadata{
		Name:       "test-artifact",
		WorkflowID: "workflow-123",
		RunID:      "run-456",
		StepName:   "build",
	}

	expected := "workflow-123/run-456/build/test-artifact"
	assert.Equal(t, expected, metadata.StorageKey())
}

func TestArchiveDirectory(t *testing.T) {
	// Create temporary directory with test files
	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "file1.txt")
	testFile2 := filepath.Join(tmpDir, "subdir", "file2.txt")

	err := os.WriteFile(testFile1, []byte("content1"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Dir(testFile2), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("content2"), 0o644)
	require.NoError(t, err)

	// Archive directory
	var buf bytes.Buffer
	err = ArchiveDirectory(tmpDir, &buf)
	require.NoError(t, err)
	assert.Greater(t, buf.Len(), 0)
}

func TestExtractArchive(t *testing.T) {
	// Create temporary directory with test files
	srcDir := t.TempDir()
	testFile1 := filepath.Join(srcDir, "file1.txt")
	testFile2 := filepath.Join(srcDir, "subdir", "file2.txt")

	err := os.WriteFile(testFile1, []byte("content1"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Dir(testFile2), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("content2"), 0o644)
	require.NoError(t, err)

	// Archive directory
	var buf bytes.Buffer
	err = ArchiveDirectory(srcDir, &buf)
	require.NoError(t, err)

	// Extract to new directory
	destDir := t.TempDir()
	err = ExtractArchive(&buf, destDir)
	require.NoError(t, err)

	// Verify extracted files
	extractedFile1 := filepath.Join(destDir, "file1.txt")
	content1, err := os.ReadFile(extractedFile1)
	require.NoError(t, err)
	assert.Equal(t, []byte("content1"), content1)

	extractedFile2 := filepath.Join(destDir, "subdir", "file2.txt")
	content2, err := os.ReadFile(extractedFile2)
	require.NoError(t, err)
	assert.Equal(t, []byte("content2"), content2)
}

func TestUploadDownloadFile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a test file
	srcFile := filepath.Join(t.TempDir(), "source.txt")
	err = os.WriteFile(srcFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	// Upload
	metadata := ArtifactMetadata{
		Name:       "upload-test",
		Path:       srcFile,
		Type:       "file",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
	}

	uploadInput := UploadArtifactInput{
		Metadata:   metadata,
		SourcePath: srcFile,
	}

	err = UploadArtifactActivity(ctx, store, uploadInput)
	require.NoError(t, err)

	// Download
	destFile := filepath.Join(t.TempDir(), "dest.txt")
	downloadInput := DownloadArtifactInput{
		Metadata: metadata,
		DestPath: destFile,
	}

	err = DownloadArtifactActivity(ctx, store, downloadInput)
	require.NoError(t, err)

	// Verify content
	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)
}

func TestUploadDownloadDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a test directory
	srcDir := t.TempDir()
	err = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0o644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0o644)
	require.NoError(t, err)

	// Upload
	metadata := ArtifactMetadata{
		Name:       "dir-test",
		Path:       srcDir,
		Type:       "directory",
		WorkflowID: "wf-1",
		RunID:      "run-1",
		StepName:   "step-1",
	}

	uploadInput := UploadArtifactInput{
		Metadata:   metadata,
		SourcePath: srcDir,
	}

	err = UploadArtifactActivity(ctx, store, uploadInput)
	require.NoError(t, err)

	// Download
	destDir := t.TempDir()
	downloadInput := DownloadArtifactInput{
		Metadata: metadata,
		DestPath: destDir,
	}

	err = DownloadArtifactActivity(ctx, store, downloadInput)
	require.NoError(t, err)

	// Verify content
	content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content1"), content1)

	content2, err := os.ReadFile(filepath.Join(destDir, "subdir", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content2"), content2)
}

func TestCleanupWorkflowArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Upload multiple artifacts
	workflowID := "workflow-cleanup-test"
	runID := "run-123"

	artifacts := []ArtifactMetadata{
		{Name: "file1", WorkflowID: workflowID, RunID: runID, StepName: "step1"},
		{Name: "file2", WorkflowID: workflowID, RunID: runID, StepName: "step2"},
		{Name: "file3", WorkflowID: workflowID, RunID: runID, StepName: "step3"},
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

func TestDeleteArtifactActivity(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name        string
		setupFunc   func() ArtifactMetadata
		expectError bool
	}{
		{
			name: "delete existing artifact",
			setupFunc: func() ArtifactMetadata {
				metadata := ArtifactMetadata{
					Name:       "delete-test",
					WorkflowID: "wf-1",
					RunID:      "run-1",
					StepName:   "step-1",
					Type:       "file",
				}
				err := store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
				require.NoError(t, err)
				return metadata
			},
			expectError: false,
		},
		{
			name: "delete non-existent artifact",
			setupFunc: func() ArtifactMetadata {
				return ArtifactMetadata{
					Name:       "non-existent",
					WorkflowID: "wf-1",
					RunID:      "run-1",
					StepName:   "step-1",
					Type:       "file",
				}
			},
			expectError: false, // Should not error on non-existent
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := tt.setupFunc()
			err := DeleteArtifactActivity(ctx, store, metadata)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify artifact doesn't exist
				exists, err := store.Exists(ctx, metadata)
				require.NoError(t, err)
				assert.False(t, exists)
			}
		})
	}
}

func TestLocalFileStore_Close(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	// Close should not error (no-op for local store)
	err = store.Close()
	assert.NoError(t, err)
}

func TestLocalFileStore_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("download non-existent file", func(t *testing.T) {
		metadata := ArtifactMetadata{
			Name:       "non-existent",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
		}

		reader, err := store.Download(ctx, metadata)
		assert.Error(t, err)
		assert.Nil(t, reader)
		assert.Contains(t, err.Error(), "artifact not found")
	})

	t.Run("list non-existent prefix", func(t *testing.T) {
		// List non-existent prefix should return nil without error
		artifacts, err := store.List(ctx, "non-existent/prefix/")
		assert.NoError(t, err)
		assert.Nil(t, artifacts)
	})
}

func TestUploadArtifactActivity_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("non-existent source file", func(t *testing.T) {
		input := UploadArtifactInput{
			Metadata: ArtifactMetadata{
				Name:       "test",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				StepName:   "step-1",
				Type:       "file",
			},
			SourcePath: "/non/existent/file.txt",
		}

		err := UploadArtifactActivity(ctx, store, input)
		assert.Error(t, err)
	})

	t.Run("auto-detect file type", func(t *testing.T) {
		// Create a test file
		srcFile := filepath.Join(t.TempDir(), "autodetect.txt")
		err := os.WriteFile(srcFile, []byte("test"), 0o644)
		require.NoError(t, err)

		input := UploadArtifactInput{
			Metadata: ArtifactMetadata{
				Name:       "autodetect",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				StepName:   "step-1",
				// Type not specified - should auto-detect
			},
			SourcePath: srcFile,
		}

		err = UploadArtifactActivity(ctx, store, input)
		assert.NoError(t, err)
	})

	t.Run("auto-detect directory type", func(t *testing.T) {
		// Create a test directory
		srcDir := t.TempDir()
		err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0o644)
		require.NoError(t, err)

		input := UploadArtifactInput{
			Metadata: ArtifactMetadata{
				Name:       "autodetect-dir",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				StepName:   "step-1",
				// Type not specified - should auto-detect as directory
			},
			SourcePath: srcDir,
		}

		err = UploadArtifactActivity(ctx, store, input)
		assert.NoError(t, err)
	})

	t.Run("unsupported artifact type", func(t *testing.T) {
		srcFile := filepath.Join(t.TempDir(), "test.txt")
		err := os.WriteFile(srcFile, []byte("test"), 0o644)
		require.NoError(t, err)

		input := UploadArtifactInput{
			Metadata: ArtifactMetadata{
				Name:       "test",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				StepName:   "step-1",
				Type:       "unsupported-type",
			},
			SourcePath: srcFile,
		}

		err = UploadArtifactActivity(ctx, store, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported artifact type")
	})

	t.Run("archive type upload", func(t *testing.T) {
		// Create a test directory
		srcDir := t.TempDir()
		err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0o644)
		require.NoError(t, err)

		input := UploadArtifactInput{
			Metadata: ArtifactMetadata{
				Name:       "archive-test",
				WorkflowID: "wf-1",
				RunID:      "run-1",
				StepName:   "step-1",
				Type:       "archive",
			},
			SourcePath: srcDir,
		}

		err = UploadArtifactActivity(ctx, store, input)
		assert.NoError(t, err)
	})
}

func TestDownloadArtifactActivity_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("unsupported artifact type", func(t *testing.T) {
		// Upload an artifact with unsupported type for download
		metadata := ArtifactMetadata{
			Name:       "test",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
			Type:       "file",
		}

		err := store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
		require.NoError(t, err)

		// Try to download with unsupported type
		metadata.Type = "unsupported"
		input := DownloadArtifactInput{
			Metadata: metadata,
			DestPath: filepath.Join(t.TempDir(), "dest.txt"),
		}

		err = DownloadArtifactActivity(ctx, store, input)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported artifact type")
	})

	t.Run("download archive type", func(t *testing.T) {
		// Create and upload a directory as archive
		srcDir := t.TempDir()
		err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0o644)
		require.NoError(t, err)

		metadata := ArtifactMetadata{
			Name:       "archive-download",
			WorkflowID: "wf-1",
			RunID:      "run-1",
			StepName:   "step-1",
			Type:       "archive",
		}

		uploadInput := UploadArtifactInput{
			Metadata:   metadata,
			SourcePath: srcDir,
		}

		err = UploadArtifactActivity(ctx, store, uploadInput)
		require.NoError(t, err)

		// Download as archive
		destDir := t.TempDir()
		downloadInput := DownloadArtifactInput{
			Metadata: metadata,
			DestPath: destDir,
		}

		err = DownloadArtifactActivity(ctx, store, downloadInput)
		assert.NoError(t, err)
	})
}
