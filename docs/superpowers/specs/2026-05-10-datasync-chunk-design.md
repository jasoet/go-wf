# Design: Generic Partitioned Sync for go-wf

**Date:** 2026-05-10
**Status:** Revised — pending re-approval (resolves review blockers from initial draft)
**Scope:** Add `datasync/chunk/` package to go-wf providing generic partitioned sync workflows built on Temporal.

## Context

para-sync's `internal/syncwf/` package provides date-range chunked sync workflows (fetch → map → write per chunk) with resumable backfill support. The pattern is proven but currently lives in an application-specific package. This design upstreams the pattern into go-wf as a generic library feature.

## Goals

- Provide a reusable partitioned sync pattern in go-wf
- Support any ordered partition key type (dates, offsets, IDs)
- Leave room for opaque-token pagination as a future separate pattern
- Reuse existing go-wf datasync primitives (`Source`, `Mapper`, `Sink`, `Runner`)
- Keep Temporal-specific code isolated from pure interfaces
- Do not modify para-sync until go-wf implementation is tested and stable

## Non-Goals

- Opaque-token pagination (out of scope; different pattern)
- Postgres tracker implementation (stays in para-sync)
- Application-specific fetchers (stays in para-sync)
- Modifying para-sync as part of this work

---

## Architecture & Package Layout

```
datasync/
  internal/
    heartbeat/
      heartbeat.go      // Loop, Interval — shared by activity/ and chunk/
      heartbeat_test.go
  chunk/
    doc.go              // Package godoc
    key.go              // KeyOf interface, time<->int64 conversion helpers
    partitioner.go      // Partitioner[K], DatePartitioner
    fetcher.go          // PartitionFetcher[T, K]
    tracker.go          // ProgressTracker[K] interface
    result.go           // PartitionResult[K], SyncResult[K]
    limiter.go          // RateLimitOpts, WithRateLimitRetry
    sleeper.go          // HeartbeatSleeper
    iterate.go          // IteratePartitions (non-Temporal helper)
    sync.go             // ChunkedSync builder + workflow
    date_sync.go        // DateChunkedSync convenience wrapper
```

**Principles:**
- No Temporal SDK imports in `partitioner.go`, `fetcher.go`, `tracker.go`, `result.go`, `key.go`
- Temporal-specific code (workflow builders, activity options) lives in `sync.go` and `date_sync.go`
- `PostgresProgressTracker` stays in para-sync — go-wf defines the interface only
- Heartbeat helpers live in `datasync/internal/heartbeat` so both `datasync/activity` and `datasync/chunk` can use them without exporting them publicly

---

## Type Constraint Decision (resolves review blocker)

The initial draft used `constraints.Ordered` from `golang.org/x/exp/constraints`. Two corrections:

1. **Use `cmp.Ordered` from stdlib**, not `constraints.Ordered`. `cmp.Ordered` has been in stdlib since Go 1.21; this project is on Go 1.26.
2. **`time.Time` does not satisfy `cmp.Ordered`** (it is a struct, not a primitive). The generic builder cannot directly accept `K = time.Time` and use `<` / `>=` for cursor filtering.

**Resolution:** the generic builder constrains `K cmp.Ordered`. `DateChunkedSync` is a wrapper that internally uses `K = int64` (Unix nanoseconds). All user-facing methods accept `time.Time`; the wrapper converts at the boundary. Conversion utilities live in `key.go`.

This means `DateChunkedSync` is not a paper-thin re-export — it owns one type-erasure layer (time.Time ↔ int64) — but the generic `ChunkedSync` itself stays simple and uses native Go ordering operators.

```go
// key.go
package chunk

func TimeToKey(t time.Time) int64 { return t.UnixNano() }
func KeyToTime(k int64) time.Time { return time.Unix(0, k).UTC() }
```

---

## Core Interfaces & Types

### Partitioner

```go
package chunk

import "cmp"

// Partitioner generates a sequence of partition keys for a sync run.
// Implementations must be deterministic given the same workflow.Now snapshot
// and tracker cursor — the result is recorded in workflow history.
type Partitioner[K cmp.Ordered] interface {
    Partitions(ctx context.Context) ([]Partition[K], error)
}

type Partition[K cmp.Ordered] struct {
    Start K
    End   K
}
```

**Constraint on K:** `cmp.Ordered` (allowed primitives) AND must be JSON-encodable (it crosses Temporal activity boundaries). Standard primitives satisfy both.

### PartitionFetcher

```go
// PartitionFetcher fetches records for a single partition [start, end).
type PartitionFetcher[T any, K cmp.Ordered] func(
    ctx context.Context,
    start, end K,
) ([]T, error)
```

### ProgressTracker

```go
// ProgressTracker persists progress so a sync can resume after failure.
type ProgressTracker[K cmp.Ordered] interface {
    Cursor(ctx context.Context, jobName string) (K, bool, error) // (cursor, exists, err)
    Advance(ctx context.Context, jobName string, completed K) error
}
```

The `bool` return distinguishes "no cursor recorded yet" from "cursor is the zero value of K" (e.g., `int64(0)` is a meaningful value).

### Results

```go
// PartitionResult preserves the typed key end-to-end so callers can program
// against it without re-parsing strings.
type PartitionResult[K cmp.Ordered] struct {
    Start    K
    End      K
    Fetched  int
    Inserted int
    Updated  int
    Skipped  int
}

type SyncResult[K cmp.Ordered] struct {
    JobName         string
    TotalPartitions int
    TotalFetched    int
    TotalInserted   int
    TotalUpdated    int
    TotalSkipped    int
    Partitions      []PartitionResult[K]
}
```

`FullJobRegistration` itself remains type-erased (no `K` parameter) — only the workflow's typed return value carries `K`. Callers that want a non-generic view of results can use a small adapter.

---

## Builder API

### ChunkedSync

```go
type ChunkedSync[In, Out any, K cmp.Ordered] struct { ... }

func NewChunkedSync[In, Out any, K cmp.Ordered](name string) *ChunkedSync[In, Out, K]

func (c *ChunkedSync[In, Out, K]) Partitioner(p Partitioner[K]) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) Fetcher(f PartitionFetcher[In, K]) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) Mapper(m datasync.Mapper[In, Out]) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) Sink(s datasync.Sink[Out]) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) WithTracker(t ProgressTracker[K]) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) PartitionSleep(d time.Duration) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) Schedule(d time.Duration) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) RateLimitRetry(opts RateLimitOpts) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) ActivityRetry(p temporal.RetryPolicy) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) ActivityTimeouts(startToClose, heartbeat time.Duration) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) MaxPartitionsPerExecution(n int) *ChunkedSync[In, Out, K]
func (c *ChunkedSync[In, Out, K]) Disabled(b bool) *ChunkedSync[In, Out, K]

func (c *ChunkedSync[In, Out, K]) Build() datasyncwf.FullJobRegistration
```

**Naming changes from initial draft:**
- `ChunkSleep` → `PartitionSleep` (consistency with the rest of the API).
- New `MaxPartitionsPerExecution` and `ActivityTimeouts` (see below).

### DateChunkedSync

```go
type DateChunkedSync[In, Out any] struct {
    inner *ChunkedSync[In, Out, int64] // internal key is Unix nanos
}

func NewDateChunkedSync[In, Out any](name string) *DateChunkedSync[In, Out]

func (d *DateChunkedSync[In, Out]) LookBack(window time.Duration) *DateChunkedSync[In, Out]
func (d *DateChunkedSync[In, Out]) ChunkSize(size time.Duration) *DateChunkedSync[In, Out]
func (d *DateChunkedSync[In, Out]) Timezone(loc *time.Location) *DateChunkedSync[In, Out]
func (d *DateChunkedSync[In, Out]) Fetcher(f PartitionFetcher[In, time.Time]) *DateChunkedSync[In, Out]
func (d *DateChunkedSync[In, Out]) Mapper(m datasync.Mapper[In, Out]) *DateChunkedSync[In, Out]
func (d *DateChunkedSync[In, Out]) Sink(s datasync.Sink[Out]) *DateChunkedSync[In, Out]
func (d *DateChunkedSync[In, Out]) WithTracker(t ProgressTracker[time.Time]) *DateChunkedSync[In, Out]
// other options delegate to inner

func (d *DateChunkedSync[In, Out]) Build() datasyncwf.FullJobRegistration
```

**Conversion responsibilities:** `DateChunkedSync` adapts user-supplied `PartitionFetcher[In, time.Time]` and `ProgressTracker[time.Time]` to the inner `int64` representation. The user never sees Unix nanos.

**Key decisions:**
- `WithTracker` is optional. Without it, the workflow is stateless.
- `MaxPartitionsPerExecution(n)`: when set and a partition list exceeds n, the workflow processes the first n partitions, advances the tracker, then issues `ContinueAsNew` with the same input. Required when partition counts can exceed Temporal's history budget (rule of thumb: > ~5,000 partitions). Must be combined with `WithTracker` to be useful.

---

## Workflow & Activity Data Flow

### Activity: `runPartition[In, Out, K]`

1. Record heartbeat: `"partition <start> to <end>: starting"`
2. Start `heartbeat.Loop` goroutine (shared helper from `datasync/internal/heartbeat`)
3. `fetcher(ctx, partition.Start, partition.End)` → records
4. Set phase `"mapping"` → `mapper.Map(ctx, records)` → mapped
5. Set phase `"writing"` → `sink.Write(ctx, mapped)` → write result
6. Return `PartitionResult[K]`

### Workflow: `chunkedSyncWorkflow[In, Out, K]`

1. Call `partitioner.Partitions(ctx)` activity to get partition list (see "Partition list delivery" below)
2. If `tracker` is set: call `tracker.Cursor` activity to get resume point
   - Filter partitions: only process partitions where `partition.Start >= cursor` (using native `>=` from `cmp.Ordered`)
3. If `MaxPartitionsPerExecution` is set and remaining > N: take first N, defer the rest to a `ContinueAsNew` at step 5
4. For each partition in this execution:
   - Execute `runPartition` activity
   - Append result to `SyncResult[K]`
   - If `tracker` is set: call `tracker.Advance` activity with `partition.End`
   - If not last partition + `partitionSleep > 0`: `workflow.Sleep(ctx, partitionSleep)`
5. If a deferred remainder exists: `workflow.NewContinueAsNewError(ctx, workflowName, sameInput)` so the next execution picks up from the advanced cursor

### Partition list delivery

The initial draft stated unconditionally that `Partitions()` is an activity call. That creates a payload-size risk for very large lists.

**Resolution:** keep activity-based delivery (preserves flexibility for DB-backed partitioners), but constrain it via two mechanisms:

1. **Documented soft limit:** ~5,000 partitions per call. Users with larger ranges must use `MaxPartitionsPerExecution` so the workflow chunks itself across executions.
2. **Built-in `DatePartitioner` is bounded:** `LookBack / ChunkSize` cannot produce more than that comfortably for typical configurations (e.g., 1 year / 1 hour = 8,760 — already at the boundary; users hitting this should use `MaxPartitionsPerExecution`).

Inline computation in the workflow (the para-sync approach) is simpler but only works for closed-form partitioners. We accept the activity overhead to keep the interface uniform.

**Key decisions:**
- `Partitions()` is an activity call, not inline. Keeps the partitioner interface flexible and supports DB-backed partitioners.
- Cursor read/advance are separate small activities with short timeouts and aggressive retries.
- `ContinueAsNew` handles unbounded ranges, replacing para-sync's `PerFireWindow` concept with one that's automatic when a tracker is configured.

---

## Heartbeat Integration

**Plan:**
1. Move `heartbeatLoop` and `heartbeatInterval` from `datasync/activity/sync.go` to `datasync/internal/heartbeat/heartbeat.go`. Rename to `heartbeat.Loop` and `heartbeat.Interval` (still unexported outside the module's internal tree).
2. `datasync/activity/sync.go` switches to using the `heartbeat` package.
3. `datasync/chunk/run_partition.go` uses the same package.
4. Add phase tracking to `runPartition` so heartbeats report `"partition <start> to <end>: fetching"`.

**Phase strings:** `starting`, `fetching`, `mapping`, `writing`

**Key decisions:**
- Reuse go-wf's existing `heartbeatInterval` logic (dynamic, based on `HeartbeatTimeout/3`) rather than para-sync's hardcoded 10s.
- `jobName` in the heartbeat payload comes from the builder name.
- Using `internal/` (not exported) keeps the helpers free to evolve without breaking external consumers.

---

## Activity Defaults & Limits

Defaults adopted from para-sync, justified there by production observations:

```go
const (
    defaultStartToCloseTimeout = 20 * time.Minute
    defaultHeartbeatTimeout    = 15 * time.Minute
)

var defaultActivityRetryPolicy = temporal.RetryPolicy{
    MaximumAttempts:    3,
    InitialInterval:    60 * time.Second,
    BackoffCoefficient: 2.0,
    MaximumInterval:    5 * time.Minute,
}

var defaultCursorActivityOptions = workflow.ActivityOptions{
    StartToCloseTimeout: 10 * time.Second,
    RetryPolicy: &temporal.RetryPolicy{
        MaximumAttempts:    3,
        InitialInterval:    2 * time.Second,
        BackoffCoefficient: 2.0,
        MaximumInterval:    8 * time.Second,
    },
}
```

`ActivityTimeouts(startToClose, heartbeat)` lets callers override the partition activity's timeouts. The cursor read/advance options are not user-configurable — they target small local DB calls.

**Schedule overlap policy:** `Build()` does not set an overlap policy. Callers wire the schedule's `OverlapPolicy` externally (matching para-sync's pattern and the existing `datasyncwf.FullJobRegistration` contract).

---

## Error Handling & Rate Limiting

### Rate Limiting

Port `WithRateLimitRetry` and `RateLimitOpts` from para-sync:

```go
type RateLimitDetector func(err error) bool

type RateLimitOpts struct {
    Detector    RateLimitDetector
    MaxAttempts int           // default 3
    Sleep       time.Duration // default 60s
    Sleeper     func(ctx context.Context, d time.Duration) // default HeartbeatSleeper
}

func WithRateLimitRetry[In any, K cmp.Ordered](
    inner PartitionFetcher[In, K],
    opts RateLimitOpts,
) PartitionFetcher[In, K]
```

### Error Handling in Workflow

- Per-partition activity failures retried via `temporal.RetryPolicy` (configurable, default 3 attempts with 60s/120s/240s backoff, max 5min)
- If all retries exhaust: workflow fails. Temporal's workflow retry policy applies at workflow level if configured.
- If `tracker` is set: cursor advanced for successful partitions. Failed partition NOT advanced, so next execution retries from failed partition.

**Key decisions:**
- Rate-limit retry is a `PartitionFetcher` decorator, not baked into the activity.
- `HeartbeatSleeper` stays in the chunk package (specific to activity context).

---

## Testing Strategy

### Unit tests (no Temporal required)

- `partitioner_test.go` — `DatePartitioner` edge cases: timezone handling, chunk boundary alignment, DST transitions, last-partition clamping
- `key_test.go` — `TimeToKey` / `KeyToTime` round-trip, monotonicity preserved across DST
- `limiter_test.go` — `WithRateLimitRetry`: success path, rate-limit detection, max attempts, ctx cancellation, custom sleeper
- `iterate_test.go` — `IteratePartitions`: correct sequence, last chunk clamping, sleep skipping, error propagation

### Temporal activity tests (`testsuite.TestActivityEnvironment`)

- `run_partition_test.go` — `runPartition` with slow fetch/write, verifies heartbeats captured via `heartbeatCaptureOutbound` interceptor (existing pattern in `datasync/activity/sync_test.go`)
- Mock `PartitionFetcher`, `Mapper`, `Sink`, `ProgressTracker`

### Temporal workflow tests (`testsuite.TestWorkflowEnvironment`)

- `sync_test.go` — `ChunkedSync` builder. Cursor scenarios that MUST be covered:
  - **No tracker:** all partitions processed
  - **Tracker present, cursor missing (exists=false):** all partitions processed, cursor advanced after each
  - **Tracker present, cursor mid-range:** partitions before cursor skipped, remainder processed
  - **Tracker present, cursor at/past last partition end:** zero partitions processed, no error
  - **Failure mid-list:** cursor reflects last successful partition; workflow returns error
  - **`MaxPartitionsPerExecution` triggers ContinueAsNew:** verify second execution starts from advanced cursor
- `date_sync_test.go` — `DateChunkedSync`: date formatting, lookback window, timezone alignment, time→int64→time round-trip preserves wall-clock semantics

**Key decisions:**
- Reuse go-wf's existing test patterns: `TestActivityEnvironment`, `TestWorkflowEnvironment`, `heartbeatCaptureOutbound` interceptor.
- No integration tests in the chunk package — those live in para-sync with real Postgres tracker + real APIs.

---

## What Stays in para-sync vs Moves to go-wf

| Stays in para-sync | Moves to go-wf `datasync/chunk/` |
|---|---|
| `PostgresProgressTracker` (gorm dependency) | `ProgressTracker[K]` interface |
| `timeutil.JakartaLoc` timezone constant | `DatePartitioner` (generic, configurable timezone) |
| WIB-specific date formatting tricks | `Partition[K]`, `Partitioner[K]` interface |
| Vendor-specific `ChunkFetcher` closures | `PartitionFetcher[T, K]` type |
| App-specific job registration wiring | `ChunkedSync`, `DateChunkedSync` builders |
| | `PartitionResult[K]`, `SyncResult[K]` |
| | `RateLimitOpts`, `WithRateLimitRetry` |
| | `HeartbeatSleeper`, `IteratePartitions` |
| | `runPartition` activity + workflow functions |
| | `heartbeat.Loop` / `heartbeat.Interval` (in `datasync/internal/heartbeat`) |

**Key decisions:**
- go-wf provides the framework. para-sync provides the implementations.
- para-sync becomes a consumer of `datasync/chunk` just like it currently consumes `datasync` and `datasync/workflow`.
- Para-sync is not modified as part of this work.

---

## Naming

- Package: `datasync/chunk`
- Builders: `ChunkedSync[In, Out, K]`, `DateChunkedSync[In, Out]`
- Conceptual descriptors in prose ("partition", "partitioner", "PartitionResult") preserved — they describe the abstraction. Chunks are the implementation choice.

---

## Future Work (out of scope)

- Opaque-token pagination pattern (separate from ordered-key partitioning)
- Additional built-in partitioners (offset-based, ID-range)
- Streaming `Partitioner` variant for partition counts that exceed Temporal payload size
- Metrics and OTel integration in `runPartition` (follow `datasync/activity/sync.go` pattern)
- Para-sync migration to consume `datasync/chunk` (after go-wf implementation is stable)

---

## Changelog

**2026-05-10 (revision 2)** — addresses deep-review blockers:
- Switched from `constraints.Ordered` (x/exp) to `cmp.Ordered` (stdlib).
- Resolved `time.Time` ordering by making `DateChunkedSync` use `int64` Unix nanos internally.
- Moved heartbeat helpers to `datasync/internal/heartbeat` to share between `activity/` and `chunk/` without exporting.
- Typed `PartitionResult[K]` and `SyncResult[K]` end-to-end (no string serialization in result).
- Added `MaxPartitionsPerExecution` + `ContinueAsNew` to bound workflow history.
- Added `ActivityTimeouts` option and documented defaults adopted from para-sync.
- Renamed `ChunkSleep` → `PartitionSleep` for naming consistency.
- Documented JSON-encodability requirement on `K`.
- Documented schedule-overlap-policy delegation to caller.
- Expanded testing strategy with explicit cursor edge cases.
- Resolved naming: builder is `ChunkedSync` / `DateChunkedSync` in `datasync/chunk` package.
