package store

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstrumentedStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewInstrumentedStore(local)
	ctx := context.Background()
	key := "workflow/run/step/data.bin"
	data := []byte("instrumented store test")

	// Upload
	err = s.Upload(ctx, key, bytes.NewReader(data))
	require.NoError(t, err)

	// Exists
	exists, err := s.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	// Download
	rc, err := s.Download(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, data, got)

	// List
	keys, err := s.List(ctx, "workflow/run/step")
	require.NoError(t, err)
	assert.Equal(t, []string{"workflow/run/step/data.bin"}, keys)

	// Delete
	err = s.Delete(ctx, key)
	require.NoError(t, err)

	// Exists after delete
	exists, err = s.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	// Close
	require.NoError(t, s.Close())
}

func TestInstrumentedStore_DownloadNonExistent(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewInstrumentedStore(local)

	_, err = s.Download(context.Background(), "nonexistent/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestInstrumentedStore_DeleteNonExistent(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewInstrumentedStore(local)

	err = s.Delete(context.Background(), "nonexistent/key")
	assert.NoError(t, err)
}

func TestInstrumentedStore_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewInstrumentedStore(local)

	keys, err := s.List(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, keys)
}

func TestInstrumentedStore_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewInstrumentedStore(local)
	ctx := context.Background()

	testKeys := []string{
		"wf/run/step/a.json",
		"wf/run/step/b.json",
		"wf/run/other/c.json",
	}

	for _, key := range testKeys {
		err := s.Upload(ctx, key, bytes.NewReader([]byte("data")))
		require.NoError(t, err)
	}

	// List with step prefix
	result, err := s.List(ctx, "wf/run/step")
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "wf/run/step/a.json")
	assert.Contains(t, result, "wf/run/step/b.json")

	// List with broader prefix
	result, err = s.List(ctx, "wf/run")
	require.NoError(t, err)
	assert.Len(t, result, 3)
}

func TestInstrumentedStore_ImplementsRawStore(t *testing.T) {
	dir := t.TempDir()
	local, err := NewLocalStore(dir)
	require.NoError(t, err)

	var _ RawStore = NewInstrumentedStore(local) //nolint:staticcheck // compile-time interface check
}
