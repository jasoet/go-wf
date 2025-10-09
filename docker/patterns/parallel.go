package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/template"
)

// FanOutFanIn creates a workflow that executes multiple tasks in parallel.
//
// Example:
//
//	input, err := patterns.FanOutFanIn(
//	    "alpine:latest",
//	    []string{"task-1", "task-2", "task-3"})
func FanOutFanIn(image string, tasks []string) (*docker.ParallelInput, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("at least one task is required")
	}

	wb := builder.NewWorkflowBuilder("fan-out-fan-in").Parallel(true)

	for _, taskName := range tasks {
		task := template.NewContainer(taskName, image,
			template.WithCommand("sh", "-c", fmt.Sprintf("echo 'Processing %s' && sleep 1", taskName)))

		wb.Add(task)
	}

	return wb.BuildParallel()
}

// ParallelDataProcessing creates a workflow that processes multiple data items in parallel.
//
// Example:
//
//	input, err := patterns.ParallelDataProcessing(
//	    "processor:v1",
//	    []string{"data-1.csv", "data-2.csv", "data-3.csv"},
//	    "process.sh")
func ParallelDataProcessing(image string, dataItems []string, command string) (*docker.ParallelInput, error) {
	if len(dataItems) == 0 {
		return nil, fmt.Errorf("at least one data item is required")
	}

	wb := builder.NewWorkflowBuilder("data-processing").Parallel(true)

	for i, dataItem := range dataItems {
		task := template.NewContainer(fmt.Sprintf("process-%d", i), image,
			template.WithCommand("sh", "-c", fmt.Sprintf("%s %s", command, dataItem)),
			template.WithEnv("DATA_ITEM", dataItem),
			template.WithEnv("ITEM_INDEX", fmt.Sprintf("%d", i)))

		wb.Add(task)
	}

	return wb.BuildParallel()
}

// ParallelTestSuite creates a workflow that runs multiple test suites in parallel.
//
// Example:
//
//	input, err := patterns.ParallelTestSuite(
//	    "golang:1.25",
//	    map[string]string{
//	        "unit": "go test ./internal/...",
//	        "integration": "go test ./tests/integration/...",
//	    })
func ParallelTestSuite(image string, testSuites map[string]string) (*docker.ParallelInput, error) {
	if len(testSuites) == 0 {
		return nil, fmt.Errorf("at least one test suite is required")
	}

	wb := builder.NewWorkflowBuilder("test-suite").Parallel(true).FailFast(true)

	for suiteName, testCmd := range testSuites {
		task := template.NewContainer("test-"+suiteName, image,
			template.WithCommand("sh", "-c", testCmd),
			template.WithWorkDir("/workspace"))

		wb.Add(task)
	}

	return wb.BuildParallel()
}

// ParallelDeployment creates a workflow that deploys to multiple regions in parallel.
//
// Example:
//
//	input, err := patterns.ParallelDeployment(
//	    "deployer:v1",
//	    []string{"us-west", "us-east", "eu-central"})
func ParallelDeployment(deployImage string, regions []string) (*docker.ParallelInput, error) {
	if len(regions) == 0 {
		return nil, fmt.Errorf("at least one region is required")
	}

	wb := builder.NewWorkflowBuilder("multi-region-deploy").
		Parallel(true).
		FailFast(false)

	for _, region := range regions {
		deploy := template.NewContainer("deploy-"+region, deployImage,
			template.WithCommand("deploy.sh"),
			template.WithEnv("REGION", region))

		wb.Add(deploy)
	}

	return wb.BuildParallel()
}

// MapReduce creates a map-reduce style workflow.
//
// Example:
//
//	input, err := patterns.MapReduce(
//	    "alpine:latest",
//	    []string{"file1.txt", "file2.txt"},
//	    "wc -w",
//	    "awk '{sum+=$1} END {print sum}'")
func MapReduce(image string, inputs []string, mapCmd, reduceCmd string) (*docker.PipelineInput, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("at least one input is required")
	}

	// Create map tasks
	mapBuilder := builder.NewWorkflowBuilder("map-phase").Parallel(true)

	for i, input := range inputs {
		mapTask := template.NewContainer(fmt.Sprintf("map-%d", i), image,
			template.WithCommand("sh", "-c", fmt.Sprintf("echo 'Mapping %s' && %s %s", input, mapCmd, input)),
			template.WithEnv("INPUT", input))

		mapBuilder.Add(mapTask)
	}

	// Build the complete pipeline with map and reduce
	// Note: In a real implementation, you'd need to handle data passing between stages
	mapInput, err := mapBuilder.BuildParallel()
	if err != nil {
		return nil, err
	}

	// Create reduce task
	reduce := template.NewContainer("reduce", image,
		template.WithCommand("sh", "-c", fmt.Sprintf("echo 'Reducing...' && %s", reduceCmd)))

	// Convert to pipeline for now (simplified)
	wb := builder.NewWorkflowBuilder("map-reduce")
	for _, container := range mapInput.Containers {
		wb.AddInput(container)
	}
	wb.Add(reduce)

	return wb.BuildPipeline()
}
