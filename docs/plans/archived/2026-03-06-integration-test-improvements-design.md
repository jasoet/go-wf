# Integration Test Improvements Design

Date: 2026-03-06

## Problem

The integration tests have three categories of issues:

1. **Efficiency**: Each test spins up its own container (7 Temporal containers for 7 tests, 5 MinIO containers for 5 tests)
2. **Correctness**: Four tests don't assert output content, only checking exit code/success status
3. **Coverage gaps**: No tests for failure scenarios, DAG workflows, or Loop workflows

## Approach: Incremental Refactor

Refactor in-place using `TestMain` for shared containers, fix assertions, and add new tests. Prioritized: efficiency first, then correctness, then coverage.

## Design

### Phase 1: Shared Container Infrastructure

#### `docker/integration_test.go`

Add `TestMain(m *testing.M)` that:
- Starts one Temporal container (`temporalio/temporal:latest`)
- Creates a shared Temporal client and worker
- Registers all workflows via `docker.RegisterAll(w)`
- Runs all tests via `m.Run()`
- Cleans up: stops worker, closes client, terminates container

Package-level vars:
- `testClient client.Client`
- `testTaskQueue string` (constant: `"integration-test-queue"`)

Each test function uses `testClient` and `testTaskQueue` directly — no per-test container setup.

#### `docker/artifacts/minio_integration_test.go`

Add `TestMain(m *testing.M)` that:
- Starts one MinIO container (`minio/minio:latest`)
- Stores config in package-level `testMinioConfig MinioConfig`
- Runs all tests via `m.Run()`
- Cleans up: terminates container

Each test creates its own `MinioStore` from the shared config for test isolation.

### Phase 2: Fix Existing Assertions

Add output content assertions to four tests:

| Test | Assertion |
|------|-----------|
| `ContainerWithEnvironment` | `assert.Contains(result.Stdout, "test_value")` |
| `ContainerWithWorkDir` | `assert.Contains(result.Stdout, "/tmp")` |
| `ContainerWithEntrypoint` | `assert.Contains(result.Stdout, "test")` |
| `ContainerWithUser` | `assert.Contains(result.Stdout, "nobody")` |

Also migrate all tests from raw `t.Errorf`/`t.Fatalf` to `testify/assert` + `require` for consistency.

### Phase 3: New Test Coverage

#### Failure Scenarios

| Test | Setup | Key Assertions |
|------|-------|----------------|
| `ContainerFailure` | `alpine` with `exit 1` | `Success == false`, `ExitCode == 1` |
| `PipelineStopOnError` | step1 `exit 1`, `StopOnError=true` | `TotalFailed >= 1`, step2 never ran |
| `PipelineContinueOnError` | step1 `exit 1`, `StopOnError=false` | `TotalFailed == 1`, `TotalSuccess == 1` |
| `ParallelFailFast` | one of 3 `exit 1`, `FailureStrategy="fail_fast"` | `TotalFailed >= 1` |
| `ParallelContinue` | one of 3 `exit 1`, `FailureStrategy="continue"` | `TotalFailed == 1`, `TotalSuccess == 2` |

#### DAG Workflow

| Test | Setup | Key Assertions |
|------|-------|----------------|
| `DAGWorkflow` | A -> B, A -> C (diamond) | `TotalSuccess == 3`, all nodes complete |
| `DAGWorkflowFailFast` | A fails, B depends on A, `FailFast=true` | B skipped, `TotalFailed >= 1` |

#### Loop Workflow

| Test | Setup | Key Assertions |
|------|-------|----------------|
| `LoopSequential` | 3 items sequential | All succeed, correct result count |
| `LoopParallel` | 3 items parallel | `TotalSuccess == 3` |

### Summary

- **Before**: 12 containers for 12 tests, 4 tests with weak assertions, 0 failure/DAG/loop tests
- **After**: 2 containers total, all tests with strong assertions, 9 new tests covering failure handling, DAG, and Loop workflows
