//go:build example

// This example demonstrates basic single function execution using the registry pattern.
// It registers a handler, starts a worker, and executes a single function workflow.
//
// Run: task example:function -- basic.go

package main

import (
	"context"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/function"
	fnactivity "github.com/jasoet/go-wf/function/activity"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/function/workflow"
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

	// Create function registry and register handlers
	registry := fn.NewRegistry()
	_ = registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		name := input.Args["name"]
		if name == "" {
			name = "World"
		}
		return &fn.FunctionOutput{
			Result: map[string]string{"greeting": "Hello, " + name + "!"},
		}, nil
	})

	// Create and start worker
	w := worker.New(c, "function-tasks", worker.Options{})
	fn.RegisterWorkflows(w)
	fn.RegisterActivity(w, fnactivity.NewExecuteFunctionActivity(registry))

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Execute single function workflow
	input := payload.FunctionExecutionInput{
		Name: "greet",
		Args: map[string]string{"name": "Temporal"},
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "greet-example",
			TaskQueue: "function-tasks",
		},
		workflow.ExecuteFunctionWorkflow,
		input,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started workflow ID: %s, RunID: %s", we.GetID(), we.GetRunID())

	var result payload.FunctionExecutionOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	log.Printf("Function executed successfully!")
	log.Printf("  Name: %s", result.Name)
	log.Printf("  Success: %v", result.Success)
	log.Printf("  Duration: %v", result.Duration)
	log.Printf("  Result: %v", result.Result)
}
