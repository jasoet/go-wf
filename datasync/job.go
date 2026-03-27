package datasync

import (
	"time"

	"github.com/jasoet/go-wf/workflow/store"
)

// Job defines a complete sync pipeline: source -> mapper -> sink.
type Job[T any, U any] struct {
	Name     string
	Source   Source[T]
	Mapper   Mapper[T, U]
	Sink     Sink[U]
	Schedule time.Duration

	ActivityTimeout         time.Duration
	HeartbeatTimeout        time.Duration
	MaxRetries              int32
	RetryInitialInterval    time.Duration
	RetryBackoffCoefficient float64
	RetryMaxInterval        time.Duration

	Metadata any
	Store    store.RawStore
}
