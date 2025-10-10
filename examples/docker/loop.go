//go:build example

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/patterns"
	"github.com/jasoet/go-wf/docker/workflow"
	"go.temporal.io/sdk/client"
)

// This example demonstrates loop workflows in the go-wf/docker package.
// It shows how to:
// 1. Use simple withItems loops (parallel and sequential)
// 2. Use parameterized loops with multiple parameters
// 3. Use builder API for loops
// 4. Use pattern functions for common loop scenarios

func main() {
	// Create Temporal client
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalln("Unable to create Temporal client", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Example 1: Simple parallel loop (withItems)
	fmt.Println("\n=== Example 1: Simple Parallel Loop ===")
	simpleParallelLoopExample(ctx, c)

	// Example 2: Sequential loop (withItems)
	fmt.Println("\n=== Example 2: Sequential Loop ===")
	sequentialLoopExample(ctx, c)

	// Example 3: Parameterized loop (withParam) - Matrix deployment
	fmt.Println("\n=== Example 3: Parameterized Loop - Matrix Deployment ===")
	parameterizedLoopExample(ctx, c)

	// Example 4: Using Loop Builder
	fmt.Println("\n=== Example 4: Using Loop Builder ===")
	loopBuilderExample(ctx, c)

	// Example 5: Using Pattern Functions
	fmt.Println("\n=== Example 5: Using Pattern Functions ===")
	loopPatternExample(ctx, c)

	// Example 6: Batch Processing with Concurrency Limit
	fmt.Println("\n=== Example 6: Batch Processing ===")
	batchProcessingExample(ctx, c)
}

func simpleParallelLoopExample(ctx context.Context, c client.Client) {
	// Process multiple files in parallel
	files := []string{"data1.csv", "data2.csv", "data3.csv", "data4.csv"}

	input := docker.LoopInput{
		Items: files,
		Template: docker.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", "echo 'Processing file: {{item}} at index {{index}}' && sleep 1"},
			Env: map[string]string{
				"FILE_NAME": "{{item}}",
				"INDEX":     "{{index}}",
			},
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "loop-parallel-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.LoopWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started parallel loop workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.LoopOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Parallel loop completed: Success=%d, Failed=%d, Duration=%s\n",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func sequentialLoopExample(ctx context.Context, c client.Client) {
	// Deploy to regions sequentially (to control rate limiting)
	regions := []string{"us-west-1", "us-east-1", "eu-central-1"}

	input := docker.LoopInput{
		Items: regions,
		Template: docker.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", "echo 'Deploying to region: {{item}}' && sleep 2"},
			Env: map[string]string{
				"REGION":       "{{item}}",
				"DEPLOY_INDEX": "{{index}}",
			},
		},
		Parallel:        false, // Sequential execution
		FailureStrategy: "fail_fast",
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "loop-sequential-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.LoopWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started sequential loop workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.LoopOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Sequential loop completed: Success=%d, Failed=%d, Duration=%s\n",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func parameterizedLoopExample(ctx context.Context, c client.Client) {
	// Deploy to all combinations of environments and regions
	input := docker.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "staging", "prod"},
			"region": {"us-west", "us-east"},
		},
		Template: docker.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"sh", "-c", "echo 'Deploying to env={{.env}} region={{.region}}' && sleep 1"},
			Env: map[string]string{
				"ENVIRONMENT": "{{.env}}",
				"REGION":      "{{.region}}",
				"INDEX":       "{{index}}",
			},
		},
		Parallel:        true,
		FailureStrategy: "fail_fast",
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "loop-parameterized-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.ParameterizedLoopWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started parameterized loop workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.LoopOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Parameterized loop completed: Combinations=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func loopBuilderExample(ctx context.Context, c client.Client) {
	// Using loop builder for fluent API
	items := []string{"task-1", "task-2", "task-3"}

	template := docker.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo 'Executing task: {{item}}' && sleep 1"},
	}

	// Build loop using builder
	loopBuilder := builder.ForEach(items, template).
		Parallel(true).
		MaxConcurrency(2).
		FailFast(false)

	input, err := loopBuilder.BuildLoop()
	if err != nil {
		log.Fatalln("Unable to build loop", err)
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "loop-builder-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.LoopWorkflow, *input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started builder loop workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.LoopOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Builder loop completed: Success=%d, Failed=%d, Duration=%s\n",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func loopPatternExample(ctx context.Context, c client.Client) {
	// Using pattern function for matrix build
	buildMatrix := map[string][]string{
		"go_version": {"1.21", "1.22", "1.23"},
		"platform":   {"linux", "darwin"},
	}

	input, err := patterns.MatrixBuild(buildMatrix, "alpine:latest")
	if err != nil {
		log.Fatalln("Unable to create matrix build", err)
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "loop-matrix-build-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.ParameterizedLoopWorkflow, *input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started matrix build workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.LoopOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Matrix build completed: Builds=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func batchProcessingExample(ctx context.Context, c client.Client) {
	// Batch processing with concurrency limit
	batchFiles := []string{
		"batch1.json", "batch2.json", "batch3.json",
		"batch4.json", "batch5.json", "batch6.json",
	}

	input, err := patterns.BatchProcessing(batchFiles, "alpine:latest", 3)
	if err != nil {
		log.Fatalln("Unable to create batch processing", err)
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "loop-batch-processing-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.LoopWorkflow, *input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started batch processing workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.LoopOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Batch processing completed: Batches=%d, Success=%d, Failed=%d, Duration=%s\n",
		result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}
