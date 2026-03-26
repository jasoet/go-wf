//go:build example

package main

import (
	"context"
	"log"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/container"
	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/container/workflow"
)

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
	w := worker.New(c, "container-tasks", worker.Options{})
	container.RegisterAll(w)

	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Fatalf("Worker failed: %v", err)
		}
	}()
	defer w.Stop()

	time.Sleep(time.Second)

	log.Println("=== Advanced Features Demo ===\n")

	// Example 1: Workflow Parameters (Template Variables)
	log.Println("Example 1: Workflow Parameters")
	runParameterizedWorkflow(c)

	time.Sleep(2 * time.Second)

	// Example 2: Resource Limits
	log.Println("\nExample 2: Resource Limits")
	runResourceLimitedWorkflow(c)

	time.Sleep(2 * time.Second)

	// Example 3: Conditional Execution & ContinueOnFail
	log.Println("\nExample 3: Conditional Execution")
	runConditionalWorkflow(c)

	time.Sleep(2 * time.Second)

	// Example 4: Wait Strategies
	log.Println("\nExample 4: Wait Strategies")
	runWaitStrategiesDemo(c)

	time.Sleep(2 * time.Second)

	// Example 5: Retry, Secrets, DependsOn, File Outputs
	log.Println("\nExample 5: Retry & Secrets")
	runRetryAndSecretsDemo(c)
}

// Example 1: Workflow Parameters (Similar to Argo parameters)
func runParameterizedWorkflow(c client.Client) {
	// Define workflow with template variables
	input := payload.ContainerExecutionInput{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo 'Deploying version {{.version}} to {{.environment}}' && echo 'Repository: {{.repo}}'"},
		Env: map[string]string{
			"APP_VERSION": "{{.version}}",
			"ENVIRONMENT": "{{.environment}}",
			"REPO_URL":    "{{.repo}}",
			"DEPLOY_TIME": "{{.timestamp}}",
			"DEPLOYED_BY": "{{.user}}",
		},
		AutoRemove: true,
	}

	// Define parameters (like Argo's parameters)
	params := []payload.WorkflowParameter{
		{Name: "version", Value: "v1.2.3"},
		{Name: "environment", Value: "production"},
		{Name: "repo", Value: "https://github.com/myorg/myapp"},
		{Name: "timestamp", Value: time.Now().Format(time.RFC3339)},
		{Name: "user", Value: "ops-team"},
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "parameterized-workflow",
			TaskQueue: "container-tasks",
		},
		workflow.WorkflowWithParameters,
		input,
		params,
	)

	var result payload.ContainerExecutionOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Parameterized workflow failed: %v", err)
		return
	}

	log.Printf("Parameterized workflow completed successfully")
	log.Printf("Container output:\n%s", result.Stdout)
}

// Example 2: Resource Limits (CPU, Memory, GPU)
func runResourceLimitedWorkflow(c client.Client) {
	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "small-task",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Running small task' && sleep 1"},
						AutoRemove: true,
					},
					Resources: &payload.ResourceLimits{
						CPURequest:    "100m",
						CPULimit:      "200m",
						MemoryRequest: "64Mi",
						MemoryLimit:   "128Mi",
					},
				},
			},
			{
				Name: "large-task",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Running large task' && sleep 2"},
						AutoRemove: true,
					},
					Resources: &payload.ResourceLimits{
						CPURequest:    "1000m",
						CPULimit:      "2000m",
						MemoryRequest: "1Gi",
						MemoryLimit:   "2Gi",
					},
				},
				Dependencies: []string{"small-task"},
			},
			{
				Name: "ml-task",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "tensorflow/tensorflow:latest",
						Command:    []string{"sh", "-c", "echo 'Running ML training' && sleep 2"},
						AutoRemove: true,
					},
					Resources: &payload.ResourceLimits{
						CPURequest:    "2000m",
						CPULimit:      "4000m",
						MemoryRequest: "4Gi",
						MemoryLimit:   "8Gi",
						GPUCount:      1, // Request 1 GPU
					},
				},
				Dependencies: []string{"large-task"},
			},
		},
		FailFast: true,
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "resource-limited-workflow",
			TaskQueue: "container-tasks",
		},
		workflow.DAGWorkflow,
		input,
	)

	var result payload.DAGWorkflowOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Resource-limited workflow failed: %v", err)
		return
	}

	log.Printf("Resource-limited workflow completed: Success=%d", result.TotalSuccess)
}

// Example 3: Conditional Execution & ContinueOnFail
func runConditionalWorkflow(c client.Client) {
	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "test",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Running tests' && exit 0"},
						AutoRemove: true,
					},
				},
			},
			{
				Name: "deploy-staging",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Deploying to staging' && sleep 1"},
						Env:        map[string]string{"ENVIRONMENT": "staging"},
						AutoRemove: true,
					},
					// Conditional: only deploy if tests passed
					Conditional: &payload.ConditionalBehavior{
						When:           "{{steps.test.exitCode}} == 0",
						ContinueOnFail: false,
					},
				},
				Dependencies: []string{"test"},
			},
			{
				Name: "deploy-production",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Deploying to production' && sleep 1"},
						Env:        map[string]string{"ENVIRONMENT": "production"},
						AutoRemove: true,
					},
					// Conditional: only deploy to prod if staging succeeded
					Conditional: &payload.ConditionalBehavior{
						When:           "{{steps.deploy-staging.exitCode}} == 0",
						ContinueOnFail: false,
					},
				},
				Dependencies: []string{"deploy-staging"},
			},
			{
				Name: "rollback",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:      "alpine:latest",
						Command:    []string{"sh", "-c", "echo 'Rolling back deployment'"},
						AutoRemove: true,
					},
					// Conditional: only rollback if production deploy failed
					Conditional: &payload.ConditionalBehavior{
						When:            "{{steps.deploy-production.exitCode}} != 0",
						ContinueOnFail:  true,
						ContinueOnError: true,
					},
				},
				Dependencies: []string{"deploy-production"},
			},
		},
		FailFast: false, // Don't stop - allow rollback to run
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "conditional-workflow",
			TaskQueue: "container-tasks",
		},
		workflow.DAGWorkflow,
		input,
	)

	var result payload.DAGWorkflowOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Conditional workflow failed: %v", err)
		return
	}

	log.Printf("Conditional workflow completed")
	for _, nodeResult := range result.NodeResults {
		log.Printf("  Node %s: Success=%v", nodeResult.NodeName, nodeResult.Success)
	}
}

// Example 4: Wait Strategies (Container Readiness)
func runWaitStrategiesDemo(c client.Client) {
	input := payload.PipelineInput{
		StopOnError: true,
		Containers: []payload.ContainerExecutionInput{
			{
				Name:  "log-wait",
				Image: "postgres:16-alpine",
				Env: map[string]string{
					"POSTGRES_PASSWORD": "test",
					"POSTGRES_DB":       "testdb",
				},
				WaitStrategy: payload.WaitStrategyConfig{
					Type:           "log",
					LogMessage:     "ready to accept connections",
					StartupTimeout: 30 * time.Second,
				},
				AutoRemove: true,
			},
			{
				Name:  "port-wait",
				Image: "redis:7-alpine",
				WaitStrategy: payload.WaitStrategyConfig{
					Type:           "port",
					Port:           "6379",
					StartupTimeout: 10 * time.Second,
				},
				AutoRemove: true,
			},
			{
				Name:  "healthy-wait",
				Image: "postgres:16-alpine",
				Env: map[string]string{
					"POSTGRES_PASSWORD": "test",
				},
				WaitStrategy: payload.WaitStrategyConfig{
					Type:           "healthy",
					StartupTimeout: 30 * time.Second,
				},
				AutoRemove: true,
			},
		},
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "wait-strategies-demo",
			TaskQueue: "container-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		input,
	)

	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Wait strategies demo failed: %v", err)
		return
	}

	log.Printf("Wait strategies demo completed: Success=%d", result.TotalSuccess)
}

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
			TaskQueue: "container-tasks",
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
