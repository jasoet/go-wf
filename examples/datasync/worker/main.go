//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/datasync"
	dsworkflow "github.com/jasoet/go-wf/datasync/workflow"
)

// Shared worker that registers multiple datasync jobs.
// Each job runs on its own task queue via a separate Temporal worker.
// Start this worker, then trigger workflows via Temporal CLI or UI.

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
		{ID: 4, Name: "Diana", Email: "diana@example.com"},
		{ID: 5, Name: "Eve", Email: "eve@example.com"},
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
		{ID: "ORD-003", UserID: 1, Total: 245.50},
		{ID: "ORD-004", UserID: 3, Total: 32.00},
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

	log.Println("Starting DataSync Temporal Worker...")

	// Define sync jobs
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

	// Create a separate worker for each job's task queue
	workerOpts := worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	}

	w1 := worker.New(c, dsworkflow.TaskQueue(userJob.Name), workerOpts)
	dsworkflow.RegisterJob(w1, userJob)

	w2 := worker.New(c, dsworkflow.TaskQueue(productJob.Name), workerOpts)
	dsworkflow.RegisterJob(w2, productJob)

	w3 := worker.New(c, dsworkflow.TaskQueue(orderJob.Name), workerOpts)
	dsworkflow.RegisterJob(w3, orderJob)

	log.Println("Registered sync jobs:")
	for _, name := range []string{userJob.Name, productJob.Name, orderJob.Name} {
		log.Printf("  - %s (queue: %s)", name, dsworkflow.TaskQueue(name))
	}

	// Start all workers in background
	for i, w := range []worker.Worker{w1, w2, w3} {
		w := w
		i := i
		go func() {
			if err := w.Start(); err != nil {
				log.Fatalf("Worker %d failed to start: %v", i, err)
			}
		}()
	}

	fmt.Println()
	log.Println("All workers started. Waiting for sync workflows...")
	log.Println("Trigger with: temporal workflow start --task-queue sync-sync-users --type sync-users")

	// Block until interrupted
	<-worker.InterruptCh()

	log.Println("Shutting down workers...")
	w1.Stop()
	w2.Stop()
	w3.Stop()
	log.Println("Workers stopped")
}
