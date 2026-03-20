# Test Coverage Improvement Design

**Date:** 2026-03-17
**Goal:** Bring all packages to 85%+ test coverage
**Approach:** Unit tests with mocks (Approach 1)

## Current State

| Package | Coverage | Target | Gap |
|---------|----------|--------|-----|
| `function/activity` | 53.8% | 85%+ | ~31% |
| `docker/activity` | 56.8% | 85%+ | ~28% |
| `workflow/artifacts` | 65.7% | 85%+ | ~19% |
| `workflow` | 82.7% | 85%+ | ~2% |

## Root Cause

All four packages share the same gap: **OTel instrumentation code paths when config is non-nil are untested**. Current tests only cover the nil-config pass-through. The with-config branches (span creation, metric recording, error/success logging) account for the majority of uncovered lines.

## Testing Strategy

### OTel Config in Tests

Create a minimal `otel.Config` using no-op providers so the with-config code path executes without requiring real exporters:

```go
import (
    pkgotel "github.com/jasoet/pkg/v2/otel"
    sdkmetric "go.opentelemetry.io/otel/sdk/metric"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func otelContext() context.Context {
    cfg := pkgotel.NewConfig("test-service").
        WithTracerProvider(sdktrace.NewTracerProvider()).
        WithMeterProvider(sdkmetric.NewMeterProvider())
    return pkgotel.ContextWithConfig(context.Background(), cfg)
}
```

This makes `ConfigFromContext(ctx)` return non-nil, triggering all instrumented code paths. No assertions on span/metric values — just verify the code runs correctly and returns expected results.

## Package-Specific Plans

### 1. `function/activity` (53.8% → 85%+)

**File:** `function/activity/otel_test.go` (extend existing)

New tests:
- `TestInstrumentedExecuteFunctionActivity_WithOTelConfig_Success` — OTel context + successful handler → verify output matches, no error
- `TestInstrumentedExecuteFunctionActivity_WithOTelConfig_HandlerError` — OTel context + handler returns error → verify error propagated
- `TestInstrumentedExecuteFunctionActivity_WithOTelConfig_NilOutput` — OTel context + handler returns nil output → verify no panic
- `TestRecordFunctionMetrics_WithConfig` — OTel context → verify no panic, counter + histogram code runs

### 2. `docker/activity` (56.8% → 85%+)

**File:** `docker/activity/otel_test.go` (extend existing)

New tests:
- `TestInstrumentedStartContainerActivity_WithOTelConfig_Success` — OTel context + successful activity → verify output, no error
- `TestInstrumentedStartContainerActivity_WithOTelConfig_Error` — OTel context + activity returns error → verify error propagated, span records error
- `TestRecordDockerMetrics_WithConfig` — OTel context → verify no panic with various status/exitCode/duration values
- `TestImageBaseName_EdgeCases` — empty string, multiple colons, colon at start

**File:** `docker/activity/container_test.go` (extend existing)

New tests:
- `TestBuildWaitStrategy_HTTPDefaults` — verify HTTP strategy default status code (200)
- `TestBuildWaitStrategy_AllTypes` — verify strategy properties more thoroughly (not just non-nil checks)

### 3. `workflow/artifacts` (65.7% → 85%+)

**File:** `workflow/artifacts/otel_test.go` (extend existing)

New tests:
- `TestInstrumentedStore_Upload_WithOTelConfig` — OTel context + mock store → verify delegation + no panic
- `TestInstrumentedStore_Download_WithOTelConfig` — same pattern
- `TestInstrumentedStore_Delete_WithOTelConfig` — same pattern
- `TestInstrumentedStore_Exists_WithOTelConfig` — same pattern
- `TestInstrumentedStore_List_WithOTelConfig` — same pattern
- `TestInstrumentedStore_Upload_WithOTelConfig_Error` — OTel context + mock returns error → verify error propagated, span records error
- `TestInstrumentedStore_Download_WithOTelConfig_Error` — same pattern
- `TestRecordArtifactMetrics_WithConfig` — OTel context → verify no panic

### 4. `workflow` (82.7% → 85%+)

**File:** `workflow/otel_test.go` (extend existing `OtelWorkflowTestSuite`)

New tests:
- `TestInstrumentedLoopWorkflow_Sequential_Success` — sequential loop, all items succeed
- `TestInstrumentedLoopWorkflow_Parallel_Success` — parallel loop, all items succeed
- `TestInstrumentedLoopWorkflow_Error` — loop with item failure
- `TestInstrumentedParameterizedLoopWorkflow_Success` — parameterized loop, all combinations succeed
- `TestInstrumentedParameterizedLoopWorkflow_Error` — parameterized loop with failure

## What We Won't Test

- MinioStore unit tests — integration tests cover it, mocking the Minio client adds fragile coupling
- OS-level error paths in archive helpers (permission denied, disk full) — hard to simulate reliably
- Logger output content — verifying log messages is brittle
- Specific span attribute values or metric values — we only verify the code path executes without errors

## Test Patterns

All new tests follow existing project conventions:
- `testify/assert` and `testify/require` for assertions
- Table-driven tests where multiple similar cases exist
- Hand-written mock structs (no code generation)
- Suite-based tests for workflow (Temporal `TestWorkflowEnvironment`)
- `assert.NotPanics` for OTel metric recording tests
