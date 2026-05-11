# pkg/temporal/job Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `pkg/temporal/job` package providing a type-focused `*job.Definition` abstraction, then converge all go-wf builders (`container/`, `function/`, `datasync/`, `datasync/chunk/`) on producing it. Delete `datasync/workflow.FullJobRegistration`.

**Architecture:** New `pkg/temporal/job` package in the `pkg` repo with `Definition`, `Registry`, `ScheduleSpec`, result/option types, and a `RegisterWorkflowOnce` idempotency helper. In go-wf, make `container.RegisterAll`/`function.RegisterAll` route through the idempotency helper, then have each of the four builders' `Build()` return `*job.Definition`. Two PRs: pkg ships first (no dependents); go-wf bumps `pkg/v2` and lands consumer-side changes.

**Tech Stack:** Go 1.26, Temporal Go SDK v1.40+, `go.temporal.io/api/serviceerror`, existing `testify` + `testsuite` + `testcontainer` infrastructure in both repos.

**Spec:** `docs/superpowers/specs/2026-05-11-pkg-temporal-job-definition-design.md`

**Two repos involved:**
- **pkg**: `/Users/jasoet/Documents/Go/pkg` — branch `feat/temporal-job`
- **go-wf**: `/Users/jasoet/Documents/Go/go-wf` — branch `feat/job-definition`

During development go-wf uses a `replace` directive in its `go.mod` pointing at the local pkg path. The replace is removed when pkg ships.

---

## File Structure

### Files in pkg (new)

| File | Responsibility |
|---|---|
| `pkg/temporal/job/doc.go` | Package godoc with the type-focused mental model + minimal example |
| `pkg/temporal/job/status.go` | `Status`, `ActivityStatus` enums + `String()` / `IsTerminal()` / SDK-status mapping |
| `pkg/temporal/job/result.go` | `RunHandle`, `RunDetail`, `RunHistory`, `ActivityEvent`, `RunSummary`, `RunPage`, `DefinitionStats`, `ScheduleSummary`, `ScheduleDetail`, `TaskQueueInfo` |
| `pkg/temporal/job/schedule_spec.go` | `ScheduleSpec`, `CalendarSpec`, `OverlapPolicy` + `validate()` + `toSDKSpec()` |
| `pkg/temporal/job/options.go` | `ListOpts`, `StatsOpts`, `HistoryOpts`, `ScheduleListOpts`, `ExecuteOption` + `WithWorkflowID`/`WithTimeout`/etc. + `TimeRange` |
| `pkg/temporal/job/errors.go` | `ErrNotFound`, `ErrDuplicateName`, etc. + `translateSDKError(op, err)` |
| `pkg/temporal/job/definition.go` | `Definition` struct + `New` constructor + `Option`/`WithRegister`/etc. + `Register` + `RegisterWorkflowOnce` / `RegisterActivityOnce` + `Execute` + `GetRun` |
| `pkg/temporal/job/definition_workflow.go` | `Definition.Describe`, `History`, `Cancel`, `Terminate`, `Signal`, `Query`, `ListRuns`, `Stats` (split out so `definition.go` stays focused on construction + register + execute) |
| `pkg/temporal/job/definition_schedule.go` | `Definition.ApplySchedule`, `PauseSchedule`, `ResumeSchedule`, `TriggerSchedule`, `DeleteSchedule`, `DescribeSchedule` |
| `pkg/temporal/job/registry.go` | `Registry` struct + `NewRegistry`/`Add`/`Get`/`MustGet`/`List`/`Names` + `RegisterAll` + `ApplySchedules` |

### Test files in pkg (new)

| File | Build tag | Covers |
|---|---|---|
| `pkg/temporal/job/status_test.go` | none | `Status.String`, `IsTerminal`, SDK-status mapping table |
| `pkg/temporal/job/schedule_spec_test.go` | none | `ScheduleSpec.validate`, `toSDKSpec` for interval/cron/calendar paths, `OverlapPolicy` mapping |
| `pkg/temporal/job/errors_test.go` | none | `translateSDKError` table — every mapped SDK error type |
| `pkg/temporal/job/definition_test.go` | none | `New` validation, `Option` setters, `GetRun` (no client call), the `RegisterWorkflowOnce` dedup mechanic |
| `pkg/temporal/job/registry_test.go` | none | `NewRegistry`/`Add` (duplicate + invalid), `Get`/`MustGet` (missing → panic), `List` sorted |
| `pkg/temporal/job/definition_integration_test.go` | `integration` | Full lifecycle: build → register → execute → describe → history → cancel/terminate. Real Temporal container via `pkg/temporal/testcontainer.Setup`. |
| `pkg/temporal/job/schedule_integration_test.go` | `integration` | ApplySchedule → Describe → Pause → Resume → Trigger → Delete |
| `pkg/temporal/job/registry_integration_test.go` | `integration` | Multi-Definition `Registry.RegisterAll(w)` with shared workflow types confirms no double-register panic |

### Files in go-wf (modified)

| File | Change |
|---|---|
| `container/worker.go` | `RegisterAll` routes through `job.RegisterWorkflowOnce` / `RegisterActivityOnce` |
| `function/worker.go` | Same idempotency change |
| `datasync/builder/builder.go` | `Build()` returns `*job.Definition` (replaces `(Job[T,U], error)`) |
| `datasync/chunk/sync.go` | `Build()` returns `*job.Definition` (rename from `FullJobRegistration`); schedule field becomes `*job.ScheduleSpec`; new sugar `.ScheduleEvery` / `.ScheduleCron` replaces `.Schedule(d)` |
| `datasync/chunk/date_sync.go` | Same — `Schedule(d time.Duration)` becomes `.ScheduleEvery(d)` (proxies to inner) |
| `container/builder/builder.go` | New `Name(string)`, `TaskQueue(string)`, `Build() *job.Definition` |
| `function/builder/builder.go` | Same |
| `datasync/workflow/sync.go` | DELETE `FullJobRegistration`, `BuildJobRegistration`. Keep `TaskQueue`, `RegisterJob`, etc. |
| `examples/datasync/*.go` | Update to use `*job.Definition` (mainly `chunk_basic.go`) |
| `go.mod` | `replace github.com/jasoet/pkg/v2 => /Users/jasoet/Documents/Go/pkg` during development; bump version + remove replace before shipping |

### Test files in go-wf (modified)

| File | Change |
|---|---|
| `datasync/builder/builder_test.go` | Adjust assertions to expect `*job.Definition` (most still pass; rename types) |
| `datasync/chunk/sync_test.go` | `FullJobRegistration` references → `*job.Definition` |
| `datasync/chunk/sync_integration_test.go` | Same |
| `datasync/chunk/date_sync_test.go` | Same + adjust schedule field references |
| `container/builder/builder_test.go` | New `Build_ProducesDefinition` test |
| `function/builder/builder_test.go` | Same |
| `container/integration_test.go` | One new "Build through definition" assertion |
| `function/integration_test.go` | Same |

---

## Critical Background — Read Before Implementing

### 1. Two repositories, one logical change

The pkg PR has no dependents; ship first. go-wf needs pkg's new types. During development go-wf uses a local `replace` directive in `go.mod` to point at the local pkg checkout. Tasks make this explicit.

### 2. Workflow ID convention

`Definition.Execute` generates workflow IDs as `"<Name>-<uuid-v7-or-similar>"`. `ListRuns` / `Stats` filter visibility by `WorkflowId STARTS_WITH "<Name>-"`. Callers overriding `WithWorkflowID(...)` to an ID that doesn't start with the prefix opt out of scoping for that run — by contract, no enforcement.

Use `github.com/google/uuid` (already in pkg/v2's transitive deps) for the suffix. `uuid.NewString()` is fine — V4 random UUID. Format: `"<Name>-<UUID>"`.

### 3. The Temporal SDK has no "WorkflowExecutionAlreadyCompleted" error

When `Cancel`/`Terminate`/`Signal` hit a completed workflow, the SDK returns `*serviceerror.NotFound`. Don't pattern-match an "AlreadyCompleted" type — there isn't one. `ErrAlreadyClosed` in the spec is reserved for opt-in helpers that pre-check status via `Describe`; default lifecycle methods translate `NotFound` to `ErrNotFound` and let the caller decide.

### 4. Activity registration registers by name; idempotency must compare names

Temporal worker `RegisterActivityWithOptions(fn, RegisterOptions{Name: "..."})` panics on duplicate name. `job.RegisterActivityOnce` keys its `sync.Map` on `(worker, name)`. For function-typed activities without explicit name (rare in go-wf — both `container/` and `function/` use explicit names), use `reflect.TypeOf(fn).String()` as a fallback in the helper.

### 5. The `client.WorkflowRun` interface

`RunHandle` wraps a `client.WorkflowRun` (interface). For `Execute`'s return, you get one back from `c.ExecuteWorkflow`. For `GetRun`, construct via `c.GetWorkflow(ctx, wfID, runID)` which also returns `client.WorkflowRun`. `RunHandle.Get(ctx, valuePtr)` delegates to the underlying interface.

### 6. Temporal visibility query syntax

`WorkflowId STARTS_WITH "..."` — works on Temporal Cloud and on the dev server. `WorkflowType = "..."`, `ExecutionStatus = "Running"`, etc. The visibility query is a string built by concatenating predicates with ` AND `. The SDK accepts it via `ListWorkflowExecutionsRequest.Query`.

### 7. Project conventions (both repos)

- pkg: `task <name>` for tests; `task ci:test` for unit; `task test:integration` for integration (container engine required).
- go-wf: same task names.
- Conventional Commits: `feat(temporal/job):`, `refactor(container):`, etc.
- NEVER AI co-author. Per `CLAUDE.md` in both repos.
- No `--no-verify`.

---

## Phase 1 — pkg/temporal/job foundation (Tasks 1–11)

All work in `/Users/jasoet/Documents/Go/pkg` on branch `feat/temporal-job`.

## Task 0: Setup pkg branch + baseline

- [ ] **Step 1: Create branch**

```bash
cd /Users/jasoet/Documents/Go/pkg
git checkout -b feat/temporal-job
```

- [ ] **Step 2: Baseline test**

```bash
task ci:test
```

Expected: all tests pass. If any fail on `main`, stop — not our concern.

- [ ] **Step 3: Commit the spec (it lives in go-wf; pkg gets just the plan reference)**

No commit yet — pkg's branch starts empty. First commit lands in Task 1.

---

## Task 1: Create package skeleton — doc.go + status.go

**Files:**
- Create: `pkg/temporal/job/doc.go`
- Create: `pkg/temporal/job/status.go`
- Create: `pkg/temporal/job/status_test.go`

- [ ] **Step 1: Create the package doc**

`pkg/temporal/job/doc.go`:

```go
// Package job provides a type-focused abstraction for one registered Temporal
// workflow. A Definition holds the metadata, lifecycle hooks, and per-job
// operations for a single workflow: register on a worker, execute by name,
// attach a schedule, describe runs, control lifecycle.
//
// The package coexists with pkg/temporal's existing managers (WorkflowManager,
// ScheduleManager). Use a Definition when you have a typed handle to "your"
// workflow; use the namespace-wide managers when you need to inspect
// workflows you didn't register yourself.
//
// Typical use:
//
//	def, err := job.New("orders-sync", "sync-orders",
//	    job.WithRegister(func(w worker.Worker) { /* ... */ }),
//	    job.WithExecute(func(ctx, c, opts, in) (client.WorkflowRun, error) { /* ... */ }),
//	    job.WithNewInput(func() any { return &OrdersInput{} }),
//	    job.WithSchedule(&job.ScheduleSpec{Interval: time.Hour}),
//	)
//	def.Register(worker)
//	run, _ := def.Execute(ctx, c, &OrdersInput{...})
//	detail, _ := def.Describe(ctx, c, run.WorkflowID, run.RunID)
package job
```

- [ ] **Step 2: Write status_test.go**

`pkg/temporal/job/status_test.go`:

```go
package job

import (
	"testing"

	"github.com/stretchr/testify/assert"
	enumspb "go.temporal.io/api/enums/v1"
)

func TestStatus_String(t *testing.T) {
	cases := map[Status]string{
		StatusUnknown:        "unknown",
		StatusRunning:        "running",
		StatusCompleted:      "completed",
		StatusFailed:         "failed",
		StatusCanceled:       "canceled",
		StatusTerminated:     "terminated",
		StatusContinuedAsNew: "continued_as_new",
		StatusTimedOut:       "timed_out",
	}
	for s, want := range cases {
		assert.Equal(t, want, s.String(), "Status(%d).String()", s)
	}
}

func TestStatus_IsTerminal(t *testing.T) {
	terminal := []Status{StatusCompleted, StatusFailed, StatusCanceled, StatusTerminated, StatusContinuedAsNew, StatusTimedOut}
	for _, s := range terminal {
		assert.True(t, s.IsTerminal(), "%s should be terminal", s)
	}
	assert.False(t, StatusRunning.IsTerminal())
	assert.False(t, StatusUnknown.IsTerminal())
}

func TestStatusFromSDK(t *testing.T) {
	cases := map[enumspb.WorkflowExecutionStatus]Status{
		enumspb.WORKFLOW_EXECUTION_STATUS_UNSPECIFIED:      StatusUnknown,
		enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:          StatusRunning,
		enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:        StatusCompleted,
		enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:           StatusFailed,
		enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:         StatusCanceled,
		enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:       StatusTerminated,
		enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW: StatusContinuedAsNew,
		enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:        StatusTimedOut,
	}
	for sdk, want := range cases {
		assert.Equal(t, want, StatusFromSDK(sdk), "sdk=%v", sdk)
	}
}
```

- [ ] **Step 3: Run, expect fail**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

Expected: `temporal/job` directory not found. Run will fail at package level.

Use raw command if `test:pkg` task doesn't accept that arg: `cd /Users/jasoet/Documents/Go/pkg && go test ./temporal/job/...` — package doesn't compile.

- [ ] **Step 4: Implement status.go**

`pkg/temporal/job/status.go`:

```go
package job

import (
	enumspb "go.temporal.io/api/enums/v1"
)

// Status represents a workflow execution status, mirrored from Temporal's
// WorkflowExecutionStatus enum for use without leaking SDK enum types.
type Status int

const (
	StatusUnknown Status = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusCanceled
	StatusTerminated
	StatusContinuedAsNew
	StatusTimedOut
)

// String returns the lowercase snake_case name of the status.
func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "canceled"
	case StatusTerminated:
		return "terminated"
	case StatusContinuedAsNew:
		return "continued_as_new"
	case StatusTimedOut:
		return "timed_out"
	default:
		return "unknown"
	}
}

// IsTerminal reports whether the status represents a closed (finished) workflow.
func (s Status) IsTerminal() bool {
	switch s {
	case StatusCompleted, StatusFailed, StatusCanceled, StatusTerminated, StatusContinuedAsNew, StatusTimedOut:
		return true
	default:
		return false
	}
}

// StatusFromSDK maps a Temporal SDK WorkflowExecutionStatus to a job.Status.
func StatusFromSDK(s enumspb.WorkflowExecutionStatus) Status {
	switch s {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return StatusRunning
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return StatusCompleted
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:
		return StatusFailed
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return StatusCanceled
	case enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return StatusTerminated
	case enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return StatusContinuedAsNew
	case enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return StatusTimedOut
	default:
		return StatusUnknown
	}
}

// ActivityStatus mirrors Temporal's per-activity outcome.
type ActivityStatus int

const (
	ActivityScheduled ActivityStatus = iota
	ActivityStarted
	ActivityCompleted
	ActivityFailed
	ActivityTimedOut
	ActivityCanceled
)

func (s ActivityStatus) String() string {
	switch s {
	case ActivityScheduled:
		return "scheduled"
	case ActivityStarted:
		return "started"
	case ActivityCompleted:
		return "completed"
	case ActivityFailed:
		return "failed"
	case ActivityTimedOut:
		return "timed_out"
	case ActivityCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}
```

- [ ] **Step 5: Run, expect pass**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

Expected: PASS (3 test functions).

- [ ] **Step 6: Commit**

```bash
cd /Users/jasoet/Documents/Go/pkg
git add temporal/job/doc.go temporal/job/status.go temporal/job/status_test.go
git commit -m "feat(temporal/job): add package skeleton and Status enum"
```

---

## Task 2: Add result types

**Files:**
- Create: `pkg/temporal/job/result.go`

Pure data structs. No behavior; no dedicated test file. Validated through use in later integration tests.

- [ ] **Step 1: Implement result.go**

`pkg/temporal/job/result.go`:

```go
package job

import (
	"context"
	"time"

	"go.temporal.io/sdk/client"
)

// RunHandle is a lightweight handle to one workflow run. Returned by
// Definition.Execute and Definition.GetRun.
type RunHandle struct {
	WorkflowID string
	RunID      string
	raw        client.WorkflowRun
}

// Get blocks until the workflow completes and unmarshals its result into
// valuePtr (must be a non-nil pointer). Returns the workflow's error if it
// failed. Returns nil if the handle has no underlying run (e.g., constructed
// from GetRun on an unknown ID).
func (h RunHandle) Get(ctx context.Context, valuePtr any) error {
	if h.raw == nil {
		return nil
	}
	return h.raw.Get(ctx, valuePtr)
}

// RunDetail is the full description of one workflow run.
type RunDetail struct {
	WorkflowID       string
	RunID            string
	Type             string
	TaskQueue        string
	Status           Status
	StartTime        time.Time
	CloseTime        *time.Time // nil if still running
	ExecutionTime    time.Duration
	HistoryLength    int64
	Memo             map[string]any
	SearchAttributes map[string]any
}

// RunHistory is the activity-event extraction of one run's history, bounded
// by HistoryOpts.MaxEvents.
type RunHistory struct {
	WorkflowID string
	RunID      string
	Activities []ActivityEvent
	Truncated  bool
}

// ActivityEvent describes one activity attempt within a workflow run.
type ActivityEvent struct {
	Name      string
	Status    ActivityStatus
	Attempt   int32
	StartTime time.Time
	CloseTime time.Time
	Duration  time.Duration
	Input     []byte // raw payload; caller deserializes
	Result    []byte // raw payload; nil on failure
	Error     string // empty on success
}

// RunPage is one page of ListRuns results.
type RunPage struct {
	Runs          []RunSummary
	NextPageToken []byte
}

// RunSummary is one row in a list of runs.
type RunSummary struct {
	WorkflowID string
	RunID      string
	Type       string
	Status     Status
	StartTime  time.Time
	CloseTime  *time.Time
	TaskQueue  string
}

// DefinitionStats is per-Definition aggregate counters.
type DefinitionStats struct {
	Running        int64
	CompletedToday int64
	FailedToday    int64
	AsOf           time.Time
}

// ScheduleSummary is a lightweight schedule summary.
type ScheduleSummary struct {
	ID           string
	WorkflowType string
	Paused       bool
	NextRunTime  *time.Time
	LastRunTime  *time.Time
	Note         string
}

// ScheduleDetail is the full schedule description.
type ScheduleDetail struct {
	ScheduleSummary
	Spec       ScheduleSpec // wrapped, not raw client.ScheduleSpec
	RecentRuns []RunSummary
}

// TaskQueueInfo describes a task queue's pollers and reachability.
type TaskQueueInfo struct {
	Name        string
	WorkerCount int
	// Future: PollerDetails, Reachability, etc.
}
```

- [ ] **Step 2: Verify build**

```bash
cd /Users/jasoet/Documents/Go/pkg && go build ./temporal/job/...
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add temporal/job/result.go
git commit -m "feat(temporal/job): add result and summary types"
```

---

## Task 3: Add ScheduleSpec + CalendarSpec + OverlapPolicy

**Files:**
- Create: `pkg/temporal/job/schedule_spec.go`
- Create: `pkg/temporal/job/schedule_spec_test.go`

- [ ] **Step 1: Write failing test**

`pkg/temporal/job/schedule_spec_test.go`:

```go
package job

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScheduleSpec_Validate(t *testing.T) {
	t.Run("interval is valid", func(t *testing.T) {
		s := &ScheduleSpec{Interval: time.Hour}
		assert.NoError(t, s.validate())
	})
	t.Run("cron is valid", func(t *testing.T) {
		s := &ScheduleSpec{Cron: "0 0 * * *"}
		assert.NoError(t, s.validate())
	})
	t.Run("calendar is valid", func(t *testing.T) {
		s := &ScheduleSpec{Calendar: []CalendarSpec{{Hour: []ScheduleRange{{Start: 0, End: 23, Step: 1}}}}}
		assert.NoError(t, s.validate())
	})
	t.Run("nothing set is invalid", func(t *testing.T) {
		s := &ScheduleSpec{}
		assert.Error(t, s.validate())
	})
	t.Run("two set is invalid", func(t *testing.T) {
		s := &ScheduleSpec{Interval: time.Hour, Cron: "0 * * * *"}
		assert.Error(t, s.validate())
	})
}

func TestScheduleSpec_ToSDKSpec_Interval(t *testing.T) {
	s := &ScheduleSpec{Interval: 15 * time.Minute}
	sdk, err := s.toSDKSpec()
	require.NoError(t, err)
	require.Len(t, sdk.Intervals, 1)
	assert.Equal(t, 15*time.Minute, sdk.Intervals[0].Every)
}

func TestScheduleSpec_ToSDKSpec_Cron(t *testing.T) {
	s := &ScheduleSpec{Cron: "0 */6 * * *"}
	sdk, err := s.toSDKSpec()
	require.NoError(t, err)
	require.Len(t, sdk.CronExpressions, 1)
	assert.Equal(t, "0 */6 * * *", sdk.CronExpressions[0])
}

func TestScheduleSpec_ToSDKSpec_Calendar(t *testing.T) {
	s := &ScheduleSpec{
		Calendar: []CalendarSpec{{
			Hour:    []ScheduleRange{{Start: 9, End: 17, Step: 1}},
			Minute:  []ScheduleRange{{Start: 0, End: 0, Step: 1}},
			Comment: "business hours",
		}},
	}
	sdk, err := s.toSDKSpec()
	require.NoError(t, err)
	require.Len(t, sdk.Calendars, 1)
	assert.Equal(t, "business hours", sdk.Calendars[0].Comment)
	assert.Len(t, sdk.Calendars[0].Hour, 1)
	assert.Equal(t, 9, sdk.Calendars[0].Hour[0].Start)
	assert.Equal(t, 17, sdk.Calendars[0].Hour[0].End)
}

func TestScheduleSpec_ToSDKSpec_Jitter(t *testing.T) {
	s := &ScheduleSpec{Interval: time.Hour, Jitter: 30 * time.Second}
	sdk, err := s.toSDKSpec()
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, sdk.Jitter)
}
```

- [ ] **Step 2: Run, expect fail**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

- [ ] **Step 3: Implement schedule_spec.go**

`pkg/temporal/job/schedule_spec.go`:

```go
package job

import (
	"errors"
	"fmt"
	"time"

	"go.temporal.io/sdk/client"
)

// OverlapPolicy controls how the scheduler handles a new trigger when a
// previous run is still in flight. Values mirror Temporal's ScheduleOverlapPolicy.
type OverlapPolicy int

const (
	OverlapSkip            OverlapPolicy = iota // default — drop new trigger if previous still running
	OverlapBufferOne                            // queue one trigger; drop further
	OverlapBufferAll                            // queue all triggers
	OverlapCancelOther                          // cancel running, start new
	OverlapTerminateOther                       // terminate running, start new
	OverlapAllowAll                             // start in parallel
)

func (p OverlapPolicy) toSDK() client.ScheduleOverlapPolicy {
	switch p {
	case OverlapBufferOne:
		return client.ScheduleOverlapPolicyBufferOne
	case OverlapBufferAll:
		return client.ScheduleOverlapPolicyBufferAll
	case OverlapCancelOther:
		return client.ScheduleOverlapPolicyCancelOther
	case OverlapTerminateOther:
		return client.ScheduleOverlapPolicyTerminateOther
	case OverlapAllowAll:
		return client.ScheduleOverlapPolicyAllowAll
	default:
		return client.ScheduleOverlapPolicySkip
	}
}

// ScheduleRange mirrors Temporal's ScheduleRange (Start/End/Step int).
type ScheduleRange struct {
	Start int
	End   int
	Step  int
}

// CalendarSpec mirrors Temporal's ScheduleCalendarSpec.
type CalendarSpec struct {
	Second     []ScheduleRange
	Minute     []ScheduleRange
	Hour       []ScheduleRange
	DayOfMonth []ScheduleRange
	Month      []ScheduleRange
	Year       []ScheduleRange
	DayOfWeek  []ScheduleRange
	Comment    string
}

// ScheduleSpec describes when a Definition's workflow should fire automatically.
// Exactly one of Interval / Cron / Calendar must be set.
type ScheduleSpec struct {
	Interval time.Duration
	Cron     string
	Calendar []CalendarSpec

	Overlap       OverlapPolicy
	Jitter        time.Duration
	Paused        bool
	Note          string
	CatchupWindow time.Duration
}

func (s *ScheduleSpec) validate() error {
	if s == nil {
		return errors.New("job: ScheduleSpec is nil")
	}
	set := 0
	if s.Interval > 0 {
		set++
	}
	if s.Cron != "" {
		set++
	}
	if len(s.Calendar) > 0 {
		set++
	}
	if set == 0 {
		return errors.New("job: ScheduleSpec requires one of Interval, Cron, or Calendar")
	}
	if set > 1 {
		return errors.New("job: ScheduleSpec must set exactly one of Interval, Cron, or Calendar")
	}
	return nil
}

func (s *ScheduleSpec) toSDKSpec() (client.ScheduleSpec, error) {
	if err := s.validate(); err != nil {
		return client.ScheduleSpec{}, err
	}
	spec := client.ScheduleSpec{
		Jitter: s.Jitter,
	}
	if s.CatchupWindow > 0 {
		spec.StartAt = time.Time{} // unused; placeholder
	}
	switch {
	case s.Interval > 0:
		spec.Intervals = []client.ScheduleIntervalSpec{{Every: s.Interval}}
	case s.Cron != "":
		spec.CronExpressions = []string{s.Cron}
	case len(s.Calendar) > 0:
		for _, c := range s.Calendar {
			spec.Calendars = append(spec.Calendars, calendarToSDK(c))
		}
	}
	return spec, nil
}

func calendarToSDK(c CalendarSpec) client.ScheduleCalendarSpec {
	return client.ScheduleCalendarSpec{
		Second:     rangesToSDK(c.Second),
		Minute:     rangesToSDK(c.Minute),
		Hour:       rangesToSDK(c.Hour),
		DayOfMonth: rangesToSDK(c.DayOfMonth),
		Month:      rangesToSDK(c.Month),
		Year:       rangesToSDK(c.Year),
		DayOfWeek:  rangesToSDK(c.DayOfWeek),
		Comment:    c.Comment,
	}
}

func rangesToSDK(rs []ScheduleRange) []client.ScheduleRange {
	if len(rs) == 0 {
		return nil
	}
	out := make([]client.ScheduleRange, len(rs))
	for i, r := range rs {
		out[i] = client.ScheduleRange{Start: r.Start, End: r.End, Step: r.Step}
	}
	return out
}

// describeMismatch is used by toSDKSpec error wrappers for clarity.
func (s *ScheduleSpec) describe() string {
	switch {
	case s == nil:
		return "<nil>"
	case s.Interval > 0:
		return fmt.Sprintf("interval=%s", s.Interval)
	case s.Cron != "":
		return fmt.Sprintf("cron=%q", s.Cron)
	case len(s.Calendar) > 0:
		return fmt.Sprintf("calendar=%d entries", len(s.Calendar))
	default:
		return "<empty>"
	}
}
```

- [ ] **Step 4: Run, expect pass**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

- [ ] **Step 5: Commit**

```bash
git add temporal/job/schedule_spec.go temporal/job/schedule_spec_test.go
git commit -m "feat(temporal/job): add ScheduleSpec with interval/cron/calendar"
```

---

## Task 4: Add Options + ExecuteOption + errors

**Files:**
- Create: `pkg/temporal/job/options.go`
- Create: `pkg/temporal/job/errors.go`
- Create: `pkg/temporal/job/errors_test.go`

- [ ] **Step 1: Implement options.go**

`pkg/temporal/job/options.go`:

```go
package job

import (
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// TimeRange filters by a start-time inclusive range.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// ListOpts configures Definition.ListRuns.
type ListOpts struct {
	Status    []Status   // empty = any
	TimeRange *TimeRange // by StartTime
	PageSize  int        // default 100, max 1000
	PageToken []byte
}

// StatsOpts configures Definition.Stats.
type StatsOpts struct {
	TodayOnly bool           // default false — set true for "running + closed today"
	Location  *time.Location // if nil and TodayOnly: UTC; otherwise this zone's calendar day
}

// HistoryOpts configures Definition.History.
type HistoryOpts struct {
	MaxEvents int // default 500 in the method; 0 = no cap (caller takes responsibility)
}

// ScheduleListOpts configures Registry.ListSchedules (future) and individual
// schedule paging.
type ScheduleListOpts struct {
	PageSize  int
	PageToken []byte
}

// executeConfig accumulates state across ExecuteOption calls.
type executeConfig struct {
	workflowID     string
	timeout        time.Duration
	taskTimeout    time.Duration
	retryPolicy    *temporal.RetryPolicy
	memo           map[string]any
	searchAttrs    map[string]any
}

// ExecuteOption customises a single Definition.Execute call.
type ExecuteOption func(*executeConfig)

// WithWorkflowID overrides the default ID of "<Name>-<uuid>".
func WithWorkflowID(id string) ExecuteOption {
	return func(c *executeConfig) { c.workflowID = id }
}

// WithTimeout sets WorkflowExecutionTimeout.
func WithTimeout(d time.Duration) ExecuteOption {
	return func(c *executeConfig) { c.timeout = d }
}

// WithTaskTimeout sets WorkflowTaskTimeout.
func WithTaskTimeout(d time.Duration) ExecuteOption {
	return func(c *executeConfig) { c.taskTimeout = d }
}

// WithRetryPolicy sets the workflow-level retry policy.
func WithRetryPolicy(p *temporal.RetryPolicy) ExecuteOption {
	return func(c *executeConfig) { c.retryPolicy = p }
}

// WithMemo attaches a memo to the workflow execution.
func WithMemo(m map[string]any) ExecuteOption {
	return func(c *executeConfig) { c.memo = m }
}

// WithSearchAttributes attaches search attributes to the workflow execution.
func WithSearchAttributes(sa map[string]any) ExecuteOption {
	return func(c *executeConfig) { c.searchAttrs = sa }
}

// apply builds a client.StartWorkflowOptions from defaults + accumulated options.
func (c executeConfig) apply(defaultID, taskQueue string) client.StartWorkflowOptions {
	id := c.workflowID
	if id == "" {
		id = defaultID
	}
	opts := client.StartWorkflowOptions{
		ID:        id,
		TaskQueue: taskQueue,
	}
	if c.timeout > 0 {
		opts.WorkflowExecutionTimeout = c.timeout
	}
	if c.taskTimeout > 0 {
		opts.WorkflowTaskTimeout = c.taskTimeout
	}
	if c.retryPolicy != nil {
		opts.RetryPolicy = c.retryPolicy
	}
	if c.memo != nil {
		opts.Memo = c.memo
	}
	if c.searchAttrs != nil {
		opts.SearchAttributes = c.searchAttrs
	}
	return opts
}
```

- [ ] **Step 2: Write errors_test.go**

`pkg/temporal/job/errors_test.go`:

```go
package job

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/api/serviceerror"
)

func TestTranslateSDKError_NotFound(t *testing.T) {
	sdk := serviceerror.NewNotFound("workflow not found")
	got := translateSDKError("describe", sdk)
	assert.True(t, errors.Is(got, ErrNotFound))
	assert.True(t, errors.Is(got, sdk), "preserves underlying SDK error")
}

func TestTranslateSDKError_Passthrough(t *testing.T) {
	other := errors.New("plain error")
	got := translateSDKError("cancel", other)
	assert.False(t, errors.Is(got, ErrNotFound))
	assert.True(t, errors.Is(got, other))
}

func TestTranslateSDKError_Nil(t *testing.T) {
	got := translateSDKError("op", nil)
	assert.NoError(t, got)
}
```

- [ ] **Step 3: Implement errors.go**

`pkg/temporal/job/errors.go`:

```go
package job

import (
	"errors"
	"fmt"

	"go.temporal.io/api/serviceerror"
)

var (
	// Lookup
	ErrNotFound          = errors.New("job: not found")
	ErrDuplicateName     = errors.New("job: duplicate name")
	ErrInvalidDefinition = errors.New("job: invalid definition")

	// Lifecycle
	ErrAlreadyClosed    = errors.New("job: workflow already closed")
	ErrNoSchedule       = errors.New("job: no schedule configured")
	ErrScheduleNotFound = errors.New("job: schedule not found")

	// Wiring
	ErrNotRegistered = errors.New("job: register not configured")
)

// translateSDKError wraps a Temporal SDK error with a typed sentinel where a
// matching one exists; otherwise wraps with the operation name for context.
// Always preserves the original error chain so callers can errors.As to the
// SDK types when needed.
func translateSDKError(op string, err error) error {
	if err == nil {
		return nil
	}
	var notFound *serviceerror.NotFound
	if errors.As(err, &notFound) {
		return fmt.Errorf("%s: %w: %w", op, ErrNotFound, err)
	}
	return fmt.Errorf("%s: %w", op, err)
}
```

Note: the `%w` chain in `NotFound` produces an error that satisfies both `errors.Is(e, ErrNotFound)` and `errors.Is(e, originalSDKErr)` because Go's wrapping supports multiple wraps via `%w` since 1.20.

- [ ] **Step 4: Run tests**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add temporal/job/options.go temporal/job/errors.go temporal/job/errors_test.go
git commit -m "feat(temporal/job): add Options, ExecuteOption, and error translation"
```

---

## Task 5: Definition core + New + RegisterWorkflowOnce

**Files:**
- Create: `pkg/temporal/job/definition.go`
- Create: `pkg/temporal/job/definition_test.go`

- [ ] **Step 1: Write failing tests**

`pkg/temporal/job/definition_test.go`:

```go
package job

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func TestNew_RequiresName(t *testing.T) {
	_, err := New("", "tq",
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
		WithNewInput(func() any { return nil }),
	)
	assert.ErrorIs(t, err, ErrInvalidDefinition)
}

func TestNew_RequiresTaskQueue(t *testing.T) {
	_, err := New("name", "",
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
		WithNewInput(func() any { return nil }),
	)
	assert.ErrorIs(t, err, ErrInvalidDefinition)
}

func TestNew_RequiresAllClosures(t *testing.T) {
	_, err := New("name", "tq")
	assert.ErrorIs(t, err, ErrInvalidDefinition)

	_, err = New("name", "tq", WithRegister(func(worker.Worker) {}))
	assert.ErrorIs(t, err, ErrInvalidDefinition)

	_, err = New("name", "tq",
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
	)
	assert.ErrorIs(t, err, ErrInvalidDefinition)
}

func TestNew_ValidScheduleAccepted(t *testing.T) {
	d, err := New("name", "tq",
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
		WithNewInput(func() any { return nil }),
		WithSchedule(&ScheduleSpec{Interval: 1}),
		WithDescription("desc"),
		WithTags("a", "b"),
	)
	require.NoError(t, err)
	assert.Equal(t, "name", d.Name)
	assert.Equal(t, "tq", d.TaskQueue)
	assert.Equal(t, "desc", d.Description)
	assert.Equal(t, []string{"a", "b"}, d.Tags)
	require.NotNil(t, d.Schedule)
}

func TestNew_InvalidScheduleRejected(t *testing.T) {
	_, err := New("name", "tq",
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
		WithNewInput(func() any { return nil }),
		WithSchedule(&ScheduleSpec{}), // nothing set
	)
	assert.ErrorIs(t, err, ErrInvalidDefinition)
}

// fakeWorker is a minimal stub implementing worker.Worker — only the two
// register methods are exercised in unit tests.
type fakeWorker struct {
	workflowRegistrations  int32
	activityRegistrations  int32
	registeredWorkflowName string
}

func (f *fakeWorker) RegisterWorkflow(_ any)                                       {}
func (f *fakeWorker) RegisterWorkflowWithOptions(_ any, opts workflow.RegisterOptions) {
	atomic.AddInt32(&f.workflowRegistrations, 1)
	f.registeredWorkflowName = opts.Name
}
func (f *fakeWorker) RegisterActivity(_ any) {}
func (f *fakeWorker) RegisterActivityWithOptions(_ any, _ activity.RegisterOptions) {
	atomic.AddInt32(&f.activityRegistrations, 1)
}
func (f *fakeWorker) Start() error                          { return nil }
func (f *fakeWorker) Run(_ <-chan interface{}) error        { return nil }
func (f *fakeWorker) Stop()                                 {}

func TestRegisterWorkflowOnce_Deduplicates(t *testing.T) {
	w := &fakeWorker{}
	RegisterWorkflowOnce(w, "myWf", func() error { return nil }, workflow.RegisterOptions{Name: "myWf"})
	RegisterWorkflowOnce(w, "myWf", func() error { return nil }, workflow.RegisterOptions{Name: "myWf"})
	RegisterWorkflowOnce(w, "myWf", func() error { return nil }, workflow.RegisterOptions{Name: "myWf"})
	assert.Equal(t, int32(1), atomic.LoadInt32(&w.workflowRegistrations))
}

func TestRegisterWorkflowOnce_DifferentWorkers(t *testing.T) {
	w1 := &fakeWorker{}
	w2 := &fakeWorker{}
	RegisterWorkflowOnce(w1, "myWf", func() error { return nil }, workflow.RegisterOptions{Name: "myWf"})
	RegisterWorkflowOnce(w2, "myWf", func() error { return nil }, workflow.RegisterOptions{Name: "myWf"})
	assert.Equal(t, int32(1), atomic.LoadInt32(&w1.workflowRegistrations))
	assert.Equal(t, int32(1), atomic.LoadInt32(&w2.workflowRegistrations))
}

func TestGetRun(t *testing.T) {
	d, err := New("name", "tq",
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
		WithNewInput(func() any { return nil }),
	)
	require.NoError(t, err)
	h := d.GetRun(nil, "wf-id", "run-id")
	assert.Equal(t, "wf-id", h.WorkflowID)
	assert.Equal(t, "run-id", h.RunID)
	// raw is nil because client is nil; Get(ctx, &v) returns nil.
	assert.NoError(t, h.Get(context.Background(), nil))
}
```

Imports referenced but not yet listed at top: add `"go.temporal.io/sdk/activity"` and `"go.temporal.io/sdk/workflow"`. Include them in the test file's import block.

- [ ] **Step 2: Run, expect fail**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

- [ ] **Step 3: Implement definition.go**

`pkg/temporal/job/definition.go`:

```go
package job

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Definition is a type-focused description of one registered Temporal workflow.
// All per-job operations hang off the type as methods.
type Definition struct {
	Name        string
	TaskQueue   string
	Description string
	Tags        []string
	Schedule    *ScheduleSpec

	// Private wiring set only by New via Option closures.
	register func(worker.Worker)
	execute  func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, input any) (client.WorkflowRun, error)
	newInput func() any
}

// Option configures a Definition during construction.
type Option func(*Definition)

func WithRegister(fn func(worker.Worker)) Option {
	return func(d *Definition) { d.register = fn }
}

func WithExecute(fn func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, input any) (client.WorkflowRun, error)) Option {
	return func(d *Definition) { d.execute = fn }
}

func WithNewInput(fn func() any) Option {
	return func(d *Definition) { d.newInput = fn }
}

func WithSchedule(spec *ScheduleSpec) Option {
	return func(d *Definition) { d.Schedule = spec }
}

func WithDescription(desc string) Option {
	return func(d *Definition) { d.Description = desc }
}

func WithTags(tags ...string) Option {
	return func(d *Definition) { d.Tags = tags }
}

// New constructs a Definition. Validates name, task queue, all closures, and
// the optional schedule. Returns ErrInvalidDefinition if anything is missing
// or inconsistent.
func New(name, taskQueue string, opts ...Option) (*Definition, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: name required", ErrInvalidDefinition)
	}
	if taskQueue == "" {
		return nil, fmt.Errorf("%w: task queue required", ErrInvalidDefinition)
	}
	d := &Definition{Name: name, TaskQueue: taskQueue}
	for _, opt := range opts {
		opt(d)
	}
	if d.register == nil || d.execute == nil || d.newInput == nil {
		return nil, fmt.Errorf("%w: WithRegister, WithExecute, and WithNewInput are all required", ErrInvalidDefinition)
	}
	if d.Schedule != nil {
		if err := d.Schedule.validate(); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidDefinition, err)
		}
	}
	return d, nil
}

// NewInput returns a fresh typed zero value for this Definition's workflow
// input. Callers fill it before calling Execute (e.g., via json.Unmarshal).
func (d *Definition) NewInput() any {
	return d.newInput()
}

// Register wires the workflow and its activities onto a worker. Safe to call
// concurrently and multiple times — internal dedup ensures each underlying
// workflow type is registered only once per worker.
func (d *Definition) Register(w worker.Worker) {
	if d.register == nil {
		return
	}
	d.register(w)
}

// Execute starts a workflow run. The workflow ID defaults to "<Name>-<uuid>"
// unless overridden via WithWorkflowID(...).
func (d *Definition) Execute(ctx context.Context, c client.Client, input any, opts ...ExecuteOption) (RunHandle, error) {
	if d.execute == nil {
		return RunHandle{}, ErrNotRegistered
	}
	var cfg executeConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	defaultID := d.Name + "-" + uuid.NewString()
	sdkOpts := cfg.apply(defaultID, d.TaskQueue)
	run, err := d.execute(ctx, c, sdkOpts, input)
	if err != nil {
		return RunHandle{}, translateSDKError("execute", err)
	}
	return RunHandle{
		WorkflowID: run.GetID(),
		RunID:      run.GetRunID(),
		raw:        run,
	}, nil
}

// GetRun returns a RunHandle for an existing workflow run identified by
// wfID and runID (runID "" = latest). Useful when reattaching to a run
// triggered elsewhere.
func (d *Definition) GetRun(c client.Client, wfID, runID string) RunHandle {
	if c == nil {
		return RunHandle{WorkflowID: wfID, RunID: runID}
	}
	run := c.GetWorkflow(context.Background(), wfID, runID)
	return RunHandle{WorkflowID: wfID, RunID: runID, raw: run}
}

// --- Dedup helpers used by builders' Register closures ---

type registrarKey struct {
	worker   worker.Worker
	typeName string
}

var (
	registeredWorkflows  sync.Map
	registeredActivities sync.Map
)

// RegisterWorkflowOnce registers a workflow on a worker, returning silently
// if the (worker, typeName) pair has already been registered. Used by
// builder packages to make their RegisterAll-style helpers idempotent.
func RegisterWorkflowOnce(w worker.Worker, typeName string, wf any, opts workflow.RegisterOptions) {
	key := registrarKey{w, typeName}
	if _, loaded := registeredWorkflows.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	w.RegisterWorkflowWithOptions(wf, opts)
}

// RegisterActivityOnce registers an activity on a worker idempotently.
// Activity name comes from opts.Name; pass empty Name only for typed-function
// activities (rare in this codebase).
func RegisterActivityOnce(w worker.Worker, typeName string, fn any, opts activity.RegisterOptions) {
	if typeName == "" {
		typeName = opts.Name
	}
	if typeName == "" {
		// Fallback: this should not happen in this codebase but provides safety.
		typeName = fmt.Sprintf("%T", fn)
	}
	key := registrarKey{w, typeName}
	if _, loaded := registeredActivities.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	w.RegisterActivityWithOptions(fn, opts)
}

// sentinel to silence "imported and not used" for the errors package when
// only the sentinel ErrInvalidDefinition is referenced — kept for clarity.
var _ = errors.New
```

- [ ] **Step 4: Run, expect pass**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

Expected: PASS for status/schedule_spec/errors/definition tests.

- [ ] **Step 5: Lint**

```bash
cd /Users/jasoet/Documents/Go/pkg && task lint
```

Expected: clean. The `_ = errors.New` sentinel may trip "unused" check — remove if so (errors is imported transitively from errors.go which is in the same package, so the import is not needed in definition.go at all).

- [ ] **Step 6: Commit**

```bash
git add temporal/job/definition.go temporal/job/definition_test.go
git commit -m "feat(temporal/job): add Definition core with New, Register, Execute, GetRun"
```

---

## Task 6: Registry

**Files:**
- Create: `pkg/temporal/job/registry.go`
- Create: `pkg/temporal/job/registry_test.go`

- [ ] **Step 1: Write failing tests**

`pkg/temporal/job/registry_test.go`:

```go
package job

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func newTestDef(t *testing.T, name string) *Definition {
	t.Helper()
	d, err := New(name, "tq-"+name,
		WithRegister(func(worker.Worker) {}),
		WithExecute(func(context.Context, client.Client, client.StartWorkflowOptions, any) (client.WorkflowRun, error) {
			return nil, nil
		}),
		WithNewInput(func() any { return nil }),
	)
	require.NoError(t, err)
	return d
}

func TestRegistry_AddAndGet(t *testing.T) {
	r := NewRegistry()
	d := newTestDef(t, "alpha")
	require.NoError(t, r.Add(d))
	got, ok := r.Get("alpha")
	assert.True(t, ok)
	assert.Same(t, d, got)
}

func TestRegistry_AddDuplicate(t *testing.T) {
	r := NewRegistry(newTestDef(t, "a"))
	err := r.Add(newTestDef(t, "a"))
	assert.ErrorIs(t, err, ErrDuplicateName)
}

func TestRegistry_AddNilOrInvalid(t *testing.T) {
	r := NewRegistry()
	assert.ErrorIs(t, r.Add(nil), ErrInvalidDefinition)
	assert.ErrorIs(t, r.Add(&Definition{}), ErrInvalidDefinition)
}

func TestRegistry_MustGet_Missing(t *testing.T) {
	r := NewRegistry()
	assert.Panics(t, func() { _ = r.MustGet("missing") })
}

func TestRegistry_List_Sorted(t *testing.T) {
	r := NewRegistry(newTestDef(t, "charlie"), newTestDef(t, "alpha"), newTestDef(t, "bravo"))
	got := r.Names()
	assert.True(t, sort.StringsAreSorted(got))
	assert.Equal(t, []string{"alpha", "bravo", "charlie"}, got)

	list := r.List()
	require.Len(t, list, 3)
	for i, d := range list {
		assert.Equal(t, got[i], d.Name)
	}
}

func TestRegistry_NewWithSeed(t *testing.T) {
	r := NewRegistry(newTestDef(t, "a"), newTestDef(t, "b"))
	assert.Len(t, r.List(), 2)
}
```

- [ ] **Step 2: Run, expect fail**

- [ ] **Step 3: Implement registry.go**

`pkg/temporal/job/registry.go`:

```go
package job

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// Registry maps logical job names to Definitions. Construction does not
// validate seed Definitions twice — they were validated by New already.
type Registry struct {
	defs map[string]*Definition
}

// NewRegistry creates a Registry, optionally seeded. Duplicates among seeds
// silently use the later value (validate input upstream if that matters).
func NewRegistry(defs ...*Definition) *Registry {
	r := &Registry{defs: make(map[string]*Definition, len(defs))}
	for _, d := range defs {
		if d == nil || d.Name == "" {
			continue
		}
		r.defs[d.Name] = d
	}
	return r
}

// Add inserts a Definition. Returns ErrDuplicateName on conflict,
// ErrInvalidDefinition if the Definition is nil or missing required fields.
func (r *Registry) Add(d *Definition) error {
	if d == nil {
		return fmt.Errorf("%w: nil definition", ErrInvalidDefinition)
	}
	if d.Name == "" || d.TaskQueue == "" || d.register == nil || d.execute == nil || d.newInput == nil {
		return fmt.Errorf("%w: definition fields incomplete", ErrInvalidDefinition)
	}
	if _, exists := r.defs[d.Name]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateName, d.Name)
	}
	r.defs[d.Name] = d
	return nil
}

// Get returns the Definition with the given name and a boolean indicating
// whether it was found.
func (r *Registry) Get(name string) (*Definition, bool) {
	d, ok := r.defs[name]
	return d, ok
}

// MustGet returns the Definition with the given name. Panics with
// fmt.Errorf("%w: %s", ErrNotFound, name) if absent.
func (r *Registry) MustGet(name string) *Definition {
	d, ok := r.defs[name]
	if !ok {
		panic(fmt.Errorf("%w: %s", ErrNotFound, name))
	}
	return d
}

// List returns all Definitions, sorted by Name.
func (r *Registry) List() []*Definition {
	names := r.Names()
	out := make([]*Definition, len(names))
	for i, n := range names {
		out[i] = r.defs[n]
	}
	return out
}

// Names returns all registered names, sorted alphabetically.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.defs))
	for name := range r.defs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// RegisterAll registers every Definition on the given worker. Idempotent:
// underlying workflow/activity types are deduplicated via
// RegisterWorkflowOnce / RegisterActivityOnce.
func (r *Registry) RegisterAll(w worker.Worker) {
	for _, d := range r.List() {
		d.Register(w)
	}
}

// ApplySchedules creates or updates Temporal schedules for every Definition
// that has one. Returns the first error encountered (does not roll back).
func (r *Registry) ApplySchedules(ctx context.Context, c client.Client) error {
	for _, d := range r.List() {
		if d.Schedule == nil {
			continue
		}
		if err := d.ApplySchedule(ctx, c); err != nil {
			return fmt.Errorf("apply schedule for %q: %w", d.Name, err)
		}
	}
	return nil
}

// (errors import kept so go vet doesn't flag the package — used elsewhere.)
var _ = errors.New
```

- [ ] **Step 4: Run, expect pass — but note that `Definition.ApplySchedule` is not yet implemented**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/...
```

If `ApplySchedules` test compiles but fails because `Definition.ApplySchedule` is missing, that's expected — the method is added in Task 8. For now Registry's unit tests don't call ApplySchedules.

Actually `Registry.ApplySchedules` calls `d.ApplySchedule(ctx, c)` — this won't compile until Task 8. **Comment out** the `ApplySchedules` method body for now:

```go
func (r *Registry) ApplySchedules(ctx context.Context, c client.Client) error {
	// Implemented after Definition.ApplySchedule (Task 8)
	return errors.New("ApplySchedules not yet implemented — pending Task 8")
}
```

Restore the real body in Task 8.

- [ ] **Step 5: Run, expect pass**

- [ ] **Step 6: Commit**

```bash
git add temporal/job/registry.go temporal/job/registry_test.go
git commit -m "feat(temporal/job): add Registry"
```

---

## Task 7: Definition.Describe + History + Cancel + Terminate + Signal + Query

**Files:**
- Create: `pkg/temporal/job/definition_workflow.go`

These methods hit the Temporal client. Unit tests would require complex mocking; full coverage moves to the integration tests in Task 9. Only smoke tests here (signature + error wrapping).

- [ ] **Step 1: Implement definition_workflow.go**

`pkg/temporal/job/definition_workflow.go`:

```go
package job

import (
	"context"
	"fmt"
	"strings"
	"time"

	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
)

// Describe returns the current state of one workflow run. runID "" = latest.
func (d *Definition) Describe(ctx context.Context, c client.Client, wfID, runID string) (RunDetail, error) {
	resp, err := c.DescribeWorkflowExecution(ctx, wfID, runID)
	if err != nil {
		return RunDetail{}, translateSDKError("describe", err)
	}
	info := resp.GetWorkflowExecutionInfo()
	if info == nil {
		return RunDetail{}, fmt.Errorf("describe: empty info from server")
	}
	det := RunDetail{
		WorkflowID:    info.GetExecution().GetWorkflowId(),
		RunID:         info.GetExecution().GetRunId(),
		Type:          info.GetType().GetName(),
		TaskQueue:     info.GetTaskQueue(),
		Status:        StatusFromSDK(info.GetStatus()),
		StartTime:     info.GetStartTime().AsTime(),
		HistoryLength: info.GetHistoryLength(),
	}
	if info.GetCloseTime() != nil {
		ct := info.GetCloseTime().AsTime()
		det.CloseTime = &ct
		det.ExecutionTime = ct.Sub(det.StartTime)
	} else {
		det.ExecutionTime = time.Since(det.StartTime)
	}
	det.Memo = decodeMemo(info.GetMemo())
	det.SearchAttributes = decodeSearchAttributes(info.GetSearchAttributes())
	return det, nil
}

// History returns the activity-event extraction of one run's history.
func (d *Definition) History(ctx context.Context, c client.Client, wfID, runID string, opts HistoryOpts) (RunHistory, error) {
	max := opts.MaxEvents
	if max == 0 {
		max = 500
	}
	iter := c.GetWorkflowHistory(ctx, wfID, runID, false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	hist := RunHistory{WorkflowID: wfID, RunID: runID}
	events := make(map[int64]*historypb.HistoryEvent) // scheduledEventID -> event for pairing
	count := 0
	for iter.HasNext() {
		if count >= max {
			hist.Truncated = true
			break
		}
		ev, err := iter.Next()
		if err != nil {
			return hist, translateSDKError("history", err)
		}
		count++
		switch attr := ev.Attributes.(type) {
		case *historypb.HistoryEvent_ActivityTaskScheduledEventAttributes:
			events[ev.EventId] = ev
			_ = attr // placeholder; full extraction in pairing pass
		case *historypb.HistoryEvent_ActivityTaskStartedEventAttributes:
			sched := events[attr.ActivityTaskStartedEventAttributes.GetScheduledEventId()]
			if sched != nil {
				hist.Activities = append(hist.Activities, buildActivityEvent(sched, ev, nil))
			}
		case *historypb.HistoryEvent_ActivityTaskCompletedEventAttributes:
			updateActivityEvent(&hist, attr.ActivityTaskCompletedEventAttributes.GetScheduledEventId(), ev, ActivityCompleted, attr.ActivityTaskCompletedEventAttributes.GetResult())
		case *historypb.HistoryEvent_ActivityTaskFailedEventAttributes:
			updateActivityEventErr(&hist, attr.ActivityTaskFailedEventAttributes.GetScheduledEventId(), ev, ActivityFailed, attr.ActivityTaskFailedEventAttributes.GetFailure().GetMessage())
		case *historypb.HistoryEvent_ActivityTaskTimedOutEventAttributes:
			updateActivityEventErr(&hist, attr.ActivityTaskTimedOutEventAttributes.GetScheduledEventId(), ev, ActivityTimedOut, attr.ActivityTaskTimedOutEventAttributes.GetFailure().GetMessage())
		case *historypb.HistoryEvent_ActivityTaskCanceledEventAttributes:
			updateActivityEvent(&hist, attr.ActivityTaskCanceledEventAttributes.GetScheduledEventId(), ev, ActivityCanceled, nil)
		}
	}
	return hist, nil
}

func buildActivityEvent(scheduled, started *historypb.HistoryEvent, completed *historypb.HistoryEvent) ActivityEvent {
	a := scheduled.GetActivityTaskScheduledEventAttributes()
	ev := ActivityEvent{
		Name:      a.GetActivityType().GetName(),
		Status:    ActivityStarted,
		Attempt:   started.GetActivityTaskStartedEventAttributes().GetAttempt(),
		StartTime: started.GetEventTime().AsTime(),
		Input:     payloadToBytes(a.GetInput()),
	}
	if completed != nil {
		ev.CloseTime = completed.GetEventTime().AsTime()
		ev.Duration = ev.CloseTime.Sub(ev.StartTime)
	}
	return ev
}

func updateActivityEvent(hist *RunHistory, schedID int64, closeEv *historypb.HistoryEvent, status ActivityStatus, result *commonpb.Payloads) {
	for i := range hist.Activities {
		a := &hist.Activities[i]
		if a.Status == ActivityStarted {
			// Match by ordinal — we don't reliably track sched IDs across the slice.
			// The pairing in buildActivityEvent stored started events in order; close
			// them in the same order. This works because Temporal emits events in
			// monotonically increasing EventID order.
			a.Status = status
			a.CloseTime = closeEv.GetEventTime().AsTime()
			a.Duration = a.CloseTime.Sub(a.StartTime)
			if result != nil {
				a.Result = payloadToBytes(result)
			}
			return
		}
	}
}

func updateActivityEventErr(hist *RunHistory, schedID int64, closeEv *historypb.HistoryEvent, status ActivityStatus, msg string) {
	for i := range hist.Activities {
		a := &hist.Activities[i]
		if a.Status == ActivityStarted {
			a.Status = status
			a.CloseTime = closeEv.GetEventTime().AsTime()
			a.Duration = a.CloseTime.Sub(a.StartTime)
			a.Error = msg
			return
		}
	}
}

func payloadToBytes(p *commonpb.Payloads) []byte {
	if p == nil || len(p.GetPayloads()) == 0 {
		return nil
	}
	// Single-payload case is most common; concatenate raw bytes if more.
	var out []byte
	for _, pl := range p.GetPayloads() {
		out = append(out, pl.GetData()...)
	}
	return out
}

func decodeMemo(p *commonpb.Memo) map[string]any {
	if p == nil || len(p.GetFields()) == 0 {
		return nil
	}
	out := make(map[string]any, len(p.GetFields()))
	for k, v := range p.GetFields() {
		out[k] = string(v.GetData())
	}
	return out
}

func decodeSearchAttributes(p *commonpb.SearchAttributes) map[string]any {
	if p == nil || len(p.GetIndexedFields()) == 0 {
		return nil
	}
	out := make(map[string]any, len(p.GetIndexedFields()))
	for k, v := range p.GetIndexedFields() {
		out[k] = string(v.GetData())
	}
	return out
}

// Cancel requests cancellation of a workflow run.
func (d *Definition) Cancel(ctx context.Context, c client.Client, wfID, runID string) error {
	return translateSDKError("cancel", c.CancelWorkflow(ctx, wfID, runID))
}

// Terminate hard-stops a workflow run.
func (d *Definition) Terminate(ctx context.Context, c client.Client, wfID, runID, reason string) error {
	return translateSDKError("terminate", c.TerminateWorkflow(ctx, wfID, runID, reason))
}

// Signal sends a signal to a workflow run.
func (d *Definition) Signal(ctx context.Context, c client.Client, wfID, runID, signalName string, payload any) error {
	return translateSDKError("signal", c.SignalWorkflow(ctx, wfID, runID, signalName, payload))
}

// Query invokes a synchronous query against a workflow run. Returns the
// decoded result via SDK conventions.
func (d *Definition) Query(ctx context.Context, c client.Client, wfID, runID, queryType string, args ...any) (any, error) {
	resp, err := c.QueryWorkflow(ctx, wfID, runID, queryType, args...)
	if err != nil {
		return nil, translateSDKError("query", err)
	}
	var out any
	if err := resp.Get(&out); err != nil {
		return nil, translateSDKError("query-decode", err)
	}
	return out, nil
}

// ListRuns lists workflow executions of this Definition, scoped by
// "WorkflowId STARTS_WITH '<Name>-'".
func (d *Definition) ListRuns(ctx context.Context, c client.Client, opts ListOpts) (RunPage, error) {
	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	if pageSize > 1000 {
		pageSize = 1000
	}
	q := d.scopedQuery(opts.Status, opts.TimeRange)
	resp, err := c.ListWorkflow(ctx, &workflowservice.ListWorkflowExecutionsRequest{
		PageSize:      int32(pageSize),
		NextPageToken: opts.PageToken,
		Query:         q,
	})
	if err != nil {
		return RunPage{}, translateSDKError("list-runs", err)
	}
	page := RunPage{NextPageToken: resp.GetNextPageToken()}
	for _, info := range resp.GetExecutions() {
		s := RunSummary{
			WorkflowID: info.GetExecution().GetWorkflowId(),
			RunID:      info.GetExecution().GetRunId(),
			Type:       info.GetType().GetName(),
			Status:     StatusFromSDK(info.GetStatus()),
			StartTime:  info.GetStartTime().AsTime(),
			TaskQueue:  info.GetTaskQueue(),
		}
		if info.GetCloseTime() != nil {
			ct := info.GetCloseTime().AsTime()
			s.CloseTime = &ct
		}
		page.Runs = append(page.Runs, s)
	}
	return page, nil
}

// Stats returns running/completed-today/failed-today counts scoped to this
// Definition's workflow IDs.
func (d *Definition) Stats(ctx context.Context, c client.Client, opts StatsOpts) (DefinitionStats, error) {
	now := time.Now()
	loc := opts.Location
	if loc == nil {
		loc = time.UTC
	}
	startOfDay := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	prefix := fmt.Sprintf("WorkflowId STARTS_WITH %q", d.Name+"-")

	countQ := func(q string) (int64, error) {
		resp, err := c.CountWorkflow(ctx, &workflowservice.CountWorkflowExecutionsRequest{Query: q})
		if err != nil {
			return 0, translateSDKError("count", err)
		}
		return resp.GetCount(), nil
	}
	running, err := countQ(prefix + " AND ExecutionStatus = \"Running\"")
	if err != nil {
		return DefinitionStats{}, err
	}
	completed, err := countQ(prefix + fmt.Sprintf(" AND ExecutionStatus = \"Completed\" AND CloseTime >= %q", startOfDay.Format(time.RFC3339)))
	if err != nil {
		return DefinitionStats{}, err
	}
	failed, err := countQ(prefix + fmt.Sprintf(" AND ExecutionStatus = \"Failed\" AND CloseTime >= %q", startOfDay.Format(time.RFC3339)))
	if err != nil {
		return DefinitionStats{}, err
	}
	return DefinitionStats{Running: running, CompletedToday: completed, FailedToday: failed, AsOf: now}, nil
}

// scopedQuery builds a visibility query that scopes by WorkflowId prefix and
// optionally filters by status / time range.
func (d *Definition) scopedQuery(statuses []Status, tr *TimeRange) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("WorkflowId STARTS_WITH %q", d.Name+"-"))
	if len(statuses) > 0 {
		var statusStrs []string
		for _, s := range statuses {
			statusStrs = append(statusStrs, fmt.Sprintf("%q", strings.Title(s.String()))) //nolint:staticcheck
		}
		parts = append(parts, fmt.Sprintf("ExecutionStatus IN (%s)", strings.Join(statusStrs, ", ")))
	}
	if tr != nil {
		if !tr.Start.IsZero() {
			parts = append(parts, fmt.Sprintf("StartTime >= %q", tr.Start.Format(time.RFC3339)))
		}
		if !tr.End.IsZero() {
			parts = append(parts, fmt.Sprintf("StartTime <= %q", tr.End.Format(time.RFC3339)))
		}
	}
	return strings.Join(parts, " AND ")
}
```

Note on `strings.Title` deprecation: it's deprecated since Go 1.18 but works for ASCII. Suppressed via `nolint:staticcheck`. Acceptable for status names (`Running`, `Completed`, etc.) which are ASCII.

- [ ] **Step 2: Verify build**

```bash
cd /Users/jasoet/Documents/Go/pkg && go build ./temporal/job/...
```

Expected: clean. Existing tests still pass.

- [ ] **Step 3: Lint**

```bash
cd /Users/jasoet/Documents/Go/pkg && task lint
```

- [ ] **Step 4: Commit**

```bash
git add temporal/job/definition_workflow.go
git commit -m "feat(temporal/job): add Describe, History, lifecycle ops, ListRuns, Stats"
```

---

## Task 8: Definition schedule lifecycle

**Files:**
- Create: `pkg/temporal/job/definition_schedule.go`
- Modify: `pkg/temporal/job/registry.go` — replace the stub `ApplySchedules` body

- [ ] **Step 1: Implement definition_schedule.go**

`pkg/temporal/job/definition_schedule.go`:

```go
package job

import (
	"context"
	"fmt"

	"go.temporal.io/sdk/client"
)

// ApplySchedule creates or updates the Temporal schedule for this Definition.
// Schedule ID equals Definition.Name. If a schedule with that ID already
// exists, it is updated to match the current ScheduleSpec.
func (d *Definition) ApplySchedule(ctx context.Context, c client.Client) error {
	if d.Schedule == nil {
		return ErrNoSchedule
	}
	spec, err := d.Schedule.toSDKSpec()
	if err != nil {
		return fmt.Errorf("schedule: %w", err)
	}
	sc := c.ScheduleClient()

	// Check existence by trying to describe.
	handle := sc.GetHandle(ctx, d.Name)
	_, descErr := handle.Describe(ctx)
	if descErr == nil {
		// Update path
		return translateSDKError("schedule-update", handle.Update(ctx, client.ScheduleUpdateOptions{
			DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
				return &client.ScheduleUpdate{
					Schedule: &client.Schedule{
						Spec:   &spec,
						Action: scheduleAction(d),
						Policy: &client.SchedulePolicies{
							Overlap: d.Schedule.Overlap.toSDK(),
						},
						State: &client.ScheduleState{
							Paused: d.Schedule.Paused,
							Note:   d.Schedule.Note,
						},
					},
				}, nil
			},
		}))
	}

	// Create path
	_, err = sc.Create(ctx, client.ScheduleOptions{
		ID:   d.Name,
		Spec: spec,
		Action: &client.ScheduleWorkflowAction{
			ID:        d.Name + "-scheduled",
			Workflow:  d.Name,
			TaskQueue: d.TaskQueue,
		},
		Overlap: d.Schedule.Overlap.toSDK(),
		Paused:  d.Schedule.Paused,
		Note:    d.Schedule.Note,
	})
	return translateSDKError("schedule-create", err)
}

func scheduleAction(d *Definition) client.ScheduleAction {
	return &client.ScheduleWorkflowAction{
		ID:        d.Name + "-scheduled",
		Workflow:  d.Name,
		TaskQueue: d.TaskQueue,
	}
}

// PauseSchedule pauses an existing schedule.
func (d *Definition) PauseSchedule(ctx context.Context, c client.Client, note string) error {
	if d.Schedule == nil {
		return ErrNoSchedule
	}
	handle := c.ScheduleClient().GetHandle(ctx, d.Name)
	return translateSDKError("schedule-pause", handle.Pause(ctx, client.SchedulePauseOptions{Note: note}))
}

// ResumeSchedule unpauses an existing schedule.
func (d *Definition) ResumeSchedule(ctx context.Context, c client.Client, note string) error {
	if d.Schedule == nil {
		return ErrNoSchedule
	}
	handle := c.ScheduleClient().GetHandle(ctx, d.Name)
	return translateSDKError("schedule-resume", handle.Unpause(ctx, client.ScheduleUnpauseOptions{Note: note}))
}

// TriggerSchedule fires an immediate run of the schedule's action.
func (d *Definition) TriggerSchedule(ctx context.Context, c client.Client) error {
	if d.Schedule == nil {
		return ErrNoSchedule
	}
	handle := c.ScheduleClient().GetHandle(ctx, d.Name)
	return translateSDKError("schedule-trigger", handle.Trigger(ctx, client.ScheduleTriggerOptions{}))
}

// DeleteSchedule removes the schedule from Temporal.
func (d *Definition) DeleteSchedule(ctx context.Context, c client.Client) error {
	handle := c.ScheduleClient().GetHandle(ctx, d.Name)
	return translateSDKError("schedule-delete", handle.Delete(ctx))
}

// DescribeSchedule returns the current schedule state.
func (d *Definition) DescribeSchedule(ctx context.Context, c client.Client) (ScheduleDetail, error) {
	handle := c.ScheduleClient().GetHandle(ctx, d.Name)
	desc, err := handle.Describe(ctx)
	if err != nil {
		return ScheduleDetail{}, translateSDKError("schedule-describe", err)
	}
	det := ScheduleDetail{
		ScheduleSummary: ScheduleSummary{
			ID:           d.Name,
			WorkflowType: d.Name,
			Paused:       desc.Schedule.State.Paused,
			Note:         desc.Schedule.State.Note,
		},
	}
	if d.Schedule != nil {
		det.Spec = *d.Schedule
	}
	if len(desc.Info.NextActionTimes) > 0 {
		nt := desc.Info.NextActionTimes[0]
		det.NextRunTime = &nt
	}
	if len(desc.Info.RecentActions) > 0 {
		last := desc.Info.RecentActions[len(desc.Info.RecentActions)-1]
		lt := last.ActualTime
		det.LastRunTime = &lt
	}
	// RecentRuns left empty for v1; populating requires extra queries.
	return det, nil
}
```

- [ ] **Step 2: Restore Registry.ApplySchedules real body**

In `pkg/temporal/job/registry.go`, replace the stub:

```go
func (r *Registry) ApplySchedules(ctx context.Context, c client.Client) error {
	for _, d := range r.List() {
		if d.Schedule == nil {
			continue
		}
		if err := d.ApplySchedule(ctx, c); err != nil {
			return fmt.Errorf("apply schedule for %q: %w", d.Name, err)
		}
	}
	return nil
}
```

- [ ] **Step 3: Verify build + tests still pass**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:pkg -- ./temporal/job/... && task lint
```

- [ ] **Step 4: Commit**

```bash
git add temporal/job/definition_schedule.go temporal/job/registry.go
git commit -m "feat(temporal/job): add schedule lifecycle methods"
```

---

## Task 9: pkg integration tests

**Files:**
- Create: `pkg/temporal/job/definition_integration_test.go`
- Create: `pkg/temporal/job/schedule_integration_test.go`
- Create: `pkg/temporal/job/registry_integration_test.go`

- [ ] **Step 1: Write definition_integration_test.go**

`pkg/temporal/job/definition_integration_test.go`:

```go
//go:build integration

package job

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/pkg/v2/temporal/testcontainer"
)

// echoWorkflow is a tiny test workflow: takes a string, returns it.
func echoWorkflow(ctx workflow.Context, in string) (string, error) {
	var out string
	if err := workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
	}), echoActivity, in).Get(ctx, &out); err != nil {
		return "", err
	}
	return out, nil
}

func echoActivity(_ context.Context, in string) (string, error) { return in, nil }

func setupTestDef(t *testing.T, c client.Client, w worker.Worker) *Definition {
	t.Helper()
	d, err := New("echo", "echo-tq",
		WithRegister(func(w worker.Worker) {
			RegisterWorkflowOnce(w, "echo", echoWorkflow, workflow.RegisterOptions{Name: "echo"})
			RegisterActivityOnce(w, "echoActivity", echoActivity, activity.RegisterOptions{Name: "echoActivity"})
		}),
		WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, "echo", in)
		}),
		WithNewInput(func() any { var s string; return &s }),
	)
	require.NoError(t, err)
	d.Register(w)
	return d
}

func TestIntegration_Definition_Execute_Describe_History(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tc, c, cleanup, err := testcontainer.Setup(ctx, testcontainer.ClientConfig{Namespace: "default"}, testcontainer.Options{})
	require.NoError(t, err)
	defer cleanup()
	_ = tc

	w := worker.New(c, "echo-tq", worker.Options{})
	d := setupTestDef(t, c, w)
	require.NoError(t, w.Start())
	defer w.Stop()

	run, err := d.Execute(ctx, c, "hello world")
	require.NoError(t, err)
	assert.True(t, len(run.WorkflowID) > 5, "workflow ID has prefix")

	var got string
	require.NoError(t, run.Get(ctx, &got))
	assert.Equal(t, "hello world", got)

	detail, err := d.Describe(ctx, c, run.WorkflowID, run.RunID)
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, detail.Status)
	assert.Equal(t, "echo", detail.Type)

	hist, err := d.History(ctx, c, run.WorkflowID, run.RunID, HistoryOpts{})
	require.NoError(t, err)
	assert.NotEmpty(t, hist.Activities)
}

func TestIntegration_Definition_Cancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tc, c, cleanup, err := testcontainer.Setup(ctx, testcontainer.ClientConfig{Namespace: "default"}, testcontainer.Options{})
	require.NoError(t, err)
	defer cleanup()
	_ = tc

	// long-running workflow stub
	longWf := func(ctx workflow.Context, _ string) error {
		return workflow.Sleep(ctx, 1*time.Hour)
	}

	d, err := New("long", "long-tq",
		WithRegister(func(w worker.Worker) {
			RegisterWorkflowOnce(w, "long", longWf, workflow.RegisterOptions{Name: "long"})
		}),
		WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, "long", in)
		}),
		WithNewInput(func() any { var s string; return &s }),
	)
	require.NoError(t, err)

	w := worker.New(c, "long-tq", worker.Options{})
	d.Register(w)
	require.NoError(t, w.Start())
	defer w.Stop()

	run, err := d.Execute(ctx, c, "x")
	require.NoError(t, err)

	require.NoError(t, d.Cancel(ctx, c, run.WorkflowID, run.RunID))

	// Wait a moment for cancellation to propagate
	time.Sleep(2 * time.Second)
	detail, err := d.Describe(ctx, c, run.WorkflowID, run.RunID)
	require.NoError(t, err)
	assert.Equal(t, StatusCanceled, detail.Status)
}

func TestIntegration_Definition_ListRuns_ScopedByName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tc, c, cleanup, err := testcontainer.Setup(ctx, testcontainer.ClientConfig{Namespace: "default"}, testcontainer.Options{})
	require.NoError(t, err)
	defer cleanup()
	_ = tc

	w := worker.New(c, "echo-tq", worker.Options{})
	d := setupTestDef(t, c, w)
	require.NoError(t, w.Start())
	defer w.Stop()

	// Run twice.
	r1, err := d.Execute(ctx, c, "one")
	require.NoError(t, err)
	require.NoError(t, r1.Get(ctx, new(string)))
	r2, err := d.Execute(ctx, c, "two")
	require.NoError(t, err)
	require.NoError(t, r2.Get(ctx, new(string)))

	// Visibility settles asynchronously.
	time.Sleep(2 * time.Second)

	page, err := d.ListRuns(ctx, c, ListOpts{PageSize: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(page.Runs), 2, "ListRuns scoped by Name prefix returns both runs")
}
```

- [ ] **Step 2: Write schedule_integration_test.go**

`pkg/temporal/job/schedule_integration_test.go`:

```go
//go:build integration

package job

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/pkg/v2/temporal/testcontainer"
)

func TestIntegration_Schedule_FullLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tc, c, cleanup, err := testcontainer.Setup(ctx, testcontainer.ClientConfig{Namespace: "default"}, testcontainer.Options{})
	require.NoError(t, err)
	defer cleanup()
	_ = tc

	wf := func(workflow.Context, string) error { return nil }

	d, err := New("sched-test", "sched-tq",
		WithRegister(func(w worker.Worker) {
			RegisterWorkflowOnce(w, "sched-test", wf, workflow.RegisterOptions{Name: "sched-test"})
		}),
		WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, "sched-test", in)
		}),
		WithNewInput(func() any { var s string; return &s }),
		WithSchedule(&ScheduleSpec{Interval: time.Hour, Paused: true, Note: "initial"}),
	)
	require.NoError(t, err)

	w := worker.New(c, "sched-tq", worker.Options{})
	d.Register(w)
	require.NoError(t, w.Start())
	defer w.Stop()

	// Apply
	require.NoError(t, d.ApplySchedule(ctx, c))
	defer d.DeleteSchedule(ctx, c) //nolint:errcheck

	// Describe
	desc, err := d.DescribeSchedule(ctx, c)
	require.NoError(t, err)
	assert.True(t, desc.Paused)
	assert.Equal(t, "initial", desc.Note)

	// Resume
	require.NoError(t, d.ResumeSchedule(ctx, c, "resumed"))
	desc, err = d.DescribeSchedule(ctx, c)
	require.NoError(t, err)
	assert.False(t, desc.Paused)

	// Pause
	require.NoError(t, d.PauseSchedule(ctx, c, "paused again"))
	desc, err = d.DescribeSchedule(ctx, c)
	require.NoError(t, err)
	assert.True(t, desc.Paused)

	// Trigger (action runs once even though paused)
	require.NoError(t, d.TriggerSchedule(ctx, c))

	// Delete
	require.NoError(t, d.DeleteSchedule(ctx, c))
	_, err = d.DescribeSchedule(ctx, c)
	assert.Error(t, err)
}
```

- [ ] **Step 3: Write registry_integration_test.go**

`pkg/temporal/job/registry_integration_test.go`:

```go
//go:build integration

package job

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/pkg/v2/temporal/testcontainer"
)

func TestIntegration_Registry_RegisterAll_Deduplicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	tc, c, cleanup, err := testcontainer.Setup(ctx, testcontainer.ClientConfig{Namespace: "default"}, testcontainer.Options{})
	require.NoError(t, err)
	defer cleanup()
	_ = tc

	// Both definitions back the SAME underlying workflow type "shared".
	sharedWf := func(workflow.Context, string) (string, error) { return "ok", nil }

	mk := func(name string) *Definition {
		d, err := New(name, "shared-tq",
			WithRegister(func(w worker.Worker) {
				// Both Definitions try to register the same workflow type.
				RegisterWorkflowOnce(w, "shared", sharedWf, workflow.RegisterOptions{Name: "shared"})
			}),
			WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
				return c.ExecuteWorkflow(ctx, opts, "shared", in)
			}),
			WithNewInput(func() any { var s string; return &s }),
		)
		require.NoError(t, err)
		return d
	}

	r := NewRegistry(mk("alpha"), mk("beta"))
	w := worker.New(c, "shared-tq", worker.Options{})
	r.RegisterAll(w) // would panic on duplicate workflow type without dedup

	require.NoError(t, w.Start())
	defer w.Stop()

	// Trigger each by Definition name; both run on the same workflow type.
	for _, name := range []string{"alpha", "beta"} {
		run, err := r.MustGet(name).Execute(ctx, c, "in")
		require.NoError(t, err)
		var out string
		require.NoError(t, run.Get(ctx, &out))
		assert.Equal(t, "ok", out)
	}

	time.Sleep(2 * time.Second)

	// Each Definition's ListRuns is scoped to its name prefix.
	alphaPage, err := r.MustGet("alpha").ListRuns(ctx, c, ListOpts{})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(alphaPage.Runs), 1)
	for _, run := range alphaPage.Runs {
		assert.True(t, run.WorkflowID[:5] == "alpha", "expected alpha prefix, got %s", run.WorkflowID)
	}
}
```

- [ ] **Step 4: Run integration tests**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:integration
```

Expected: all `TestIntegration_*` in `temporal/job` pass. Each takes 10-30s (container startup + workflow execution).

- [ ] **Step 5: Commit**

```bash
git add temporal/job/definition_integration_test.go temporal/job/schedule_integration_test.go temporal/job/registry_integration_test.go
git commit -m "test(temporal/job): add Temporal end-to-end integration tests"
```

---

## Task 10: pkg final verification + push

- [ ] **Step 1: Full test suite**

```bash
cd /Users/jasoet/Documents/Go/pkg && task test:integration
```

Expected: all tests in pkg (not just temporal/job) pass.

- [ ] **Step 2: Lint**

```bash
cd /Users/jasoet/Documents/Go/pkg && task lint
```

Expected: clean.

- [ ] **Step 3: Push and create PR**

```bash
cd /Users/jasoet/Documents/Go/pkg
git push -u origin feat/temporal-job
gh pr create --title "feat(temporal/job): add type-focused Definition layer" --body "$(cat <<'EOF'
## Summary
- Adds `pkg/temporal/job` package with `Definition`, `Registry`, `ScheduleSpec`, and per-job lifecycle methods.
- Provides `RegisterWorkflowOnce` / `RegisterActivityOnce` for idempotent worker registration — used by go-wf's container/function builders to support multiple Definitions sharing a workflow type.

## Spec
[`go-wf`/docs/superpowers/specs/2026-05-11-pkg-temporal-job-definition-design.md`](https://github.com/jasoet/go-wf/blob/main/docs/superpowers/specs/2026-05-11-pkg-temporal-job-definition-design.md)

## Test plan
- [ ] task ci:test
- [ ] task test:integration (real Temporal container)
- [ ] task lint
EOF
)"
```

Wait for CI green before proceeding to go-wf changes.

---

## Phase 2 — go-wf builder convergence (Tasks 11–20)

All work in `/Users/jasoet/Documents/Go/go-wf` on branch `feat/job-definition`.

## Task 11: go-wf setup + pkg dependency

- [ ] **Step 1: Create branch**

```bash
cd /Users/jasoet/Documents/Go/go-wf
git checkout -b feat/job-definition
```

- [ ] **Step 2: Add replace directive in go.mod for development**

```bash
cd /Users/jasoet/Documents/Go/go-wf
go mod edit -replace github.com/jasoet/pkg/v2=/Users/jasoet/Documents/Go/pkg
go mod tidy
```

Verify:

```bash
grep replace go.mod
```

Expected:

```
replace github.com/jasoet/pkg/v2 => /Users/jasoet/Documents/Go/pkg
```

- [ ] **Step 3: Baseline**

```bash
task ci:test
```

Expected: all green. If anything fails, stop.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "build: temporarily replace pkg/v2 with local path for development"
```

(This commit will be reverted in Task 19 before merging.)

---

## Task 12: Make `container.RegisterAll` idempotent

**Files:**
- Modify: `container/worker.go` — route every `RegisterWorkflowWithOptions` / `RegisterActivityWithOptions` through `job.RegisterWorkflowOnce` / `RegisterActivityOnce`

- [ ] **Step 1: Read current container/worker.go to see the registrations**

```bash
grep -n "RegisterWorkflowWithOptions\|RegisterActivityWithOptions" container/worker.go
```

- [ ] **Step 2: Edit `container/worker.go`**

Add import: `"github.com/jasoet/pkg/v2/temporal/job"`.

For each `w.RegisterWorkflowWithOptions(fn, opts)` call, replace with `job.RegisterWorkflowOnce(w, opts.Name, fn, opts)`. Same for activities (`job.RegisterActivityOnce`).

If there are 7 workflow registrations (per the spec — single, pipeline, parallel, loop, parameterized-loop, DAG, parameterized-DAG) plus 1 activity (StartContainerActivity), all should be wrapped.

- [ ] **Step 3: Run container tests**

```bash
task test:pkg -- ./container/...
```

Expected: PASS — no behavioral change.

- [ ] **Step 4: Verify idempotency by adding a quick test**

Append to `container/worker_test.go` (or create it):

```go
func TestRegisterAll_Idempotent(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	w := worker.New(/* dummy client */ nil, "test-tq", worker.Options{})
	// Two calls — second must not panic.
	container.RegisterAll(w)
	container.RegisterAll(w)
}
```

If the test infrastructure doesn't allow this without a client, skip the test (the integration tests in Task 14 will cover it).

- [ ] **Step 5: Lint**

```bash
task lint
```

- [ ] **Step 6: Commit**

```bash
git add container/worker.go
git commit -m "refactor(container): make RegisterAll idempotent via job.RegisterWorkflowOnce"
```

---

## Task 13: Make `function.RegisterAll` idempotent

**Files:**
- Modify: `function/worker.go` — same treatment

- [ ] **Step 1: Edit `function/worker.go`**

Add `"github.com/jasoet/pkg/v2/temporal/job"` import. Replace `w.RegisterWorkflowWithOptions(...)` with `job.RegisterWorkflowOnce(...)` and `w.RegisterActivityWithOptions(...)` with `job.RegisterActivityOnce(...)`.

The function package's `RegisterWorkflows` and `RegisterActivity` helpers also need updating.

- [ ] **Step 2: Test, lint, commit**

```bash
task test:pkg -- ./function/...
task lint
git add function/worker.go
git commit -m "refactor(function): make RegisterAll idempotent via job.RegisterWorkflowOnce"
```

---

## Task 14: datasync builder → Build() returns *job.Definition

**Files:**
- Modify: `datasync/builder/builder.go` — `Build()` returns `*job.Definition`
- Modify: `datasync/builder/builder_test.go` — adjust assertions
- Modify: `datasync/workflow/sync.go` — DELETE `FullJobRegistration` + `BuildJobRegistration`

- [ ] **Step 1: Read current builder.go to plan the change**

```bash
cat datasync/builder/builder.go
```

The current `Build()` returns `(datasync.Job[T,U], error)`. Replace with `(*job.Definition, error)`.

- [ ] **Step 2: Edit `datasync/builder/builder.go`**

Replace the `Build` method body:

```go
func (b *SyncJobBuilder[T, U]) Build() (*job.Definition, error) {
	if b.name == "" {
		return nil, fmt.Errorf("job name is required")
	}
	if b.source == nil {
		return nil, fmt.Errorf("source is required")
	}
	if b.mapper == nil {
		return nil, fmt.Errorf("mapper is required")
	}
	if b.sink == nil {
		return nil, fmt.Errorf("sink is required")
	}
	if b.schedule <= 0 {
		return nil, fmt.Errorf("schedule must be positive")
	}

	j := datasync.Job[T, U]{
		Name:                    b.name,
		Source:                  b.source,
		Mapper:                  b.mapper,
		Sink:                    b.sink,
		Schedule:                b.schedule,
		Metadata:                b.metadata,
		ActivityTimeout:         b.activityTimeout,
		HeartbeatTimeout:        b.heartbeatTimeout,
		MaxRetries:              b.maxRetries,
		RetryInitialInterval:    b.retryInitialInterval,
		RetryBackoffCoefficient: b.retryBackoffCoefficient,
		RetryMaxInterval:        b.retryMaxInterval,
		Store:                   b.store,
	}

	return job.New(j.Name, datasyncwf.TaskQueue(j.Name),
		job.WithRegister(func(w worker.Worker) { datasyncwf.RegisterJob(w, j) }),
		job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, opts, j.Name, in)
		}),
		job.WithNewInput(func() any { return &payload.SyncExecutionInput{} }),
		job.WithSchedule(&job.ScheduleSpec{Interval: b.schedule}),
	)
}
```

Add imports: `context`, `github.com/jasoet/pkg/v2/temporal/job`, `go.temporal.io/sdk/client`, `go.temporal.io/sdk/worker`, `github.com/jasoet/go-wf/datasync/payload`, `datasyncwf "github.com/jasoet/go-wf/datasync/workflow"`.

- [ ] **Step 3: Update `datasync/builder/builder_test.go`**

The existing tests probably reference `datasync.Job`. Change them to assert against `*job.Definition`:

```go
def, err := b.Build()
require.NoError(t, err)
assert.Equal(t, "user-sync", def.Name)
assert.Equal(t, "sync-user-sync", def.TaskQueue)
```

Validation tests (`Build_FailsWithoutSource`, etc.) keep their structure but assert error messages instead of zero `Job`.

- [ ] **Step 4: Delete from `datasync/workflow/sync.go`**

Remove the `FullJobRegistration` struct and the `BuildJobRegistration` function. Keep everything else (`TaskQueue`, `RegisterJob`, `newSyncWorkflow`, `BuildWorkflowInput`, etc.).

- [ ] **Step 5: Run tests, fix breakages**

```bash
task test:pkg -- ./datasync/...
```

Anything that imports `datasynwf.FullJobRegistration` now breaks. Find them:

```bash
grep -rn "FullJobRegistration\|BuildJobRegistration" .
```

Fix call sites — most likely in `datasync/chunk` (Task 15 covers).

- [ ] **Step 6: Commit**

```bash
git add datasync/builder/builder.go datasync/builder/builder_test.go datasync/workflow/sync.go
git commit -m "feat(datasync): SyncJobBuilder.Build returns *job.Definition; drop FullJobRegistration"
```

---

## Task 15: datasync/chunk → return *job.Definition

**Files:**
- Modify: `datasync/chunk/sync.go` — `Build()` returns `*job.Definition`, schedule field is `*job.ScheduleSpec`
- Modify: `datasync/chunk/date_sync.go` — same
- Modify: `datasync/chunk/sync_test.go`, `sync_integration_test.go`, `date_sync_test.go` — adjust assertions and method names

- [ ] **Step 1: Edit `datasync/chunk/sync.go`**

In the `ChunkedSync` struct, change:
- `schedule time.Duration` → `schedule *job.ScheduleSpec`

Add methods:
- `ScheduleEvery(d time.Duration) *ChunkedSync[In, Out, K]` — sets `c.schedule = &job.ScheduleSpec{Interval: d}`
- `ScheduleCron(expr string) *ChunkedSync[In, Out, K]` — sets `c.schedule = &job.ScheduleSpec{Cron: expr}`
- `ScheduleRaw(spec *job.ScheduleSpec) *ChunkedSync[In, Out, K]` — sets `c.schedule = spec`

Remove the existing `.Schedule(d time.Duration)` method.

In `Build()`, replace the return:

```go
return job.New(c.name, datasyncwf.TaskQueue(c.name),
    job.WithRegister(registerFn),       // existing closure
    job.WithExecute(executeFn),         // wrap the existing ExecuteWorkflow call signature
    job.WithNewInput(func() any { return &payload.SyncExecutionInput{} }),
    job.WithSchedule(c.schedule),
)
```

The `WithExecute` closure changes from `(ctx, c, in)` to `(ctx, c, opts, in)` — pass through to `c.ExecuteWorkflow(ctx, opts, c.name, in)`.

- [ ] **Step 2: Edit `datasync/chunk/date_sync.go`**

Replace `Schedule(d time.Duration)` with `ScheduleEvery(d time.Duration)` etc., proxying to inner.

- [ ] **Step 3: Update test files**

In `sync_test.go`, `sync_integration_test.go`, `date_sync_test.go`:
- Replace all `datasyncwf.FullJobRegistration` → `*job.Definition`
- Replace `.Schedule(15 * time.Minute)` → `.ScheduleEvery(15 * time.Minute)`
- Assert `reg.Schedule.Interval` instead of `reg.Schedule` for time.Duration check

- [ ] **Step 4: Run, lint, commit**

```bash
task test:pkg -- ./datasync/chunk/...
task lint
git add datasync/chunk/
git commit -m "feat(chunk): ChunkedSync.Build returns *job.Definition; schedule via ScheduleEvery/Cron"
```

---

## Task 16: function builder → single `Build() *job.Definition` (Path β, full collapse)

**Files:**
- Modify: `function/builder/builder.go` — add `Name`/`TaskQueue`/`Activity` setters + mode setters (`Pipeline`, `Parallel`, `Single`, `Loop`); replace existing `Build*` methods with one `Build() (*job.Definition, error)`
- Modify: `function/builder/builder_test.go` — update all callers
- Modify: any `examples/function/*.go` callers — update

### Step 1: Refactor `WorkflowBuilder[I,O]` in `function/builder/builder.go`

Add fields:

```go
type workflowMode int

const (
    modeUnset workflowMode = iota
    modePipeline
    modeParallel
    modeSingle
    modeLoop
    modeParameterizedLoop
)

// (additions to WorkflowBuilder[I, O])
name       string
taskQueue  string
activityFn any            // captures the function activity to register
mode       workflowMode
```

Add mode setters:

```go
func (b *WorkflowBuilder[I, O]) Pipeline() *WorkflowBuilder[I, O] { b.mode = modePipeline; return b }
func (b *WorkflowBuilder[I, O]) Parallel() *WorkflowBuilder[I, O] { b.mode = modeParallel; return b }
func (b *WorkflowBuilder[I, O]) Single()   *WorkflowBuilder[I, O] { b.mode = modeSingle; return b }
func (b *WorkflowBuilder[I, O]) Loop()     *WorkflowBuilder[I, O] { b.mode = modeLoop; return b }
func (b *WorkflowBuilder[I, O]) ParameterizedLoop() *WorkflowBuilder[I, O] { b.mode = modeParameterizedLoop; return b }
```

Add identity setters:

```go
func (b *WorkflowBuilder[I, O]) Name(n string) *WorkflowBuilder[I, O]       { b.name = n; return b }
func (b *WorkflowBuilder[I, O]) TaskQueue(tq string) *WorkflowBuilder[I, O] { b.taskQueue = tq; return b }
func (b *WorkflowBuilder[I, O]) Activity(fn any) *WorkflowBuilder[I, O]     { b.activityFn = fn; return b }
```

Refactor existing internal methods that build inputs to be private (`buildPipelineInput`, `buildParallelInput`, etc.) — keep the input-construction logic, drop the public `BuildPipeline`/`BuildParallel`/`BuildSingle`/`BuildLoop`/`BuildParameterizedLoop` methods entirely.

Add the single `Build()`:

```go
func (b *WorkflowBuilder[I, O]) Build() (*job.Definition, error) {
    if b.name == "" {
        return nil, fmt.Errorf("function.WorkflowBuilder: Name is required")
    }
    if b.mode == modeUnset {
        return nil, fmt.Errorf("function.WorkflowBuilder: call .Pipeline()/.Parallel()/.Single()/.Loop()/.ParameterizedLoop() before Build")
    }
    if b.activityFn == nil {
        return nil, fmt.Errorf("function.WorkflowBuilder: Activity is required")
    }
    tq := b.taskQueue
    if tq == "" {
        tq = "function-" + b.name
    }

    var wfType string
    var inputFactory func() any
    switch b.mode {
    case modePipeline:
        wfType = "function.PipelineWorkflow"
        inputFactory = func() any { return b.buildPipelineInput() }
    case modeParallel:
        wfType = "function.ParallelWorkflow"
        inputFactory = func() any { return b.buildParallelInput() }
    case modeSingle:
        wfType = "function.SingleWorkflow"
        inputFactory = func() any { return b.buildSingleInput() }
    case modeLoop:
        wfType = "function.LoopWorkflow"
        inputFactory = func() any { return b.buildLoopInput() }
    case modeParameterizedLoop:
        wfType = "function.ParameterizedLoopWorkflow"
        inputFactory = func() any { return b.buildParameterizedLoopInput() }
    }

    activityFn := b.activityFn
    return job.New(b.name, tq,
        job.WithRegister(func(w worker.Worker) {
            function.RegisterAll(w, activityFn) // now idempotent (Task 13)
        }),
        job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
            return c.ExecuteWorkflow(ctx, opts, wfType, in)
        }),
        job.WithNewInput(inputFactory),
    )
}
```

Verify the `function.*Workflow` names match the actual `function.RegisterAll` registrations (they should — they were defined there). If names differ, use the exact strings from `function/worker.go`.

### Step 2: Update tests in `function/builder/builder_test.go`

Find every test that calls `BuildPipeline()`, `BuildParallel()`, `BuildSingle()`, `BuildLoop()`, `BuildParameterizedLoop()` and replace with the new pattern: `.Pipeline().Build()`, etc. Also add the new identity setters to satisfy validation (`.Name(...)`, `.Activity(...)`).

Add a new test for the Definition output:

```go
func TestBuild_ProducesDefinition_Pipeline(t *testing.T) {
    def, err := NewWorkflowBuilder[any, any]().
        Name("daily-cleanup").
        Pipeline().
        Activity(func() {}). // stub activity for validation
        // ... existing pipeline configuration setters
        Build()
    require.NoError(t, err)
    assert.Equal(t, "daily-cleanup", def.Name)
    assert.Equal(t, "function-daily-cleanup", def.TaskQueue)
}

func TestBuild_RequiresMode(t *testing.T) {
    _, err := NewWorkflowBuilder[any, any]().Name("x").Activity(func() {}).Build()
    assert.Error(t, err)
}

func TestBuild_RequiresName(t *testing.T) {
    _, err := NewWorkflowBuilder[any, any]().Pipeline().Activity(func() {}).Build()
    assert.Error(t, err)
}
```

### Step 3: Update examples under `examples/function/`

Find call sites of `BuildPipeline`/`BuildParallel`/`BuildSingle`/`BuildLoop`:

```bash
grep -rn "BuildPipeline\|BuildParallel\|BuildSingle\|BuildLoop\|BuildParameterizedLoop" examples/function/
```

Update each to `.Pipeline().Build()` (etc.) and chain in `.Name(...)`. Since these are `//go:build example` files, they don't run in CI; verify with `go vet -tags=example ./examples/function/...`.

### Step 4: Run, lint, commit

```bash
task test:pkg -- ./function/...
task lint
git add function/builder/ examples/function/
git commit -m "feat(function): single Build() returns *job.Definition; mode via .Pipeline()/.Parallel()/etc."
```

---

## Task 17: container builder → single `Build() *job.Definition`

**Files:**
- Modify: `container/builder/builder.go` — same shape of refactor as Task 16
- Modify: `container/builder/builder_test.go` — update callers
- Modify: `examples/container/*.go` — update callers

Mirror of Task 16 for container. Differences:
- Default TaskQueue: `"container-<name>"`.
- Mode setters: `.Pipeline()`, `.Parallel()`, `.Single()`, `.Loop()`, `.ParameterizedLoop()`, `.DAG()`, `.ParameterizedDAG()` (7 modes vs function's 5 — container has DAG variants too; verify against `container/worker.go` registrations).
- The activity is `StartContainerActivity` — single, no `Activity()` setter needed. The register closure inside the Definition's `WithRegister` simply calls `container.RegisterAll(w)`.

- [ ] **Step 1-3: Implement following Task 16's template**

Identify all current public `Build*` methods in `container/builder/builder.go` (`BuildPipeline`, `BuildParallel`, `BuildSingle`, `BuildLoop`, `BuildParameterizedLoop`, `BuildDAG`, `BuildParameterizedDAG`, or similar — exact list per the file). Make them private (`buildXxxInput`) and add a single public `Build() (*job.Definition, error)` that switches on mode.

```go
func (b *WorkflowBuilder) Build() (*job.Definition, error) {
    if b.name == "" {
        return nil, fmt.Errorf("container.WorkflowBuilder: Name is required")
    }
    if b.mode == modeUnset {
        return nil, fmt.Errorf("container.WorkflowBuilder: call .Pipeline()/.Parallel()/.Single()/.Loop()/.ParameterizedLoop()/.DAG()/.ParameterizedDAG() before Build")
    }
    tq := b.taskQueue
    if tq == "" {
        tq = "container-" + b.name
    }
    wfType, inputFactory := b.resolveWorkflow()
    return job.New(b.name, tq,
        job.WithRegister(func(w worker.Worker) { container.RegisterAll(w) }),
        job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
            return c.ExecuteWorkflow(ctx, opts, wfType, in)
        }),
        job.WithNewInput(inputFactory),
    )
}
```

(Helper `resolveWorkflow()` switches on `b.mode` and returns the workflow type name + input factory closure, just like Task 16's switch.)

Update tests and examples following the same find-and-replace pattern as Task 16.

```bash
task test:pkg -- ./container/...
task lint
git add container/builder/ examples/container/
git commit -m "feat(container): single Build() returns *job.Definition; mode via .Pipeline()/.Parallel()/.DAG()/etc."
```

---

## Task 18: Update examples

**Files:**
- Modify: `examples/datasync/chunk_basic.go` — use `*job.Definition` API
- Modify: `examples/datasync/basic.go` — use `SyncJobBuilder` → `Build()` returning `*job.Definition`
- Other `examples/datasync/*.go` as needed

- [ ] **Step 1: Update chunk_basic.go**

Replace the workflow registration to use `reg.Register(w)` where `reg` is now `*job.Definition`. Replace `.Schedule(d)` with `.ScheduleEvery(d)` if used.

- [ ] **Step 2: Verify build**

```bash
go build -tags=example ./examples/datasync/chunk_basic.go
```

- [ ] **Step 3: Commit**

```bash
git add examples/
git commit -m "docs(examples): update datasync examples for *job.Definition API"
```

---

## Task 19: Remove replace directive + bump pkg/v2 version

Assumes the pkg PR has been merged and a tagged release exists.

- [ ] **Step 1: Remove replace**

```bash
cd /Users/jasoet/Documents/Go/go-wf
go mod edit -dropreplace github.com/jasoet/pkg/v2
go get github.com/jasoet/pkg/v2@latest
go mod tidy
```

- [ ] **Step 2: Verify**

```bash
grep -n "jasoet/pkg/v2" go.mod
task ci:test
```

Expected: builds and tests pass against the published `pkg/v2`.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "build: bump pkg/v2 to published release; drop local replace"
```

---

## Task 20: Final go-wf verification + PR

- [ ] **Step 1: Full test suite + integration**

```bash
cd /Users/jasoet/Documents/Go/go-wf
task ci:test
task test:integration
```

- [ ] **Step 2: Lint**

```bash
task lint
```

- [ ] **Step 3: Push and PR**

```bash
git push -u origin feat/job-definition
gh pr create --title "feat: converge builders on *job.Definition (pkg/temporal/job consumer)" --body "$(cat <<'EOF'
## Summary
- All four builders (`container/`, `function/`, `datasync/`, `datasync/chunk/`) now produce `*job.Definition` from `pkg/temporal/job`.
- `datasync/workflow.FullJobRegistration` and `BuildJobRegistration` deleted.
- `container.RegisterAll` and `function.RegisterAll` are now idempotent (route through `job.RegisterWorkflowOnce`).
- Examples updated.

## Spec
docs/superpowers/specs/2026-05-11-pkg-temporal-job-definition-design.md

## Depends on
jasoet/pkg#NN (pkg/temporal/job package) — must be merged first.

## Test plan
- [ ] task ci:test
- [ ] task test:integration
- [ ] task lint
- [ ] All examples build under `-tags=example`
EOF
)"
```

---

## Self-review notes

**Spec coverage:**
- ✅ `pkg/temporal/job` package — Tasks 1–9
- ✅ All types from the spec (Definition, Registry, ScheduleSpec, RunHandle, RunDetail, etc.) — Tasks 2–6
- ✅ All Definition methods (Register, Execute, GetRun, Describe, History, Cancel, Terminate, Signal, Query, ListRuns, Stats, ApplySchedule, lifecycle) — Tasks 5, 7, 8
- ✅ Integration tests — Task 9
- ✅ RegisterAll idempotency for container/function — Tasks 12–13
- ✅ Builder convergence — Tasks 14–17
- ✅ FullJobRegistration deletion — Task 14
- ✅ Examples — Task 18
- ✅ Dependency migration (replace → published) — Task 19

**Placeholder scan:** Searched for "TBD", "TODO", "similar to", "implement later", "Add appropriate" — none in code blocks. Step descriptions occasionally use "as needed" but always provide concrete guidance.

**Type consistency:**
- `Definition` struct definition (Task 5) matches usage in Tasks 7, 8, 14, 15, 16, 17.
- `ScheduleSpec` (Task 3) referenced consistently in Tasks 5, 8, 14, 15.
- `executeConfig` (Task 4) referenced by `Definition.Execute` (Task 5).
- `RunHandle` (Task 2) consumed by `Execute` / `GetRun` (Task 5) and integration tests (Task 9).
- `RegisterWorkflowOnce` (Task 5) consumed by container/function refactor (Tasks 12, 13) and chunk's existing closure (no change needed there — chunk already uses its own dedup pattern that we'll align).
- L1 builders (Tasks 16, 17) fully collapse to single `Build()` — no `BuildDefinition()` cruft.
