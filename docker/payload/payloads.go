package payload

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
)

// ContainerExecutionInput defines input for single container execution.
type ContainerExecutionInput struct {
	// Required fields
	Image string `json:"image" validate:"required"`

	// Optional configuration
	Command    []string          `json:"command,omitempty"`
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Ports      []string          `json:"ports,omitempty"`
	Volumes    map[string]string `json:"volumes,omitempty"`
	WorkDir    string            `json:"work_dir,omitempty"`
	User       string            `json:"user,omitempty"`

	// Wait strategy
	WaitStrategy WaitStrategyConfig `json:"wait_strategy,omitempty"`

	// Timeouts
	StartTimeout time.Duration `json:"start_timeout,omitempty"`
	RunTimeout   time.Duration `json:"run_timeout,omitempty"`

	// Cleanup
	AutoRemove bool `json:"auto_remove"`

	// Metadata
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

// WaitStrategyConfig defines container readiness check.
type WaitStrategyConfig struct {
	Type           string        `json:"type" validate:"oneof='' log port http healthy"`
	LogMessage     string        `json:"log_message,omitempty"`
	Port           string        `json:"port,omitempty"`
	HTTPPath       string        `json:"http_path,omitempty"`
	HTTPStatus     int           `json:"http_status,omitempty"`
	StartupTimeout time.Duration `json:"startup_timeout,omitempty"`
}

// ContainerExecutionOutput defines output from container execution.
type ContainerExecutionOutput struct {
	ContainerID string            `json:"container_id"`
	Name        string            `json:"name,omitempty"`
	ExitCode    int               `json:"exit_code"`
	Stdout      string            `json:"stdout,omitempty"`
	Stderr      string            `json:"stderr,omitempty"`
	Endpoint    string            `json:"endpoint,omitempty"`
	Ports       map[string]string `json:"ports,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  time.Time         `json:"finished_at"`
	Duration    time.Duration     `json:"duration"`
	Success     bool              `json:"success"`
	Error       string            `json:"error,omitempty"`
}

// PipelineInput defines sequential container execution.
type PipelineInput struct {
	Containers  []ContainerExecutionInput `json:"containers" validate:"required,min=1"`
	StopOnError bool                      `json:"stop_on_error"`
	Cleanup     bool                      `json:"cleanup"` // Cleanup after each step
}

// PipelineOutput defines pipeline execution results.
type PipelineOutput struct {
	Results       []ContainerExecutionOutput `json:"results"`
	TotalSuccess  int                        `json:"total_success"`
	TotalFailed   int                        `json:"total_failed"`
	TotalDuration time.Duration              `json:"total_duration"`
}

// ParallelInput defines parallel container execution.
type ParallelInput struct {
	Containers      []ContainerExecutionInput `json:"containers" validate:"required,min=1"`
	MaxConcurrency  int                       `json:"max_concurrency,omitempty"` // 0 = unlimited
	FailureStrategy string                    `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// ParallelOutput defines parallel execution results.
type ParallelOutput struct {
	Results       []ContainerExecutionOutput `json:"results"`
	TotalSuccess  int                        `json:"total_success"`
	TotalFailed   int                        `json:"total_failed"`
	TotalDuration time.Duration              `json:"total_duration"`
}

// Validate validates input using struct tags.
func (i *ContainerExecutionInput) Validate() error {
	validate := validator.New()
	return validate.Struct(i)
}

// Validate validates pipeline input using struct tags.
func (i *PipelineInput) Validate() error {
	validate := validator.New()
	return validate.Struct(i)
}

// Validate validates parallel input using struct tags.
func (i *ParallelInput) Validate() error {
	validate := validator.New()
	return validate.Struct(i)
}

// LoopInput defines loop iteration over items (withItems pattern).
type LoopInput struct {
	// Items to iterate over
	Items []string `json:"items" validate:"required,min=1"`

	// Template container to execute for each item
	Template ContainerExecutionInput `json:"template" validate:"required"`

	// Parallel execution mode
	Parallel bool `json:"parallel"`

	// MaxConcurrency limits parallel executions (0 = unlimited)
	MaxConcurrency int `json:"max_concurrency,omitempty"`

	// FailureStrategy determines how to handle failures
	FailureStrategy string `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// ParameterizedLoopInput defines loop iteration with multiple parameters (withParam pattern).
type ParameterizedLoopInput struct {
	// Parameters contains multiple parameter arrays to iterate over
	// The loop will create the cartesian product of all parameters
	Parameters map[string][]string `json:"parameters" validate:"required,min=1"`

	// Template container to execute for each parameter combination
	Template ContainerExecutionInput `json:"template" validate:"required"`

	// Parallel execution mode
	Parallel bool `json:"parallel"`

	// MaxConcurrency limits parallel executions (0 = unlimited)
	MaxConcurrency int `json:"max_concurrency,omitempty"`

	// FailureStrategy determines how to handle failures
	FailureStrategy string `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// LoopOutput defines loop execution results.
type LoopOutput struct {
	Results       []ContainerExecutionOutput `json:"results"`
	TotalSuccess  int                        `json:"total_success"`
	TotalFailed   int                        `json:"total_failed"`
	TotalDuration time.Duration              `json:"total_duration"`
	ItemCount     int                        `json:"item_count"`
}

// Validate validates loop input using struct tags.
func (i *LoopInput) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	return i.Template.Validate()
}

// Validate validates parameterized loop input using struct tags.
func (i *ParameterizedLoopInput) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}

	// Ensure at least one parameter array exists
	if len(i.Parameters) == 0 {
		return fmt.Errorf("at least one parameter array is required")
	}

	// Ensure all parameter arrays are non-empty
	for key, values := range i.Parameters {
		if len(values) == 0 {
			return fmt.Errorf("parameter array '%s' cannot be empty", key)
		}
	}

	return i.Template.Validate()
}
