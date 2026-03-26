//go:build example

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"
	wf "go.temporal.io/sdk/workflow"

	fn "github.com/jasoet/go-wf/function"
	fnactivity "github.com/jasoet/go-wf/function/activity"
	fnpayload "github.com/jasoet/go-wf/function/payload"
	fnwf "github.com/jasoet/go-wf/function/workflow"
	"github.com/jasoet/go-wf/workflow/artifacts"
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

	log.Println("Starting Function Temporal Worker...")

	// Create worker
	w := worker.New(c, "function-tasks", worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	// Create function registry and register all handlers
	registry := fn.NewRegistry()
	registerAllHandlers(registry)

	// Create artifact stores for DAG examples
	localStore, err := createLocalArtifactStore()
	if err != nil {
		log.Printf("Warning: could not create local artifact store: %v", err)
	}

	s3Store := createS3ArtifactStore()

	// Register artifact-backed DAG workflows
	if localStore != nil {
		w.RegisterWorkflowWithOptions(
			newArtifactDAGWorkflow(localStore),
			wf.RegisterOptions{Name: "ArtifactDAGWorkflow-Local"},
		)
	}
	if s3Store != nil {
		w.RegisterWorkflowWithOptions(
			newArtifactDAGWorkflow(s3Store),
			wf.RegisterOptions{Name: "ArtifactDAGWorkflow-S3"},
		)
	}

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
	if localStore != nil {
		log.Println("  - ArtifactDAGWorkflow-Local")
	}
	if s3Store != nil {
		log.Println("  - ArtifactDAGWorkflow-S3")
	}
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

	_ = registry.Register("greet", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		name := input.Args["name"]
		if name == "" {
			name = "World"
		}
		return &fn.FunctionOutput{
			Result: map[string]string{"greeting": "Hello, " + name + "!"},
		}, nil
	})

	// --- From pipeline.go (3 handlers) ---

	_ = registry.Register("validate", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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

	_ = registry.Register("transform", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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

	_ = registry.Register("notify", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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

	// --- From loop.go (5 handlers) ---

	_ = registry.Register("process-csv", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		file := input.Args["file"]
		log.Printf("Processing CSV file: %s", file)
		return &fn.FunctionOutput{
			Result: map[string]string{"file": file, "status": "processed"},
		}, nil
	})

	_ = registry.Register("run-migration", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		migration := input.Args["migration"]
		log.Printf("Running migration: %s", migration)
		return &fn.FunctionOutput{
			Result: map[string]string{"migration": migration, "status": "applied"},
		}, nil
	})

	_ = registry.Register("deploy-service", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		env := input.Args["environment"]
		region := input.Args["region"]
		log.Printf("Deploying to %s in %s", env, region)
		return &fn.FunctionOutput{
			Result: map[string]string{"env": env, "region": region, "status": "deployed"},
		}, nil
	})

	_ = registry.Register("sync-tenant", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		tenant := input.Args["tenant"]
		log.Printf("Syncing tenant: %s", tenant)
		return &fn.FunctionOutput{
			Result: map[string]string{"tenant": tenant, "status": "synced"},
		}, nil
	})

	_ = registry.Register("health-check", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		service := input.Args["service"]
		env := input.Args["environment"]
		log.Printf("Health check: %s in %s", service, env)
		return &fn.FunctionOutput{
			Result: map[string]string{"service": service, "env": env, "healthy": "true"},
		}, nil
	})

	// --- From builder.go (6 handlers, with "transform" renamed to "etl-transform") ---

	_ = registry.Register("extract", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		source := input.Args["source"]
		log.Printf("[extract] Extracting data from %s", source)
		return &fn.FunctionOutput{
			Result: map[string]string{"records": "1500", "source": source},
		}, nil
	})

	_ = registry.Register("etl-transform", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		format := input.Args["format"]
		log.Printf("[etl-transform] Transforming data to %s format", format)
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

	// --- From dag.go (3 handlers) ---

	_ = registry.Register("compile", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[compile] Compiling application...")
		time.Sleep(300 * time.Millisecond)
		return &fn.FunctionOutput{
			Result: map[string]string{"artifact": "app-binary", "status": "compiled", "version": "1.0.0"},
		}, nil
	})

	_ = registry.Register("run-tests", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[run-tests] Running test suite...")
		time.Sleep(500 * time.Millisecond)
		return &fn.FunctionOutput{
			Result: map[string]string{"passed": "42", "failed": "0", "skipped": "3"},
		}, nil
	})

	_ = registry.Register("publish-artifact", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		artifactPath := input.Args["artifact_path"]
		log.Printf("[publish-artifact] Publishing artifact: %s", artifactPath)
		return &fn.FunctionOutput{
			Result: map[string]string{"published": "true", "registry": "artifacts.example.com", "artifact": artifactPath},
		}, nil
	})

	// --- Artifact demo handlers (3 handlers) ---

	_ = registry.Register("generate-report", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		reportType := input.Args["type"]
		if reportType == "" {
			reportType = "summary"
		}
		log.Printf("[generate-report] Generating %s report...", reportType)

		// Simulate generating report data
		reportData := fmt.Sprintf(`{"report_type":"%s","generated_at":"%s","records":150,"status":"complete"}`,
			reportType, time.Now().Format(time.RFC3339))

		return &fn.FunctionOutput{
			Result: map[string]string{"type": reportType, "records": "150", "status": "generated"},
			Data:   []byte(reportData),
		}, nil
	})

	_ = registry.Register("process-report", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Printf("[process-report] Processing report data (%d bytes)...", len(input.Data))

		// Simulate processing the report data
		processed := fmt.Sprintf(`{"original_size":%d,"processed_at":"%s","transformations":["filtered","aggregated","sorted"]}`,
			len(input.Data), time.Now().Format(time.RFC3339))

		return &fn.FunctionOutput{
			Result: map[string]string{"original_size": fmt.Sprintf("%d", len(input.Data)), "status": "processed"},
			Data:   []byte(processed),
		}, nil
	})

	_ = registry.Register("archive-report", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Printf("[archive-report] Archiving report data (%d bytes)...", len(input.Data))

		return &fn.FunctionOutput{
			Result: map[string]string{
				"archived":     "true",
				"archive_size": fmt.Sprintf("%d", len(input.Data)),
				"location":     "artifacts/reports/",
			},
		}, nil
	})

	log.Printf("Registered %d handler functions", 24)
}

func createLocalArtifactStore() (artifacts.ArtifactStore, error) {
	return artifacts.NewLocalFileStore("/tmp/go-wf-artifacts")
}

func createS3ArtifactStore() artifacts.ArtifactStore {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	endpoint := "localhost:9000"
	if e := os.Getenv("S3_ENDPOINT"); e != "" {
		endpoint = e
	}
	accessKey := "rustfsadmin"
	if k := os.Getenv("S3_ACCESS_KEY"); k != "" {
		accessKey = k
	}
	secretKey := "rustfsadmin"
	if k := os.Getenv("S3_SECRET_KEY"); k != "" {
		secretKey = k
	}

	store, err := artifacts.NewS3Store(ctx, artifacts.S3Config{
		Endpoint:  endpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
		Bucket:    "go-wf-artifacts",
		Prefix:    "functions/",
		UseSSL:    false,
	})
	if err != nil {
		log.Printf("Warning: S3-compatible storage not available, skipping artifact store: %v", err)
		return nil
	}
	return store
}

func newArtifactDAGWorkflow(store artifacts.ArtifactStore) func(wf.Context, fnpayload.DAGWorkflowInput) (*fnpayload.FunctionDAGWorkflowOutput, error) {
	return func(ctx wf.Context, input fnpayload.DAGWorkflowInput) (*fnpayload.FunctionDAGWorkflowOutput, error) {
		input.ArtifactStore = store
		return fnwf.InstrumentedDAGWorkflow(ctx, input)
	}
}
