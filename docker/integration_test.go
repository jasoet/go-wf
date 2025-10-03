//go:build integration

package docker_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// TemporalContainer represents a Temporal server test container.
type TemporalContainer struct {
	testcontainers.Container
	HostPort string
}

// StartTemporalContainer starts a Temporal server container for testing.
func StartTemporalContainer(ctx context.Context, t *testing.T) (*TemporalContainer, error) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "temporalio/auto-setup:latest",
		ExposedPorts: []string{"7233/tcp"},
		Env: map[string]string{
			"TEMPORAL_BROADCAST_ADDRESS": "0.0.0.0",
		},
		WaitingFor: wait.ForListeningPort("7233/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start temporal container: %w", err)
	}

	// Get the mapped port
	mappedPort, err := container.MappedPort(ctx, "7233")
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			t.Logf("Failed to terminate container: %v", termErr)
		}
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	// Get the host
	host, err := container.Host(ctx)
	if err != nil {
		if termErr := container.Terminate(ctx); termErr != nil {
			t.Logf("Failed to terminate container: %v", termErr)
		}
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	hostPort := fmt.Sprintf("%s:%s", host, mappedPort.Port())

	return &TemporalContainer{
		Container: container,
		HostPort:  hostPort,
	}, nil
}

// TestIntegration_ExecuteContainerWorkflow tests single container execution with real Temporal server.
func TestIntegration_ExecuteContainerWorkflow(t *testing.T) {
	ctx := context.Background()

	// Start Temporal container
	temporal, err := StartTemporalContainer(ctx, t)
	if err != nil {
		t.Fatalf("Failed to start Temporal container: %v", err)
	}
	defer func() {
		if err := temporal.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Temporal container: %v", err)
		}
	}()

	// Wait a bit for Temporal to be ready
	time.Sleep(3 * time.Second)

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: temporal.HostPort,
	})
	if err != nil {
		t.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Create and start worker
	taskQueue := "integration-test-queue"
	w := worker.New(c, taskQueue, worker.Options{})
	docker.RegisterAll(w)

	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}
	defer w.Stop()

	// Execute workflow
	input := docker.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "integration test"},
		AutoRemove: true,
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-execute-container",
			TaskQueue: taskQueue,
		},
		docker.ExecuteContainerWorkflow,
		input,
	)
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// Wait for result
	var result docker.ContainerExecutionOutput
	if err := we.Get(ctx, &result); err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	// Verify result
	if !result.Success {
		t.Errorf("Expected successful execution, got: %+v", result)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}
	if result.ContainerID == "" {
		t.Error("Expected non-empty container ID")
	}
}

// TestIntegration_ContainerPipelineWorkflow tests pipeline execution with real Temporal server.
func TestIntegration_ContainerPipelineWorkflow(t *testing.T) {
	ctx := context.Background()

	// Start Temporal container
	temporal, err := StartTemporalContainer(ctx, t)
	if err != nil {
		t.Fatalf("Failed to start Temporal container: %v", err)
	}
	defer func() {
		if err := temporal.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Temporal container: %v", err)
		}
	}()

	time.Sleep(3 * time.Second)

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: temporal.HostPort,
	})
	if err != nil {
		t.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Create and start worker
	taskQueue := "integration-test-pipeline-queue"
	w := worker.New(c, taskQueue, worker.Options{})
	docker.RegisterAll(w)

	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}
	defer w.Stop()

	// Execute pipeline workflow
	input := docker.PipelineInput{
		Containers: []docker.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "step 1"},
				AutoRemove: true,
				Name:       "step1",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "step 2"},
				AutoRemove: true,
				Name:       "step2",
			},
		},
		StopOnError: true,
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-pipeline",
			TaskQueue: taskQueue,
		},
		docker.ContainerPipelineWorkflow,
		input,
	)
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// Wait for result
	var result docker.PipelineOutput
	if err := we.Get(ctx, &result); err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	// Verify result
	if result.TotalSuccess != 2 {
		t.Errorf("Expected 2 successful containers, got %d", result.TotalSuccess)
	}
	if result.TotalFailed != 0 {
		t.Errorf("Expected 0 failed containers, got %d", result.TotalFailed)
	}
	if len(result.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(result.Results))
	}
}

// TestIntegration_ParallelContainersWorkflow tests parallel execution with real Temporal server.
func TestIntegration_ParallelContainersWorkflow(t *testing.T) {
	ctx := context.Background()

	// Start Temporal container
	temporal, err := StartTemporalContainer(ctx, t)
	if err != nil {
		t.Fatalf("Failed to start Temporal container: %v", err)
	}
	defer func() {
		if err := temporal.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Temporal container: %v", err)
		}
	}()

	time.Sleep(3 * time.Second)

	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: temporal.HostPort,
	})
	if err != nil {
		t.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Create and start worker
	taskQueue := "integration-test-parallel-queue"
	w := worker.New(c, taskQueue, worker.Options{})
	docker.RegisterAll(w)

	if err := w.Start(); err != nil {
		t.Fatalf("Failed to start worker: %v", err)
	}
	defer w.Stop()

	// Execute parallel workflow
	input := docker.ParallelInput{
		Containers: []docker.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 1"},
				AutoRemove: true,
				Name:       "task1",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 2"},
				AutoRemove: true,
				Name:       "task2",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 3"},
				AutoRemove: true,
				Name:       "task3",
			},
		},
		FailureStrategy: "continue",
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-parallel",
			TaskQueue: taskQueue,
		},
		docker.ParallelContainersWorkflow,
		input,
	)
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// Wait for result
	var result docker.ParallelOutput
	if err := we.Get(ctx, &result); err != nil {
		t.Fatalf("Workflow failed: %v", err)
	}

	// Verify result
	if result.TotalSuccess != 3 {
		t.Errorf("Expected 3 successful containers, got %d", result.TotalSuccess)
	}
	if result.TotalFailed != 0 {
		t.Errorf("Expected 0 failed containers, got %d", result.TotalFailed)
	}
	if len(result.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result.Results))
	}
}
