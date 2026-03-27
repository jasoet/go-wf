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

// This example demonstrates a basic datasync job:
// fetch users from an in-memory source, identity-map, write to an in-memory sink.

// User represents a user record.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserSource fetches users from an in-memory store.
type UserSource struct{}

func (s *UserSource) Name() string { return "in-memory-users" }

func (s *UserSource) Fetch(_ context.Context) ([]User, error) {
	log.Println("[source] Fetching users...")
	return []User{
		{ID: 1, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Name: "Charlie", Email: "charlie@example.com"},
		{ID: 4, Name: "Diana", Email: "diana@example.com"},
		{ID: 5, Name: "Eve", Email: "eve@example.com"},
	}, nil
}

// UserSink writes users to an in-memory destination.
type UserSink struct{}

func (s *UserSink) Name() string { return "in-memory-user-sink" }

func (s *UserSink) Write(_ context.Context, records []User) (datasync.WriteResult, error) {
	for _, u := range records {
		log.Printf("[sink] Writing user: ID=%d, Name=%s, Email=%s", u.ID, u.Name, u.Email)
	}
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

	// Define sync job with identity mapper
	job := datasync.Job[User, User]{
		Name:   "user-sync",
		Source: &UserSource{},
		Mapper: datasync.IdentityMapper[User](),
		Sink:   &UserSink{},
	}

	// Create and start worker
	taskQueue := dsworkflow.TaskQueue("user-sync")
	w := worker.New(c, taskQueue, worker.Options{})
	dsworkflow.RegisterJob(w, job)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Execute sync workflow
	input := dsworkflow.BuildWorkflowInput(job)
	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "user-sync-example",
			TaskQueue: taskQueue,
		},
		job.Name,
		input,
	)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started workflow ID: %s, RunID: %s", we.GetID(), we.GetRunID())

	var result payload.SyncExecutionOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("Sync completed!\n")
	fmt.Printf("  Job: %s\n", result.JobName)
	fmt.Printf("  Success: %v\n", result.Success)
	fmt.Printf("  Fetched: %d\n", result.TotalFetched)
	fmt.Printf("  Inserted: %d\n", result.Inserted)
	fmt.Printf("  Updated: %d\n", result.Updated)
	fmt.Printf("  Skipped: %d\n", result.Skipped)
	fmt.Printf("  Duration: %v\n", result.ProcessingTime)
}
