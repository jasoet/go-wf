package artifacts

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
