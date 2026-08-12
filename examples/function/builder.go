//go:build example

// This example demonstrates the fluent builder API for constructing function workflows:
// ETL pipelines, parallel pre-flight checks, reusable FunctionSource, and dynamic WorkflowSourceFunc.
//
// Run: task example:function -- builder.go

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/v2/function"
	fnactivity "github.com/jasoet/go-wf/v2/function/activity"
	"github.com/jasoet/go-wf/v2/function/builder"
	"github.com/jasoet/go-wf/v2/function/payload"
	"github.com/jasoet/go-wf/v2/function/workflow"
	generic "github.com/jasoet/go-wf/v2/workflow"
)

// This example demonstrates the Builder API for constructing function workflows:
// 1. Pipeline via builder with Add and Pipeline().Build()
// 2. Parallel via builder with Parallel(), MaxConcurrency, FailFast
// 3. Using FunctionSource as a WorkflowSource
// 4. Using dynamic input generation

func main() {
	// Create Temporal client
	c, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Create function registry with ETL handlers
	registry := fn.NewRegistry()

	_ = registry.Register("extract", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		source := input.Args["source"]
		log.Printf("[extract] Extracting data from %s", source)
		return &fn.FunctionOutput{
			Result: map[string]string{"records": "1500", "source": source},
		}, nil
	})

	_ = registry.Register("transform", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		format := input.Args["format"]
		log.Printf("[transform] Transforming data to %s format", format)
		return &fn.FunctionOutput{
			Result: map[string]string{"format": format, "records": "1480", "dropped": "20"},
		}, nil
	})

	_ = registry.Register("load", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		target := input.Args["target"]
		log.Printf("[load] Loading data into %s", target)
		return &fn.FunctionOutput{
			Result: map[string]string{"target": target, "loaded": "1480"},
		}, nil
	})

	_ = registry.Register("validate-config", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		env := input.Args["env"]
		log.Printf("[validate-config] Validating config for %s", env)
		return &fn.FunctionOutput{
			Result: map[string]string{"env": env, "valid": "true"},
		}, nil
	})

	_ = registry.Register("check-deps", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		service := input.Args["service"]
		log.Printf("[check-deps] Checking dependencies for %s", service)
		return &fn.FunctionOutput{
			Result: map[string]string{"service": service, "healthy": "true"},
		}, nil
	})

	_ = registry.Register("run-smoke-tests", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		target := input.Args["target"]
		log.Printf("[run-smoke-tests] Running smoke tests against %s", target)
		return &fn.FunctionOutput{
			Result: map[string]string{"target": target, "passed": "12", "failed": "0"},
		}, nil
	})

	activityFn := fnactivity.NewExecuteFunctionActivity(registry)

	// Create and start worker
	w := worker.New(c, "function-tasks", worker.Options{})
	fn.RegisterWorkflows(w)
	fn.RegisterActivity(w, activityFn)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	ctx := context.Background()

	// Example 1: Pipeline via builder with Add
	fmt.Println("\n=== Example 1: ETL Pipeline via Builder ===")
	runETLPipeline(ctx, c, activityFn)

	time.Sleep(time.Second)

	// Example 2: Parallel via builder with MaxConcurrency and FailFast
	fmt.Println("\n=== Example 2: Parallel Pre-flight Checks ===")
	runParallelChecks(ctx, c, activityFn)

	time.Sleep(time.Second)

	// Example 3: Using FunctionSource as WorkflowSource
	fmt.Println("\n=== Example 3: FunctionSource as WorkflowSource ===")
	runFunctionSourceExample(ctx, c, activityFn)

	time.Sleep(time.Second)

	// Example 4: Using dynamic inputs
	fmt.Println("\n=== Example 4: Dynamic Inputs ===")
	runDynamicInputExample(ctx, c, activityFn)
}

// Example 1: ETL Pipeline using Add for direct payload construction
func runETLPipeline(ctx context.Context, c client.Client, activityFn any) {
	def, err := builder.NewFunctionBuilder().
		Name("etl-pipeline").
		Activity(activityFn).
		Pipeline().
		Add(&payload.FunctionExecutionInput{
			Name: "extract",
			Args: map[string]string{
				"source": "s3://data-lake/raw/2024-01",
				"format": "parquet",
			},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "transform",
			Args: map[string]string{
				"format":  "json",
				"filters": "active=true",
			},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "load",
			Args: map[string]string{
				"target": "postgresql://analytics/warehouse",
				"mode":   "upsert",
			},
		}).
		StopOnError(true).
		Build()
	if err != nil {
		log.Printf("Failed to build ETL pipeline: %v", err)
		return
	}

	// Use the raw input factory to construct the workflow input.
	pipelineInput := def.NewInput()

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "etl-pipeline-example",
			TaskQueue: def.TaskQueue,
		},
		workflow.FunctionPipelineWorkflow,
		pipelineInput,
	)
	if err != nil {
		log.Printf("Failed to start pipeline: %v", err)
		return
	}

	var result generic.PipelineOutput[payload.FunctionExecutionOutput]
	if err := we.Get(ctx, &result); err != nil {
		log.Printf("Pipeline failed: %v", err)
		return
	}

	log.Printf("ETL Pipeline completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)

	for i, r := range result.Results {
		log.Printf("  Step %d (%s): Success=%v, Duration=%v", i+1, r.Name, r.Success, r.Duration)
	}
}

// Example 2: Parallel pre-flight checks with concurrency control
func runParallelChecks(ctx context.Context, c client.Client, activityFn any) {
	def, err := builder.NewFunctionBuilder().
		Name("pre-flight-checks").
		Activity(activityFn).
		Parallel().
		Add(&payload.FunctionExecutionInput{
			Name: "validate-config",
			Args: map[string]string{"env": "production"},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "check-deps",
			Args: map[string]string{"service": "api-gateway"},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "check-deps",
			Args: map[string]string{"service": "auth-service"},
		}).
		Add(&payload.FunctionExecutionInput{
			Name: "run-smoke-tests",
			Args: map[string]string{"target": "https://staging.example.com"},
		}).
		MaxConcurrency(2).
		FailFast(true).
		Build()
	if err != nil {
		log.Printf("Failed to build parallel checks: %v", err)
		return
	}

	parallelInput := def.NewInput()

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "pre-flight-checks-example",
			TaskQueue: def.TaskQueue,
		},
		workflow.ParallelFunctionsWorkflow,
		parallelInput,
	)
	if err != nil {
		log.Printf("Failed to start parallel checks: %v", err)
		return
	}

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	if err := we.Get(ctx, &result); err != nil {
		log.Printf("Parallel checks failed: %v", err)
		return
	}

	log.Printf("Pre-flight checks completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)

	for _, r := range result.Results {
		log.Printf("  %s: Success=%v, Duration=%v", r.Name, r.Success, r.Duration)
	}
}

// Example 3: Using FunctionSource to wrap inputs as WorkflowSource
func runFunctionSourceExample(ctx context.Context, c client.Client, activityFn any) {
	// Create reusable FunctionSource components
	extractInput := payload.FunctionExecutionInput{
		Name: "extract",
		Args: map[string]string{
			"source": "api://crm/contacts",
			"format": "csv",
		},
	}

	transformInput := payload.FunctionExecutionInput{
		Name: "transform",
		Args: map[string]string{
			"format": "json",
		},
	}

	loadInput := payload.FunctionExecutionInput{
		Name: "load",
		Args: map[string]string{
			"target": "elasticsearch://search/contacts",
			"mode":   "replace",
		},
	}

	// Compose pipeline from reusable inputs
	def, err := builder.NewFunctionBuilder().
		Name("crm-sync").
		Activity(activityFn).
		Pipeline().
		Add(&extractInput).
		Add(&transformInput).
		Add(&loadInput).
		StopOnError(true).
		Build()
	if err != nil {
		log.Printf("Failed to build CRM sync pipeline: %v", err)
		return
	}

	pipelineInput := def.NewInput()

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "crm-sync-example",
			TaskQueue: def.TaskQueue,
		},
		workflow.FunctionPipelineWorkflow,
		pipelineInput,
	)
	if err != nil {
		log.Printf("Failed to start CRM sync: %v", err)
		return
	}

	var result generic.PipelineOutput[payload.FunctionExecutionOutput]
	if err := we.Get(ctx, &result); err != nil {
		log.Printf("CRM sync failed: %v", err)
		return
	}

	log.Printf("CRM sync completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

// Example 4: Using dynamic input generation
func runDynamicInputExample(ctx context.Context, c client.Client, activityFn any) {
	// Generate inputs dynamically at build time
	environments := []string{"staging", "production"}

	wb := builder.NewFunctionBuilder().
		Name("dynamic-validation").
		Activity(activityFn).
		Parallel().
		MaxConcurrency(3).
		FailFast(false)

	for _, env := range environments {
		env := env // capture loop variable
		input := payload.FunctionExecutionInput{
			Name: "validate-config",
			Args: map[string]string{
				"env":       env,
				"timestamp": time.Now().Format(time.RFC3339),
			},
		}
		wb.Add(&input)
	}

	def, err := wb.Build()
	if err != nil {
		log.Printf("Failed to build dynamic validation: %v", err)
		return
	}

	parallelInput := def.NewInput()

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "dynamic-validation-example",
			TaskQueue: def.TaskQueue,
		},
		workflow.ParallelFunctionsWorkflow,
		parallelInput,
	)
	if err != nil {
		log.Printf("Failed to start dynamic validation: %v", err)
		return
	}

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	if err := we.Get(ctx, &result); err != nil {
		log.Printf("Dynamic validation failed: %v", err)
		return
	}

	log.Printf("Dynamic validation completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)

	for _, r := range result.Results {
		log.Printf("  %s (env=%s): Success=%v", r.Name, r.Result["env"], r.Success)
	}
}
