package store

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

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

	return filepath.WalkDir(sourceDir, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks to prevent including files outside source directory.
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		fi, err := d.Info()
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
