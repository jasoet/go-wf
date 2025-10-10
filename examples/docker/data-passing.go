//go:build example

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jasoet/go-wf/docker"
	"go.temporal.io/sdk/client"
)

// This example demonstrates explicit data passing between workflow steps.
// It shows how to:
// 1. Capture outputs from container execution (stdout, stderr, exitCode, files)
// 2. Extract values using JSONPath and regex
// 3. Pass outputs from one step as inputs to dependent steps
// 4. Use the DAG workflow to orchestrate multi-step workflows with data flow

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

	// Example 1: Build -> Test -> Deploy pipeline with data passing
	fmt.Println("\n=== Example 1: Build -> Test -> Deploy Pipeline ===")
	buildTestDeployPipeline(ctx, c)

	// Example 2: Extract JSON outputs
	fmt.Println("\n=== Example 2: Extract JSON Outputs ===")
	jsonOutputExtraction(ctx, c)

	// Example 3: Extract using regex
	fmt.Println("\n=== Example 3: Extract Using Regex ===")
	regexExtraction(ctx, c)

	// Example 4: Multiple outputs and inputs
	fmt.Println("\n=== Example 4: Multiple Outputs and Inputs ===")
	multipleOutputsInputs(ctx, c)
}

func buildTestDeployPipeline(ctx context.Context, c client.Client) {
	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "build",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo '{"version":"1.2.3","build_id":"abc123"}' && exit 0`},
					},
					// Define outputs to capture
					Outputs: []docker.OutputDefinition{
						{
							Name:      "version",
							ValueFrom: "stdout",
							JSONPath:  "$.version",
						},
						{
							Name:      "build_id",
							ValueFrom: "stdout",
							JSONPath:  "$.build_id",
						},
						{
							Name:      "exit_code",
							ValueFrom: "exitCode",
						},
					},
				},
			},
			{
				Name: "test",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo "Testing version: $BUILD_VERSION (build: $BUILD_ID)" && echo "Tests passed" && exit 0`},
					},
					// Map outputs from build step to inputs
					Inputs: []docker.InputMapping{
						{
							Name:     "BUILD_VERSION",
							From:     "build.version",
							Required: true,
						},
						{
							Name:     "BUILD_ID",
							From:     "build.build_id",
							Required: true,
						},
					},
					Outputs: []docker.OutputDefinition{
						{
							Name:      "test_result",
							ValueFrom: "stdout",
							Regex:     `Tests (\w+)`,
						},
					},
				},
				Dependencies: []string{"build"},
			},
			{
				Name: "deploy",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo "Deploying version $VERSION to production" && echo "Deployment successful" && exit 0`},
					},
					Inputs: []docker.InputMapping{
						{
							Name:     "VERSION",
							From:     "build.version",
							Required: true,
						},
						{
							Name:     "TEST_RESULT",
							From:     "test.test_result",
							Required: true,
						},
					},
				},
				Dependencies: []string{"test"},
			},
		},
		FailFast: true,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "data-passing-pipeline",
		TaskQueue: "docker-workflow",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started DAG workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result docker.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Pipeline completed: Success=%d, Failed=%d, Duration=%s\n",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)

	// Print captured outputs
	fmt.Println("\nCaptured Outputs:")
	for stepName, outputs := range result.StepOutputs {
		fmt.Printf("  %s:\n", stepName)
		for key, value := range outputs {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}
}

func jsonOutputExtraction(ctx context.Context, c client.Client) {
	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "generate-config",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image: "alpine:latest",
						Command: []string{"sh", "-c", `cat <<EOF
{
  "database": {
    "host": "localhost",
    "port": 5432,
    "name": "mydb"
  },
  "cache": {
    "enabled": true,
    "ttl": 3600
  },
  "servers": ["server1", "server2", "server3"]
}
EOF
`},
					},
					Outputs: []docker.OutputDefinition{
						{
							Name:      "db_host",
							ValueFrom: "stdout",
							JSONPath:  "$.database.host",
						},
						{
							Name:      "db_port",
							ValueFrom: "stdout",
							JSONPath:  "$.database.port",
						},
						{
							Name:      "cache_enabled",
							ValueFrom: "stdout",
							JSONPath:  "$.cache.enabled",
						},
						{
							Name:      "first_server",
							ValueFrom: "stdout",
							JSONPath:  "$.servers[0]",
						},
					},
				},
			},
			{
				Name: "use-config",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo "Connecting to $DB_HOST:$DB_PORT" && echo "Cache enabled: $CACHE_ENABLED" && echo "Primary server: $PRIMARY_SERVER"`},
					},
					Inputs: []docker.InputMapping{
						{
							Name:     "DB_HOST",
							From:     "generate-config.db_host",
							Required: true,
						},
						{
							Name:     "DB_PORT",
							From:     "generate-config.db_port",
							Required: true,
						},
						{
							Name:     "CACHE_ENABLED",
							From:     "generate-config.cache_enabled",
							Required: false,
							Default:  "false",
						},
						{
							Name:     "PRIMARY_SERVER",
							From:     "generate-config.first_server",
							Required: true,
						},
					},
				},
				Dependencies: []string{"generate-config"},
			},
		},
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "json-extraction-example",
		TaskQueue: "docker-workflow",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s\n", we.GetID())

	var result docker.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("JSON extraction completed: Success=%d\n", result.TotalSuccess)

	// Print extracted values
	if outputs, ok := result.StepOutputs["generate-config"]; ok {
		fmt.Println("\nExtracted JSON Values:")
		for key, value := range outputs {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

func regexExtraction(ctx context.Context, c client.Client) {
	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "build-app",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo "Building application..." && echo "Build completed: version v1.2.3, build #456" && echo "Artifact: myapp-v1.2.3.tar.gz"`},
					},
					Outputs: []docker.OutputDefinition{
						{
							Name:      "version",
							ValueFrom: "stdout",
							Regex:     `version (v\d+\.\d+\.\d+)`,
						},
						{
							Name:      "build_number",
							ValueFrom: "stdout",
							Regex:     `build #(\d+)`,
						},
						{
							Name:      "artifact_name",
							ValueFrom: "stdout",
							Regex:     `Artifact: ([\w\-\.]+)`,
						},
					},
				},
			},
			{
				Name: "upload-artifact",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo "Uploading $ARTIFACT_NAME (version $VERSION, build $BUILD_NUM)"`},
					},
					Inputs: []docker.InputMapping{
						{
							Name:     "ARTIFACT_NAME",
							From:     "build-app.artifact_name",
							Required: true,
						},
						{
							Name:     "VERSION",
							From:     "build-app.version",
							Required: true,
						},
						{
							Name:     "BUILD_NUM",
							From:     "build-app.build_number",
							Required: true,
						},
					},
				},
				Dependencies: []string{"build-app"},
			},
		},
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "regex-extraction-example",
		TaskQueue: "docker-workflow",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s\n", we.GetID())

	var result docker.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Regex extraction completed: Success=%d\n", result.TotalSuccess)

	// Print extracted values
	if outputs, ok := result.StepOutputs["build-app"]; ok {
		fmt.Println("\nExtracted using Regex:")
		for key, value := range outputs {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

func multipleOutputsInputs(ctx context.Context, c client.Client) {
	input := docker.DAGWorkflowInput{
		Nodes: []docker.DAGNode{
			{
				Name: "analyze",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo '{"files":100,"lines":5000,"complexity":"low"}' && echo "Analysis complete" >&2 && exit 0`},
					},
					Outputs: []docker.OutputDefinition{
						{
							Name:      "file_count",
							ValueFrom: "stdout",
							JSONPath:  "$.files",
						},
						{
							Name:      "line_count",
							ValueFrom: "stdout",
							JSONPath:  "$.lines",
						},
						{
							Name:      "complexity",
							ValueFrom: "stdout",
							JSONPath:  "$.complexity",
						},
						{
							Name:      "status",
							ValueFrom: "stderr",
							Regex:     `Analysis (\w+)`,
						},
						{
							Name:      "result_code",
							ValueFrom: "exitCode",
						},
					},
				},
			},
			{
				Name: "report",
				Container: docker.ExtendedContainerInput{
					ContainerExecutionInput: docker.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", `echo "Analysis Report:" && echo "Files: $FILES, Lines: $LINES" && echo "Complexity: $COMPLEXITY, Status: $STATUS" && echo "Exit code: $EXIT_CODE"`},
					},
					Inputs: []docker.InputMapping{
						{
							Name:     "FILES",
							From:     "analyze.file_count",
							Required: true,
						},
						{
							Name:     "LINES",
							From:     "analyze.line_count",
							Required: true,
						},
						{
							Name:     "COMPLEXITY",
							From:     "analyze.complexity",
							Required: true,
						},
						{
							Name:     "STATUS",
							From:     "analyze.status",
							Required: true,
						},
						{
							Name:     "EXIT_CODE",
							From:     "analyze.result_code",
							Required: false,
							Default:  "0",
						},
					},
				},
				Dependencies: []string{"analyze"},
			},
		},
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "multiple-outputs-example",
		TaskQueue: "docker-workflow",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s\n", we.GetID())

	var result docker.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Multiple outputs workflow completed: Success=%d\n", result.TotalSuccess)

	// Print all step outputs
	fmt.Println("\nAll Step Outputs:")
	for stepName, outputs := range result.StepOutputs {
		fmt.Printf("  %s:\n", stepName)
		for key, value := range outputs {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}
}
