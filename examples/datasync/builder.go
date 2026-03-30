//go:build example

// This example demonstrates the fluent builder API for constructing datasync jobs.
// It shows record mapping with custom transformations from User to UserDTO.
//
// Run: task example:datasync -- builder.go

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
	"github.com/jasoet/go-wf/datasync/builder"
	"github.com/jasoet/go-wf/datasync/payload"
	dsworkflow "github.com/jasoet/go-wf/datasync/workflow"
)

// This example demonstrates the fluent builder API for constructing sync jobs.
// It transforms User records into UserDTO using a custom RecordMapper.

// User represents a source user record.
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserDTO represents the transformed output.
type UserDTO struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
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

// UserDTOSink writes UserDTO records to an in-memory destination.
type UserDTOSink struct{}

func (s *UserDTOSink) Name() string { return "in-memory-dto-sink" }

func (s *UserDTOSink) Write(_ context.Context, records []UserDTO) (datasync.WriteResult, error) {
	for _, dto := range records {
		log.Printf("[sink] Writing DTO: DisplayName=%s, Email=%s", dto.DisplayName, dto.Email)
	}
	return datasync.WriteResult{Inserted: len(records)}, nil
}

// RecordMapper transforms User records into UserDTO records.
type RecordMapper struct{}

func (m *RecordMapper) Map(_ context.Context, records []User) ([]UserDTO, error) {
	dtos := make([]UserDTO, len(records))
	for i, u := range records {
		dtos[i] = UserDTO{
			DisplayName: fmt.Sprintf("%s (#%d)", u.Name, u.ID),
			Email:       u.Email,
		}
	}
	log.Printf("[mapper] Transformed %d users to DTOs", len(dtos))
	return dtos, nil
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

	// Build sync job using the fluent builder API
	job, err := builder.NewSyncJobBuilder[User, UserDTO]("user-dto-sync").
		WithSource(&UserSource{}).
		WithMapper(&RecordMapper{}).
		WithSink(&UserDTOSink{}).
		WithSchedule(10 * time.Minute).
		WithMetadata(map[string]string{
			"team":        "platform",
			"description": "Sync users to DTO format",
		}).
		WithMaxRetries(5).
		WithActivityTimeout(3 * time.Minute).
		Build()
	if err != nil {
		log.Fatalf("Failed to build job: %v", err)
	}

	// Create and start worker
	taskQueue := dsworkflow.TaskQueue(job.Name)
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
			ID:        "user-dto-sync-example",
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
	fmt.Printf("  Duration: %v\n", result.ProcessingTime)
}
