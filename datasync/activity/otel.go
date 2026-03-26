package activity

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

var (
	datasyncMeter = otel.Meter("go-wf/datasync")

	syncOpsTotal       metric.Int64Counter
	syncOpsDuration    metric.Float64Histogram
	syncRecordsFetched metric.Int64Counter
	syncRecordsWritten metric.Int64Counter
)

func init() {
	var err error
	syncOpsTotal, err = datasyncMeter.Int64Counter("go_wf.datasync.operations_total",
		metric.WithDescription("Total datasync operations by job and status"))
	mustInit(err)
	syncOpsDuration, err = datasyncMeter.Float64Histogram("go_wf.datasync.operation_duration_seconds",
		metric.WithDescription("Datasync operation duration in seconds"), metric.WithUnit("s"))
	mustInit(err)
	syncRecordsFetched, err = datasyncMeter.Int64Counter("go_wf.datasync.records_fetched",
		metric.WithDescription("Records fetched from source"))
	mustInit(err)
	syncRecordsWritten, err = datasyncMeter.Int64Counter("go_wf.datasync.records_written",
		metric.WithDescription("Records written to sink"))
	mustInit(err)
}

func mustInit(err error) {
	if err != nil {
		panic("failed to create datasync metric: " + err.Error())
	}
}
