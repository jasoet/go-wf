package store

import (
	"context"
	"io"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	storeMeterScope        = "go-wf/store"
	storeOperationTotal    = "go_wf.store.operation.total"
	storeOperationDuration = "go_wf.store.operation.duration"
)

// InstrumentedStore wraps any RawStore with OpenTelemetry spans and metrics.
// When OTel config is not in context, all calls delegate directly to the inner store
// with zero overhead.
type InstrumentedStore struct {
	inner RawStore
}

// NewInstrumentedStore creates a new InstrumentedStore wrapping the given store.
func NewInstrumentedStore(inner RawStore) RawStore {
	return &InstrumentedStore{inner: inner}
}

func (s *InstrumentedStore) Upload(ctx context.Context, key string, data io.Reader) error {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Upload(ctx, key, data)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "store", "Upload",
		pkgotel.F("store.key", key),
	)
	defer lc.End()

	err := s.inner.Upload(lc.Context(), key, data)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "store upload failed")
		recordStoreMetrics(lc.Context(), "Upload", "failure", time.Since(start))
		return err
	}

	lc.Success("store upload completed")
	recordStoreMetrics(lc.Context(), "Upload", "success", time.Since(start))
	return nil
}

func (s *InstrumentedStore) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Download(ctx, key)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "store", "Download",
		pkgotel.F("store.key", key),
	)
	defer lc.End()

	reader, err := s.inner.Download(lc.Context(), key)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "store download failed")
		recordStoreMetrics(lc.Context(), "Download", "failure", time.Since(start))
		return nil, err
	}

	lc.Success("store download completed")
	recordStoreMetrics(lc.Context(), "Download", "success", time.Since(start))
	return reader, nil
}

func (s *InstrumentedStore) Delete(ctx context.Context, key string) error {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Delete(ctx, key)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "store", "Delete",
		pkgotel.F("store.key", key),
	)
	defer lc.End()

	err := s.inner.Delete(lc.Context(), key)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "store delete failed")
		recordStoreMetrics(lc.Context(), "Delete", "failure", time.Since(start))
		return err
	}

	lc.Success("store delete completed")
	recordStoreMetrics(lc.Context(), "Delete", "success", time.Since(start))
	return nil
}

func (s *InstrumentedStore) Exists(ctx context.Context, key string) (bool, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Exists(ctx, key)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "store", "Exists",
		pkgotel.F("store.key", key),
	)
	defer lc.End()

	exists, err := s.inner.Exists(lc.Context(), key)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "store exists check failed")
		recordStoreMetrics(lc.Context(), "Exists", "failure", time.Since(start))
		return false, err
	}

	lc.Span.AddAttribute("store.exists", exists)
	lc.Success("store exists checked")
	recordStoreMetrics(lc.Context(), "Exists", "success", time.Since(start))
	return exists, nil
}

func (s *InstrumentedStore) List(ctx context.Context, prefix string) ([]string, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.List(ctx, prefix)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "store", "List",
		pkgotel.F("store.prefix", prefix),
	)
	defer lc.End()

	keys, err := s.inner.List(lc.Context(), prefix)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "store list failed")
		recordStoreMetrics(lc.Context(), "List", "failure", time.Since(start))
		return nil, err
	}

	lc.Span.AddAttribute("store.count", len(keys))
	lc.Success("store keys listed")
	recordStoreMetrics(lc.Context(), "List", "success", time.Since(start))
	return keys, nil
}

func (s *InstrumentedStore) Close() error {
	return s.inner.Close()
}

// recordStoreMetrics records counter and histogram metrics for store operations.
func recordStoreMetrics(ctx context.Context, operation, status string, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(storeMeterScope)

	counter, err := meter.Int64Counter(storeOperationTotal,
		metric.WithDescription("Total number of store operations"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", operation),
				attribute.String("status", status),
			),
		)
	}

	histogram, err := meter.Float64Histogram(storeOperationDuration,
		metric.WithDescription("Duration of store operations in seconds"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("operation", operation),
			),
		)
	}
}
