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

// This example demonstrates sequential orchestration of multiple sync jobs.
// Two independent sync jobs (users and orders) are executed one after the other
// using standard Temporal client calls.

// User and Order types for our two sync jobs.

type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type Order struct {
	ID     string  `json:"id"`
	UserID int     `json:"userId"`
	Total  float64 `json:"total"`
}

// --- User source/sink ---

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

type UserSink struct{}

func (s *UserSink) Name() string { return "user-sink" }

func (s *UserSink) Write(_ context.Context, records []User) (datasync.WriteResult, error) {
	log.Printf("[user-sink] Writing %d users", len(records))
	return datasync.WriteResult{Inserted: len(records)}, nil
}

// --- Order source/sink ---

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

	// Define two sync jobs
	userJob := datasync.Job[User, User]{
		Name:   "fetch-users",
		Source: &UserSource{},
		Mapper: datasync.IdentityMapper[User](),
		Sink:   &UserSink{},
	}

	orderJob := datasync.Job[Order, Order]{
		Name:   "fetch-orders",
		Source: &OrderSource{},
		Mapper: datasync.IdentityMapper[Order](),
		Sink:   &OrderSink{},
	}

	// Register both jobs on separate task queues
	userQueue := dsworkflow.TaskQueue(userJob.Name)
	orderQueue := dsworkflow.TaskQueue(orderJob.Name)

	w1 := worker.New(c, userQueue, worker.Options{})
	dsworkflow.RegisterJob(w1, userJob)

	w2 := worker.New(c, orderQueue, worker.Options{})
	dsworkflow.RegisterJob(w2, orderJob)

	go func() {
		if err := w1.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("User worker failed: %v", err)
		}
	}()
	defer w1.Stop()

	go func() {
		if err := w2.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Order worker failed: %v", err)
		}
	}()
	defer w2.Stop()

	time.Sleep(time.Second)

	ctx := context.Background()

	// Execute job 1: sync users
	log.Println("=== Step 1: Syncing users ===")
	userInput := dsworkflow.BuildWorkflowInput(userJob)
	we1, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "pipeline-fetch-users",
			TaskQueue: userQueue,
		},
		userJob.Name,
		userInput,
	)
	if err != nil {
		log.Fatalf("Failed to start user sync: %v", err)
	}

	var userResult payload.SyncExecutionOutput
	if err := we1.Get(ctx, &userResult); err != nil {
		log.Fatalf("User sync failed: %v", err)
	}
	log.Printf("  Users synced: fetched=%d, inserted=%d", userResult.TotalFetched, userResult.Inserted)

	// Execute job 2: sync orders (after users complete)
	log.Println("=== Step 2: Syncing orders ===")
	orderInput := dsworkflow.BuildWorkflowInput(orderJob)
	we2, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "pipeline-fetch-orders",
			TaskQueue: orderQueue,
		},
		orderJob.Name,
		orderInput,
	)
	if err != nil {
		log.Fatalf("Failed to start order sync: %v", err)
	}

	var orderResult payload.SyncExecutionOutput
	if err := we2.Get(ctx, &orderResult); err != nil {
		log.Fatalf("Order sync failed: %v", err)
	}
	log.Printf("  Orders synced: fetched=%d, inserted=%d", orderResult.TotalFetched, orderResult.Inserted)

	// Print aggregated results
	fmt.Printf("\nPipeline completed!\n")
	fmt.Printf("  Step 1 - %s: fetched=%d, inserted=%d, duration=%v\n",
		userResult.JobName, userResult.TotalFetched, userResult.Inserted, userResult.ProcessingTime)
	fmt.Printf("  Step 2 - %s: fetched=%d, inserted=%d, duration=%v\n",
		orderResult.JobName, orderResult.TotalFetched, orderResult.Inserted, orderResult.ProcessingTime)
	fmt.Printf("  Total records synced: %d\n", userResult.Inserted+orderResult.Inserted)
}
