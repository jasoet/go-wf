//go:build example

package main

import (
	"context"
	"log"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/docker/workflow"
	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create Temporal client
	c, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Create and start worker
	w := worker.New(c, "docker-tasks", worker.Options{})
	docker.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Execute parallel workflow - Run multiple test suites concurrently
	input := payload.ParallelInput{
		MaxConcurrency:  3,
		FailureStrategy: "continue", // Continue even if some fail
		Containers: []payload.ContainerExecutionInput{
			{
				Name:    "unit-tests",
				Image:   "golang:1.23-alpine",
				Command: []string{"sh", "-c", "echo 'Running unit tests...' && sleep 3"},
			},
			{
				Name:    "integration-tests",
				Image:   "golang:1.23-alpine",
				Command: []string{"sh", "-c", "echo 'Running integration tests...' && sleep 4"},
			},
			{
				Name:    "lint-check",
				Image:   "golangci/golangci-lint:latest",
				Command: []string{"sh", "-c", "echo 'Running linter...' && sleep 2"},
			},
		},
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "parallel-tests-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ParallelContainersWorkflow,
		input,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started parallel workflow: %s", we.GetID())

	// Wait for result
	var result payload.ParallelOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	log.Printf("Parallel execution completed!")
	log.Printf("  Total Success: %d", result.TotalSuccess)
	log.Printf("  Total Failed: %d", result.TotalFailed)
	log.Printf("  Total Duration: %v", result.TotalDuration)

	for _, r := range result.Results {
		status := "✓"
		if !r.Success {
			status = "✗"
		}
		log.Printf("  %s %s: Exit=%d, Duration=%v", status, r.Name, r.ExitCode, r.Duration)
	}
}
