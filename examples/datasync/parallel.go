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

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/payload"
	dsworkflow "github.com/jasoet/go-wf/datasync/workflow"
)

// This example demonstrates parallel execution of multiple sync jobs.
// Three independent data sources are synced concurrently.

// --- Data types ---

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Product struct {
	SKU   string  `json:"sku"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type Order struct {
	ID     string  `json:"id"`
	UserID int     `json:"userId"`
	Total  float64 `json:"total"`
}

// --- Sources ---

type UserSource struct{}

func (s *UserSource) Name() string { return "user-source" }

func (s *UserSource) Fetch(_ context.Context) ([]User, error) {
	log.Println("[user-source] Fetching users...")
	return []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
	}, nil
}

type ProductSource struct{}

func (s *ProductSource) Name() string { return "product-source" }

func (s *ProductSource) Fetch(_ context.Context) ([]Product, error) {
	log.Println("[product-source] Fetching products...")
	return []Product{
		{SKU: "SKU-001", Name: "Widget", Price: 19.99},
		{SKU: "SKU-002", Name: "Gadget", Price: 49.99},
		{SKU: "SKU-003", Name: "Doohickey", Price: 9.99},
		{SKU: "SKU-004", Name: "Thingamajig", Price: 29.99},
	}, nil
}

type OrderSource struct{}

func (s *OrderSource) Name() string { return "order-source" }

func (s *OrderSource) Fetch(_ context.Context) ([]Order, error) {
	log.Println("[order-source] Fetching orders...")
	return []Order{
		{ID: "ORD-001", UserID: 1, Total: 150.00},
		{ID: "ORD-002", UserID: 2, Total: 89.99},
	}, nil
}

// --- Sinks ---

type UserSink struct{}

func (s *UserSink) Name() string { return "user-sink" }

func (s *UserSink) Write(_ context.Context, records []User) (datasync.WriteResult, error) {
	log.Printf("[user-sink] Writing %d users", len(records))
	return datasync.WriteResult{Inserted: len(records)}, nil
}

type ProductSink struct{}

func (s *ProductSink) Name() string { return "product-sink" }

func (s *ProductSink) Write(_ context.Context, records []Product) (datasync.WriteResult, error) {
	log.Printf("[product-sink] Writing %d products", len(records))
	return datasync.WriteResult{Inserted: len(records)}, nil
}

type OrderSink struct{}

func (s *OrderSink) Name() string { return "order-sink" }

func (s *OrderSink) Write(_ context.Context, records []Order) (datasync.WriteResult, error) {
	log.Printf("[order-sink] Writing %d orders", len(records))
	return datasync.WriteResult{Inserted: len(records)}, nil
}

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

	// Define three sync jobs
	userJob := datasync.Job[User, User]{
		Name: "sync-users", Source: &UserSource{},
		Mapper: datasync.IdentityMapper[User](), Sink: &UserSink{},
	}
	productJob := datasync.Job[Product, Product]{
		Name: "sync-products", Source: &ProductSource{},
		Mapper: datasync.IdentityMapper[Product](), Sink: &ProductSink{},
	}
	orderJob := datasync.Job[Order, Order]{
		Name: "sync-orders", Source: &OrderSource{},
		Mapper: datasync.IdentityMapper[Order](), Sink: &OrderSink{},
	}

	// Register each job on its own task queue and worker
	type jobWorker struct {
		w       worker.Worker
		queue   string
		name    string
		wfInput payload.SyncExecutionInput
	}

	jobs := []jobWorker{
		func() jobWorker {
			q := dsworkflow.TaskQueue(userJob.Name)
			w := worker.New(c, q, worker.Options{})
			dsworkflow.RegisterJob(w, userJob)
			return jobWorker{w: w, queue: q, name: userJob.Name, wfInput: dsworkflow.BuildWorkflowInput(userJob)}
		}(),
		func() jobWorker {
			q := dsworkflow.TaskQueue(productJob.Name)
			w := worker.New(c, q, worker.Options{})
			dsworkflow.RegisterJob(w, productJob)
			return jobWorker{w: w, queue: q, name: productJob.Name, wfInput: dsworkflow.BuildWorkflowInput(productJob)}
		}(),
		func() jobWorker {
			q := dsworkflow.TaskQueue(orderJob.Name)
			w := worker.New(c, q, worker.Options{})
			dsworkflow.RegisterJob(w, orderJob)
			return jobWorker{w: w, queue: q, name: orderJob.Name, wfInput: dsworkflow.BuildWorkflowInput(orderJob)}
		}(),
	}

	// Start all workers
	for _, j := range jobs {
		j := j
		go func() {
			if err := j.w.Run(worker.InterruptCh()); err != nil {
				log.Fatalf("Worker %s failed: %v", j.name, err)
			}
		}()
		defer j.w.Stop()
	}

	time.Sleep(time.Second)

	ctx := context.Background()

	// Start all three workflows concurrently
	log.Println("Starting all sync jobs in parallel...")
	type workflowRun struct {
		name string
		run  client.WorkflowRun
	}

	var runs []workflowRun
	for _, j := range jobs {
		we, err := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{
				ID:        "parallel-" + j.name,
				TaskQueue: j.queue,
			},
			j.name,
			j.wfInput,
		)
		if err != nil {
			log.Fatalf("Failed to start %s: %v", j.name, err)
		}
		log.Printf("  Started %s (RunID: %s)", j.name, we.GetRunID())
		runs = append(runs, workflowRun{name: j.name, run: we})
	}

	// Wait for all to complete and collect results
	var results []payload.SyncExecutionOutput
	for _, r := range runs {
		var result payload.SyncExecutionOutput
		if err := r.run.Get(ctx, &result); err != nil {
			log.Fatalf("%s failed: %v", r.name, err)
		}
		results = append(results, result)
	}

	// Print aggregated results
	fmt.Printf("\nAll sync jobs completed!\n")
	totalInserted := 0
	for _, r := range results {
		fmt.Printf("  %s: fetched=%d, inserted=%d, duration=%v\n",
			r.JobName, r.TotalFetched, r.Inserted, r.ProcessingTime)
		totalInserted += r.Inserted
	}
	fmt.Printf("  Total records synced: %d\n", totalInserted)
}
