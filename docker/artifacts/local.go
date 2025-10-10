package artifacts

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalFileStore implements ArtifactStore using the local filesystem.
type LocalFileStore struct {
	// BasePath is the root directory for storing artifacts
	BasePath string
}

// NewLocalFileStore creates a new local file store.
func NewLocalFileStore(basePath string) (*LocalFileStore, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalFileStore{
		BasePath: basePath,
	}, nil
}

// Upload uploads an artifact to the local filesystem.
func (s *LocalFileStore) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	// Build full path
	fullPath := filepath.Join(s.BasePath, metadata.StorageKey())

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	written, err := io.Copy(file, data)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Update metadata size
	metadata.Size = written

	return nil
}

// Download downloads an artifact from the local filesystem.
func (s *LocalFileStore) Download(ctx context.Context, metadata ArtifactMetadata) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.BasePath, metadata.StorageKey())

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("artifact not found: %s", metadata.Name)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes an artifact from the local filesystem.
func (s *LocalFileStore) Delete(ctx context.Context, metadata ArtifactMetadata) error {
	fullPath := filepath.Join(s.BasePath, metadata.StorageKey())

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in the local filesystem.
func (s *LocalFileStore) Exists(ctx context.Context, metadata ArtifactMetadata) (bool, error) {
	fullPath := filepath.Join(s.BasePath, metadata.StorageKey())

	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	return true, nil
}

// List returns all artifacts matching the given prefix.
func (s *LocalFileStore) List(ctx context.Context, prefix string) ([]ArtifactMetadata, error) {
	searchPath := filepath.Join(s.BasePath, prefix)
	var artifacts []ArtifactMetadata

	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Get relative path from base
		relPath, err := filepath.Rel(s.BasePath, path)
		if err != nil {
			return err
		}

		// Parse storage key: workflow_id/run_id/step_name/artifact_name
		parts := strings.Split(relPath, string(filepath.Separator))
		if len(parts) < 4 {
			return nil // Skip invalid paths
		}

		artifacts = append(artifacts, ArtifactMetadata{
			Name:       parts[3],
			WorkflowID: parts[0],
			RunID:      parts[1],
			StepName:   parts[2],
			Size:       info.Size(),
		})

		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No artifacts found
		}
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	return artifacts, nil
}

// Close cleans up resources (no-op for local file store).
func (s *LocalFileStore) Close() error {
	return nil
}

// ArchiveDirectory creates a tar.gz archive of a directory.
func ArchiveDirectory(sourceDir string, writer io.Writer) error {
	gzipWriter := gzip.NewWriter(writer)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	return filepath.Walk(sourceDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create tar header
		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}

		// Update header name to be relative to source
		relPath, err := filepath.Rel(sourceDir, file)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}

		// If not a regular file, skip
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Write file data
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tarWriter, f)
		return err
	})
}

// ExtractArchive extracts a tar.gz archive to a directory.
func ExtractArchive(reader io.Reader, destDir string) error {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
			f, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			// Copy data
			if _, err := io.Copy(f, tarReader); err != nil {
				f.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			f.Close()
		}
	}

	return nil
}
