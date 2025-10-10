package client

import (
	"context"
	"fmt"
	"time"

	"github.com/jasoet/go-wf/docker"
	wf "github.com/jasoet/go-wf/docker/workflow"
	"go.temporal.io/sdk/client"
)

// WorkflowStatus represents the status of a workflow execution.
type WorkflowStatus struct {
	WorkflowID string
	RunID      string
	Status     string
	StartTime  time.Time
	CloseTime  *time.Time
	Result     interface{}
	Error      error
}

// SubmitWorkflow submits a container workflow for execution.
//
// Example:
//
//	status, err := docker.SubmitWorkflow(ctx, temporalClient, input, "docker-queue")
func SubmitWorkflow(ctx context.Context, c client.Client, input interface{}, taskQueue string) (*WorkflowStatus, error) {
	workflowID := fmt.Sprintf("docker-workflow-%d", time.Now().Unix())

	var workflowFunc interface{}
	switch v := input.(type) {
	case docker.ContainerExecutionInput:
		workflowFunc = wf.ExecuteContainerWorkflow
	case *docker.ContainerExecutionInput:
		workflowFunc = wf.ExecuteContainerWorkflow
	case docker.PipelineInput:
		workflowFunc = wf.ContainerPipelineWorkflow
	case *docker.PipelineInput:
		workflowFunc = wf.ContainerPipelineWorkflow
	case docker.ParallelInput:
		workflowFunc = wf.ParallelContainersWorkflow
	case *docker.ParallelInput:
		workflowFunc = wf.ParallelContainersWorkflow
	default:
		return nil, fmt.Errorf("unsupported workflow input type: %T", v)
	}

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	we, err := c.ExecuteWorkflow(ctx, options, workflowFunc, input)
	if err != nil {
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	return &WorkflowStatus{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Status:     "Running",
		StartTime:  time.Now(),
	}, nil
}

// SubmitAndWait submits a workflow and waits for completion.
//
// Example:
//
//	status, err := docker.SubmitAndWait(ctx, temporalClient, input, "docker-queue", 10*time.Minute)
func SubmitAndWait(ctx context.Context, c client.Client, input interface{}, taskQueue string, timeout time.Duration) (*WorkflowStatus, error) {
	status, err := SubmitWorkflow(ctx, c, input, taskQueue)
	if err != nil {
		return nil, err
	}

	// Wait for completion
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	we := c.GetWorkflow(timeoutCtx, status.WorkflowID, status.RunID)

	var result interface{}
	err = we.Get(timeoutCtx, &result)

	status.CloseTime = timePtr(time.Now())
	status.Result = result

	if err != nil {
		status.Status = "Failed"
		status.Error = err
		return status, err
	}

	status.Status = "Completed"
	return status, nil
}

// GetWorkflowStatus retrieves the current status of a workflow.
//
// Example:
//
//	status, err := docker.GetWorkflowStatus(ctx, temporalClient, workflowID, runID)
func GetWorkflowStatus(ctx context.Context, c client.Client, workflowID, runID string) (*WorkflowStatus, error) {
	// Simplified implementation - get workflow run and check if completed
	we := c.GetWorkflow(ctx, workflowID, runID)

	// Try to get the result - this will tell us if it's completed
	var result interface{}
	err := we.Get(ctx, &result)

	status := &WorkflowStatus{
		WorkflowID: workflowID,
		RunID:      runID,
		StartTime:  time.Now(), // Placeholder
	}

	if err != nil {
		// Check if it's still running or failed
		status.Status = "Failed or Running"
		status.Error = err
	} else {
		status.Status = "Completed"
		status.Result = result
		status.CloseTime = timePtr(time.Now())
	}

	return status, nil
}

// CancelWorkflow cancels a running workflow.
//
// Example:
//
//	err := docker.CancelWorkflow(ctx, temporalClient, workflowID, runID)
func CancelWorkflow(ctx context.Context, c client.Client, workflowID, runID string) error {
	return c.CancelWorkflow(ctx, workflowID, runID)
}

// TerminateWorkflow terminates a running workflow.
//
// Example:
//
//	err := docker.TerminateWorkflow(ctx, temporalClient, workflowID, runID, "reason")
func TerminateWorkflow(ctx context.Context, c client.Client, workflowID, runID, reason string) error {
	return c.TerminateWorkflow(ctx, workflowID, runID, reason)
}

// ListWorkflows lists workflows with optional filtering.
//
// Example:
//
//	workflows, err := docker.ListWorkflows(ctx, temporalClient, "docker-queue")
func ListWorkflows(ctx context.Context, c client.Client, query string) ([]*WorkflowStatus, error) {
	// Note: This is a simplified implementation
	// In production, you'd use the Temporal visibility API to list workflows

	return nil, fmt.Errorf("list workflows not implemented - use Temporal visibility API directly")
}

// WatchWorkflow watches a workflow execution and streams updates.
//
// Example:
//
//	updates := make(chan *WorkflowStatus)
//	err := docker.WatchWorkflow(ctx, temporalClient, workflowID, runID, updates)
func WatchWorkflow(ctx context.Context, c client.Client, workflowID, runID string, updates chan<- *WorkflowStatus) error {
	// Poll for updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := GetWorkflowStatus(ctx, c, workflowID, runID)
			if err != nil {
				return err
			}

			updates <- status

			// Stop watching if completed
			if status.Status == "Completed" || status.Status == "Failed" {
				close(updates)
				return nil
			}
		}
	}
}

// SignalWorkflow sends a signal to a running workflow.
//
// Example:
//
//	err := docker.SignalWorkflow(ctx, temporalClient, workflowID, runID, "pause", nil)
func SignalWorkflow(ctx context.Context, c client.Client, workflowID, runID, signalName string, arg interface{}) error {
	return c.SignalWorkflow(ctx, workflowID, runID, signalName, arg)
}

// QueryWorkflow queries a running workflow.
//
// Example:
//
//	var result string
//	err := docker.QueryWorkflow(ctx, temporalClient, workflowID, runID, "status", &result)
func QueryWorkflow(ctx context.Context, c client.Client, workflowID, runID, queryType string, result interface{}) error {
	response, err := c.QueryWorkflow(ctx, workflowID, runID, queryType)
	if err != nil {
		return err
	}
	return response.Get(result)
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

// WorkflowExecutionInfo provides detailed information about a workflow execution.
type WorkflowExecutionInfo struct {
	WorkflowID    string
	RunID         string
	WorkflowType  string
	StartTime     time.Time
	CloseTime     *time.Time
	Status        string
	HistoryLength int64
}

// GetWorkflowHistory retrieves the history of a workflow execution.
//
// Example:
//
//	history, err := docker.GetWorkflowHistory(ctx, temporalClient, workflowID, runID)
func GetWorkflowHistory(ctx context.Context, c client.Client, workflowID, runID string) (*WorkflowExecutionInfo, error) {
	// Note: This would require accessing the Temporal history service
	// This is a placeholder for the full implementation

	return nil, fmt.Errorf("get workflow history not implemented - use Temporal history API directly")
}
