# Integration Test Improvements Design

## Goal

Improve integration test coverage across docker, function, and workflow modules by fixing issues in existing tests, filling coverage gaps, and adding end-to-end function integration tests against a real Temporal server.

## Architecture

All integration tests follow the same pattern: `TestMain` starts testcontainers (Temporal server, MinIO, etc.), creates clients/workers, runs tests, then tears down. Function integration tests are lighter than docker tests — they only need a Temporal container since functions execute in-process.

A shared Temporal testcontainer helper will be extracted to `workflow/testutil/` to eliminate setup duplication between `docker/` and `function/` integration tests.

## Scope

### 1. Fix Existing Docker Integration Tests

- Remove legacy `// +build integration` tag (only `//go:build integration` needed since Go 1.17+)
- No other structural changes needed — tests are well-written

### 2. Add Missing Docker Integration Tests

Fill gaps in `docker/integration_test.go`:
- Loop with failure (sequential fail-fast, parallel continue-with-failures)
- Parameterized loop workflow
- DAG with continue-on-failure (FailFast=false)
- Parallel with MaxConcurrency

### 3. Shared Temporal Testcontainer Helper

Extract to `workflow/testutil/temporal.go`:
- `StartTemporalContainer(ctx) → (client.Client, cleanup func(), error)`
- Encapsulates container creation, port mapping, client dial, initialization wait
- Used by both `docker/integration_test.go` and `function/integration_test.go`
- Tagged with `//go:build integration` so it doesn't affect unit test builds

### 4. Add Function Integration Tests

New file `function/integration_test.go`:
- Uses shared Temporal testcontainer helper
- Creates real registry with test handlers, real activity, real worker
- Tests all 5 workflows end-to-end:
  - `ExecuteFunctionWorkflow` — single function
  - `FunctionPipelineWorkflow` — sequential with stop-on-error and continue
  - `ParallelFunctionsWorkflow` — concurrent with fail-fast and continue
  - `LoopWorkflow` — sequential and parallel with template substitution
  - `ParameterizedLoopWorkflow` — matrix-style parameter expansion
- Tests error semantics: handler errors → Success=false (no Temporal retry)

## Test Handlers

Simple in-process handlers for integration tests:
- `echo` — returns args as result (validates payload serialization)
- `fail` — returns error (validates error semantics)
- `slow` — sleeps briefly (validates concurrency/ordering)
- `concat` — joins args (validates template substitution in loops)

## Not In Scope

- Docker activity integration tests (`docker/activity/`) — already well-covered
- MinIO integration tests (`workflow/artifacts/`) — already well-covered
- Builder/template tests — these are unit-testable, no integration value
