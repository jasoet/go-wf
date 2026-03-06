# Integration Test Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Consolidate integration test containers via TestMain, fix missing assertions, and add failure/DAG/Loop test coverage.

**Architecture:** Each test package gets a single `TestMain` managing one shared container. Existing tests are refactored to use shared state. New tests are added for failure scenarios, DAG, and Loop workflows.

**Tech Stack:** Go, testcontainers-go, Temporal SDK, testify (assert/require)

---

### Task 1: Add TestMain to docker/integration_test.go

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Add package-level vars and TestMain**

Replace the per-test container setup with a shared `TestMain`. Add these package-level vars and the `TestMain` function at the top of the file (after imports):

```go
var (
	testClient    client.Client
	testTaskQueue = "integration-test-queue"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start Temporal container
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
		log.Fatalf("Failed to start Temporal container: %v", err)
	}

	mappedPort, err := container.MappedPort(ctx, "7233")
	if err != nil {
		log.Fatalf("Failed to get mapped port: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get host: %v", err)
	}

	hostPort := fmt.Sprintf("%s:%s", host, mappedPort.Port())
	log.Printf("Temporal container started at %s", hostPort)

	// Wait for Temporal to fully initialize
	time.Sleep(3 * time.Second)

	// Create Temporal client
	testClient, err = client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}

	// Create and start worker
	w := worker.New(testClient, testTaskQueue, worker.Options{})
	docker.RegisterAll(w)

	if err := w.Start(); err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	w.Stop()
	testClient.Close()
	if err := container.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate container: %v", err)
	}

	os.Exit(code)
}
```

Add `"log"` and `"os"` to the import block. Remove the `TemporalContainer` struct and `StartTemporalContainer` function entirely.

**Step 2: Refactor all existing tests to use shared state**

Each test loses its container setup boilerplate. Replace the pattern:

```go
// OLD pattern (remove all of this from each test):
temporal, err := StartTemporalContainer(ctx, t)
// ... defer terminate ...
c, err := client.Dial(...)
// ... defer close ...
taskQueue := "integration-test-xxx-queue"
w := worker.New(c, taskQueue, worker.Options{})
docker.RegisterAll(w)
w.Start()
// ... defer stop ...
```

With just:

```go
ctx := context.Background()
```

Then replace all references:
- `c.ExecuteWorkflow(...)` -> `testClient.ExecuteWorkflow(...)`
- Each test's custom `taskQueue` -> `testTaskQueue`
- Each test's custom workflow ID -> keep unique per test (already unique)

Also migrate from raw `t.Errorf`/`t.Fatalf` to `testify/assert` + `require`. Add imports:
```go
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/require"
```

Example — `TestIntegration_ExecuteContainerWorkflow` becomes:

```go
func TestIntegration_ExecuteContainerWorkflow(t *testing.T) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "integration test"},
		AutoRemove: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-execute-container",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ContainerExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
	assert.NotEmpty(t, result.ContainerID)
	assert.Contains(t, result.Stdout, "integration test")
}
```

Apply this same pattern to all 7 existing tests: remove container/client/worker setup, use `testClient`/`testTaskQueue`, use `require`/`assert`.

**Step 3: Run tests to verify refactor**

Run: `task test:integration`
Expected: All 7 existing tests pass using the single shared Temporal container.

**Step 4: Commit**

```
git add docker/integration_test.go
git commit -m "refactor(tests): consolidate Temporal container via TestMain"
```

---

### Task 2: Add TestMain to docker/artifacts/minio_integration_test.go

**Files:**
- Modify: `docker/artifacts/minio_integration_test.go`

**Step 1: Add package-level var and TestMain**

Add package-level var and `TestMain`:

```go
var testMinioConfig MinioConfig

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/live").WithPort("9000").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Failed to start MinIO container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("Failed to get host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		log.Fatalf("Failed to get mapped port: %v", err)
	}

	testMinioConfig = MinioConfig{
		Endpoint:  host + ":" + port.Port(),
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "test-artifacts",
		Prefix:    "workflows/",
		UseSSL:    false,
		Region:    "us-east-1",
	}

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		log.Printf("Failed to terminate MinIO container: %v", err)
	}

	os.Exit(code)
}
```

Add `"log"` and `"os"` to imports. Remove `setupMinioContainer` function.

**Step 2: Refactor all 5 tests to use testMinioConfig**

Each test replaces:
```go
container, config := setupMinioContainer(ctx, t)
defer func() { ... container.Terminate ... }()
store, err := NewMinioStore(ctx, config)
```

With:
```go
store, err := NewMinioStore(ctx, testMinioConfig)
require.NoError(t, err)
defer store.Close()
```

Use unique bucket prefixes or artifact names per test to avoid collision (already the case — each test uses different WorkflowID/RunID).

**Step 3: Run tests**

Run: `task test:integration`
Expected: All 5 MinIO tests pass using single shared container.

**Step 4: Commit**

```
git add docker/artifacts/minio_integration_test.go
git commit -m "refactor(tests): consolidate MinIO container via TestMain"
```

---

### Task 3: Fix missing output assertions

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Add Stdout assertions to four tests**

In `TestIntegration_ContainerWithEnvironment`, add after the success check:
```go
assert.Contains(t, result.Stdout, "test_value")
```

In `TestIntegration_ContainerWithWorkDir`, add:
```go
assert.Contains(t, result.Stdout, "/tmp")
```

In `TestIntegration_ContainerWithEntrypoint`, add:
```go
assert.Contains(t, result.Stdout, "test")
```

In `TestIntegration_ContainerWithUser`, add:
```go
assert.Contains(t, result.Stdout, "nobody")
```

**Step 2: Run tests**

Run: `task test:integration`
Expected: All tests pass with the new assertions.

**Step 3: Commit**

```
git add docker/integration_test.go
git commit -m "test(docker): add output content assertions to integration tests"
```

---

### Task 4: Add failure scenario tests

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Add container failure test**

```go
func TestIntegration_ContainerFailure(t *testing.T) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "exit 1"},
		AutoRemove: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-container-failure",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ContainerExecutionOutput
	// The workflow may return an error for non-zero exit codes
	_ = we.Get(ctx, &result)

	assert.False(t, result.Success)
	assert.Equal(t, 1, result.ExitCode)
}
```

**Step 2: Add pipeline StopOnError test**

```go
func TestIntegration_PipelineStopOnError(t *testing.T) {
	ctx := context.Background()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "exit 1"},
				AutoRemove: true,
				Name:       "failing-step",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "should not run"},
				AutoRemove: true,
				Name:       "skipped-step",
			},
		},
		StopOnError: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-pipeline-stop-on-error",
			TaskQueue: testTaskQueue,
		},
		workflow.ContainerPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.PipelineOutput
	_ = we.Get(ctx, &result)

	assert.GreaterOrEqual(t, result.TotalFailed, 1)
	assert.Equal(t, 0, result.TotalSuccess)
	// Only the first step should have run
	assert.Equal(t, 1, len(result.Results))
}
```

**Step 3: Add pipeline ContinueOnError test**

```go
func TestIntegration_PipelineContinueOnError(t *testing.T) {
	ctx := context.Background()

	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "exit 1"},
				AutoRemove: true,
				Name:       "failing-step",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "still running"},
				AutoRemove: true,
				Name:       "continuing-step",
			},
		},
		StopOnError: false,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-pipeline-continue-on-error",
			TaskQueue: testTaskQueue,
		},
		workflow.ContainerPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.PipelineOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 1, result.TotalFailed)
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Equal(t, 2, len(result.Results))
}
```

**Step 4: Add parallel FailFast test**

```go
func TestIntegration_ParallelFailFast(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "sleep 5 && echo done"},
				AutoRemove: true,
				Name:       "slow-task",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "exit 1"},
				AutoRemove: true,
				Name:       "failing-task",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "sleep 5 && echo done"},
				AutoRemove: true,
				Name:       "another-slow-task",
			},
		},
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-parallel-fail-fast",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelContainersWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	_ = we.Get(ctx, &result)

	assert.GreaterOrEqual(t, result.TotalFailed, 1)
}
```

**Step 5: Add parallel Continue test**

```go
func TestIntegration_ParallelContinue(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 1"},
				AutoRemove: true,
				Name:       "ok-task-1",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "exit 1"},
				AutoRemove: true,
				Name:       "failing-task",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 3"},
				AutoRemove: true,
				Name:       "ok-task-3",
			},
		},
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-parallel-continue",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelContainersWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ParallelOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 1, result.TotalFailed)
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 3, len(result.Results))
}
```

**Step 6: Run tests**

Run: `task test:integration`
Expected: All failure scenario tests pass.

**Step 7: Commit**

```
git add docker/integration_test.go
git commit -m "test(docker): add failure scenario integration tests"
```

---

### Task 5: Add DAG workflow integration tests

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Add DAG happy-path test**

```go
func TestIntegration_DAGWorkflow(t *testing.T) {
	ctx := context.Background()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "root",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "root node"},
						AutoRemove: true,
					},
				},
			},
			{
				Name: "branch-a",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "branch a"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"root"},
			},
			{
				Name: "branch-b",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "branch b"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"root"},
			},
		},
		FailFast: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-dag",
			TaskQueue: testTaskQueue,
		},
		workflow.DAGWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.DAGWorkflowOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
	assert.NotNil(t, result.Results["root"])
	assert.NotNil(t, result.Results["branch-a"])
	assert.NotNil(t, result.Results["branch-b"])
}
```

**Step 2: Add DAG fail-fast test**

```go
func TestIntegration_DAGWorkflowFailFast(t *testing.T) {
	ctx := context.Background()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "root",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "exit 1"},
						AutoRemove: true,
					},
				},
			},
			{
				Name: "dependent",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"echo", "should not run"},
						AutoRemove: true,
					},
				},
				Dependencies: []string{"root"},
			},
		},
		FailFast: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-dag-fail-fast",
			TaskQueue: testTaskQueue,
		},
		workflow.DAGWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.DAGWorkflowOutput
	_ = we.Get(ctx, &result)

	assert.GreaterOrEqual(t, result.TotalFailed, 1)
	// Dependent node should not have executed
	assert.Nil(t, result.Results["dependent"])
}
```

**Step 3: Run tests**

Run: `task test:integration`
Expected: DAG tests pass.

**Step 4: Commit**

```
git add docker/integration_test.go
git commit -m "test(docker): add DAG workflow integration tests"
```

---

### Task 6: Add Loop workflow integration tests

**Files:**
- Modify: `docker/integration_test.go`

**Step 1: Add sequential loop test**

```go
func TestIntegration_LoopSequential(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"alpha", "beta", "gamma"},
		Template: payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"echo", "{{item}}"},
			AutoRemove: true,
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-loop-sequential",
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
	assert.Len(t, result.Results, 3)
}
```

**Step 2: Add parallel loop test**

```go
func TestIntegration_LoopParallel(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"one", "two", "three"},
		Template: payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"echo", "{{item}}"},
			AutoRemove: true,
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-loop-parallel",
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
	assert.Len(t, result.Results, 3)
}
```

**Step 3: Run tests**

Run: `task test:integration`
Expected: All tests pass including the new loop tests.

**Step 4: Commit**

```
git add docker/integration_test.go
git commit -m "test(docker): add Loop workflow integration tests"
```

---

### Task 7: Final verification and lint

**Step 1: Run full integration test suite**

Run: `task test:integration`
Expected: All 18 tests pass (7 original + 2 DAG + 2 Loop + 5 failure + 2 assertion-only changes).

**Step 2: Run lint**

Run: `task lint`
Expected: Zero lint errors.

**Step 3: Run format**

Run: `task fmt`
Expected: No formatting changes needed.

**Step 4: Final commit (if any fmt changes)**

```
git add -A
git commit -m "style(tests): format integration tests"
```
