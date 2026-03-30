//go:build example

// This example is a shared container worker that registers all container workflows and activities.
// Start this worker in a separate terminal, then trigger workflows via other example files.
//
// Run: task example:container:worker

package main

import (
	"log"
	"os"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/container"
)

func main() {
	// Create Temporal client
	config := temporal.DefaultConfig()
	if hostPort := os.Getenv("TEMPORAL_HOST_PORT"); hostPort != "" {
		config.HostPort = hostPort
	}
	c, closer, err := temporal.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	log.Println("Starting Docker Temporal Worker...")

	// Create worker
	w := worker.New(c, "container-tasks", worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Register all container workflows and activities
	container.RegisterAll(w)

	log.Println("Registered workflows:")
	log.Println("  - ExecuteContainerWorkflow")
	log.Println("  - ContainerPipelineWorkflow")
	log.Println("  - ParallelContainersWorkflow")
	log.Println()
	log.Println("Registered activities:")
	log.Println("  - StartContainerActivity")
	log.Println()
	log.Println("Worker listening on task queue: container-tasks")

	// Run worker (blocks until interrupted)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}

	log.Println("Worker stopped")
}
