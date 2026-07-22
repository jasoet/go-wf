package store

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// storeOtelContext returns a context carrying a minimal OTel config backed by
// real (no-op export) SDK providers, so instrumented paths are exercised.
func storeOtelContext() context.Context {
	cfg := pkgotel.NewConfig("store-test").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

// failingStore is a RawStore whose operations always fail.
type failingStore struct{ err error }

func (f *failingStore) Upload(context.Context, string, io.Reader) error { return f.err }
func (f *failingStore) Download(context.Context, string) (io.ReadCloser, error) {
	return nil, f.err
}
func (f *failingStore) Delete(context.Context, string) error           { return f.err }
func (f *failingStore) Exists(context.Context, string) (bool, error)   { return false, f.err }
func (f *failingStore) List(context.Context, string) ([]string, error) { return nil, f.err }
func (f *failingStore) Close() error                                   { return nil }

func TestInstrumentedStore_WithOTelConfig_Success(t *testing.T) {
	local, err := NewLocalStore(t.TempDir())
	require.NoError(t, err)

	s := NewInstrumentedStore(local)
	ctx := storeOtelContext()
	key := "wf/run/step/data.bin"

	err = s.Upload(ctx, key, bytes.NewReader([]byte("payload")))
	require.NoError(t, err)

	exists, err := s.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	rc, err := s.Download(ctx, key)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	assert.Equal(t, []byte("payload"), got)

	keys, err := s.List(ctx, "wf/run")
	require.NoError(t, err)
	assert.Equal(t, []string{key}, keys)

	require.NoError(t, s.Delete(ctx, key))

	exists, err = s.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestInstrumentedStore_WithOTelConfig_Failures(t *testing.T) {
	boom := errors.New("boom")
	s := NewInstrumentedStore(&failingStore{err: boom})
	ctx := storeOtelContext()

	err := s.Upload(ctx, "k", bytes.NewReader(nil))
	assert.ErrorIs(t, err, boom)

	_, err = s.Download(ctx, "k")
	assert.ErrorIs(t, err, boom)

	err = s.Delete(ctx, "k")
	assert.ErrorIs(t, err, boom)

	_, err = s.Exists(ctx, "k")
	assert.ErrorIs(t, err, boom)

	_, err = s.List(ctx, "k")
	assert.ErrorIs(t, err, boom)

	require.NoError(t, s.Close())
}

func TestRecordStoreMetrics(t *testing.T) {
	t.Run("nil config is a no-op", func(t *testing.T) {
		assert.NotPanics(t, func() {
			recordStoreMetrics(context.Background(), "Upload", "success", time.Second)
		})
	})

	t.Run("with config records without error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			recordStoreMetrics(storeOtelContext(), "Upload", "failure", 250*time.Millisecond)
		})
	})
}
