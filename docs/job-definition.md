# Job Definitions

The `pkg/v2/temporal/job` package provides `*job.Definition` — a type-focused,
transport-agnostic abstraction for one registered Temporal workflow. Every go-wf
builder produces a `*job.Definition`, and the same type is the basis for a
registry that lets consumer applications expose workflow management (list runs,
trigger by name, attach schedules, cancel, terminate) without hard-coded type
switches.

## Why This Exists

Multiple consumer projects wrap the Temporal SDK to expose workflow management.
Each re-implements the same pattern: a `map[string]workflowFn` plus a `switch`
on type name in the trigger handler. The `Definition` and `Registry` abstraction
replaces that pattern with a typed object that owns all per-job operations.

## The Type

A `*job.Definition` holds:

| Field | Purpose |
|---|---|
| `Name` | Logical job identifier; used as workflow ID prefix and for `ListRuns` scoping |
| `TaskQueue` | Temporal task queue name |
| `Description` | Optional human-readable description |
| `Tags` | Optional user-defined tags |
| `Schedule` | Optional `*job.ScheduleSpec` for recurring triggers |

These fields are read-only after construction. Internal wiring (register, execute,
newInput closures) is set via `Option` functions and is not exported.

## Temporal Client

```go
c, err := temporal.NewClient(temporal.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer c.Close()
```

`temporal.NewClient` returns two values: `(client.Client, error)`. There is no
separate closer — call `c.Close()` directly.

## Registration and Execution

```go
// Register workflows and activities on a worker (idempotent).
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)

// Get a typed zero-value of the workflow input.
input := def.NewInput()

// Start a workflow run. Workflow ID defaults to "<Name>-<uuid>".
run, err := def.Execute(ctx, c, input)
// run.WorkflowID, run.RunID

// Attach to an existing run.
handle := def.GetRun(c, workflowID, runID)
```

`def.Register(w)` is safe to call multiple times and concurrently. The
builder-supplied registration closure uses `job.RegisterWorkflowOnce` and
`job.RegisterActivityOnce` for deduplication, so multiple Definitions that share
the same underlying workflow type (e.g., multiple container Definitions on one
worker) register that type only once.

### Execute Options

```go
run, err := def.Execute(ctx, c, input,
    job.WithWorkflowID("my-custom-id"),        // override "<Name>-<uuid>"
    job.WithTimeout(10*time.Minute),            // WorkflowExecutionTimeout
    job.WithTaskTimeout(30*time.Second),        // WorkflowTaskTimeout
    job.WithRetryPolicy(&temporal.RetryPolicy{MaximumAttempts: 1}),
    job.WithMemo(map[string]any{"env": "prod"}),
)
```

## Per-Run Lifecycle

All methods accept `wfID` (workflow ID) and `runID` (run ID). Pass `runID = ""`
to target the latest run.

### Describe

```go
detail, err := def.Describe(ctx, c, workflowID, runID)
// detail.Status, detail.StartTime, detail.CloseTime, detail.HistoryLength
// detail.Memo, detail.SearchAttributes
```

### History

Returns a bounded activity-event extraction of the workflow history:

```go
hist, err := def.History(ctx, c, workflowID, runID, job.HistoryOpts{MaxEvents: 200})
for _, act := range hist.Activities {
    fmt.Printf("%s  %s  attempt=%d  duration=%s\n",
        act.Name, act.Status, act.Attempt, act.Duration)
}
if hist.Truncated {
    fmt.Println("history truncated — increase MaxEvents or reduce range")
}
```

### Cancel and Terminate

```go
// Request graceful cancellation (workflow receives CancellationError).
err = def.Cancel(ctx, c, workflowID, runID)

// Hard-stop immediately.
err = def.Terminate(ctx, c, workflowID, runID, "reason string")
```

### Signal and Query

```go
err = def.Signal(ctx, c, workflowID, runID, "pause", nil)

result, err := def.Query(ctx, c, workflowID, runID, "status")
```

## Per-Job Aggregates

These methods are scoped to runs whose workflow ID starts with `"<Name>-"`.

### ListRuns

```go
page, err := def.ListRuns(ctx, c, job.ListOpts{
    Status:   []job.Status{job.StatusRunning},
    PageSize: 50,
})
for _, run := range page.Runs {
    fmt.Printf("%s  %s  %s\n", run.WorkflowID, run.Status, run.StartTime)
}
// Paginate:
if page.NextPageToken != nil {
    nextPage, err := def.ListRuns(ctx, c, job.ListOpts{PageToken: page.NextPageToken})
}
```

### Stats

```go
stats, err := def.Stats(ctx, c, job.StatsOpts{Location: time.Local})
fmt.Printf("Running: %d  CompletedToday: %d  FailedToday: %d\n",
    stats.Running, stats.CompletedToday, stats.FailedToday)
```

## Schedules

A `*job.ScheduleSpec` describes when a workflow fires automatically. Exactly one
of `Interval`, `Cron`, or `Calendar` must be set.

```go
type ScheduleSpec struct {
    Interval  time.Duration  // e.g., 15 * time.Minute
    Cron      string         // standard cron expression
    Calendar  []CalendarSpec // fine-grained calendar rules

    Overlap       OverlapPolicy  // default: OverlapSkip
    Jitter        time.Duration
    Paused        bool
    Note          string
    CatchupWindow time.Duration
}
```

`OverlapPolicy` controls what happens when a trigger fires while a previous run is
still in flight:

| Constant | Behavior |
|---|---|
| `OverlapSkip` | Drop the new trigger (default) |
| `OverlapBufferOne` | Queue one trigger; drop further |
| `OverlapBufferAll` | Queue all triggers |
| `OverlapCancelOther` | Cancel the running execution and start new |
| `OverlapTerminateOther` | Terminate the running execution and start new |
| `OverlapAllowAll` | Allow unlimited parallel runs |

### Schedule lifecycle methods

```go
// Create or update the schedule (idempotent). Schedule ID equals def.Name.
err = def.ApplySchedule(ctx, c)

// Pause and resume.
err = def.PauseSchedule(ctx, c, "maintenance window")
err = def.ResumeSchedule(ctx, c, "maintenance complete")

// Fire an immediate run outside the normal cadence.
err = def.TriggerSchedule(ctx, c)

// Remove the schedule permanently.
err = def.DeleteSchedule(ctx, c)

// Inspect current schedule state.
detail, err := def.DescribeSchedule(ctx, c)
// detail.Paused, detail.NextRunTime, detail.LastRunTime, detail.Spec
```

## Registry

`job.Registry` maps logical job names to Definitions. Use it when a consumer
application needs to operate on jobs by name (e.g., a REST API that triggers
or cancels jobs by name).

```go
reg := job.NewRegistry(defA, defB, defC)

// Register all Definitions on a worker (idempotent).
reg.RegisterAll(w)

// Create or update all schedules.
err = reg.ApplySchedules(ctx, c)

// Look up by name.
def, ok := reg.Get("orders-sync")

// Panics if not found — for use in initialization code where absence is a bug.
def = reg.MustGet("orders-sync")

run, err := def.Execute(ctx, c, input)
```

`Add` inserts a Definition at runtime:

```go
err = reg.Add(newDef)  // returns ErrDuplicateName on conflict
```

`List()` returns all Definitions sorted alphabetically by Name.
`Names()` returns just the names.

## Building Definitions in go-wf

Every go-wf builder's `.Build()` returns `(*job.Definition, error)`:

```go
// DataSync
def, err := builder.NewSyncJobBuilder[Order, OrderRow]("orders").
    WithSource(src).WithMapper(mapper).WithSink(sink).
    WithSchedule(15 * time.Minute).
    Build()

// Chunked DataSync (date-range partitions)
def, err := chunk.NewDateChunkedSync[Order, OrderRow]("orders-chunked").
    LookBack(7*24*time.Hour).ChunkSize(24*time.Hour).
    Fetcher(fetcher).Mapper(mapper).Sink(sink).
    ScheduleEvery(15 * time.Minute).
    MaxPartitionsPerExecution(50).
    Build()

// Function workflow
def, err := function.NewFunctionBuilder().
    Name("greet-pipeline").
    Activity(activityFn).
    Pipeline().
    Add(&fn.FunctionExecutionInput{Name: "greet"}).
    Build()

// Container workflow
def, err := container.NewWorkflowBuilder().
    Name("nightly-deploy").
    Single().
    Add(deployTemplate).
    Build()
```

## Constructing a Definition Directly

Consumers without go-wf builders can construct a Definition manually using
`job.New`:

```go
def, err := job.New("my-workflow", "my-task-queue",
    job.WithRegister(func(w worker.Worker) {
        w.RegisterWorkflow(MyWorkflow)
        w.RegisterActivity(MyActivity)
    }),
    job.WithExecute(func(ctx context.Context, c client.Client, opts client.StartWorkflowOptions, in any) (client.WorkflowRun, error) {
        return c.ExecuteWorkflow(ctx, opts, MyWorkflow, in)
    }),
    job.WithNewInput(func() any { return &MyInput{} }),
    job.WithSchedule(&job.ScheduleSpec{Interval: time.Hour}),
    job.WithDescription("Processes daily orders"),
    job.WithTags("orders", "nightly"),
)
```

`job.New` validates that `name`, `taskQueue`, and all three required closures
(`WithRegister`, `WithExecute`, `WithNewInput`) are present. It also validates the
`ScheduleSpec` if one is provided.

`job.RegisterWorkflowOnce` and `job.RegisterActivityOnce` are available for
builder authors who want to write idempotent registration closures:

```go
job.WithRegister(func(w worker.Worker) {
    job.RegisterWorkflowOnce(w, "MyWorkflow", MyWorkflow, workflow.RegisterOptions{Name: "MyWorkflow"})
    job.RegisterActivityOnce(w, "MyActivity", MyActivity, activity.RegisterOptions{Name: "MyActivity"})
})
```

## See Also

- [DataSync Workflows](datasync-workflows.md) — `SyncJobBuilder`
- [Chunked Sync Workflows](chunk-workflows.md) — `DateChunkedSync` / `ChunkedSync`
- [Container Workflows](container-workflows.md) — `container.WorkflowBuilder`
- [Function Workflows](function-workflows.md) — `function.WorkflowBuilder`
- `pkg/temporal/job/doc.go` in `jasoet/pkg` — package-level godoc
