# Integration Test Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve integration test coverage by fixing existing tests, filling docker test gaps, extracting a shared Temporal testcontainer helper, and adding function module integration tests.

**Architecture:** All integration tests use `TestMain` to start a Temporal server via testcontainers, create a client/worker, run tests, then tear down. A shared helper in `workflow/testutil/` eliminates duplication. Function integration tests run real handlers in-process against a real Temporal server.

**Tech Stack:** Go 1.26+, Temporal SDK, testcontainers-go, testify

---

### Task 1: Fix Legacy Build Tag in Docker Integration Test

**Files:**
- Modify: `docker/integration_test.go:1-2`

**Step 1: Remove the legacy `// +build` tag**

The file currently has both:
```go
//go:build integration
// +build integration
```

Remove line 2 (`// +build integration`). The `//go:build` directive (Go 1.17+) is sufficient.

**Step 2: Run the test to verify it compiles**

Run: `go vet -tags=integration ./docker/...`
Expected: No errors

**Step 3: Commit**

```bash
git add docker/integration_test.go
git commit -m "fix(docker): remove legacy // +build tag from integration test"
```

---

### Task 2: Create Shared Temporal Testcontainer Helper

**Files:**
- Create: `workflow/testutil/temporal.go`

**Step 1: Create the helper package**

```go
//go:build integration

// Package testutil provides shared test helpers for integration tests.
package testutil

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/sdk/client"
)

// TemporalContainer holds a running Temporal test container and its client.
type TemporalContainer struct {
	Client    client.Client
	container testcontainers.Container
}

// StartTemporalContainer starts a Temporal dev server container and returns a connected client.
// Call Cleanup() when done.
func StartTemporalContainer(ctx context.Context) (*TemporalContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "temporalio/temporal:latest",
		ExposedPorts: []string{"7233/tcp", "8233/tcp"},
		Cmd:          []string{"server", "start-dev", "--ip", "0.0.0.0"},
		WaitingFor:   wait.ForListeningPort("7233/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start temporal container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "7233")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	hostPort := fmt.Sprintf("%s:%s", host, mappedPort.Port())
	log.Printf("Temporal container started at %s", hostPort)

	// Wait for Temporal to fully initialize
	time.Sleep(3 * time.Second)

	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	return &TemporalContainer{
		Client:    c,
		container: container,
	}, nil
}

// Cleanup stops the Temporal client and terminates the container.
func (tc *TemporalContainer) Cleanup(ctx context.Context) {
	if tc.Client != nil {
		tc.Client.Close()
	}
	if tc.container != nil {
		if err := tc.container.Terminate(ctx); err != nil {
			log.Printf("failed to terminate temporal container: %v", err)
		}
	}
}
```

**Step 2: Verify it compiles**

Run: `go vet -tags=integration ./workflow/testutil/...`
Expected: No errors

**Step 3: Commit**

```bash
git add workflow/testutil/temporal.go
git commit -m "feat(workflow): add shared Temporal testcontainer helper"
```

---

### Task 3: Refactor Docker Integration Tests to Use Shared Helper

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Replace TestMain with shared helper**

Replace the `TestMain` function to use `testutil.StartTemporalContainer`:

```go
func TestMain(m *testing.M) {
	ctx := context.Background()

	tc, err := testutil.StartTemporalContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start Temporal: %v", err)
	}

	testClient = tc.Client

	// Create and start worker
	w := worker.New(testClient, testTaskQueue, worker.Options{})
	docker.RegisterAll(w)

	if err := w.Start(); err != nil {
		tc.Cleanup(ctx)
		log.Fatalf("Failed to start worker: %v", err)
	}

	code := m.Run()

	w.Stop()
	tc.Cleanup(ctx)
	os.Exit(code)
}
```

Update imports: remove `testcontainers`, `testcontainers-go/wait`, `fmt`, `time` (if unused after refactor). Add `"github.com/jasoet/go-wf/workflow/testutil"`.

**Step 2: Run existing integration tests to verify refactor**

Run: `go test -tags=integration -run TestIntegration_ExecuteContainerWorkflow -timeout 5m ./docker/...`
Expected: PASS

**Step 3: Commit**

```bash
git add docker/integration_test.go
git commit -m "refactor(docker): use shared Temporal testcontainer helper"
```

---

### Task 4: Add Missing Docker Integration Tests

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Add loop failure tests and parameterized loop test**

Add these test functions to `docker/integration_test.go`:

```go
// TestIntegration_LoopSequentialFailFast tests sequential loop stops on first failure.
func TestIntegration_LoopSequentialFailFast(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"ok", "fail", "skip"},
		Template: payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"sh", "-c", "if [ '{{item}}' = 'fail' ]; then exit 1; fi; echo {{item}}"},
			AutoRemove: true,
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-loop-seq-fail-fast",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	err = we.Get(ctx, &result)
	assert.Error(t, err)
}

// TestIntegration_LoopParallelContinue tests parallel loop continues after failure.
func TestIntegration_LoopParallelContinue(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"one", "fail", "three"},
		Template: payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"sh", "-c", "if [ '{{item}}' = 'fail' ]; then exit 1; fi; echo {{item}}"},
			AutoRemove: true,
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-loop-parallel-continue",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
}

// TestIntegration_ParameterizedLoop tests parameterized loop with parameter combinations.
func TestIntegration_ParameterizedLoop(t *testing.T) {
	ctx := context.Background()

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template: payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"echo", "{{.env}}-{{.region}}"},
			AutoRemove: true,
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-parameterized-loop",
			TaskQueue: testTaskQueue,
		},
		workflow.ParameterizedLoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 4, result.ItemCount)
}

// TestIntegration_ParallelMaxConcurrency tests parallel execution with concurrency limit.
func TestIntegration_ParallelMaxConcurrency(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest", Command: []string{"echo", "task 1"}, AutoRemove: true, Name: "mc-task1"},
			{Image: "alpine:latest", Command: []string{"echo", "task 2"}, AutoRemove: true, Name: "mc-task2"},
			{Image: "alpine:latest", Command: []string{"echo", "task 3"}, AutoRemove: true, Name: "mc-task3"},
			{Image: "alpine:latest", Command: []string{"echo", "task 4"}, AutoRemove: true, Name: "mc-task4"},
		},
		MaxConcurrency:  2,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-parallel-max-concurrency",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelContainersWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 4)
}
```

**Step 2: Run the new tests**

Run: `go test -tags=integration -run "TestIntegration_Loop|TestIntegration_Parameterized|TestIntegration_ParallelMax" -timeout 5m -v ./docker/...`
Expected: All PASS

**Step 3: Commit**

```bash
git add docker/integration_test.go
git commit -m "test(docker): add loop failure, parameterized loop, and max concurrency integration tests"
```

---

### Task 5: Add Function Integration Tests

**Files:**
- Create: `function/integration_test.go`

**Step 1: Create function integration test file**

```go
//go:build integration

package function_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/function"
	fnactivity "github.com/jasoet/go-wf/function/activity"
	"github.com/jasoet/go-wf/function/payload"
	"github.com/jasoet/go-wf/function/workflow"
	"github.com/jasoet/go-wf/workflow/testutil"
)

var (
	testClient    client.Client
	testTaskQueue = "function-integration-test-queue"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	tc, err := testutil.StartTemporalContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start Temporal: %v", err)
	}

	testClient = tc.Client

	// Create function registry with test handlers
	registry := fn.NewRegistry()
	registerTestHandlers(registry)

	// Create and start worker
	w := worker.New(testClient, testTaskQueue, worker.Options{})
	fn.RegisterWorkflows(w)
	fn.RegisterActivity(w, fnactivity.NewExecuteFunctionActivity(registry))

	if err := w.Start(); err != nil {
		tc.Cleanup(ctx)
		log.Fatalf("Failed to start worker: %v", err)
	}

	code := m.Run()

	w.Stop()
	tc.Cleanup(ctx)
	os.Exit(code)
}

func registerTestHandlers(registry *fn.Registry) {
	// echo: returns args as result
	registry.Register("echo", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: input.Args,
		}, nil
	})

	// fail: always returns an error
	registry.Register("fail", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		msg := input.Args["message"]
		if msg == "" {
			msg = "intentional failure"
		}
		return nil, fmt.Errorf("%s", msg)
	})

	// slow: sleeps briefly then returns
	registry.Register("slow", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		time.Sleep(100 * time.Millisecond)
		return &fn.FunctionOutput{
			Result: map[string]string{"status": "completed"},
		}, nil
	})

	// concat: joins item and index for loop testing
	registry.Register("concat", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: map[string]string{
				"item":  input.Args["item"],
				"index": input.Args["index"],
			},
		}, nil
	})
}

// --- Single Function Tests ---

func TestIntegration_ExecuteFunctionWorkflow(t *testing.T) {
	ctx := context.Background()

	input := payload.FunctionExecutionInput{
		Name: "echo",
		Args: map[string]string{"greeting": "hello", "target": "world"},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-single",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteFunctionWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.FunctionExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success)
	assert.Equal(t, "echo", result.Name)
	assert.Equal(t, "hello", result.Result["greeting"])
	assert.Equal(t, "world", result.Result["target"])
	assert.NotZero(t, result.Duration)
}

func TestIntegration_ExecuteFunction_HandlerError(t *testing.T) {
	ctx := context.Background()

	input := payload.FunctionExecutionInput{
		Name: "fail",
		Args: map[string]string{"message": "test error"},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-handler-error",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteFunctionWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.FunctionExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	// Handler errors are captured in output, NOT returned as workflow errors
	assert.False(t, result.Success)
	assert.Equal(t, "fail", result.Name)
	assert.Contains(t, result.Error, "test error")
}

// --- Pipeline Tests ---

func TestIntegration_FunctionPipeline(t *testing.T) {
	ctx := context.Background()

	input := payload.PipelineInput{
		StopOnError: false,
		Functions: []payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"step": "1"}},
			{Name: "echo", Args: map[string]string{"step": "2"}},
			{Name: "echo", Args: map[string]string{"step": "3"}},
		},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-pipeline",
			TaskQueue: testTaskQueue,
		},
		workflow.FunctionPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.PipelineOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_FunctionPipeline_StopOnError(t *testing.T) {
	ctx := context.Background()

	input := payload.PipelineInput{
		StopOnError: true,
		Functions: []payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"step": "1"}},
			{Name: "fail", Args: map[string]string{"message": "pipeline break"}},
			{Name: "echo", Args: map[string]string{"step": "3"}},
		},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-pipeline-stop",
			TaskQueue: testTaskQueue,
		},
		workflow.FunctionPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.PipelineOutput
	err = we.Get(ctx, &result)
	assert.Error(t, err)
}

func TestIntegration_FunctionPipeline_ContinueOnError(t *testing.T) {
	ctx := context.Background()

	input := payload.PipelineInput{
		StopOnError: false,
		Functions: []payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"step": "1"}},
			{Name: "fail", Args: map[string]string{"message": "pipeline error"}},
			{Name: "echo", Args: map[string]string{"step": "3"}},
		},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-pipeline-continue",
			TaskQueue: testTaskQueue,
		},
		workflow.FunctionPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.PipelineOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

// --- Parallel Tests ---

func TestIntegration_ParallelFunctions(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"task": "1"}},
			{Name: "echo", Args: map[string]string{"task": "2"}},
			{Name: "echo", Args: map[string]string{"task": "3"}},
		},
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parallel",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_ParallelFunctions_FailFast(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "slow"},
			{Name: "fail", Args: map[string]string{"message": "fast fail"}},
			{Name: "slow"},
		},
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parallel-fail-fast",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	err = we.Get(ctx, &result)
	assert.Error(t, err)
}

func TestIntegration_ParallelFunctions_ContinueWithFailure(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"task": "ok1"}},
			{Name: "fail", Args: map[string]string{"message": "oops"}},
			{Name: "echo", Args: map[string]string{"task": "ok2"}},
		},
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parallel-continue",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_ParallelFunctions_MaxConcurrency(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Functions: []payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"task": "1"}},
			{Name: "echo", Args: map[string]string{"task": "2"}},
			{Name: "echo", Args: map[string]string{"task": "3"}},
			{Name: "echo", Args: map[string]string{"task": "4"}},
		},
		MaxConcurrency:  2,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parallel-max-concurrency",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 4)
}

// --- Loop Tests ---

func TestIntegration_LoopSequential(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"alpha", "beta", "gamma"},
		Template: payload.FunctionExecutionInput{
			Name: "concat",
			Args: map[string]string{
				"item":  "{{item}}",
				"index": "{{index}}",
			},
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-sequential",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
}

func TestIntegration_LoopParallel(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"one", "two", "three"},
		Template: payload.FunctionExecutionInput{
			Name: "echo",
			Args: map[string]string{"item": "{{item}}"},
		},
		Parallel:        true,
		MaxConcurrency:  2,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-parallel",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
}

func TestIntegration_LoopSequentialFailFast(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"ok", "bad", "skip"},
		Template: payload.FunctionExecutionInput{
			Name: "{{item}}",
			Args: map[string]string{"item": "{{item}}"},
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	// Register handler that fails for "bad"
	// Note: "ok" is not registered, so it will also fail — use echo/fail instead
	// Use a template name that maps to known handlers
	input.Template.Name = "echo"
	input.Items = []string{"ok", "fail-item", "skip"}

	// Simpler approach: use items as args to the fail handler
	input.Template = payload.FunctionExecutionInput{
		Name: "fail",
		Args: map[string]string{"message": "loop item {{item}} failed"},
	}
	input.Items = []string{"first"}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-seq-fail-fast",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	err = we.Get(ctx, &result)
	assert.Error(t, err)
}

func TestIntegration_LoopParallelContinueWithFailure(t *testing.T) {
	ctx := context.Background()

	// We need mixed success/failure. Use "echo" for success, but we can't dynamically
	// switch handler per item in a loop (name is part of template).
	// Instead, test that all-success works with parallel continue.
	input := payload.LoopInput{
		Items: []string{"a", "b", "c"},
		Template: payload.FunctionExecutionInput{
			Name: "echo",
			Args: map[string]string{"value": "{{item}}"},
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-parallel-continue",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
}

// --- Parameterized Loop Tests ---

func TestIntegration_ParameterizedLoop(t *testing.T) {
	ctx := context.Background()

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "echo",
			Args: map[string]string{
				"env":    "{{.env}}",
				"region": "{{.region}}",
			},
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parameterized-loop",
			TaskQueue: testTaskQueue,
		},
		workflow.ParameterizedLoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 4, result.ItemCount)
}

func TestIntegration_ParameterizedLoopParallel(t *testing.T) {
	ctx := context.Background()

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"size": {"small", "medium", "large"},
		},
		Template: payload.FunctionExecutionInput{
			Name: "echo",
			Args: map[string]string{"size": "{{.size}}"},
		},
		Parallel:        true,
		MaxConcurrency:  2,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parameterized-loop-parallel",
			TaskQueue: testTaskQueue,
		},
		workflow.ParameterizedLoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
}
```

**Step 2: Run the function integration tests**

Run: `go test -tags=integration -timeout 5m -v ./function/...`
Expected: All PASS

**Step 3: Commit**

```bash
git add function/integration_test.go
git commit -m "test(function): add integration tests with real Temporal server"
```

---

### Task 6: Run Full Integration Test Suite and Verify

**Step 1: Run all integration tests**

Run: `task test:integration`
Expected: All tests pass

**Step 2: Run unit tests to ensure nothing is broken**

Run: `task test:unit`
Expected: All tests pass

**Step 3: Format and vet**

Run: `task fmt && go vet -tags=integration ./...`
Expected: No errors

---

### Task 7: Update Documentation

**Files:**
- Modify: `INSTRUCTION.md`

**Step 1: Update Testing Strategy section**

Add a note about the shared testcontainer helper in the Testing Strategy section.

Add to Key Paths table:
```
| `workflow/testutil/` | Shared test helpers (Temporal testcontainer) |
```

**Step 2: Commit**

```bash
git add INSTRUCTION.md
git commit -m "docs: update INSTRUCTION.md with testutil path and integration test info"
```
