//go:build example

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/artifacts"
	"github.com/jasoet/go-wf/docker/payload"
	"go.temporal.io/sdk/client"
)

// This example demonstrates artifact storage and retrieval in workflows.
// It shows how to:
// 1. Configure artifact storage (local or Minio)
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

	// Example 3: Using Minio for artifact storage
	fmt.Println("\n=== Example 3: Using Minio for Artifact Storage ===")
	minioArtifactStorage(ctx, c)
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
						Volumes: []docker.VolumeMount{
							{Source: "/path/to/source", Target: "/workspace", ReadOnly: true},
							{Source: "/tmp/build-output", Target: "/output", ReadOnly: false},
						},
						WorkingDir: "/workspace",
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
						Volumes: []docker.VolumeMount{
							{Source: "/tmp/test-workspace", Target: "/app", ReadOnly: false},
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
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
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
						Volumes: []docker.VolumeMount{
							{Source: "/path/to/source", Target: "/workspace", ReadOnly: true},
							{Source: "/tmp/build-output", Target: "/output", ReadOnly: false},
						},
						WorkingDir: "/workspace",
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
						Volumes: []docker.VolumeMount{
							{Source: "/tmp/test-workspace", Target: "/app", ReadOnly: false},
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
						Volumes: []docker.VolumeMount{
							{Source: "/tmp/deploy-workspace", Target: "/deploy", ReadOnly: false},
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
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
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

func minioArtifactStorage(ctx context.Context, c client.Client) {
	// Create Minio store for artifacts
	// In production, these would come from configuration
	minioConfig := artifacts.MinioConfig{
		Endpoint:  "localhost:9000",
		AccessKey: "minioadmin",
		SecretKey: "minioadmin",
		Bucket:    "workflow-artifacts",
		Prefix:    "workflows/",
		UseSSL:    false,
		Region:    "us-east-1",
	}

	store, err := artifacts.NewMinioStore(ctx, minioConfig)
	if err != nil {
		log.Fatalln("Failed to create Minio store", err)
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
						Volumes: []docker.VolumeMount{
							{Source: "/tmp/data", Target: "/data", ReadOnly: false},
						},
					},
					// Upload processed data to Minio
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
						Volumes: []docker.VolumeMount{
							{Source: "/tmp/analysis", Target: "/analysis", ReadOnly: false},
						},
					},
					// Download processed data from Minio
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
		ID:        "minio-artifacts-example",
		TaskQueue: "docker-tasks",
	}

	we, err := c.ExecuteWorkflow(ctx, workflowOptions, docker.DAGWorkflow, input)
	if err != nil {
		log.Fatalln("Unable to execute workflow", err)
	}

	fmt.Printf("Started workflow: WorkflowID=%s, RunID=%s\n", we.GetID(), we.GetRunID())

	var result payload.DAGWorkflowOutput
	err = we.Get(ctx, &result)
	if err != nil {
		log.Fatalln("Unable to get workflow result", err)
	}

	fmt.Printf("Minio pipeline completed: Success=%d, Failed=%d\n",
		result.TotalSuccess, result.TotalFailed)
}
