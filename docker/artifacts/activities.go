package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// ArtifactTypeDirectory represents a directory artifact type.
	ArtifactTypeDirectory = "directory"
	// ArtifactTypeFile represents a file artifact type.
	ArtifactTypeFile = "file"
)

// UploadArtifactInput contains input for uploading an artifact.
type UploadArtifactInput struct {
	// Metadata contains artifact metadata
	Metadata ArtifactMetadata

	// SourcePath is the local path to upload from
	SourcePath string
}

// DownloadArtifactInput contains input for downloading an artifact.
type DownloadArtifactInput struct {
	// Metadata contains artifact metadata
	Metadata ArtifactMetadata

	// DestPath is the local path to download to
	DestPath string
}

// UploadArtifactActivity uploads an artifact from local filesystem to the artifact store.
func UploadArtifactActivity(ctx context.Context, store ArtifactStore, input UploadArtifactInput) error {
	// Determine artifact type if not specified
	if input.Metadata.Type == "" {
		fileInfo, err := os.Stat(input.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to stat source path: %w", err)
		}

		if fileInfo.IsDir() {
			input.Metadata.Type = ArtifactTypeDirectory
		} else {
			input.Metadata.Type = ArtifactTypeFile
		}
	}

	// Handle different artifact types
	switch input.Metadata.Type {
	case ArtifactTypeFile:
		return uploadFile(ctx, store, input)
	case ArtifactTypeDirectory, "archive":
		return uploadDirectory(ctx, store, input)
	default:
		return fmt.Errorf("unsupported artifact type: %s", input.Metadata.Type)
	}
}

// uploadFile uploads a single file.
func uploadFile(ctx context.Context, store ArtifactStore, input UploadArtifactInput) error {
	file, err := os.Open(input.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	input.Metadata.Size = fileInfo.Size()

	return store.Upload(ctx, input.Metadata, file)
}

// uploadDirectory creates a tar.gz archive and uploads it.
func uploadDirectory(ctx context.Context, store ArtifactStore, input UploadArtifactInput) error {
	// Create a buffer to hold the archive
	var buf bytes.Buffer

	// Archive the directory
	if err := ArchiveDirectory(input.SourcePath, &buf); err != nil {
		return fmt.Errorf("failed to archive directory: %w", err)
	}

	// Update metadata
	input.Metadata.Size = int64(buf.Len())
	input.Metadata.ContentType = "application/gzip"

	// Upload the archive
	return store.Upload(ctx, input.Metadata, &buf)
}

// DownloadArtifactActivity downloads an artifact from the artifact store to local filesystem.
func DownloadArtifactActivity(ctx context.Context, store ArtifactStore, input DownloadArtifactInput) error {
	// Download artifact
	reader, err := store.Download(ctx, input.Metadata)
	if err != nil {
		return fmt.Errorf("failed to download artifact: %w", err)
	}
	defer func() { _ = reader.Close() }()

	// Handle different artifact types
	switch input.Metadata.Type {
	case ArtifactTypeFile:
		return downloadFile(reader, input.DestPath)
	case ArtifactTypeDirectory, "archive":
		return downloadDirectory(reader, input.DestPath)
	default:
		return fmt.Errorf("unsupported artifact type: %s", input.Metadata.Type)
	}
}

// downloadFile downloads a single file.
func downloadFile(reader io.Reader, destPath string) error {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	// Copy data
	_, err = io.Copy(file, reader)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// downloadDirectory extracts an archive to a directory.
func downloadDirectory(reader io.Reader, destPath string) error {
	// Create destination directory
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract archive
	if err := ExtractArchive(reader, destPath); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

// DeleteArtifactActivity deletes an artifact from the store.
func DeleteArtifactActivity(ctx context.Context, store ArtifactStore, metadata ArtifactMetadata) error {
	return store.Delete(ctx, metadata)
}

// CleanupWorkflowArtifacts deletes all artifacts for a workflow.
func CleanupWorkflowArtifacts(ctx context.Context, store ArtifactStore, workflowID, runID string) error {
	// List all artifacts for this workflow
	prefix := workflowID + "/" + runID + "/"
	artifacts, err := store.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to list artifacts: %w", err)
	}

	// Delete each artifact
	for _, artifact := range artifacts {
		if err := store.Delete(ctx, artifact); err != nil {
			// Log error but continue cleanup
			return fmt.Errorf("failed to delete artifact %s: %w", artifact.Name, err)
		}
	}

	return nil
}
