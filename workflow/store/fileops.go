package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// FileTypeDirectory represents a directory artifact type.
	FileTypeDirectory = "directory"
	// FileTypeFile represents a single-file artifact type.
	FileTypeFile = "file"
	// FileTypeArchive represents a tar.gz archive artifact type.
	FileTypeArchive = "archive"
)

// UploadFile uploads a file or directory from the local filesystem to the store
// under the given key. Supported types: "file" uploads the file as-is;
// "directory"/"archive" stream a tar.gz of the directory via io.Pipe.
// An empty typ auto-detects from the source path. Uploads are capped at
// MaxUploadSize (enforced by RawStore implementations).
func UploadFile(ctx context.Context, raw RawStore, key, sourcePath, typ string) error {
	// Determine artifact type if not specified
	if typ == "" {
		fileInfo, err := os.Stat(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to stat source path: %w", err)
		}

		if fileInfo.IsDir() {
			typ = FileTypeDirectory
		} else {
			typ = FileTypeFile
		}
	}

	switch typ {
	case FileTypeFile:
		return uploadSingleFile(ctx, raw, key, sourcePath)
	case FileTypeDirectory, FileTypeArchive:
		return uploadDirectoryArchive(ctx, raw, key, sourcePath)
	default:
		return fmt.Errorf("unsupported artifact type: %s", typ)
	}
}

// uploadSingleFile uploads a single file under the given key.
func uploadSingleFile(ctx context.Context, raw RawStore, key, sourcePath string) (err error) {
	file, err := os.Open(sourcePath) //#nosec G304 -- sourcePath is from workflow config, not user input
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	return raw.Upload(ctx, key, file)
}

// uploadDirectoryArchive creates a tar.gz archive and uploads it using streaming.
func uploadDirectoryArchive(ctx context.Context, raw RawStore, key, sourcePath string) error {
	pr, pw := io.Pipe()

	// Archive in a goroutine, streaming to the pipe writer.
	var archiveErr error
	go func() {
		archiveErr = ArchiveDirectory(sourcePath, pw)
		pw.CloseWithError(archiveErr)
	}()

	// Upload reads from the pipe reader (streaming, no full buffer).
	if err := raw.Upload(ctx, key, pr); err != nil {
		// Drain the pipe to unblock the archive goroutine.
		_ = pr.Close() //nolint:errcheck // intentionally ignoring close error during error handling
		if archiveErr != nil {
			return fmt.Errorf("failed to archive directory: %w", archiveErr)
		}
		return fmt.Errorf("failed to upload directory archive: %w", err)
	}

	if archiveErr != nil {
		return fmt.Errorf("failed to archive directory: %w", archiveErr)
	}

	return nil
}

// DownloadFile downloads the data stored under key to a local path.
// Supported types: "file" writes the file (creating parent directories);
// "directory"/"archive" extracts a tar.gz stream into destPath.
func DownloadFile(ctx context.Context, raw RawStore, key, destPath, typ string) (err error) {
	reader, err := raw.Download(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", key, err)
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	switch typ {
	case FileTypeFile:
		return downloadSingleFile(reader, destPath)
	case FileTypeDirectory, FileTypeArchive:
		return downloadDirectoryArchive(reader, destPath)
	default:
		return fmt.Errorf("unsupported artifact type: %s", typ)
	}
}

// downloadSingleFile writes the reader contents to a single file.
func downloadSingleFile(reader io.Reader, destPath string) error {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create destination file
	file, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //#nosec G304 -- destPath is from workflow config, not user input
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

// downloadDirectoryArchive extracts an archive to a directory.
func downloadDirectoryArchive(reader io.Reader, destPath string) error {
	// Create destination directory
	if err := os.MkdirAll(destPath, 0o750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract archive
	if err := ExtractArchive(reader, destPath); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

// DeletePrefix removes every key stored under the given prefix.
// It lists the prefix and deletes each key, continuing past individual
// failures and reporting the first error encountered.
func DeletePrefix(ctx context.Context, raw RawStore, prefix string) error {
	keys, err := raw.List(ctx, prefix)
	if err != nil {
		return fmt.Errorf("failed to list keys: %w", err)
	}

	var errs []error
	for _, key := range keys {
		select {
		case <-ctx.Done():
			return fmt.Errorf("delete canceled: %w", ctx.Err())
		default:
		}
		if err := raw.Delete(ctx, key); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete %s: %w", key, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("delete completed with %d errors: %v", len(errs), errs[0])
	}
	return nil
}
