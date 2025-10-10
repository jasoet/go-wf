//go:build example

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/template"
	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// Create Temporal client
	c, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

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

	// Example 1: Simple CI/CD Pipeline with Builder
	log.Println("=== Example 1: CI/CD Pipeline ===")
	runCICDPipeline(c)

	time.Sleep(2 * time.Second)

	// Example 2: Parallel Data Processing
	log.Println("\n=== Example 2: Parallel Data Processing ===")
	runParallelProcessing(c)

	time.Sleep(2 * time.Second)

	// Example 3: Script Templates
	log.Println("\n=== Example 3: Multi-Language Scripts ===")
	runScriptExamples(c)

	time.Sleep(2 * time.Second)

	// Example 4: HTTP Templates (Health Checks & Webhooks)
	log.Println("\n=== Example 4: HTTP Operations ===")
	runHTTPExamples(c)

	time.Sleep(2 * time.Second)

	// Example 5: Loop-like Pattern (Programmatic Container Creation)
	log.Println("\n=== Example 5: Loop Pattern ===")
	runLoopPattern(c)

	time.Sleep(2 * time.Second)

	// Example 6: Exit Handlers
	log.Println("\n=== Example 6: Exit Handlers ===")
	runExitHandlers(c)
}

// Example 1: CI/CD Pipeline using Builder
func runCICDPipeline(c client.Client) {
	input, err := builder.NewWorkflowBuilder("cicd-pipeline").
		Add(template.NewContainer("checkout", "alpine/git",
			template.WithCommand("sh", "-c", "echo 'Cloning repository...' && sleep 1"))).
		Add(template.NewBashScript("build",
			`echo "Building application..."
			echo "Compiling..."
			sleep 2
			echo "Build complete!"`,
			template.WithScriptImage("golang:1.23-alpine"))).
		Add(template.NewBashScript("test",
			`echo "Running tests..."
			echo "Test suite: PASS"
			sleep 2`,
			template.WithScriptImage("golang:1.23-alpine"))).
		Add(template.NewContainer("package", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Creating artifacts...' && sleep 1"))).
		Add(template.NewContainer("deploy", "alpine:latest",
			template.WithCommand("sh", "-c", "echo 'Deploying to staging...' && sleep 1"),
			template.WithEnv("ENVIRONMENT", "staging"))).
		StopOnError(true).
		WithTimeout(5 * time.Minute).
		WithAutoRemove(true).
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build pipeline: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "cicd-pipeline-example",
			TaskQueue: "docker-tasks",
		},
		docker.ContainerPipelineWorkflow,
		*input,
	)

	var result docker.PipelineOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Pipeline failed: %v", err)
		return
	}

	log.Printf("Pipeline completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

// Example 2: Parallel Data Processing
func runParallelProcessing(c client.Client) {
	// Process multiple data files in parallel
	dataFiles := []string{"data-001.csv", "data-002.csv", "data-003.csv", "data-004.csv"}

	wb := builder.NewWorkflowBuilder("parallel-processing").
		Parallel(true).
		MaxConcurrency(2). // Process 2 at a time
		FailFast(false)    // Continue even if some fail

	for i, file := range dataFiles {
		wb.Add(template.NewPythonScript(fmt.Sprintf("process-%d", i),
			fmt.Sprintf(`
import time
print("Processing %s...")
time.sleep(2)
print("%s processed successfully")
			`, file, file),
			template.WithScriptImage("python:3.11-slim")))
	}

	input, err := wb.BuildParallel()
	if err != nil {
		log.Printf("Failed to build parallel workflow: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "parallel-processing-example",
			TaskQueue: "docker-tasks",
		},
		docker.ParallelContainersWorkflow,
		*input,
	)

	var result docker.ParallelOutput
	if err := we.Get(context.Background(), &result); err != nil {
		log.Printf("Parallel processing failed: %v", err)
		return
	}

	log.Printf("Parallel processing completed: Success=%d, Failed=%d, Duration=%v",
		result.TotalSuccess, result.TotalFailed, result.TotalDuration)
}

// Example 3: Multi-Language Script Templates
func runScriptExamples(c client.Client) {
	input, err := builder.NewWorkflowBuilder("scripts-demo").
		Add(template.NewBashScript("bash-script",
			`echo "Running Bash script"
			date
			uname -a`)).
		Add(template.NewPythonScript("python-script",
			`import sys
import platform
print(f"Python {sys.version}")
print(f"Platform: {platform.system()}")`)).
		Add(template.NewNodeScript("node-script",
			`console.log("Node.js version:", process.version);
			console.log("Platform:", process.platform);`)).
		Add(template.NewRubyScript("ruby-script",
			`puts "Ruby version: #{RUBY_VERSION}"
			puts "Platform: #{RUBY_PLATFORM}"`)).
		StopOnError(false). // Continue even if scripts fail
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build script workflow: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "scripts-demo-example",
			TaskQueue: "docker-tasks",
		},
		docker.ContainerPipelineWorkflow,
		*input,
	)

	var result docker.PipelineOutput
	we.Get(context.Background(), &result)
	log.Printf("Scripts demo completed: Success=%d", result.TotalSuccess)
}

// Example 4: HTTP Operations (Health Checks & Webhooks)
func runHTTPExamples(c client.Client) {
	input, err := builder.NewWorkflowBuilder("http-demo").
		Add(template.NewHTTPHealthCheck("check-google",
			"https://www.google.com",
			template.WithHTTPExpectedStatus(200))).
		Add(template.NewHTTP("api-call",
			template.WithHTTPURL("https://httpbin.org/post"),
			template.WithHTTPMethod("POST"),
			template.WithHTTPBody(`{"message": "test"}`),
			template.WithHTTPHeader("Content-Type", "application/json"),
			template.WithHTTPExpectedStatus(200))).
		StopOnError(true).
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build HTTP workflow: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "http-demo-example",
			TaskQueue: "docker-tasks",
		},
		docker.ContainerPipelineWorkflow,
		*input,
	)

	var result docker.PipelineOutput
	we.Get(context.Background(), &result)
	log.Printf("HTTP operations completed: Success=%d", result.TotalSuccess)
}

// Example 5: Loop Pattern (Argo withItems equivalent)
func runLoopPattern(c client.Client) {
	// Simulate Argo's withItems by programmatically creating containers
	environments := []string{"dev", "staging", "production"}

	wb := builder.NewWorkflowBuilder("deploy-loop")

	// Create a deployment container for each environment
	for _, env := range environments {
		wb.Add(template.NewContainer(fmt.Sprintf("deploy-%s", env), "alpine:latest",
			template.WithCommand("sh", "-c",
				fmt.Sprintf("echo 'Deploying to %s...' && sleep 1", env)),
			template.WithEnv("ENVIRONMENT", env),
			template.WithEnv("DEPLOY_TIME", time.Now().Format(time.RFC3339))))
	}

	input, err := wb.StopOnError(true).BuildPipeline()
	if err != nil {
		log.Printf("Failed to build loop workflow: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "deploy-loop-example",
			TaskQueue: "docker-tasks",
		},
		docker.ContainerPipelineWorkflow,
		*input,
	)

	var result docker.PipelineOutput
	we.Get(context.Background(), &result)
	log.Printf("Loop pattern completed: %d environments deployed", result.TotalSuccess)
}

// Example 6: Exit Handlers (Cleanup & Notifications)
func runExitHandlers(c client.Client) {
	// Create main workflow
	mainTask := template.NewContainer("main-task", "alpine:latest",
		template.WithCommand("sh", "-c", "echo 'Running main task...' && sleep 2"))

	// Create cleanup handler
	cleanup := template.NewBashScript("cleanup",
		`echo "Performing cleanup..."
		echo "Removing temporary files..."
		echo "Cleanup complete"`)

	// Create notification handler
	notify := template.NewBashScript("notify",
		`echo "Workflow completed"
		echo "Sending notification..."`)

	input, err := builder.NewWorkflowBuilder("exit-handler-demo").
		Add(mainTask).
		AddExitHandler(cleanup).
		AddExitHandler(notify).
		BuildPipeline()
	if err != nil {
		log.Printf("Failed to build exit handler workflow: %v", err)
		return
	}

	we, _ := c.ExecuteWorkflow(context.Background(),
		client.StartWorkflowOptions{
			ID:        "exit-handler-example",
			TaskQueue: "docker-tasks",
		},
		docker.ContainerPipelineWorkflow,
		*input,
	)

	var result docker.PipelineOutput
	we.Get(context.Background(), &result)
	log.Printf("Exit handler demo completed")
}
