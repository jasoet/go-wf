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
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalFileStore{
		BasePath: basePath,
	}, nil
}

// Upload uploads an artifact to the local filesystem.
func (s *LocalFileStore) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}

	// Build full path
	fullPath := filepath.Join(s.BasePath, metadata.StorageKey())

	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //#nosec G304 -- path validated by ValidateMetadata
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

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
	if err := ValidateMetadata(metadata); err != nil {
		return nil, err
	}

	fullPath := filepath.Join(s.BasePath, metadata.StorageKey())

	file, err := os.Open(fullPath) //#nosec G304 -- path validated by ValidateMetadata
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
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}

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
	if err := ValidateMetadata(metadata); err != nil {
		return false, err
	}

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
	if err := ValidatePrefix(prefix); err != nil {
		return nil, err
	}

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
func ArchiveDirectory(sourceDir string, writer io.Writer) (err error) {
	gzipWriter := gzip.NewWriter(writer)
	defer func() {
		if closeErr := gzipWriter.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	tarWriter := tar.NewWriter(gzipWriter)
	defer func() {
		if closeErr := tarWriter.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

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
		f, err := os.Open(file) //#nosec G304,G122 -- path from filepath.Walk within controlled sourceDir
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}()

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
	defer func() {
		if closeErr := gzipReader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	tarReader := tar.NewReader(gzipReader)
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("failed to resolve destination directory: %w", err)
	}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Sanitize path to prevent directory traversal.
		target := filepath.Join(absDestDir, filepath.Clean(header.Name))
		if !strings.HasPrefix(target, absDestDir+string(filepath.Separator)) && target != absDestDir {
			return fmt.Errorf("illegal file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			if err := extractFileFromArchive(tarReader, target); err != nil {
				return err
			}
		}
	}

	return nil
}

// extractFileFromArchive writes a single file from the tar reader with size-limited copy.
func extractFileFromArchive(tarReader *tar.Reader, target string) error {
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //#nosec G304 -- target is sanitized against path traversal
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	// Limit extraction to 1GB to prevent decompression bomb.
	const maxFileSize = 1 << 30
	if _, err := io.Copy(f, io.LimitReader(tarReader, maxFileSize)); err != nil {
		if closeErr := f.Close(); closeErr != nil {
			return fmt.Errorf("failed to write file: %w (close error: %v)", err, closeErr)
		}
		return fmt.Errorf("failed to write file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}
	return nil
}
