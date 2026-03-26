package payload

import (
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/jasoet/go-wf/workflow"
)

// Compile-time interface checks.
var (
	_ workflow.TaskInput  = (*SyncExecutionInput)(nil)
	_ workflow.TaskOutput = SyncExecutionOutput{}
)

var pkgValidator = validator.New()

// SyncExecutionInput defines input for sync workflow execution.
type SyncExecutionInput struct {
	JobName    string `json:"jobName" validate:"required,max=255"`
	SourceName string `json:"sourceName" validate:"required,max=255"`
	SinkName   string `json:"sinkName" validate:"required,max=255"`
	Metadata   any    `json:"metadata,omitempty"`
}

func (s *SyncExecutionInput) Validate() error {
	return pkgValidator.Struct(s)
}

func (s *SyncExecutionInput) ActivityName() string {
	return s.JobName + ".SyncData"
}

// SyncExecutionOutput defines output from sync workflow execution.
type SyncExecutionOutput struct {
	JobName        string        `json:"jobName"`
	TotalFetched   int           `json:"totalFetched"`
	Inserted       int           `json:"inserted"`
	Updated        int           `json:"updated"`
	Skipped        int           `json:"skipped"`
	ProcessingTime time.Duration `json:"processingTime"`
	Success        bool          `json:"success"`
	Error          string        `json:"error,omitempty"`
}

func (s SyncExecutionOutput) IsSuccess() bool  { return s.Success }
func (s SyncExecutionOutput) GetError() string { return s.Error }
