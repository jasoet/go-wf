# Design: `pkg/temporal/job` — Type-Focused Workflow Definition Layer

**Date:** 2026-05-11
**Status:** Pending review
**Scope:** Add a new `pkg/temporal/job` package providing a type-focused `Definition` abstraction for one registered Temporal workflow; converge all go-wf builders (`container/`, `function/`, `datasync/`, `datasync/chunk/`) on producing `*job.Definition`; delete `datasync/workflow.FullJobRegistration`.

## Context

Multiple consumer projects (e.g., iws) wrap the Temporal SDK to expose workflow management to end-users (list runs, trigger by name, attach schedules, cancel, terminate) from their own UI/API instead of the Temporal Web UI. Each project re-implements the same pattern: a `map[string]workflowFn` + a `switch` on type name in the trigger handler, ad-hoc serialization, ad-hoc activity history extraction.

`pkg/temporal` already provides namespace-wide operational primitives (`WorkflowManager`, `ScheduleManager`, `WorkerManager`). What it lacks is a **per-workflow, type-focused** abstraction: an object that represents one registered workflow and lets the holder do everything related to that one workflow (register on a worker, execute by name, attach schedule, describe runs, control lifecycle).

`go-wf` already has the shape of this in `datasync/workflow.FullJobRegistration` (`Name`, `TaskQueue`, `Schedule`, `Register func(worker.Worker)`, `WorkflowInput`), but it is incomplete (no `Execute`, no per-job inspection/control methods, no schedule lifecycle), inconsistent across packages (`container/` and `function/` use a package-level `RegisterAll(w)` instead of producing a descriptor), and not reusable outside go-wf.

## Goals

- Add a generic, transport-agnostic `job.Definition` type in `pkg/temporal/job` usable by **any** Temporal SDK consumer.
- Every per-job operation hangs off the type as a method (Path Y — fat type): register, execute, describe, history, cancel, terminate, signal, query, schedule lifecycle, list runs, stats.
- Converge all four go-wf builders (`container/`, `function/`, `datasync/builder/`, `datasync/chunk/`) on producing `*job.Definition`. No more `FullJobRegistration`.
- A registry primitive (`job.Registry`) that lets consumers look up Definitions by name, deduplicating workflow-type registration when several Definitions back the same type.
- Keep existing `pkg/temporal` namespace-wide managers (`WorkflowManager`, `ScheduleManager`, `WorkerManager`) untouched. They coexist with `job.Definition` — managers are the "stranger workflows" tool; Definitions are the "my workflows" tool.

## Non-Goals

- HTTP / REST / gRPC handlers — out of scope. Consumers expose Definitions over whatever transport they choose.
- UI components — out of scope.
- Authentication / authorization — out of scope. The library is auth-agnostic.
- Backward compatibility with `FullJobRegistration` — all known consumers are user-controlled and migrate in lockstep with this change.
- Temporal Update API support (update-with-start) — defer to a future version.

---

## Architecture

Two repositories, one new package each side.

```
pkg/temporal/job/                NEW. Generic, transport-agnostic.
  definition.go                  Definition struct + per-job methods
  registry.go                    Registry type
  schedule_spec.go               ScheduleSpec wrapper
  status.go                      Status enum + transition helpers
  result.go                      RunHandle, RunDetail, RunHistory, ActivityEvent, RunSummary, RunPage,
                                 DefinitionStats, ScheduleSummary, ScheduleDetail
  options.go                     ListOpts, StatsOpts, HistoryOpts, ScheduleListOpts, ExecuteOption
  errors.go                      Typed errors + SDK-error translation

go-wf/                           MODIFIED. All builders converge on *job.Definition.
  container/builder/             Adds Build() *job.Definition (Path β)
  function/builder/              Adds Build() *job.Definition (Path β)
  datasync/builder/              Build() returns *job.Definition (replaces Job[T,U] return)
  datasync/chunk/                Build() returns *job.Definition (renamed from FullJobRegistration)
  datasync/workflow/             FullJobRegistration DELETED; BuildJobRegistration DELETED.
```

### Layer responsibilities

- **`pkg/temporal/job`** owns: the type, its methods, error mapping, registry lookup. Knows nothing about container/function/datasync.
- **`go-wf` builders** own: producing concrete `*job.Definition` values wired with the right `register` / `execute` / `newInput` closures for their workflow shape.
- **Existing `pkg/temporal` managers**: untouched. Namespace-wide ops stay there.

### Runtime data flow

1. App startup: builders construct `*job.Definition` values → grouped into a `job.Registry`.
2. Worker setup: `registry.RegisterAll(w)` calls each Definition's `Register(w)`, deduplicated so the same workflow type isn't double-registered.
3. Schedules: `registry.ApplySchedules(ctx, c)` creates/updates Temporal schedules for Definitions that have a `Schedule`.
4. Trigger by name: `registry.MustGet("orders-sync").Execute(ctx, c, input)`.
5. Inspect/control: `def.Describe(ctx, c, wfID, runID)` / `def.Cancel(...)` / `def.History(...)`.

### Layering distinction (L1 vs L2)

- **L1 — execution primitives**: `container/`, `function/`. Generic workflow shapes (Pipeline/Parallel/Loop/DAG) + a single configurable activity. Multiple Definitions can back the same workflow type — dedup is essential.
- **L2 — domain patterns**: `datasync/`, `datasync/chunk/`. Specific workflow shape with semantic meaning; one builder = one workflow type = one Definition. Dedup not strictly needed but Registry uses the same mechanic uniformly.

---

## Core Types

### `Definition`

```go
package job

type Definition struct {
    Name        string
    TaskQueue   string
    Description string
    Tags        []string
    Schedule    *ScheduleSpec // nil = unscheduled

    // private wiring set by builder constructors only
    register func(worker.Worker)
    execute  func(ctx context.Context, c client.Client, input any) (client.WorkflowRun, error)
    newInput func() any
}
```

Construction uses a single functional-options API:

```go
func New(name, taskQueue string, opts ...Option) (*Definition, error)

type Option func(*Definition)

func WithRegister(fn func(worker.Worker)) Option
func WithExecute(fn func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, input any) (client.WorkflowRun, error)) Option
func WithNewInput(fn func() any) Option
func WithSchedule(spec *ScheduleSpec) Option
func WithDescription(desc string) Option
func WithTags(tags ...string) Option
```

`New` validates that `Register`, `Execute`, and `NewInput` are all supplied; returns `ErrInvalidDefinition` otherwise. Both go-wf builders and raw-Temporal users use the same constructor.

#### Per-job methods (Path Y — fat)

```go
// Registration
func (d *Definition) Register(w worker.Worker)

// Execution
func (d *Definition) NewInput() any
func (d *Definition) Execute(ctx context.Context, c client.Client, input any, opts ...ExecuteOption) (RunHandle, error)

// Run handle for a run you already know the ID of (skips re-Execute)
func (d *Definition) GetRun(c client.Client, wfID, runID string) RunHandle

// Schedule control (only valid when d.Schedule != nil)
func (d *Definition) ApplySchedule(ctx context.Context, c client.Client) error
func (d *Definition) PauseSchedule(ctx context.Context, c client.Client, note string) error
func (d *Definition) ResumeSchedule(ctx context.Context, c client.Client, note string) error
func (d *Definition) TriggerSchedule(ctx context.Context, c client.Client) error
func (d *Definition) DeleteSchedule(ctx context.Context, c client.Client) error
func (d *Definition) DescribeSchedule(ctx context.Context, c client.Client) (ScheduleDetail, error)

// Per-run inspection / control ("" runID = latest run)
func (d *Definition) Describe(ctx context.Context, c client.Client, wfID, runID string) (RunDetail, error)
func (d *Definition) History(ctx context.Context, c client.Client, wfID, runID string, opts HistoryOpts) (RunHistory, error)
func (d *Definition) Cancel(ctx context.Context, c client.Client, wfID, runID string) error
func (d *Definition) Terminate(ctx context.Context, c client.Client, wfID, runID, reason string) error
func (d *Definition) Signal(ctx context.Context, c client.Client, wfID, runID, signalName string, payload any) error
func (d *Definition) Query(ctx context.Context, c client.Client, wfID, runID, queryType string, args ...any) (any, error)

// Per-job listing
func (d *Definition) ListRuns(ctx context.Context, c client.Client, opts ListOpts) (RunPage, error)
func (d *Definition) Stats(ctx context.Context, c client.Client, opts StatsOpts) (DefinitionStats, error)
```

### Run scoping (uniform across L1 and L2)

`Definition.Execute` generates workflow IDs of the form `"<Name>-<uuid>"` unless the caller overrides via `WithWorkflowID(...)`. `ListRuns` and `Stats` filter the visibility query by `WorkflowId STARTS_WITH "<Name>-"` so the scope is unambiguous regardless of layer:

- **L2** (datasync/chunk): `Name == WorkflowType`, but we still use the ID-prefix filter — it's just as accurate and avoids a layer-conditional code path.
- **L1** (container/function): multiple Definitions share a Temporal workflow type, but each Definition's runs have distinct ID prefixes, so the filter cleanly separates them.

Callers that override `WithWorkflowID(...)` to an ID that doesn't start with `"<Name>-"` opt out of `ListRuns`/`Stats` scoping for that run. Documented contract; no enforcement.

### ExecuteOption

```go
type ExecuteOption func(*executeConfig)

func WithWorkflowID(id string) ExecuteOption           // override default "<Name>-<uuid>"
func WithTimeout(d time.Duration) ExecuteOption        // WorkflowExecutionTimeout
func WithTaskTimeout(d time.Duration) ExecuteOption    // WorkflowTaskTimeout
func WithRetryPolicy(p *temporal.RetryPolicy) ExecuteOption
func WithMemo(m map[string]any) ExecuteOption
func WithSearchAttributes(sa map[string]any) ExecuteOption
```

`Definition.Execute` constructs a default `client.StartWorkflowOptions{ID: generateID(d.Name), TaskQueue: d.TaskQueue}`, applies each `ExecuteOption`, then passes the finalised opts to the builder-supplied execute closure.

### `Registry`

```go
type Registry struct {
    defs map[string]*Definition
}

func NewRegistry(defs ...*Definition) *Registry
func (r *Registry) Add(d *Definition) error                          // ErrDuplicateName / ErrInvalidDefinition
func (r *Registry) Get(name string) (*Definition, bool)
func (r *Registry) MustGet(name string) *Definition                  // panics if missing
func (r *Registry) List() []*Definition                              // sorted by Name
func (r *Registry) Names() []string

func (r *Registry) RegisterAll(w worker.Worker)
func (r *Registry) ApplySchedules(ctx context.Context, c client.Client) error
```

### `ScheduleSpec`

```go
type ScheduleSpec struct {
    // Exactly one of Interval / Cron / Calendar must be set
    Interval time.Duration
    Cron     string
    Calendar []CalendarSpec // escape hatch, mapped to client.ScheduleCalendarSpec
    // CalendarSpec mirrors the field set of client.ScheduleCalendarSpec:
    //   type CalendarSpec struct {
    //       Second, Minute, Hour, DayOfMonth, Month, Year, DayOfWeek []ScheduleRange
    //       Comment string
    //   }
    //   type ScheduleRange struct { Start, End, Step int }

    // Behavior
    Overlap       OverlapPolicy // Skip (default), BufferOne, BufferAll, CancelOther, TerminateOther, AllowAll
    Jitter        time.Duration
    Paused        bool
    Note          string
    CatchupWindow time.Duration
}
```

go-wf builder sugar: `.ScheduleEvery(d)`, `.ScheduleCron(expr)`, `.ScheduleRaw(*ScheduleSpec)`.

### Result types (all in `pkg/temporal/job`, plain Go structs, no JSON tags)

```go
type RunHandle struct {
    WorkflowID, RunID string
    raw               client.WorkflowRun
}
func (h RunHandle) Get(ctx context.Context, valuePtr any) error

type RunDetail struct {
    WorkflowID, RunID, Type, TaskQueue string
    Status                             Status
    StartTime                          time.Time
    CloseTime                          *time.Time
    ExecutionTime                      time.Duration
    HistoryLength                      int64
    Memo, SearchAttributes             map[string]any
}

type RunHistory struct {
    WorkflowID, RunID string
    Activities        []ActivityEvent
    Truncated         bool
}
type ActivityEvent struct {
    Name                 string
    Status               ActivityStatus
    Attempt              int32
    StartTime, CloseTime time.Time
    Duration             time.Duration
    Input                []byte
    Result               []byte
    Error                string
}

type RunPage struct {
    Runs          []RunSummary
    NextPageToken []byte
}
type RunSummary struct {
    WorkflowID, RunID, Type string
    Status                  Status
    StartTime               time.Time
    CloseTime               *time.Time
    TaskQueue               string
}

type DefinitionStats struct {
    Running        int64
    CompletedToday int64
    FailedToday    int64
    AsOf           time.Time
}

type ScheduleSummary struct {
    ID, WorkflowType string
    Paused           bool
    NextRunTime      *time.Time
    LastRunTime      *time.Time
    Note             string
}
type ScheduleDetail struct {
    ScheduleSummary
    Spec       ScheduleSpec   // wrapped, not raw client.ScheduleSpec
    RecentRuns []RunSummary
}

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
func (s Status) String() string
func (s Status) IsTerminal() bool

type ActivityStatus int
const (
    ActivityScheduled ActivityStatus = iota
    ActivityStarted
    ActivityCompleted
    ActivityFailed
    ActivityTimedOut
    ActivityCanceled
)
```

### Options

```go
type ListOpts struct {
    Status    []Status     // empty = any
    TimeRange *TimeRange   // by StartTime
    PageSize  int          // default 100, max 1000
    PageToken []byte
}
type StatsOpts struct {
    TodayOnly bool       // default true — "today" = UTC calendar day [00:00, 24:00) at server time
    Location  *time.Location // optional; if set, "today" uses this zone instead of UTC
}
type HistoryOpts struct {
    MaxEvents int // default 500; 0 = no cap (caller takes responsibility)
}
type ScheduleListOpts struct {
    PageSize  int
    PageToken []byte
}
type TimeRange struct {
    Start, End time.Time
}
```

---

## go-wf Builder Integration

### Deletions

- `datasync/workflow.FullJobRegistration` (struct).
- `datasync/workflow.BuildJobRegistration` (function).

### Changes per builder

**`datasync/builder.SyncJobBuilder[T,U]`** — `Build()` returns `*job.Definition` (replaces `Job[T,U]` return). The internal `Job[T,U]` struct is constructed and wrapped via `job.New`:

```go
return job.New(j.Name, datasyncwf.TaskQueue(j.Name),
    job.WithRegister(func(w worker.Worker) { datasyncwf.RegisterJob(w, j) }),
    job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
        return c.ExecuteWorkflow(ctx, opts, j.Name, in)
    }),
    job.WithNewInput(func() any { return &payload.SyncExecutionInput{} }),
    job.WithSchedule(b.schedule),
)
```

The closure receives a fully-prepared `StartWorkflowOptions` (ID, TaskQueue, plus any caller overrides applied via `ExecuteOption`). The closure only needs to know which workflow function name to invoke.

**`datasync/chunk.ChunkedSync` / `DateChunkedSync`** — already produces FullJobRegistration today; trivial rename to `*job.Definition`. Schedule field type changes from `time.Duration` to `*job.ScheduleSpec`; new sugar methods replace `.Schedule(d)`.

**`function/builder.WorkflowBuilder[I,O]`** — gains `Name(string)`, optional `TaskQueue(string)` (default `"function-<name>"`), `Build() *job.Definition`. Definition's `Register` calls `function.RegisterAll(w, activityFn)` once per worker via dedup. `Execute` calls `c.ExecuteWorkflow(ctx, opts, "function.Pipeline" /* or whichever */, input)`.

**`container/builder.WorkflowBuilder`** — mirror of function. `TaskQueue(string)` default `"container-<name>"`.

### Dedup mechanic for Path β

Two cooperating pieces:

**1. `job.RegisterWorkflowOnce` / `job.RegisterActivityOnce` helpers** for fine-grained idempotency:

```go
// pkg/temporal/job/definition.go

type registrarKey struct {
    worker   worker.Worker
    typeName string
}
var registeredWorkflows sync.Map
var registeredActivities sync.Map

func RegisterWorkflowOnce(w worker.Worker, typeName string, wf any, opts workflow.RegisterOptions) {
    key := registrarKey{w, typeName}
    if _, loaded := registeredWorkflows.LoadOrStore(key, struct{}{}); loaded {
        return
    }
    w.RegisterWorkflowWithOptions(wf, opts)
}
// RegisterActivityOnce has the same shape.
```

**2. `container.RegisterAll` / `function.RegisterAll` MUST be made idempotent** by refactoring their internal calls to `worker.RegisterWorkflowWithOptions` / `worker.RegisterActivityWithOptions` to go through `job.RegisterWorkflowOnce` / `job.RegisterActivityOnce`.

Required change in `container/worker.go` and `function/worker.go`: every `w.RegisterWorkflowWithOptions(wfFn, opts)` becomes `job.RegisterWorkflowOnce(w, opts.Name, wfFn, opts)`. Same pattern for activities.

After this change, calling `function.RegisterAll(w, activityFn)` twice on the same worker is safe — second call is a no-op for already-registered types.

Note on memory: the `sync.Map` keyed by `(worker, typeName)` retains entries for the worker's lifetime. In tests that spin up many short-lived workers, entries accumulate. The leak is bounded (one small entry per `(worker, typeName)` pair created) and acceptable for the library's expected usage pattern (a small number of long-lived workers per process). If this becomes a problem, a future revision can attach the dedup state to a `*Registry` instead of package-global state.

---

## Error Handling

### Typed errors

```go
var (
    ErrNotFound         = errors.New("job: not found")
    ErrDuplicateName    = errors.New("job: duplicate name")
    ErrInvalidDefinition= errors.New("job: invalid definition")
    ErrAlreadyClosed    = errors.New("job: workflow already closed")
    ErrNoSchedule       = errors.New("job: no schedule configured")
    ErrScheduleNotFound = errors.New("job: schedule not found")
    ErrNotRegistered    = errors.New("job: register not configured")
)
```

### SDK translation table

Verified against `go.temporal.io/api/serviceerror` (v1.50):

| Temporal SDK error | Maps to (via `errors.Is`) |
|---|---|
| `*serviceerror.NotFound` (workflow execution or namespace) | `ErrNotFound` |
| `*serviceerror.AlreadyExists` (e.g., `WorkflowExecutionAlreadyStartedError`) | passed through wrapped |
| `*serviceerror.CancellationAlreadyRequested` | passed through wrapped |
| (others) | passed through wrapped with operation context |

There is no dedicated SDK error type for "workflow already completed" — when calling `Cancel` / `Terminate` / `Signal` on a closed workflow, Temporal returns `*serviceerror.NotFound` (because the visibility query treats closed executions as not-active). `ErrAlreadyClosed` is therefore reserved for cases where we can detect closure via a preceding `Describe` (optional helper, not the default `Cancel`/`Terminate` path).

All translations preserve the SDK error via `fmt.Errorf("op: %w", sdkErr)` so consumers can `errors.As` to deeper types.

### Validation

`Registry.Add(d)` validates `Name`, `TaskQueue`, all three closures non-nil, and `Schedule.valid()` if set. Builders must produce complete Definitions; partial values fail Add.

---

## Testing Strategy

### `pkg/temporal/job` — unit + integration

**Unit (no Temporal):** `registry_test.go`, `schedule_spec_test.go`, `status_test.go`, `errors_test.go`, `definition_test.go` (validation/wiring contract).

**Integration (`//go:build integration`, real Temporal container via existing `pkg/temporal/testcontainer.Setup`):**
- `definition_integration_test.go` — full lifecycle (Build → Register → Execute → Describe → Cancel).
- `schedule_integration_test.go` — Apply → Describe → Pause → Resume → Trigger → Delete.
- `registry_integration_test.go` — multi-Definition register-all with shared workflow types.
- `dedup_integration_test.go` — `RegisterWorkflowOnce` mechanic verified end-to-end.

### `go-wf` builder tests

Each builder gains:
- Unit: `Build_ProducesDefinition` (name/taskqueue/schedule mirrored correctly), validation tests.
- Integration: `Build_DefinitionExecuteRoundTrip` — workflow runs end-to-end.

Existing `datasync/chunk/sync_integration_test.go` continues to work after the type swap.

### Coverage targets

`pkg/temporal/job` ≥ 90%. go-wf builder packages: existing targets continue.

---

## Migration plan

Single-commit migration acceptable (no backward compat). Order within the PR:

1. Add `pkg/temporal/job` package (no dependents yet).
2. Update `pkg` examples / docs that reference workflow management to mention the new package.
3. Update `go-wf` builders: container, function, datasync, datasync/chunk all converge on `*job.Definition`.
4. Delete `FullJobRegistration` and `BuildJobRegistration`.
5. Update `go-wf/examples/datasync/*.go` and `go-wf/examples/datasync/chunk_basic.go` to use `*job.Definition`.
6. Update para-sync (out of scope for this PR, tracked separately).

Both repos updated in coordinated PRs (one in `pkg`, one in `go-wf`). The `pkg` PR ships first; go-wf then bumps its `pkg/v2` dependency and lands the consumer-side changes.

---

## Future Work (explicitly out of scope)

- Temporal Update API support.
- Visibility-level batch operations (e.g., bulk cancel by query).
- Default HTTP / gRPC handler set built atop the registry (consumers build this themselves for now).
- A CLI that consumes a `Registry` (could be a separate `pkg/temporal/jobcli` package).
- Tracing/metrics auto-instrumentation per Definition operation (the OTel hooks already exist on the underlying client; this layer just passes calls through).
