# Container Workflows

The `container/` package is a concrete implementation of go-wf's generic workflow
core that executes Docker/Podman containers as Temporal activities. It provides a
complete toolkit for defining, composing, and operating container-based workflows
-- from single container runs to complex DAG orchestrations with artifact passing
and output extraction.

For generic pattern concepts see [workflow-patterns.md](workflow-patterns.md).
For artifact storage details see [store.md](store.md).

## Package Layout

| Subpackage | Purpose |
|---|---|
| `container/payload` | All input/output structs (`ContainerExecutionInput`, `PipelineInput`, `DAGNode`, etc.) |
| `container/activity` | Temporal activity that starts a container and collects results |
| `container/workflow` | Workflow implementations (pipeline, parallel, loop, DAG, parameterized) |
| `container/builder` | Fluent builder API for constructing workflow inputs |
| `container/template` | Pre-built `WorkflowSource` implementations (Container, Script, HTTP) |
| `container/patterns` | Ready-made pattern functions (CI/CD, fan-out, matrix build, etc.) |

The top-level `container/` package re-exports commonly used types so consumers
can write `container.ContainerExecutionInput` instead of importing `container/payload`.

## Activity: Container Execution

The core activity is `activity.StartContainerActivity`. It:

1. Creates a Docker/Podman container from `ContainerExecutionInput`.
2. Starts the container and optionally waits for a readiness strategy.
3. Waits for exit, collects stdout/stderr (truncated to 1 MB), and returns
   `ContainerExecutionOutput`.

### ContainerExecutionInput

```go
type ContainerExecutionInput struct {
    Image        string            `json:"image" validate:"required"`
    Command      []string          `json:"command,omitempty"`
    Entrypoint   []string          `json:"entrypoint,omitempty"`
    Env          map[string]string `json:"env,omitempty"`
    Ports        []string          `json:"ports,omitempty"`
    Volumes      map[string]string `json:"volumes,omitempty"`
    WorkDir      string            `json:"work_dir,omitempty"`
    User         string            `json:"user,omitempty"`
    WaitStrategy WaitStrategyConfig `json:"wait_strategy,omitempty"`
    StartTimeout time.Duration     `json:"start_timeout,omitempty"`
    RunTimeout   time.Duration     `json:"run_timeout,omitempty"`
    AutoRemove   bool              `json:"auto_remove"`
    Name         string            `json:"name,omitempty"`
    Labels       map[string]string `json:"labels,omitempty"`
}
```

`Volumes` are validated against a deny-list of sensitive host paths (`/etc`,
`/proc`, `/sys`, `/dev`, Docker/Podman sockets, etc.).

### Wait Strategies

Wait strategies determine when a container is considered ready:

| Type | Config Fields | Description |
|---|---|---|
| `log` | `LogMessage`, `StartupTimeout` | Waits for a specific log line |
| `port` | `Port` | Waits for a TCP port to accept connections |
| `http` | `Port`, `HTTPPath`, `HTTPStatus` | Waits for an HTTP endpoint to return a status code |
| `healthy` | -- | Waits for the Docker health check to pass |

```go
WaitStrategyConfig{
    Type:           "log",
    LogMessage:     "ready to accept connections",
    StartupTimeout: 60 * time.Second,
}
```

### ContainerExecutionOutput

```go
type ContainerExecutionOutput struct {
    ContainerID string            `json:"container_id"`
    Name        string            `json:"name,omitempty"`
    ExitCode    int               `json:"exit_code"`
    Stdout      string            `json:"stdout,omitempty"`
    Stderr      string            `json:"stderr,omitempty"`
    Endpoint    string            `json:"endpoint,omitempty"`
    Ports       map[string]string `json:"ports,omitempty"`
    StartedAt   time.Time         `json:"started_at"`
    FinishedAt  time.Time         `json:"finished_at"`
    Duration    time.Duration     `json:"duration"`
    Success     bool              `json:"success"`
    Error       string            `json:"error,omitempty"`
}
```

`Success` is true when exit code is 0 and no error occurred. Stdout/stderr are
each capped at 1 MB to avoid oversized Temporal payloads.

## Templates

Templates implement the `builder.WorkflowSource` interface, converting
high-level configuration into a `ContainerExecutionInput`.

### ContainerTemplate

General-purpose container with functional options:

```go
c := template.NewContainer("build", "golang:1.25",
    template.WithCommand("go", "build", "-o", "app"),
    template.WithWorkDir("/workspace"),
    template.WithEnv("CGO_ENABLED", "0"),
    template.WithVolume("/host/src", "/workspace"),
    template.WithWaitForLog("Build complete"),
    template.WithAutoRemove(true))
```

### ScriptTemplate

Runs inline scripts with language-aware defaults for image and interpreter:

```go
s := template.NewBashScript("cleanup",
    `echo "Cleaning up..." && rm -rf /tmp/build-*`,
    template.WithScriptEnv("LOG_LEVEL", "debug"))
```

Supported languages with auto-selected images: `bash` (bash:5.2), `python`
(python:3.11-slim), `node` (node:20-slim), `ruby` (ruby:3.2-slim), `golang`
(golang:1.25). Convenience constructors: `NewBashScript`, `NewPythonScript`,
`NewNodeScript`, `NewRubyScript`, `NewGoScript`.

### HTTPTemplate

Makes HTTP requests via a curl container, with status code validation:

```go
h := template.NewHTTPHealthCheck("health", "https://myapp.com/health")

w := template.NewHTTPWebhook("notify",
    "https://hooks.slack.com/services/...",
    `{"text": "Deploy complete"}`)
```

Options include `WithHTTPMethod`, `WithHTTPHeader`, `WithHTTPBody`,
`WithHTTPExpectedStatus`, `WithHTTPTimeout`, `WithHTTPFollowRedirect`, and
`WithHTTPInsecure`.

## Builder API

The `container/builder` package provides a fluent API that produces a `*job.Definition`
ready for registration and execution. Using the builder is preferred over constructing
workflow inputs manually.

### WorkflowBuilder

`NewWorkflowBuilder()` takes no arguments. Set the name with `.Name(string)`.

```go
import (
    "github.com/jasoet/go-wf/v2/container/builder"
    "github.com/jasoet/pkg/v2/temporal/job"
)

def, err := builder.NewWorkflowBuilder().
    Name("ci-pipeline").
    Pipeline().          // select execution mode
    Add(buildStep).      // WorkflowSource
    Add(testStep).
    Add(deployStep).
    AddExitHandler(cleanupStep).
    StopOnError(true).
    WithTimeout(5 * time.Minute).
    WithAutoRemove(true).
    Build()              // returns (*job.Definition, error)
if err != nil {
    log.Fatal(err)
}
```

**Identity setters (required):**

| Method | Description |
|---|---|
| `.Name(string)` | Job name — also used as workflow ID prefix |
| `.TaskQueue(string)` | Override task queue (default: `"container-<name>"`) |

**Mode setters (pick exactly one before calling Build):**

| Method | Registered Temporal workflow |
|---|---|
| `.Pipeline()` | `ContainerPipelineWorkflow` — sequential, stop-on-error |
| `.Parallel()` | `ParallelContainersWorkflow` — concurrent |
| `.Single()` | `ExecuteContainerWorkflow` — single container |

**Configuration:**

`StopOnError(bool)`, `Cleanup(bool)`, `FailFast(bool)`, `MaxConcurrency(int)`,
`WithTimeout(time.Duration)`, `WithAutoRemove(bool)`.

**`.Build()` returns `(*job.Definition, error)`.** The Definition handles
worker registration and workflow execution. Consume it like this:

```go
// Register with a Temporal worker
w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)

// Execute a workflow run
run, err := def.Execute(ctx, c, def.NewInput())
```

`def.Register(w)` is idempotent — `container.RegisterAll(w)` is called
internally via `job.RegisterWorkflowOnce` / `job.RegisterActivityOnce`, so
multiple Definitions sharing the same worker will not double-register workflows
or activities.

**Lower-level raw-input helpers** (`BuildPipeline()`, `BuildParallel()`, `BuildSingle()`,
`BuildGenericPipeline()`, `BuildGenericParallel()`) remain available on
`WorkflowBuilder` for callers that only need the payload struct without a full
`job.Definition`. Prefer `Build()` for new code.

### LoopBuilder

Builds loop workflow inputs for iterating over items or parameter matrices. `Build()` returns
`(*job.Definition, error)`.

```go
def, err := builder.NewLoopBuilder([]string{"file1.csv", "file2.csv"}).
    Name("process-files").
    WithTemplate(containerTemplate).
    Parallel(true).
    MaxConcurrency(3).
    Build()
```

Or use the convenience constructor:

```go
def, err := builder.ForEach(
    []string{"file1.csv", "file2.csv"},
    containerTemplate,
).Name("process-files").Parallel(true).MaxConcurrency(3).Build()
```

For parameterized loops (cartesian product of parameters):

```go
def, err := builder.ForEachParam(
    map[string][]string{
        "env":    {"staging", "prod"},
        "region": {"us-west", "eu-central"},
    },
    deployTemplate,
).Name("multi-region-deploy").Parallel(true).FailFast(true).Build()
```

Template substitution placeholders: `{{item}}`, `{{index}}`, and `{{.paramName}}`
for parameterized loops.

`BuildLoop()` and `BuildParameterizedLoop()` return raw input structs without
a Definition, for callers that manage Temporal options manually.

### GenericBuilder

For non-container use cases, `GenericBuilder[I, O]` provides the same fluent API
over arbitrary `TaskInput`/`TaskOutput` types and returns raw input structs
(not a `*job.Definition`):

```go
gb := builder.NewGenericBuilder[*MyInput, MyOutput]()
gb.Add(input1).Add(input2)
pipeline, err := gb.BuildPipeline()
```

## Pre-built Patterns

The `container/patterns` package provides ready-to-use workflow constructors.

### CI/CD Patterns

```go
// Basic build-test-deploy pipeline
pipeline, err := patterns.BuildTestDeploy("golang:1.25", "golang:1.25", "deployer:v1")

// With post-deploy health check
pipeline, err := patterns.BuildTestDeployWithHealthCheck("golang:1.25", "deployer:v1", "https://app/health")

// With Slack notification as exit handler
pipeline, err := patterns.BuildTestDeployWithNotification("golang:1.25", "deployer:v1", webhookURL, message)

// Sequential multi-environment deployment
pipeline, err := patterns.MultiEnvironmentDeploy("deployer:v1", []string{"staging", "production"})
```

### Parallel Patterns

```go
// Fan-out/fan-in
parallel, err := patterns.FanOutFanIn("alpine:latest", []string{"task-1", "task-2"})

// Parallel data processing
parallel, err := patterns.ParallelDataProcessing("processor:v1", dataFiles, "process.sh")

// Parallel test suites with fail-fast
parallel, err := patterns.ParallelTestSuite("golang:1.25", map[string]string{
    "unit":        "go test ./internal/...",
    "integration": "go test ./tests/...",
})

// Multi-region deployment
parallel, err := patterns.ParallelDeployment("deployer:v1", []string{"us-west", "eu-central"})
```

### Loop Patterns

```go
// Parallel loop over items
loop, err := patterns.ParallelLoop(items, "processor:v1", "process.sh {{item}}")

// Sequential loop
loop, err := patterns.SequentialLoop(steps, "deployer:v1", "deploy.sh {{item}}")

// Batch processing with concurrency limit
loop, err := patterns.BatchProcessing(dataFiles, "processor:v1", 3)

// Multi-region parameterized deployment (cartesian product)
loop, err := patterns.MultiRegionDeployment(
    []string{"dev", "staging"}, []string{"us-west", "eu-central"}, "deployer:v1")

// Matrix build (all combinations of parameters)
loop, err := patterns.MatrixBuild(map[string][]string{
    "go_version": {"1.21", "1.22", "1.23"},
    "platform":   {"linux", "darwin"},
}, "builder:v1")

// ML hyperparameter sweep
loop, err := patterns.ParameterSweep(map[string][]string{
    "learning_rate": {"0.001", "0.01"},
    "batch_size":    {"32", "64"},
}, "ml-trainer:v1", 5)
```

## DAG Workflows

DAG workflows express complex dependency graphs where execution order is
determined by declared dependencies rather than position in a list.

### Defining a DAG

```go
input := payload.DAGWorkflowInput{
    Nodes: []payload.DAGNode{
        {Name: "checkout", Container: payload.ExtendedContainerInput{
            ContainerExecutionInput: payload.ContainerExecutionInput{
                Image: "alpine/git", Command: []string{"git", "clone", repoURL},
            },
        }},
        {Name: "build", Container: buildContainer, Dependencies: []string{"checkout"}},
        {Name: "unit-test", Container: unitTest, Dependencies: []string{"build"}},
        {Name: "lint", Container: lintContainer, Dependencies: []string{"build"}},
        {Name: "deploy", Container: deployContainer,
            Dependencies: []string{"unit-test", "lint"}},
    },
    FailFast: true,
}
```

Validation enforces: unique node names matching `^[a-zA-Z][a-zA-Z0-9_-]*$`, all
dependency references must exist, and no circular dependencies (detected via DFS).

### Conditional Execution

Each DAG node can have conditional behavior:

```go
Container: payload.ExtendedContainerInput{
    Conditional: &payload.ConditionalBehavior{
        When:           `{{steps.test.exitCode}} == 0`,
        ContinueOnFail: true,
    },
}
```

### Resource Limits

```go
Container: payload.ExtendedContainerInput{
    Resources: &payload.ResourceLimits{
        CPULimit:    "1000m",
        MemoryLimit: "512Mi",
        GPUCount:    1,
    },
}
```

## Data Passing

### Output Definitions

Define how to capture values from container output:

```go
Container: payload.ExtendedContainerInput{
    Outputs: []payload.OutputDefinition{
        {Name: "build_id", ValueFrom: "stdout", JSONPath: "$.build.id"},
        {Name: "version", ValueFrom: "stdout", Regex: `v(\d+\.\d+\.\d+)`},
        {Name: "exit", ValueFrom: "exitCode"},
        {Name: "config", ValueFrom: "file", Path: "/output/config.json"},
    },
}
```

`ValueFrom` options: `stdout`, `stderr`, `exitCode`, `file`. Post-extraction
filters: `JSONPath` (e.g., `$.build.id`) and `Regex` (first capture group).
Each definition supports a `Default` fallback.

### Input Mappings

Map outputs from upstream steps into downstream environment variables:

```go
Container: payload.ExtendedContainerInput{
    Inputs: []payload.InputMapping{
        {Name: "BUILD_ID", From: "build.build_id", Required: true},
        {Name: "VERSION", From: "build.version", Default: "latest"},
    },
}
```

The `From` format is `step-name.output-name`. Values are injected as environment
variables on the downstream container.

## Artifacts

DAG workflows support artifact passing between nodes via an `ArtifactStore`.

### Declaring Artifacts

```go
// Producer node
{Name: "build", Container: payload.ExtendedContainerInput{
    OutputArtifacts: []payload.Artifact{
        {Name: "binary", Path: "/output/app", Type: "file"},
    },
}}

// Consumer node
{Name: "deploy", Container: payload.ExtendedContainerInput{
    InputArtifacts: []payload.Artifact{
        {Name: "binary", Path: "/input/app", Type: "file", Optional: false},
    },
}, Dependencies: []string{"build"}}
```

### Configuring the Store

```go
input := payload.DAGWorkflowInput{
    Nodes:         nodes,
    ArtifactStore: artifacts.NewLocalFileStore("/tmp/artifacts"),
}
```

Two built-in implementations: `LocalFileStore` (local filesystem) and `S3Store`
(S3-compatible object storage). Storage keys follow the format
`workflow_id/run_id/step_name/artifact_name`. See [store.md](store.md) for
details.

## Operations API

The top-level `container` package provides functions for managing workflow
executions against a Temporal client.

### Submit and Wait

```go
// Fire and forget
status, err := container.SubmitWorkflow(ctx, temporalClient, input, "container-queue")

// Submit and block until completion
status, err := container.SubmitAndWait(ctx, temporalClient, input, "container-queue", 10*time.Minute)

// Type-safe submit with specific workflow function
status, err := container.SubmitTypedWorkflow(ctx, temporalClient,
    workflow.ExecuteContainerWorkflow, containerInput, "container-queue")

// Type-safe submit and wait with typed result
status, result, err := container.SubmitAndWaitTyped[payload.ContainerExecutionOutput](
    ctx, temporalClient, workflow.ExecuteContainerWorkflow, containerInput,
    "container-queue", 10*time.Minute)
```

`SubmitWorkflow` auto-detects the workflow function based on input type
(`ContainerExecutionInput`, `PipelineInput`, or `ParallelInput`).

### Lifecycle Management

```go
container.CancelWorkflow(ctx, client, workflowID, runID)
container.TerminateWorkflow(ctx, client, workflowID, runID, "reason")
```

### Monitoring

```go
// Poll status
status, err := container.GetWorkflowStatus(ctx, client, workflowID, runID)

// Stream updates via channel (polls every 5s)
updates := make(chan *container.WorkflowStatus)
go container.WatchWorkflow(ctx, client, workflowID, runID, updates)
for status := range updates {
    fmt.Println(status.Status)
}
```

### Signals and Queries

```go
container.SignalWorkflow(ctx, client, workflowID, runID, "pause", nil)
container.QueryWorkflow(ctx, client, workflowID, runID, "status", &result)
```

## Worker Setup

### Builder-based (preferred)

When you use `container.WorkflowBuilder` or `container.LoopBuilder`, the resulting
`*job.Definition` handles registration automatically:

```go
import (
    "github.com/jasoet/go-wf/v2/container/builder"
    "github.com/jasoet/pkg/v2/temporal"
    "go.temporal.io/sdk/worker"
)

def, err := builder.NewWorkflowBuilder().
    Name("nightly-deploy").
    Pipeline().
    Add(deployStep).
    Build()
if err != nil {
    log.Fatal(err)
}

c, err := temporal.NewClient(temporal.DefaultConfig())
if err != nil {
    log.Fatal(err)
}
defer c.Close()

w := worker.New(c, def.TaskQueue, worker.Options{})
def.Register(w)   // calls container.RegisterAll internally; idempotent
w.Run(worker.InterruptCh())
```

`def.Register(w)` calls `container.RegisterAll(w)` via idempotent helpers, so
multiple Definitions on the same worker register the shared workflow/activity
types only once.

### Manual registration

If you are dispatching workflows via the lower-level `c.ExecuteWorkflow` path:

```go
w := worker.New(temporalClient, "container-queue", worker.Options{})
container.RegisterAll(w)  // registers all workflows + instrumented activity
w.Run(worker.InterruptCh())
```

`RegisterAll` calls `RegisterWorkflows` (which registers `ExecuteContainerWorkflow`,
`ContainerPipelineWorkflow`, `ParallelContainersWorkflow`, `LoopWorkflow`,
`ParameterizedLoopWorkflow`, `DAGWorkflow`, and `WorkflowWithParameters`) and
`RegisterActivities` (which registers the OTel-instrumented `StartContainerActivity`).
`container.RegisterAll` is now safe to call multiple times — repeated calls for an
already-registered (worker, type) pair are silently ignored.

For finer control, call `RegisterWorkflows` and `RegisterActivities` separately.

See [Job Definition](job-definition.md) for the full `*job.Definition` API.
