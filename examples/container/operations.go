//go:build example

package main

import (
	"context"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/container"
	"github.com/jasoet/go-wf/container/payload"
)

// This example demonstrates the workflow operations API for managing
// workflow lifecycle: submission, status checking, cancellation,
// termination, signaling, querying, and watching.

func main() {
	// Create Temporal client
	c, closer, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	// Create and start worker
	w := worker.New(c, "container-tasks", worker.Options{})
	docker.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Example 1: Submit a workflow and check its status
	log.Println("=== Example 1: SubmitWorkflow + GetWorkflowStatus ===")
	submitAndCheckStatus(c)

	time.Sleep(2 * time.Second)

	// Example 2: Submit a workflow and wait with a timeout
	log.Println("\n=== Example 2: SubmitAndWait with timeout ===")
	submitWithTimeout(c)

	time.Sleep(2 * time.Second)

	// Example 3: Cancel and terminate workflows
	log.Println("\n=== Example 3: CancelWorkflow + TerminateWorkflow ===")
	cancelAndTerminate(c)

	time.Sleep(2 * time.Second)

	// Example 4: Watch, signal, and query a workflow
	log.Println("\n=== Example 4: WatchWorkflow + SignalWorkflow + QueryWorkflow ===")
	watchWorkflowUpdates(c)

	log.Println("All operations examples completed.")
}

// submitAndCheckStatus demonstrates SubmitWorkflow and GetWorkflowStatus.
// It submits a pipeline workflow, then polls its status until completion.
func submitAndCheckStatus(c client.Client) {
	ctx := context.Background()

	input := payload.PipelineInput{
		StopOnError: true,
		Containers: []payload.ContainerExecutionInput{
			{
				Name:       "ops-step-1",
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "echo 'Step 1 running' && sleep 2"},
				AutoRemove: true,
			},
			{
				Name:       "ops-step-2",
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "echo 'Step 2 running' && sleep 1"},
				AutoRemove: true,
			},
		},
	}

	// Submit the workflow (non-blocking)
	status, err := docker.SubmitWorkflow(ctx, c, input, "container-tasks")
	if err != nil {
		log.Printf("Failed to submit workflow: %v", err)
		return
	}

	log.Printf("Submitted workflow: ID=%s, RunID=%s, Status=%s",
		status.WorkflowID, status.RunID, status.Status)

	// Check status while running
	time.Sleep(time.Second)
	currentStatus, err := docker.GetWorkflowStatus(ctx, c, status.WorkflowID, status.RunID)
	if err != nil {
		log.Printf("Failed to get status: %v", err)
		return
	}
	log.Printf("  Current status: %s (started: %s)", currentStatus.Status, currentStatus.StartTime.Format(time.RFC3339))

	// Wait for completion and check final status
	time.Sleep(5 * time.Second)
	finalStatus, err := docker.GetWorkflowStatus(ctx, c, status.WorkflowID, status.RunID)
	if err != nil {
		log.Printf("Failed to get final status: %v", err)
		return
	}
	log.Printf("  Final status: %s", finalStatus.Status)
}

// submitWithTimeout demonstrates SubmitAndWait with a configurable timeout.
// The workflow is submitted and the call blocks until it completes or the timeout fires.
func submitWithTimeout(c client.Client) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Running with timeout' && sleep 2 && echo 'Finished'"},
		AutoRemove: true,
		Name:       "ops-timeout-demo",
	}

	// Submit and wait up to 30 seconds
	status, err := docker.SubmitAndWait(ctx, c, input, "container-tasks", 30*time.Second)
	if err != nil {
		log.Printf("Workflow failed or timed out: %v", err)
		if status != nil {
			log.Printf("  Partial status: ID=%s, Status=%s", status.WorkflowID, status.Status)
		}
		return
	}

	closeTimeStr := "N/A"
	if status.CloseTime != nil {
		closeTimeStr = status.CloseTime.Format(time.RFC3339)
	}

	log.Printf("Workflow completed within timeout!")
	log.Printf("  ID=%s, Status=%s", status.WorkflowID, status.Status)
	log.Printf("  Start=%s, Close=%s", status.StartTime.Format(time.RFC3339), closeTimeStr)
}

// cancelAndTerminate demonstrates CancelWorkflow and TerminateWorkflow.
// It starts two long-running workflows, cancels one and terminates the other.
func cancelAndTerminate(c client.Client) {
	ctx := context.Background()

	// Start a long-running workflow for cancellation
	cancelInput := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Will be cancelled' && sleep 120"},
		AutoRemove: true,
		Name:       "ops-cancel-demo",
	}

	cancelStatus, err := docker.SubmitWorkflow(ctx, c, cancelInput, "container-tasks")
	if err != nil {
		log.Printf("Failed to submit cancel-demo workflow: %v", err)
		return
	}
	log.Printf("Submitted cancel-demo: ID=%s", cancelStatus.WorkflowID)

	// Start another long-running workflow for termination
	terminateInput := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Will be terminated' && sleep 120"},
		AutoRemove: true,
		Name:       "ops-terminate-demo",
	}

	terminateStatus, err := docker.SubmitWorkflow(ctx, c, terminateInput, "container-tasks")
	if err != nil {
		log.Printf("Failed to submit terminate-demo workflow: %v", err)
		return
	}
	log.Printf("Submitted terminate-demo: ID=%s", terminateStatus.WorkflowID)

	// Give workflows time to start
	time.Sleep(3 * time.Second)

	// Cancel the first workflow
	err = docker.CancelWorkflow(ctx, c, cancelStatus.WorkflowID, cancelStatus.RunID)
	if err != nil {
		log.Printf("Failed to cancel workflow: %v", err)
	} else {
		log.Printf("Successfully cancelled workflow: %s", cancelStatus.WorkflowID)
	}

	// Terminate the second workflow with a reason
	err = docker.TerminateWorkflow(ctx, c, terminateStatus.WorkflowID, terminateStatus.RunID,
		"Terminated by operations example")
	if err != nil {
		log.Printf("Failed to terminate workflow: %v", err)
	} else {
		log.Printf("Successfully terminated workflow: %s", terminateStatus.WorkflowID)
	}

	// Check the status of both workflows after cancellation/termination
	time.Sleep(2 * time.Second)

	cancelledStatus, _ := docker.GetWorkflowStatus(ctx, c, cancelStatus.WorkflowID, cancelStatus.RunID)
	if cancelledStatus != nil {
		log.Printf("Cancelled workflow status: %s", cancelledStatus.Status)
	}

	terminatedStatus, _ := docker.GetWorkflowStatus(ctx, c, terminateStatus.WorkflowID, terminateStatus.RunID)
	if terminatedStatus != nil {
		log.Printf("Terminated workflow status: %s", terminatedStatus.Status)
	}
}

// watchWorkflowUpdates demonstrates WatchWorkflow, SignalWorkflow, and QueryWorkflow.
// It starts a workflow, watches for updates in a goroutine, sends a signal, and queries state.
func watchWorkflowUpdates(c client.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Watchable workflow' && sleep 15 && echo 'Done watching'"},
		AutoRemove: true,
		Name:       "ops-watch-demo",
	}

	// Submit the workflow
	status, err := docker.SubmitWorkflow(ctx, c, input, "container-tasks")
	if err != nil {
		log.Printf("Failed to submit workflow: %v", err)
		return
	}
	log.Printf("Submitted watch-demo: ID=%s", status.WorkflowID)

	// Watch for updates in a goroutine
	updates := make(chan *docker.WorkflowStatus, 10)
	watchDone := make(chan error, 1)

	go func() {
		watchDone <- docker.WatchWorkflow(ctx, c, status.WorkflowID, status.RunID, updates)
	}()

	// Consume updates in another goroutine
	go func() {
		for update := range updates {
			log.Printf("  [Watch] Status update: %s (WorkflowID=%s)",
				update.Status, update.WorkflowID)
		}
		log.Println("  [Watch] Updates channel closed")
	}()

	// Send a signal to the workflow (best effort — may not be handled by the container workflow)
	time.Sleep(2 * time.Second)
	err = docker.SignalWorkflow(ctx, c, status.WorkflowID, status.RunID, "custom-signal",
		map[string]string{"action": "info", "source": "operations-example"})
	if err != nil {
		log.Printf("  Signal failed (expected for container workflows): %v", err)
	} else {
		log.Println("  Signal sent successfully to workflow")
	}

	// Query the workflow (best effort — requires workflow to register a query handler)
	var queryResult string
	err = docker.QueryWorkflow(ctx, c, status.WorkflowID, status.RunID, "status", &queryResult)
	if err != nil {
		log.Printf("  Query failed (may not be supported): %v", err)
	} else {
		log.Printf("  Query result: %s", queryResult)
	}

	// Wait for the watch to complete
	select {
	case err := <-watchDone:
		if err != nil {
			log.Printf("  Watch ended with error: %v", err)
		} else {
			log.Println("  Watch completed successfully")
		}
	case <-time.After(90 * time.Second):
		log.Println("  Watch timed out")
		cancel()
	}
}
