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

	// Give worker time to start
	time.Sleep(time.Second)

	// Execute container workflow
	input := payload.ContainerExecutionInput{
		Image: "postgres:16-alpine",
		Env: map[string]string{
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_USER":     "test",
			"POSTGRES_DB":       "test",
		},
		Ports: []string{"5432:5432"},
		WaitStrategy: payload.WaitStrategyConfig{
			Type:           "log",
			LogMessage:     "ready to accept connections",
			StartupTimeout: 30 * time.Second,
		},
		AutoRemove: true,
		Name:       "example-postgres",
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "postgres-setup-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started workflow ID: %s, RunID: %s", we.GetID(), we.GetRunID())

	// Wait for result
	var result payload.ContainerExecutionOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	log.Printf("Container executed successfully!")
	log.Printf("  Container ID: %s", result.ContainerID)
	log.Printf("  Exit Code: %d", result.ExitCode)
	log.Printf("  Duration: %v", result.Duration)
	log.Printf("  Endpoint: %s", result.Endpoint)
	log.Printf("  Success: %v", result.Success)
}
