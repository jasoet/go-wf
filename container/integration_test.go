//go:build integration

package container_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/container"
	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/container/workflow"
	"github.com/jasoet/go-wf/workflow/testutil"
)

var (
	testClient    client.Client
	testTaskQueue = "integration-test-queue"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start Temporal container using shared helper
	tc, err := testutil.StartTemporalContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start temporal container: %v", err)
	}

	testClient = tc.Client

	// Create and start worker
	w := worker.New(testClient, testTaskQueue, worker.Options{})
	docker.RegisterAll(w)

	if err := w.Start(); err != nil {
		tc.Cleanup(ctx)
		log.Fatalf("Failed to start worker: %v", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	w.Stop()
	tc.Cleanup(ctx)

	os.Exit(code)
}

// TestIntegration_ExecuteContainerWorkflow tests single container execution with real Temporal server.
func TestIntegration_ExecuteContainerWorkflow(t *testing.T) {
	ctx := context.Background()

	// Execute workflow
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

	// Wait for result
	var result payload.ContainerExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	// Verify result
	assert.True(t, result.Success, "Expected successful execution, got: %+v", result)
	assert.Equal(t, 0, result.ExitCode, "Expected exit code 0")
	assert.NotEmpty(t, result.ContainerID, "Expected non-empty container ID")
	assert.Contains(t, result.Stdout, "integration test")
}

// TestIntegration_ContainerPipelineWorkflow tests pipeline execution with real Temporal server.
func TestIntegration_ContainerPipelineWorkflow(t *testing.T) {
	ctx := context.Background()

	// Execute pipeline workflow
	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "step 1"},
				AutoRemove: true,
				Name:       "step1",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "step 2"},
				AutoRemove: true,
				Name:       "step2",
			},
		},
		StopOnError: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-pipeline",
			TaskQueue: testTaskQueue,
		},
		workflow.ContainerPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	// Wait for result
	var result payload.PipelineOutput
	require.NoError(t, we.Get(ctx, &result))

	// Verify result
	assert.Equal(t, 2, result.TotalSuccess, "Expected 2 successful containers")
	assert.Equal(t, 0, result.TotalFailed, "Expected 0 failed containers")
	assert.Len(t, result.Results, 2, "Expected 2 results")
}

// TestIntegration_ParallelContainersWorkflow tests parallel execution with real Temporal server.
func TestIntegration_ParallelContainersWorkflow(t *testing.T) {
	ctx := context.Background()

	// Execute parallel workflow
	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 1"},
				AutoRemove: true,
				Name:       "task1",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 2"},
				AutoRemove: true,
				Name:       "task2",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 3"},
				AutoRemove: true,
				Name:       "task3",
			},
		},
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-parallel",
			TaskQueue: testTaskQueue,
		},
		workflow.ParallelContainersWorkflow,
		input,
	)
	require.NoError(t, err)

	// Wait for result
	var result payload.ParallelOutput
	require.NoError(t, we.Get(ctx, &result))

	// Verify result
	assert.Equal(t, 3, result.TotalSuccess, "Expected 3 successful containers")
	assert.Equal(t, 0, result.TotalFailed, "Expected 0 failed containers")
	assert.Len(t, result.Results, 3, "Expected 3 results")
}

// TestIntegration_ContainerWithEnvironment tests container with environment variables.
func TestIntegration_ContainerWithEnvironment(t *testing.T) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo $TEST_VAR"},
		Env:        map[string]string{"TEST_VAR": "test_value"},
		AutoRemove: true,
		Name:       "env-test",
		Labels:     map[string]string{"test": "integration"},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-env",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ContainerExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success, "Expected successful execution")
	assert.Equal(t, 0, result.ExitCode, "Expected exit code 0")
	assert.Contains(t, result.Stdout, "test_value")
}

// TestIntegration_ContainerWithWorkDir tests container with custom working directory.
func TestIntegration_ContainerWithWorkDir(t *testing.T) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"pwd"},
		WorkDir:    "/tmp",
		AutoRemove: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-workdir",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ContainerExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success, "Expected successful execution")
	assert.Contains(t, result.Stdout, "/tmp")
}

// TestIntegration_ContainerWithEntrypoint tests container with custom entrypoint.
func TestIntegration_ContainerWithEntrypoint(t *testing.T) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Entrypoint: []string{"/bin/sh", "-c"},
		Command:    []string{"echo test"},
		AutoRemove: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-entrypoint",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ContainerExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success, "Expected successful execution")
	assert.Contains(t, result.Stdout, "test")
}

// TestIntegration_ContainerWithUser tests container with custom user.
func TestIntegration_ContainerWithUser(t *testing.T) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"id"},
		User:       "nobody",
		AutoRemove: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "integration-test-user",
			TaskQueue: testTaskQueue,
		},
		workflow.ExecuteContainerWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.ContainerExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success, "Expected successful execution")
	assert.Contains(t, result.Stdout, "nobody")
}

// TestIntegration_ContainerFailure tests container with non-zero exit code.
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
	_ = we.Get(ctx, &result)

	assert.False(t, result.Success)
	assert.Equal(t, 1, result.ExitCode)
}

// TestIntegration_PipelineStopOnError tests pipeline stops on first failure.
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
	err = we.Get(ctx, &result)

	// Pipeline returns an error when StopOnError=true and a step fails
	assert.Error(t, err)
}

// TestIntegration_PipelineContinueOnError tests pipeline continues after failure.
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

// TestIntegration_ParallelFailFast tests parallel stops on first failure.
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
	err = we.Get(ctx, &result)

	// Parallel returns an error when FailureStrategy=fail_fast and a container fails
	assert.Error(t, err)
}

// TestIntegration_ParallelContinue tests parallel continues after failure.
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

// TestIntegration_DAGWorkflow tests DAG workflow with diamond dependency pattern.
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

// TestIntegration_DAGWorkflowFailFast tests DAG workflow stops when dependency fails.
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
	err = we.Get(ctx, &result)

	// The workflow returns an error when a dependency fails with FailFast=true
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dependency root failed")
}

// TestIntegration_LoopSequential tests sequential loop workflow.
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

// TestIntegration_LoopParallel tests parallel loop workflow.
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
			ID:        "integration-test-loop-sequential-fail-fast",
			TaskQueue: testTaskQueue,
		},
		workflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.LoopOutput
	err = we.Get(ctx, &result)

	// Sequential loop returns an error when FailureStrategy=fail_fast and an item fails
	assert.Error(t, err)
}

// TestIntegration_LoopParallelContinue tests parallel loop continues after failure.
func TestIntegration_LoopParallelContinue(t *testing.T) {
	ctx := context.Background()

	input := payload.LoopInput{
		Items: []string{"ok1", "fail", "ok2"},
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
	assert.Len(t, result.Results, 3)
}

// TestIntegration_ParameterizedLoop tests parameterized loop with cartesian product of parameters.
func TestIntegration_ParameterizedLoop(t *testing.T) {
	ctx := context.Background()

	input := payload.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template: payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"echo", "{{env}}-{{region}}"},
			AutoRemove: true,
		},
		Parallel:        false,
		FailureStrategy: "continue",
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
	assert.Len(t, result.Results, 4)
}

// TestIntegration_ParallelMaxConcurrency tests parallel execution with limited concurrency.
func TestIntegration_ParallelMaxConcurrency(t *testing.T) {
	ctx := context.Background()

	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 1"},
				AutoRemove: true,
				Name:       "conc-task-1",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 2"},
				AutoRemove: true,
				Name:       "conc-task-2",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 3"},
				AutoRemove: true,
				Name:       "conc-task-3",
			},
			{
				Image:      "alpine:latest",
				Command:    []string{"echo", "task 4"},
				AutoRemove: true,
				Name:       "conc-task-4",
			},
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
