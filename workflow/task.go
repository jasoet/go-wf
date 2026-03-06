package workflow

// TaskInput is the constraint for all workflow task inputs.
type TaskInput interface {
	Validate() error
	ActivityName() string
}

// TaskOutput is the constraint for all workflow task outputs.
type TaskOutput interface {
	IsSuccess() bool
	GetError() string
}
