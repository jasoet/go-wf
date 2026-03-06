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
	"github.com/jasoet/go-wf/docker/patterns"
	"github.com/jasoet/go-wf/docker/payload"
	"github.com/jasoet/go-wf/docker/template"
	"github.com/jasoet/go-wf/docker/workflow"
)

// This example demonstrates all 16 pre-built pattern functions from the
// docker/patterns package, organized into three groups:
// 1. CI/CD patterns (4 functions)
// 2. Parallel patterns (5 functions)
// 3. Loop patterns (7 functions)

func main() {
	c, closer, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	w := worker.New(c, "docker-tasks", worker.Options{})
	docker.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	fmt.Println("\n========== CI/CD Patterns ==========")
	runCICDPatterns(c)

	fmt.Println("\n========== Parallel Patterns ==========")
	runParallelPatterns(c)

	fmt.Println("\n========== Loop Patterns ==========")
	runLoopPatterns(c)

	fmt.Println("\nAll 16 pattern demos completed.")
}

// runCICDPatterns demonstrates the 4 CI/CD pattern functions.
func runCICDPatterns(c client.Client) {
	ctx := context.Background()

	// 1. BuildTestDeploy
	fmt.Println("\n--- Pattern: BuildTestDeploy ---")
	{
		input, err := patterns.BuildTestDeploy("alpine:latest", "alpine:latest", "alpine:latest")
		if err != nil {
			log.Fatalf("BuildTestDeploy failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-build-test-deploy",
			TaskQueue: "docker-tasks",
		}, workflow.ContainerPipelineWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("BuildTestDeploy completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 2. BuildTestDeployWithHealthCheck
	fmt.Println("\n--- Pattern: BuildTestDeployWithHealthCheck ---")
	{
		input, err := patterns.BuildTestDeployWithHealthCheck(
			"alpine:latest", "alpine:latest", "http://localhost:8080/health")
		if err != nil {
			log.Fatalf("BuildTestDeployWithHealthCheck failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-build-test-deploy-health",
			TaskQueue: "docker-tasks",
		}, workflow.ContainerPipelineWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("BuildTestDeployWithHealthCheck completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 3. BuildTestDeployWithNotification
	fmt.Println("\n--- Pattern: BuildTestDeployWithNotification ---")
	{
		input, err := patterns.BuildTestDeployWithNotification(
			"alpine:latest", "alpine:latest",
			"https://hooks.example.com/webhook",
			`{"text": "Deployment complete"}`)
		if err != nil {
			log.Fatalf("BuildTestDeployWithNotification failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-build-test-deploy-notify",
			TaskQueue: "docker-tasks",
		}, workflow.ContainerPipelineWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("BuildTestDeployWithNotification completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 4. MultiEnvironmentDeploy
	fmt.Println("\n--- Pattern: MultiEnvironmentDeploy ---")
	{
		input, err := patterns.MultiEnvironmentDeploy(
			"alpine:latest", []string{"staging", "production"})
		if err != nil {
			log.Fatalf("MultiEnvironmentDeploy failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-multi-env-deploy",
			TaskQueue: "docker-tasks",
		}, workflow.ContainerPipelineWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("MultiEnvironmentDeploy completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}
}

// runParallelPatterns demonstrates the 5 parallel pattern functions.
func runParallelPatterns(c client.Client) {
	ctx := context.Background()

	// 5. FanOutFanIn
	fmt.Println("\n--- Pattern: FanOutFanIn ---")
	{
		input, err := patterns.FanOutFanIn("alpine:latest", []string{"task-a", "task-b", "task-c"})
		if err != nil {
			log.Fatalf("FanOutFanIn failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-fan-out-fan-in",
			TaskQueue: "docker-tasks",
		}, workflow.ParallelContainersWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("FanOutFanIn completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 6. ParallelDataProcessing
	fmt.Println("\n--- Pattern: ParallelDataProcessing ---")
	{
		input, err := patterns.ParallelDataProcessing(
			"alpine:latest",
			[]string{"data-1.csv", "data-2.csv", "data-3.csv"},
			"echo processing")
		if err != nil {
			log.Fatalf("ParallelDataProcessing failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-parallel-data-processing",
			TaskQueue: "docker-tasks",
		}, workflow.ParallelContainersWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("ParallelDataProcessing completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 7. ParallelTestSuite
	fmt.Println("\n--- Pattern: ParallelTestSuite ---")
	{
		input, err := patterns.ParallelTestSuite("alpine:latest", map[string]string{
			"unit":        "echo 'running unit tests'",
			"integration": "echo 'running integration tests'",
		})
		if err != nil {
			log.Fatalf("ParallelTestSuite failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-parallel-test-suite",
			TaskQueue: "docker-tasks",
		}, workflow.ParallelContainersWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("ParallelTestSuite completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 8. ParallelDeployment
	fmt.Println("\n--- Pattern: ParallelDeployment ---")
	{
		input, err := patterns.ParallelDeployment(
			"alpine:latest", []string{"us-west", "us-east", "eu-central"})
		if err != nil {
			log.Fatalf("ParallelDeployment failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-parallel-deployment",
			TaskQueue: "docker-tasks",
		}, workflow.ParallelContainersWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.ParallelOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("ParallelDeployment completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 9. MapReduce
	fmt.Println("\n--- Pattern: MapReduce ---")
	{
		input, err := patterns.MapReduce(
			"alpine:latest",
			[]string{"file1.txt", "file2.txt"},
			"wc -w",
			"awk '{sum+=$1} END {print sum}'")
		if err != nil {
			log.Fatalf("MapReduce failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-map-reduce",
			TaskQueue: "docker-tasks",
		}, workflow.ContainerPipelineWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.PipelineOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("MapReduce completed: Success=%d, Failed=%d, Duration=%s\n",
			result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}
}

// runLoopPatterns demonstrates the 7 loop pattern functions.
func runLoopPatterns(c client.Client) {
	ctx := context.Background()

	// 10. ParallelLoop
	fmt.Println("\n--- Pattern: ParallelLoop ---")
	{
		input, err := patterns.ParallelLoop(
			[]string{"item-1", "item-2", "item-3"},
			"alpine:latest",
			"echo processing {{item}}")
		if err != nil {
			log.Fatalf("ParallelLoop failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-parallel-loop",
			TaskQueue: "docker-tasks",
		}, workflow.LoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("ParallelLoop completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 11. SequentialLoop
	fmt.Println("\n--- Pattern: SequentialLoop ---")
	{
		input, err := patterns.SequentialLoop(
			[]string{"step-1", "step-2", "step-3"},
			"alpine:latest",
			"echo executing {{item}}")
		if err != nil {
			log.Fatalf("SequentialLoop failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-sequential-loop",
			TaskQueue: "docker-tasks",
		}, workflow.LoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("SequentialLoop completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 12. BatchProcessing
	fmt.Println("\n--- Pattern: BatchProcessing ---")
	{
		input, err := patterns.BatchProcessing(
			[]string{"batch1.json", "batch2.json", "batch3.json", "batch4.json"},
			"alpine:latest",
			2)
		if err != nil {
			log.Fatalf("BatchProcessing failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-batch-processing",
			TaskQueue: "docker-tasks",
		}, workflow.LoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("BatchProcessing completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 13. MultiRegionDeployment
	fmt.Println("\n--- Pattern: MultiRegionDeployment ---")
	{
		input, err := patterns.MultiRegionDeployment(
			[]string{"dev", "staging"},
			[]string{"us-west", "eu-central"},
			"alpine:latest")
		if err != nil {
			log.Fatalf("MultiRegionDeployment failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-multi-region-deployment",
			TaskQueue: "docker-tasks",
		}, workflow.ParameterizedLoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("MultiRegionDeployment completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 14. MatrixBuild
	fmt.Println("\n--- Pattern: MatrixBuild ---")
	{
		input, err := patterns.MatrixBuild(
			map[string][]string{
				"go_version": {"1.22", "1.23"},
				"platform":   {"linux", "darwin"},
			},
			"alpine:latest")
		if err != nil {
			log.Fatalf("MatrixBuild failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-matrix-build",
			TaskQueue: "docker-tasks",
		}, workflow.ParameterizedLoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("MatrixBuild completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 15. ParameterSweep
	fmt.Println("\n--- Pattern: ParameterSweep ---")
	{
		input, err := patterns.ParameterSweep(
			map[string][]string{
				"learning_rate": {"0.001", "0.01"},
				"batch_size":    {"32", "64"},
			},
			"alpine:latest",
			2)
		if err != nil {
			log.Fatalf("ParameterSweep failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-parameter-sweep",
			TaskQueue: "docker-tasks",
		}, workflow.ParameterizedLoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("ParameterSweep completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}

	// 16. ParallelLoopWithTemplate
	fmt.Println("\n--- Pattern: ParallelLoopWithTemplate ---")
	{
		bashTemplate := template.NewBashScript("process-item",
			"echo 'Processing item: {{item}}' && sleep 1")

		input, err := patterns.ParallelLoopWithTemplate(
			[]string{"alpha", "beta", "gamma"},
			bashTemplate)
		if err != nil {
			log.Fatalf("ParallelLoopWithTemplate failed: %v", err)
		}

		we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        "pattern-parallel-loop-template",
			TaskQueue: "docker-tasks",
		}, workflow.LoopWorkflow, *input)
		if err != nil {
			log.Fatalf("Failed to start workflow: %v", err)
		}

		var result payload.LoopOutput
		if err := we.Get(ctx, &result); err != nil {
			log.Fatalf("Workflow failed: %v", err)
		}
		fmt.Printf("ParallelLoopWithTemplate completed: Items=%d, Success=%d, Failed=%d, Duration=%s\n",
			result.ItemCount, result.TotalSuccess, result.TotalFailed, result.TotalDuration)
	}
}
