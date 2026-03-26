package builder

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/workflow/artifacts"
)

// SyncJobBuilder provides a fluent API for constructing Job[T, U].
type SyncJobBuilder[T any, U any] struct {
	name                    string
	source                  datasync.Source[T]
	mapper                  datasync.Mapper[T, U]
	sink                    datasync.Sink[U]
	schedule                time.Duration
	metadata                any
	activityTimeout         time.Duration
	heartbeatTimeout        time.Duration
	maxRetries              int32
	retryInitialInterval    time.Duration
	retryBackoffCoefficient float64
	retryMaxInterval        time.Duration
	artifactConfig          *artifacts.ArtifactConfig
}

// NewSyncJobBuilder creates a new builder with the given job name.
func NewSyncJobBuilder[T, U any](name string) *SyncJobBuilder[T, U] {
	return &SyncJobBuilder[T, U]{name: name}
}

// WithSource sets the data source.
func (b *SyncJobBuilder[T, U]) WithSource(source datasync.Source[T]) *SyncJobBuilder[T, U] {
	b.source = source
	return b
}

// WithMapper sets the data mapper.
func (b *SyncJobBuilder[T, U]) WithMapper(mapper datasync.Mapper[T, U]) *SyncJobBuilder[T, U] {
	b.mapper = mapper
	return b
}

// WithSink sets the data sink.
func (b *SyncJobBuilder[T, U]) WithSink(sink datasync.Sink[U]) *SyncJobBuilder[T, U] {
	b.sink = sink
	return b
}

// WithSchedule sets the sync interval.
func (b *SyncJobBuilder[T, U]) WithSchedule(schedule time.Duration) *SyncJobBuilder[T, U] {
	b.schedule = schedule
	return b
}

// WithMetadata sets domain-specific metadata visible in Temporal UI.
func (b *SyncJobBuilder[T, U]) WithMetadata(metadata any) *SyncJobBuilder[T, U] {
	b.metadata = metadata
	return b
}

// WithActivityTimeout sets the max activity duration.
func (b *SyncJobBuilder[T, U]) WithActivityTimeout(timeout time.Duration) *SyncJobBuilder[T, U] {
	b.activityTimeout = timeout
	return b
}

// WithHeartbeatTimeout sets the heartbeat interval.
func (b *SyncJobBuilder[T, U]) WithHeartbeatTimeout(timeout time.Duration) *SyncJobBuilder[T, U] {
	b.heartbeatTimeout = timeout
	return b
}

// WithMaxRetries sets the max retry attempts.
func (b *SyncJobBuilder[T, U]) WithMaxRetries(retries int32) *SyncJobBuilder[T, U] {
	b.maxRetries = retries
	return b
}

// WithRetryInitialInterval sets the first retry backoff duration.
func (b *SyncJobBuilder[T, U]) WithRetryInitialInterval(interval time.Duration) *SyncJobBuilder[T, U] {
	b.retryInitialInterval = interval
	return b
}

// WithRetryBackoffCoefficient sets the retry backoff multiplier.
func (b *SyncJobBuilder[T, U]) WithRetryBackoffCoefficient(coeff float64) *SyncJobBuilder[T, U] {
	b.retryBackoffCoefficient = coeff
	return b
}

// WithRetryMaxInterval sets the max retry backoff.
func (b *SyncJobBuilder[T, U]) WithRetryMaxInterval(interval time.Duration) *SyncJobBuilder[T, U] {
	b.retryMaxInterval = interval
	return b
}

// WithArtifactConfig sets the artifact storage configuration.
func (b *SyncJobBuilder[T, U]) WithArtifactConfig(config *artifacts.ArtifactConfig) *SyncJobBuilder[T, U] {
	b.artifactConfig = config
	return b
}

// Build validates and returns the Job.
func (b *SyncJobBuilder[T, U]) Build() (datasync.Job[T, U], error) {
	if b.name == "" {
		return datasync.Job[T, U]{}, fmt.Errorf("job name is required")
	}
	if b.source == nil {
		return datasync.Job[T, U]{}, fmt.Errorf("source is required")
	}
	if b.mapper == nil {
		return datasync.Job[T, U]{}, fmt.Errorf("mapper is required")
	}
	if b.sink == nil {
		return datasync.Job[T, U]{}, fmt.Errorf("sink is required")
	}
	if b.schedule <= 0 {
		return datasync.Job[T, U]{}, fmt.Errorf("schedule must be positive")
	}

	return datasync.Job[T, U]{
		Name:                    b.name,
		Source:                  b.source,
		Mapper:                  b.mapper,
		Sink:                    b.sink,
		Schedule:                b.schedule,
		Metadata:                b.metadata,
		ActivityTimeout:         b.activityTimeout,
		HeartbeatTimeout:        b.heartbeatTimeout,
		MaxRetries:              b.maxRetries,
		RetryInitialInterval:    b.retryInitialInterval,
		RetryBackoffCoefficient: b.retryBackoffCoefficient,
		RetryMaxInterval:        b.retryMaxInterval,
		ArtifactConfig:          b.artifactConfig,
	}, nil
}
