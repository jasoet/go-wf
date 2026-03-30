//go:build example

// This example demonstrates parallel function execution with controlled concurrency.
// Multiple independent data fetching tasks run concurrently with failure handling.
//
// Run: task example:function -- parallel.go

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

// This example demonstrates parallel function execution.
// Multiple independent data fetching tasks run concurrently
// with controlled concurrency and failure handling.

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

	// Create function registry with data fetching handlers
	registry := fn.NewRegistry()

	// Fetch users from user service
	_ = registry.Register("fetch-users", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[fetch-users] Fetching user data...")
		time.Sleep(500 * time.Millisecond) // Simulate API call

		users := []map[string]string{
			{"id": "1", "name": "Alice"},
			{"id": "2", "name": "Bob"},
			{"id": "3", "name": "Charlie"},
		}
		data, _ := json.Marshal(users)

		return &fn.FunctionOutput{
			Result: map[string]string{"count": "3", "source": "user-service"},
			Data:   data,
		}, nil
	})

	// Fetch orders from order service
	_ = registry.Register("fetch-orders", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[fetch-orders] Fetching order data...")
		time.Sleep(700 * time.Millisecond) // Simulate API call

		orders := []map[string]string{
			{"id": "ORD-001", "total": "150.00", "status": "completed"},
			{"id": "ORD-002", "total": "89.99", "status": "pending"},
		}
		data, _ := json.Marshal(orders)

		return &fn.FunctionOutput{
			Result: map[string]string{"count": "2", "source": "order-service"},
			Data:   data,
		}, nil
	})

	// Fetch inventory from warehouse service
	_ = registry.Register("fetch-inventory", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[fetch-inventory] Fetching inventory data...")
		time.Sleep(300 * time.Millisecond) // Simulate API call

		inventory := map[string]int{
			"SKU-A": 150,
			"SKU-B": 42,
			"SKU-C": 0,
		}
		data, _ := json.Marshal(inventory)

		return &fn.FunctionOutput{
			Result: map[string]string{
				"total_skus":   "3",
				"out_of_stock": "1",
				"source":       "warehouse-service",
			},
			Data: data,
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

	// Execute parallel workflow - fetch all data sources concurrently
	parallelInput := payload.ParallelInput{
		MaxConcurrency:  3,
		FailureStrategy: "continue", // Continue even if one source fails
		Functions: []payload.FunctionExecutionInput{
			{
				Name: "fetch-users",
				Args: map[string]string{"limit": "100", "offset": "0"},
			},
			{
				Name: "fetch-orders",
				Args: map[string]string{"status": "all", "days": "30"},
			},
			{
				Name: "fetch-inventory",
				Args: map[string]string{"warehouse": "main"},
			},
		},
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "dashboard-data-fetch",
			TaskQueue: "function-tasks",
		},
		workflow.ParallelFunctionsWorkflow,
		parallelInput,
	)
	if err != nil {
		log.Fatalf("Failed to start parallel workflow: %v", err)
	}

	log.Printf("Started parallel workflow: %s", we.GetID())

	var result payload.ParallelOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Parallel execution failed: %v", err)
	}

	log.Printf("Parallel execution completed!")
	log.Printf("  Total Success: %d", result.TotalSuccess)
	log.Printf("  Total Failed: %d", result.TotalFailed)
	log.Printf("  Total Duration: %v", result.TotalDuration)

	for _, r := range result.Results {
		status := "OK"
		if !r.Success {
			status = "FAIL"
		}
		log.Printf("  [%s] %s: records=%s, source=%s, duration=%v",
			status, r.Name,
			r.Result["count"],
			r.Result["source"],
			r.Duration)
	}

	// Aggregate results
	totalRecords := 0
	for _, r := range result.Results {
		if r.Success {
			if count, ok := r.Result["count"]; ok {
				var n int
				fmt.Sscanf(count, "%d", &n)
				totalRecords += n
			}
		}
	}
	log.Printf("  Total records fetched: %d", totalRecords)
}
