package datasync

import "time"

// JobRegistration is a type-erased wrapper around Job[T, U] that captures
// job metadata without carrying generic type parameters.
type JobRegistration struct {
	Name       string
	Schedule   time.Duration
	Disabled   bool
	SourceName string
	SinkName   string
	Metadata   any
}

// BuildRegistration extracts type-erased registration info from a typed Job.
func BuildRegistration[T, U any](job Job[T, U], disabled bool) JobRegistration {
	return JobRegistration{
		Name:       job.Name,
		Schedule:   job.Schedule,
		Disabled:   disabled,
		SourceName: job.Source.Name(),
		SinkName:   job.Sink.Name(),
		Metadata:   job.Metadata,
	}
}
