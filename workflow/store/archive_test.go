package store

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	testFile1 := filepath.Join(tmpDir, "file1.txt")
	testFile2 := filepath.Join(tmpDir, "subdir", "file2.txt")

	err := os.WriteFile(testFile1, []byte("content1"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Dir(testFile2), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("content2"), 0o644)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ArchiveDirectory(tmpDir, &buf)
	require.NoError(t, err)
	assert.Greater(t, buf.Len(), 0)
}

func TestExtractArchive(t *testing.T) {
	srcDir := t.TempDir()
	testFile1 := filepath.Join(srcDir, "file1.txt")
	testFile2 := filepath.Join(srcDir, "subdir", "file2.txt")

	err := os.WriteFile(testFile1, []byte("content1"), 0o644)
	require.NoError(t, err)

	err = os.MkdirAll(filepath.Dir(testFile2), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(testFile2, []byte("content2"), 0o644)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ArchiveDirectory(srcDir, &buf)
	require.NoError(t, err)

	destDir := t.TempDir()
	err = ExtractArchive(&buf, destDir)
	require.NoError(t, err)

	content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content1"), content1)

	content2, err := os.ReadFile(filepath.Join(destDir, "subdir", "file2.txt"))
	require.NoError(t, err)
	assert.Equal(t, []byte("content2"), content2)
}

func TestExtractArchive_InvalidGzip(t *testing.T) {
	destDir := t.TempDir()
	err := ExtractArchive(bytes.NewReader([]byte("not gzip data")), destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create gzip reader")
}

func TestExtractArchive_DirectoryTraversal(t *testing.T) {
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gzWriter)

	header := &tar.Header{
		Name: "../../etc/passwd",
		Mode: 0o600,
		Size: 4,
	}
	err := tarWriter.WriteHeader(header)
	require.NoError(t, err)
	_, err = tarWriter.Write([]byte("evil"))
	require.NoError(t, err)

	require.NoError(t, tarWriter.Close())
	require.NoError(t, gzWriter.Close())

	destDir := t.TempDir()
	err = ExtractArchive(&buf, destDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "illegal file path")
}

func TestArchiveDirectory_NonExistentSource(t *testing.T) {
	var buf bytes.Buffer
	err := ArchiveDirectory("/non/existent/directory", &buf)
	assert.Error(t, err)
}

func TestArchiveDirectory_SkipsSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "regular.txt"), []byte("content"), 0o644)
	require.NoError(t, err)

	symPath := filepath.Join(tmpDir, "link.txt")
	err = os.Symlink("/etc/hostname", symPath)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = ArchiveDirectory(tmpDir, &buf)
	require.NoError(t, err)

	destDir := t.TempDir()
	err = ExtractArchive(&buf, destDir)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "regular.txt"))
	assert.NoError(t, err)

	_, err = os.Stat(filepath.Join(destDir, "link.txt"))
	assert.True(t, os.IsNotExist(err))
}
