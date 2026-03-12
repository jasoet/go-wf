package artifacts

import (
	"context"
	"io"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	artifactMeterScope        = "go-wf/artifacts"
	artifactOperationTotal    = "go_wf.artifact.operation.total"
	artifactOperationDuration = "go_wf.artifact.operation.duration"
)

// InstrumentedStore wraps any ArtifactStore with OpenTelemetry spans and metrics.
// When OTel config is not in context, all calls delegate directly to the inner store
// with zero overhead.
type InstrumentedStore struct {
	inner ArtifactStore
}

// NewInstrumentedStore creates a new InstrumentedStore wrapping the given store.
func NewInstrumentedStore(inner ArtifactStore) *InstrumentedStore {
	return &InstrumentedStore{inner: inner}
}

func (s *InstrumentedStore) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Upload(ctx, metadata, data)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Upload",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.type", metadata.Type),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
		pkgotel.F("artifact.step_name", metadata.StepName),
	)
	defer lc.End()

	err := s.inner.Upload(lc.Context(), metadata, data)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "artifact upload failed")
		recordArtifactMetrics(lc.Context(), "Upload", "failure", time.Since(start))
		return err
	}

	lc.Span.AddAttribute("artifact.size", metadata.Size)
	lc.Success("artifact uploaded")
	recordArtifactMetrics(lc.Context(), "Upload", "success", time.Since(start))
	return nil
}

func (s *InstrumentedStore) Download(ctx context.Context, metadata ArtifactMetadata) (io.ReadCloser, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Download(ctx, metadata)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Download",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
		pkgotel.F("artifact.step_name", metadata.StepName),
	)
	defer lc.End()

	reader, err := s.inner.Download(lc.Context(), metadata)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "artifact download failed")
		recordArtifactMetrics(lc.Context(), "Download", "failure", time.Since(start))
		return nil, err
	}

	lc.Success("artifact downloaded")
	recordArtifactMetrics(lc.Context(), "Download", "success", time.Since(start))
	return reader, nil
}

func (s *InstrumentedStore) Delete(ctx context.Context, metadata ArtifactMetadata) error {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Delete(ctx, metadata)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Delete",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
		pkgotel.F("artifact.step_name", metadata.StepName),
	)
	defer lc.End()

	err := s.inner.Delete(lc.Context(), metadata)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "artifact delete failed")
		recordArtifactMetrics(lc.Context(), "Delete", "failure", time.Since(start))
		return err
	}

	lc.Success("artifact deleted")
	recordArtifactMetrics(lc.Context(), "Delete", "success", time.Since(start))
	return nil
}

func (s *InstrumentedStore) Exists(ctx context.Context, metadata ArtifactMetadata) (bool, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.Exists(ctx, metadata)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "Exists",
		pkgotel.F("artifact.name", metadata.Name),
		pkgotel.F("artifact.workflow_id", metadata.WorkflowID),
	)
	defer lc.End()

	exists, err := s.inner.Exists(lc.Context(), metadata)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "artifact exists check failed")
		recordArtifactMetrics(lc.Context(), "Exists", "failure", time.Since(start))
		return false, err
	}

	lc.Span.AddAttribute("artifact.exists", exists)
	lc.Success("artifact exists checked")
	recordArtifactMetrics(lc.Context(), "Exists", "success", time.Since(start))
	return exists, nil
}

func (s *InstrumentedStore) List(ctx context.Context, prefix string) ([]ArtifactMetadata, error) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return s.inner.List(ctx, prefix)
	}

	start := time.Now()

	lc := pkgotel.Layers.StartRepository(ctx, "artifacts", "List",
		pkgotel.F("artifact.prefix", prefix),
	)
	defer lc.End()

	items, err := s.inner.List(lc.Context(), prefix)
	if err != nil {
		//nolint:errcheck,gosec // error is intentionally not used; we return the original error
		lc.Error(err, "artifact list failed")
		recordArtifactMetrics(lc.Context(), "List", "failure", time.Since(start))
		return nil, err
	}

	lc.Span.AddAttribute("artifact.count", len(items))
	lc.Success("artifacts listed")
	recordArtifactMetrics(lc.Context(), "List", "success", time.Since(start))
	return items, nil
}

func (s *InstrumentedStore) Close() error {
	return s.inner.Close()
}

// recordArtifactMetrics records counter and histogram metrics for artifact operations.
func recordArtifactMetrics(ctx context.Context, operation, status string, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(artifactMeterScope)

	counter, err := meter.Int64Counter(artifactOperationTotal,
		metric.WithDescription("Total number of artifact operations"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("operation", operation),
				attribute.String("status", status),
			),
		)
	}

	histogram, err := meter.Float64Histogram(artifactOperationDuration,
		metric.WithDescription("Duration of artifact operations in seconds"),
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
