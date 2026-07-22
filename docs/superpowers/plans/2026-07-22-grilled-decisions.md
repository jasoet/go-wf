# Grilled Decisions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the 6 design decisions from the 2026-07-22 grilling session: per-builder activity options, worker versioning, secret references, storage convergence, pkg/v2 release discipline, and an enforced 85% coverage floor.

**Architecture:** All changes are additive except Phase 4 (storage convergence), which deletes `workflow/artifacts` after rehoming its unique capabilities (archive helpers, file upload/download activities) into `workflow/store`. Activity options travel inside the existing input structs (the only channel that crosses the workflow boundary). Secret references resolve worker-side in activities via a `secret://` value prefix. Worker versioning is an opt-in env-gated helper in a new `worker` package.

**Tech Stack:** Go 1.26, Temporal SDK v1.41.1 (`worker.DeploymentOptions` API), testcontainers, Task, golangci-lint.

## Global Constraints

- Conventional commits (`feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`); semantic-release derives versions from them.
- Tests: table-driven; unit tests have no build tag, integration tests use `//go:build integration`, examples use `//go:build example`.
- Commands: `task test:unit` (fast), `task test` (all + coverage, needs Docker), `task lint`, `task fmt`.
- Run `task fmt` before every commit. Zero golangci-lint errors. Line length < 190.
- `go.temporal.io/sdk v1.41.1` — use `worker.DeploymentOptions` (NOT the deprecated `BuildID` field).
- Do not change `datasync/builder` or `datasync/chunk` activity-option knobs — they already have them (precedent).
- Do not wire the dead `ExtendedContainerInput.Secrets` field; the secret mechanism is the `secret://` value prefix (Phase 3).
- Non-goals: DAG-mode activity options, `ExecuteTaskWorkflow` options, Encrypting DataConverter, moving `job.Definition` out of pkg/v2.

---

## Phase 1 — Per-builder ExecutionOptions (Decision: Q2)

### Task 1.1: Generic core — `ExecutionOptions` type + resolver

**Files:**
- Modify: `workflow/types.go` (add type + field on 4 input structs)
- Modify: `workflow/helpers.go:23-34` (add `ResolveActivityOptions`)
- Test: `workflow/helpers_test.go`

**Interfaces:**
- Produces: `workflow.ExecutionOptions{StartToCloseTimeout time.Duration; RetryPolicy *temporal.RetryPolicy}`; `workflow.ResolveActivityOptions(opts *ExecutionOptions) wf.ActivityOptions`. Fields `Options *ExecutionOptions` with json tag `options,omitempty` on `PipelineInput`, `ParallelInput`, `LoopInput`, `ParameterizedLoopInput`.

- [ ] **Step 1: Write the failing test** (append to `workflow/helpers_test.go`)

```go
func TestResolveActivityOptions(t *testing.T) {
	tests := []struct {
		name string
		opts *ExecutionOptions
		want time.Duration
	}{
		{"nil keeps default", nil, 10 * time.Minute},
		{"override timeout", &ExecutionOptions{StartToCloseTimeout: 45 * time.Minute}, 45 * time.Minute},
		{"zero timeout keeps default", &ExecutionOptions{}, 10 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ao := ResolveActivityOptions(tt.opts)
			assert.Equal(t, tt.want, ao.StartToCloseTimeout)
			require.NotNil(t, ao.RetryPolicy)
		})
	}

	t.Run("retry policy override", func(t *testing.T) {
		rp := &temporal.RetryPolicy{MaximumAttempts: 5}
		ao := ResolveActivityOptions(&ExecutionOptions{RetryPolicy: rp})
		assert.Equal(t, int32(5), ao.RetryPolicy.MaximumAttempts)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./workflow/ -run TestResolveActivityOptions -v`
Expected: FAIL — `undefined: ResolveActivityOptions`

- [ ] **Step 3: Implement**

In `workflow/types.go` add (after the existing imports, `temporal "go.temporal.io/sdk/temporal"` is already imported in helpers.go — add to types.go imports if missing):

```go
// ExecutionOptions overrides the default Temporal activity options for an orchestration run.
// It travels inside the orchestration input so builders can control activity behavior
// without changing workflow wrapper signatures.
type ExecutionOptions struct {
	// StartToCloseTimeout caps a single activity attempt. Zero keeps the default (10m).
	StartToCloseTimeout time.Duration `json:"start_to_close_timeout,omitempty"`
	// RetryPolicy replaces the default retry policy (3 attempts, 2.0 backoff) when set.
	RetryPolicy *temporal.RetryPolicy `json:"retry_policy,omitempty"`
}
```

Add `Options *ExecutionOptions \`json:"options,omitempty"\`` to `PipelineInput[I, O]` (types.go:14), `ParallelInput[I, O]` (:42), `LoopInput[I, O]` (:72), `ParameterizedLoopInput[I, O]` (:91).

In `workflow/helpers.go` after `DefaultActivityOptions`:

```go
// ResolveActivityOptions returns DefaultActivityOptions() with any non-nil overrides applied.
func ResolveActivityOptions(opts *ExecutionOptions) wf.ActivityOptions {
	ao := DefaultActivityOptions()
	if opts == nil {
		return ao
	}
	if opts.StartToCloseTimeout > 0 {
		ao.StartToCloseTimeout = opts.StartToCloseTimeout
	}
	if opts.RetryPolicy != nil {
		ao.RetryPolicy = opts.RetryPolicy
	}
	return ao
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./workflow/ -run TestResolveActivityOptions -v`
Expected: PASS

- [ ] **Step 5: Wire the core workflows**

Replace `DefaultActivityOptions()` with `ResolveActivityOptions(input.Options)` at:
- `workflow/pipeline.go:23`
- `workflow/parallel.go:19`
- `workflow/loop.go:21` and `workflow/loop.go:93`

(`workflow/execute.go` is untouched — single-task inputs have no Options field.)

Run: `go test ./workflow/...`
Expected: PASS (existing tests unaffected — nil Options preserves behavior)

- [ ] **Step 6: Commit**

```bash
task fmt && git add workflow/ && git commit -m "feat(workflow): add ExecutionOptions overrides to generic orchestration inputs"
```

### Task 1.2: Container payload twins + wrappers

**Files:**
- Modify: `container/payload/payloads.go` (`PipelineInput`:107, `ParallelInput`:122, `LoopInput`:162, `ParameterizedLoopInput`:181)
- Modify: `container/workflow/*.go` wrappers (pipeline, parallel, loop) — copy `Options` into the generic input
- Test: `container/workflow/*_test.go` (follow existing wrapper test patterns)

**Interfaces:**
- Consumes: `workflow.ExecutionOptions` from Task 1.1.
- Produces: `Options *generic.ExecutionOptions` field (json `options,omitempty`) on the four container payload input structs; wrappers forward it.

- [ ] **Step 1: Add fields + forward in wrappers.** The wrapper edit pattern (from `container/workflow/pipeline.go`) is:

```go
genericInput := generic.PipelineInput[*payload.ContainerExecutionInput, payload.ContainerExecutionOutput]{
	Tasks:       toTaskPtrs(input.Containers),
	StopOnError: input.StopOnError,
	Cleanup:     input.Cleanup,
	Options:     input.Options,
}
```

Apply the equivalent one-line addition in the parallel and loop wrappers.

- [ ] **Step 2: Add a wrapper test per modified wrapper** asserting `input.Options` reaches the generic input (existing wrapper tests use the Temporal testsuite with mocked activities — mirror the nearest existing test, e.g. `container/workflow/loop_test.go`, setting `Options` and asserting the workflow still completes).

- [ ] **Step 3: Run + commit**

Run: `go test ./container/...`
Expected: PASS

```bash
task fmt && git add container/ && git commit -m "feat(container): forward ExecutionOptions through container payload inputs and wrappers"
```

### Task 1.3: Builders — option, RunTimeout derivation, Build()-time validation

**Files:**
- Modify: `container/builder/builder.go` (WorkflowBuilder:179, Build:475; LoopBuilder Build:750)
- Modify: `container/builder/options.go` (new `BuilderOption`)
- Modify: `function/builder/builder.go` (WorkflowBuilder:44, Build:202; LoopBuilder Build:453)
- Modify: `function/builder/options.go`
- Test: `container/builder/builder_test.go`, `function/builder/builder_test.go`

**Interfaces:**
- Produces: fluent `WithExecutionOptions(opts *generic.ExecutionOptions)` on both WorkflowBuilders (and container/function LoopBuilders via the same mechanism); container `BuilderOption` `WithExecutionOptions(opts)`; derivation rule `StartToClose = maxRunTimeout + 2m` when no explicit options and any `RunTimeout > 0`; Build() error when explicit `StartToCloseTimeout <= maxRunTimeout`.

- [ ] **Step 1: Write failing builder tests**

Container (`container/builder/builder_test.go`):

```go
func TestWorkflowBuilder_ExecutionOptions(t *testing.T) {
	t.Run("derives StartToClose from RunTimeout with margin", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("derive").Single().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 45 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(*payload.ContainerExecutionInput)
		require.True(t, ok)
		_ = in // single mode: options live on the wrapper input; see pipeline case
	})

	t.Run("pipeline input carries derived options", func(t *testing.T) {
		def, err := NewWorkflowBuilder().
			Name("derive-pipe").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 45 * time.Minute}).
			Build()
		require.NoError(t, err)
		in, ok := def.NewInput().(*payload.PipelineInput)
		require.True(t, ok)
		require.NotNil(t, in.Options)
		assert.Equal(t, 47*time.Minute, in.Options.StartToCloseTimeout)
	})

	t.Run("explicit StartToClose below max RunTimeout fails Build", func(t *testing.T) {
		_, err := NewWorkflowBuilder(WithExecutionOptions(&generic.ExecutionOptions{StartToCloseTimeout: 5 * time.Minute})).
			Name("bad").Pipeline().
			AddInput(payload.ContainerExecutionInput{Image: "alpine", RunTimeout: 10 * time.Minute}).
			Build()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start_to_close_timeout")
	})
}
```

Function (`function/builder/builder_test.go`): analogous test that `WithExecutionOptions(...)` lands on the produced `*workflow.PipelineInput[I, O].Options`.

- [ ] **Step 2: Run to verify fail** — `go test ./container/builder/ ./function/builder/ -run ExecutionOptions` → FAIL.

- [ ] **Step 3: Implement**

container/builder/builder.go — add field `executionOptions *generic.ExecutionOptions` to `WorkflowBuilder`; fluent:

```go
// WithExecutionOptions sets Temporal activity options for the built workflow.
// When nil and any container sets RunTimeout, Build derives
// StartToCloseTimeout = max(RunTimeout) + 2 minutes.
func (b *WorkflowBuilder) WithExecutionOptions(opts *generic.ExecutionOptions) *WorkflowBuilder {
	b.executionOptions = opts
	return b
}
```

In `Build()`, before capturing inputs:

```go
const runTimeoutMargin = 2 * time.Minute

maxRun := time.Duration(0)
for _, c := range b.containers {
	if c.RunTimeout > maxRun {
		maxRun = c.RunTimeout
	}
}
opts := b.executionOptions
if opts == nil && maxRun > 0 {
	opts = &generic.ExecutionOptions{StartToCloseTimeout: maxRun + runTimeoutMargin}
}
if opts != nil && maxRun > 0 && opts.StartToCloseTimeout > 0 && opts.StartToCloseTimeout <= maxRun {
	b.errors = append(b.errors, fmt.Errorf("start_to_close_timeout (%s) must exceed max container run_timeout (%s)", opts.StartToCloseTimeout, maxRun))
}
```

Then set `Options: opts` on each `payload.PipelineInput` / `ParallelInput` / loop input captured in `newInputFn`. Repeat for `LoopBuilder`. In `container/builder/options.go` add the `BuilderOption` twin (mirrors `WithGlobalTimeout` at options.go:91).

function/builder: add `executionOptions *workflow.ExecutionOptions` field + same fluent; set `Options: b.executionOptions` on produced generic inputs; add `WithExecutionOptions[I, O]` to options.go mirroring `WithStopOnError` (options.go:15). No derivation (function inputs have no enforced timeout).

- [ ] **Step 4: Run to verify pass** — same command → PASS. Then `go test ./container/... ./function/...` all green.

- [ ] **Step 5: Commit**

```bash
task fmt && git add container/builder function/builder && git commit -m "feat(builder): per-builder ExecutionOptions with RunTimeout-derived StartToClose and Build-time validation"
```

---

## Phase 2 — Worker Versioning helper (Decision: Q3)

### Task 2.1: `worker` package

**Files:**
- Create: `worker/worker.go`
- Test: `worker/worker_test.go`

**Interfaces:**
- Produces: `worker.VersionedOptions(opts worker.Options, deploymentName, buildID string) worker.Options`; `worker.OptionsFromEnv(opts worker.Options) worker.Options` (env `TEMPORAL_DEPLOYMENT_NAME`, `TEMPORAL_BUILD_ID`); `worker.New(c client.Client, taskQueue string, opts worker.Options) worker.Worker`.

- [ ] **Step 1: Write the failing test** (`worker/worker_test.go`)

```go
package worker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	temporalwf "go.temporal.io/sdk/workflow"
	sdkworker "go.temporal.io/sdk/worker"
)

func TestVersionedOptions(t *testing.T) {
	opts := VersionedOptions(sdkworker.Options{}, "go-wf", "abc123")
	require.True(t, opts.DeploymentOptions.UseVersioning)
	assert.Equal(t, "go-wf", opts.DeploymentOptions.Version.DeploymentName)
	assert.Equal(t, "abc123", opts.DeploymentOptions.Version.BuildID)
	assert.Equal(t, temporalwf.VersioningBehaviorPinned, opts.DeploymentOptions.DefaultVersioningBehavior)
}

func TestOptionsFromEnv(t *testing.T) {
	t.Run("unset env leaves options untouched", func(t *testing.T) {
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.False(t, opts.DeploymentOptions.UseVersioning)
	})
	t.Run("both vars enable versioning", func(t *testing.T) {
		t.Setenv("TEMPORAL_DEPLOYMENT_NAME", "go-wf")
		t.Setenv("TEMPORAL_BUILD_ID", "deadbeef")
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.True(t, opts.DeploymentOptions.UseVersioning)
		assert.Equal(t, "deadbeef", opts.DeploymentOptions.Version.BuildID)
	})
	t.Run("only one var set leaves options untouched", func(t *testing.T) {
		t.Setenv("TEMPORAL_BUILD_ID", "deadbeef")
		opts := OptionsFromEnv(sdkworker.Options{})
		assert.False(t, opts.DeploymentOptions.UseVersioning)
	})
}
```

- [ ] **Step 2: Run to verify fail** — `go test ./worker/` → FAIL (no package).

- [ ] **Step 3: Implement** (`worker/worker.go`)

```go
// Package worker provides helpers for creating Temporal workers with go-wf
// conventions, including opt-in Worker Versioning (Worker Deployments).
package worker

import (
	"os"

	"go.temporal.io/sdk/client"
	temporalwf "go.temporal.io/sdk/workflow"
	sdkworker "go.temporal.io/sdk/worker"
)

// Environment variables enabling Worker Versioning. Both must be set.
const (
	// EnvDeploymentName names the Temporal Worker Deployment (e.g. "go-wf").
	EnvDeploymentName = "TEMPORAL_DEPLOYMENT_NAME"
	// EnvBuildID identifies this build of the worker (e.g. a git SHA).
	EnvBuildID = "TEMPORAL_BUILD_ID"
)

// VersionedOptions returns opts with Worker Versioning enabled. The default
// versioning behavior is Pinned: an execution stays on the worker build it
// started with, so deploying new workflow code never breaks in-flight replays.
func VersionedOptions(opts sdkworker.Options, deploymentName, buildID string) sdkworker.Options {
	opts.DeploymentOptions = sdkworker.DeploymentOptions{
		UseVersioning: true,
		Version: sdkworker.WorkerDeploymentVersion{
			DeploymentName: deploymentName,
			BuildID:        buildID,
		},
		DefaultVersioningBehavior: temporalwf.VersioningBehaviorPinned,
	}
	return opts
}

// OptionsFromEnv returns VersionedOptions when both EnvDeploymentName and
// EnvBuildID are set; otherwise it returns opts unchanged.
func OptionsFromEnv(opts sdkworker.Options) sdkworker.Options {
	name, id := os.Getenv(EnvDeploymentName), os.Getenv(EnvBuildID)
	if name == "" || id == "" {
		return opts
	}
	return VersionedOptions(opts, name, id)
}

// New creates a Temporal worker on taskQueue. When TEMPORAL_DEPLOYMENT_NAME
// and TEMPORAL_BUILD_ID are set, Worker Versioning is enabled automatically.
func New(c client.Client, taskQueue string, opts sdkworker.Options) sdkworker.Worker {
	return sdkworker.New(c, taskQueue, OptionsFromEnv(opts))
}
```

- [ ] **Step 4: Run to verify pass** — `go test ./worker/` → PASS.

- [ ] **Step 5: Commit** — `task fmt && git add worker/ && git commit -m "feat(worker): env-gated Worker Versioning helper (Worker Deployments, pinned behavior)"`

### Task 2.2: Adopt in example workers + compose + docs

**Files:**
- Modify: `examples/container/worker/main.go:35`, `examples/function/worker/main.go:44`, `examples/datasync/worker/main.go:145-157` — replace `worker.New(` with `gowfworker.New(` (import `gowfworker "github.com/jasoet/go-wf/worker"`; alias the SDK import as today)
- Modify: `compose.yml` — add `TEMPORAL_DEPLOYMENT_NAME: go-wf` and `TEMPORAL_BUILD_ID: ${BUILD_ID:-dev}` to `container-worker`, `function-worker`, `datasync-worker` env blocks
- Modify: `docs/architecture.md` (new "Workflow Versioning" section under "How Temporal Is Used"), `docs/getting-started.md` (one paragraph)

- [ ] **Step 1: Apply the three worker edits** (mechanical; keep the existing `worker.Options{MaxConcurrent...}` values, just route through the helper).

- [ ] **Step 2: Verify examples compile** — `go build -tags example ./examples/...` → success.

- [ ] **Step 3: Verify the compose stack supports Worker Deployments** — `task local:start`, then `task local:stop`. If workers crash with versioning errors, bump `temporalio/auto-setup` in compose.yml to a version supporting Worker Deployments and retest; if still failing, revert only the compose.yml env lines (helper stays env-gated and harmless) and note it in the commit body.

- [ ] **Step 4: Docs** — architecture.md section:

```markdown
### Workflow Versioning

Workers created via `github.com/jasoet/go-wf/worker`.New enable Temporal Worker
Versioning (Worker Deployments) when `TEMPORAL_DEPLOYMENT_NAME` and
`TEMPORAL_BUILD_ID` are set. The default behavior is Pinned: in-flight
executions finish on the worker build they started with, so workflow-code
deploys never break replay. Without the env vars, workers behave exactly as
`worker.New` from the Temporal SDK.
```

- [ ] **Step 5: Commit** — `task fmt && git add examples/ compose.yml docs/ && git commit -m "feat(examples): adopt worker versioning helper in example workers and compose stack"`

---

## Phase 3 — Secret references (Decision: Q4)

### Task 3.1: `workflow/secrets` package

**Files:**
- Create: `workflow/secrets/secrets.go`
- Test: `workflow/secrets/secrets_test.go`

**Interfaces:**
- Produces: `secrets.RefPrefix = "secret://"`; `secrets.Resolver` interface + `ResolverFunc`; `secrets.EnvResolver(prefix string) Resolver`; `secrets.SetDefault(r Resolver)`; `secrets.Resolve(ctx, value string) (string, error)`; `secrets.ResolveMap(ctx, map[string]string) (map[string]string, error)`.

- [ ] **Step 1: Write the failing test**

```go
package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	ctx := context.Background()
	t.Run("plain value passes through", func(t *testing.T) {
		v, err := Resolve(ctx, "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", v)
	})
	t.Run("secret ref resolves from default env resolver", func(t *testing.T) {
		t.Setenv("SECRET_PGPASS", "s3cr3t")
		v, err := Resolve(ctx, "secret://PGPASS")
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", v)
	})
	t.Run("missing ref errors", func(t *testing.T) {
		_, err := Resolve(ctx, "secret://NOPE_MISSING")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NOPE_MISSING")
	})
}

func TestResolveMap(t *testing.T) {
	t.Setenv("SECRET_TOKEN", "tok")
	out, err := ResolveMap(context.Background(), map[string]string{
		"PLAIN": "x", "AUTH": "secret://TOKEN",
	})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"PLAIN": "x", "AUTH": "tok"}, out)
	// input map not mutated
}

func TestSetDefault(t *testing.T) {
	defer SetDefault(EnvResolver("SECRET_"))
	SetDefault(ResolverFunc(func(_ context.Context, ref string) (string, error) {
		return "custom-" + ref, nil
	}))
	v, err := Resolve(context.Background(), "secret://A")
	require.NoError(t, err)
	assert.Equal(t, "custom-A", v)
}
```

- [ ] **Step 2: Run to verify fail** — `go test ./workflow/secrets/` → FAIL.

- [ ] **Step 3: Implement** (`workflow/secrets/secrets.go`)

```go
// Package secrets resolves secret references worker-side so plaintext secrets
// never enter Temporal workflow history. Payloads carry references such as
// "secret://PGPASS"; activities resolve them at runtime via a Resolver.
package secrets

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// RefPrefix marks a payload value as a secret reference.
const RefPrefix = "secret://"

// Resolver resolves a secret reference (the part after "secret://") to its value.
type Resolver interface {
	Resolve(ctx context.Context, ref string) (string, error)
}

// ResolverFunc adapts a function to Resolver.
type ResolverFunc func(ctx context.Context, ref string) (string, error)

// Resolve implements Resolver.
func (f ResolverFunc) Resolve(ctx context.Context, ref string) (string, error) { return f(ctx, ref) }

// EnvResolver resolves refs from environment variables: ref "PGPASS" reads
// the variable prefix+"PGPASS" (default prefix "SECRET_").
func EnvResolver(prefix string) Resolver {
	return ResolverFunc(func(_ context.Context, ref string) (string, error) {
		v, ok := os.LookupEnv(prefix + ref)
		if !ok {
			return "", fmt.Errorf("secret %q not found (env %s%s)", ref, prefix, ref)
		}
		return v, nil
	})
}

var defaultResolver Resolver = EnvResolver("SECRET_")

// SetDefault replaces the process-wide resolver used by activities. Pass nil
// to restore the built-in SECRET_-prefixed env resolver.
func SetDefault(r Resolver) {
	if r == nil {
		defaultResolver = EnvResolver("SECRET_")
		return
	}
	defaultResolver = r
}

// Resolve returns value unchanged unless it has the secret:// prefix, in which
// case the reference is resolved via the default resolver.
func Resolve(ctx context.Context, value string) (string, error) {
	if !strings.HasPrefix(value, RefPrefix) {
		return value, nil
	}
	return defaultResolver.Resolve(ctx, strings.TrimPrefix(value, RefPrefix))
}

// ResolveMap resolves every value in m, returning a new map; m is not mutated.
func ResolveMap(ctx context.Context, m map[string]string) (map[string]string, error) {
	if len(m) == 0 {
		return m, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		rv, err := Resolve(ctx, v)
		if err != nil {
			return nil, fmt.Errorf("key %s: %w", k, err)
		}
		out[k] = rv
	}
	return out, nil
}
```

- [ ] **Step 4: Run to verify pass** — `go test ./workflow/secrets/` → PASS.

- [ ] **Step 5: Commit** — `task fmt && git add workflow/secrets/ && git commit -m "feat(secrets): worker-side secret reference resolution (secret:// prefix)"`

### Task 3.2: Wire into container + function activities; docs

**Files:**
- Modify: `container/activity/container.go:51-53` — resolve `input.Env` before `dockerpkg.WithEnvMap`
- Modify: `function/activity/function.go:46-51` — resolve `input.Env` before building `fnInput`
- Create: `docs/security.md` (trust boundary + secret references)
- Modify: `README.md` (link to docs/security.md; adjust the Quick Start `POSTGRES_PASSWORD` example comment), `container/README.md:12` (fix the false "secrets" claim to reference `secret://` refs)

**Interfaces:**
- Consumes: `secrets.ResolveMap`.

- [ ] **Step 1: Write failing activity tests** — in `container/activity/container_test.go` and the function activity tests, add a case where `Env` contains `secret://X`, the default resolver is stubbed via `secrets.SetDefault`, and assert the resolved value reaches the executor/handler (follow the existing mock patterns in those files; reset with `defer secrets.SetDefault(nil)`).

- [ ] **Step 2: Run to verify fail.**

- [ ] **Step 3: Implement** — container/activity/container.go:

```go
	if len(input.Env) > 0 {
		env, err := secrets.ResolveMap(ctx, input.Env)
		if err != nil {
			return &payload.ContainerExecutionOutput{
				Name: input.Name, StartedAt: startTime, FinishedAt: time.Now(),
				Success: false, Error: err.Error(),
			}, err
		}
		opts = append(opts, dockerpkg.WithEnvMap(env))
	}
```

function/activity/function.go: resolve `input.Env` the same way before `fnInput := fn.FunctionInput{...}`; a resolution failure is an infrastructure error (return Go error).

- [ ] **Step 4: Run to verify pass** — `go test ./container/activity/ ./function/activity/` → PASS.

- [ ] **Step 5: Write docs/security.md** covering: (1) trust boundary — anyone who can submit to a task queue can run code on the worker host (arbitrary images/commands); volume denylist exists (`container/payload/payloads.go` `ValidateVolumes`) but is defense-in-depth, not a sandbox; (2) workflow inputs persist in Temporal history forever — never put plaintext secrets in `Env`, `Args`, `Data`, HTTP template headers, or schedule inputs; use `secret://NAME` values resolved worker-side (default env `SECRET_NAME`, override via `secrets.SetDefault`); (3) datasync already resolves sources/sinks worker-side by name — keep credentials there.

- [ ] **Step 6: Commit** — `task fmt && git add container/activity function/activity docs/security.md README.md container/README.md && git commit -m "feat(activity): resolve secret:// env references worker-side; document trust boundary"`

---

## Phase 4 — Converge artifacts into store (Decision: Q5)

### Task 4.1: Rehome archive + file operations into `workflow/store`

**Files:**
- Create: `workflow/store/archive.go` (move `ArchiveDirectory`, `ExtractArchive` from `workflow/artifacts/local.go:188,258`, keep 1GB caps and traversal sanitization verbatim)
- Create: `workflow/store/fileops.go`
- Test: `workflow/store/archive_test.go`, `workflow/store/fileops_test.go` (port the unique cases from `workflow/artifacts/local_test.go`: symlink skip, traversal rejection, invalid gzip, file/dir/archive round-trip, streaming upload, missing key)

**Interfaces:**
- Produces:
  - `store.ArchiveDirectory(sourceDir string, writer io.Writer) error`
  - `store.ExtractArchive(reader io.Reader, destDir string) error`
  - `store.UploadFile(ctx context.Context, raw RawStore, key, sourcePath, typ string) error` — typ `"file"` uploads the file; `"directory"`/`"archive"` streams tar.gz via io.Pipe; enforces `MaxUploadSize` (1GB, already in local.go:14)
  - `store.DownloadFile(ctx context.Context, raw RawStore, key, destPath, typ string) error` — `"file"` writes the file; `"directory"`/`"archive"` extracts
  - `store.DeletePrefix(ctx context.Context, raw RawStore, prefix string) error` — List + Delete each (replaces `CleanupWorkflowArtifacts`)

- [ ] **Step 1: Port the tests** (they fail — package funcs don't exist yet).
- [ ] **Step 2: Run to verify fail** — `go test ./workflow/store/` → FAIL.
- [ ] **Step 3: Implement** — copy the archive helpers adjusting package name; implement fileops on `RawStore` using the same control flow as `workflow/artifacts/activities.go:37-180` but with string keys instead of `ArtifactMetadata`.
- [ ] **Step 4: Run to verify pass** — `go test ./workflow/store/` → PASS.
- [ ] **Step 5: Commit** — `task fmt && git add workflow/store/ && git commit -m "feat(store): add archive helpers and file upload/download operations on RawStore"`

### Task 4.2: Migrate container + function DAG payloads and workflows

**Files:**
- Modify: `container/payload/payloads_extended.go:8,187` — `ArtifactStore store.RawStore \`json:"-"\`` (import `github.com/jasoet/go-wf/workflow/store`)
- Modify: `function/payload/payload_extended.go:8,71,74,90` — same field swap; define `ArtifactRef` locally (copy from `workflow/artifacts/store.go:117`: `Name`, `Path`, `Type` oneof `file directory archive bytes`, `Optional`)
- Modify: `container/workflow/dag.go:237,293-307` — build keys with `store.NewKeyBuilder().WithWorkflow(id).WithRun(runID).WithStep(step).WithName(name).Build()`; replace `artifacts.DownloadArtifactActivity`/`UploadArtifactActivity` calls with activity closures calling `store.DownloadFile`/`store.UploadFile` (keep the existing ExecuteActivity vs local-activity shape per file — function DAG at `function/workflow/dag.go:321-420` uses local activities; `"bytes"` type becomes `store.NewBytesStore(raw).Load(ctx, key)` / `.Save(ctx, key, data)`)
- Test: `function/workflow/dag_test.go:382-475` — fixtures use `fnpayload.ArtifactRef` and `store.NewLocalStore(t.TempDir())`
- Test: `container/workflow/dag_test.go` — update any artifacts imports likewise

**Interfaces:**
- Consumes: store APIs from Task 4.1.
- Produces: `DAGWorkflowInput.ArtifactStore store.RawStore` in both payload packages; `function/payload.ArtifactRef`.

- [ ] **Step 1: Update the payload structs + dag.go call sites** (both modules).
- [ ] **Step 2: Update the DAG tests** to the new types.
- [ ] **Step 3: Run** — `go test ./container/... ./function/...` → PASS.
- [ ] **Step 4: Commit** — `task fmt && git add container/ function/ && git commit -m "refactor(dag): migrate DAG artifact storage from workflow/artifacts to workflow/store"`

### Task 4.3: Migrate examples, delete `workflow/artifacts`, clean references

**Files:**
- Modify: `examples/container/artifacts.go` (`NewLocalFileStore`→`store.NewLocalStore`, `artifacts.S3Config`→`store.S3Config`, drop the dead `ArtifactConfig` printf), `examples/function/worker/main.go:401-440` (store constructors; `newArtifactDAGWorkflow` closure types)
- Delete: `workflow/artifacts/` (whole directory)
- Modify: `.github/workflows/integration-test.yml:56` — drop `./workflow/artifacts/...` from the `store)` package list
- Modify: `INSTRUCTION.md:46,71,145`, `docs/architecture.md:49` ("Legacy artifact store" line → describe store fileops), any docs mentioning `workflow/artifacts`

- [ ] **Step 1: Migrate examples; verify** — `go build -tags example ./examples/...` → success.
- [ ] **Step 2: Delete the package** — `git rm -r workflow/artifacts`; `grep -rn "workflow/artifacts" --include="*.go" .` → no matches.
- [ ] **Step 3: Update CI + docs references.**
- [ ] **Step 4: Full test run** — `task test` → green (both S3 integration suites now live under `workflow/store` only).
- [ ] **Step 5: Commit** — `task fmt && git add -A && git commit -m "refactor(store)!: remove workflow/artifacts; all artifact storage converges on workflow/store"`

---

## Phase 5 — pkg/v2 tag discipline (Decision: Q6)

### Task 5.1: Pin a tagged pkg/v2 + release gate

**Files:**
- Create: `scripts/check-pkg-version.sh`
- Modify: `go.mod` (if a suitable tag exists), `.github/workflows/release.yml` (add gate step before semantic-release), `docs/contributing.md` (one paragraph on the rule)

- [ ] **Step 1: Check available tags** — `go list -m -versions github.com/jasoet/pkg/v2`. If a tag ≥ the currently pinned commit `f8ae822218ab` exists, `go get github.com/jasoet/pkg/v2@<that-tag>` and run `go test ./...` (unit). If none, leave go.mod on the pseudo-version and note in the commit body that the gate activates once pkg/v2 tags a release containing that commit.
- [ ] **Step 2: Create `scripts/check-pkg-version.sh`** (chmod +x):

```bash
#!/usr/bin/env bash
# Ensures go-wf releases only pin tagged releases of github.com/jasoet/pkg/v2.
set -euo pipefail
version=$(go list -m -f '{{.Version}}' github.com/jasoet/pkg/v2)
# Pseudo-versions look like v2.12.1-0.20260511023026-f8ae822218ab (timestamp+sha suffix).
if [[ "$version" =~ -[0-9]{14}-[0-9a-f]{12}$ ]]; then
  echo "ERROR: github.com/jasoet/pkg/v2 is pinned to pseudo-version $version"
  echo "Tag a pkg/v2 release and 'go get github.com/jasoet/pkg/v2@latest' before releasing go-wf."
  exit 1
fi
echo "OK: github.com/jasoet/pkg/v2 pinned to tagged release $version"
```

- [ ] **Step 3: Add release gate** — in `.github/workflows/release.yml` `test` job after the Test step:

```yaml
      - name: Check pkg/v2 pin
        run: nix develop --command bash scripts/check-pkg-version.sh
```

- [ ] **Step 4: Verify the script both ways** — `bash scripts/check-pkg-version.sh` (passes if pinned to tag); temporarily `go get github.com/jasoet/pkg/v2@f8ae822218ab` → script fails → restore.
- [ ] **Step 5: Commit** — `git add scripts/ go.mod go.sum .github/workflows/release.yml docs/contributing.md && git commit -m "ci: gate releases on tagged pkg/v2 dependency"`

---

## Phase 6 — Coverage ≥85% + CI gate (Decision: Q7)

### Task 6.1: Coverage gate

**Files:**
- Create: `scripts/check-coverage.sh`
- Modify: `Taskfile.yml` (new `ci:coverage` task), `.github/workflows/ci.yml` (add gate after Test)

- [ ] **Step 1: Create `scripts/check-coverage.sh`** (chmod +x):

```bash
#!/usr/bin/env bash
# Fails if total statement coverage in the given profile is below the threshold.
set -euo pipefail
profile="${1:-output/coverage.out}"
threshold="${2:-85}"
total=$(go tool cover -func="$profile" | awk '/^total:/ {gsub(/%/, "", $3); print $3}')
echo "total coverage: ${total}% (threshold ${threshold}%)"
awk -v t="$total" -v th="$threshold" 'BEGIN { exit (t+0 >= th+0) ? 0 : 1 }'
```

- [ ] **Step 2: Taskfile** — add:

```yaml
  ci:coverage:
    desc: Fail if total coverage is below the threshold (usage: task ci:coverage -- [profile] [threshold])
    silent: true
    cmds:
      - "bash scripts/check-coverage.sh {{.CLI_ARGS}}"
```

- [ ] **Step 3: ci.yml** — append after the Test step:

```yaml
      - name: Coverage gate
        run: nix develop --command bash scripts/check-coverage.sh output/coverage.out 85
```

(Confirm the `ci:test` task writes `output/coverage.out`; if it writes another profile, point the gate at that one.)

- [ ] **Step 4: Commit** — `git add scripts/ Taskfile.yml .github/workflows/ci.yml && git commit -m "ci: enforce 85% total coverage gate"`

### Task 6.2: Earn the 85%

**Files:**
- Test: `container/builder/*_test.go` (64.1%), `function/builder/*_test.go` (64.7%), `datasync/builder/*_test.go` (67.5%), `function/workflow/*_test.go` (67.6%), `workflow/store/*_test.go` (63.7%)

- [ ] **Step 1: Measure the post-Phase-1–5 baseline** — `task test`; record per-package coverage (`go tool cover -func=output/coverage.out | sort -k3 -n`). Note: deleting `workflow/artifacts` changes the denominator.
- [ ] **Step 2: List uncovered functions per laggard package** — `go tool cover -func=output/coverage.out | awk '$3+0 < 60'`. Write table-driven tests for the uncovered builder option funcs, error paths in `Build()`, and store fileops error branches first (highest line-count wins). Follow existing test file patterns; no new mocking frameworks.
- [ ] **Step 3: Iterate until** `bash scripts/check-coverage.sh output/coverage.out 85` passes.
- [ ] **Step 4: Update README.md** — replace "85%+ coverage" claims with the measured number and note the CI gate.
- [ ] **Step 5: Commit** — `task fmt && git add -A && git commit -m "test: raise total coverage above 85% and document the enforced gate"`

---

## Final verification

- [ ] `task test` green (unit + integration, -race)
- [ ] `task lint` clean
- [ ] `bash scripts/check-coverage.sh output/coverage.out 85` passes
- [ ] `bash scripts/check-pkg-version.sh` passes (or its failure is the documented gate doing its job)
- [ ] `grep -rn "workflow/artifacts" --include="*.go" .` → no matches
