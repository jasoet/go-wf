# Datasync Chunk Package Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `datasync/chunk/` package providing generic partitioned-sync workflows (`ChunkedSync` and `DateChunkedSync` builders) on top of Temporal, reusing existing `datasync` primitives (`Mapper`, `Sink`, `FullJobRegistration`) and heartbeat machinery.

**Architecture:** New package `datasync/chunk` with builder API and Temporal workflow + activities. Heartbeat helpers extracted from `datasync/activity/sync.go` into `datasync/internal/heartbeat` so both `datasync/activity` and `datasync/chunk` can share them. Generic key parameter `K cmp.Ordered`; `DateChunkedSync` wraps `ChunkedSync[..., int64]` with `time.Time` ↔ Unix-nano conversion at the boundary so user-facing API stays time-based without violating the constraint.

**Tech Stack:** Go 1.26 (uses `cmp.Ordered` from stdlib), Temporal Go SDK v1.40.0, `sync/atomic`, existing `testify` + `testsuite` patterns.

**Spec:** `docs/superpowers/specs/2026-05-10-datasync-chunk-design.md`

**Branch:** create `feat/datasync-chunk` before starting (current workspace is on `main`).

---

## File Structure

| File | Type | Responsibility |
|---|---|---|
| `datasync/internal/heartbeat/heartbeat.go` | Create | Shared `Loop`, `Interval`, `PhaseMessage` helpers |
| `datasync/internal/heartbeat/heartbeat_test.go` | Create | Tests for `Interval` table + `Loop` smoke test |
| `datasync/activity/sync.go` | Modify | Replace local `heartbeatLoop` / `heartbeatInterval` with `heartbeat.Loop` / `heartbeat.Interval` |
| `datasync/activity/sync_test.go` | Modify | Remove `TestHeartbeatInterval` (moves to internal pkg); keep slow-fetch / slow-write integration tests |
| `datasync/chunk/doc.go` | Create | Package godoc + small example |
| `datasync/chunk/key.go` | Create | `TimeToKey`, `KeyToTime` conversion |
| `datasync/chunk/key_test.go` | Create | Round-trip + DST tests |
| `datasync/chunk/result.go` | Create | `Partition[K]`, `PartitionResult[K]`, `SyncResult[K]` |
| `datasync/chunk/tracker.go` | Create | `ProgressTracker[K]` interface |
| `datasync/chunk/fetcher.go` | Create | `PartitionFetcher[T, K]` type alias |
| `datasync/chunk/partitioner.go` | Create | `Partitioner[K]` interface + `DatePartitioner` implementation |
| `datasync/chunk/partitioner_test.go` | Create | `DatePartitioner` boundary, timezone, DST tests |
| `datasync/chunk/sleeper.go` | Create | `HeartbeatSleeper` |
| `datasync/chunk/sleeper_test.go` | Create | `HeartbeatSleeper` cancellation test |
| `datasync/chunk/limiter.go` | Create | `RateLimitOpts` + `WithRateLimitRetry` decorator |
| `datasync/chunk/limiter_test.go` | Create | Rate-limit retry table tests |
| `datasync/chunk/iterate.go` | Create | `IteratePartitions` non-Temporal helper |
| `datasync/chunk/iterate_test.go` | Create | Sequence, clamping, sleep-skip, ctx-cancel |
| `datasync/chunk/run_partition.go` | Create | `runPartition` activity (generic) |
| `datasync/chunk/run_partition_test.go` | Create | Activity test with `TestActivityEnvironment` + heartbeat capture |
| `datasync/chunk/sync.go` | Create | `ChunkedSync` builder + workflow |
| `datasync/chunk/sync_test.go` | Create | Workflow tests using `TestWorkflowEnvironment` |
| `datasync/chunk/date_sync.go` | Create | `DateChunkedSync` wrapper |
| `datasync/chunk/date_sync_test.go` | Create | DateChunkedSync wrapper tests |

No modifications to `datasync/job.go`, `datasync/workflow/sync.go`, `datasync/builder/builder.go`, `datasync/runner.go`. Para-sync is **not** modified.

---

## Critical Background — Read Before Implementing

### 1. `cmp.Ordered` does not include `time.Time`

`cmp.Ordered` (Go 1.21+ stdlib) admits primitives only — `~int | ~float* | ~string` and friends. `time.Time` is a struct and **does not satisfy `cmp.Ordered`**. The generic builder `ChunkedSync[In, Out, K cmp.Ordered]` therefore cannot be parameterized with `K = time.Time`.

`DateChunkedSync` resolves this by parameterizing the inner builder as `ChunkedSync[In, Out, int64]` (Unix nanoseconds) and converting at the API boundary. User-supplied `PartitionFetcher[T, time.Time]` is wrapped into a `PartitionFetcher[T, int64]` that calls `KeyToTime` before delegating. Same for `ProgressTracker[time.Time]`.

This is the single most load-bearing decision in the package — keep the conversion contained to `date_sync.go`.

### 2. Generic activity registration with Temporal

Temporal activity registration (`worker.RegisterActivityWithOptions(fn, opts)`) accepts any function whose signature is `func(ctx, In) (Out, error)`. Generic functions cannot be registered directly, but **closures that bind the type parameters can**.

The pattern (mirroring `datasync/activity` and para-sync's `runChunk`):

```go
func (c *ChunkedSync[In, Out, K]) Build() datasyncwf.FullJobRegistration {
    fetcher := c.fetcher  // captured with concrete In, K
    mapper := c.mapper
    sink := c.sink

    runPartitionFn := func(ctx context.Context, in runPartitionInput[K]) (PartitionResult[K], error) {
        return runPartition[In, Out, K](ctx, in, fetcher, mapper, sink)
    }
    // worker.RegisterActivityWithOptions(runPartitionFn, ...)
}
```

The closure has a non-generic concrete signature; Temporal registers it normally. JSON serialization handles `Partition[K]` and `PartitionResult[K]` provided `K` is JSON-encodable (all `cmp.Ordered` primitives are).

### 3. Temporal heartbeat throttling

The activity SDK throttles heartbeats sent to the server at `0.8 × HeartbeatTimeout`:
- First `RecordHeartbeat` after a quiet period is sent immediately.
- Subsequent calls within the throttle window are batched; only the latest payload is kept.
- If the activity completes before the throttle timer fires, batched payloads are DROPPED. Only the first heartbeat reaches `SetOnActivityHeartbeatListener`.

**Implication for tests:** assert *at least one* heartbeat captured with the expected phase, not "more than N". The pattern in `datasync/activity/sync_test.go` uses a custom interceptor (`heartbeatCaptureOutbound`) that captures all calls (including throttled ones) — adopt the same approach for chunk package tests.

### 4. `assert` vs `require` inside the heartbeat listener

`SetOnActivityHeartbeatListener`'s callback runs from the heartbeat-emitting goroutine, **not** the test goroutine. `require.NoError` calls `t.FailNow` → `runtime.Goexit`, which terminates the wrong goroutine. **Always use `assert` inside listener callbacks.** This is documented in `datasync/activity/sync_test.go:184`.

### 5. Workflow determinism

Workflow code must be deterministic across replays. The `Partitions()` activity result is recorded in workflow history, so partition lists are stable across replays. Cursor read/advance are also activities → recorded. The only non-activity logic in the workflow is the `for` loop comparing `partition.Start >= cursor`, which uses `cmp.Ordered` operators — purely deterministic.

`workflow.Sleep` (used for partition-sleep gaps) is replay-safe. `workflow.NewContinueAsNewError` ends the workflow execution and starts a fresh one with the same workflow type; it is the standard mechanism for unbounded loops.

### 6. Project conventions

- Run all commands through `task <name>`. For tests during development: `task test:pkg -- ./datasync/chunk/...` and `task test:run -- -run TestName ./datasync/chunk/...`.
- Conventional Commits: `<type>(<scope>): <description>`, e.g. `feat(chunk): add Partitioner interface`.
- One branch per change, squash merge.
- No AI co-author lines in commits (per `CLAUDE.md`).

---

## Task 0: Setup Branch and Baseline

- [ ] **Step 1: Create feature branch**

```bash
git checkout -b feat/datasync-chunk
```

- [ ] **Step 2: Verify baseline tests pass before any changes**

```bash
task ci:test
```

Expected: all tests pass. If any fail on `main` before changes, stop and investigate — not a chunk-package issue.

- [ ] **Step 3: Commit the spec (currently untracked)**

```bash
git add docs/superpowers/specs/2026-05-10-datasync-chunk-design.md
git commit -m "docs(chunk): add datasync/chunk design spec"
```

---

## Task 1: Extract Heartbeat Helpers to `datasync/internal/heartbeat`

**Files:**
- Create: `datasync/internal/heartbeat/heartbeat.go`
- Create: `datasync/internal/heartbeat/heartbeat_test.go`

This is the prerequisite move so `datasync/chunk` can reuse the same mechanics.

- [ ] **Step 1: Create the new package with `Interval`**

Create `datasync/internal/heartbeat/heartbeat.go`:

```go
// Package heartbeat provides shared Temporal-activity heartbeat helpers used by
// datasync/activity and datasync/chunk. Living under internal/ keeps these
// helpers off the public API while still allowing reuse across sibling
// packages within datasync.
package heartbeat

import (
	"context"
	"sync/atomic"
	"time"

	"go.temporal.io/sdk/activity"
)

// Interval derives the periodic heartbeat tick from the configured
// HeartbeatTimeout. Returns max(1s, hbTimeout/3); falls back to 10s when
// hbTimeout is 0 (no timeout configured).
func Interval(hbTimeout time.Duration) time.Duration {
	if hbTimeout == 0 {
		return 10 * time.Second
	}
	interval := hbTimeout / 3
	if interval < 1*time.Second {
		return 1 * time.Second
	}
	return interval
}

// Loop ticks at the given interval until done is closed or ctx is canceled,
// calling activity.RecordHeartbeat with the message returned by message().
// message is invoked once per tick, so callers may compute payloads lazily
// (e.g., reading an atomic.Pointer for the current phase).
func Loop(
	ctx context.Context,
	interval time.Duration,
	message func() string,
	done <-chan struct{},
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			activity.RecordHeartbeat(ctx, message())
		}
	}
}

// PhaseMessage returns a message function suitable for Loop that formats a
// heartbeat as "<prefix>: <phase>" where phase is read atomically. If the
// phase pointer has never been set, "starting" is used.
func PhaseMessage(prefix string, phase *atomic.Pointer[string]) func() string {
	return func() string {
		p := "starting"
		if ptr := phase.Load(); ptr != nil {
			p = *ptr
		}
		return prefix + ": " + p
	}
}
```

- [ ] **Step 2: Add the table test for `Interval`**

Create `datasync/internal/heartbeat/heartbeat_test.go`:

```go
package heartbeat

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

func TestInterval(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"zero falls back to 10s default", 0, 10 * time.Second},
		{"100ms hits 1s floor", 100 * time.Millisecond, 1 * time.Second},
		{"1s hits 1s floor", 1 * time.Second, 1 * time.Second},
		{"3s yields 1s (floor exact)", 3 * time.Second, 1 * time.Second},
		{"4s yields ~1333ms (non-round, above floor)", 4 * time.Second, 4 * time.Second / 3},
		{"6s yields 2s", 6 * time.Second, 2 * time.Second},
		{"30s yields 10s (default-config case)", 30 * time.Second, 10 * time.Second},
		{"1m yields 20s", 1 * time.Minute, 20 * time.Second},
		{"5m yields 100s", 5 * time.Minute, 100 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Interval(tc.timeout)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestPhaseMessage_DefaultsToStarting(t *testing.T) {
	var phase atomic.Pointer[string]
	msg := PhaseMessage("syncing job-x", &phase)
	assert.Equal(t, "syncing job-x: starting", msg())
}

func TestPhaseMessage_ReadsCurrentPhase(t *testing.T) {
	var phase atomic.Pointer[string]
	msg := PhaseMessage("syncing job-x", &phase)
	p := "writing"
	phase.Store(&p)
	assert.Equal(t, "syncing job-x: writing", msg())
}

// heartbeatCaptureOutbound is an activity outbound interceptor that records
// every RecordHeartbeat call (bypasses Temporal's throttling/dedup that the
// SetOnActivityHeartbeatListener test hook applies).
type heartbeatCaptureOutbound struct {
	sdkinterceptor.ActivityOutboundInterceptorBase
	mu       sync.Mutex
	captured []string
}

func (h *heartbeatCaptureOutbound) RecordHeartbeat(ctx context.Context, details ...interface{}) {
	if len(details) > 0 {
		if s, ok := details[0].(string); ok {
			h.mu.Lock()
			h.captured = append(h.captured, s)
			h.mu.Unlock()
		}
	}
	h.ActivityOutboundInterceptorBase.RecordHeartbeat(ctx, details...)
}

func (h *heartbeatCaptureOutbound) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.captured))
	copy(out, h.captured)
	return out
}

type capturingInbound struct {
	sdkinterceptor.ActivityInboundInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInbound) Init(o sdkinterceptor.ActivityOutboundInterceptor) error {
	c.outbound.ActivityOutboundInterceptorBase.Next = o
	return c.ActivityInboundInterceptorBase.Init(c.outbound)
}

type capturingInterceptor struct {
	sdkinterceptor.WorkerInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInterceptor) InterceptActivity(_ context.Context, next sdkinterceptor.ActivityInboundInterceptor) sdkinterceptor.ActivityInboundInterceptor {
	in := &capturingInbound{outbound: c.outbound}
	in.ActivityInboundInterceptorBase.Next = next
	return in
}

func TestLoop_TicksAndStopsOnDone(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	// Sanity: SetOnActivityHeartbeatListener captures at least the first heartbeat,
	// but the interceptor below captures all calls.
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	cap := &heartbeatCaptureOutbound{}
	testEnv.SetWorkerOptions(worker.Options{
		Interceptors: []sdkinterceptor.WorkerInterceptor{&capturingInterceptor{outbound: cap}},
	})

	act := func(ctx context.Context) error {
		var phase atomic.Pointer[string]
		p := "running"
		phase.Store(&p)
		done := make(chan struct{})
		go Loop(ctx, 50*time.Millisecond, PhaseMessage("syncing test", &phase), done)
		time.Sleep(180 * time.Millisecond)
		close(done)
		return nil
	}
	testEnv.RegisterActivity(act)

	_, err := testEnv.ExecuteActivity(act)
	require.NoError(t, err)

	got := cap.snapshot()
	require.GreaterOrEqual(t, len(got), 1, "expected at least one heartbeat, got %v", got)
	for _, m := range got {
		assert.Equal(t, "syncing test: running", m)
	}
}
```

- [ ] **Step 3: Run new tests**

```bash
task test:pkg -- ./datasync/internal/heartbeat/...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add datasync/internal/heartbeat/
git commit -m "feat(datasync): add internal/heartbeat shared helpers"
```

---

## Task 2: Migrate `datasync/activity` to use `internal/heartbeat`

**Files:**
- Modify: `datasync/activity/sync.go` — drop local helpers, import shared package
- Modify: `datasync/activity/sync_test.go` — drop `TestHeartbeatInterval` (now in heartbeat pkg)

- [ ] **Step 1: Replace local helpers in `datasync/activity/sync.go`**

In `datasync/activity/sync.go`:

1. Add import: `"github.com/jasoet/go-wf/datasync/internal/heartbeat"`.
2. Replace the `heartbeatLoop` goroutine setup and helper definition. The current code at lines 60–63 reads:

```go
interval := heartbeatInterval(activity.GetInfo(ctx).HeartbeatTimeout)
done := make(chan struct{})
defer close(done)
go heartbeatLoop(ctx, interval, input.JobName, &phase, done)
```

Change to:

```go
interval := heartbeat.Interval(activity.GetInfo(ctx).HeartbeatTimeout)
done := make(chan struct{})
defer close(done)
go heartbeat.Loop(ctx, interval, heartbeat.PhaseMessage("syncing "+input.JobName, &phase), done)
```

3. Delete the `heartbeatLoop` function (currently lines 191–218) and the `heartbeatInterval` function (currently lines 220–232) from `sync.go`.

- [ ] **Step 2: Remove `TestHeartbeatInterval` from `datasync/activity/sync_test.go`**

Delete the `TestHeartbeatInterval` function (currently lines 150–172). It was moved to the heartbeat package in Task 1.

The slow-fetch / slow-write integration tests (`TestActivities_SyncData_HeartbeatsDuringSlowFetch`, `TestActivities_SyncData_HeartbeatsDuringSlowWrite`) stay — they test the SyncData integration, not the helper directly.

- [ ] **Step 3: Run all datasync tests**

```bash
task test:pkg -- ./datasync/...
```

Expected: PASS. If a test fails referring to the unexported helpers, ensure all references in `sync.go` were updated.

- [ ] **Step 4: Lint**

```bash
task lint
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add datasync/activity/sync.go datasync/activity/sync_test.go
git commit -m "refactor(datasync): use internal/heartbeat from activity package"
```

---

## Task 3: Add `key.go` — Time/Int64 Conversion

**Files:**
- Create: `datasync/chunk/key.go`
- Create: `datasync/chunk/key_test.go`

- [ ] **Step 1: Write failing tests**

Create `datasync/chunk/key_test.go`:

```go
package chunk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKey_RoundTrip_UTC(t *testing.T) {
	in := time.Date(2026, 5, 10, 23, 30, 0, 0, time.UTC)
	k := TimeToKey(in)
	out := KeyToTime(k)
	assert.True(t, in.Equal(out), "round-trip preserves instant: in=%s out=%s", in, out)
	assert.Equal(t, time.UTC, out.Location(), "KeyToTime returns UTC")
}

func TestKey_RoundTrip_NonUTC(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skipf("Asia/Jakarta tz unavailable: %v", err)
	}
	in := time.Date(2026, 5, 10, 23, 30, 0, 0, loc)
	k := TimeToKey(in)
	out := KeyToTime(k)
	assert.True(t, in.Equal(out), "round-trip preserves instant across zones: in=%s out=%s", in, out)
}

func TestKey_Monotonicity(t *testing.T) {
	a := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	b := a.Add(1 * time.Second)
	assert.Less(t, TimeToKey(a), TimeToKey(b), "monotonicity preserved across seconds")
}

func TestKey_DSTBoundary(t *testing.T) {
	// US/Eastern springs forward 2026-03-08 02:00 -> 03:00. Two distinct
	// instants either side must remain ordered when converted to keys.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York tz unavailable: %v", err)
	}
	before := time.Date(2026, 3, 8, 1, 30, 0, 0, loc)
	after := time.Date(2026, 3, 8, 3, 30, 0, 0, loc)
	assert.Less(t, TimeToKey(before), TimeToKey(after))
}
```

- [ ] **Step 2: Run test, expect fail**

```bash
task test:pkg -- ./datasync/chunk/...
```

Expected: build error (TimeToKey/KeyToTime undefined).

- [ ] **Step 3: Implement `key.go`**

Create `datasync/chunk/key.go`:

```go
package chunk

import "time"

// TimeToKey converts a time.Time to its int64 Unix-nanosecond representation.
// Used internally by DateChunkedSync to project time.Time partition keys onto
// the cmp.Ordered constraint required by ChunkedSync.
func TimeToKey(t time.Time) int64 {
	return t.UnixNano()
}

// KeyToTime converts a Unix-nanosecond key back to a UTC time.Time. Callers
// that need a different display zone should call .In(loc) on the result.
func KeyToTime(k int64) time.Time {
	return time.Unix(0, k).UTC()
}
```

- [ ] **Step 4: Run tests, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/key.go datasync/chunk/key_test.go
git commit -m "feat(chunk): add TimeToKey/KeyToTime conversions"
```

---

## Task 4: Add `result.go` — Partition and Result Types

**Files:**
- Create: `datasync/chunk/result.go`

These are pure data types with no behavior. No test file — types are validated through their use in later workflow tests.

- [ ] **Step 1: Implement `result.go`**

Create `datasync/chunk/result.go`:

```go
package chunk

import "cmp"

// Partition is a half-open key range [Start, End) processed as one unit.
type Partition[K cmp.Ordered] struct {
	Start K `json:"start"`
	End   K `json:"end"`
}

// PartitionResult is the per-partition outcome captured in the workflow result.
// Keys are preserved end-to-end so callers can program against them without
// re-parsing strings.
type PartitionResult[K cmp.Ordered] struct {
	Start    K   `json:"start"`
	End      K   `json:"end"`
	Fetched  int `json:"fetched"`
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

// SyncResult is the workflow-level summary aggregating all partitions.
type SyncResult[K cmp.Ordered] struct {
	JobName         string               `json:"jobName"`
	TotalPartitions int                  `json:"totalPartitions"`
	TotalFetched    int                  `json:"totalFetched"`
	TotalInserted   int                  `json:"totalInserted"`
	TotalUpdated    int                  `json:"totalUpdated"`
	TotalSkipped    int                  `json:"totalSkipped"`
	Partitions      []PartitionResult[K] `json:"partitions,omitempty"`
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./datasync/chunk/...
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add datasync/chunk/result.go
git commit -m "feat(chunk): add Partition and result types"
```

---

## Task 5: Add `tracker.go` — ProgressTracker Interface

**Files:**
- Create: `datasync/chunk/tracker.go`

- [ ] **Step 1: Implement `tracker.go`**

Create `datasync/chunk/tracker.go`:

```go
package chunk

import (
	"cmp"
	"context"
)

// ProgressTracker persists progress so a chunked sync can resume after failure
// rather than restart from the configured range start.
//
// Implementations are NOT provided by go-wf — define your own (e.g.,
// Postgres-backed) and pass it via ChunkedSync.WithTracker.
type ProgressTracker[K cmp.Ordered] interface {
	// Cursor returns the last-completed partition.End for the named job.
	// The bool reports whether a cursor has been recorded — the zero value
	// of K may be a meaningful key (e.g., int64(0)).
	Cursor(ctx context.Context, jobName string) (K, bool, error)

	// Advance records that all partitions ending at completed (inclusive)
	// have been successfully processed. Implementations should be
	// idempotent; the workflow may retry the call.
	Advance(ctx context.Context, jobName string, completed K) error
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./datasync/chunk/...
```

- [ ] **Step 3: Commit**

```bash
git add datasync/chunk/tracker.go
git commit -m "feat(chunk): add ProgressTracker interface"
```

---

## Task 6: Add `fetcher.go` — PartitionFetcher Type

**Files:**
- Create: `datasync/chunk/fetcher.go`

- [ ] **Step 1: Implement `fetcher.go`**

Create `datasync/chunk/fetcher.go`:

```go
package chunk

import (
	"cmp"
	"context"
)

// PartitionFetcher fetches records of type T for a single partition [start, end).
// Implementations should respect ctx cancellation. The returned slice may be
// empty (no records in range) but should not be nil unless an error is also
// returned.
type PartitionFetcher[T any, K cmp.Ordered] func(ctx context.Context, start, end K) ([]T, error)
```

- [ ] **Step 2: Verify build, commit**

```bash
go build ./datasync/chunk/...
git add datasync/chunk/fetcher.go
git commit -m "feat(chunk): add PartitionFetcher type"
```

---

## Task 7: Add `partitioner.go` — Partitioner Interface and DatePartitioner

**Files:**
- Create: `datasync/chunk/partitioner.go`
- Create: `datasync/chunk/partitioner_test.go`

- [ ] **Step 1: Write failing tests**

Create `datasync/chunk/partitioner_test.go`:

```go
package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatePartitioner_BasicWindow(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 10, 12, 0, 0, 0, loc)
	p := &DatePartitioner{
		Now:       func() time.Time { return now },
		Loc:       loc,
		LookBack:  72 * time.Hour,
		ChunkSize: 24 * time.Hour,
	}

	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	require.Len(t, parts, 3)

	assert.Equal(t, TimeToKey(time.Date(2026, 5, 7, 0, 0, 0, 0, loc)), parts[0].Start)
	assert.Equal(t, TimeToKey(time.Date(2026, 5, 8, 0, 0, 0, 0, loc)), parts[0].End)
	assert.Equal(t, TimeToKey(time.Date(2026, 5, 9, 0, 0, 0, 0, loc)), parts[1].End)
	assert.Equal(t, TimeToKey(time.Date(2026, 5, 10, 0, 0, 0, 0, loc)), parts[2].End)
}

func TestDatePartitioner_LastChunkClampedToNow(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 10, 6, 0, 0, 0, loc)
	p := &DatePartitioner{
		Now:       func() time.Time { return now },
		Loc:       loc,
		LookBack:  48 * time.Hour,
		ChunkSize: 24 * time.Hour,
	}
	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	require.Len(t, parts, 2)
	// Last partition end clamped to now (06:00), not the next midnight.
	assert.Equal(t, TimeToKey(now), parts[len(parts)-1].End)
}

func TestDatePartitioner_AlignsToCalendarMidnightInTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skipf("tz unavailable: %v", err)
	}
	// Now is mid-day in WIB; alignment should push start to a WIB midnight.
	now := time.Date(2026, 5, 10, 14, 30, 0, 0, loc)
	p := &DatePartitioner{
		Now:       func() time.Time { return now },
		Loc:       loc,
		LookBack:  24 * time.Hour,
		ChunkSize: 24 * time.Hour,
	}
	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	require.Len(t, parts, 1)
	startTime := KeyToTime(parts[0].Start).In(loc)
	assert.Equal(t, 0, startTime.Hour())
	assert.Equal(t, 0, startTime.Minute())
}

func TestDatePartitioner_Validation(t *testing.T) {
	t.Run("LookBack required", func(t *testing.T) {
		p := &DatePartitioner{Now: time.Now, Loc: time.UTC, ChunkSize: time.Hour}
		_, err := p.Partitions(context.Background())
		assert.Error(t, err)
	})
	t.Run("ChunkSize required", func(t *testing.T) {
		p := &DatePartitioner{Now: time.Now, Loc: time.UTC, LookBack: time.Hour}
		_, err := p.Partitions(context.Background())
		assert.Error(t, err)
	})
}

func TestDatePartitioner_ZeroNowDefaultsToTimeNow(t *testing.T) {
	// When Now is nil, partitioner should fall back to time.Now and produce
	// a non-empty partition list for a non-zero LookBack.
	p := &DatePartitioner{
		Loc:       time.UTC,
		LookBack:  time.Hour,
		ChunkSize: time.Hour,
	}
	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, parts)
}
```

- [ ] **Step 2: Run, expect fail**

```bash
task test:pkg -- ./datasync/chunk/...
```

Expected: build errors (`Partitioner`, `DatePartitioner` undefined).

- [ ] **Step 3: Implement `partitioner.go`**

Create `datasync/chunk/partitioner.go`:

```go
package chunk

import (
	"cmp"
	"context"
	"errors"
	"time"
)

// Partitioner generates a sequence of partition keys for a sync run.
//
// Implementations must be deterministic for a given moment in time — the
// result is recorded in workflow history and used as the basis for activity
// scheduling. A non-deterministic partitioner would still work (its output is
// captured in history), but consumers should prefer deterministic ones for
// debuggability.
type Partitioner[K cmp.Ordered] interface {
	Partitions(ctx context.Context) ([]Partition[K], error)
}

// DatePartitioner is a time-range partitioner that emits int64 Unix-nano keys
// covering [now-LookBack, now) in ChunkSize steps. Start of the range is
// aligned to the most recent midnight in Loc so a 24h ChunkSize yields a
// single calendar day per partition.
//
// Used internally by DateChunkedSync; library consumers normally don't
// instantiate DatePartitioner directly.
type DatePartitioner struct {
	// Now returns the reference instant. If nil, time.Now is used.
	Now func() time.Time
	// Loc is the timezone used for midnight alignment. Defaults to UTC if nil.
	Loc *time.Location
	// LookBack is the total window size. Must be > 0.
	LookBack time.Duration
	// ChunkSize is the size of each partition. Must be > 0.
	ChunkSize time.Duration
}

// Partitions implements Partitioner[int64].
func (d *DatePartitioner) Partitions(_ context.Context) ([]Partition[int64], error) {
	if d.LookBack <= 0 {
		return nil, errors.New("DatePartitioner: LookBack must be > 0")
	}
	if d.ChunkSize <= 0 {
		return nil, errors.New("DatePartitioner: ChunkSize must be > 0")
	}
	loc := d.Loc
	if loc == nil {
		loc = time.UTC
	}
	nowFn := d.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	start := alignToMidnight(now.Add(-d.LookBack), loc)

	var out []Partition[int64]
	for cur := start; cur.Before(now); cur = cur.Add(d.ChunkSize) {
		end := cur.Add(d.ChunkSize)
		if end.After(now) {
			end = now
		}
		out = append(out, Partition[int64]{
			Start: TimeToKey(cur),
			End:   TimeToKey(end),
		})
	}
	return out, nil
}

func alignToMidnight(t time.Time, loc *time.Location) time.Time {
	w := t.In(loc)
	return time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, loc)
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

Expected: all partitioner tests PASS.

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/partitioner.go datasync/chunk/partitioner_test.go
git commit -m "feat(chunk): add Partitioner interface and DatePartitioner"
```

---

## Task 8: Add `sleeper.go` — HeartbeatSleeper

**Files:**
- Create: `datasync/chunk/sleeper.go`
- Create: `datasync/chunk/sleeper_test.go`

- [ ] **Step 1: Write failing test**

Create `datasync/chunk/sleeper_test.go`:

```go
package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

func TestHeartbeatSleeper_RespectsContextCancel(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	act := func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		HeartbeatSleeper(ctx, 10*time.Second)
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 5*time.Second, "should return promptly after cancel")
		return nil
	}
	testEnv.RegisterActivity(act)
	_, err := testEnv.ExecuteActivity(act)
	assert.NoError(t, err)
}

func TestHeartbeatSleeper_Returns_OnDeadline(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	act := func(ctx context.Context) error {
		start := time.Now()
		HeartbeatSleeper(ctx, 80*time.Millisecond)
		assert.GreaterOrEqual(t, time.Since(start), 80*time.Millisecond)
		return nil
	}
	testEnv.RegisterActivity(act)
	_, err := testEnv.ExecuteActivity(act)
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run, expect fail**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 3: Implement `sleeper.go`**

Create `datasync/chunk/sleeper.go`:

```go
package chunk

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

// heartbeatSleeperInterval is the inner-tick frequency for HeartbeatSleeper.
// 10s leaves comfortable margin under the typical 1-minute HeartbeatTimeout.
const heartbeatSleeperInterval = 10 * time.Second

// HeartbeatSleeper sleeps d while emitting Temporal activity heartbeats every
// 10s so the activity does not exceed its HeartbeatTimeout. Returns early
// when ctx is canceled.
//
// Use as the default Sleeper in RateLimitOpts. Panics if called outside an
// activity context (where activity.RecordHeartbeat is invalid).
func HeartbeatSleeper(ctx context.Context, d time.Duration) {
	deadline := time.Now().Add(d)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		step := heartbeatSleeperInterval
		if remaining < step {
			step = remaining
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(step):
			activity.RecordHeartbeat(ctx, "chunk sleep")
		}
	}
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/sleeper.go datasync/chunk/sleeper_test.go
git commit -m "feat(chunk): add HeartbeatSleeper"
```

---

## Task 9: Add `limiter.go` — Rate Limit Retry Decorator

**Files:**
- Create: `datasync/chunk/limiter.go`
- Create: `datasync/chunk/limiter_test.go`

- [ ] **Step 1: Write failing test**

Create `datasync/chunk/limiter_test.go`:

```go
package chunk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errRateLimit = errors.New("rate limit hit")

func rateLimitDetector(err error) bool {
	return errors.Is(err, errRateLimit)
}

func TestRateLimitRetry_FirstCallSucceeds(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return []string{"ok"}, nil
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector: rateLimitDetector,
		Sleeper:  func(_ context.Context, _ time.Duration) {},
	})
	got, err := wrapped(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, got)
	assert.Equal(t, 1, calls)
}

func TestRateLimitRetry_RetriesOnDetectedError(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		if calls < 3 {
			return nil, errRateLimit
		}
		return []string{"ok"}, nil
	}
	sleeps := 0
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 5,
		Sleep:       1 * time.Millisecond,
		Sleeper: func(_ context.Context, _ time.Duration) {
			sleeps++
		},
	})
	got, err := wrapped(context.Background(), 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"ok"}, got)
	assert.Equal(t, 3, calls)
	assert.Equal(t, 2, sleeps, "sleep called between attempts but not after success")
}

func TestRateLimitRetry_GivesUpAfterMaxAttempts(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return nil, errRateLimit
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 3,
		Sleeper:     func(_ context.Context, _ time.Duration) {},
	})
	_, err := wrapped(context.Background(), 0, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errRateLimit))
	assert.Equal(t, 3, calls)
}

func TestRateLimitRetry_NonRateLimitErrorReturnsImmediately(t *testing.T) {
	other := errors.New("other failure")
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return nil, other
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 5,
		Sleeper:     func(_ context.Context, _ time.Duration) {},
	})
	_, err := wrapped(context.Background(), 0, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, other))
	assert.Equal(t, 1, calls)
}

func TestRateLimitRetry_ContextCancelStopsRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		return nil, errRateLimit
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    rateLimitDetector,
		MaxAttempts: 5,
		Sleeper: func(_ context.Context, _ time.Duration) {
			cancel()
		},
	})
	_, err := wrapped(ctx, 0, 10)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestRateLimitRetry_NilDetectorTreatsAllErrorsAsNonRateLimit(t *testing.T) {
	calls := 0
	inner := func(_ context.Context, _, _ int64) ([]string, error) {
		calls++
		return nil, errRateLimit
	}
	wrapped := WithRateLimitRetry[string, int64](inner, RateLimitOpts{
		Detector:    nil,
		MaxAttempts: 5,
		Sleeper:     func(_ context.Context, _ time.Duration) {},
	})
	_, err := wrapped(context.Background(), 0, 10)
	require.Error(t, err)
	assert.Equal(t, 1, calls)
}
```

- [ ] **Step 2: Run, expect fail**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 3: Implement `limiter.go`**

Create `datasync/chunk/limiter.go`:

```go
package chunk

import (
	"cmp"
	"context"
	"time"
)

// RateLimitDetector returns true if err indicates a rate-limit response from
// the upstream API. Different APIs use different signaling — implementers
// supply this function for their specific API.
type RateLimitDetector func(err error) bool

// RateLimitOpts configures the WithRateLimitRetry decorator.
type RateLimitOpts struct {
	// Detector identifies rate-limit errors. If nil, the decorator treats
	// every error as non-rate-limit (no retries) — useful for callers that
	// want a no-op default and provide a real detector at runtime.
	Detector RateLimitDetector

	// MaxAttempts is the total number of calls (sleeps = MaxAttempts - 1).
	// If zero, defaults to 3.
	MaxAttempts int

	// Sleep is the duration between attempts. If zero, defaults to 60s.
	Sleep time.Duration

	// Sleeper performs the inter-attempt sleep. If nil, defaults to
	// HeartbeatSleeper which emits Temporal heartbeats during the sleep.
	// Tests typically pass a recording fake.
	Sleeper func(ctx context.Context, d time.Duration)
}

const (
	defaultMaxAttempts = 3
	defaultSleep       = 60 * time.Second
)

// WithRateLimitRetry decorates a PartitionFetcher with retry on detected
// rate-limit errors. The decorator calls inner up to opts.MaxAttempts times,
// sleeping opts.Sleep between attempts, only when the previous attempt
// returned an error matched by opts.Detector. Other errors return
// immediately without retry.
//
// Returns ctx.Err() if ctx is canceled during a sleep.
func WithRateLimitRetry[T any, K cmp.Ordered](
	inner PartitionFetcher[T, K],
	opts RateLimitOpts,
) PartitionFetcher[T, K] {
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultMaxAttempts
	}
	sleep := opts.Sleep
	if sleep <= 0 {
		sleep = defaultSleep
	}
	sleeper := opts.Sleeper
	if sleeper == nil {
		sleeper = HeartbeatSleeper
	}
	detector := opts.Detector

	return func(ctx context.Context, start, end K) ([]T, error) {
		var lastErr error
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			records, err := inner(ctx, start, end)
			if err == nil {
				return records, nil
			}
			lastErr = err
			isRateLimit := detector != nil && detector(err)
			if !isRateLimit {
				return nil, err
			}
			if attempt < maxAttempts {
				sleeper(ctx, sleep)
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
			}
		}
		return nil, lastErr
	}
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/limiter.go datasync/chunk/limiter_test.go
git commit -m "feat(chunk): add WithRateLimitRetry decorator"
```

---

## Task 10: Add `iterate.go` — IteratePartitions Helper

**Files:**
- Create: `datasync/chunk/iterate.go`
- Create: `datasync/chunk/iterate_test.go`

This is a non-Temporal helper for callers who want to walk partitions outside a workflow (utilities, scripts, tests).

- [ ] **Step 1: Write failing test**

Create `datasync/chunk/iterate_test.go`:

```go
package chunk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIteratePartitions_ProcessesAllInOrder(t *testing.T) {
	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
	}
	var seen []Partition[int64]
	err := IteratePartitions[int64](context.Background(), parts, 0, nil,
		func(p Partition[int64]) error {
			seen = append(seen, p)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, parts, seen)
}

func TestIteratePartitions_StopsOnError(t *testing.T) {
	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
	}
	boom := errors.New("boom")
	count := 0
	err := IteratePartitions[int64](context.Background(), parts, 0, nil,
		func(_ Partition[int64]) error {
			count++
			return boom
		})
	require.Error(t, err)
	assert.True(t, errors.Is(err, boom))
	assert.Equal(t, 1, count, "stopped after first error")
}

func TestIteratePartitions_SleepsBetweenButNotAfterLast(t *testing.T) {
	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
	}
	sleeps := 0
	err := IteratePartitions[int64](context.Background(), parts, 5*time.Millisecond,
		func(_ context.Context, _ time.Duration) { sleeps++ },
		func(_ Partition[int64]) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, 2, sleeps, "len(parts)-1 sleeps")
}

func TestIteratePartitions_ZeroSleepSkipsCallback(t *testing.T) {
	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	sleeps := 0
	err := IteratePartitions[int64](context.Background(), parts, 0,
		func(_ context.Context, _ time.Duration) { sleeps++ },
		func(_ Partition[int64]) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, 0, sleeps)
}

func TestIteratePartitions_ContextCancelDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	err := IteratePartitions[int64](ctx, parts, 100*time.Millisecond,
		func(_ context.Context, _ time.Duration) { cancel() },
		func(_ Partition[int64]) error { return nil })
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}
```

- [ ] **Step 2: Run, expect fail**

- [ ] **Step 3: Implement `iterate.go`**

Create `datasync/chunk/iterate.go`:

```go
package chunk

import (
	"cmp"
	"context"
	"time"
)

// IteratePartitions walks parts in order, calling fn for each. Between
// successful partitions, sleeper(ctx, sleep) is called. The sleep is skipped
// after the last partition, and skipped entirely when sleep == 0.
//
// Iteration stops promptly if ctx is canceled (returning ctx.Err()) or if fn
// returns an error.
//
// Intended for non-Temporal callers (utilities, scripts, tests). Inside a
// Temporal workflow, the ChunkedSync builder uses an inline equivalent that
// substitutes workflow.Sleep for the sleeper callback so the deterministic
// clock is used.
func IteratePartitions[K cmp.Ordered](
	ctx context.Context,
	parts []Partition[K],
	sleep time.Duration,
	sleeper func(ctx context.Context, d time.Duration),
	fn func(p Partition[K]) error,
) error {
	for i, p := range parts {
		if err := fn(p); err != nil {
			return err
		}
		if i < len(parts)-1 && sleep > 0 && sleeper != nil {
			sleeper(ctx, sleep)
			if ctx.Err() != nil {
				return ctx.Err()
			}
		}
	}
	return nil
}
```

- [ ] **Step 4: Run, expect pass**

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/iterate.go datasync/chunk/iterate_test.go
git commit -m "feat(chunk): add IteratePartitions helper"
```

---

## Task 11: Add `run_partition.go` — Per-Partition Activity

**Files:**
- Create: `datasync/chunk/run_partition.go`
- Create: `datasync/chunk/run_partition_test.go`

- [ ] **Step 1: Write failing test**

Create `datasync/chunk/run_partition_test.go`:

```go
package chunk

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/datasync"
)

// heartbeatCaptureOutbound mirrors the pattern in datasync/activity/sync_test.go.
type heartbeatCaptureOutbound struct {
	sdkinterceptor.ActivityOutboundInterceptorBase
	mu       sync.Mutex
	captured []string
}

func (h *heartbeatCaptureOutbound) RecordHeartbeat(ctx context.Context, details ...interface{}) {
	if len(details) > 0 {
		if s, ok := details[0].(string); ok {
			h.mu.Lock()
			h.captured = append(h.captured, s)
			h.mu.Unlock()
		}
	}
	h.ActivityOutboundInterceptorBase.RecordHeartbeat(ctx, details...)
}

func (h *heartbeatCaptureOutbound) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.captured))
	copy(out, h.captured)
	return out
}

type capturingInbound struct {
	sdkinterceptor.ActivityInboundInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInbound) Init(o sdkinterceptor.ActivityOutboundInterceptor) error {
	c.outbound.ActivityOutboundInterceptorBase.Next = o
	return c.ActivityInboundInterceptorBase.Init(c.outbound)
}

type capturingInterceptor struct {
	sdkinterceptor.WorkerInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInterceptor) InterceptActivity(_ context.Context, next sdkinterceptor.ActivityInboundInterceptor) sdkinterceptor.ActivityInboundInterceptor {
	in := &capturingInbound{outbound: c.outbound}
	in.ActivityInboundInterceptorBase.Next = next
	return in
}

type stubMapper struct{}

func (stubMapper) Map(_ context.Context, recs []string) ([]string, error) {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = strings.ToUpper(r)
	}
	return out, nil
}

type stubSink struct {
	name  string
	sleep time.Duration
	err   error
}

func (s *stubSink) Name() string { return s.name }
func (s *stubSink) Write(ctx context.Context, recs []string) (datasync.WriteResult, error) {
	if s.sleep > 0 {
		select {
		case <-ctx.Done():
			return datasync.WriteResult{}, ctx.Err()
		case <-time.After(s.sleep):
		}
	}
	if s.err != nil {
		return datasync.WriteResult{}, s.err
	}
	return datasync.WriteResult{Inserted: len(recs)}, nil
}

func TestRunPartition_HappyPath(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return []string{"a", "b"}, nil
	}
	mapper := stubMapper{}
	sink := &stubSink{name: "sink"}

	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, mapper, sink)
	}
	testEnv.RegisterActivity(act)

	val, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.NoError(t, err)
	var got PartitionResult[int64]
	require.NoError(t, val.Get(&got))
	assert.Equal(t, int64(0), got.Start)
	assert.Equal(t, int64(100), got.End)
	assert.Equal(t, 2, got.Fetched)
	assert.Equal(t, 2, got.Inserted)
}

func TestRunPartition_EmptyFetchSkipsMapAndWrite(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return nil, nil
	}
	mapperCalled := false
	mapperFn := datasync.MapperFunc[string, string](func(_ context.Context, recs []string) ([]string, error) {
		mapperCalled = true
		return recs, nil
	})
	sink := &stubSink{name: "sink"}

	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, mapperFn, sink)
	}
	testEnv.RegisterActivity(act)

	val, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.NoError(t, err)
	var got PartitionResult[int64]
	require.NoError(t, val.Get(&got))
	assert.Equal(t, 0, got.Fetched)
	assert.False(t, mapperCalled, "mapper should be skipped when fetcher returns no records")
}

func TestRunPartition_FetcherError_Wrapped(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	boom := errors.New("fetch boom")
	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return nil, boom
	}
	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, stubMapper{}, &stubSink{name: "s"})
	}
	testEnv.RegisterActivity(act)

	_, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.Error(t, err)
}

func TestRunPartition_HeartbeatsDuringSlowWrite(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	cap := &heartbeatCaptureOutbound{}
	testEnv.SetWorkerOptions(worker.Options{
		Interceptors: []sdkinterceptor.WorkerInterceptor{&capturingInterceptor{outbound: cap}},
	})

	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return []string{"a"}, nil
	}
	// sleep > Interval(0)==10s would be flaky; use a shorter sleep with a smaller heartbeat interval.
	// HeartbeatTimeout is unset in TestActivityEnvironment, so heartbeatInterval falls back to 10s; sleep > 10s guarantees at least one tick. Match existing pattern.
	sink := &stubSink{name: "sink", sleep: 11 * time.Second}

	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, stubMapper{}, sink)
	}
	testEnv.RegisterActivity(act)

	_, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.NoError(t, err)

	got := cap.snapshot()
	require.NotEmpty(t, got)
	foundWriting := false
	for _, m := range got {
		if strings.Contains(m, ": writing") {
			foundWriting = true
			break
		}
	}
	assert.True(t, foundWriting, "expected 'writing' phase in heartbeats; got %v", got)
}
```

- [ ] **Step 2: Run, expect fail**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 3: Implement `run_partition.go`**

Create `datasync/chunk/run_partition.go`:

```go
package chunk

import (
	"cmp"
	"context"
	"fmt"
	"sync/atomic"

	"go.temporal.io/sdk/activity"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/internal/heartbeat"
)

// runPartitionInput is the activity input for a single partition.
type runPartitionInput[K cmp.Ordered] struct {
	Partition Partition[K] `json:"partition"`
	JobName   string       `json:"jobName"`
}

// runPartition is the per-partition activity body. It records a starting
// heartbeat, spawns a heartbeat goroutine that ticks every Interval(...) with
// the current phase, then runs fetch -> map -> write.
//
// Activity registration: callers wrap this in a closure that binds the
// concrete In, Out, K parameters and registers it under "<jobName>.RunPartition".
func runPartition[In, Out any, K cmp.Ordered](
	ctx context.Context,
	in runPartitionInput[K],
	fetcher PartitionFetcher[In, K],
	mapper datasync.Mapper[In, Out],
	sink datasync.Sink[Out],
) (PartitionResult[K], error) {
	result := PartitionResult[K]{Start: in.Partition.Start, End: in.Partition.End}

	var phase atomic.Pointer[string]
	setPhase := func(p string) { phase.Store(&p) }
	setPhase("starting")

	prefix := fmt.Sprintf("partition %v to %v", in.Partition.Start, in.Partition.End)
	if in.JobName != "" {
		prefix = in.JobName + " " + prefix
	}
	activity.RecordHeartbeat(ctx, prefix+": starting")

	interval := heartbeat.Interval(activity.GetInfo(ctx).HeartbeatTimeout)
	done := make(chan struct{})
	defer close(done)
	go heartbeat.Loop(ctx, interval, heartbeat.PhaseMessage(prefix, &phase), done)

	setPhase("fetching")
	records, err := fetcher(ctx, in.Partition.Start, in.Partition.End)
	if err != nil {
		return result, fmt.Errorf("fetch %v..%v: %w", in.Partition.Start, in.Partition.End, err)
	}
	result.Fetched = len(records)
	if result.Fetched == 0 {
		return result, nil
	}

	setPhase("mapping")
	mapped, err := mapper.Map(ctx, records)
	if err != nil {
		return result, fmt.Errorf("map %v..%v: %w", in.Partition.Start, in.Partition.End, err)
	}

	setPhase("writing")
	wr, err := sink.Write(ctx, mapped)
	if err != nil {
		return result, fmt.Errorf("write %v..%v: %w", in.Partition.Start, in.Partition.End, err)
	}
	result.Inserted = wr.Inserted
	result.Updated = wr.Updated
	result.Skipped = wr.Skipped
	return result, nil
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

The slow-write test runs ~11s; this is acceptable but flagged.

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/run_partition.go datasync/chunk/run_partition_test.go
git commit -m "feat(chunk): add runPartition activity with phase-aware heartbeat"
```

---

## Task 12: Add `sync.go` — ChunkedSync Builder Skeleton

**Files:**
- Create: `datasync/chunk/sync.go` (skeleton — workflow logic added in Tasks 13–15)
- Create: `datasync/chunk/sync_test.go` (build-validation tests only at this stage)

This task only adds the builder struct, setters, and `Build()` validation. The workflow body is a stub returning an error so `Build()` panics on missing fields can be tested first.

- [ ] **Step 1: Write failing tests for `Build()` validation**

Create `datasync/chunk/sync_test.go`:

```go
package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jasoet/go-wf/datasync"
)

type stubPartitioner struct {
	parts []Partition[int64]
}

func (s *stubPartitioner) Partitions(_ context.Context) ([]Partition[int64], error) {
	return s.parts, nil
}

func TestChunkedSync_Build_RequiresPartitioner(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresFetcher(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresMapper(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Sink(&stubSink{name: "sink"}).
		Build()
}

func TestChunkedSync_Build_RequiresSink(t *testing.T) {
	defer func() { assert.NotNil(t, recover()) }()
	_ = NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Build()
}

func TestChunkedSync_Build_PopulatesRegistration(t *testing.T) {
	reg := NewChunkedSync[string, string, int64]("job-x").
		Partitioner(&stubPartitioner{}).
		Fetcher(func(_ context.Context, _, _ int64) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Schedule(15 * time.Minute).
		Disabled(true).
		Build()
	assert.Equal(t, "job-x", reg.Name)
	assert.Equal(t, "sync-job-x", reg.TaskQueue)
	assert.Equal(t, 15*time.Minute, reg.Schedule)
	assert.True(t, reg.Disabled)
}
```

- [ ] **Step 2: Run, expect fail**

- [ ] **Step 3: Implement `sync.go` skeleton**

Create `datasync/chunk/sync.go`:

```go
package chunk

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/payload"
	datasyncwf "github.com/jasoet/go-wf/datasync/workflow"
)

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

// ChunkedSync builds a Temporal workflow + activities that walk a sequence of
// partitions and run fetch -> map -> write per partition.
type ChunkedSync[In, Out any, K cmp.Ordered] struct {
	name           string
	partitioner    Partitioner[K]
	fetcher        PartitionFetcher[In, K]
	mapper         datasync.Mapper[In, Out]
	sink           datasync.Sink[Out]
	tracker        ProgressTracker[K]
	partitionSleep time.Duration
	schedule       time.Duration
	rateLimitOpts  *RateLimitOpts
	activityRetry  *temporal.RetryPolicy
	startToClose   time.Duration
	heartbeat      time.Duration
	maxPerExec     int
	disabled       bool
}

// NewChunkedSync starts a builder for a job named name. The name appears in
// Temporal as the workflow type, the activity prefix, and the schedule id.
func NewChunkedSync[In, Out any, K cmp.Ordered](name string) *ChunkedSync[In, Out, K] {
	return &ChunkedSync[In, Out, K]{name: name}
}

func (c *ChunkedSync[In, Out, K]) Partitioner(p Partitioner[K]) *ChunkedSync[In, Out, K] {
	c.partitioner = p
	return c
}

func (c *ChunkedSync[In, Out, K]) Fetcher(f PartitionFetcher[In, K]) *ChunkedSync[In, Out, K] {
	c.fetcher = f
	return c
}

func (c *ChunkedSync[In, Out, K]) Mapper(m datasync.Mapper[In, Out]) *ChunkedSync[In, Out, K] {
	c.mapper = m
	return c
}

func (c *ChunkedSync[In, Out, K]) Sink(s datasync.Sink[Out]) *ChunkedSync[In, Out, K] {
	c.sink = s
	return c
}

func (c *ChunkedSync[In, Out, K]) WithTracker(t ProgressTracker[K]) *ChunkedSync[In, Out, K] {
	c.tracker = t
	return c
}

func (c *ChunkedSync[In, Out, K]) PartitionSleep(d time.Duration) *ChunkedSync[In, Out, K] {
	c.partitionSleep = d
	return c
}

func (c *ChunkedSync[In, Out, K]) Schedule(d time.Duration) *ChunkedSync[In, Out, K] {
	c.schedule = d
	return c
}

func (c *ChunkedSync[In, Out, K]) RateLimitRetry(opts RateLimitOpts) *ChunkedSync[In, Out, K] {
	c.rateLimitOpts = &opts
	return c
}

func (c *ChunkedSync[In, Out, K]) ActivityRetry(p temporal.RetryPolicy) *ChunkedSync[In, Out, K] {
	c.activityRetry = &p
	return c
}

func (c *ChunkedSync[In, Out, K]) ActivityTimeouts(startToClose, hb time.Duration) *ChunkedSync[In, Out, K] {
	c.startToClose = startToClose
	c.heartbeat = hb
	return c
}

func (c *ChunkedSync[In, Out, K]) MaxPartitionsPerExecution(n int) *ChunkedSync[In, Out, K] {
	c.maxPerExec = n
	return c
}

func (c *ChunkedSync[In, Out, K]) Disabled(b bool) *ChunkedSync[In, Out, K] {
	c.disabled = b
	return c
}

// Build constructs the FullJobRegistration. Panics if a required field is
// missing — caught at process startup, not in production hot paths.
func (c *ChunkedSync[In, Out, K]) Build() datasyncwf.FullJobRegistration {
	if c.partitioner == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Partitioner is required", c.name))
	}
	if c.fetcher == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Fetcher is required", c.name))
	}
	if c.mapper == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Mapper is required", c.name))
	}
	if c.sink == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Sink is required", c.name))
	}

	jobName := c.name
	partitionsActName := jobName + ".Partitions"
	runPartitionActName := jobName + ".RunPartition"
	readCursorActName := jobName + ".ReadCursor"
	advanceCursorActName := jobName + ".AdvanceCursor"

	fetcher := c.fetcher
	if c.rateLimitOpts != nil {
		fetcher = WithRateLimitRetry[In, K](c.fetcher, *c.rateLimitOpts)
	}
	mapper := c.mapper
	sink := c.sink
	tracker := c.tracker
	partitioner := c.partitioner

	startToClose := c.startToClose
	if startToClose == 0 {
		startToClose = defaultStartToCloseTimeout
	}
	hbTimeout := c.heartbeat
	if hbTimeout == 0 {
		hbTimeout = defaultHeartbeatTimeout
	}

	retry := c.activityRetry
	if retry == nil {
		p := defaultActivityRetryPolicy
		retry = &p
	}
	partitionActivityOptions := workflow.ActivityOptions{
		StartToCloseTimeout: startToClose,
		HeartbeatTimeout:    hbTimeout,
		RetryPolicy:         retry,
	}
	partitionsListOptions := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retry,
	}

	// Activity closures (bind type parameters at registration time).
	partitionsActFn := func(ctx context.Context) ([]Partition[K], error) {
		return partitioner.Partitions(ctx)
	}
	runPartitionActFn := func(ctx context.Context, in runPartitionInput[K]) (PartitionResult[K], error) {
		return runPartition[In, Out, K](ctx, in, fetcher, mapper, sink)
	}

	type cursorResult struct {
		Cursor K    `json:"cursor"`
		Exists bool `json:"exists"`
	}
	var readCursorActFn func(ctx context.Context, jobName string) (cursorResult, error)
	var advanceCursorActFn func(ctx context.Context, completed K) error
	if tracker != nil {
		readCursorActFn = func(ctx context.Context, name string) (cursorResult, error) {
			cur, exists, err := tracker.Cursor(ctx, name)
			return cursorResult{Cursor: cur, Exists: exists}, err
		}
		advanceCursorActFn = func(ctx context.Context, completed K) error {
			return tracker.Advance(ctx, jobName, completed)
		}
	}

	wfState := chunkedSyncWorkflow[In, Out, K]{
		jobName:                  jobName,
		partitionsActivityName:   partitionsActName,
		runPartitionActivityName: runPartitionActName,
		readCursorActivityName:   readCursorActName,
		advanceCursorActivityName: advanceCursorActName,
		partitionActivityOptions:  partitionActivityOptions,
		partitionsListOptions:     partitionsListOptions,
		partitionSleep:            c.partitionSleep,
		hasTracker:                tracker != nil,
		maxPerExec:                c.maxPerExec,
	}

	return datasyncwf.FullJobRegistration{
		Name:      jobName,
		TaskQueue: datasyncwf.TaskQueue(jobName),
		Schedule:  c.schedule,
		Disabled:  c.disabled,
		WorkflowInput: payload.SyncExecutionInput{
			JobName: jobName,
		},
		Register: func(w worker.Worker) {
			w.RegisterWorkflowWithOptions(wfState.run, workflow.RegisterOptions{Name: jobName})
			w.RegisterActivityWithOptions(partitionsActFn, activity.RegisterOptions{Name: partitionsActName})
			w.RegisterActivityWithOptions(runPartitionActFn, activity.RegisterOptions{Name: runPartitionActName})
			if tracker != nil {
				w.RegisterActivityWithOptions(readCursorActFn, activity.RegisterOptions{Name: readCursorActName})
				w.RegisterActivityWithOptions(advanceCursorActFn, activity.RegisterOptions{Name: advanceCursorActName})
			}
		},
	}
}

// chunkedSyncWorkflow holds the workflow-side configuration captured at
// Build time. The run method is the registered workflow function.
type chunkedSyncWorkflow[In, Out any, K cmp.Ordered] struct {
	jobName                   string
	partitionsActivityName    string
	runPartitionActivityName  string
	readCursorActivityName    string
	advanceCursorActivityName string
	partitionActivityOptions  workflow.ActivityOptions
	partitionsListOptions     workflow.ActivityOptions
	partitionSleep            time.Duration
	hasTracker                bool
	maxPerExec                int
}

// run is the Temporal workflow function. Implemented in Tasks 13–15.
func (s chunkedSyncWorkflow[In, Out, K]) run(ctx workflow.Context, _ payload.SyncExecutionInput) (SyncResult[K], error) {
	return SyncResult[K]{JobName: s.jobName}, fmt.Errorf("ChunkedSync workflow not yet implemented")
}
```

- [ ] **Step 4: Run, expect Build-validation tests pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

The Build-validation tests (panic on missing fields, registration metadata) should pass. Workflow tests don't exist yet.

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/sync.go datasync/chunk/sync_test.go
git commit -m "feat(chunk): add ChunkedSync builder skeleton"
```

---

## Task 13: Implement Workflow Happy Path (No Tracker)

**Files:**
- Modify: `datasync/chunk/sync.go` — replace stub `run` method
- Modify: `datasync/chunk/sync_test.go` — add workflow tests for no-tracker path

- [ ] **Step 1: Write failing test for happy path**

Append to `datasync/chunk/sync_test.go`:

```go
func TestChunkedSync_Workflow_NoTracker_AllPartitionsProcessed(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			return PartitionResult[int64]{
				Start:    in.Partition.Start,
				End:      in.Partition.End,
				Fetched:  3,
				Inserted: 3,
			}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                  "job-x",
		partitionsActivityName:   "job-x.Partitions",
		runPartitionActivityName: "job-x.RunPartition",
		partitionActivityOptions: workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:    workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})

	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result SyncResult[int64]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalPartitions)
	assert.Equal(t, 6, result.TotalFetched)
	assert.Equal(t, 6, result.TotalInserted)
	assert.Len(t, result.Partitions, 2)
}
```

Add imports to `sync_test.go` if missing:

```go
import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/datasync/payload"
)
```

- [ ] **Step 2: Run, expect fail (workflow returns "not yet implemented")**

- [ ] **Step 3: Implement workflow run() — no-tracker path**

Replace the stub `run` method in `datasync/chunk/sync.go`:

```go
func (s chunkedSyncWorkflow[In, Out, K]) run(ctx workflow.Context, input payload.SyncExecutionInput) (SyncResult[K], error) {
	summary := SyncResult[K]{JobName: s.jobName}

	// 1. Get partition list via activity.
	listCtx := workflow.WithActivityOptions(ctx, s.partitionsListOptions)
	var parts []Partition[K]
	if err := workflow.ExecuteActivity(listCtx, s.partitionsActivityName).Get(listCtx, &parts); err != nil {
		return summary, fmt.Errorf("partitions: %w", err)
	}
	if len(parts) == 0 {
		return summary, nil
	}

	// 2. Process each partition.
	partCtx := workflow.WithActivityOptions(ctx, s.partitionActivityOptions)
	for i, p := range parts {
		var pr PartitionResult[K]
		if err := workflow.ExecuteActivity(partCtx, s.runPartitionActivityName, runPartitionInput[K]{
			Partition: p,
			JobName:   s.jobName,
		}).Get(partCtx, &pr); err != nil {
			return summary, fmt.Errorf("partition %v..%v: %w", p.Start, p.End, err)
		}
		summary.Partitions = append(summary.Partitions, pr)
		summary.TotalPartitions++
		summary.TotalFetched += pr.Fetched
		summary.TotalInserted += pr.Inserted
		summary.TotalUpdated += pr.Updated
		summary.TotalSkipped += pr.Skipped

		if i < len(parts)-1 && s.partitionSleep > 0 {
			if err := workflow.Sleep(ctx, s.partitionSleep); err != nil {
				return summary, fmt.Errorf("partition-sleep: %w", err)
			}
		}
	}

	_ = input // reserved for future use (input contains JobName for logging)
	return summary, nil
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/sync.go datasync/chunk/sync_test.go
git commit -m "feat(chunk): implement workflow happy path without tracker"
```

---

## Task 14: Workflow with Tracker — Cursor Read and Advance

**Files:**
- Modify: `datasync/chunk/sync.go` — extend `run` with cursor logic
- Modify: `datasync/chunk/sync_test.go` — add tracker tests

- [ ] **Step 1: Write failing tests covering all cursor scenarios**

Append to `datasync/chunk/sync_test.go`:

```go
type stubTracker struct {
	cursor   int64
	exists   bool
	advanced []int64
	readErr  error
	advErr   error
}

func (s *stubTracker) Cursor(_ context.Context, _ string) (int64, bool, error) {
	return s.cursor, s.exists, s.readErr
}

func (s *stubTracker) Advance(_ context.Context, _ string, c int64) error {
	if s.advErr != nil {
		return s.advErr
	}
	s.advanced = append(s.advanced, c)
	return nil
}

type cursorResultLocal struct {
	Cursor int64 `json:"cursor"`
	Exists bool  `json:"exists"`
}

func TestChunkedSync_Workflow_Tracker_NoCursor_ProcessesAll(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResultLocal{Cursor: 0, Exists: false}, nil)
	advanced := []int64{}
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).
		Return(func(_ context.Context, c int64) error {
			advanced = append(advanced, c)
			return nil
		})
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End, Fetched: 1, Inserted: 1}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	assert.Equal(t, []int64{10, 20}, advanced)
}

func TestChunkedSync_Workflow_Tracker_MidRangeCursor_FiltersBefore(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
	}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResultLocal{Cursor: 10, Exists: true}, nil)
	processed := []int64{}
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			processed = append(processed, in.Partition.Start)
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
		})
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).Return(nil)

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Partitions starting at 0 (< cursor 10) are skipped; 10 and 20 are processed.
	assert.Equal(t, []int64{10, 20}, processed)
}

func TestChunkedSync_Workflow_Tracker_CursorPastEnd_ProcessesNothing(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResultLocal{Cursor: 100, Exists: true}, nil)

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result SyncResult[int64]
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 0, result.TotalPartitions)
}

func TestChunkedSync_Workflow_Tracker_PartitionFails_CursorNotAdvanced(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResultLocal{Cursor: 0, Exists: false}, nil)
	advanced := []int64{}
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).
		Return(func(_ context.Context, c int64) error {
			advanced = append(advanced, c)
			return nil
		})
	calls := 0
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			calls++
			if calls == 1 {
				return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
			}
			return PartitionResult[int64]{}, errors.New("boom")
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())

	// Cursor advanced for the first (successful) partition only.
	assert.Equal(t, []int64{10}, advanced)
}
```

Add the missing import to `sync_test.go`:

```go
import "errors"
```

- [ ] **Step 2: Run, expect fail (workflow doesn't yet read cursor or advance it)**

- [ ] **Step 3: Extend `run` to handle tracker**

Replace the `run` method body in `datasync/chunk/sync.go`:

```go
func (s chunkedSyncWorkflow[In, Out, K]) run(ctx workflow.Context, input payload.SyncExecutionInput) (SyncResult[K], error) {
	summary := SyncResult[K]{JobName: s.jobName}

	listCtx := workflow.WithActivityOptions(ctx, s.partitionsListOptions)
	var parts []Partition[K]
	if err := workflow.ExecuteActivity(listCtx, s.partitionsActivityName).Get(listCtx, &parts); err != nil {
		return summary, fmt.Errorf("partitions: %w", err)
	}
	if len(parts) == 0 {
		return summary, nil
	}

	// Cursor filtering when tracker is configured.
	if s.hasTracker {
		type cursorResult struct {
			Cursor K    `json:"cursor"`
			Exists bool `json:"exists"`
		}
		cursorCtx := workflow.WithActivityOptions(ctx, defaultCursorActivityOptions)
		var cur cursorResult
		if err := workflow.ExecuteActivity(cursorCtx, s.readCursorActivityName, s.jobName).Get(cursorCtx, &cur); err != nil {
			return summary, fmt.Errorf("read cursor: %w", err)
		}
		if cur.Exists {
			filtered := make([]Partition[K], 0, len(parts))
			for _, p := range parts {
				if p.Start >= cur.Cursor {
					filtered = append(filtered, p)
				}
			}
			parts = filtered
		}
		if len(parts) == 0 {
			return summary, nil
		}
	}

	partCtx := workflow.WithActivityOptions(ctx, s.partitionActivityOptions)
	cursorCtx := workflow.WithActivityOptions(ctx, defaultCursorActivityOptions)
	for i, p := range parts {
		var pr PartitionResult[K]
		if err := workflow.ExecuteActivity(partCtx, s.runPartitionActivityName, runPartitionInput[K]{
			Partition: p,
			JobName:   s.jobName,
		}).Get(partCtx, &pr); err != nil {
			return summary, fmt.Errorf("partition %v..%v: %w", p.Start, p.End, err)
		}
		summary.Partitions = append(summary.Partitions, pr)
		summary.TotalPartitions++
		summary.TotalFetched += pr.Fetched
		summary.TotalInserted += pr.Inserted
		summary.TotalUpdated += pr.Updated
		summary.TotalSkipped += pr.Skipped

		if s.hasTracker {
			if err := workflow.ExecuteActivity(cursorCtx, s.advanceCursorActivityName, p.End).Get(cursorCtx, nil); err != nil {
				return summary, fmt.Errorf("advance cursor: %w", err)
			}
		}

		if i < len(parts)-1 && s.partitionSleep > 0 {
			if err := workflow.Sleep(ctx, s.partitionSleep); err != nil {
				return summary, fmt.Errorf("partition-sleep: %w", err)
			}
		}
	}

	_ = input
	return summary, nil
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

All tracker tests should pass. The earlier no-tracker test still passes because `hasTracker=false` skips the new code.

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/sync.go datasync/chunk/sync_test.go
git commit -m "feat(chunk): add tracker-based cursor read and advance"
```

---

## Task 15: MaxPartitionsPerExecution + ContinueAsNew

**Files:**
- Modify: `datasync/chunk/sync.go` — add ContinueAsNew branch
- Modify: `datasync/chunk/sync_test.go` — add MaxPartitionsPerExecution test

- [ ] **Step 1: Write failing test**

Append to `datasync/chunk/sync_test.go`:

```go
func TestChunkedSync_Workflow_MaxPerExecution_TriggersContinueAsNew(t *testing.T) {
	suite := &testsuite.WorkflowTestSuite{}
	env := suite.NewTestWorkflowEnvironment()

	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
		{Start: 30, End: 40},
	}
	env.OnActivity("job-x.Partitions", mock.Anything).Return(parts, nil)
	env.OnActivity("job-x.ReadCursor", mock.Anything, "job-x").
		Return(cursorResultLocal{Cursor: 0, Exists: false}, nil)
	env.OnActivity("job-x.AdvanceCursor", mock.Anything, mock.Anything).Return(nil)
	processed := []int64{}
	env.OnActivity("job-x.RunPartition", mock.Anything, mock.Anything).
		Return(func(_ context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
			processed = append(processed, in.Partition.Start)
			return PartitionResult[int64]{Start: in.Partition.Start, End: in.Partition.End}, nil
		})

	wf := chunkedSyncWorkflow[string, string, int64]{
		jobName:                   "job-x",
		partitionsActivityName:    "job-x.Partitions",
		runPartitionActivityName:  "job-x.RunPartition",
		readCursorActivityName:    "job-x.ReadCursor",
		advanceCursorActivityName: "job-x.AdvanceCursor",
		partitionActivityOptions:  workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		partitionsListOptions:     workflow.ActivityOptions{StartToCloseTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}},
		hasTracker:                true,
		maxPerExec:                2,
	}
	env.RegisterWorkflowWithOptions(wf.run, workflow.RegisterOptions{Name: "job-x"})
	env.ExecuteWorkflow("job-x", payload.SyncExecutionInput{JobName: "job-x"})
	require.True(t, env.IsWorkflowCompleted())

	// ContinueAsNew is signaled by the workflow returning a *workflow.ContinueAsNewError;
	// the test environment surfaces it as the workflow error.
	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.True(t, workflow.IsContinueAsNewError(err), "expected ContinueAsNewError, got %T: %v", err, err)

	// Only the first 2 partitions processed in this execution.
	assert.Equal(t, []int64{0, 10}, processed)
}
```

- [ ] **Step 2: Run, expect fail (no truncation/continue logic yet)**

- [ ] **Step 3: Replace the `run` method with the final version**

Replace the `run` method body in `datasync/chunk/sync.go` with the full version below (adds MaxPerExec truncation before the for-loop and ContinueAsNew at the end):

```go
func (s chunkedSyncWorkflow[In, Out, K]) run(ctx workflow.Context, input payload.SyncExecutionInput) (SyncResult[K], error) {
	summary := SyncResult[K]{JobName: s.jobName}

	listCtx := workflow.WithActivityOptions(ctx, s.partitionsListOptions)
	var parts []Partition[K]
	if err := workflow.ExecuteActivity(listCtx, s.partitionsActivityName).Get(listCtx, &parts); err != nil {
		return summary, fmt.Errorf("partitions: %w", err)
	}
	if len(parts) == 0 {
		return summary, nil
	}

	if s.hasTracker {
		type cursorResult struct {
			Cursor K    `json:"cursor"`
			Exists bool `json:"exists"`
		}
		cursorCtx := workflow.WithActivityOptions(ctx, defaultCursorActivityOptions)
		var cur cursorResult
		if err := workflow.ExecuteActivity(cursorCtx, s.readCursorActivityName, s.jobName).Get(cursorCtx, &cur); err != nil {
			return summary, fmt.Errorf("read cursor: %w", err)
		}
		if cur.Exists {
			filtered := make([]Partition[K], 0, len(parts))
			for _, p := range parts {
				if p.Start >= cur.Cursor {
					filtered = append(filtered, p)
				}
			}
			parts = filtered
		}
		if len(parts) == 0 {
			return summary, nil
		}
	}

	// Truncate to MaxPartitionsPerExecution; defer the rest to a fresh execution.
	deferred := false
	if s.maxPerExec > 0 && len(parts) > s.maxPerExec {
		parts = parts[:s.maxPerExec]
		deferred = true
	}

	partCtx := workflow.WithActivityOptions(ctx, s.partitionActivityOptions)
	cursorCtx := workflow.WithActivityOptions(ctx, defaultCursorActivityOptions)
	for i, p := range parts {
		var pr PartitionResult[K]
		if err := workflow.ExecuteActivity(partCtx, s.runPartitionActivityName, runPartitionInput[K]{
			Partition: p,
			JobName:   s.jobName,
		}).Get(partCtx, &pr); err != nil {
			return summary, fmt.Errorf("partition %v..%v: %w", p.Start, p.End, err)
		}
		summary.Partitions = append(summary.Partitions, pr)
		summary.TotalPartitions++
		summary.TotalFetched += pr.Fetched
		summary.TotalInserted += pr.Inserted
		summary.TotalUpdated += pr.Updated
		summary.TotalSkipped += pr.Skipped

		if s.hasTracker {
			if err := workflow.ExecuteActivity(cursorCtx, s.advanceCursorActivityName, p.End).Get(cursorCtx, nil); err != nil {
				return summary, fmt.Errorf("advance cursor: %w", err)
			}
		}

		if i < len(parts)-1 && s.partitionSleep > 0 {
			if err := workflow.Sleep(ctx, s.partitionSleep); err != nil {
				return summary, fmt.Errorf("partition-sleep: %w", err)
			}
		}
	}

	if deferred {
		return summary, workflow.NewContinueAsNewError(ctx, s.jobName, input)
	}
	return summary, nil
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 5: Lint**

```bash
task lint
```

- [ ] **Step 6: Commit**

```bash
git add datasync/chunk/sync.go datasync/chunk/sync_test.go
git commit -m "feat(chunk): bound execution via MaxPartitionsPerExecution + ContinueAsNew"
```

---

## Task 16: DateChunkedSync Wrapper

**Files:**
- Create: `datasync/chunk/date_sync.go`
- Create: `datasync/chunk/date_sync_test.go`

- [ ] **Step 1: Write failing tests**

Create `datasync/chunk/date_sync_test.go`:

```go
package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/datasync"
)

func TestDateChunkedSync_Build_ConfiguresDatePartitioner(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	require.NoError(t, err)

	reg := NewDateChunkedSync[string, string]("date-job").
		LookBack(48 * time.Hour).
		ChunkSize(24 * time.Hour).
		Timezone(loc).
		Fetcher(func(_ context.Context, _, _ time.Time) ([]string, error) {
			return []string{"x"}, nil
		}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		Schedule(15 * time.Minute).
		Build()

	assert.Equal(t, "date-job", reg.Name)
	assert.Equal(t, "sync-date-job", reg.TaskQueue)
	assert.Equal(t, 15*time.Minute, reg.Schedule)
}

func TestDateChunkedSync_Fetcher_ConvertsKeysToTime(t *testing.T) {
	got := struct {
		start, end time.Time
	}{}
	d := NewDateChunkedSync[string, string]("date-job").
		LookBack(time.Hour).
		ChunkSize(time.Hour).
		Fetcher(func(_ context.Context, s, e time.Time) ([]string, error) {
			got.start = s
			got.end = e
			return nil, nil
		}).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"})

	// Drive the inner fetcher directly with int64 keys.
	startKey := TimeToKey(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC))
	endKey := TimeToKey(time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC))
	_, err := d.inner.fetcher(context.Background(), startKey, endKey)
	require.NoError(t, err)
	assert.True(t, got.start.Equal(time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)))
	assert.True(t, got.end.Equal(time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC)))
}

func TestDateChunkedSync_Tracker_ConvertsCursor(t *testing.T) {
	timeCursor := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	advancedTimes := []time.Time{}

	tracker := stubTimeTracker{
		cursor: timeCursor,
		exists: true,
		advance: func(t time.Time) error {
			advancedTimes = append(advancedTimes, t)
			return nil
		},
	}

	d := NewDateChunkedSync[string, string]("date-job").
		LookBack(time.Hour).
		ChunkSize(time.Hour).
		Fetcher(func(_ context.Context, _, _ time.Time) ([]string, error) { return nil, nil }).
		Mapper(datasync.IdentityMapper[string]()).
		Sink(&stubSink{name: "sink"}).
		WithTracker(tracker)

	require.NotNil(t, d.inner.tracker, "tracker adapter should be installed")

	// Read cursor via inner adapter.
	gotKey, exists, err := d.inner.tracker.Cursor(context.Background(), "date-job")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, TimeToKey(timeCursor), gotKey)

	// Advance via inner adapter.
	advanceTime := time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC)
	require.NoError(t, d.inner.tracker.Advance(context.Background(), "date-job", TimeToKey(advanceTime)))
	require.Len(t, advancedTimes, 1)
	assert.True(t, advanceTime.Equal(advancedTimes[0]))
}

type stubTimeTracker struct {
	cursor  time.Time
	exists  bool
	advance func(time.Time) error
}

func (s stubTimeTracker) Cursor(_ context.Context, _ string) (time.Time, bool, error) {
	return s.cursor, s.exists, nil
}

func (s stubTimeTracker) Advance(_ context.Context, _ string, t time.Time) error {
	if s.advance != nil {
		return s.advance(t)
	}
	return nil
}
```

- [ ] **Step 2: Run, expect fail (DateChunkedSync undefined)**

- [ ] **Step 3: Implement `date_sync.go`**

Create `datasync/chunk/date_sync.go`:

```go
package chunk

import (
	"context"
	"time"

	"go.temporal.io/sdk/temporal"

	"github.com/jasoet/go-wf/datasync"
	datasyncwf "github.com/jasoet/go-wf/datasync/workflow"
)

// DateChunkedSync wraps ChunkedSync[In, Out, int64] with a time.Time-based
// API. Internally, time.Time keys are projected onto Unix-nanosecond int64
// keys so the generic ChunkedSync builder (which requires K cmp.Ordered)
// can be used unchanged.
type DateChunkedSync[In, Out any] struct {
	inner    *ChunkedSync[In, Out, int64]
	loc      *time.Location
	lookBack time.Duration
	chunkSz  time.Duration
}

// NewDateChunkedSync starts a DateChunkedSync builder for a job named name.
func NewDateChunkedSync[In, Out any](name string) *DateChunkedSync[In, Out] {
	return &DateChunkedSync[In, Out]{
		inner: NewChunkedSync[In, Out, int64](name),
		loc:   time.UTC,
	}
}

func (d *DateChunkedSync[In, Out]) LookBack(window time.Duration) *DateChunkedSync[In, Out] {
	d.lookBack = window
	return d
}

func (d *DateChunkedSync[In, Out]) ChunkSize(size time.Duration) *DateChunkedSync[In, Out] {
	d.chunkSz = size
	return d
}

func (d *DateChunkedSync[In, Out]) Timezone(loc *time.Location) *DateChunkedSync[In, Out] {
	if loc != nil {
		d.loc = loc
	}
	return d
}

// Fetcher accepts a time.Time-based fetcher; it is wrapped to satisfy the
// inner int64-keyed builder.
func (d *DateChunkedSync[In, Out]) Fetcher(f PartitionFetcher[In, time.Time]) *DateChunkedSync[In, Out] {
	d.inner.Fetcher(func(ctx context.Context, start, end int64) ([]In, error) {
		return f(ctx, KeyToTime(start), KeyToTime(end))
	})
	return d
}

func (d *DateChunkedSync[In, Out]) Mapper(m datasync.Mapper[In, Out]) *DateChunkedSync[In, Out] {
	d.inner.Mapper(m)
	return d
}

func (d *DateChunkedSync[In, Out]) Sink(s datasync.Sink[Out]) *DateChunkedSync[In, Out] {
	d.inner.Sink(s)
	return d
}

// WithTracker adapts a time.Time-keyed tracker to the inner int64 representation.
func (d *DateChunkedSync[In, Out]) WithTracker(t ProgressTracker[time.Time]) *DateChunkedSync[In, Out] {
	d.inner.WithTracker(timeTrackerAdapter{inner: t})
	return d
}

func (d *DateChunkedSync[In, Out]) PartitionSleep(s time.Duration) *DateChunkedSync[In, Out] {
	d.inner.PartitionSleep(s)
	return d
}

func (d *DateChunkedSync[In, Out]) Schedule(s time.Duration) *DateChunkedSync[In, Out] {
	d.inner.Schedule(s)
	return d
}

func (d *DateChunkedSync[In, Out]) RateLimitRetry(opts RateLimitOpts) *DateChunkedSync[In, Out] {
	d.inner.RateLimitRetry(opts)
	return d
}

func (d *DateChunkedSync[In, Out]) ActivityRetry(p temporal.RetryPolicy) *DateChunkedSync[In, Out] {
	d.inner.ActivityRetry(p)
	return d
}

func (d *DateChunkedSync[In, Out]) ActivityTimeouts(startToClose, hb time.Duration) *DateChunkedSync[In, Out] {
	d.inner.ActivityTimeouts(startToClose, hb)
	return d
}

func (d *DateChunkedSync[In, Out]) MaxPartitionsPerExecution(n int) *DateChunkedSync[In, Out] {
	d.inner.MaxPartitionsPerExecution(n)
	return d
}

func (d *DateChunkedSync[In, Out]) Disabled(b bool) *DateChunkedSync[In, Out] {
	d.inner.Disabled(b)
	return d
}

// Build configures the DatePartitioner from LookBack/ChunkSize/Timezone and
// delegates to ChunkedSync.Build.
func (d *DateChunkedSync[In, Out]) Build() datasyncwf.FullJobRegistration {
	d.inner.Partitioner(&DatePartitioner{
		Loc:       d.loc,
		LookBack:  d.lookBack,
		ChunkSize: d.chunkSz,
	})
	return d.inner.Build()
}

// timeTrackerAdapter projects a ProgressTracker[time.Time] onto the int64-keyed
// interface used by the inner ChunkedSync.
type timeTrackerAdapter struct {
	inner ProgressTracker[time.Time]
}

func (a timeTrackerAdapter) Cursor(ctx context.Context, jobName string) (int64, bool, error) {
	t, exists, err := a.inner.Cursor(ctx, jobName)
	if err != nil {
		return 0, false, err
	}
	if !exists {
		return 0, false, nil
	}
	return TimeToKey(t), true, nil
}

func (a timeTrackerAdapter) Advance(ctx context.Context, jobName string, completed int64) error {
	return a.inner.Advance(ctx, jobName, KeyToTime(completed))
}
```

- [ ] **Step 4: Run, expect pass**

```bash
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 5: Commit**

```bash
git add datasync/chunk/date_sync.go datasync/chunk/date_sync_test.go
git commit -m "feat(chunk): add DateChunkedSync wrapper for time.Time keys"
```

---

## Task 17: Package godoc

**Files:**
- Create: `datasync/chunk/doc.go`

- [ ] **Step 1: Implement `doc.go`**

Create `datasync/chunk/doc.go`:

```go
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
```

- [ ] **Step 2: Verify build**

```bash
task lint
task test:pkg -- ./datasync/chunk/...
```

- [ ] **Step 3: Commit**

```bash
git add datasync/chunk/doc.go
git commit -m "docs(chunk): add package godoc with usage example"
```

---

## Task 18: Final Verification

- [ ] **Step 1: Run the full test suite**

```bash
task ci:test
```

Expected: all tests pass (including pre-existing datasync/activity tests after the heartbeat refactor).

- [ ] **Step 2: Run the full lint**

```bash
task lint
```

Expected: clean.

- [ ] **Step 3: Verify no para-sync changes leaked into this branch**

```bash
git diff main --stat -- ':(exclude)docs/' | grep -E '^\s*(para|gitlab)' || echo "no para-sync touched"
```

Expected: `no para-sync touched` (the chunk package lives entirely in go-wf).

- [ ] **Step 4: Push branch + open PR**

```bash
git push -u origin feat/datasync-chunk
gh pr create --title "feat(datasync): add chunk package for partitioned sync" --body "$(cat <<'EOF'
## Summary
- Adds `datasync/chunk` package with `ChunkedSync[In, Out, K]` and `DateChunkedSync[In, Out]` builders.
- Extracts heartbeat helpers into `datasync/internal/heartbeat` for reuse between `datasync/activity` and `datasync/chunk`.
- No changes to `datasync/job.go`, `datasync/workflow/`, `datasync/builder/`, or any consumer (para-sync untouched).

## Spec
docs/superpowers/specs/2026-05-10-datasync-chunk-design.md

## Test plan
- [ ] task ci:test
- [ ] task lint
- [ ] Verify para-sync builds against this go-wf branch (out of scope for this PR)
EOF
)"
```

---

## Self-Review Notes

- **Spec coverage:** every section of the spec maps to a task — heartbeat extraction (Task 1–2), key conversion (3), interfaces and types (4–6), partitioner (7), sleeper (8), rate limiter (9), iterate (10), activity (11), builder + workflow (12–15), date wrapper (16), docs (17), verification (18).
- **Type consistency:** `runPartitionInput[K]` defined in Task 11 is referenced unchanged in Tasks 12–15. `ChunkedSync[In, Out, K cmp.Ordered]` constraint is consistent throughout. `cursorResult` is defined as a local struct in `Build()` (Task 12) and matched by `cursorResultLocal` in tests (Task 14) — both have `Cursor K` + `Exists bool` shape.
- **No placeholders:** every code step shows complete code. No "implement here" or "similar to above".
