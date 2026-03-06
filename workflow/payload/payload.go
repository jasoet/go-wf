package payload

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/jasoet/go-wf/workflow"
)

// PipelineInput defines sequential task execution.
type PipelineInput[I workflow.TaskInput] struct {
	Tasks       []I  `json:"tasks" validate:"required,min=1"`
	StopOnError bool `json:"stop_on_error"`
	Cleanup     bool `json:"cleanup"`
}

// Validate validates pipeline input.
func (i *PipelineInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	for idx := range i.Tasks {
		if err := i.Tasks[idx].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// PipelineOutput defines pipeline execution results.
type PipelineOutput[O workflow.TaskOutput] struct {
	Results       []O           `json:"results"`
	TotalSuccess  int           `json:"total_success"`
	TotalFailed   int           `json:"total_failed"`
	TotalDuration time.Duration `json:"total_duration"`
}

// ParallelInput defines parallel task execution.
type ParallelInput[I workflow.TaskInput] struct {
	Tasks           []I    `json:"tasks" validate:"required,min=1"`
	MaxConcurrency  int    `json:"max_concurrency,omitempty"`
	FailureStrategy string `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// Validate validates parallel input.
func (i *ParallelInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	for idx := range i.Tasks {
		if err := i.Tasks[idx].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ParallelOutput defines parallel execution results.
type ParallelOutput[O workflow.TaskOutput] struct {
	Results       []O           `json:"results"`
	TotalSuccess  int           `json:"total_success"`
	TotalFailed   int           `json:"total_failed"`
	TotalDuration time.Duration `json:"total_duration"`
}

// LoopInput defines loop iteration over items.
type LoopInput[I workflow.TaskInput] struct {
	Items           []string `json:"items" validate:"required,min=1"`
	Template        I        `json:"template" validate:"required"`
	Parallel        bool     `json:"parallel"`
	MaxConcurrency  int      `json:"max_concurrency,omitempty"`
	FailureStrategy string   `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// Validate validates loop input.
func (i *LoopInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	return i.Template.Validate()
}

// ParameterizedLoopInput defines loop with multiple parameters.
type ParameterizedLoopInput[I workflow.TaskInput] struct {
	Parameters      map[string][]string `json:"parameters" validate:"required,min=1"`
	Template        I                   `json:"template" validate:"required"`
	Parallel        bool                `json:"parallel"`
	MaxConcurrency  int                 `json:"max_concurrency,omitempty"`
	FailureStrategy string              `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// Validate validates parameterized loop input.
func (i *ParameterizedLoopInput[I]) Validate() error {
	validate := validator.New()
	if err := validate.Struct(i); err != nil {
		return err
	}
	for key, values := range i.Parameters {
		if len(values) == 0 {
			return fmt.Errorf("parameter array '%s' cannot be empty", key)
		}
	}
	return i.Template.Validate()
}

// LoopOutput defines loop execution results.
type LoopOutput[O workflow.TaskOutput] struct {
	Results       []O           `json:"results"`
	TotalSuccess  int           `json:"total_success"`
	TotalFailed   int           `json:"total_failed"`
	TotalDuration time.Duration `json:"total_duration"`
	ItemCount     int           `json:"item_count"`
}
