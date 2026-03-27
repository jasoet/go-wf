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
	fnworkflow "github.com/jasoet/go-wf/function/workflow"
	generic "github.com/jasoet/go-wf/workflow"
	"github.com/jasoet/go-wf/workflow/testutil"
)

var (
	testClient    client.Client
	testTaskQueue = "fn-integration-test-queue"
)

func registerTestHandlers(registry *fn.Registry) {
	// echo — returns input.Args as Result
	_ = registry.Register("echo", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: input.Args,
		}, nil
	})

	// fail — returns error with message from args or default
	_ = registry.Register("fail", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		msg := input.Args["message"]
		if msg == "" {
			msg = "intentional failure"
		}
		return nil, fmt.Errorf("%s", msg)
	})

	// slow — sleeps 100ms, returns {"status": "completed"}
	_ = registry.Register("slow", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		time.Sleep(100 * time.Millisecond)
		return &fn.FunctionOutput{
			Result: map[string]string{"status": "completed"},
		}, nil
	})

	// maybeFailOnItem — fails if args["item"] == "bad", otherwise echoes args.
	_ = registry.Register("maybe-fail", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		if input.Args["item"] == "bad" {
			return nil, fmt.Errorf("item %s is bad", input.Args["item"])
		}
		return &fn.FunctionOutput{
			Result: input.Args,
		}, nil
	})

	// concat — returns {"item": input.Args["item"], "index": input.Args["index"]}
	_ = registry.Register("concat", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: map[string]string{
				"item":  input.Args["item"],
				"index": input.Args["index"],
			},
		}, nil
	})
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	tc, err := testutil.StartTemporalContainer(ctx)
	if err != nil {
		log.Fatalf("Failed to start temporal container: %v", err)
	}

	testClient = tc.Client

	registry := fn.NewRegistry()
	registerTestHandlers(registry)

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

// --- Single Function Tests ---

func TestIntegration_ExecuteFunctionWorkflow(t *testing.T) {
	ctx := context.Background()

	input := payload.FunctionExecutionInput{
		Name: "echo",
		Args: map[string]string{"greeting": "hello", "target": "world"},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-execute-function",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.ExecuteFunctionWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.FunctionExecutionOutput
	require.NoError(t, we.Get(ctx, &result))

	assert.True(t, result.Success, "Expected successful execution")
	assert.Equal(t, "echo", result.Name)
	assert.Equal(t, "hello", result.Result["greeting"])
	assert.Equal(t, "world", result.Result["target"])
	assert.Empty(t, result.Error)
}

func TestIntegration_ExecuteFunction_HandlerError(t *testing.T) {
	ctx := context.Background()

	input := payload.FunctionExecutionInput{
		Name: "fail",
		Args: map[string]string{"message": "something went wrong"},
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-execute-handler-error",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.ExecuteFunctionWorkflow,
		input,
	)
	require.NoError(t, err)

	var result payload.FunctionExecutionOutput
	err = we.Get(ctx, &result)
	// Handler errors are captured as business results; workflow itself should not return error
	require.NoError(t, err)

	assert.False(t, result.Success, "Expected failed execution")
	assert.Contains(t, result.Error, "something went wrong")
}

// --- Pipeline Tests ---

func TestIntegration_FunctionPipeline(t *testing.T) {
	ctx := context.Background()

	input := generic.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"step": "1"}},
			{Name: "echo", Args: map[string]string{"step": "2"}},
			{Name: "echo", Args: map[string]string{"step": "3"}},
		},
		StopOnError: false,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-pipeline",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.FunctionPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.PipelineOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_FunctionPipeline_StopOnError(t *testing.T) {
	ctx := context.Background()

	input := generic.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"step": "1"}},
			{Name: "fail", Args: map[string]string{"message": "pipeline failure"}},
			{Name: "echo", Args: map[string]string{"step": "3"}},
		},
		StopOnError: true,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-pipeline-stop-on-error",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.FunctionPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.PipelineOutput[payload.FunctionExecutionOutput]
	err = we.Get(ctx, &result)

	assert.Error(t, err)
}

func TestIntegration_FunctionPipeline_ContinueOnError(t *testing.T) {
	ctx := context.Background()

	input := generic.PipelineInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"step": "1"}},
			{Name: "fail", Args: map[string]string{"message": "pipeline failure"}},
			{Name: "echo", Args: map[string]string{"step": "3"}},
		},
		StopOnError: false,
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-pipeline-continue-on-error",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.FunctionPipelineWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.PipelineOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

// --- Parallel Tests ---

func TestIntegration_ParallelFunctions(t *testing.T) {
	ctx := context.Background()

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
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
		fnworkflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_ParallelFunctions_FailFast(t *testing.T) {
	ctx := context.Background()

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "slow"},
			{Name: "fail", Args: map[string]string{"message": "parallel failure"}},
			{Name: "slow"},
		},
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parallel-fail-fast",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	err = we.Get(ctx, &result)

	assert.Error(t, err)
}

func TestIntegration_ParallelFunctions_ContinueWithFailure(t *testing.T) {
	ctx := context.Background()

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
			{Name: "echo", Args: map[string]string{"task": "1"}},
			{Name: "fail", Args: map[string]string{"message": "parallel failure"}},
			{Name: "echo", Args: map[string]string{"task": "3"}},
		},
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parallel-continue",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_ParallelFunctions_MaxConcurrency(t *testing.T) {
	ctx := context.Background()

	input := generic.ParallelInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Tasks: []*payload.FunctionExecutionInput{
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
		fnworkflow.ParallelFunctionsWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.ParallelOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Len(t, result.Results, 4)
}

// --- Loop Tests ---

func TestIntegration_LoopSequential(t *testing.T) {
	ctx := context.Background()

	input := generic.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Items: []string{"alpha", "beta", "gamma"},
		Template: &payload.FunctionExecutionInput{
			Name: "concat",
			Args: map[string]string{"item": "{{item}}", "index": "{{index}}"},
		},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-sequential",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.LoopOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_LoopParallel(t *testing.T) {
	ctx := context.Background()

	input := generic.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Items: []string{"one", "two", "three"},
		Template: &payload.FunctionExecutionInput{
			Name: "echo",
			Args: map[string]string{"value": "{{item}}"},
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
		fnworkflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.LoopOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
	assert.Len(t, result.Results, 3)
}

func TestIntegration_LoopSequentialFailFast(t *testing.T) {
	ctx := context.Background()

	input := generic.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Items: []string{"ok", "bad", "skip"},
		Template: &payload.FunctionExecutionInput{
			Name: "maybe-fail",
			Args: map[string]string{"item": "{{item}}"},
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-sequential-fail-fast",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.LoopOutput[payload.FunctionExecutionOutput]
	err = we.Get(ctx, &result)

	assert.Error(t, err)
}

func TestIntegration_LoopParallelContinue(t *testing.T) {
	ctx := context.Background()

	input := generic.LoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Items: []string{"ok1", "bad", "ok2"},
		Template: &payload.FunctionExecutionInput{
			Name: "maybe-fail",
			Args: map[string]string{"item": "{{item}}"},
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-loop-parallel-continue",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.LoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.LoopOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
	assert.Len(t, result.Results, 3)
}

// --- Parameterized Loop Tests ---

func TestIntegration_ParameterizedLoop(t *testing.T) {
	ctx := context.Background()

	input := generic.ParameterizedLoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us", "eu"},
		},
		Template: &payload.FunctionExecutionInput{
			Name: "echo",
			Args: map[string]string{"env": "{{.env}}", "region": "{{.region}}"},
		},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	we, err := testClient.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "fn-integration-parameterized-loop",
			TaskQueue: testTaskQueue,
		},
		fnworkflow.ParameterizedLoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.LoopOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 4, result.ItemCount)
	assert.Len(t, result.Results, 4)
}

func TestIntegration_ParameterizedLoopParallel(t *testing.T) {
	ctx := context.Background()

	input := generic.ParameterizedLoopInput[*payload.FunctionExecutionInput, payload.FunctionExecutionOutput]{
		Parameters: map[string][]string{
			"size": {"small", "medium", "large"},
		},
		Template: &payload.FunctionExecutionInput{
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
		fnworkflow.ParameterizedLoopWorkflow,
		input,
	)
	require.NoError(t, err)

	var result generic.LoopOutput[payload.FunctionExecutionOutput]
	require.NoError(t, we.Get(ctx, &result))

	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
	assert.Equal(t, 3, result.ItemCount)
	assert.Len(t, result.Results, 3)
}
