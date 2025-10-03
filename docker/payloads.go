package docker

import (
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
