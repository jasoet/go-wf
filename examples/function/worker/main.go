//go:build example

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/function"
	fnactivity "github.com/jasoet/go-wf/function/activity"
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

	log.Println("Starting Function Temporal Worker...")

	// Create worker
	w := worker.New(c, "function-tasks", worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Create function registry and register all handlers
	registry := fn.NewRegistry()
	registerAllHandlers(registry)

	// Register workflows and activity
	fn.RegisterWorkflows(w)
	fn.RegisterActivity(w, fnactivity.NewExecuteFunctionActivity(registry))

	log.Println("Registered workflows:")
	log.Println("  - ExecuteFunctionWorkflow")
	log.Println("  - FunctionPipelineWorkflow")
	log.Println("  - ParallelFunctionsWorkflow")
	log.Println("  - LoopWorkflow")
	log.Println("  - ParameterizedLoopWorkflow")
	log.Println("  - InstrumentedDAGWorkflow")
	log.Println()
	log.Println("Registered activities:")
	log.Println("  - ExecuteFunctionActivity")
	log.Println()
	log.Println("Worker listening on task queue: function-tasks")

	// Run worker (blocks until interrupted)
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}

	log.Println("Worker stopped")
}

func registerAllHandlers(registry *fn.Registry) {
	// --- From basic.go (1 handler) ---

	registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		name := input.Args["name"]
		if name == "" {
			name = "World"
		}
		return &fn.FunctionOutput{
			Result: map[string]string{"greeting": "Hello, " + name + "!"},
		}, nil
	})

	// --- From pipeline.go (3 handlers) ---

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

	// --- From parallel.go (3 handlers) ---

	registry.Register("fetch-users", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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

	registry.Register("fetch-orders", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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

	registry.Register("fetch-inventory", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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

	// --- From loop.go (5 handlers) ---

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

	// --- From builder.go (6 handlers, with "transform" renamed to "etl-transform") ---

	registry.Register("extract", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		source := input.Args["source"]
		log.Printf("[extract] Extracting data from %s", source)
		return &fn.FunctionOutput{
			Result: map[string]string{"records": "1500", "source": source},
		}, nil
	})

	registry.Register("etl-transform", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		format := input.Args["format"]
		log.Printf("[etl-transform] Transforming data to %s format", format)
		return &fn.FunctionOutput{
			Result: map[string]string{"format": format, "records": "1480", "dropped": "20"},
		}, nil
	})

	registry.Register("load", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		target := input.Args["target"]
		log.Printf("[load] Loading data into %s", target)
		return &fn.FunctionOutput{
			Result: map[string]string{"target": target, "loaded": "1480"},
		}, nil
	})

	registry.Register("validate-config", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		env := input.Args["env"]
		log.Printf("[validate-config] Validating config for %s", env)
		return &fn.FunctionOutput{
			Result: map[string]string{"env": env, "valid": "true"},
		}, nil
	})

	registry.Register("check-deps", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		service := input.Args["service"]
		log.Printf("[check-deps] Checking dependencies for %s", service)
		return &fn.FunctionOutput{
			Result: map[string]string{"service": service, "healthy": "true"},
		}, nil
	})

	registry.Register("run-smoke-tests", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		target := input.Args["target"]
		log.Printf("[run-smoke-tests] Running smoke tests against %s", target)
		return &fn.FunctionOutput{
			Result: map[string]string{"target": target, "passed": "12", "failed": "0"},
		}, nil
	})

	// --- From dag.go (3 handlers) ---

	registry.Register("compile", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[compile] Compiling application...")
		time.Sleep(300 * time.Millisecond)
		return &fn.FunctionOutput{
			Result: map[string]string{"artifact": "app-binary", "status": "compiled", "version": "1.0.0"},
		}, nil
	})

	registry.Register("run-tests", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[run-tests] Running test suite...")
		time.Sleep(500 * time.Millisecond)
		return &fn.FunctionOutput{
			Result: map[string]string{"passed": "42", "failed": "0", "skipped": "3"},
		}, nil
	})

	registry.Register("publish-artifact", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		artifactPath := input.Args["artifact_path"]
		log.Printf("[publish-artifact] Publishing artifact: %s", artifactPath)
		return &fn.FunctionOutput{
			Result: map[string]string{"published": "true", "registry": "artifacts.example.com", "artifact": artifactPath},
		}, nil
	})

	log.Printf("Registered %d handler functions", 21)
}
