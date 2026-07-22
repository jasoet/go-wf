package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadDownloadFile(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	srcFile := filepath.Join(t.TempDir(), "source.txt")
	err = os.WriteFile(srcFile, []byte("test content"), 0o644)
	require.NoError(t, err)

	key := "wf-1/run-1/step-1/upload-test"
	err = UploadFile(ctx, raw, key, srcFile, "file")
	require.NoError(t, err)

	destFile := filepath.Join(t.TempDir(), "dest.txt")
	err = DownloadFile(ctx, raw, key, destFile, "file")
	require.NoError(t, err)

	content, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, []byte("test content"), content)
}

func TestUploadDownloadDirectory(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	srcDir := t.TempDir()
	err = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0o644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0o644)
	require.NoError(t, err)

	key := "wf-1/run-1/step-1/dir-test"
	err = UploadFile(ctx, raw, key, srcDir, "directory")
	require.NoError(t, err)

	destDir := t.TempDir()
	err = DownloadFile(ctx, raw, key, destDir, "directory")
	require.NoError(t, err)

	content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content1"), content1)

	content2, err := os.ReadFile(filepath.Join(destDir, "subdir", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content2"), content2)
}

func TestUploadDownloadArchiveType(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	srcDir := t.TempDir()
	err = os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("archived"), 0o644)
	require.NoError(t, err)

	key := "wf-1/run-1/step-1/archive-test"
	err = UploadFile(ctx, raw, key, srcDir, "archive")
	require.NoError(t, err)

	destDir := t.TempDir()
	err = DownloadFile(ctx, raw, key, destDir, "archive")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(destDir, "file.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("archived"), content)
}

func TestUploadDirectory_StreamingRoundTrip(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	srcDir := t.TempDir()
	err = os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0o644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("content2"), 0o644)
	require.NoError(t, err)

	key := "wf-1/run-1/step-1/streaming-test"
	err = UploadFile(ctx, raw, key, srcDir, "directory")
	require.NoError(t, err)

	// The stored object is a gzip stream, readable end-to-end.
	rc, err := raw.Download(ctx, key)
	require.NoError(t, err)
	defer rc.Close() //nolint:errcheck // test cleanup

	destDir := t.TempDir()
	err = ExtractArchive(rc, destDir)
	require.NoError(t, err)

	content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content1"), content1)

	content2, err := os.ReadFile(filepath.Join(destDir, "subdir", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content2"), content2)
}

func TestUploadFile_AutoDetectType(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("file", func(t *testing.T) {
		srcFile := filepath.Join(t.TempDir(), "autodetect.txt")
		err := os.WriteFile(srcFile, []byte("test"), 0o644)
		require.NoError(t, err)

		err = UploadFile(ctx, raw, "wf-1/run-1/step-1/autodetect", srcFile, "")
		assert.NoError(t, err)
	})

	t.Run("directory", func(t *testing.T) {
		srcDir := t.TempDir()
		err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0o644)
		require.NoError(t, err)

		err = UploadFile(ctx, raw, "wf-1/run-1/step-1/autodetect-dir", srcDir, "")
		assert.NoError(t, err)
	})
}

func TestUploadFile_ErrorCases(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("stat error on auto-detect", func(t *testing.T) {
		err := UploadFile(ctx, raw, "wf-1/run-1/step-1/test", "/non/existent/path", "")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to stat source path")
	})

	t.Run("non-existent source file", func(t *testing.T) {
		err := UploadFile(ctx, raw, "wf-1/run-1/step-1/test", "/non/existent/file.txt", "file")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open source file")
	})

	t.Run("non-existent source directory", func(t *testing.T) {
		err := UploadFile(ctx, raw, "wf-1/run-1/step-1/test", "/non/existent/directory", "directory")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to archive directory")
	})

	t.Run("unsupported type", func(t *testing.T) {
		srcFile := filepath.Join(t.TempDir(), "test.txt")
		err := os.WriteFile(srcFile, []byte("test"), 0o644)
		require.NoError(t, err)

		err = UploadFile(ctx, raw, "wf-1/run-1/step-1/test", srcFile, "unsupported-type")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported artifact type")
	})
}

func TestDownloadFile_ErrorCases(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("missing key", func(t *testing.T) {
		err := DownloadFile(ctx, raw, "wf-1/run-1/step-1/non-existent", filepath.Join(t.TempDir(), "dest.txt"), "file")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to download")
	})

	t.Run("unsupported type", func(t *testing.T) {
		key := "wf-1/run-1/step-1/test"
		srcFile := filepath.Join(t.TempDir(), "src.txt")
		err := os.WriteFile(srcFile, []byte("test"), 0o644)
		require.NoError(t, err)
		err = UploadFile(ctx, raw, key, srcFile, "file")
		require.NoError(t, err)

		err = DownloadFile(ctx, raw, key, filepath.Join(t.TempDir(), "dest.txt"), "unsupported")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported artifact type")
	})
}

func TestDeletePrefix(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	ctx := context.Background()

	keys := []string{
		"wf-cleanup/run-123/step1/file1",
		"wf-cleanup/run-123/step2/file2",
		"wf-cleanup/run-123/step3/file3",
		"wf-cleanup/run-456/step1/file4",
	}

	srcFile := filepath.Join(t.TempDir(), "src.txt")
	err = os.WriteFile(srcFile, []byte("test"), 0o644)
	require.NoError(t, err)

	for _, key := range keys {
		err := UploadFile(ctx, raw, key, srcFile, "file")
		require.NoError(t, err)
	}

	err = DeletePrefix(ctx, raw, "wf-cleanup/run-123/")
	require.NoError(t, err)

	for _, key := range keys[:3] {
		exists, err := raw.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists, "key %s should be deleted", key)
	}

	// Keys outside the prefix survive.
	exists, err := raw.Exists(ctx, keys[3])
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestDeletePrefix_EmptyPrefix(t *testing.T) {
	raw, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	err = DeletePrefix(context.Background(), raw, "non-existent-wf/non-existent-run/")
	assert.NoError(t, err)
}
