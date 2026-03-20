package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	pkgotel "github.com/jasoet/pkg/v2/otel"
)

type mockStore struct {
	uploadCalled   bool
	downloadCalled bool
	deleteCalled   bool
	existsCalled   bool
	listCalled     bool
	closeCalled    bool
}

func (m *mockStore) Upload(_ context.Context, _ ArtifactMetadata, _ io.Reader) error {
	m.uploadCalled = true
	return nil
}

func (m *mockStore) Download(_ context.Context, _ ArtifactMetadata) (io.ReadCloser, error) {
	m.downloadCalled = true
	return io.NopCloser(bytes.NewReader([]byte("data"))), nil
}

func (m *mockStore) Delete(_ context.Context, _ ArtifactMetadata) error {
	m.deleteCalled = true
	return nil
}

func (m *mockStore) Exists(_ context.Context, _ ArtifactMetadata) (bool, error) {
	m.existsCalled = true
	return true, nil
}

func (m *mockStore) List(_ context.Context, _ string) ([]ArtifactMetadata, error) {
	m.listCalled = true
	return []ArtifactMetadata{{Name: "test"}}, nil
}

func (m *mockStore) Close() error {
	m.closeCalled = true
	return nil
}

func testMetadata() ArtifactMetadata {
	return ArtifactMetadata{
		Name:       "test-artifact",
		Type:       "file",
		WorkflowID: "wf-123",
		StepName:   "step-1",
	}
}

func TestInstrumentedStore_Upload_NoOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := context.Background()

	err := store.Upload(ctx, testMetadata(), bytes.NewReader([]byte("data")))

	require.NoError(t, err)
	assert.True(t, mock.uploadCalled, "inner Upload should be called")
}

func TestInstrumentedStore_Download_NoOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := context.Background()

	reader, err := store.Download(ctx, testMetadata())

	require.NoError(t, err)
	assert.NotNil(t, reader)
	assert.True(t, mock.downloadCalled, "inner Download should be called")
	reader.Close()
}

func TestInstrumentedStore_Delete_NoOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := context.Background()

	err := store.Delete(ctx, testMetadata())

	require.NoError(t, err)
	assert.True(t, mock.deleteCalled, "inner Delete should be called")
}

func TestInstrumentedStore_Exists_NoOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := context.Background()

	exists, err := store.Exists(ctx, testMetadata())

	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, mock.existsCalled, "inner Exists should be called")
}

func TestInstrumentedStore_List_NoOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := context.Background()

	items, err := store.List(ctx, "prefix/")

	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.True(t, mock.listCalled, "inner List should be called")
}

func TestInstrumentedStore_Close_Delegates(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)

	err := store.Close()

	require.NoError(t, err)
	assert.True(t, mock.closeCalled, "inner Close should be called")
}

func TestRecordArtifactMetrics_NilConfig(t *testing.T) {
	ctx := context.Background()
	assert.NotPanics(t, func() {
		recordArtifactMetrics(ctx, "Upload", "success", time.Second)
	})
}

func TestInstrumentedStore_ImplementsInterface(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	var _ ArtifactStore = store
}

// ---------------------------------------------------------------------------
// Helpers for with-config tests
// ---------------------------------------------------------------------------

func otelContext() context.Context {
	cfg := pkgotel.NewConfig("test-service").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

type errorStore struct {
	err error
}

func (e *errorStore) Upload(_ context.Context, _ ArtifactMetadata, _ io.Reader) error {
	return e.err
}

func (e *errorStore) Download(_ context.Context, _ ArtifactMetadata) (io.ReadCloser, error) {
	return nil, e.err
}

func (e *errorStore) Delete(_ context.Context, _ ArtifactMetadata) error {
	return e.err
}

func (e *errorStore) Exists(_ context.Context, _ ArtifactMetadata) (bool, error) {
	return false, e.err
}

func (e *errorStore) List(_ context.Context, _ string) ([]ArtifactMetadata, error) {
	return nil, e.err
}

func (e *errorStore) Close() error {
	return e.err
}

// ---------------------------------------------------------------------------
// Success path tests with OTel config
// ---------------------------------------------------------------------------

func TestInstrumentedStore_Upload_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	err := store.Upload(ctx, testMetadata(), bytes.NewReader([]byte("data")))

	require.NoError(t, err)
	assert.True(t, mock.uploadCalled, "inner Upload should be called")
}

func TestInstrumentedStore_Download_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	reader, err := store.Download(ctx, testMetadata())

	require.NoError(t, err)
	assert.NotNil(t, reader)
	assert.True(t, mock.downloadCalled, "inner Download should be called")
	reader.Close()
}

func TestInstrumentedStore_Delete_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	err := store.Delete(ctx, testMetadata())

	require.NoError(t, err)
	assert.True(t, mock.deleteCalled, "inner Delete should be called")
}

func TestInstrumentedStore_Exists_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	exists, err := store.Exists(ctx, testMetadata())

	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, mock.existsCalled, "inner Exists should be called")
}

func TestInstrumentedStore_List_WithOTelConfig(t *testing.T) {
	mock := &mockStore{}
	store := NewInstrumentedStore(mock)
	ctx := otelContext()

	items, err := store.List(ctx, "prefix/")

	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.True(t, mock.listCalled, "inner List should be called")
}

// ---------------------------------------------------------------------------
// Error path tests with OTel config
// ---------------------------------------------------------------------------

func TestInstrumentedStore_Upload_WithOTelConfig_Error(t *testing.T) {
	es := &errorStore{err: fmt.Errorf("upload failed")}
	store := NewInstrumentedStore(es)
	ctx := otelContext()

	err := store.Upload(ctx, testMetadata(), bytes.NewReader([]byte("data")))

	require.Error(t, err)
	assert.Equal(t, "upload failed", err.Error())
}

func TestInstrumentedStore_Download_WithOTelConfig_Error(t *testing.T) {
	es := &errorStore{err: fmt.Errorf("download failed")}
	store := NewInstrumentedStore(es)
	ctx := otelContext()

	reader, err := store.Download(ctx, testMetadata())

	require.Error(t, err)
	assert.Nil(t, reader)
	assert.Equal(t, "download failed", err.Error())
}

func TestInstrumentedStore_Delete_WithOTelConfig_Error(t *testing.T) {
	es := &errorStore{err: fmt.Errorf("delete failed")}
	store := NewInstrumentedStore(es)
	ctx := otelContext()

	err := store.Delete(ctx, testMetadata())

	require.Error(t, err)
	assert.Equal(t, "delete failed", err.Error())
}

func TestInstrumentedStore_Exists_WithOTelConfig_Error(t *testing.T) {
	es := &errorStore{err: fmt.Errorf("exists failed")}
	store := NewInstrumentedStore(es)
	ctx := otelContext()

	exists, err := store.Exists(ctx, testMetadata())

	require.Error(t, err)
	assert.False(t, exists)
	assert.Equal(t, "exists failed", err.Error())
}

func TestInstrumentedStore_List_WithOTelConfig_Error(t *testing.T) {
	es := &errorStore{err: fmt.Errorf("list failed")}
	store := NewInstrumentedStore(es)
	ctx := otelContext()

	items, err := store.List(ctx, "prefix/")

	require.Error(t, err)
	assert.Nil(t, items)
	assert.Equal(t, "list failed", err.Error())
}

// ---------------------------------------------------------------------------
// Metrics recording with config
// ---------------------------------------------------------------------------

func TestRecordArtifactMetrics_WithConfig(t *testing.T) {
	operations := []string{"Upload", "Download", "Delete", "Exists", "List"}
	statuses := []string{"success", "failure"}

	for _, op := range operations {
		for _, status := range statuses {
			t.Run(fmt.Sprintf("%s_%s", op, status), func(t *testing.T) {
				ctx := otelContext()
				assert.NotPanics(t, func() {
					recordArtifactMetrics(ctx, op, status, time.Second)
				})
			})
		}
	}
}
