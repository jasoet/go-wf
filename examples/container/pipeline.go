//go:build example

// This example demonstrates a sequential container pipeline (build -> test -> deploy).
// Each step runs in order with stop-on-error behavior and automatic cleanup.
//
// Run: task example:container -- pipeline.go

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
	"github.com/jasoet/go-wf/container/workflow"
)

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
	container.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Execute pipeline workflow - Build → Test → Deploy simulation
	input := payload.PipelineInput{
		StopOnError: true,
		Cleanup:     true,
		Containers: []payload.ContainerExecutionInput{
			{
				Name:    "build",
				Image:   "golang:1.23-alpine",
				Command: []string{"sh", "-c", "echo 'Building application...' && sleep 2"},
			},
			{
				Name:    "test",
				Image:   "golang:1.23-alpine",
				Command: []string{"sh", "-c", "echo 'Running tests...' && sleep 2"},
			},
			{
				Name:    "package",
				Image:   "alpine:latest",
				Command: []string{"sh", "-c", "echo 'Creating package...' && sleep 1"},
			},
		},
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "build-pipeline-example",
			TaskQueue: "container-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		input,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started pipeline workflow: %s", we.GetID())

	// Wait for result
	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	log.Printf("Pipeline completed!")
	log.Printf("  Total Success: %d", result.TotalSuccess)
	log.Printf("  Total Failed: %d", result.TotalFailed)
	log.Printf("  Total Duration: %v", result.TotalDuration)

	for i, r := range result.Results {
		log.Printf("  Step %d (%s): Exit=%d, Duration=%v", i+1, r.Name, r.ExitCode, r.Duration)
	}
}
