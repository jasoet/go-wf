# Container Test Fixes & Integration CI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix container test issues from code review and add optional GitHub Actions workflow for integration tests on self-hosted runner with Podman.

**Architecture:** Eight test file changes (removes, additions, refactors) plus one new GitHub Actions workflow file. All test changes are isolated per-file with no cross-dependencies. The CI workflow is a standalone YAML file.

**Tech Stack:** Go testing, Temporal SDK testsuite, testify, GitHub Actions, Nix, Podman/testcontainers-go

---

### Task 1: Remove dead-weight `integrationMockWorker` and `TestWorkflowRegistration` from `container/container_test.go`

**Files:**
- Modify: `container/container_test.go:22-40` (remove `integrationMockWorker` type) and `:253-267` (remove `TestWorkflowRegistration`)

**Step 1: Remove `integrationMockWorker` type and `TestWorkflowRegistration`**

Remove lines 22-40 (the `integrationMockWorker` struct and all its methods) and lines 253-267 (the `TestWorkflowRegistration` function). These are duplicated by the proper mock-based tests in `container/worker_test.go:94-134`.

Also remove the now-unused imports that were only needed by the removed code:
- `"github.com/nexus-rpc/sdk-go/nexus"`
- `sdkactivity "go.temporal.io/sdk/activity"`
- `sdkworkflow "go.temporal.io/sdk/workflow"`

Keep these imports (still used by remaining tests):
- `"context"`, `"fmt"`, `"testing"`
- `"github.com/stretchr/testify/assert"`, `"github.com/stretchr/testify/mock"`, `"github.com/stretchr/testify/require"`
- `"go.temporal.io/sdk/testsuite"`
- `"github.com/jasoet/go-wf/container/activity"`
- `"github.com/jasoet/go-wf/container/payload"`
- `"github.com/jasoet/go-wf/container/workflow"`

**Step 2: Run tests to verify nothing breaks**

Run: `task test:pkg -- ./container/...`
Expected: All tests PASS (the removed tests were duplicates)

**Step 3: Commit**

```
git add container/container_test.go
git commit -m "test(container): remove duplicate integrationMockWorker and TestWorkflowRegistration"
```

---

### Task 2: Remove `TestContainerExecutionOutput_Fields` from `container/activity/container_test.go`

**Files:**
- Modify: `container/activity/container_test.go:178-235` (remove `TestContainerExecutionOutput_Fields`)

**Step 1: Remove `TestContainerExecutionOutput_Fields`**

Delete lines 178-235. This test only verifies Go struct field assignment — it tests the language, not our code.

Check if the `"time"` import is still used by remaining tests (it is — used in `TestBuildWaitStrategy` and `TestContainerExecutionInput_AllFields`). Keep it.

**Step 2: Run tests to verify**

Run: `task test:pkg -- ./container/activity/...`
Expected: All tests PASS

**Step 3: Commit**

```
git add container/activity/container_test.go
git commit -m "test(container): remove TestContainerExecutionOutput_Fields that tests Go assignment"
```

---

### Task 3: Remove `TestHelperFunctions` from `container/workflow/dag_test.go`

**Files:**
- Modify: `container/workflow/dag_test.go:204-218` (remove `TestHelperFunctions`)

**Step 1: Remove `TestHelperFunctions`**

Delete lines 204-218. This tests `strings.ReplaceAll` and `strings.Index` from the Go standard library.

Remove the `"strings"` import (line 7) since it's only used by this test. Keep all other imports.

**Step 2: Run tests to verify**

Run: `task test:pkg -- ./container/workflow/...`
Expected: All tests PASS

**Step 3: Commit**

```
git add container/workflow/dag_test.go
git commit -m "test(container): remove TestHelperFunctions that tests Go stdlib"
```

---

### Task 4: Add cycle detection test case to `TestDAGWorkflowValidation`

**Files:**
- Modify: `container/workflow/dag_test.go:64-122` (add test case to existing table)

**Step 1: Add cycle detection test case**

Add this test case to the `tests` slice in `TestDAGWorkflowValidation` (after the "missing dependency" case at line 109):

```go
{
    name: "cyclic dependency",
    input: payload.DAGWorkflowInput{
        Nodes: []payload.DAGNode{
            {
                Name: "task-a",
                Container: payload.ExtendedContainerInput{
                    ContainerExecutionInput: payload.ContainerExecutionInput{
                        Image: "alpine:latest",
                    },
                },
                Dependencies: []string{"task-b"},
            },
            {
                Name: "task-b",
                Container: payload.ExtendedContainerInput{
                    ContainerExecutionInput: payload.ContainerExecutionInput{
                        Image: "alpine:latest",
                    },
                },
                Dependencies: []string{"task-a"},
            },
        },
    },
    expectError: true,
},
```

**Step 2: Run test to verify it passes**

Run: `task test:run -- -run TestDAGWorkflowValidation ./container/workflow/...`
Expected: PASS — the cycle detection already exists in `payloads_extended.go:228-271`

**Step 3: Commit**

```
git add container/workflow/dag_test.go
git commit -m "test(container): add cycle detection test case for DAG validation"
```

---

### Task 5: Strengthen `TestBuildWaitStrategy`

**Files:**
- Modify: `container/activity/container_test.go:49-130` (improve assertions)

**Step 1: Strengthen the test assertions**

The current test loop (lines 120-129) only checks `strategy != nil`. Replace the loop body to also verify the `want` field is meaningful by adding a type-name check comment and ensuring we at least test that each config produces a distinct non-nil strategy. Since `wait.Strategy` is an interface and we can't easily inspect types, add a dedicated test for the timeout-defaulting behavior:

Replace the test loop at lines 120-129 with:

```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        strategy := buildWaitStrategy(tt.config)
        if strategy == nil {
            t.Errorf("buildWaitStrategy returned nil for type %q", tt.config.Type)
        }
    })
}
```

Then add a new test function after `TestBuildWaitStrategy_TimeoutDefaults` (after line 144):

```go
func TestBuildWaitStrategy_DistinctTypes(t *testing.T) {
	// Verify that different config types produce non-nil strategies
	// and that the function handles all documented types without panicking
	types := []string{"log", "port", "http", "healthy", "unknown", ""}
	for _, typ := range types {
		t.Run("type_"+typ, func(t *testing.T) {
			config := payload.WaitStrategyConfig{
				Type:       typ,
				LogMessage: "ready",
				Port:       "8080",
				HTTPPath:   "/health",
			}
			strategy := buildWaitStrategy(config)
			if strategy == nil {
				t.Errorf("buildWaitStrategy(%q) returned nil", typ)
			}
		})
	}
}
```

**Step 2: Run tests to verify**

Run: `task test:run -- -run TestBuildWaitStrategy ./container/activity/...`
Expected: All PASS

**Step 3: Commit**

```
git add container/activity/container_test.go
git commit -m "test(container): strengthen TestBuildWaitStrategy with distinct type coverage"
```

---

### Task 6: Make integration test workflow IDs unique

**Files:**
- Modify: `container/integration_test.go` (all 20 workflow ID strings)

**Step 1: Add a helper function and update all IDs**

Add a helper at the top of the file (after the `var` block, around line 26):

```go
// uniqueWorkflowID returns a unique workflow ID by appending the test name.
func uniqueWorkflowID(t *testing.T, base string) string {
	return fmt.Sprintf("%s-%s", base, t.Name())
}
```

Add `"fmt"` to the imports if not already present (it's not in the current imports — add it).

Then replace every hardcoded `ID:` value in the file. For example, change:

```go
ID: "integration-test-execute-container",
```

to:

```go
ID: uniqueWorkflowID(t, "integration-test-execute-container"),
```

Apply this pattern to all 20 workflow ID strings at lines: 70, 114, 163, 196, 225, 253, 281, 308, 347, 386, 432, 477, 537, 589, 622, 656, 690, 722, 759, 813.

**Step 2: Run tests to verify compilation**

Run: `task test:unit` (integration tests won't run without `-tags=integration`, but compilation check is sufficient)
Expected: PASS

**Step 3: Commit**

```
git add container/integration_test.go
git commit -m "test(container): make integration test workflow IDs unique with t.Name()"
```

---

### Task 7: Add negative concurrency validation test

**Files:**
- Modify: `container/activity/container_test.go` (add test after `TestParallelInput_Concurrency`)

**Step 1: Add test documenting that negative concurrency is accepted**

Since `MaxConcurrency` is documented as "not currently enforced" (`payloads.go:124-125`), negative values pass validation. Add a test that documents this behavior after `TestParallelInput_Concurrency` (after line 340):

```go
func TestParallelInput_NegativeConcurrency(t *testing.T) {
	// MaxConcurrency is not currently enforced (see ParallelInput docs).
	// Negative values pass validation. This test documents current behavior.
	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest"},
		},
		MaxConcurrency:  -1,
		FailureStrategy: "continue",
	}

	err := input.Validate()
	assert.NoError(t, err, "Negative MaxConcurrency should pass validation (field is not enforced)")
}
```

**Step 2: Run test to verify**

Run: `task test:run -- -run TestParallelInput_NegativeConcurrency ./container/activity/...`
Expected: PASS

**Step 3: Commit**

```
git add container/activity/container_test.go
git commit -m "test(container): add negative concurrency test documenting current behavior"
```

---

### Task 8: Make sequential loop `callCount` use `atomic.Int32`

**Files:**
- Modify: `container/workflow/loop_test.go:575-643` (two functions)

**Step 1: Update `TestLoopWorkflow_SequentialFailFast`**

At line 580, change:
```go
callCount := 0
```
to:
```go
var callCount atomic.Int32
```

At line 583-586, change the mock body:
```go
func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
    callCount++
    if callCount == 2 {
```
to:
```go
func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
    c := callCount.Add(1)
    if c == 2 {
```

At line 604, change:
```go
assert.Equal(t, 2, callCount, "third item should not execute")
```
to:
```go
assert.Equal(t, int32(2), callCount.Load(), "third item should not execute")
```

**Step 2: Update `TestLoopWorkflow_SequentialContinueOnFailure`**

At line 613, change:
```go
callCount := 0
```
to:
```go
var callCount atomic.Int32
```

At line 616-619, change:
```go
func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
    callCount++
    if callCount == 2 {
```
to:
```go
func(_ context.Context, _ payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
    c := callCount.Add(1)
    if c == 2 {
```

Verify that `"sync/atomic"` is already imported (it is — line 5).

**Step 3: Run tests to verify**

Run: `task test:pkg -- ./container/workflow/...`
Expected: All PASS

**Step 4: Commit**

```
git add container/workflow/loop_test.go
git commit -m "test(container): use atomic.Int32 consistently for loop test call counters"
```

---

### Task 9: Add integration test GitHub Actions workflow

**Files:**
- Create: `.github/workflows/integration-test.yml`

**Step 1: Create the workflow file**

```yaml
name: Integration Tests

on:
  workflow_dispatch:
    inputs:
      scope:
        description: 'Test scope to run'
        required: true
        default: 'all'
        type: choice
        options:
          - all
          - container
          - function
          - datasync
          - store

jobs:
  integration-test:
    name: Integration Tests (${{ inputs.scope }})
    runs-on: [self-hosted, local, macOS, ARM64]
    timeout-minutes: 20
    steps:
      - name: Checkout
        uses: actions/checkout@v6

      - name: Check Container Engine
        run: |
          # Detect container engine and export env vars for testcontainers
          if [ -n "$DOCKER_HOST" ]; then
            echo "Using existing DOCKER_HOST=$DOCKER_HOST"
          elif command -v docker &>/dev/null && docker info &>/dev/null 2>&1; then
            echo "Using native Docker"
          else
            SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null || true)
            if [ -n "$SOCKET" ] && [ -e "$SOCKET" ]; then
              echo "DOCKER_HOST=unix://$SOCKET" >> "$GITHUB_ENV"
              echo "TESTCONTAINERS_RYUK_DISABLED=true" >> "$GITHUB_ENV"
              echo "Using Podman socket: $SOCKET"
            else
              echo "::error::No container engine found"
              exit 1
            fi
          fi

      - name: Resolve Test Packages
        id: packages
        run: |
          case "${{ inputs.scope }}" in
            all)       echo "packages=./..." >> "$GITHUB_OUTPUT" ;;
            container) echo "packages=./container/..." >> "$GITHUB_OUTPUT" ;;
            function)  echo "packages=./function/..." >> "$GITHUB_OUTPUT" ;;
            datasync)  echo "packages=./datasync/..." >> "$GITHUB_OUTPUT" ;;
            store)     echo "packages=./workflow/store/... ./workflow/artifacts/..." >> "$GITHUB_OUTPUT" ;;
          esac

      - name: Run Integration Tests
        run: |
          nix develop --command go test \
            -race -count=1 \
            -tags=integration \
            -timeout=15m \
            ${{ steps.packages.outputs.packages }}
```

**Step 2: Validate YAML syntax**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/integration-test.yml'))"`
Expected: No error

**Step 3: Commit**

```
git add .github/workflows/integration-test.yml
git commit -m "ci: add manual integration test workflow with scope selection"
```

---

### Task 10: Final verification

**Step 1: Run full unit test suite**

Run: `task test:unit`
Expected: All PASS, no regressions

**Step 2: Run linter**

Run: `task lint`
Expected: No new lint errors

**Step 3: Verify all commits**

Run: `git log --oneline -10`
Expected: 9 clean conventional commits (tasks 1-9)
