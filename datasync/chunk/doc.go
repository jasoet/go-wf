// Package chunk provides Temporal-backed partitioned-sync workflows on top of
// the datasync primitives (Mapper, Sink, FullJobRegistration).
//
// A ChunkedSync walks a list of Partition[K] in order, running fetch -> map ->
// write per partition. Optionally, a ProgressTracker[K] persists progress so
// that long-running syncs resume after failure rather than restart from the
// beginning of the range.
//
// For time-based partitioning, use DateChunkedSync — a thin wrapper that
// adapts time.Time keys to the int64 (Unix-nano) representation required by
// the generic builder (cmp.Ordered does not include time.Time).
//
// Example (date-based, with tracker):
//
//	reg := chunk.NewDateChunkedSync[goers.Order, db.OrderRow]("orders-sync").
//	    LookBack(7 * 24 * time.Hour).
//	    ChunkSize(24 * time.Hour).
//	    Timezone(time.UTC).
//	    Fetcher(myGoersFetcher).
//	    Mapper(myMapper).
//	    Sink(myDBSink).
//	    WithTracker(myPostgresTracker).
//	    Schedule(15 * time.Minute).
//	    MaxPartitionsPerExecution(50).
//	    Build()
//
//	// reg is a datasync/workflow.FullJobRegistration — register with a worker
//	// alongside other jobs from datasync/workflow.
//
// The package does not provide tracker implementations. Define your own (e.g.,
// Postgres-backed) and pass it via WithTracker.
package chunk
