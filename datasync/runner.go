package datasync

import (
	"context"
	"fmt"
	"time"
)

// Runner orchestrates a single fetch-map-write cycle.
// Use for testing and simple in-process sync without Temporal.
type Runner[T any, U any] struct {
	source Source[T]
	mapper Mapper[T, U]
	sink   Sink[U]
}

func NewRunner[T any, U any](source Source[T], mapper Mapper[T, U], sink Sink[U]) *Runner[T, U] {
	return &Runner[T, U]{source: source, mapper: mapper, sink: sink}
}

func (r *Runner[T, U]) Run(ctx context.Context) (*Result, error) {
	start := time.Now()

	records, err := r.source.Fetch(ctx)
	if err != nil {
		return nil, fmt.Errorf("source %s fetch failed: %w", r.source.Name(), err)
	}

	result := &Result{TotalFetched: len(records)}

	if len(records) == 0 {
		result.ProcessingTime = time.Since(start)
		return result, nil
	}

	mapped, err := r.mapper.Map(ctx, records)
	if err != nil {
		return nil, fmt.Errorf("mapper failed: %w", err)
	}

	wr, err := r.sink.Write(ctx, mapped)
	if err != nil {
		return nil, fmt.Errorf("sink %s write failed: %w", r.sink.Name(), err)
	}

	result.WriteResult = wr
	result.ProcessingTime = time.Since(start)
	return result, nil
}
