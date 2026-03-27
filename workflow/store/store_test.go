package store

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Codec Tests ---

func TestJSONCodec_RoundTrip(t *testing.T) {
	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	codec := &JSONCodec[sample]{}
	original := sample{Name: "test", Value: 42}

	reader, err := codec.Encode(original)
	require.NoError(t, err)

	decoded, err := codec.Decode(reader)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestJSONCodec_RoundTrip_Slice(t *testing.T) {
	codec := &JSONCodec[[]string]{}
	original := []string{"alpha", "beta", "gamma"}

	reader, err := codec.Encode(original)
	require.NoError(t, err)

	decoded, err := codec.Decode(reader)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestJSONCodec_RoundTrip_Map(t *testing.T) {
	codec := &JSONCodec[map[string]int]{}
	original := map[string]int{"a": 1, "b": 2}

	reader, err := codec.Encode(original)
	require.NoError(t, err)

	decoded, err := codec.Decode(reader)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestJSONCodec_DecodeError(t *testing.T) {
	codec := &JSONCodec[int]{}
	reader := bytes.NewReader([]byte("not-json"))

	_, err := codec.Decode(reader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json decode")
}

func TestBytesCodec_RoundTrip(t *testing.T) {
	codec := &BytesCodec{}
	original := []byte("hello, world!")

	reader, err := codec.Encode(original)
	require.NoError(t, err)

	decoded, err := codec.Decode(reader)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestBytesCodec_EmptyData(t *testing.T) {
	codec := &BytesCodec{}
	original := []byte{}

	reader, err := codec.Encode(original)
	require.NoError(t, err)

	decoded, err := codec.Decode(reader)
	require.NoError(t, err)
	assert.Empty(t, decoded)
}

// --- KeyBuilder Tests ---

func TestKeyBuilder_FullKey(t *testing.T) {
	key := NewKeyBuilder().
		WithWorkflow("wf-123").
		WithRun("run-456").
		WithStep("process").
		WithName("output.json").
		Build()

	assert.Equal(t, "wf-123/run-456/process/output.json", key)
}

func TestKeyBuilder_PartialKey(t *testing.T) {
	key := NewKeyBuilder().
		WithWorkflow("wf-123").
		WithRun("run-456").
		Build()

	assert.Equal(t, "wf-123/run-456", key)
}

func TestKeyBuilder_SinglePart(t *testing.T) {
	key := NewKeyBuilder().
		WithWorkflow("wf-123").
		Build()

	assert.Equal(t, "wf-123", key)
}

func TestKeyBuilder_Empty(t *testing.T) {
	key := NewKeyBuilder().Build()
	assert.Equal(t, "", key)
}

// --- LocalStore Tests ---

func TestLocalStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	ctx := context.Background()
	key := "workflow/run/step/data.bin"
	data := []byte("hello store")

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
}

func TestLocalStore_DeleteNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	err = s.Delete(context.Background(), "nonexistent/key")
	assert.NoError(t, err)
}

func TestLocalStore_DownloadNonExistent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	_, err = s.Download(context.Background(), "nonexistent/key")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key not found")
}

func TestLocalStore_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	keys, err := s.List(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, keys)
}

func TestLocalStore_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	ctx := context.Background()

	tests := []struct {
		name string
		key  string
	}{
		{"dotdot", "../escape/file"},
		{"middle dotdot", "a/../../escape"},
		{"null byte", "a/b\x00c"},
		{"empty key", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Upload(ctx, tt.key, bytes.NewReader([]byte("bad")))
			assert.Error(t, err)

			_, err = s.Download(ctx, tt.key)
			assert.Error(t, err)

			_, err = s.Exists(ctx, tt.key)
			assert.Error(t, err)

			err = s.Delete(ctx, tt.key)
			assert.Error(t, err)
		})
	}
}

func TestLocalStore_ListPathTraversal(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	_, err = s.List(context.Background(), "../escape")
	assert.Error(t, err)
}

func TestLocalStore_Close(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	assert.NoError(t, s.Close())
}

func TestLocalStore_ListMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	ctx := context.Background()

	keys := []string{
		"wf/run/step/a.json",
		"wf/run/step/b.json",
		"wf/run/other/c.json",
	}

	for _, key := range keys {
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

// --- TypedStore + LocalStore Integration Tests ---

func TestTypedStore_JSONRoundTrip(t *testing.T) {
	type record struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	dir := t.TempDir()
	raw, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewJSONStore[record](raw)
	ctx := context.Background()
	key := "wf/run/step/record.json"

	original := record{ID: 1, Name: "test-record"}

	err = s.Save(ctx, key, original)
	require.NoError(t, err)

	loaded, err := s.Load(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, original, loaded)

	exists, err := s.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	keys, err := s.List(ctx, "wf/run/step")
	require.NoError(t, err)
	assert.Equal(t, []string{"wf/run/step/record.json"}, keys)

	err = s.Delete(ctx, key)
	require.NoError(t, err)

	exists, err = s.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, s.Close())
}

func TestTypedStore_BytesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	raw, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewBytesStore(raw)
	ctx := context.Background()
	key := "wf/run/step/data.bin"
	data := []byte("binary content here")

	err = s.Save(ctx, key, data)
	require.NoError(t, err)

	loaded, err := s.Load(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, data, loaded)
}

func TestTypedStore_WithKeyBuilder(t *testing.T) {
	type config struct {
		Enabled bool   `json:"enabled"`
		Mode    string `json:"mode"`
	}

	dir := t.TempDir()
	raw, err := NewLocalStore(dir)
	require.NoError(t, err)

	s := NewJSONStore[config](raw)
	ctx := context.Background()

	key := NewKeyBuilder().
		WithWorkflow("sync-workflow").
		WithRun("run-001").
		WithStep("transform").
		WithName("config.json").
		Build()

	original := config{Enabled: true, Mode: "full"}

	err = s.Save(ctx, key, original)
	require.NoError(t, err)

	loaded, err := s.Load(ctx, key)
	require.NoError(t, err)
	assert.Equal(t, original, loaded)
}

// --- KeyBuilder Branching Safety Test ---

func TestKeyBuilder_BranchingSafety(t *testing.T) {
	base := NewKeyBuilder().WithWorkflow("wf-1").WithRun("run-1")

	key1 := base.WithStep("step-a").Build()
	key2 := base.WithStep("step-b").Build()

	assert.Equal(t, "wf-1/run-1/step-a", key1)
	assert.Equal(t, "wf-1/run-1/step-b", key2)
}

// --- Upload Size Limit Test ---

func TestLocalStore_UploadExceedsMaxSize(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a reader that reports more data than MaxUploadSize.
	// We use a LimitReader of zeros slightly over the limit to avoid allocating 1GB+.
	overSize := int64(MaxUploadSize + 1)
	data := io.LimitReader(zeroReader{}, overSize)

	err = s.Upload(ctx, "too/large/file", data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data exceeds maximum upload size of 1GB")
}

func TestLocalStore_UploadExactMaxSize(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocalStore(dir)
	require.NoError(t, err)

	ctx := context.Background()

	// Exactly MaxUploadSize should succeed.
	data := io.LimitReader(zeroReader{}, MaxUploadSize)

	err = s.Upload(ctx, "exact/size/file", data)
	require.NoError(t, err)
}

// zeroReader is an io.Reader that produces an infinite stream of zero bytes.
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
