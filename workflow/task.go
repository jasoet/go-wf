package workflow

// TaskInput is the interface constraint that every workflow task input must
// satisfy.  Validate returns an error if the input is invalid, and
// ActivityName returns the Temporal activity name used to dispatch the task.
type TaskInput interface {
	Validate() error
	ActivityName() string
}

// TaskOutput is the interface constraint that every workflow task output must
// satisfy.  IsSuccess reports whether the task completed successfully, and
// GetError returns a human-readable error description (empty on success).
type TaskOutput interface {
	IsSuccess() bool
	GetError() string
}
