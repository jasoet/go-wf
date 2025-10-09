package docker

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflowError(t *testing.T) {
	tests := []struct {
		name     string
		err      *WorkflowError
		expected string
	}{
		{
			name: "error with message",
			err: &WorkflowError{
				Type:    ErrorTypeValidation,
				Message: "invalid input provided",
			},
			expected: "[validation] invalid input provided",
		},
		{
			name: "error with wrapped error",
			err: &WorkflowError{
				Type:    ErrorTypeExecution,
				Message: "execution failed",
				Err:     errors.New("container crashed"),
			},
			expected: "[execution] execution failed: container crashed",
		},
		{
			name: "error without message",
			err: &WorkflowError{
				Type:    ErrorTypeTimeout,
				Message: "",
			},
			expected: "[timeout] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestWorkflowErrorUnwrap(t *testing.T) {
	baseErr := errors.New("base error")
	wfErr := &WorkflowError{
		Type:    ErrorTypeExecution,
		Message: "wrapped",
		Err:     baseErr,
	}

	unwrapped := wfErr.Unwrap()
	assert.Equal(t, baseErr, unwrapped)
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name string
		err  *WorkflowError
		typ  ErrorType
	}{
		{
			name: "invalid input error",
			err:  ErrInvalidInput,
			typ:  ErrorTypeValidation,
		},
		{
			name: "execution failed error",
			err:  ErrExecutionFailed,
			typ:  ErrorTypeExecution,
		},
		{
			name: "timeout error",
			err:  ErrTimeout,
			typ:  ErrorTypeTimeout,
		},
		{
			name: "invalid configuration error",
			err:  ErrInvalidConfiguration,
			typ:  ErrorTypeConfiguration,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.typ, tt.err.Type)
		})
	}
}

func TestErrorWrap(t *testing.T) {
	wrappedErr := ErrInvalidInput.Wrap("validation failed")
	assert.Contains(t, wrappedErr.Error(), "validation failed")
	assert.Contains(t, wrappedErr.Error(), "invalid input")
	assert.Equal(t, ErrorTypeValidation, wrappedErr.Type)
	assert.NotNil(t, wrappedErr.Unwrap())
}

func TestNewValidationError(t *testing.T) {
	baseErr := errors.New("base error")
	err := NewValidationError("validation failed", baseErr)

	assert.Equal(t, ErrorTypeValidation, err.Type)
	assert.Equal(t, "validation failed", err.Message)
	assert.Equal(t, baseErr, err.Err)
}

func TestNewExecutionError(t *testing.T) {
	baseErr := errors.New("execution error")
	err := NewExecutionError("container failed", baseErr)

	assert.Equal(t, ErrorTypeExecution, err.Type)
	assert.Equal(t, "container failed", err.Message)
	assert.Equal(t, baseErr, err.Err)
}
