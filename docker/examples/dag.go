//go:build example

package main

import (
	"context"
	"log"
	"time"

	"github.com/jasoet/go-wf/docker"
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

	// DAG Workflow Example - Similar to Argo Workflows DAG
	// This creates a complex dependency graph for a CI/CD pipeline:
	//
	//                    checkout
	//                       |
	//                    install
	//                  /    |    \
	//              build  lint  security-scan
	//                  \    |    /
	//                   test-unit
	//                       |
	//              +--------+--------+
	//              |                 |
	//        test-integration  test-e2e
	//              |                 |
	//              +--------+--------+
	//                       |
	//                    deploy
	//                       |
	//                 health-check
	//                       |
	//                   smoke-test

	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "checkout",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "alpine/git",
						Command:    []string{"sh", "-c", "echo 'Checking out code...' && sleep 1"},
						AutoRemove: true,
					},
				},
			},
			{
				Name: "install",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "node:20-alpine",
						Command:    []string{"sh", "-c", "echo 'Installing dependencies...' && sleep 2"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"checkout"},
			},
			{
				Name: "build",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "node:20-alpine",
						Command:    []string{"sh", "-c", "echo 'Building application...' && sleep 3"},
						AutoRemove: true,
					},
					Resources: &docker.ResourceLimits{
						CPURequest:    "500m",
						CPULimit:      "1000m",
						MemoryRequest: "512Mi",
						MemoryLimit:   "1Gi",
					},
				},
				Dependencies: []string{"install"},
			},
			{
				Name: "lint",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "node:20-alpine",
						Command:    []string{"sh", "-c", "echo 'Running linter...' && sleep 2"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"install"},
			},
			{
				Name: "security-scan",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "aquasec/trivy:latest",
						Command:    []string{"sh", "-c", "echo 'Running security scan...' && sleep 2"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"install"},
			},
			{
				Name: "test-unit",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "node:20-alpine",
						Command:    []string{"sh", "-c", "echo 'Running unit tests...' && sleep 3"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"build", "lint", "security-scan"},
			},
			{
				Name: "test-integration",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "node:20-alpine",
						Command:    []string{"sh", "-c", "echo 'Running integration tests...' && sleep 4"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"test-unit"},
			},
			{
				Name: "test-e2e",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "mcr.microsoft.com/playwright:latest",
						Command:    []string{"sh", "-c", "echo 'Running E2E tests...' && sleep 5"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"test-unit"},
			},
			{
				Name: "deploy",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Deploying to staging...' && sleep 2"},
						Env:        map[string]string{"ENVIRONMENT": "staging"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"test-integration", "test-e2e"},
			},
			{
				Name: "health-check",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "curlimages/curl:latest",
						Command:    []string{"sh", "-c", "echo 'Health check...' && sleep 1"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"deploy"},
			},
			{
				Name: "smoke-test",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Running smoke tests...' && sleep 2"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"health-check"},
			},
		},
		FailFast: true, // Stop on first failure
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "dag-cicd-example",
			TaskQueue: "docker-tasks",
		},
		docker.DAGWorkflow,
		input,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started DAG workflow: %s (RunID: %s)", we.GetID(), we.GetRunID())
	log.Printf("View in Temporal Web UI: http://localhost:8233/namespaces/default/workflows/%s", we.GetID())

	// Wait for result
	var result docker.DAGWorkflowOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("DAG workflow failed: %v", err)
	}

	log.Printf("\nDAG Workflow completed!")
	log.Printf("  Total Success: %d", result.TotalSuccess)
	log.Printf("  Total Failed: %d", result.TotalFailed)
	log.Printf("  Total Duration: %v", result.TotalDuration)

	log.Println("\nExecution Order:")
	for i, nodeResult := range result.NodeResults {
		status := "✓"
		if !nodeResult.Success {
			status = "✗"
		}
		log.Printf("  %d. %s %s (Exit: %d, Duration: %v)",
			i+1, status, nodeResult.NodeName,
			nodeResult.Result.ExitCode, nodeResult.Result.Duration)
	}
}
