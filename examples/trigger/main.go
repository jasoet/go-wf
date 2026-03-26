//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	pkgtemporal "github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"

	containerpayload "github.com/jasoet/go-wf/container/payload"
	containerwf "github.com/jasoet/go-wf/container/workflow"
	fnpayload "github.com/jasoet/go-wf/function/payload"
	fnwf "github.com/jasoet/go-wf/function/workflow"
	"github.com/jasoet/go-wf/workflow/artifacts"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: trigger <run|run-all|schedule|clean>")
		os.Exit(1)
	}

	config := pkgtemporal.DefaultConfig()
	if hostPort := os.Getenv("TEMPORAL_HOST_PORT"); hostPort != "" {
		config.HostPort = hostPort
	}
	c, closer, err := pkgtemporal.NewClient(config)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	ctx := context.Background()

	var cmdErr error
	switch os.Args[1] {
	case "run":
		cmdErr = runAll(ctx, c)
	case "run-all":
		cmdErr = runAll(ctx, c)
		if cmdErr == nil {
			createSchedules(ctx, c)
		}
	case "schedule":
		createSchedules(ctx, c)
	case "clean":
		cleanSchedules(ctx, c)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Usage: trigger <run|run-all|schedule|clean>")
		os.Exit(1)
	}

	if cmdErr != nil {
		os.Exit(1)
	}
}

func submit(ctx context.Context, c client.Client, workflowID, taskQueue string, workflowFunc interface{}, input interface{}) error {
	we, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}, workflowFunc, input)
	if err != nil {
		log.Printf("  FAILED %s: %v", workflowID, err)
		return err
	}
	log.Printf("  Submitted %s (RunID: %s)", we.GetID(), we.GetRunID())
	return nil
}

func runAll(ctx context.Context, c client.Client) error {
	ts := time.Now().Format("20060102-150405")
	containerQueue := "container-tasks"
	fnQueue := "function-tasks"
	var failures int

	log.Println("=== Submitting Docker Workflows ===")

	track := func(err error) {
		if err != nil {
			failures++
		}
	}

	// 1. Basic container
	track(submit(ctx, c, fmt.Sprintf("demo-container-basic-%s", ts), containerQueue,
		containerwf.ExecuteContainerWorkflow,
		containerpayload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"echo", "Hello from basic container"},
			AutoRemove: true,
			Name:       "demo-basic",
		}))

	// 2. Pipeline
	track(submit(ctx, c, fmt.Sprintf("demo-container-pipeline-%s", ts), containerQueue,
		containerwf.ContainerPipelineWorkflow,
		containerpayload.PipelineInput{
			StopOnError: true,
			Containers: []containerpayload.ContainerExecutionInput{
				{Image: "alpine:latest", Command: []string{"echo", "Step 1: Building..."}, AutoRemove: true, Name: "build"},
				{Image: "alpine:latest", Command: []string{"echo", "Step 2: Testing..."}, AutoRemove: true, Name: "test"},
				{Image: "alpine:latest", Command: []string{"echo", "Step 3: Deploying..."}, AutoRemove: true, Name: "deploy"},
			},
		}))

	// 3. Parallel
	track(submit(ctx, c, fmt.Sprintf("demo-container-parallel-%s", ts), containerQueue,
		containerwf.ParallelContainersWorkflow,
		containerpayload.ParallelInput{
			Containers: []containerpayload.ContainerExecutionInput{
				{Image: "alpine:latest", Command: []string{"echo", "Parallel task A"}, AutoRemove: true, Name: "task-a"},
				{Image: "alpine:latest", Command: []string{"echo", "Parallel task B"}, AutoRemove: true, Name: "task-b"},
				{Image: "alpine:latest", Command: []string{"echo", "Parallel task C"}, AutoRemove: true, Name: "task-c"},
			},
		}))

	// 4. Loop
	track(submit(ctx, c, fmt.Sprintf("demo-container-loop-%s", ts), containerQueue,
		containerwf.LoopWorkflow,
		containerpayload.LoopInput{
			Items: []string{"item-1", "item-2", "item-3"},
			Template: containerpayload.ContainerExecutionInput{
				Image:      "alpine:latest",
				Command:    []string{"echo", "Processing loop item"},
				AutoRemove: true,
			},
		}))

	// 5. Parameterized Loop
	track(submit(ctx, c, fmt.Sprintf("demo-container-paramloop-%s", ts), containerQueue,
		containerwf.ParameterizedLoopWorkflow,
		containerpayload.ParameterizedLoopInput{
			Parameters: map[string][]string{
				"env":    {"dev", "staging"},
				"region": {"us-east-1", "eu-west-1"},
			},
			Template: containerpayload.ContainerExecutionInput{
				Image:      "alpine:latest",
				Command:    []string{"echo", "Deploying parameterized"},
				AutoRemove: true,
			},
		}))

	// 6. DAG
	track(submit(ctx, c, fmt.Sprintf("demo-container-dag-%s", ts), containerQueue,
		containerwf.DAGWorkflow,
		containerpayload.DAGWorkflowInput{
			Nodes: []containerpayload.DAGNode{
				{Name: "build", Container: containerpayload.ExtendedContainerInput{
					ContainerExecutionInput: containerpayload.ContainerExecutionInput{
						Image: "alpine:latest", Command: []string{"echo", "Building..."}, AutoRemove: true, Name: "dag-build",
					},
				}},
				{Name: "test", Container: containerpayload.ExtendedContainerInput{
					ContainerExecutionInput: containerpayload.ContainerExecutionInput{
						Image: "alpine:latest", Command: []string{"echo", "Testing..."}, AutoRemove: true, Name: "dag-test",
					},
				}, Dependencies: []string{"build"}},
				{Name: "deploy", Container: containerpayload.ExtendedContainerInput{
					ContainerExecutionInput: containerpayload.ContainerExecutionInput{
						Image: "alpine:latest", Command: []string{"echo", "Deploying..."}, AutoRemove: true, Name: "dag-deploy",
					},
				}, Dependencies: []string{"test"}},
			},
			FailFast: true,
		}))

	log.Println()
	log.Println("=== Submitting Function Workflows ===")

	// 1. Basic function
	track(submit(ctx, c, fmt.Sprintf("demo-fn-basic-%s", ts), fnQueue,
		fnwf.ExecuteFunctionWorkflow,
		fnpayload.FunctionExecutionInput{
			Name: "greet",
			Args: map[string]string{"name": "Temporal"},
		}))

	// 2. Pipeline
	track(submit(ctx, c, fmt.Sprintf("demo-fn-pipeline-%s", ts), fnQueue,
		fnwf.FunctionPipelineWorkflow,
		fnpayload.PipelineInput{
			StopOnError: true,
			Functions: []fnpayload.FunctionExecutionInput{
				{Name: "validate", Args: map[string]string{"email": "user@example.com", "name": "Demo"}},
				{Name: "transform", Args: map[string]string{"name": "Demo", "email": "user@example.com"}},
				{Name: "notify", Args: map[string]string{"name": "Demo", "channel": "slack"}},
			},
		}))

	// 3. Parallel
	track(submit(ctx, c, fmt.Sprintf("demo-fn-parallel-%s", ts), fnQueue,
		fnwf.ParallelFunctionsWorkflow,
		fnpayload.ParallelInput{
			Functions: []fnpayload.FunctionExecutionInput{
				{Name: "fetch-users"},
				{Name: "fetch-orders"},
				{Name: "fetch-inventory"},
			},
		}))

	// 4. Loop
	track(submit(ctx, c, fmt.Sprintf("demo-fn-loop-%s", ts), fnQueue,
		fnwf.LoopWorkflow,
		fnpayload.LoopInput{
			Items: []string{"data-2024-01.csv", "data-2024-02.csv", "data-2024-03.csv"},
			Template: fnpayload.FunctionExecutionInput{
				Name: "process-csv",
				Args: map[string]string{"format": "standard"},
			},
		}))

	// 5. Parameterized Loop
	track(submit(ctx, c, fmt.Sprintf("demo-fn-paramloop-%s", ts), fnQueue,
		fnwf.ParameterizedLoopWorkflow,
		fnpayload.ParameterizedLoopInput{
			Parameters: map[string][]string{
				"environment": {"dev", "staging"},
				"region":      {"us-east-1", "eu-west-1"},
			},
			Template: fnpayload.FunctionExecutionInput{
				Name: "deploy-service",
				Args: map[string]string{"version": "v1.2.3"},
			},
		}))

	// 6. DAG — ETL with validation
	track(submit(ctx, c, fmt.Sprintf("demo-fn-dag-etl-%s", ts), fnQueue,
		fnwf.InstrumentedDAGWorkflow,
		fnpayload.DAGWorkflowInput{
			Nodes: []fnpayload.FunctionDAGNode{
				{Name: "validate-config", Function: fnpayload.FunctionExecutionInput{
					Name: "validate-config", Args: map[string]string{"env": "production"},
				}},
				{Name: "extract", Function: fnpayload.FunctionExecutionInput{
					Name: "extract", Args: map[string]string{"source": "database"},
				}},
				{Name: "transform", Function: fnpayload.FunctionExecutionInput{
					Name: "etl-transform", Args: map[string]string{"format": "parquet"},
				}, Dependencies: []string{"validate-config", "extract"}},
				{Name: "load", Function: fnpayload.FunctionExecutionInput{
					Name: "load", Args: map[string]string{"target": "warehouse"},
				}, Dependencies: []string{"transform"}},
			},
			FailFast: true,
		}))

	// 7. DAG — CI Pipeline
	track(submit(ctx, c, fmt.Sprintf("demo-fn-dag-ci-%s", ts), fnQueue,
		fnwf.InstrumentedDAGWorkflow,
		fnpayload.DAGWorkflowInput{
			Nodes: []fnpayload.FunctionDAGNode{
				{
					Name:     "compile",
					Function: fnpayload.FunctionExecutionInput{Name: "compile"},
					Outputs:  []fnpayload.OutputMapping{{Name: "artifact", ResultKey: "artifact"}},
				},
				{Name: "unit-test", Function: fnpayload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"compile"}},
				{Name: "lint", Function: fnpayload.FunctionExecutionInput{Name: "validate-config", Args: map[string]string{"env": "ci"}}, Dependencies: []string{"compile"}},
				{
					Name:         "publish",
					Function:     fnpayload.FunctionExecutionInput{Name: "publish-artifact", Args: map[string]string{}},
					Dependencies: []string{"unit-test", "lint"},
					Inputs:       []fnpayload.FunctionInputMapping{{Name: "artifact_path", From: "compile.artifact"}},
				},
			},
			FailFast: true,
		}))

	// 8. DAG — Artifact demo (LocalFile)
	track(submit(ctx, c, fmt.Sprintf("demo-fn-dag-artifact-local-%s", ts), fnQueue,
		"ArtifactDAGWorkflow-Local",
		fnpayload.DAGWorkflowInput{
			Nodes: []fnpayload.FunctionDAGNode{
				{
					Name:            "generate",
					Function:        fnpayload.FunctionExecutionInput{Name: "generate-report", Args: map[string]string{"type": "sales"}},
					OutputArtifacts: []artifacts.ArtifactRef{{Name: "report-data", Type: "bytes"}},
				},
				{
					Name:            "process",
					Function:        fnpayload.FunctionExecutionInput{Name: "process-report"},
					Dependencies:    []string{"generate"},
					InputArtifacts:  []artifacts.ArtifactRef{{Name: "report-data", Type: "bytes"}},
					OutputArtifacts: []artifacts.ArtifactRef{{Name: "processed-data", Type: "bytes"}},
				},
				{
					Name:           "archive",
					Function:       fnpayload.FunctionExecutionInput{Name: "archive-report"},
					Dependencies:   []string{"process"},
					InputArtifacts: []artifacts.ArtifactRef{{Name: "processed-data", Type: "bytes"}},
				},
			},
			FailFast: true,
		}))

	// 9. DAG — Artifact demo (MinIO)
	track(submit(ctx, c, fmt.Sprintf("demo-fn-dag-artifact-minio-%s", ts), fnQueue,
		"ArtifactDAGWorkflow-MinIO",
		fnpayload.DAGWorkflowInput{
			Nodes: []fnpayload.FunctionDAGNode{
				{
					Name:            "generate",
					Function:        fnpayload.FunctionExecutionInput{Name: "generate-report", Args: map[string]string{"type": "inventory"}},
					OutputArtifacts: []artifacts.ArtifactRef{{Name: "report-data", Type: "bytes"}},
				},
				{
					Name:            "process",
					Function:        fnpayload.FunctionExecutionInput{Name: "process-report"},
					Dependencies:    []string{"generate"},
					InputArtifacts:  []artifacts.ArtifactRef{{Name: "report-data", Type: "bytes"}},
					OutputArtifacts: []artifacts.ArtifactRef{{Name: "processed-data", Type: "bytes"}},
				},
				{
					Name:           "archive",
					Function:       fnpayload.FunctionExecutionInput{Name: "archive-report"},
					Dependencies:   []string{"process"},
					InputArtifacts: []artifacts.ArtifactRef{{Name: "processed-data", Type: "bytes"}},
				},
			},
			FailFast: true,
		}))

	log.Println()
	if failures > 0 {
		log.Printf("%d workflow(s) failed to submit", failures)
		return fmt.Errorf("%d workflow(s) failed to submit", failures)
	}
	log.Println("All workflows submitted. Visit http://localhost:8233 to inspect them.")
	return nil
}

// scheduleDefinition holds data for creating a single schedule.
type scheduleDefinition struct {
	ID           string
	Interval     time.Duration
	WorkflowID   string
	WorkflowFunc interface{}
	Input        interface{}
	TaskQueue    string
}

func createSchedules(ctx context.Context, c client.Client) {
	log.Println("=== Creating Schedules ===")

	schedules := []scheduleDefinition{
		{
			ID:           "schedule-container-pipeline",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-container-pipeline",
			WorkflowFunc: containerwf.ContainerPipelineWorkflow,
			TaskQueue:    "container-tasks",
			Input: containerpayload.PipelineInput{
				StopOnError: true,
				Containers: []containerpayload.ContainerExecutionInput{
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled build"}, AutoRemove: true, Name: "build"},
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled test"}, AutoRemove: true, Name: "test"},
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled deploy"}, AutoRemove: true, Name: "deploy"},
				},
			},
		},
		{
			ID:           "schedule-container-parallel",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-container-parallel",
			WorkflowFunc: containerwf.ParallelContainersWorkflow,
			TaskQueue:    "container-tasks",
			Input: containerpayload.ParallelInput{
				Containers: []containerpayload.ContainerExecutionInput{
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled parallel A"}, AutoRemove: true, Name: "par-a"},
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled parallel B"}, AutoRemove: true, Name: "par-b"},
				},
			},
		},
		{
			ID:           "schedule-fn-pipeline",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-pipeline",
			WorkflowFunc: fnwf.FunctionPipelineWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.PipelineInput{
				StopOnError: true,
				Functions: []fnpayload.FunctionExecutionInput{
					{Name: "validate", Args: map[string]string{"email": "cron@example.com", "name": "CronUser"}},
					{Name: "transform", Args: map[string]string{"name": "CronUser", "email": "cron@example.com"}},
					{Name: "notify", Args: map[string]string{"name": "CronUser", "channel": "email"}},
				},
			},
		},
		{
			ID:           "schedule-fn-dag-ci",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-dag-ci",
			WorkflowFunc: fnwf.InstrumentedDAGWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.DAGWorkflowInput{
				Nodes: []fnpayload.FunctionDAGNode{
					{
						Name:     "compile",
						Function: fnpayload.FunctionExecutionInput{Name: "compile"},
						Outputs:  []fnpayload.OutputMapping{{Name: "artifact", ResultKey: "artifact"}},
					},
					{Name: "unit-test", Function: fnpayload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"compile"}},
					{Name: "lint", Function: fnpayload.FunctionExecutionInput{Name: "validate-config", Args: map[string]string{"env": "ci"}}, Dependencies: []string{"compile"}},
					{
						Name:         "publish",
						Function:     fnpayload.FunctionExecutionInput{Name: "publish-artifact", Args: map[string]string{}},
						Dependencies: []string{"unit-test", "lint"},
						Inputs:       []fnpayload.FunctionInputMapping{{Name: "artifact_path", From: "compile.artifact"}},
					},
				},
				FailFast: true,
			},
		},
		{
			ID:           "schedule-fn-loop",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-loop",
			WorkflowFunc: fnwf.LoopWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.LoopInput{
				Items: []string{"report-daily.csv", "report-weekly.csv", "report-monthly.csv"},
				Template: fnpayload.FunctionExecutionInput{
					Name: "process-csv",
					Args: map[string]string{"format": "standard"},
				},
			},
		},
		{
			ID:           "schedule-container-loop",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-container-loop",
			WorkflowFunc: containerwf.LoopWorkflow,
			TaskQueue:    "container-tasks",
			Input: containerpayload.LoopInput{
				Items: []string{"item-1", "item-2", "item-3"},
				Template: containerpayload.ContainerExecutionInput{
					Image:      "alpine:latest",
					Command:    []string{"echo", "Scheduled loop item"},
					AutoRemove: true,
				},
			},
		},
		{
			ID:           "schedule-container-dag",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-container-dag",
			WorkflowFunc: containerwf.DAGWorkflow,
			TaskQueue:    "container-tasks",
			Input: containerpayload.DAGWorkflowInput{
				Nodes: []containerpayload.DAGNode{
					{Name: "build", Container: containerpayload.ExtendedContainerInput{
						ContainerExecutionInput: containerpayload.ContainerExecutionInput{
							Image: "alpine:latest", Command: []string{"echo", "Building..."}, AutoRemove: true, Name: "dag-build",
						},
					}},
					{Name: "test", Container: containerpayload.ExtendedContainerInput{
						ContainerExecutionInput: containerpayload.ContainerExecutionInput{
							Image: "alpine:latest", Command: []string{"echo", "Testing..."}, AutoRemove: true, Name: "dag-test",
						},
					}, Dependencies: []string{"build"}},
					{Name: "deploy", Container: containerpayload.ExtendedContainerInput{
						ContainerExecutionInput: containerpayload.ContainerExecutionInput{
							Image: "alpine:latest", Command: []string{"echo", "Deploying..."}, AutoRemove: true, Name: "dag-deploy",
						},
					}, Dependencies: []string{"test"}},
				},
				FailFast: true,
			},
		},
		{
			ID:           "schedule-fn-parallel",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-parallel",
			WorkflowFunc: fnwf.ParallelFunctionsWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.ParallelInput{
				Functions: []fnpayload.FunctionExecutionInput{
					{Name: "fetch-users"},
					{Name: "fetch-orders"},
					{Name: "fetch-inventory"},
				},
			},
		},
		{
			ID:           "schedule-fn-paramloop",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-paramloop",
			WorkflowFunc: fnwf.ParameterizedLoopWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.ParameterizedLoopInput{
				Parameters: map[string][]string{
					"environment": {"dev", "staging"},
					"region":      {"us-east-1", "eu-west-1"},
				},
				Template: fnpayload.FunctionExecutionInput{
					Name: "deploy-service",
					Args: map[string]string{"version": "v1.2.3"},
				},
			},
		},
		{
			ID:           "schedule-fn-dag-etl",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-dag-etl",
			WorkflowFunc: fnwf.InstrumentedDAGWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.DAGWorkflowInput{
				Nodes: []fnpayload.FunctionDAGNode{
					{Name: "validate-config", Function: fnpayload.FunctionExecutionInput{
						Name: "validate-config", Args: map[string]string{"env": "production"},
					}},
					{Name: "extract", Function: fnpayload.FunctionExecutionInput{
						Name: "extract", Args: map[string]string{"source": "database"},
					}},
					{Name: "transform", Function: fnpayload.FunctionExecutionInput{
						Name: "etl-transform", Args: map[string]string{"format": "parquet"},
					}, Dependencies: []string{"validate-config", "extract"}},
					{Name: "load", Function: fnpayload.FunctionExecutionInput{
						Name: "load", Args: map[string]string{"target": "warehouse"},
					}, Dependencies: []string{"transform"}},
				},
				FailFast: true,
			},
		},
		{
			ID:           "schedule-fn-dag-artifact",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-fn-dag-artifact",
			WorkflowFunc: "ArtifactDAGWorkflow-MinIO",
			TaskQueue:    "function-tasks",
			Input: fnpayload.DAGWorkflowInput{
				Nodes: []fnpayload.FunctionDAGNode{
					{
						Name:            "generate",
						Function:        fnpayload.FunctionExecutionInput{Name: "generate-report", Args: map[string]string{"type": "scheduled"}},
						OutputArtifacts: []artifacts.ArtifactRef{{Name: "report-data", Type: "bytes"}},
					},
					{
						Name:            "process",
						Function:        fnpayload.FunctionExecutionInput{Name: "process-report"},
						Dependencies:    []string{"generate"},
						InputArtifacts:  []artifacts.ArtifactRef{{Name: "report-data", Type: "bytes"}},
						OutputArtifacts: []artifacts.ArtifactRef{{Name: "processed-data", Type: "bytes"}},
					},
					{
						Name:           "archive",
						Function:       fnpayload.FunctionExecutionInput{Name: "archive-report"},
						Dependencies:   []string{"process"},
						InputArtifacts: []artifacts.ArtifactRef{{Name: "processed-data", Type: "bytes"}},
					},
				},
				FailFast: true,
			},
		},
	}

	for _, s := range schedules {
		_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
			ID: s.ID,
			Spec: client.ScheduleSpec{
				Intervals: []client.ScheduleIntervalSpec{
					{Every: s.Interval},
				},
			},
			Action: &client.ScheduleWorkflowAction{
				ID:        s.WorkflowID,
				Workflow:  s.WorkflowFunc,
				Args:      []interface{}{s.Input},
				TaskQueue: s.TaskQueue,
			},
		})
		if err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "already") {
				log.Printf("  Schedule %s already exists, skipping", s.ID)
			} else {
				log.Printf("  FAILED to create schedule %s: %v", s.ID, err)
			}
			continue
		}
		log.Printf("  Created schedule %s (every %s)", s.ID, s.Interval)
	}

	log.Println()
	log.Println("Schedules created. Visit http://localhost:8233/schedules to view them.")
}

func cleanSchedules(ctx context.Context, c client.Client) {
	log.Println("=== Cleaning Schedules ===")

	scheduleIDs := []string{
		"schedule-container-pipeline",
		"schedule-container-parallel",
		"schedule-container-loop",
		"schedule-container-dag",
		"schedule-fn-pipeline",
		"schedule-fn-dag-ci",
		"schedule-fn-loop",
		"schedule-fn-parallel",
		"schedule-fn-paramloop",
		"schedule-fn-dag-etl",
		"schedule-fn-dag-artifact",
	}

	for _, id := range scheduleIDs {
		handle := c.ScheduleClient().GetHandle(ctx, id)
		err := handle.Delete(ctx)
		if err != nil {
			errMsg := strings.ToLower(err.Error())
			if strings.Contains(errMsg, "not found") ||
				strings.Contains(errMsg, "not exist") ||
				strings.Contains(errMsg, "already completed") {
				log.Printf("  Schedule %s not found, skipping", id)
			} else {
				log.Printf("  FAILED to delete schedule %s: %v", id, err)
			}
			continue
		}
		log.Printf("  Deleted schedule %s", id)
	}

	log.Println()
	log.Println("Schedule cleanup complete.")
}
