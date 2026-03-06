# Example Code Coverage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Improve go-wf example coverage from ~55% to ~95% by adding 3 new example files and modifying 2 existing files.

**Architecture:** Add self-contained example files that embed their own Temporal worker. Each new file demonstrates a distinct API category (operations, patterns, builder-advanced). Two existing files get new functions appended for uncovered features.

**Tech Stack:** Go 1.25, Temporal SDK, go-wf docker package (builder, template, patterns, operations, payload)

---

### Task 1: Create `operations.go` — Workflow Lifecycle Management

**Files:**
- Create: `examples/docker/operations.go`

**Step 1: Create the operations example file**

This file demonstrates all 8 functions from `docker/operations.go`. It is self-contained with an embedded worker.

```go
//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/docker/workflow"
)

// This example demonstrates the workflow operations API for managing
// workflow lifecycle: submission, status checking, cancellation,
// termination, signaling, querying, and watching.

func main() {
	// Create Temporal client
	c, closer, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	// Create and start worker
	w := worker.New(c, "docker-tasks", worker.Options{})
	docker.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Example 1: Submit and check status
	log.Println("=== Example 1: Submit and Check Status ===")
	submitAndCheckStatus(c)

	time.Sleep(2 * time.Second)

	// Example 2: Submit and wait with timeout
	log.Println("\n=== Example 2: Submit and Wait ===")
	submitWithTimeout(c)

	time.Sleep(2 * time.Second)

	// Example 3: Cancel and terminate workflows
	log.Println("\n=== Example 3: Cancel and Terminate ===")
	cancelAndTerminate(c)

	time.Sleep(2 * time.Second)

	// Example 4: Watch workflow updates
	log.Println("\n=== Example 4: Watch Workflow Updates ===")
	watchWorkflowUpdates(c)
}

// Example 1: SubmitWorkflow + GetWorkflowStatus
func submitAndCheckStatus(c client.Client) {
	ctx := context.Background()

	input := payload.PipelineInput{
		StopOnError: true,
		Containers: []payload.ContainerExecutionInput{
			{
				Name:       "step-1",
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "echo 'Step 1 running' && sleep 2"},
				AutoRemove: true,
			},
			{
				Name:       "step-2",
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "echo 'Step 2 running' && sleep 1"},
				AutoRemove: true,
			},
		},
	}

	// Submit workflow (fire-and-forget)
	status, err := docker.SubmitWorkflow(ctx, c, input, "docker-tasks")
	if err != nil {
		log.Printf("Failed to submit workflow: %v", err)
		return
	}
	log.Printf("Submitted workflow: ID=%s, RunID=%s", status.WorkflowID, status.RunID)

	// Check status while running
	time.Sleep(time.Second)
	currentStatus, err := docker.GetWorkflowStatus(ctx, c, status.WorkflowID, status.RunID)
	if err != nil {
		log.Printf("Failed to get status: %v", err)
		return
	}
	log.Printf("Current status: %s (started: %s)", currentStatus.Status, currentStatus.StartTime.Format(time.RFC3339))

	// Wait for completion
	time.Sleep(5 * time.Second)
	finalStatus, err := docker.GetWorkflowStatus(ctx, c, status.WorkflowID, status.RunID)
	if err != nil {
		log.Printf("Failed to get final status: %v", err)
		return
	}
	log.Printf("Final status: %s", finalStatus.Status)
}

// Example 2: SubmitAndWait with timeout
func submitWithTimeout(c client.Client) {
	ctx := context.Background()

	input := payload.ContainerExecutionInput{
		Name:       "quick-task",
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Quick task done' && sleep 1"},
		AutoRemove: true,
	}

	// Submit and wait with 30 second timeout
	status, err := docker.SubmitAndWait(ctx, c, input, "docker-tasks", 30*time.Second)
	if err != nil {
		log.Printf("SubmitAndWait failed: %v", err)
		return
	}
	log.Printf("Workflow completed: ID=%s, Status=%s", status.WorkflowID, status.Status)
}

// Example 3: CancelWorkflow + TerminateWorkflow
func cancelAndTerminate(c client.Client) {
	ctx := context.Background()

	// Start a long-running workflow
	longInput := payload.ContainerExecutionInput{
		Name:       "long-task-cancel",
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Long task started' && sleep 60"},
		AutoRemove: true,
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "cancel-demo-workflow",
			TaskQueue: "docker-tasks",
		},
		workflow.ExecuteContainerWorkflow,
		longInput,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}
	log.Printf("Started long-running workflow: %s", we.GetID())

	// Cancel it after 2 seconds
	time.Sleep(2 * time.Second)
	err = docker.CancelWorkflow(ctx, c, we.GetID(), we.GetRunID())
	if err != nil {
		log.Printf("Cancel failed: %v", err)
	} else {
		log.Println("Workflow cancelled successfully")
	}

	// Start another for terminate demo
	longInput.Name = "long-task-terminate"
	we2, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "terminate-demo-workflow",
			TaskQueue: "docker-tasks",
		},
		workflow.ExecuteContainerWorkflow,
		longInput,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}
	log.Printf("Started another long-running workflow: %s", we2.GetID())

	// Terminate it after 2 seconds
	time.Sleep(2 * time.Second)
	err = docker.TerminateWorkflow(ctx, c, we2.GetID(), we2.GetRunID(), "demo termination")
	if err != nil {
		log.Printf("Terminate failed: %v", err)
	} else {
		log.Println("Workflow terminated successfully")
	}
}

// Example 4: SignalWorkflow + QueryWorkflow + WatchWorkflow
func watchWorkflowUpdates(c client.Client) {
	ctx := context.Background()

	input := payload.PipelineInput{
		StopOnError: true,
		Containers: []payload.ContainerExecutionInput{
			{
				Name:       "watch-step-1",
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "echo 'Watch step 1' && sleep 2"},
				AutoRemove: true,
			},
			{
				Name:       "watch-step-2",
				Image:      "alpine:latest",
				Command:    []string{"sh", "-c", "echo 'Watch step 2' && sleep 2"},
				AutoRemove: true,
			},
		},
	}

	we, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "watch-demo-workflow",
			TaskQueue: "docker-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		input,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}
	log.Printf("Started workflow for watching: %s", we.GetID())

	// Watch workflow updates via channel
	updates := make(chan *docker.WorkflowStatus, 10)
	go func() {
		if err := docker.WatchWorkflow(ctx, c, we.GetID(), we.GetRunID(), updates); err != nil {
			log.Printf("Watch error: %v", err)
		}
	}()

	// Print updates as they arrive
	for update := range updates {
		log.Printf("  Update: Status=%s", update.Status)
		if update.CloseTime != nil {
			log.Printf("  Workflow completed at: %s", update.CloseTime.Format(time.RFC3339))
			break
		}
	}

	// Signal a new workflow (demonstrate SignalWorkflow)
	signalInput := payload.ContainerExecutionInput{
		Name:       "signal-target",
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Signal target' && sleep 5"},
		AutoRemove: true,
	}

	we3, err := c.ExecuteWorkflow(ctx,
		client.StartWorkflowOptions{
			ID:        "signal-demo-workflow",
			TaskQueue: "docker-tasks",
		},
		workflow.ExecuteContainerWorkflow,
		signalInput,
	)
	if err != nil {
		log.Printf("Failed to start signal target: %v", err)
		return
	}

	time.Sleep(time.Second)
	err = docker.SignalWorkflow(ctx, c, we3.GetID(), we3.GetRunID(), "my-signal", map[string]string{"action": "proceed"})
	if err != nil {
		log.Printf("Signal failed: %v", err)
	} else {
		log.Println("Signal sent successfully")
	}

	// Query workflow
	var queryResult interface{}
	err = docker.QueryWorkflow(ctx, c, we3.GetID(), we3.GetRunID(), "__stack_trace", &queryResult)
	if err != nil {
		log.Printf("Query failed (expected for some query types): %v", err)
	} else {
		log.Printf("Query result received")
	}
}
```

**Step 2: Verify the file compiles**

Run: `go build -tags example examples/docker/operations.go`
Expected: No compilation errors

**Step 3: Commit**

```bash
git add examples/docker/operations.go
git commit -m "feat(examples): add operations API example covering workflow lifecycle"
```

---

### Task 2: Create `patterns.go` — Pre-built Pattern Functions

**Files:**
- Create: `examples/docker/patterns-demo.go`

Note: Named `patterns-demo.go` to avoid collision with the `patterns` package import.

**Step 1: Create the patterns demo file**

```go
//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/patterns"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/docker/template"
	"github.com/jasoet/go-wf/docker/workflow"
)

// This example demonstrates all 16 pre-built pattern functions from the
// docker/patterns package: CI/CD patterns, parallel patterns, and loop patterns.

func main() {
	// Create Temporal client
	c, closer, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	// Create and start worker
	w := worker.New(c, "docker-tasks", worker.Options{})
	docker.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// CI/CD patterns
	log.Println("=== CI/CD Patterns ===")
	runCICDPatterns(c)

	time.Sleep(2 * time.Second)

	// Parallel patterns
	log.Println("\n=== Parallel Patterns ===")
	runParallelPatterns(c)

	time.Sleep(2 * time.Second)

	// Loop patterns
	log.Println("\n=== Loop Patterns ===")
	runLoopPatterns(c)
}

// runCICDPatterns demonstrates all 4 CI/CD pattern functions
func runCICDPatterns(c client.Client) {
	ctx := context.Background()

	// 1. BuildTestDeploy — basic CI/CD pipeline
	log.Println("  1. BuildTestDeploy")
	btdInput, err := patterns.BuildTestDeploy("alpine:latest", "alpine:latest", "alpine:latest")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-btd", TaskQueue: "docker-tasks"},
			workflow.ContainerPipelineWorkflow, *btdInput)
		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d, Failed=%d", result.TotalSuccess, result.TotalFailed)
		}
	}

	time.Sleep(time.Second)

	// 2. BuildTestDeployWithHealthCheck — pipeline with health check
	log.Println("  2. BuildTestDeployWithHealthCheck")
	hcInput, err := patterns.BuildTestDeployWithHealthCheck("alpine:latest", "alpine:latest", "https://httpbin.org/status/200")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-btd-hc", TaskQueue: "docker-tasks"},
			workflow.ContainerPipelineWorkflow, *hcInput)
		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 3. BuildTestDeployWithNotification — pipeline with webhook notification
	log.Println("  3. BuildTestDeployWithNotification")
	notifyInput, err := patterns.BuildTestDeployWithNotification("alpine:latest", "alpine:latest", "https://httpbin.org/post", "Deploy complete")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-btd-notify", TaskQueue: "docker-tasks"},
			workflow.ContainerPipelineWorkflow, *notifyInput)
		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 4. MultiEnvironmentDeploy — deploy to multiple environments
	log.Println("  4. MultiEnvironmentDeploy")
	meInput, err := patterns.MultiEnvironmentDeploy("alpine:latest", []string{"dev", "staging", "production"})
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-multi-env", TaskQueue: "docker-tasks"},
			workflow.ContainerPipelineWorkflow, *meInput)
		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}
}

// runParallelPatterns demonstrates all 5 parallel pattern functions
func runParallelPatterns(c client.Client) {
	ctx := context.Background()

	// 1. FanOutFanIn — distribute tasks then collect
	log.Println("  1. FanOutFanIn")
	fofiInput, err := patterns.FanOutFanIn("alpine:latest", []string{"analyze", "transform", "validate"})
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-fanout", TaskQueue: "docker-tasks"},
			workflow.ParallelContainersWorkflow, *fofiInput)
		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 2. ParallelDataProcessing — process data items concurrently
	log.Println("  2. ParallelDataProcessing")
	pdpInput, err := patterns.ParallelDataProcessing("alpine:latest", []string{"file1.csv", "file2.csv", "file3.csv"}, "echo 'Processing'")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-data-proc", TaskQueue: "docker-tasks"},
			workflow.ParallelContainersWorkflow, *pdpInput)
		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 3. ParallelTestSuite — run test suites concurrently
	log.Println("  3. ParallelTestSuite")
	ptsInput, err := patterns.ParallelTestSuite("alpine:latest", map[string]string{
		"unit":        "echo 'Unit tests passed'",
		"integration": "echo 'Integration tests passed'",
		"e2e":         "echo 'E2E tests passed'",
	})
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-test-suite", TaskQueue: "docker-tasks"},
			workflow.ParallelContainersWorkflow, *ptsInput)
		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 4. ParallelDeployment — deploy to regions concurrently
	log.Println("  4. ParallelDeployment")
	pdInput, err := patterns.ParallelDeployment("alpine:latest", []string{"us-west-1", "eu-central-1", "ap-southeast-1"})
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-parallel-deploy", TaskQueue: "docker-tasks"},
			workflow.ParallelContainersWorkflow, *pdInput)
		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 5. MapReduce — map then reduce pipeline
	log.Println("  5. MapReduce")
	mrInput, err := patterns.MapReduce("alpine:latest",
		[]string{"chunk1", "chunk2", "chunk3"},
		"echo 'Mapping'",
		"echo 'Reducing'")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		// MapReduce returns PipelineInput (map parallel + reduce sequential)
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-mapreduce", TaskQueue: "docker-tasks"},
			workflow.ContainerPipelineWorkflow, *mrInput)
		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}
}

// runLoopPatterns demonstrates all 7 loop pattern functions
func runLoopPatterns(c client.Client) {
	ctx := context.Background()

	// 1. ParallelLoop — process items in parallel
	log.Println("  1. ParallelLoop")
	plInput, err := patterns.ParallelLoop([]string{"item-a", "item-b", "item-c"}, "alpine:latest", "echo 'Processing'")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-par-loop", TaskQueue: "docker-tasks"},
			workflow.LoopWorkflow, *plInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 2. SequentialLoop — process items one by one
	log.Println("  2. SequentialLoop")
	slInput, err := patterns.SequentialLoop([]string{"step-1", "step-2", "step-3"}, "alpine:latest", "echo 'Sequential step'")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-seq-loop", TaskQueue: "docker-tasks"},
			workflow.LoopWorkflow, *slInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 3. BatchProcessing — process with concurrency limits
	log.Println("  3. BatchProcessing")
	bpInput, err := patterns.BatchProcessing([]string{"batch1.json", "batch2.json", "batch3.json", "batch4.json"}, "alpine:latest", 2)
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-batch", TaskQueue: "docker-tasks"},
			workflow.LoopWorkflow, *bpInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 4. MultiRegionDeployment — parameterized env × region matrix
	log.Println("  4. MultiRegionDeployment")
	mrdInput, err := patterns.MultiRegionDeployment(
		[]string{"staging", "production"},
		[]string{"us-west", "eu-central"},
		"alpine:latest")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-multi-region", TaskQueue: "docker-tasks"},
			workflow.ParameterizedLoopWorkflow, *mrdInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Combinations=%d, Success=%d", result.ItemCount, result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 5. MatrixBuild — build matrix (already covered in loop.go, shown here for completeness)
	log.Println("  5. MatrixBuild")
	mbInput, err := patterns.MatrixBuild(map[string][]string{
		"os":         {"linux", "darwin"},
		"go_version": {"1.22", "1.23"},
	}, "alpine:latest")
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-matrix", TaskQueue: "docker-tasks"},
			workflow.ParameterizedLoopWorkflow, *mbInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Combinations=%d, Success=%d", result.ItemCount, result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 6. ParameterSweep — hyperparameter sweep with concurrency
	log.Println("  6. ParameterSweep")
	psInput, err := patterns.ParameterSweep(map[string][]string{
		"learning_rate": {"0.001", "0.01"},
		"batch_size":    {"32", "64"},
	}, "alpine:latest", 2)
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-sweep", TaskQueue: "docker-tasks"},
			workflow.ParameterizedLoopWorkflow, *psInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Combinations=%d, Success=%d", result.ItemCount, result.TotalSuccess)
		}
	}

	time.Sleep(time.Second)

	// 7. ParallelLoopWithTemplate — loop using WorkflowSource template
	log.Println("  7. ParallelLoopWithTemplate")
	templateSource := template.NewBashScript("process-item",
		`echo "Processing item: {{item}}"`,
		template.WithScriptAutoRemove(true))
	pltInput, err := patterns.ParallelLoopWithTemplate(
		[]string{"alpha", "beta", "gamma"},
		templateSource)
	if err != nil {
		log.Printf("    Failed: %v", err)
	} else {
		we, _ := c.ExecuteWorkflow(ctx,
			client.StartWorkflowOptions{ID: "pattern-loop-template", TaskQueue: "docker-tasks"},
			workflow.LoopWorkflow, *pltInput)
		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Printf("    Failed: %v", err)
		} else {
			log.Printf("    Completed: Success=%d", result.TotalSuccess)
		}
	}

	// Unused import suppression
	_ = builder.NewWorkflowBuilder
	_ = fmt.Sprintf
}
```

**Step 2: Verify the file compiles**

Run: `go build -tags example examples/docker/patterns-demo.go`
Expected: No compilation errors

**Step 3: Commit**

```bash
git add examples/docker/patterns-demo.go
git commit -m "feat(examples): add patterns demo covering all 16 pre-built pattern functions"
```

---

### Task 3: Create `builder-advanced.go` — Advanced Builder/Template APIs

**Files:**
- Create: `examples/docker/builder-advanced.go`

**Step 1: Create the advanced builder example file**

```go
//go:build example

package main

import (
	"context"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/docker/template"
	"github.com/jasoet/go-wf/docker/workflow"
)

// This example demonstrates advanced builder, template, and source APIs
// not covered in builder.go, including BuildSingle, Build (auto-select),
// Cleanup, constructor options, ContainerSource, AddInput, ForEachParam,
// ParameterizedLoopBuilder, NewGoScript, NewHTTPWebhook, and more
// container options like WithVolume, WithPorts, WithLabel.

func main() {
	// Create Temporal client
	c, closer, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	// Create and start worker
	w := worker.New(c, "docker-tasks", worker.Options{})
	docker.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	// Example 1: BuildSingle + constructor options
	log.Println("=== Example 1: BuildSingle + Constructor Options ===")
	runSingleContainerBuilder(c)

	time.Sleep(2 * time.Second)

	// Example 2: Build() auto-select + ContainerSource + AddInput
	log.Println("\n=== Example 2: Auto-Select Builder + ContainerSource ===")
	runAutoSelectBuilder(c)

	time.Sleep(2 * time.Second)

	// Example 3: Cleanup pipeline with rich container options
	log.Println("\n=== Example 3: Cleanup Pipeline ===")
	runCleanupPipeline(c)

	time.Sleep(2 * time.Second)

	// Example 4: ForEachParam + ParameterizedLoopBuilder
	log.Println("\n=== Example 4: Parameterized Loop Builder ===")
	runParameterizedLoopBuilder(c)

	time.Sleep(2 * time.Second)

	// Example 5: NewGoScript + NewHTTPWebhook + more container options
	log.Println("\n=== Example 5: Additional Templates ===")
	runAdditionalTemplates(c)
}

// Example 1: BuildSingle — build a single container execution input
func runSingleContainerBuilder(c client.Client) {
	// Use constructor options to configure the builder
	singleInput, err := builder.NewWorkflowBuilder("single-demo",
		builder.WithStopOnError(true),
		builder.WithGlobalAutoRemove(true),
		builder.WithGlobalTimeout(2*time.Minute),
	).
		Add(template.NewContainer("single-task", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Single container via BuildSingle' && sleep 1"))).
		BuildSingle()
	if err != nil {
		log.Printf("BuildSingle failed: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "build-single-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ExecuteContainerWorkflow,
		*singleInput,
	)

	var result payload.ContainerExecutionOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Failed: %v", err)
		return
	}
	log.Printf("BuildSingle completed: ExitCode=%d", result.ExitCode)
}

// Example 2: Build() auto-select + ContainerSource + AddInput
func runAutoSelectBuilder(c client.Client) {
	// ContainerSource wraps a raw payload as WorkflowSource
	rawInput := payload.ContainerExecutionInput{
		Name:       "raw-container",
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'From ContainerSource'"},
		AutoRemove: true,
	}
	source := builder.NewContainerSource(rawInput)

	// Build with auto-select: single container → pipeline or parallel
	wb := builder.NewWorkflowBuilder("auto-select-demo",
		builder.WithParallelMode(true),
		builder.WithMaxConcurrency(2),
	)

	// Add via WorkflowSource
	wb.Add(source)

	// Add via AddInput (raw ContainerExecutionInput)
	wb.AddInput(payload.ContainerExecutionInput{
		Name:       "direct-input",
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'From AddInput'"},
		AutoRemove: true,
	})

	// Add another template
	wb.Add(template.NewBashScript("script-task",
		`echo "From template source"`,
		template.WithScriptAutoRemove(true)))

	// Build() auto-selects parallel since ParallelMode is true
	result, err := wb.Build()
	if err != nil {
		log.Printf("Build failed: %v", err)
		return
	}

	parallelInput, ok := result.(*payload.ParallelInput)
	if !ok {
		log.Printf("Expected ParallelInput, got %T", result)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "auto-select-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ParallelContainersWorkflow,
		*parallelInput,
	)

	var output payload.ParallelOutput
	if err := we.Get(context.Background(), &output); err != nil {
		log.Printf("Failed: %v", err)
		return
	}
	log.Printf("Auto-select completed: Success=%d, Failed=%d", output.TotalSuccess, output.TotalFailed)
}

// Example 3: Pipeline with Cleanup + rich container options
func runCleanupPipeline(c client.Client) {
	input, err := builder.NewWorkflowBuilder("cleanup-demo",
		builder.WithCleanup(true),
	).
		Add(template.NewContainer("setup-service", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Starting service' && sleep 1"),
			template.WithVolume("/tmp/shared-data", "/data"),
			template.WithPorts("8080:80", "9090:90"),
			template.WithLabel("app", "go-wf-demo"),
			template.WithLabel("tier", "backend"),
			template.WithWaitForLog("Starting service"),
		)).
		Add(template.NewContainer("run-tests", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Running tests against service' && sleep 1"),
			template.WithWaitForPort("80"),
		)).
		Add(template.NewContainer("teardown", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Cleaning up' && sleep 1"),
		)).
		StopOnError(true).
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build cleanup pipeline: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "cleanup-pipeline-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		*input,
	)

	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Failed: %v", err)
		return
	}
	log.Printf("Cleanup pipeline completed: Success=%d, Cleanup=%v", result.TotalSuccess, true)
}

// Example 4: ForEachParam + NewParameterizedLoopBuilder
func runParameterizedLoopBuilder(c client.Client) {
	// ForEachParam — convenience for parameterized loops
	paramTemplate := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Region={{.region}} Size={{.size}}'"},
		AutoRemove: true,
	}

	loopInput, err := builder.ForEachParam(
		map[string][]string{
			"region": {"us-west", "eu-central"},
			"size":   {"small", "large"},
		},
		paramTemplate,
	).
		Parallel(true).
		MaxConcurrency(2).
		FailFast(true).
		BuildParameterizedLoop()
	if err != nil {
		log.Printf("ForEachParam failed: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "foreach-param-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ParameterizedLoopWorkflow,
		*loopInput,
	)

	var result payload.LoopOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Failed: %v", err)
		return
	}
	log.Printf("ForEachParam completed: Combinations=%d, Success=%d", result.ItemCount, result.TotalSuccess)

	time.Sleep(time.Second)

	// NewParameterizedLoopBuilder — explicit builder with WithTemplate + WithSource
	log.Println("  Using NewParameterizedLoopBuilder with WithTemplate:")
	plbInput, err := builder.NewParameterizedLoopBuilder(map[string][]string{
		"env":  {"dev", "prod"},
		"arch": {"amd64", "arm64"},
	}).
		WithTemplate(payload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"sh", "-c", "echo 'Building env={{.env}} arch={{.arch}}'"},
			AutoRemove: true,
		}).
		Parallel(true).
		BuildParameterizedLoop()
	if err != nil {
		log.Printf("NewParameterizedLoopBuilder failed: %v", err)
		return
	}

	we2, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "param-loop-builder-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ParameterizedLoopWorkflow,
		*plbInput,
	)

	var result2 payload.LoopOutput
	if err := we2.Get(context.Background(), &result2); err != nil {
		log.Printf("Failed: %v", err)
		return
	}
	log.Printf("  Completed: Combinations=%d, Success=%d", result2.ItemCount, result2.TotalSuccess)
}

// Example 5: NewGoScript + NewHTTPWebhook + additional container options
func runAdditionalTemplates(c client.Client) {
	input, err := builder.NewWorkflowBuilder("templates-demo").
		// Go script template
		Add(template.NewGoScript("go-hello",
			`package main
import "fmt"
func main() {
	fmt.Println("Hello from Go script!")
}`)).
		// HTTP webhook template
		Add(template.NewHTTPWebhook("notify-webhook",
			"https://httpbin.org/post",
			`{"event":"deploy","status":"success"}`,
			template.WithHTTPHeader("Content-Type", "application/json"),
			template.WithHTTPExpectedStatus(200))).
		// Container with additional options
		Add(template.NewContainer("configured-container", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Configured container running'"),
			template.WithVolume("/tmp/host-data", "/container-data"),
			template.WithPorts("3000:3000"),
			template.WithLabel("version", "1.0"),
			template.WithEnv("LOG_LEVEL", "debug"),
			template.WithAutoRemove(true),
		)).
		// Script with volume and ports
		Add(template.NewBashScript("script-with-extras",
			`echo "Script with volume and port bindings"`,
			template.WithScriptVolume("/tmp/script-data", "/data"),
			template.WithScriptPorts("4000:4000"),
			template.WithScriptEnv("MODE", "advanced"),
		)).
		StopOnError(false).
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build templates pipeline: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "additional-templates-example",
			TaskQueue: "docker-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		*input,
	)

	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Failed: %v", err)
		return
	}
	log.Printf("Additional templates completed: Success=%d, Failed=%d", result.TotalSuccess, result.TotalFailed)
}
```

**Step 2: Verify the file compiles**

Run: `go build -tags example examples/docker/builder-advanced.go`
Expected: No compilation errors

**Step 3: Commit**

```bash
git add examples/docker/builder-advanced.go
git commit -m "feat(examples): add advanced builder/template example with BuildSingle, ContainerSource, GoScript"
```

---

### Task 4: Add `runRetryAndSecretsDemo()` to `advanced.go`

**Files:**
- Modify: `examples/docker/advanced.go`

**Step 1: Add Example 5 call to main()**

In `examples/docker/advanced.go`, after the Example 4 block (line ~65), add:

```go
	time.Sleep(2 * time.Second)

	// Example 5: Retry, Secrets, DependsOn, File Outputs
	log.Println("\nExample 5: Retry & Secrets")
	runRetryAndSecretsDemo(c)
```

**Step 2: Add the `runRetryAndSecretsDemo` function**

Append after `runWaitStrategiesDemo` function (after line 340):

```go

// Example 5: Retry Configuration, Secrets, DependsOn, File-based Outputs
func runRetryAndSecretsDemo(c client.Client) {
	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "setup",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'v2.0.0' > /tmp/version.txt && echo 'Setup done'"},
						AutoRemove: true,
					},
					// Retry configuration
					RetryAttempts: 3,
					RetryDelay:    5 * time.Second,
					// File-based output extraction
					Outputs: []payload.OutputDefinition{
						{
							Name:      "version",
							ValueFrom: "file",
							Path:      "/tmp/version.txt",
							Default:   "unknown",
						},
						{
							Name:      "status",
							ValueFrom: "stdout",
							Regex:     `(\w+) done`,
						},
					},
				},
			},
			{
				Name: "deploy",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo \"Deploying version $APP_VERSION with secret $DB_PASSWORD\""},
						AutoRemove: true,
					},
					// Secret references (struct showcase)
					Secrets: []payload.SecretReference{
						{
							Name:   "db-credentials",
							Key:    "password",
							EnvVar: "DB_PASSWORD",
						},
						{
							Name:   "api-keys",
							Key:    "primary",
							EnvVar: "API_KEY",
						},
					},
					// DependsOn for container-level dependencies
					DependsOn: []string{"setup"},
					// Input from previous step
					Inputs: []payload.InputMapping{
						{
							Name:     "APP_VERSION",
							From:     "setup.version",
							Required: true,
						},
					},
				},
				Dependencies: []string{"setup"},
			},
		},
		FailFast: true,
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "retry-secrets-demo",
			TaskQueue: "docker-tasks",
		},
		workflow.DAGWorkflow,
		input,
	)

	var result payload.DAGWorkflowOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Retry & secrets demo failed: %v", err)
		return
	}

	log.Printf("Retry & secrets demo completed: Success=%d", result.TotalSuccess)
	for _, nodeResult := range result.NodeResults {
		log.Printf("  Node %s: Success=%v", nodeResult.NodeName, nodeResult.Success)
	}
}
```

**Step 3: Verify compilation**

Run: `go build -tags example examples/docker/advanced.go`
Expected: No compilation errors

**Step 4: Commit**

```bash
git add examples/docker/advanced.go
git commit -m "feat(examples): add retry, secrets, and file output demo to advanced.go"
```

---

### Task 5: Add `artifactCleanupExample()` to `artifacts.go`

**Files:**
- Modify: `examples/docker/artifacts.go`

**Step 1: Add Example 4 call to main()**

In `examples/docker/artifacts.go`, after Example 3 (line ~47), add:

```go

	// Example 4: Archive artifacts with cleanup config
	fmt.Println("\n=== Example 4: Archive Artifacts & Cleanup Config ===")
	artifactCleanupExample(ctx, c)
```

**Step 2: Add the `artifactCleanupExample` function**

Append after `minioArtifactStorage` function (after line 369):

```go

func artifactCleanupExample(ctx context.Context, c client.Client) {
	// Create local file store
	store, err := artifacts.NewLocalFileStore("/tmp/workflow-artifacts")
	if err != nil {
		log.Fatalln("Failed to create artifact store", err)
	}
	defer store.Close()

	// Showcase ArtifactConfig with cleanup settings
	artifactConfig := artifacts.ArtifactConfig{
		Store:         store,
		WorkflowID:    "archive-cleanup-demo",
		RunID:         "run-001",
		EnableCleanup: true,
		RetentionDays: 7,
	}

	fmt.Printf("Artifact config: Cleanup=%v, Retention=%d days\n",
		artifactConfig.EnableCleanup, artifactConfig.RetentionDays)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build-archive",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image: "alpine:latest",
						Command: []string{"sh", "-c",
							"mkdir -p /tmp/build-output && " +
								"echo 'binary content' > /tmp/build-output/app && " +
								"echo 'config content' > /tmp/build-output/config.yaml && " +
								"echo 'Archive created'"},
						AutoRemove: true,
					},
					// Archive artifact type — tars entire directory
					OutputArtifacts: []payload.Artifact{
						{
							Name: "build-archive",
							Path: "/tmp/build-output",
							Type: "archive",
						},
					},
				},
			},
			{
				Name: "use-archive",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "ls -la /tmp/extracted/ && echo 'Archive extracted'"},
						AutoRemove: true,
					},
					// Download and extract the archive
					InputArtifacts: []payload.Artifact{
						{
							Name: "build-archive",
							Path: "/tmp/extracted",
							Type: "archive",
						},
					},
				},
				Dependencies: []string{"build-archive"},
			},
		},
		ArtifactStore: store,
		FailFast:      true,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "archive-cleanup-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s\n", we.GetID())

	var result payload.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Archive workflow completed: Success=%d, Failed=%d\n",
		result.TotalSuccess, result.TotalFailed)

	// Demonstrate cleanup reference
	fmt.Println("\nTo cleanup workflow artifacts programmatically:")
	fmt.Println("  artifacts.CleanupWorkflowArtifacts(ctx, store, workflowID, runID)")
}
```

**Step 3: Verify compilation**

Run: `go build -tags example examples/docker/artifacts.go`
Expected: No compilation errors

**Step 4: Commit**

```bash
git add examples/docker/artifacts.go
git commit -m "feat(examples): add archive artifact and cleanup config demo to artifacts.go"
```

---

### Task 6: Update `README.md`

**Files:**
- Modify: `examples/docker/README.md`

**Step 1: Add new example descriptions**

After the "Advanced Features" section (section 9, ending around line 347), add these new sections:

```markdown

---

### 10. Operations API (`operations.go`)

**Purpose**: Demonstrates the workflow lifecycle management API.

**Features**:
- Fire-and-forget workflow submission (`SubmitWorkflow`)
- Submit and wait with timeout (`SubmitAndWait`)
- Query workflow status (`GetWorkflowStatus`)
- Cancel running workflows (`CancelWorkflow`)
- Force terminate workflows (`TerminateWorkflow`)
- Signal workflows (`SignalWorkflow`)
- Query workflow state (`QueryWorkflow`)
- Watch workflow updates via channel (`WatchWorkflow`)

**Examples Included**:
1. **Submit and Check Status**: Submit pipeline, poll status
2. **Submit with Timeout**: SubmitAndWait with 30s timeout
3. **Cancel and Terminate**: Cancel and terminate long-running workflows
4. **Watch Updates**: Stream workflow status changes, signal, query

**Run**:
```bash
go run -tags example operations.go
```

**Use Case**: Managing workflow lifecycle in production: monitoring, cancellation, graceful shutdown.

---

### 11. Pre-built Patterns (`patterns-demo.go`)

**Purpose**: Demonstrates all 16 pre-built pattern functions.

**Features**:
- CI/CD patterns: BuildTestDeploy, with health check, with notification, multi-environment
- Parallel patterns: FanOutFanIn, ParallelDataProcessing, ParallelTestSuite, ParallelDeployment, MapReduce
- Loop patterns: ParallelLoop, SequentialLoop, BatchProcessing, MultiRegionDeployment, MatrixBuild, ParameterSweep, ParallelLoopWithTemplate

**Examples Included**:
1. **CI/CD Patterns**: All 4 CI/CD pattern functions
2. **Parallel Patterns**: All 5 parallel pattern functions
3. **Loop Patterns**: All 7 loop pattern functions

**Run**:
```bash
go run -tags example patterns-demo.go
```

**Use Case**: Quick-start common workflow scenarios without manual builder configuration.

---

### 12. Advanced Builder/Template APIs (`builder-advanced.go`)

**Purpose**: Demonstrates advanced builder, template, and source APIs.

**Features**:
- `BuildSingle()` for single container execution
- `Build()` auto-select (pipeline or parallel)
- `Cleanup()` for cleanup between steps
- Constructor options: `WithStopOnError`, `WithParallelMode`, `WithMaxConcurrency`, `WithGlobalAutoRemove`
- `ContainerSource` / `NewContainerSource` — wrap payload as WorkflowSource
- `AddInput()` for raw ContainerExecutionInput
- `ForEachParam` + `NewParameterizedLoopBuilder` with `BuildParameterizedLoop`
- `NewGoScript` — Go script template
- `NewHTTPWebhook` — webhook notification template
- Container options: `WithVolume`, `WithPorts`, `WithLabel`, `WithWaitForLog`, `WithWaitForPort`
- Script options: `WithScriptVolume`, `WithScriptPorts`

**Examples Included**:
1. **BuildSingle**: Single container via builder with constructor options
2. **Auto-Select Builder**: Build() with ContainerSource and AddInput
3. **Cleanup Pipeline**: Pipeline with Cleanup + rich container options
4. **Parameterized Loop Builder**: ForEachParam and NewParameterizedLoopBuilder
5. **Additional Templates**: NewGoScript, NewHTTPWebhook, advanced container/script options

**Run**:
```bash
go run -tags example builder-advanced.go
```

**Use Case**: Programmatic workflow construction with full API coverage.
```

**Step 2: Update self-contained examples list**

In the "Method 2: Self-Contained Examples" section (~line 63-72), add the new files:

```bash
go run -tags example operations.go
go run -tags example patterns-demo.go
go run -tags example builder-advanced.go
```

**Step 3: Commit**

```bash
git add examples/docker/README.md
git commit -m "docs(examples): update README with operations, patterns, and builder-advanced examples"
```

---

### Task 7: Final Verification

**Step 1: Verify all new files compile individually**

Run each:
```bash
go build -tags example examples/docker/operations.go
go build -tags example examples/docker/patterns-demo.go
go build -tags example examples/docker/builder-advanced.go
go build -tags example examples/docker/advanced.go
go build -tags example examples/docker/artifacts.go
```
Expected: All pass with no errors

**Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

**Step 3: Run existing tests**

Run: `task test:unit`
Expected: All tests pass (no regression)

**Step 4: Final commit (squash if needed)**

```bash
git log --oneline -7
```

Verify 5 commits (one per task 1-5, plus README update). If all is clean, the branch is ready for PR.
