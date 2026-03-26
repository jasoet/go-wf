package datasync

import (
	"context"
	"fmt"

	pkgotel "github.com/jasoet/pkg/v2/otel"
)

// MapResult holds the output of a detailed mapping operation, including
// successfully converted records and information about skipped records.
type MapResult[U any] struct {
	Records     []U      `json:"records"`
	Skipped     int      `json:"skipped"`
	SkipReasons []string `json:"skipReasons,omitempty"`
}

// DetailedMapper extends Mapper with a MapDetailed method that returns
// a MapResult containing skip tracking information.
type DetailedMapper[T any, U any] interface {
	Mapper[T, U]
	MapDetailed(ctx context.Context, records []T) MapResult[U]
}

// RecordMapFunc converts a single source record into a sink record.
// Return an error to skip the record.
type RecordMapFunc[T any, U any] func(record *T) (U, error)

// RecordMapper implements both Mapper and DetailedMapper by applying a
// per-record conversion function. Records that return an error are skipped
// with a warning log instead of failing the entire batch.
type RecordMapper[T any, U any] struct {
	name string
	fn   RecordMapFunc[T, U]
}

// NewRecordMapper creates a RecordMapper with the given name and per-record function.
func NewRecordMapper[T any, U any](name string, fn RecordMapFunc[T, U]) *RecordMapper[T, U] {
	return &RecordMapper[T, U]{
		name: name,
		fn:   fn,
	}
}

// Map satisfies the Mapper interface. It calls MapDetailed internally and
// returns only the successfully converted records.
func (m *RecordMapper[T, U]) Map(ctx context.Context, records []T) ([]U, error) {
	result := m.MapDetailed(ctx, records)
	return result.Records, nil
}

// MapDetailed iterates over each record, calls the conversion function, and
// collects results. Records that return an error are skipped with a warning log.
func (m *RecordMapper[T, U]) MapDetailed(ctx context.Context, records []T) MapResult[U] {
	logger := pkgotel.NewLogHelper(ctx, pkgotel.ConfigFromContext(ctx), "datasync.mapper", m.name)
	result := MapResult[U]{
		Records: make([]U, 0, len(records)),
	}

	for i := range records {
		mapped, err := m.fn(&records[i])
		if err != nil {
			result.Skipped++
			reason := fmt.Sprintf("record %d: %s", i, err.Error())
			result.SkipReasons = append(result.SkipReasons, reason)
			logger.Warn("skipping record",
				pkgotel.F("index", i),
				pkgotel.F("error", err.Error()))
			continue
		}
		result.Records = append(result.Records, mapped)
	}

	return result
}
