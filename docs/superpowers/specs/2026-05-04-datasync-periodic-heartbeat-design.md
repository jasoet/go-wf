# datasync: Periodic Heartbeat During SyncData Activity — Design

**Date:** 2026-05-04
**Source spec:** `docs/plans/2026-05-04-datasync-periodic-heartbeat.md`
**Affected package:** `datasync/activity` (single file change)
**Status:** Approved, ready for implementation plan

## Problem

`datasync/activity/sync.go::SyncData` only calls `activity.RecordHeartbeat` *after* `Source.Fetch` and `Sink.Write` complete. When either call exceeds the configured `HeartbeatTimeout` (default `30s`), Temporal kills the activity even though it is making forward progress.

Confirmed in production by the `para-sync` consumer: jobs configured with `HeartbeatTimeout: 30s` intermittently fail (~1 in 10 runs) when source DBs are slow, even though no actual deadlock exists. See source spec for failure event timeline.

## Solution

Add a long-lived heartbeat goroutine that runs for the entire lifetime of `SyncData` and ticks at a derived interval. The main goroutine updates a phase string atomically as it transitions through fetch / map / write so the heartbeat payload reflects current activity.

**Change scope is intentionally tight:** `datasync/activity/sync.go` and `datasync/activity/sync_test.go` only. No changes to `workflow/sync.go`, `builder/builder.go`, or `datasync/job.go`. No new public API surface, no new `Job` fields, no behavior change for consumers who don't hit the bug.

## Design

### Helper functions (new, package-local in `activity/sync.go`)

```go
// heartbeatInterval derives the periodic heartbeat tick from the configured
// HeartbeatTimeout. Returns max(1s, hbTimeout/3); falls back to 10s when
// hbTimeout is 0 (no timeout configured).
func heartbeatInterval(hbTimeout time.Duration) time.Duration

// heartbeatLoop ticks at the given interval until done is closed or ctx is
// cancelled, calling activity.RecordHeartbeat with the current phase string.
// Phase is read via *atomic.Pointer[string] so the main goroutine can update
// it without a mutex.
func heartbeatLoop(
    ctx context.Context,
    interval time.Duration,
    jobName string,
    phase *atomic.Pointer[string],
    done <-chan struct{},
)
```

`heartbeatInterval` rationale:

| `HeartbeatTimeout` | Derived interval | Notes |
|---|---|---|
| 0 (unset) | 10s | Sensible default; matches reference impl. |
| 30s (current default) | 10s | 3 beats per timeout window. |
| 1m | 20s | 3 beats. |
| 5m | 1m40s | 3 beats. |
| 100ms (pathological) | 1s | Floor; avoids hammering Temporal. |

`heartbeatLoop` exits cleanly when `done` is closed (normal path) or `ctx` is cancelled (e.g., heartbeat timeout already tripped, retry triggered).

### Changes inside `SyncData`

Insert at the top of `SyncData`, before the existing `pkgotel.Layers.StartOperations` call:

```go
var phase atomic.Pointer[string]
setPhase := func(p string) { phase.Store(&p) }
setPhase("starting")

interval := heartbeatInterval(activity.GetInfo(ctx).HeartbeatTimeout)
done := make(chan struct{})
defer close(done)
go heartbeatLoop(ctx, interval, input.JobName, &phase, done)
```

Then add `setPhase(...)` calls at three points in the existing flow:

- `setPhase("fetching")` immediately before `a.source.Fetch(...)`
- `setPhase("mapping")` immediately before `a.mapper.Map(...)`
- `setPhase("writing")` immediately before `a.sink.Write(...)`

The existing two checkpoint heartbeats stay as-is:

- `activity.RecordHeartbeat(ctx, "fetched N records")` after fetch returns
- `activity.RecordHeartbeat(ctx, "wrote N records")` after write returns

Those are valuable progress markers for debugging — the periodic goroutine adds a third "working" beat in between, distinct from the checkpoints.

### Heartbeat payload format

Periodic beats use `"syncing <jobName>: <phase>"`, e.g.:

- `"syncing ticketing-master-sync: fetching"`
- `"syncing ticketing-master-sync: writing"`

This makes Temporal UI immediately useful when an activity is slow: operators can see *which* phase is taking time without correlating event timestamps.

### Out of scope (preserved from source spec)

- **No change to `defaultHeartbeatTimeout`** (stays `30s`). The goroutine fix solves the reported bug; bumping the default is a separate behavior change. See "Failure Modes" below for why `30s` remains the right default once the goroutine exists.
- **No change to existing checkpoint heartbeat payloads** (`"fetched N records"`, `"wrote N records"`).
- **No new builder/`Job` config fields.** Heartbeat interval is internal to the activity, derived from existing config.

## Failure Modes

This section makes explicit how the activity behaves under each failure scenario, and why the design is correct for each.

### 1. Healthy slow phase (the bug being fixed)

`Source.Fetch` takes 2 minutes against a slow DB; `HeartbeatTimeout=30s`.

- Heartbeat goroutine ticks every ~10s; Temporal sees beats; no timeout.
- `Fetch` returns; activity proceeds normally; succeeds.
- **Result:** bug fixed.

### 2. Worker death / network partition

Worker process is killed mid-activity, or loses network connection to Temporal.

- Heartbeat goroutine stops ticking (process dead) or its beats don't reach Temporal (network partition).
- After `HeartbeatTimeout` (30s), Temporal marks the activity failed with `TimeoutType: HEARTBEAT`.
- Retry policy schedules the activity on a different worker; fresh `SyncData` invocation runs with its own goroutine.
- **Result:** correct — heartbeat does its real job (worker-liveness detection).

### 3. True activity deadlock

`Source.Fetch` hangs forever (e.g., blocked on a network socket with no timeout) and ignores `ctx.Done()`.

- Heartbeat goroutine is independent of `Fetch` — keeps ticking.
- `HeartbeatTimeout` does **not** trip; the goroutine masks the deadlock from heartbeat detection.
- `StartToCloseTimeout` (default `5m`, configurable per-job) is the backstop. When it trips:
  - Temporal marks the activity failed with `TimeoutType: START_TO_CLOSE`.
  - Activity context is cancelled. The hung `Fetch` goroutine continues running in the worker (Go has no preemption), leaking until natural completion.
  - Retry policy kicks in; new attempt scheduled.
- **Result:** acceptable. The goroutine fix correctly separates two concerns the old code conflated:
  - **`HeartbeatTimeout`** = worker-liveness detector (fast detection of dead workers / network issues).
  - **`StartToCloseTimeout`** = activity-too-slow / deadlock detector (the actual end-to-end ceiling).
- Operators who currently rely on heartbeat timeout for deadlock detection should rely on `StartToCloseTimeout` instead — this matches Temporal's intended semantics.

### 4. Context cancellation propagation

When any timeout fires, the activity context is cancelled. Go goroutines are not preempted, so:

- `Source.Fetch` / `Sink.Write` must check `ctx.Done()` to bail out cooperatively.
- The heartbeat goroutine's `for { select { ... } }` includes `ctx.Done()` — it exits cleanly.
- The `defer close(done)` in `SyncData` is also exit-path-safe; `heartbeatLoop` selects on both `done` and `ctx.Done()`.

This behavior is unchanged from the current code. The fix does not alter cancellation semantics.

### 5. Activity panic

If `SyncData` panics (e.g., bug in mapper):

- Go runtime unwinds the stack; the `defer close(done)` runs; heartbeat goroutine exits.
- Temporal worker recovers from the panic, marks the activity failed, retry policy applies.
- **Result:** no goroutine leak; behavior unchanged from current code.

## Testing Plan

Three new tests in `datasync/activity/sync_test.go`:

### `TestActivities_SyncData_HeartbeatsDuringSlowFetch`

- Mock `Source.Fetch` sleeps 3 seconds before returning, respecting `ctx.Done()`.
- Configure test environment with `HeartbeatTimeout: 3 * time.Second` (yields tick interval = `max(1s, 3s/3)` = `1s`) and a `RecordHeartbeat` listener that captures every payload.
- Assert: `RecordHeartbeat` called more than twice (at least 2 periodic beats during the sleep, plus the post-fetch checkpoint).
- Assert: at least one captured payload contains `"fetching"`.

Test timing rationale: ticker fires at ~1s, ~2s, ~3s during the 3s sleep. Even with scheduling jitter, ≥2 periodic beats are guaranteed before the checkpoint. Using `HeartbeatTimeout` of `3s` (not a sub-second value) avoids fighting the `1s` floor in `heartbeatInterval`.

### `TestActivities_SyncData_HeartbeatsDuringSlowWrite`

- Mirror of the above — `Sink.Write` sleeps 3 seconds; same `HeartbeatTimeout: 3 * time.Second`; assert payload contains `"writing"`.

### `TestHeartbeatInterval` (table test)

| Input `hbTimeout` | Expected interval |
|---|---|
| `0` | `10 * time.Second` |
| `30 * time.Second` | `10 * time.Second` |
| `1 * time.Minute` | `20 * time.Second` |
| `5 * time.Minute` | `100 * time.Second` |
| `100 * time.Millisecond` | `1 * time.Second` (floor) |
| `1 * time.Second` | `1 * time.Second` (floor) |
| `3 * time.Second` | `1 * time.Second` (floor) |
| `4 * time.Second` | `1333 * time.Millisecond` |

Existing tests (`TestActivities_SyncData_Success`, `_EmptySource`, `_FetchError`, `_WriteError`, `TestToSyncExecutionOutput_*`) must continue to pass unchanged — the goroutine is additive.

## Verification After Implementation

Per the source spec: a consumer with `HeartbeatTimeout: 30s` and a `Source.Fetch` that sleeps 2 minutes should now succeed (not time out). Para-sync can roll back its defensive `HeartbeatTimeout` bumps once the fixed `go-wf` version ships.

## Implementation Notes

- Use `sync/atomic.Pointer[string]` (Go 1.19+, project is Go 1.26+) — no mutex needed for phase updates.
- `heartbeatLoop` should call `activity.RecordHeartbeat(ctx, ...)` directly, not through any abstraction. Temporal's heartbeating is the boundary the test asserts against.
- Goroutine cleanup via `defer close(done)` runs on every exit path (success, error, panic).
- No changes to `otel.go` or any metric definitions.
