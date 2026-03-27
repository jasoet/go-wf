package store

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxUploadSize is the maximum size for uploads (1GB).
	MaxUploadSize = 1 << 30
)

// LocalStore implements RawStore using the local filesystem.
type LocalStore struct {
	basePath string
}

// NewLocalStore creates a new LocalStore rooted at basePath.
// The base directory is created if it does not exist.
func NewLocalStore(basePath string) (*LocalStore, error) {
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve base path: %w", err)
	}

	return &LocalStore{
		basePath: absPath,
	}, nil
}

// validateKey checks that the key does not escape the base directory.
func (s *LocalStore) validateKey(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key must not be empty")
	}
	if strings.Contains(key, "..") {
		return "", fmt.Errorf("key contains path traversal sequence")
	}
	if strings.ContainsAny(key, "\\\x00") {
		return "", fmt.Errorf("key contains forbidden characters")
	}

	// Resolve full path and verify it stays within basePath.
	fullPath := filepath.Join(s.basePath, filepath.FromSlash(key))
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve key path: %w", err)
	}

	if !strings.HasPrefix(absPath, s.basePath+string(filepath.Separator)) {
		return "", fmt.Errorf("key escapes base directory")
	}

	return absPath, nil
}

// validatePrefix checks that the prefix does not escape the base directory.
func (s *LocalStore) validatePrefix(prefix string) (string, error) {
	if strings.Contains(prefix, "..") {
		return "", fmt.Errorf("prefix contains path traversal sequence")
	}
	if strings.ContainsAny(prefix, "\\\x00") {
		return "", fmt.Errorf("prefix contains forbidden characters")
	}

	searchPath := filepath.Join(s.basePath, filepath.FromSlash(prefix))
	absPath, err := filepath.Abs(searchPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve prefix path: %w", err)
	}

	if !strings.HasPrefix(absPath, s.basePath) {
		return "", fmt.Errorf("prefix escapes base directory")
	}

	return absPath, nil
}

// Upload stores data under the given key on the local filesystem.
func (s *LocalStore) Upload(_ context.Context, key string, data io.Reader) error {
	fullPath, err := s.validateKey(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0o750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) //#nosec G304 -- path validated by validateKey
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	limitedData := io.LimitReader(data, MaxUploadSize)
	if _, err := io.Copy(file, limitedData); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Check if data was truncated by attempting to read one more byte.
	var extra [1]byte
	if n, _ := data.Read(extra[:]); n > 0 {
		return fmt.Errorf("data exceeds maximum upload size of 1GB")
	}

	return nil
}

// Download retrieves data for the given key from the local filesystem.
func (s *LocalStore) Download(_ context.Context, key string) (io.ReadCloser, error) {
	fullPath, err := s.validateKey(key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(fullPath) //#nosec G304 -- path validated by validateKey
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// Delete removes the data stored under the given key.
func (s *LocalStore) Delete(_ context.Context, key string) error {
	fullPath, err := s.validateKey(key)
	if err != nil {
		return err
	}

	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// Exists checks whether data exists under the given key.
func (s *LocalStore) Exists(_ context.Context, key string) (bool, error) {
	fullPath, err := s.validateKey(key)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file: %w", err)
	}

	return true, nil
}

// List returns all keys matching the given prefix.
func (s *LocalStore) List(_ context.Context, prefix string) ([]string, error) {
	searchPath, err := s.validatePrefix(prefix)
	if err != nil {
		return nil, err
	}

	var keys []string

	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(s.basePath, path)
		if err != nil {
			return err
		}

		// Convert OS path separators to forward slashes for consistent keys.
		keys = append(keys, filepath.ToSlash(relPath))

		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	return keys, nil
}

// Close is a no-op for LocalStore.
func (s *LocalStore) Close() error {
	return nil
}
