package payload

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/jasoet/go-wf/workflow"
)

// Compile-time interface checks.
var (
	_ workflow.TaskInput  = (*FunctionExecutionInput)(nil)
	_ workflow.TaskOutput = FunctionExecutionOutput{}
)

// pkgValidator is a package-level validator instance to avoid repeated instantiation.
var pkgValidator = validator.New()

// safeFunctionName restricts function names to safe characters.
var safeFunctionName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

const functionActivityName = "ExecuteFunctionActivity"

// FunctionExecutionInput defines input for single function execution.
type FunctionExecutionInput struct {
	Name    string            `json:"name" validate:"required,max=255"`
	Args    map[string]string `json:"args,omitempty"`
	Data    []byte            `json:"data,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"work_dir,omitempty"`
	Timeout time.Duration     `json:"timeout,omitempty"` // Reserved for future use; not enforced by the activity.
	Labels  map[string]string `json:"labels,omitempty"`
}

// FunctionExecutionOutput defines output from function execution.
type FunctionExecutionOutput struct {
	Name       string            `json:"name"`
	Success    bool              `json:"success"`
	Error      string            `json:"error,omitempty"`
	Result     map[string]string `json:"result,omitempty"`
	Data       []byte            `json:"data,omitempty"`
	Duration   time.Duration     `json:"duration"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt time.Time         `json:"finished_at"`
}

// PipelineInput defines sequential function execution.
type PipelineInput struct {
	Functions   []FunctionExecutionInput `json:"functions" validate:"required,min=1"`
	StopOnError bool                     `json:"stop_on_error"`
}

// PipelineOutput defines pipeline execution results.
type PipelineOutput struct {
	Results       []FunctionExecutionOutput `json:"results"`
	TotalSuccess  int                       `json:"total_success"`
	TotalFailed   int                       `json:"total_failed"`
	TotalDuration time.Duration             `json:"total_duration"`
}

// ParallelInput defines parallel function execution.
type ParallelInput struct {
	Functions       []FunctionExecutionInput `json:"functions" validate:"required,min=1"`
	MaxConcurrency  int                      `json:"max_concurrency,omitempty"`
	FailureStrategy string                   `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// ParallelOutput defines parallel execution results.
type ParallelOutput struct {
	Results       []FunctionExecutionOutput `json:"results"`
	TotalSuccess  int                       `json:"total_success"`
	TotalFailed   int                       `json:"total_failed"`
	TotalDuration time.Duration             `json:"total_duration"`
}

// LoopInput defines loop iteration over items.
type LoopInput struct {
	Items           []string               `json:"items" validate:"required,min=1"`
	Template        FunctionExecutionInput `json:"template" validate:"required"`
	Parallel        bool                   `json:"parallel"`
	MaxConcurrency  int                    `json:"max_concurrency,omitempty"`
	FailureStrategy string                 `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// ParameterizedLoopInput defines loop iteration with multiple parameters.
type ParameterizedLoopInput struct {
	Parameters      map[string][]string    `json:"parameters" validate:"required,min=1"`
	Template        FunctionExecutionInput `json:"template" validate:"required"`
	Parallel        bool                   `json:"parallel"`
	MaxConcurrency  int                    `json:"max_concurrency,omitempty"`
	FailureStrategy string                 `json:"failure_strategy" validate:"oneof='' continue fail_fast"`
}

// LoopOutput defines loop execution results.
type LoopOutput struct {
	Results       []FunctionExecutionOutput `json:"results"`
	TotalSuccess  int                       `json:"total_success"`
	TotalFailed   int                       `json:"total_failed"`
	TotalDuration time.Duration             `json:"total_duration"`
	ItemCount     int                       `json:"item_count"`
}

// Validate validates input using struct tags and function name format.
func (i *FunctionExecutionInput) Validate() error {
	if err := pkgValidator.Struct(i); err != nil {
		return err
	}
	// Skip regex check for template names (containing {{...}} placeholders);
	// the substituted name will be validated when the activity executes.
	if !strings.Contains(i.Name, "{{") && !safeFunctionName.MatchString(i.Name) {
		return fmt.Errorf("invalid function name: must match [a-zA-Z][a-zA-Z0-9_-]*")
	}
	return nil
}

// ActivityName returns the Temporal activity name for function execution.
func (i *FunctionExecutionInput) ActivityName() string {
	return functionActivityName
}

// IsSuccess returns whether the function executed successfully.
func (o FunctionExecutionOutput) IsSuccess() bool {
	return o.Success
}

// GetError returns the error message from function execution.
func (o FunctionExecutionOutput) GetError() string {
	return o.Error
}

// Validate validates pipeline input using struct tags.
func (i *PipelineInput) Validate() error {
	return pkgValidator.Struct(i)
}

// Validate validates parallel input using struct tags.
func (i *ParallelInput) Validate() error {
	return pkgValidator.Struct(i)
}

// Validate validates loop input using struct tags.
func (i *LoopInput) Validate() error {
	if err := pkgValidator.Struct(i); err != nil {
		return err
	}
	return i.Template.Validate()
}

// Validate validates parameterized loop input using struct tags.
func (i *ParameterizedLoopInput) Validate() error {
	if err := pkgValidator.Struct(i); err != nil {
		return err
	}

	if len(i.Parameters) == 0 {
		return fmt.Errorf("at least one parameter array is required")
	}

	for key, values := range i.Parameters {
		if len(values) == 0 {
			return fmt.Errorf("parameter array '%s' cannot be empty", key)
		}
	}

	return i.Template.Validate()
}
