//go:build example

package main

import (
	"context"
	"fmt"
	"log"

	"go.temporal.io/sdk/client"

	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/container/workflow"
	"github.com/jasoet/go-wf/workflow/artifacts"
)

// This example demonstrates artifact storage and retrieval in workflows.
// It shows how to:
// 1. Configure artifact storage (local or S3-compatible)
// 2. Upload artifacts from one step
// 3. Download artifacts in dependent steps
// 4. Use artifacts in build -> test -> deploy pipelines

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

	// Example 1: Build -> Test pipeline with artifacts
	fmt.Println("\n=== Example 1: Build -> Test Pipeline with Artifacts ===")
	buildTestPipeline(ctx, c)

	// Example 2: Build -> Test -> Deploy with multiple artifacts
	fmt.Println("\n=== Example 2: Build -> Test -> Deploy with Multiple Artifacts ===")
	buildTestDeployPipeline(ctx, c)

	// Example 3: Using S3-compatible storage for artifacts
	fmt.Println("\n=== Example 3: Using S3-Compatible Artifact Storage ===")
	s3ArtifactStorage(ctx, c)

	// Example 4: Archive artifacts with cleanup config
	fmt.Println("\n=== Example 4: Archive Artifacts & Cleanup Config ===")
	artifactCleanupExample(ctx, c)
}

func buildTestPipeline(ctx context.Context, c client.Client) {
	// Create local file store for artifacts
	store, err := artifacts.NewLocalFileStore("/tmp/workflow-artifacts")
	if err != nil {
		log.Fatalln("Failed to create artifact store", err)
	}
	defer store.Close()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "golang:1.25",
						Command: []string{"sh", "-c", "go build -o /output/app . && echo 'Binary built successfully'"},
						Volumes: map[string]string{
							"/path/to/source":   "/workspace",
							"/tmp/build-output": "/output",
						},
						WorkDir: "/workspace",
					},
					// Define output artifacts to upload after build
					OutputArtifacts: []payload.Artifact{
						{
							Name: "binary",
							Path: "/tmp/build-output/app",
							Type: "file",
						},
					},
				},
			},
			{
				Name: "test",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", "/app/app --version && echo 'Binary test passed'"},
						Volumes: map[string]string{
							"/tmp/test-workspace": "/app",
						},
					},
					// Define input artifacts to download before test
					InputArtifacts: []payload.Artifact{
						{
							Name: "binary",
							Path: "/tmp/test-workspace/app",
							Type: "file",
						},
					},
				},
				Dependencies: []string{"build"},
			},
		},
		ArtifactStore: store,
		FailFast:      true,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "build-test-artifacts",
		TaskQueue: "container-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Pipeline completed: Success=%d, Failed=%d, Duration=%s\n",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

func buildTestDeployPipeline(ctx context.Context, c client.Client) {
	// Create local file store for artifacts
	store, err := artifacts.NewLocalFileStore("/tmp/workflow-artifacts")
	if err != nil {
		log.Fatalln("Failed to create artifact store", err)
	}
	defer store.Close()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "build",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image: "golang:1.25",
						Command: []string{"sh", "-c", `
							go build -o /output/app . &&
							echo '{"version":"1.2.3","build":"456"}' > /output/metadata.json
						`},
						Volumes: map[string]string{
							"/path/to/source":   "/workspace",
							"/tmp/build-output": "/output",
						},
						WorkDir: "/workspace",
					},
					// Multiple output artifacts
					OutputArtifacts: []payload.Artifact{
						{
							Name: "binary",
							Path: "/tmp/build-output/app",
							Type: "file",
						},
						{
							Name: "metadata",
							Path: "/tmp/build-output/metadata.json",
							Type: "file",
						},
					},
					// Capture version from output
					Outputs: []payload.OutputDefinition{
						{
							Name:      "version",
							ValueFrom: "stdout",
							Regex:     `version: (v[\d.]+)`,
							Default:   "unknown",
						},
					},
				},
			},
			{
				Name: "test",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "alpine:latest",
						Command: []string{"sh", "-c", "/app/app --version && cat /app/metadata.json"},
						Volumes: map[string]string{
							"/tmp/test-workspace": "/app",
						},
					},
					// Download artifacts from build
					InputArtifacts: []payload.Artifact{
						{
							Name: "binary",
							Path: "/tmp/test-workspace/app",
							Type: "file",
						},
						{
							Name: "metadata",
							Path: "/tmp/test-workspace/metadata.json",
							Type: "file",
						},
					},
					// Upload test results
					OutputArtifacts: []payload.Artifact{
						{
							Name:     "test-results",
							Path:     "/tmp/test-results",
							Type:     "directory",
							Optional: true,
						},
					},
				},
				Dependencies: []string{"build"},
			},
			{
				Name: "deploy",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "deployer:v1",
						Command: []string{"sh", "-c", "deploy --binary=/deploy/app --env=$ENVIRONMENT"},
						Env: map[string]string{
							"ENVIRONMENT": "staging",
						},
						Volumes: map[string]string{
							"/tmp/deploy-workspace": "/deploy",
						},
					},
					// Download binary from build
					InputArtifacts: []payload.Artifact{
						{
							Name: "binary",
							Path: "/tmp/deploy-workspace/app",
							Type: "file",
						},
					},
					// Use version from build step
					Inputs: []payload.InputMapping{
						{
							Name:     "VERSION",
							From:     "build.version",
							Required: false,
							Default:  "unknown",
						},
					},
				},
				Dependencies: []string{"test"},
			},
		},
		ArtifactStore: store,
		FailFast:      true,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "build-test-deploy-artifacts",
		TaskQueue: "container-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.DAGWorkflowOutput
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

func s3ArtifactStorage(ctx context.Context, c client.Client) {
	// Create S3 store for artifacts
	// In production, these would come from configuration
	s3Config := artifacts.S3Config{
		Endpoint:  "localhost:9000",
		AccessKey: "rustfsadmin",
		SecretKey: "rustfsadmin",
		Bucket:    "workflow-artifacts",
		Prefix:    "workflows/",
		UseSSL:    false,
		Region:    "us-east-1",
	}

	store, err := artifacts.NewS3Store(ctx, s3Config)
	if err != nil {
		log.Fatalln("Failed to create S3 store", err)
	}
	defer store.Close()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name: "process-data",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "data-processor:v1",
						Command: []string{"sh", "-c", "process --input=/data/input.csv --output=/data/output.csv"},
						Volumes: map[string]string{
							"/tmp/data": "/data",
						},
					},
					// Upload processed data to S3
					OutputArtifacts: []payload.Artifact{
						{
							Name: "processed-data",
							Path: "/tmp/data/output.csv",
							Type: "file",
						},
					},
				},
			},
			{
				Name: "analyze-results",
				Container: payload.ExtendedContainerInput{
					ContainerExecutionInput: payload.ContainerExecutionInput{
						Image:   "data-analyzer:v1",
						Command: []string{"sh", "-c", "analyze --input=/analysis/output.csv"},
						Volumes: map[string]string{
							"/tmp/analysis": "/analysis",
						},
					},
					// Download processed data from S3
					InputArtifacts: []payload.Artifact{
						{
							Name: "processed-data",
							Path: "/tmp/analysis/output.csv",
							Type: "file",
						},
					},
				},
				Dependencies: []string{"process-data"},
			},
		},
		ArtifactStore: store,
		FailFast:      true,
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        "s3-artifacts-example",
		TaskQueue: "container-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, workflow.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("S3 pipeline completed: Success=%d, Failed=%d\n",
		result.TotalSuccess, result.TotalFailed)
}

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
						Command: []string{
							"sh", "-c",
							"mkdir -p /tmp/build-output && " +
								"echo 'binary content' > /tmp/build-output/app && " +
								"echo 'config content' > /tmp/build-output/config.yaml && " +
								"echo 'Archive created'",
						},
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
		TaskQueue: "container-tasks",
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
