//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/chunk"
	"github.com/jasoet/go-wf/datasync/payload"
)

// This example demonstrates a date-chunked datasync job:
// fetch orders per day partition, identity-map, write to an in-memory sink.

// Order represents an order record.
type Order struct {
	ID     int       `json:"id"`
	Date   time.Time `json:"date"`
	Item   string    `json:"item"`
	Amount float64   `json:"amount"`
}

// OrderFetcher synthesises 5 orders for the requested date range.
func OrderFetcher(ctx context.Context, start, end time.Time) ([]Order, error) {
	log.Printf("[fetcher] Fetching orders for range %s .. %s",
		start.Format("2006-01-02 15:04"),
		end.Format("2006-01-02 15:04"),
	)
	mid := start
	orders := make([]Order, 0, 5)
	for i := 0; i < 5; i++ {
		orders = append(orders, Order{
			ID:     i + 1,
			Date:   mid,
			Item:   fmt.Sprintf("item-%d", i+1),
			Amount: float64(i+1) * 10.0,
		})
	}
	return orders, nil
}

// OrderSink logs each write and returns the number of inserted records.
type OrderSink struct{}

func (s *OrderSink) Name() string { return "in-memory-order-sink" }

func (s *OrderSink) Write(_ context.Context, records []Order) (datasync.WriteResult, error) {
	for _, o := range records {
		log.Printf("[sink] Writing order: ID=%d, Date=%s, Item=%s, Amount=%.2f",
			o.ID, o.Date.Format("2006-01-02"), o.Item, o.Amount)
	}
	return datasync.WriteResult{Inserted: len(records)}, nil
}

func main() {
	// Create Temporal client.
	c, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Build the date-chunked sync registration.
	// LookBack(72h) with ChunkSize(24h) produces 3 daily partitions.
	var fetcher chunk.TimeFetcher[Order] = OrderFetcher
	def, err := chunk.NewDateChunkedSync[Order, Order]("order-sync").
		LookBack(72 * time.Hour).
		ChunkSize(24 * time.Hour).
		Timezone(time.UTC).
		Fetcher(fetcher).
		Mapper(datasync.IdentityMapper[Order]()).
		Sink(&OrderSink{}).
		Build()
	if err != nil {
		log.Fatalf("Failed to build chunked sync: %v", err)
	}

	// Create and start worker.
	w := worker.New(c, def.TaskQueue, worker.Options{})
	def.Register(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Execute the chunked sync workflow.
	input := def.NewInput().(*payload.SyncExecutionInput)
	input.JobName = def.Name
	run, err := def.Execute(context.Background(), c, input)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	log.Printf("Started workflow ID: %s, RunID: %s", run.WorkflowID, run.RunID)

	var result chunk.SyncResult[int64]
	if err := run.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	fmt.Printf("Chunked sync completed!\n")
	fmt.Printf("  Job:              %s\n", result.JobName)
	fmt.Printf("  Partitions:       %d\n", result.TotalPartitions)
	fmt.Printf("  Total fetched:    %d\n", result.TotalFetched)
	fmt.Printf("  Total inserted:   %d\n", result.TotalInserted)
	fmt.Println()
	fmt.Println("Per-partition summary:")
	for i, p := range result.Partitions {
		start := chunk.KeyToTime(p.Start).Format("2006-01-02 15:04")
		end := chunk.KeyToTime(p.End).Format("2006-01-02 15:04")
		fmt.Printf("  [%d] %s .. %s  fetched=%d inserted=%d\n",
			i+1, start, end, p.Fetched, p.Inserted)
	}
}
