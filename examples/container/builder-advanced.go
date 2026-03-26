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

	"github.com/jasoet/go-wf/container"
	"github.com/jasoet/go-wf/container/builder"
	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/container/template"
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

	// Example 1: BuildSingle with constructor options
	log.Println("=== Example 1: Single Container Builder ===")
	runSingleContainerBuilder(c)

	time.Sleep(2 * time.Second)

	// Example 2: Build() auto-select with ContainerSource and AddInput
	log.Println("\n=== Example 2: Auto-Select Builder ===")
	runAutoSelectBuilder(c)

	time.Sleep(2 * time.Second)

	// Example 3: Cleanup pipeline with container options
	log.Println("\n=== Example 3: Cleanup Pipeline ===")
	runCleanupPipeline(c)

	time.Sleep(2 * time.Second)

	// Example 4: Parameterized loop builder
	log.Println("\n=== Example 4: Parameterized Loop Builder ===")
	runParameterizedLoopBuilder(c)

	time.Sleep(2 * time.Second)

	// Example 5: Additional templates (GoScript, HTTPWebhook)
	log.Println("\n=== Example 5: Additional Templates ===")
	runAdditionalTemplates(c)
}

// runSingleContainerBuilder demonstrates BuildSingle with constructor options:
// WithStopOnError, WithGlobalAutoRemove, WithGlobalTimeout.
func runSingleContainerBuilder(c client.Client) {
	// Use constructor options to pre-configure the builder
	singleInput, err := builder.NewWorkflowBuilder("single-container",
		builder.WithStopOnError(true),
		builder.WithGlobalAutoRemove(true),
		builder.WithGlobalTimeout(3*time.Minute),
	).
		Add(template.NewContainer("alpine-task", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Running single container task' && sleep 1 && echo 'Done'"),
			template.WithEnv("TASK_ID", "single-001"),
		)).
		BuildSingle()
	if err != nil {
		log.Printf("Failed to build single container: %v", err)
		return
	}

	// ExecuteContainerWorkflow is used for BuildSingle results
	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        fmt.Sprintf("single-builder-%d", time.Now().UnixNano()),
			TaskQueue: "container-tasks",
		},
		workflow.ExecuteContainerWorkflow,
		*singleInput,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}

	var result payload.ContainerExecutionOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Workflow failed: %v", err)
		return
	}

	log.Printf("Single container completed: ExitCode=%d, Stdout=%s", result.ExitCode, result.Stdout)
}

// runAutoSelectBuilder demonstrates Build() auto-select, ContainerSource,
// NewContainerSource, AddInput, WithParallelMode, and WithMaxConcurrency.
func runAutoSelectBuilder(c client.Client) {
	// Create a raw ContainerExecutionInput and wrap it with NewContainerSource
	rawInput := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Task from ContainerSource' && sleep 1"},
		AutoRemove: true,
		Name:       "source-task",
	}
	source := builder.NewContainerSource(rawInput)

	// Use WithParallelMode and WithMaxConcurrency as constructor options
	// Build() will auto-select ParallelInput because parallel mode is true
	wb := builder.NewWorkflowBuilder("auto-select-parallel",
		builder.WithParallelMode(true),
		builder.WithMaxConcurrency(2),
	)

	// Add via ContainerSource (implements WorkflowSource)
	wb.Add(source)

	// Add via AddInput (raw payload)
	wb.AddInput(payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Task from AddInput' && sleep 1"},
		AutoRemove: true,
		Name:       "direct-input-task",
	})

	// Add another container via template
	wb.Add(template.NewContainer("template-task", "alpine:latest",
		template.WithCommand("sh", "-c", "echo 'Task from template' && sleep 1"),
	))

	// Build() auto-selects based on parallel mode
	result, err := wb.Build()
	if err != nil {
		log.Printf("Failed to build: %v", err)
		return
	}

	// When ParallelMode is true, cast to *payload.ParallelInput
	parallelInput, ok := result.(*payload.ParallelInput)
	if !ok {
		log.Printf("Expected *payload.ParallelInput, got %T", result)
		return
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        fmt.Sprintf("auto-select-parallel-%d", time.Now().UnixNano()),
			TaskQueue: "container-tasks",
		},
		workflow.ParallelContainersWorkflow,
		*parallelInput,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}

	var output payload.ParallelOutput
	if err := we.Get(context.Background(), &output); err != nil {
		log.Printf("Parallel workflow failed: %v", err)
		return
	}

	log.Printf("Auto-select parallel completed: Success=%d, Failed=%d, Duration=%v",
		output.TotalSuccess, output.TotalFailed, output.TotalDuration)
}

// runCleanupPipeline demonstrates Cleanup(true) along with container options:
// WithVolume, WithPorts, WithLabel, WithWaitForLog, WithWaitForPort, and WithCleanup.
func runCleanupPipeline(c client.Client) {
	// Use WithCleanup as a constructor option
	wb := builder.NewWorkflowBuilder("cleanup-pipeline",
		builder.WithCleanup(true),
	)

	// Container with volume, ports, and label options
	wb.Add(template.NewContainer("app-server", "alpine:latest",
		template.WithCommand("sh", "-c", "echo 'Server starting...' && echo 'ready to serve' && sleep 2"),
		template.WithVolume("/tmp/app-data", "/data"),
		template.WithPorts("8080:8080", "9090:9090"),
		template.WithLabel("app", "demo-server"),
		template.WithLabel("tier", "frontend"),
		template.WithWaitForLog("ready to serve"),
	))

	// Container with port-based wait strategy
	wb.Add(template.NewContainer("db-service", "alpine:latest",
		template.WithCommand("sh", "-c", "echo 'Database ready' && sleep 1"),
		template.WithVolume("/tmp/db-data", "/var/lib/db"),
		template.WithLabel("app", "demo-db"),
		template.WithWaitForPort("5432"),
	))

	// Also toggle Cleanup via fluent method
	wb.Cleanup(true)

	input, err := wb.StopOnError(true).BuildPipeline()
	if err != nil {
		log.Printf("Failed to build cleanup pipeline: %v", err)
		return
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        fmt.Sprintf("cleanup-pipeline-%d", time.Now().UnixNano()),
			TaskQueue: "container-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		*input,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}

	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Cleanup pipeline failed: %v", err)
		return
	}

	log.Printf("Cleanup pipeline completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

// runParameterizedLoopBuilder demonstrates ForEachParam, NewParameterizedLoopBuilder,
// WithTemplate, and BuildParameterizedLoop.
func runParameterizedLoopBuilder(c client.Client) {
	parameters := map[string][]string{
		"env":    {"dev", "staging", "production"},
		"region": {"us-west-1", "us-east-1"},
	}

	tmpl := payload.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"sh", "-c", "echo 'Deploying to env={{.env}} region={{.region}}' && sleep 1"},
		AutoRemove: true,
		Name:       "deploy-step",
	}

	// Approach 1: ForEachParam convenience function
	loopInput1, err := builder.ForEachParam(parameters, tmpl).
		Parallel(true).
		MaxConcurrency(3).
		BuildParameterizedLoop()
	if err != nil {
		log.Printf("ForEachParam failed: %v", err)
		return
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        fmt.Sprintf("param-loop-foreach-%d", time.Now().UnixNano()),
			TaskQueue: "container-tasks",
		},
		workflow.ParameterizedLoopWorkflow,
		*loopInput1,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}

	var output1 payload.LoopOutput
	if err := we.Get(context.Background(), &output1); err != nil {
		log.Printf("Parameterized loop (ForEachParam) failed: %v", err)
		return
	}

	log.Printf("ForEachParam loop completed: ItemCount=%d, Success=%d",
		output1.ItemCount, output1.TotalSuccess)

	// Approach 2: NewParameterizedLoopBuilder + WithTemplate (step-by-step)
	loopInput2, err := builder.NewParameterizedLoopBuilder(parameters).
		WithTemplate(tmpl).
		Parallel(false).
		FailFast(true).
		BuildParameterizedLoop()
	if err != nil {
		log.Printf("NewParameterizedLoopBuilder failed: %v", err)
		return
	}

	we2, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        fmt.Sprintf("param-loop-builder-%d", time.Now().UnixNano()),
			TaskQueue: "container-tasks",
		},
		workflow.ParameterizedLoopWorkflow,
		*loopInput2,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}

	var output2 payload.LoopOutput
	if err := we2.Get(context.Background(), &output2); err != nil {
		log.Printf("Parameterized loop (builder) failed: %v", err)
		return
	}

	log.Printf("ParameterizedLoopBuilder completed: ItemCount=%d, Success=%d",
		output2.ItemCount, output2.TotalSuccess)
}

// runAdditionalTemplates demonstrates NewGoScript, NewHTTPWebhook,
// WithScriptVolume, WithScriptPorts, and additional container options.
func runAdditionalTemplates(c client.Client) {
	// Go script with volume and port options
	goScript := template.NewGoScript("go-task",
		`package main
import "fmt"
func main() {
	fmt.Println("Hello from Go script!")
	fmt.Println("Running inside a container")
}`,
		template.WithScriptVolume("/tmp/go-cache", "/go/pkg"),
		template.WithScriptPorts("8081:8081"),
		template.WithScriptEnv("GOPROXY", "direct"),
	)

	// HTTP webhook template
	webhook := template.NewHTTPWebhook("notify-webhook",
		"https://httpbin.org/post",
		`{"event": "build_complete", "status": "success"}`,
		template.WithHTTPHeader("X-Custom-Header", "workflow-event"),
		template.WithHTTPTimeout(15),
		template.WithHTTPExpectedStatus(200),
	)

	// Container with multiple advanced options
	advancedContainer := template.NewContainer("advanced-task", "alpine:latest",
		template.WithCommand("sh", "-c", "echo 'Advanced container running' && sleep 1"),
		template.WithVolume("/tmp/shared", "/shared"),
		template.WithPorts("3000:3000"),
		template.WithLabel("managed-by", "go-wf"),
		template.WithLabel("version", "v2"),
		template.WithWorkDir("/shared"),
		template.WithUser("nobody"),
		template.WithAutoRemove(true),
	)

	input, err := builder.NewWorkflowBuilder("additional-templates").
		Add(goScript).
		Add(webhook).
		Add(advancedContainer).
		StopOnError(false).
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build additional templates pipeline: %v", err)
		return
	}

	we, err := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        fmt.Sprintf("additional-templates-%d", time.Now().UnixNano()),
			TaskQueue: "container-tasks",
		},
		workflow.ContainerPipelineWorkflow,
		*input,
	)
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return
	}

	var result payload.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Additional templates pipeline failed: %v", err)
		return
	}

	log.Printf("Additional templates completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}
