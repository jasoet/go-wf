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

	dockerpayload "github.com/jasoet/go-wf/docker/payload"
	dockerwf "github.com/jasoet/go-wf/docker/workflow"
	fnpayload "github.com/jasoet/go-wf/function/payload"
	fnwf "github.com/jasoet/go-wf/function/workflow"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: trigger <run|schedule|clean>")
		os.Exit(1)
	}

	c, closer, err := pkgtemporal.NewClient(pkgtemporal.DefaultConfig())
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
	case "schedule":
		createSchedules(ctx, c)
	case "clean":
		cleanSchedules(ctx, c)
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		fmt.Println("Usage: trigger <run|schedule|clean>")
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
	dockerQueue := "docker-tasks"
	fnQueue := "function-tasks"
	var failures int

	log.Println("=== Submitting Docker Workflows ===")

	track := func(err error) {
		if err != nil {
			failures++
		}
	}

	// 1. Basic container
	track(submit(ctx, c, fmt.Sprintf("demo-docker-basic-%s", ts), dockerQueue,
		dockerwf.ExecuteContainerWorkflow,
		dockerpayload.ContainerExecutionInput{
			Image:      "alpine:latest",
			Command:    []string{"echo", "Hello from basic container"},
			AutoRemove: true,
			Name:       "demo-basic",
		}))

	// 2. Pipeline
	track(submit(ctx, c, fmt.Sprintf("demo-docker-pipeline-%s", ts), dockerQueue,
		dockerwf.ContainerPipelineWorkflow,
		dockerpayload.PipelineInput{
			StopOnError: true,
			Containers: []dockerpayload.ContainerExecutionInput{
				{Image: "alpine:latest", Command: []string{"echo", "Step 1: Building..."}, AutoRemove: true, Name: "build"},
				{Image: "alpine:latest", Command: []string{"echo", "Step 2: Testing..."}, AutoRemove: true, Name: "test"},
				{Image: "alpine:latest", Command: []string{"echo", "Step 3: Deploying..."}, AutoRemove: true, Name: "deploy"},
			},
		}))

	// 3. Parallel
	track(submit(ctx, c, fmt.Sprintf("demo-docker-parallel-%s", ts), dockerQueue,
		dockerwf.ParallelContainersWorkflow,
		dockerpayload.ParallelInput{
			Containers: []dockerpayload.ContainerExecutionInput{
				{Image: "alpine:latest", Command: []string{"echo", "Parallel task A"}, AutoRemove: true, Name: "task-a"},
				{Image: "alpine:latest", Command: []string{"echo", "Parallel task B"}, AutoRemove: true, Name: "task-b"},
				{Image: "alpine:latest", Command: []string{"echo", "Parallel task C"}, AutoRemove: true, Name: "task-c"},
			},
		}))

	// 4. Loop
	track(submit(ctx, c, fmt.Sprintf("demo-docker-loop-%s", ts), dockerQueue,
		dockerwf.LoopWorkflow,
		dockerpayload.LoopInput{
			Items: []string{"item-1", "item-2", "item-3"},
			Template: dockerpayload.ContainerExecutionInput{
				Image:      "alpine:latest",
				Command:    []string{"echo", "Processing loop item"},
				AutoRemove: true,
			},
		}))

	// 5. Parameterized Loop
	track(submit(ctx, c, fmt.Sprintf("demo-docker-paramloop-%s", ts), dockerQueue,
		dockerwf.ParameterizedLoopWorkflow,
		dockerpayload.ParameterizedLoopInput{
			Parameters: map[string][]string{
				"env":    {"dev", "staging"},
				"region": {"us-east-1", "eu-west-1"},
			},
			Template: dockerpayload.ContainerExecutionInput{
				Image:      "alpine:latest",
				Command:    []string{"echo", "Deploying parameterized"},
				AutoRemove: true,
			},
		}))

	// 6. DAG
	track(submit(ctx, c, fmt.Sprintf("demo-docker-dag-%s", ts), dockerQueue,
		dockerwf.DAGWorkflow,
		dockerpayload.DAGWorkflowInput{
			Nodes: []dockerpayload.DAGNode{
				{Name: "build", Container: dockerpayload.ExtendedContainerInput{
					ContainerExecutionInput: dockerpayload.ContainerExecutionInput{
						Image: "alpine:latest", Command: []string{"echo", "Building..."}, AutoRemove: true, Name: "dag-build",
					},
				}},
				{Name: "test", Container: dockerpayload.ExtendedContainerInput{
					ContainerExecutionInput: dockerpayload.ContainerExecutionInput{
						Image: "alpine:latest", Command: []string{"echo", "Testing..."}, AutoRemove: true, Name: "dag-test",
					},
				}, Dependencies: []string{"build"}},
				{Name: "deploy", Container: dockerpayload.ExtendedContainerInput{
					ContainerExecutionInput: dockerpayload.ContainerExecutionInput{
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
			ID:           "schedule-docker-pipeline",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-docker-pipeline",
			WorkflowFunc: dockerwf.ContainerPipelineWorkflow,
			TaskQueue:    "docker-tasks",
			Input: dockerpayload.PipelineInput{
				StopOnError: true,
				Containers: []dockerpayload.ContainerExecutionInput{
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled build"}, AutoRemove: true, Name: "build"},
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled test"}, AutoRemove: true, Name: "test"},
					{Image: "alpine:latest", Command: []string{"echo", "Scheduled deploy"}, AutoRemove: true, Name: "deploy"},
				},
			},
		},
		{
			ID:           "schedule-docker-parallel",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-docker-parallel",
			WorkflowFunc: dockerwf.ParallelContainersWorkflow,
			TaskQueue:    "docker-tasks",
			Input: dockerpayload.ParallelInput{
				Containers: []dockerpayload.ContainerExecutionInput{
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
			ID:           "schedule-docker-loop",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-docker-loop",
			WorkflowFunc: dockerwf.LoopWorkflow,
			TaskQueue:    "docker-tasks",
			Input: dockerpayload.LoopInput{
				Items: []string{"item-1", "item-2", "item-3"},
				Template: dockerpayload.ContainerExecutionInput{
					Image:      "alpine:latest",
					Command:    []string{"echo", "Scheduled loop item"},
					AutoRemove: true,
				},
			},
		},
		{
			ID:           "schedule-docker-dag",
			Interval:     2 * time.Minute,
			WorkflowID:   "scheduled-docker-dag",
			WorkflowFunc: dockerwf.DAGWorkflow,
			TaskQueue:    "docker-tasks",
			Input: dockerpayload.DAGWorkflowInput{
				Nodes: []dockerpayload.DAGNode{
					{Name: "build", Container: dockerpayload.ExtendedContainerInput{
						ContainerExecutionInput: dockerpayload.ContainerExecutionInput{
							Image: "alpine:latest", Command: []string{"echo", "Building..."}, AutoRemove: true, Name: "dag-build",
						},
					}},
					{Name: "test", Container: dockerpayload.ExtendedContainerInput{
						ContainerExecutionInput: dockerpayload.ContainerExecutionInput{
							Image: "alpine:latest", Command: []string{"echo", "Testing..."}, AutoRemove: true, Name: "dag-test",
						},
					}, Dependencies: []string{"build"}},
					{Name: "deploy", Container: dockerpayload.ExtendedContainerInput{
						ContainerExecutionInput: dockerpayload.ContainerExecutionInput{
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
		"schedule-docker-pipeline",
		"schedule-docker-parallel",
		"schedule-docker-loop",
		"schedule-docker-dag",
		"schedule-fn-pipeline",
		"schedule-fn-dag-ci",
		"schedule-fn-loop",
		"schedule-fn-parallel",
		"schedule-fn-paramloop",
		"schedule-fn-dag-etl",
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
