# OpenTelemetry Instrumentation Design

**Date:** 2026-03-12
**Status:** Approved

## Overview

Add comprehensive OpenTelemetry instrumentation to go-wf using `jasoet/pkg/v2/otel`. As a library, go-wf never initializes OTel — it reads config from context and gracefully no-ops when not configured.

## Design Principles

1. **Context-based config** — OTel config flows via `otel.ContextWithConfig(ctx, cfg)`. The consuming app owns initialization; go-wf reads from context via `otel.ConfigFromContext(ctx)`.
2. **Zero overhead when disabled** — `ConfigFromContext` returns nil → all `Layers.Start*` and `GetMeter` calls produce no-ops. No panics, no allocations.
3. **Wrapper pattern** — Each package gets an `otel.go` file with instrumented wrappers. Core logic files remain untouched.
4. **Three-signal correlation** — Spans, logs (via `LayerContext.Logger`), and metrics (via exemplars) share the same trace context.

## Layer Mapping

| go-wf Concept | OTel Layer | Span Kind | Component Name |
|---|---|---|---|
| Workflow orchestration (pipeline, parallel, loop) | `Layers.StartOperations` | Internal | `"workflow"` |
| Activity execution (container, function) | `Layers.StartService` | Internal | `"docker"` / `"function"` |
| Artifact store (upload, download, list) | `Layers.StartRepository` | Client | `"artifacts"` |

## New Files

| File | Purpose |
|---|---|
| `workflow/otel.go` | Structured logging wrappers for generic orchestration |
| `docker/activity/otel.go` | Instrumented container activity + metrics |
| `function/activity/otel.go` | Instrumented function activity + metrics |
| `workflow/artifacts/otel.go` | Instrumented artifact store decorator + metrics |

No existing files are modified (except call sites in docker/function workflow layers to use instrumented wrappers).

## Section 1: Workflow Orchestration (`workflow/otel.go`)

### Constraint

Temporal workflows use `workflow.Context`, not `context.Context`. Real OTel spans cannot be created here. Instead, structured log events are emitted via Temporal's `wf.GetLogger(ctx)` with consistent fields for correlation with downstream activity spans.

### Instrumented Wrappers

**PipelineWorkflow wrapper:**
- Log: `pipeline.start` — fields: `name`, `step_count`, `stop_on_error`
- Per step: `pipeline.step.start` — fields: `step_index`, `activity_name`
- Per step: `pipeline.step.complete` — fields: `step_index`, `activity_name`, `success`, `duration`
- Log: `pipeline.complete` — fields: `name`, `total_steps`, `success_count`, `failure_count`, `duration`

**ParallelWorkflow wrapper:**
- Log: `parallel.start` — fields: `name`, `task_count`, `max_concurrency`, `failure_strategy`
- Log: `parallel.complete` — fields: `name`, `total_tasks`, `success_count`, `failure_count`, `duration`

**LoopWorkflow wrapper:**
- Log: `loop.start` — fields: `name`, `item_count`, `parallel`
- Log: `loop.complete` — fields: `name`, `iterations`, `success_count`, `failure_count`, `duration`

**ParameterizedLoopWorkflow wrapper:**
- Log: `parameterized_loop.start` — fields: `name`, `combination_count`, `parallel`
- Log: `parameterized_loop.complete` — fields: `name`, `iterations`, `success_count`, `failure_count`, `duration`

**ExecuteTaskWorkflow** — no wrapper needed (single activity call, span created in activity layer).

### Integration

Docker/function workflow layers update their call sites to use instrumented wrappers instead of calling the generic functions directly.

## Section 2: Docker Activity (`docker/activity/otel.go`)

Full OTel spans + metrics (real `context.Context` available).

### Span

- Layer: `Layers.StartService(ctx, "docker", "StartContainer", ...)`
- Input attributes: `container.image`, `container.name`, `container.command`, `container.auto_remove`, `container.work_dir`
- Success attributes: `container.id`, `container.exit_code`, `container.duration`, `container.endpoint`
- Failure: `lc.Error(err, "container execution failed")`

### Metrics

- `go_wf.docker.task.total` (Int64Counter) — attributes: `status` (success/failure), `image`
- `go_wf.docker.task.duration` (Float64Histogram, seconds) — attributes: `image`, `exit_code`

## Section 3: Function Activity (`function/activity/otel.go`)

Full OTel spans + metrics.

### Span

- Layer: `Layers.StartService(ctx, "function", "Execute", ...)`
- Input attributes: `function.name`, `function.has_data`, `function.work_dir`
- Success attributes: `function.duration`, `function.has_result`
- Failure: `lc.Error(err, "function execution failed")`

### Metrics

- `go_wf.function.task.total` (Int64Counter) — attributes: `status`, `function_name`
- `go_wf.function.task.duration` (Float64Histogram, seconds) — attributes: `function_name`

## Section 4: Artifact Store (`workflow/artifacts/otel.go`)

Decorator pattern wrapping any `ArtifactStore` implementation.

### API

```go
func NewInstrumentedStore(inner ArtifactStore) ArtifactStore
```

### Instrumented Operations

**Upload:**
- Layer: `Layers.StartRepository(ctx, "artifacts", "Upload", ...)`
- Attributes: `artifact.name`, `artifact.type`, `artifact.workflow_id`, `artifact.step_name`
- Success: `artifact.size`

**Download:**
- Layer: `Layers.StartRepository(ctx, "artifacts", "Download", ...)`
- Attributes: `artifact.name`, `artifact.workflow_id`, `artifact.step_name`

**Delete:**
- Layer: `Layers.StartRepository(ctx, "artifacts", "Delete", ...)`
- Attributes: `artifact.name`, `artifact.workflow_id`, `artifact.step_name`

**Exists:**
- Layer: `Layers.StartRepository(ctx, "artifacts", "Exists", ...)`
- Attributes: `artifact.name`, `artifact.workflow_id`
- Success: `artifact.exists`

**List:**
- Layer: `Layers.StartRepository(ctx, "artifacts", "List", ...)`
- Attributes: `artifact.prefix`
- Success: `artifact.count`

### Metrics

- `go_wf.artifact.operation.total` (Int64Counter) — attributes: `operation`, `status`
- `go_wf.artifact.operation.duration` (Float64Histogram, seconds) — attributes: `operation`

## Section 5: Metrics Summary

### Full Catalog

| Metric Name | Type | Attributes | Location |
|---|---|---|---|
| `go_wf.docker.task.total` | Counter | `status`, `image` | docker/activity/otel.go |
| `go_wf.docker.task.duration` | Histogram (s) | `image`, `exit_code` | docker/activity/otel.go |
| `go_wf.function.task.total` | Counter | `status`, `function_name` | function/activity/otel.go |
| `go_wf.function.task.duration` | Histogram (s) | `function_name` | function/activity/otel.go |
| `go_wf.artifact.operation.total` | Counter | `operation`, `status` | workflow/artifacts/otel.go |
| `go_wf.artifact.operation.duration` | Histogram (s) | `operation` | workflow/artifacts/otel.go |

### Naming Convention

- Prefix: `go_wf.` (matches module name)
- Component: `docker.` / `function.` / `artifact.`
- What: `task.` / `operation.`
- Unit: `total` (counter) / `duration` (histogram in seconds)
- Attribute values: lowercase, underscore-separated

### Cardinality Control

- `status`: only `success` | `failure`
- `image`: docker image name without tag
- `function_name`: registered function name
- `operation`: `upload` | `download` | `delete` | `list` | `exists`
- `exit_code`: only on histogram (high-cardinality, not on counter)

## Section 6: Consumer Integration

### With OTel (application-side)

```go
// 1. Create config
otelCfg := pkgotel.NewConfig("my-app").
    WithTracerProvider(tp).
    WithMeterProvider(mp)

// 2. Store in context for activities
ctx = pkgotel.ContextWithConfig(ctx, otelCfg)

// 3. Register as usual
docker.RegisterAll(worker)

// 4. Wrap artifact store
store := artifacts.NewInstrumentedStore(artifacts.NewMinIOStore(...))
```

### Without OTel (default)

No setup needed. `ConfigFromContext` returns nil → no-op everywhere. Zero overhead.

### Dependency Impact

Only `github.com/jasoet/pkg/v2` is needed (already in go.mod for docker executor). No new transitive dependencies.

## Correlation Chain

```
Temporal Workflow Span (automatic from Temporal SDK)
  └── Activity Span (Temporal SDK)
        └── go-wf Service Span (Layers.StartService)
              ├── LogHelper.Info(ctx, ...) → log with trace_id + span_id
              └── histogram.Record(ctx, duration) → metric with trace_id exemplar
```

Workflow orchestration structured logs include step metadata that can be correlated with activity spans via Temporal's workflow/run IDs.

## Out of Scope

- Builder instrumentation (construction-time, not execution-time)
- OTel initialization (consuming app's responsibility)
- Temporal interceptors (Temporal SDK already provides basic tracing)
- Custom propagators (standard W3C trace context sufficient)
