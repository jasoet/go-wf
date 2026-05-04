# Datasync Periodic Heartbeat Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix `datasync/activity/sync.go::SyncData` heartbeat-timeout failures by adding a long-lived heartbeat goroutine that ticks during slow `Source.Fetch` / `Sink.Write` calls.

**Architecture:** Single file change. Add two package-local helpers (`heartbeatInterval`, `heartbeatLoop`) and modify `SyncData` to spawn one goroutine for the activity lifetime. Phase-aware payload (`"syncing <jobName>: <phase>"`) tracked via `atomic.Pointer[string]`. No public API changes; no changes to `workflow/`, `builder/`, or `job.go`.

**Tech Stack:** Go 1.26, Temporal Go SDK v1.40.0, `sync/atomic`, existing `testify` + `testsuite` packages already in use.

**Spec:** `docs/superpowers/specs/2026-05-04-datasync-periodic-heartbeat-design.md`
**Source bug report:** `docs/plans/2026-05-04-datasync-periodic-heartbeat.md`

**Branch:** `feat/datasync-periodic-heartbeat` (already created)

---

## File Structure

| File | Type | Responsibility |
|---|---|---|
| `datasync/activity/sync.go` | Modify | Add `heartbeatInterval` and `heartbeatLoop` helpers; spawn goroutine in `SyncData`; add `setPhase` calls before each phase. Add `atomic` and `fmt` (already imported) imports. |
| `datasync/activity/sync_test.go` | Modify | Add `sleep` field to `mockSource` and `mockSink` (with `ctx.Done()` respect); add three new tests. |

No new files. No changes elsewhere.

## Critical Background — Read Before Implementing

**Temporal SDK heartbeat throttling.** The activity SDK throttles heartbeats sent to the server at `0.8 × HeartbeatTimeout`:

- The first `RecordHeartbeat` after a quiet period is sent immediately.
- Subsequent calls within the throttle window are batched (only the most recent payload is kept).
- When the throttle timer expires, the latest batched payload is sent.
- **If the activity completes before the throttle timer fires, the batched payloads are DROPPED.** Only the first heartbeat reaches the listener.
- This is observable via `TestActivityEnvironment.SetOnActivityHeartbeatListener`, which mirrors the production batching behavior.

**Implication for tests:** assert that the listener captured **at least one** heartbeat with the expected phase string. Asserting "more than N heartbeats" is unreliable because the throttle may suppress most of them on success.

The first periodic tick fires at `~interval` after activity start. With `HeartbeatTimeout=3s`, interval is `1s`, sleep is `3s`, so the first tick at `t≈1s` is sent immediately and captures the current phase. That single capture is what the test asserts on.

---

## Task 1: Add `heartbeatInterval` Helper (TDD)

**Files:**
- Modify: `datasync/activity/sync_test.go` (add table test, add `time` import if missing)
- Modify: `datasync/activity/sync.go` (add helper function and `time` import — already imported)

- [ ] **Step 1: Write the failing table test**

Add this test to `datasync/activity/sync_test.go` (append after the existing tests):

```go
func TestHeartbeatInterval(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"zero falls back to 10s default", 0, 10 * time.Second},
		{"100ms hits 1s floor", 100 * time.Millisecond, 1 * time.Second},
		{"1s hits 1s floor", 1 * time.Second, 1 * time.Second},
		{"3s yields 1s (floor exact)", 3 * time.Second, 1 * time.Second},
		{"6s yields 2s", 6 * time.Second, 2 * time.Second},
		{"30s yields 10s (default-config case)", 30 * time.Second, 10 * time.Second},
		{"1m yields 20s", 1 * time.Minute, 20 * time.Second},
		{"5m yields 100s", 5 * time.Minute, 100 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := heartbeatInterval(tc.timeout)
			assert.Equal(t, tc.expected, got)
		})
	}
}
```

Add the `time` import to `sync_test.go` if it's not already present (check the current imports — the file currently has `context`, `fmt`, `testing` and may need `time` added).

- [ ] **Step 2: Run the test to verify it fails**

Run: `task test:run -- -run TestHeartbeatInterval ./datasync/activity/...`

Expected: compile error — `heartbeatInterval` undefined. This is the failing-test signal for an undefined function (compile failure counts as RED).

- [ ] **Step 3: Implement `heartbeatInterval`**

Add this function to `datasync/activity/sync.go`, placed after the `recordFailure` function at the end of the file:

```go
// heartbeatInterval derives the periodic heartbeat tick from the configured
// HeartbeatTimeout. Returns max(1s, hbTimeout/3); falls back to 10s when
// hbTimeout is 0 (no timeout configured).
func heartbeatInterval(hbTimeout time.Duration) time.Duration {
	if hbTimeout == 0 {
		return 10 * time.Second
	}
	interval := hbTimeout / 3
	if interval < 1*time.Second {
		return 1 * time.Second
	}
	return interval
}
```

No new imports needed — `time` is already imported.

- [ ] **Step 4: Run the test to verify it passes**

Run: `task test:run -- -run TestHeartbeatInterval ./datasync/activity/...`

Expected: PASS, all 8 sub-cases green.

- [ ] **Step 5: Commit**

```bash
git add datasync/activity/sync.go datasync/activity/sync_test.go
git commit -m "feat(datasync): add heartbeatInterval helper

Pure helper that derives the periodic heartbeat tick from the configured
HeartbeatTimeout. Returns max(1s, hbTimeout/3) with a 10s fallback when
HeartbeatTimeout is 0.

Will be used by the periodic-heartbeat goroutine in a follow-up commit."
```

---

## Task 2: Wire Heartbeat Goroutine Into `SyncData` (TDD)

This task implements the entire production change: the `heartbeatLoop` helper, the goroutine spawn, and all three `setPhase` calls. The slow-fetch test drives it; the slow-write test in Task 3 verifies the same wiring covers the write phase.

**Files:**
- Modify: `datasync/activity/sync_test.go` (add `sleep` field to `mockSource`, add slow-fetch test, add `strings`/`sync`/`time` imports as needed)
- Modify: `datasync/activity/sync.go` (add `heartbeatLoop`, modify `SyncData`, add `sync/atomic` import)

- [ ] **Step 1: Update `mockSource` to support a configurable sleep**

In `datasync/activity/sync_test.go`, replace the existing `mockSource` definition with:

```go
type mockSource[T any] struct {
	name    string
	records []T
	err     error
	sleep   time.Duration
}

func (m *mockSource[T]) Name() string { return m.name }
func (m *mockSource[T]) Fetch(ctx context.Context) ([]T, error) {
	if m.sleep > 0 {
		select {
		case <-time.After(m.sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.records, m.err
}
```

Note: existing tests construct `mockSource` with positional struct literal fields (`{name: "src", records: []string{"a"}}`). Adding `sleep` at the end as a zero-value field does not break those constructions because they all use named fields.

- [ ] **Step 2: Write the failing slow-fetch heartbeat test**

Add this test to `datasync/activity/sync_test.go`. You'll need to add three imports: `"strings"`, `"sync"`, `"time"`, and `"go.temporal.io/sdk/activity"`, and `"go.temporal.io/sdk/converter"`:

```go
func TestActivities_SyncData_HeartbeatsDuringSlowFetch(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	var (
		mu       sync.Mutex
		captured []string
	)
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, details converter.EncodedValues) {
		var payload string
		require.NoError(t, details.Get(&payload))
		mu.Lock()
		captured = append(captured, payload)
		mu.Unlock()
	})
	testEnv.SetTestTimeout(30 * time.Second)
	testEnv.SetWorkerOptions(worker.Options{
		MaxHeartbeatThrottleInterval: 500 * time.Millisecond,
	})

	source := &mockSource[string]{
		name:    "src",
		records: []string{"a"},
		sleep:   3 * time.Second,
	}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst", result: datasync.WriteResult{Inserted: 1}}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, captured, "expected at least one periodic heartbeat during slow Fetch")

	found := false
	for _, p := range captured {
		if strings.Contains(p, "fetching") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one heartbeat payload to contain 'fetching', got: %v", captured)
}
```

You'll also need to add `"go.temporal.io/sdk/worker"` to the imports for the `worker.Options` reference.

**Why `MaxHeartbeatThrottleInterval: 500ms`:** Temporal's default throttle is `0.8 × HeartbeatTimeout`. The `TestActivityEnvironment.ExecuteActivity` call doesn't honor `HeartbeatTimeout` from `ActivityOptions` (it's not a workflow context), so without overriding the worker option the throttle defaults to 30s — much longer than our 3s sleep, suppressing all heartbeats. Setting the worker's `MaxHeartbeatThrottleInterval` to 500ms caps the throttle at 500ms, making it shorter than our 1s tick interval so heartbeats reach the listener.

- [ ] **Step 3: Run the test to verify it fails**

Run: `task test:run -- -run TestActivities_SyncData_HeartbeatsDuringSlowFetch ./datasync/activity/...`

Expected: FAIL — `captured` will be empty (no periodic heartbeat goroutine exists yet). The two existing checkpoint heartbeats fire after Fetch and after Write, neither of which contains the word `"fetching"`.

- [ ] **Step 4: Add the `heartbeatLoop` helper**

Add this function to `datasync/activity/sync.go`, placed immediately above the `heartbeatInterval` function (added in Task 1):

```go
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
			p := "starting"
			if ptr := phase.Load(); ptr != nil {
				p = *ptr
			}
			activity.RecordHeartbeat(ctx, fmt.Sprintf("syncing %s: %s", jobName, p))
		}
	}
}
```

Add `"sync/atomic"` to the import block at the top of `sync.go`.

- [ ] **Step 5: Wire goroutine and `setPhase` calls into `SyncData`**

Edit the `SyncData` method in `datasync/activity/sync.go`. The current method starts at line 52. Add the heartbeat setup as the first executable statements inside the function, immediately before the existing `attrs := []attribute.KeyValue{...}` line, and add `setPhase` calls before each phase.

The current `SyncData` body begins:

```go
func (a *Activities[T, U]) SyncData(ctx context.Context, input ActivityInput) (*ActivityOutput, error) {
	attrs := []attribute.KeyValue{
		attribute.String("job", input.JobName),
		...
	}
	start := time.Now()
	...
```

Insert immediately after the opening brace (before `attrs := ...`):

```go
	var phase atomic.Pointer[string]
	setPhase := func(p string) { phase.Store(&p) }
	setPhase("starting")

	interval := heartbeatInterval(activity.GetInfo(ctx).HeartbeatTimeout)
	done := make(chan struct{})
	defer close(done)
	go heartbeatLoop(ctx, interval, input.JobName, &phase, done)

```

Then add `setPhase` calls at three points in the existing flow:

1. Immediately before `records, err := a.source.Fetch(fetchLC.Context())` (currently line 71):
   ```go
   	setPhase("fetching")
   	records, err := a.source.Fetch(fetchLC.Context())
   ```

2. Immediately before `mapped, err := a.mapper.Map(mapLC.Context(), records)` (currently line 96):
   ```go
   	setPhase("mapping")
   	mapped, err := a.mapper.Map(mapLC.Context(), records)
   ```

3. Immediately before `wr, err := a.sink.Write(writeLC.Context(), mapped)` (currently line 112):
   ```go
   	setPhase("writing")
   	wr, err := a.sink.Write(writeLC.Context(), mapped)
   ```

Do NOT remove the existing two checkpoint `activity.RecordHeartbeat` calls (currently lines 84 and 134) — they stay as informational checkpoints distinct from the periodic working-beats.

- [ ] **Step 6: Run the slow-fetch test to verify it passes**

Run: `task test:run -- -run TestActivities_SyncData_HeartbeatsDuringSlowFetch ./datasync/activity/...`

Expected: PASS. The test should take ~3 seconds to run (the sleep duration).

If it fails, check:
- `MaxHeartbeatThrottleInterval` was set on `SetWorkerOptions` (without it, throttle defaults swallow the periodic beats during a 3s window).
- The `setPhase("fetching")` line is BEFORE `a.source.Fetch(...)`, not after.
- The `go heartbeatLoop(...)` line is at the top of `SyncData`, not after the existing setup.

- [ ] **Step 7: Run the full activity-package tests to verify no regression**

Run: `task test:pkg -- ./datasync/activity/...`

Expected: all existing tests still PASS:
- `TestActivities_SyncData_Success`
- `TestActivities_SyncData_EmptySource`
- `TestActivities_SyncData_FetchError`
- `TestActivities_SyncData_WriteError`
- `TestToSyncExecutionOutput_Success`
- `TestToSyncExecutionOutput_Error`
- `TestHeartbeatInterval` (8 sub-cases)
- `TestActivities_SyncData_HeartbeatsDuringSlowFetch`

The existing tests don't set `HeartbeatTimeout`, so `activity.GetInfo(ctx).HeartbeatTimeout` returns 0, the goroutine ticks every 10s, and the activities complete in milliseconds — the goroutine never fires. No behavior change.

- [ ] **Step 8: Commit**

```bash
git add datasync/activity/sync.go datasync/activity/sync_test.go
git commit -m "fix(datasync): heartbeat continuously during SyncData phases

The activity previously only called RecordHeartbeat after Source.Fetch and
Sink.Write returned. Any phase exceeding HeartbeatTimeout (default 30s)
caused Temporal to fail the activity even when it was making forward
progress — observed in para-sync production with slow source DBs.

Spawn a long-lived goroutine for the activity lifetime, ticking at
max(1s, HeartbeatTimeout/3). The main goroutine updates a phase string
atomically (via sync/atomic.Pointer[string]) as it transitions through
fetch/map/write so the heartbeat payload reflects current activity
('syncing <jobName>: <phase>').

Existing checkpoint heartbeats ('fetched N records', 'wrote N records')
remain as informational markers between periodic beats.

Refs: docs/superpowers/specs/2026-05-04-datasync-periodic-heartbeat-design.md"
```

---

## Task 3: Add Slow-Write Coverage Test

Mirror of the slow-fetch test, but with `Sink.Write` doing the sleeping. Verifies the same goroutine and phase-tracking machinery covers the write phase. All production code is already in place from Task 2.

**Files:**
- Modify: `datasync/activity/sync_test.go` (add `sleep` field to `mockSink`, add slow-write test)

- [ ] **Step 1: Update `mockSink` to support a configurable sleep**

In `datasync/activity/sync_test.go`, replace the existing `mockSink` definition with:

```go
type mockSink[U any] struct {
	name   string
	result datasync.WriteResult
	err    error
	sleep  time.Duration
}

func (m *mockSink[U]) Name() string { return m.name }
func (m *mockSink[U]) Write(ctx context.Context, _ []U) (datasync.WriteResult, error) {
	if m.sleep > 0 {
		select {
		case <-time.After(m.sleep):
		case <-ctx.Done():
			return datasync.WriteResult{}, ctx.Err()
		}
	}
	return m.result, m.err
}
```

Note: the original `Write` signature uses `_ context.Context` — we change `_` to `ctx` to use it for cancellation. The `_ []U` stays as-is since we don't need to inspect records.

- [ ] **Step 2: Add the slow-write test**

Add this test to `datasync/activity/sync_test.go` (after `TestActivities_SyncData_HeartbeatsDuringSlowFetch`):

```go
func TestActivities_SyncData_HeartbeatsDuringSlowWrite(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	var (
		mu       sync.Mutex
		captured []string
	)
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, details converter.EncodedValues) {
		var payload string
		require.NoError(t, details.Get(&payload))
		mu.Lock()
		captured = append(captured, payload)
		mu.Unlock()
	})
	testEnv.SetTestTimeout(30 * time.Second)
	testEnv.SetWorkerOptions(worker.Options{
		MaxHeartbeatThrottleInterval: 500 * time.Millisecond,
	})

	source := &mockSource[string]{name: "src", records: []string{"a"}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{
		name:   "dst",
		result: datasync.WriteResult{Inserted: 1},
		sleep:  3 * time.Second,
	}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, captured, "expected at least one periodic heartbeat during slow Write")

	found := false
	for _, p := range captured {
		if strings.Contains(p, "writing") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one heartbeat payload to contain 'writing', got: %v", captured)
}
```

- [ ] **Step 3: Run the test to verify it passes**

Run: `task test:run -- -run TestActivities_SyncData_HeartbeatsDuringSlowWrite ./datasync/activity/...`

Expected: PASS. The implementation is already in place from Task 2; this test just confirms the same machinery covers the write phase.

The test takes ~3 seconds to run.

- [ ] **Step 4: Commit**

```bash
git add datasync/activity/sync_test.go
git commit -m "test(datasync): cover heartbeat during slow Sink.Write

Mirrors TestActivities_SyncData_HeartbeatsDuringSlowFetch but with the
sink doing the sleeping. Asserts at least one captured heartbeat payload
contains 'writing', confirming the periodic goroutine and phase tracking
cover the write phase as well as fetch."
```

---

## Task 4: Final Verification (Lint + Full Suite)

- [ ] **Step 1: Run the linter**

Run: `task lint`

Expected: clean (no warnings, no errors). If there are warnings about the new code (unused imports, shadow vars, etc.), fix them inline.

Common things to check if lint complains:
- `setPhase` is used as a closure — ensure no shadowing of the outer `phase` variable.
- `defer close(done)` placement — should be paired with `done := make(chan struct{})` immediately after.
- `atomic.Pointer[string]` zero value is fine — no need to initialize.

- [ ] **Step 2: Run the full datasync test suite**

Run: `task test:pkg -- ./datasync/...`

Expected: all PASS, including the new tests and the existing `datasync/workflow/` and `datasync/builder/` tests (which should be unaffected).

- [ ] **Step 3: Run the full project test suite**

Run: `task test`

Expected: all PASS. This runs unit + integration tests across all packages — catches anything we might have indirectly broken (e.g., examples that exercise datasync).

This may require a container engine. If `task test` reports a missing engine, fall back to `task ci:test` which skips integration.

- [ ] **Step 4: Verify the branch state and prepare for review**

Run: `git log --oneline feat/datasync-periodic-heartbeat ^main`

Expected: 4 commits (or 3 if Task 1 + Task 2 + Task 3 commits, plus the original docs commit from earlier):
1. `docs(datasync): add spec and design for periodic heartbeat fix`
2. `feat(datasync): add heartbeatInterval helper`
3. `fix(datasync): heartbeat continuously during SyncData phases`
4. `test(datasync): cover heartbeat during slow Sink.Write`

If `task lint` or `task test` flagged anything that needed a small fix, an additional commit may be present — that's fine.

- [ ] **Step 5: Ready for PR**

The branch is ready. Do NOT push or open a PR — leave that decision to the user. The implementation, tests, and verification are complete.

---

## Self-Review Notes

- **Spec coverage check:** Solution (goroutine + phase-aware payload) → Task 2. Helpers (`heartbeatInterval`, `heartbeatLoop`) → Tasks 1 & 2. All three test scenarios (`HeartbeatsDuringSlowFetch`, `HeartbeatsDuringSlowWrite`, `TestHeartbeatInterval`) → Tasks 1, 2, 3. Failure modes are documented in the spec; they're verified implicitly by Task 2's existing-tests-still-pass check (Step 7) and the lint/full-suite verification in Task 4.
- **Throttle reality vs spec:** The spec's test plan said "assert RecordHeartbeat called more than twice" — that assumption was wrong given Temporal's batching behavior. The plan corrects this: the assertion is "at least one heartbeat captured AND payload contains expected phase". The plan documents *why* in the "Critical Background" section so the implementer doesn't get tripped up.
- **Type consistency:** `heartbeatInterval(time.Duration) time.Duration`, `heartbeatLoop(ctx, interval, jobName, phase, done)`, `atomic.Pointer[string]`, `setPhase func(string)` — all consistent across tasks.
- **No placeholders:** Every step has actual code or actual commands.
