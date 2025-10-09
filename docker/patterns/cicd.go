package patterns

import (
	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/builder"
	"github.com/jasoet/go-wf/docker/template"
)

// BuildTestDeploy creates a simple CI/CD workflow with build, test, and deploy stages.
//
// Example:
//
//	input, err := patterns.BuildTestDeploy(
//	    "golang:1.25",
//	    "golang:1.25",
//	    "deployer:v1")
func BuildTestDeploy(buildImage, testImage, deployImage string) (*docker.PipelineInput, error) {
	// Build stage
	build := template.NewContainer("build", buildImage,
		template.WithCommand("sh", "-c", "echo 'Building...' && go build -o app"),
		template.WithWorkDir("/workspace"))

	// Test stage
	test := template.NewContainer("test", testImage,
		template.WithCommand("sh", "-c", "echo 'Testing...' && go test ./..."),
		template.WithWorkDir("/workspace"))

	// Deploy stage
	deploy := template.NewContainer("deploy", deployImage,
		template.WithCommand("sh", "-c", "echo 'Deploying...'"))

	return builder.NewWorkflowBuilder("ci-cd").
		Add(build).
		Add(test).
		Add(deploy).
		StopOnError(true).
		BuildPipeline()
}

// BuildTestDeployWithHealthCheck creates a CI/CD workflow with health check after deployment.
//
// Example:
//
//	input, err := patterns.BuildTestDeployWithHealthCheck(
//	    "golang:1.25",
//	    "deployer:v1",
//	    "https://myapp.com/health")
func BuildTestDeployWithHealthCheck(buildImage, deployImage, healthURL string) (*docker.PipelineInput, error) {
	build := template.NewContainer("build", buildImage,
		template.WithCommand("go", "build", "-o", "app"),
		template.WithWorkDir("/workspace"))

	test := template.NewContainer("test", buildImage,
		template.WithCommand("go", "test", "./..."),
		template.WithWorkDir("/workspace"))

	deploy := template.NewContainer("deploy", deployImage,
		template.WithCommand("deploy.sh"))

	healthCheck := template.NewHTTPHealthCheck("health-check", healthURL)

	return builder.NewWorkflowBuilder("ci-cd-health").
		Add(build).
		Add(test).
		Add(deploy).
		Add(healthCheck).
		StopOnError(true).
		BuildPipeline()
}

// BuildTestDeployWithNotification creates a CI/CD workflow with webhook notification.
//
// Example:
//
//	input, err := patterns.BuildTestDeployWithNotification(
//	    "golang:1.25",
//	    "deployer:v1",
//	    "https://hooks.slack.com/...",
//	    `{"text": "Deploy complete"}`)
func BuildTestDeployWithNotification(buildImage, deployImage, webhookURL, message string) (*docker.PipelineInput, error) {
	build := template.NewContainer("build", buildImage,
		template.WithCommand("go", "build", "-o", "app"))

	test := template.NewContainer("test", buildImage,
		template.WithCommand("go", "test", "./..."))

	deploy := template.NewContainer("deploy", deployImage,
		template.WithCommand("deploy.sh"))

	notify := template.NewHTTPWebhook("notify", webhookURL, message)

	wb := builder.NewWorkflowBuilder("ci-cd-notify").
		Add(build).
		Add(test).
		Add(deploy).
		StopOnError(true)

	// Add notification as exit handler
	wb.AddExitHandler(notify)

	return wb.BuildPipeline()
}

// MultiEnvironmentDeploy creates a workflow that deploys to multiple environments sequentially.
//
// Example:
//
//	input, err := patterns.MultiEnvironmentDeploy(
//	    "deployer:v1",
//	    []string{"staging", "production"})
func MultiEnvironmentDeploy(deployImage string, environments []string) (*docker.PipelineInput, error) {
	wb := builder.NewWorkflowBuilder("multi-env-deploy")

	for _, env := range environments {
		deploy := template.NewContainer("deploy-"+env, deployImage,
			template.WithCommand("deploy.sh"),
			template.WithEnv("ENVIRONMENT", env))

		wb.Add(deploy)
	}

	return wb.StopOnError(true).BuildPipeline()
}
