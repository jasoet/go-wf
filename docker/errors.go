package docker

import "fmt"

// ErrorType represents different types of errors.
type ErrorType string

const (
	// ErrorTypeValidation indicates validation errors
	ErrorTypeValidation ErrorType = "validation"

	// ErrorTypeExecution indicates execution errors
	ErrorTypeExecution ErrorType = "execution"

	// ErrorTypeTimeout indicates timeout errors
	ErrorTypeTimeout ErrorType = "timeout"

	// ErrorTypeConfiguration indicates configuration errors
	ErrorTypeConfiguration ErrorType = "configuration"
)

// WorkflowError represents a Docker workflow error.
type WorkflowError struct {
	Type    ErrorType
	Message string
	Err     error
}

// Error implements error interface.
func (e *WorkflowError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Type, e.Message)
}

// Unwrap returns the wrapped error.
func (e *WorkflowError) Unwrap() error {
	return e.Err
}

// Wrap wraps an error with additional context.
func (e *WorkflowError) Wrap(msg string) *WorkflowError {
	return &WorkflowError{
		Type:    e.Type,
		Message: msg,
		Err:     e,
	}
}

// Predefined errors
var (
	// ErrInvalidInput indicates invalid input provided
	ErrInvalidInput = &WorkflowError{
		Type:    ErrorTypeValidation,
		Message: "invalid input",
	}

	// ErrExecutionFailed indicates workflow execution failed
	ErrExecutionFailed = &WorkflowError{
		Type:    ErrorTypeExecution,
		Message: "execution failed",
	}

	// ErrTimeout indicates operation timed out
	ErrTimeout = &WorkflowError{
		Type:    ErrorTypeTimeout,
		Message: "operation timed out",
	}

	// ErrInvalidConfiguration indicates invalid configuration
	ErrInvalidConfiguration = &WorkflowError{
		Type:    ErrorTypeConfiguration,
		Message: "invalid configuration",
	}
)

// NewValidationError creates a new validation error.
func NewValidationError(msg string, err error) *WorkflowError {
	return &WorkflowError{
		Type:    ErrorTypeValidation,
		Message: msg,
		Err:     err,
	}
}

// NewExecutionError creates a new execution error.
func NewExecutionError(msg string, err error) *WorkflowError {
	return &WorkflowError{
		Type:    ErrorTypeExecution,
		Message: msg,
		Err:     err,
	}
}
