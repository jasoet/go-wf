package datasync

import (
	"context"
	"fmt"

	pkgotel "github.com/jasoet/pkg/v2/otel"
)

// FindFunc looks up a record by its ID. It returns nil if the record does not exist.
type FindFunc[U any, ID comparable] func(ctx context.Context, id ID) (*U, error)

// CreateFunc persists a new record.
type CreateFunc[U any] func(ctx context.Context, record *U) error

// InsertIfAbsentSink implements Sink[U] with a find-by-ID -> skip-if-exists -> create pattern.
type InsertIfAbsentSink[U any, ID comparable] struct {
	name   string
	getID  func(r *U) ID
	find   FindFunc[U, ID]
	create CreateFunc[U]
}

// NewInsertIfAbsentSink creates a new InsertIfAbsentSink.
func NewInsertIfAbsentSink[U any, ID comparable](
	name string,
	getID func(r *U) ID,
	find FindFunc[U, ID],
	create CreateFunc[U],
) *InsertIfAbsentSink[U, ID] {
	return &InsertIfAbsentSink[U, ID]{
		name:   name,
		getID:  getID,
		find:   find,
		create: create,
	}
}

// Name returns the sink's identifier.
func (s *InsertIfAbsentSink[U, ID]) Name() string {
	return s.name
}

// Write iterates records, skipping those that already exist and creating the rest.
func (s *InsertIfAbsentSink[U, ID]) Write(ctx context.Context, records []U) (WriteResult, error) {
	logger := pkgotel.NewLogHelper(ctx, pkgotel.ConfigFromContext(ctx), "datasync.sink", s.name)
	var result WriteResult

	for i := range records {
		record := &records[i]
		id := s.getID(record)

		existing, err := s.find(ctx, id)
		if err != nil {
			return result, fmt.Errorf("%s: find record %v: %w", s.name, id, err)
		}

		if existing != nil {
			result.Skipped++
			logger.Debug("record already exists, skipping", pkgotel.F("id", id))
			continue
		}

		if err := s.create(ctx, record); err != nil {
			return result, fmt.Errorf("%s: create record %v: %w", s.name, id, err)
		}

		result.Inserted++
	}

	logger.Debug("write complete",
		pkgotel.F("inserted", result.Inserted),
		pkgotel.F("skipped", result.Skipped),
		pkgotel.F("total", result.Total()))

	return result, nil
}
