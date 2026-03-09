//go:build example

package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// This example demonstrates a sequential function pipeline:
// validate -> transform -> notify
// Each step processes data and passes results to the next stage.

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

	// Create function registry with pipeline handlers
	registry := fn.NewRegistry()

	// Step 1: Validate incoming data
	registry.Register("validate", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		email := input.Args["email"]
		name := input.Args["name"]

		if email == "" || name == "" {
			return nil, fmt.Errorf("validation failed: email and name are required")
		}

		log.Printf("[validate] Data validated: name=%s, email=%s", name, email)
		return &fn.FunctionOutput{
			Result: map[string]string{
				"status":    "valid",
				"name":      name,
				"email":     email,
				"validated": time.Now().Format(time.RFC3339),
			},
		}, nil
	})

	// Step 2: Transform/enrich the data
	registry.Register("transform", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		name := input.Args["name"]
		email := input.Args["email"]

		// Simulate data enrichment
		enriched := map[string]string{
			"display_name": fmt.Sprintf("%s <%s>", name, email),
			"slug":         fmt.Sprintf("%s-user", name),
			"tier":         "standard",
			"transformed":  time.Now().Format(time.RFC3339),
		}

		record, _ := json.Marshal(enriched)
		log.Printf("[transform] Data enriched: %s", string(record))

		return &fn.FunctionOutput{
			Result: enriched,
			Data:   record,
		}, nil
	})

	// Step 3: Send notification
	registry.Register("notify", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		channel := input.Args["channel"]
		if channel == "" {
			channel = "email"
		}

		log.Printf("[notify] Notification sent via %s for user %s", channel, input.Args["name"])
		return &fn.FunctionOutput{
			Result: map[string]string{
				"channel":   channel,
				"status":    "sent",
				"sent_at":   time.Now().Format(time.RFC3339),
				"recipient": input.Args["name"],
			},
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

	// Execute pipeline: validate -> transform -> notify
	pipelineInput := payload.PipelineInput{
		StopOnError: true,
		Functions: []payload.FunctionExecutionInput{
			{
				Name: "validate",
				Args: map[string]string{
					"name":  "Alice",
					"email": "alice@example.com",
				},
			},
			{
				Name: "transform",
				Args: map[string]string{
					"name":  "Alice",
					"email": "alice@example.com",
				},
			},
			{
				Name: "notify",
				Args: map[string]string{
					"name":    "Alice",
					"channel": "slack",
				},
			},
		},
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "user-onboarding-pipeline",
			TaskQueue: "function-tasks",
		},
		workflow.FunctionPipelineWorkflow,
		pipelineInput,
	)
	if err != nil {
		log.Fatalf("Failed to start pipeline: %v", err)
	}

	log.Printf("Started pipeline workflow: %s", we.GetID())

	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Pipeline failed: %v", err)
	}

	log.Printf("Pipeline completed!")
	log.Printf("  Total Success: %d", result.TotalSuccess)
	log.Printf("  Total Failed: %d", result.TotalFailed)
	log.Printf("  Total Duration: %v", result.TotalDuration)

	for i, r := range result.Results {
		status := "PASS"
		if !r.Success {
			status = "FAIL"
		}
		log.Printf("  Step %d (%s): %s, Duration=%v", i+1, r.Name, status, r.Duration)
	}
}
