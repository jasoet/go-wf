# Unit Test Expansion Design

## Goal

Expand unit test coverage for all workflow implementations using Temporal's `TestWorkflowEnvironment` with mocked activities. No Temporal server or containers required. Target: 42 new tests across 6 files, bringing total from 38 to 80.

## Approach

Add tests to existing test files (tests alongside implementations). Use `testify/assert`+`require` consistently. Migrate existing raw `t.Errorf`/`t.Fatalf` assertions to testify.

## Test Inventory

### Container Workflow (`workflow/container_test.go`) — 3 new

1. Container failure (exit code != 0, activity succeeds) — verify result passes through
2. Default timeout applied (RunTimeout=0)
3. Result fields fully populated (all output fields verified)

### Pipeline Workflow (`workflow/pipeline_test.go`) — 5 new

1. All steps fail with StopOnError=false — TotalFailed=3, no workflow error
2. Stop on error at first step — only 1 result, workflow error
3. Result output tracking — 3 steps with different outputs, verify order and data
4. Single container pipeline — edge case
5. Activity error vs container failure — error propagation with both strategies

### Parallel Workflow (`workflow/parallel_test.go`) — 6 new

1. Multiple failures with continue — 2 succeed, 2 fail
2. All containers fail with continue — TotalFailed=3, no workflow error
3. Activity error with fail-fast — verify error wrapping
4. Activity error with continue — TotalFailed=1, rest succeed
5. Single container parallel — edge case
6. Result count matches input — len(Results)==len(Containers)

### DAG Workflow (`workflow/dag_test.go`) — 8 new

1. Diamond dependency pattern — A->B,C->D
2. Parallel branches — A->B, A->C (B,C independent)
3. FailFast=false with node failure — failed node doesn't block independent nodes
4. Dependency failure blocks downstream — A fails, B depends on A, B doesn't execute
5. Output extraction and input substitution — build outputs feed into deploy env vars
6. Node not found in nodeMap — error path
7. FailFast=true with activity error — error propagation
8. Multiple independent roots — 3 nodes, no deps

### Loop Workflow (`workflow/loop_test.go`) — 11 new

1. Sequential fail-fast — stops at first failure
2. Sequential continue on failure — continues past failure
3. Parallel fail-fast — returns error
4. Parallel continue with multiple failures — collects all results
5. All iterations fail with continue — TotalFailed=count, no error
6. Single item loop — edge case
7. Parameterized loop fail-fast — error on failure
8. Parameterized loop continue with failures — collects results
9. Substitute in entrypoint — template substitution coverage
10. Substitute in volumes — template substitution coverage
11. Substitute in workdir — template substitution coverage

### Output Extraction (`workflow/output_extraction_test.go`) — 9 new

1. ValueFrom=file success — read from temp file
2. ValueFrom=file error with default — returns default
3. ValueFrom=file error no default — returns error
4. ValueFrom=file missing path — returns error
5. Unknown value_from — returns error with default
6. Empty stdout with default — returns default
7. Whitespace trimming — leading/trailing spaces trimmed
8. JSONPath null value — returns empty string
9. JSONPath failure with default — returns default

## Style Consistency

Migrate all existing tests from raw `t.Errorf`/`t.Fatalf` to `testify/assert`+`require`.

## Mock Patterns

All workflow tests use:
```go
env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).
    Return(&payload.ContainerExecutionOutput{...}, nil)
```

For per-call control:
```go
env.OnActivity(activity.StartContainerActivity, mock.Anything, specificInput).
    Return(&payload.ContainerExecutionOutput{Success: false, ExitCode: 1}, nil).Once()
```

For callback-based mocking (verify input was modified):
```go
env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).
    Return(func(_ context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
        // verify input.Env contains expected substituted values
        return &payload.ContainerExecutionOutput{...}, nil
    })
```
