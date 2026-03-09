//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/function"
	fnactivity "github.com/jasoet/go-wf/function/activity"
	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/function/workflow"
)

// This example demonstrates loop workflows in the function module:
// 1. Simple parallel loop processing items
// 2. Sequential loop with fail_fast
// 3. Parameterized loop with env/region combinations
// 4. Builder API for loops (ForEach and ForEachParam)

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
	registry.Register("process-csv", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		file := input.Args["file"]
		log.Printf("Processing CSV file: %s", file)
		return &fn.FunctionOutput{
			Result: map[string]string{"file": file, "status": "processed"},
		}, nil
	})
	registry.Register("run-migration", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		migration := input.Args["migration"]
		log.Printf("Running migration: %s", migration)
		return &fn.FunctionOutput{
			Result: map[string]string{"migration": migration, "status": "applied"},
		}, nil
	})
	registry.Register("deploy-service", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		env := input.Args["environment"]
		region := input.Args["region"]
		log.Printf("Deploying to %s in %s", env, region)
		return &fn.FunctionOutput{
			Result: map[string]string{"env": env, "region": region, "status": "deployed"},
		}, nil
	})
	registry.Register("sync-tenant", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		tenant := input.Args["tenant"]
		log.Printf("Syncing tenant: %s", tenant)
		return &fn.FunctionOutput{
			Result: map[string]string{"tenant": tenant, "status": "synced"},
		}, nil
	})
	registry.Register("health-check", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		service := input.Args["service"]
		env := input.Args["environment"]
		log.Printf("Health check: %s in %s", service, env)
		return &fn.FunctionOutput{
			Result: map[string]string{"service": service, "env": env, "healthy": "true"},
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

	ctx := context.Background()

	// Example 1: Simple parallel loop - process data files
	fmt.Println("\n=== Example 1: Simple Parallel Loop ===")
	simpleParallelLoopExample(ctx, c)

	// Example 2: Sequential loop with fail_fast - database migrations
	fmt.Println("\n=== Example 2: Sequential Loop with Fail Fast ===")
	sequentialLoopExample(ctx, c)

	// Example 3: Parameterized loop - multi-environment deployment
	fmt.Println("\n=== Example 3: Parameterized Loop - Multi-Environment ===")
	parameterizedLoopExample(ctx, c)

	// Example 4: Builder API with ForEach
	fmt.Println("\n=== Example 4: Builder API - ForEach ===")
	forEachBuilderExample(ctx, c)

	// Example 5: Builder API with ForEachParam
	fmt.Println("\n=== Example 5: Builder API - ForEachParam ===")
	forEachParamBuilderExample(ctx, c)
}

func simpleParallelLoopExample(ctx context.Context, c client.Client) {
	// Process multiple CSV files in parallel using a function handler
	files := []string{"users.csv", "orders.csv", "products.csv", "transactions.csv"}

	input := payload.LoopInput{
		Items: files,
		Template: payload.FunctionExecutionInput{
			Name: "process-csv",
			Args: map[string]string{
				"file":   "{{item}}",
				"index":  "{{index}}",
				"format": "json",
			},
			Env: map[string]string{
				"INPUT_FILE":    "/data/{{item}}",
				"OUTPUT_FORMAT": "json",
			},
		},
		Parallel:        true,
		MaxConcurrency:  2,
		FailureStrategy: "continue",
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "loop-csv-processing",
			TaskQueue: "function-tasks",
		},
		workflow.LoopWorkflow,
		input,
	)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started parallel loop: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.LoopOutput
	if err := we.Get(ctx, &result); err != nil {
		log.Fatalln("Unable to get result", err)
	}

	fmt.Printf("Parallel loop completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func sequentialLoopExample(ctx context.Context, c client.Client) {
	// Run database migrations sequentially; stop on first failure
	migrations := []string{"001_create_users", "002_add_indexes", "003_create_orders", "004_add_constraints"}

	input := payload.LoopInput{
		Items: migrations,
		Template: payload.FunctionExecutionInput{
			Name: "run-migration",
			Args: map[string]string{
				"migration": "{{item}}",
				"step":      "{{index}}",
			},
			Env: map[string]string{
				"DB_HOST":       "localhost",
				"DB_NAME":       "appdb",
				"MIGRATION_DIR": "/migrations",
			},
		},
		Parallel:        false, // Sequential execution - order matters
		FailureStrategy: "fail_fast",
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "loop-db-migrations",
			TaskQueue: "function-tasks",
		},
		workflow.LoopWorkflow,
		input,
	)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started sequential loop: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.LoopOutput
	if err := we.Get(ctx, &result); err != nil {
		log.Fatalln("Unable to get result", err)
	}

	fmt.Printf("Sequential loop completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func parameterizedLoopExample(ctx context.Context, c client.Client) {
	// Deploy to all combinations of environments and regions
	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "staging", "prod"},
			"region": {"us-west-2", "eu-west-1"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "deploy-service",
			Args: map[string]string{
				"environment": "{{.env}}",
				"region":      "{{.region}}",
				"version":     "v1.5.0",
			},
			Env: map[string]string{
				"DEPLOY_ENV":    "{{.env}}",
				"DEPLOY_REGION": "{{.region}}",
				"INDEX":         "{{index}}",
			},
		},
		Parallel:        true,
		MaxConcurrency:  2, // Limit concurrent deployments
		FailureStrategy: "fail_fast",
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "loop-multi-env-deploy",
			TaskQueue: "function-tasks",
		},
		workflow.ParameterizedLoopWorkflow,
		input,
	)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started parameterized loop: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.LoopOutput
	if err := we.Get(ctx, &result); err != nil {
		log.Fatalln("Unable to get result", err)
	}

	fmt.Printf("Parameterized loop completed: Combinations=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func forEachBuilderExample(ctx context.Context, c client.Client) {
	// Use builder.ForEach for a concise loop definition
	tenants := []string{"acme-corp", "globex", "initech", "umbrella"}

	template := payload.FunctionExecutionInput{
		Name: "sync-tenant",
		Args: map[string]string{
			"tenant": "{{item}}",
			"mode":   "full",
		},
	}

	loopInput, err := builder.ForEach(tenants, template).
		Parallel(true).
		MaxConcurrency(2).
		FailFast(false).
		BuildLoop()
	if err != nil {
		log.Fatalln("Unable to build loop", err)
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "loop-tenant-sync",
			TaskQueue: "function-tasks",
		},
		workflow.LoopWorkflow,
		*loopInput,
	)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started ForEach loop: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.LoopOutput
	if err := we.Get(ctx, &result); err != nil {
		log.Fatalln("Unable to get result", err)
	}

	fmt.Printf("ForEach loop completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func forEachParamBuilderExample(ctx context.Context, c client.Client) {
	// Use builder.ForEachParam for parameterized matrix execution
	params := map[string][]string{
		"service": {"api", "web", "worker"},
		"env":     {"staging", "production"},
	}

	template := payload.FunctionExecutionInput{
		Name: "health-check",
		Args: map[string]string{
			"service":     "{{.service}}",
			"environment": "{{.env}}",
			"timeout":     "30s",
		},
	}

	loopInput, err := builder.ForEachParam(params, template).
		Parallel(true).
		MaxConcurrency(3).
		FailFast(false).
		BuildParameterizedLoop()
	if err != nil {
		log.Fatalln("Unable to build parameterized loop", err)
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "loop-health-checks",
			TaskQueue: "function-tasks",
		},
		workflow.ParameterizedLoopWorkflow,
		*loopInput,
	)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started ForEachParam loop: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.LoopOutput
	if err := we.Get(ctx, &result); err != nil {
		log.Fatalln("Unable to get result", err)
	}

	fmt.Printf("ForEachParam loop completed: Combinations=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}
