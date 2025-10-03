//go:build example

package main

import (
	"log"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create Temporal client
	c, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	log.Println("Starting Docker Temporal Worker...")

	// Create worker
	w := worker.New(c, "docker-tasks", worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Register all docker workflows and activities
	docker.RegisterAll(w)

	log.Println("Registered workflows:")
	log.Println("  - ExecuteContainerWorkflow")
	log.Println("  - ContainerPipelineWorkflow")
	log.Println("  - ParallelContainersWorkflow")
	log.Println()
	log.Println("Registered activities:")
	log.Println("  - StartContainerActivity")
	log.Println()
	log.Println("Worker listening on task queue: docker-tasks")

	// Run worker (blocks until interrupted)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}

	log.Println("Worker stopped")
}
