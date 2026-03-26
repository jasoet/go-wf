package activity

import (
	"context"
	"fmt"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.temporal.io/sdk/activity"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/payload"
)

// ActivityInput is the activity input for the SyncData activity.
type ActivityInput struct {
	JobName    string `json:"jobName"`
	SourceName string `json:"sourceName"`
	SinkName   string `json:"sinkName"`
	Params     any    `json:"params,omitempty"`
}

// ActivityOutput is the activity output for the SyncData activity.
type ActivityOutput struct {
	TotalFetched int           `json:"totalFetched"`
	Inserted     int           `json:"inserted"`
	Updated      int           `json:"updated"`
	Skipped      int           `json:"skipped"`
	FetchTime    time.Duration `json:"fetchTime"`
	WriteTime    time.Duration `json:"writeTime"`
}

// Activities holds the source, mapper, and sink for a sync job's Temporal activities.
type Activities[T any, U any] struct {
	source datasync.Source[T]
	mapper datasync.Mapper[T, U]
	sink   datasync.Sink[U]
}

// NewActivities creates a new Activities instance.
func NewActivities[T any, U any](source datasync.Source[T], mapper datasync.Mapper[T, U], sink datasync.Sink[U]) *Activities[T, U] {
	return &Activities[T, U]{
		source: source,
		mapper: mapper,
		sink:   sink,
	}
}

// SyncData fetches records from source, maps them, and writes to sink.
func (a *Activities[T, U]) SyncData(ctx context.Context, input ActivityInput) (*ActivityOutput, error) {
	attrs := []attribute.KeyValue{
		attribute.String("job", input.JobName),
		attribute.String("source", input.SourceName),
		attribute.String("sink", input.SinkName),
	}
	start := time.Now()

	// Operations layer: orchestrate fetch -> map -> write
	lc := pkgotel.Layers.StartOperations(ctx, "datasync", "Execute",
		pkgotel.F("job", input.JobName),
		pkgotel.F("source", input.SourceName),
		pkgotel.F("sink", input.SinkName))
	defer lc.End()

	// === Fetch ===
	fetchStart := time.Now()
	fetchLC := pkgotel.Layers.StartService(lc.Context(), "datasync", "Fetch",
		pkgotel.F("source", input.SourceName))
	records, err := a.source.Fetch(fetchLC.Context())
	if err != nil {
		_ = fetchLC.Error(err, "source fetch failed")
		fetchLC.End()
		recordFailure(ctx, start, attrs)
		return nil, fmt.Errorf("source %s fetch failed: %w", a.source.Name(), err)
	}
	fetchLC.Success("fetch complete", pkgotel.F("records", len(records)))
	fetchLC.End()
	fetchTime := time.Since(fetchStart)

	syncRecordsFetched.Add(ctx, int64(len(records)), metric.WithAttributes(attrs...))
	activity.RecordHeartbeat(ctx, fmt.Sprintf("fetched %d records", len(records)))

	if len(records) == 0 {
		lc.Success("no records to sync")
		recordSuccess(ctx, start, attrs)
		return &ActivityOutput{FetchTime: fetchTime}, nil
	}

	// === Map ===
	mapLC := pkgotel.Layers.StartService(lc.Context(), "datasync", "Map",
		pkgotel.F("job", input.JobName),
		pkgotel.F("records", len(records)))
	mapped, err := a.mapper.Map(mapLC.Context(), records)
	if err != nil {
		_ = mapLC.Error(err, "mapper failed")
		mapLC.End()
		recordFailure(ctx, start, attrs)
		return nil, fmt.Errorf("mapper failed: %w", err)
	}
	mapLC.Success("map complete", pkgotel.F("mapped", len(mapped)))
	mapLC.End()

	// === Write ===
	writeStart := time.Now()
	writeLC := pkgotel.Layers.StartRepository(lc.Context(), "datasync", "Write",
		pkgotel.F("sink", input.SinkName),
		pkgotel.F("records", len(mapped)))
	wr, err := a.sink.Write(writeLC.Context(), mapped)
	if err != nil {
		_ = writeLC.Error(err, "sink write failed")
		writeLC.End()
		recordFailure(ctx, start, attrs)
		return nil, fmt.Errorf("sink %s write failed: %w", a.sink.Name(), err)
	}
	writeLC.Success("write complete",
		pkgotel.F("inserted", wr.Inserted),
		pkgotel.F("updated", wr.Updated),
		pkgotel.F("skipped", wr.Skipped))
	writeLC.End()
	writeTime := time.Since(writeStart)

	syncRecordsWritten.Add(ctx, int64(wr.Total()), metric.WithAttributes(attrs...))
	recordSuccess(ctx, start, attrs)

	lc.Success("sync complete",
		pkgotel.F("fetched", len(records)),
		pkgotel.F("inserted", wr.Inserted))

	activity.RecordHeartbeat(ctx, fmt.Sprintf("wrote %d records", wr.Total()))

	return &ActivityOutput{
		TotalFetched: len(records),
		Inserted:     wr.Inserted,
		Updated:      wr.Updated,
		Skipped:      wr.Skipped,
		FetchTime:    fetchTime,
		WriteTime:    writeTime,
	}, nil
}

// ToSyncExecutionOutput converts ActivityOutput to a payload.SyncExecutionOutput.
func ToSyncExecutionOutput(jobName string, ao *ActivityOutput, processingTime time.Duration, err error) payload.SyncExecutionOutput {
	if err != nil {
		return payload.SyncExecutionOutput{
			JobName: jobName,
			Success: false,
			Error:   err.Error(),
		}
	}
	return payload.SyncExecutionOutput{
		JobName:        jobName,
		TotalFetched:   ao.TotalFetched,
		Inserted:       ao.Inserted,
		Updated:        ao.Updated,
		Skipped:        ao.Skipped,
		ProcessingTime: processingTime,
		Success:        true,
	}
}

func recordSuccess(ctx context.Context, start time.Time, attrs []attribute.KeyValue) {
	syncOpsTotal.Add(ctx, 1, metric.WithAttributes(append(attrs, attribute.String("status", "success"))...))
	syncOpsDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
}

func recordFailure(ctx context.Context, start time.Time, attrs []attribute.KeyValue) {
	syncOpsTotal.Add(ctx, 1, metric.WithAttributes(append(attrs, attribute.String("status", "error"))...))
	syncOpsDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
}
