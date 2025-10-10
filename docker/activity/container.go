package activity

import (
	"context"
	"time"

	"github.com/jasoet/go-wf/docker"
	dockerpkg "github.com/jasoet/pkg/v2/docker"
	"go.temporal.io/sdk/activity"
)

// StartContainerActivity starts a container, waits for completion, and returns results.
//
//nolint:gocuclo,funlen // This function orchestrates container lifecycle which requires conditional logic and multiple steps
func StartContainerActivity(ctx context.Context, input docker.ContainerExecutionInput) (*docker.ContainerExecutionOutput, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting container", "image", input.Image, "name", input.Name)

	startTime := time.Now()

	// Build docker executor options
	opts := []dockerpkg.Option{
		dockerpkg.WithImage(input.Image),
		dockerpkg.WithAutoRemove(input.AutoRemove),
	}

	if input.Name != "" {
		opts = append(opts, dockerpkg.WithName(input.Name))
	}

	if len(input.Command) > 0 {
		opts = append(opts, dockerpkg.WithCmd(input.Command...))
	}

	if len(input.Entrypoint) > 0 {
		opts = append(opts, dockerpkg.WithEntrypoint(input.Entrypoint...))
	}

	if len(input.Env) > 0 {
		opts = append(opts, dockerpkg.WithEnvMap(input.Env))
	}

	if len(input.Ports) > 0 {
		for _, port := range input.Ports {
			opts = append(opts, dockerpkg.WithPorts(port))
		}
	}

	if len(input.Volumes) > 0 {
		opts = append(opts, dockerpkg.WithVolumes(input.Volumes))
	}

	if input.WorkDir != "" {
		opts = append(opts, dockerpkg.WithWorkDir(input.WorkDir))
	}

	if input.User != "" {
		opts = append(opts, dockerpkg.WithUser(input.User))
	}

	if len(input.Labels) > 0 {
		for k, v := range input.Labels {
			opts = append(opts, dockerpkg.WithLabel(k, v))
		}
	}

	// Add wait strategy if configured
	if input.WaitStrategy.Type != "" {
		waitStrategy := buildWaitStrategy(input.WaitStrategy)
		opts = append(opts, dockerpkg.WithWaitStrategy(waitStrategy))
	}

	// Create executor
	exec, err := dockerpkg.New(opts...)
	if err != nil {
		return &docker.ContainerExecutionOutput{
			Name:       input.Name,
			StartedAt:  startTime,
			FinishedAt: time.Now(),
			Success:    false,
			Error:      err.Error(),
		}, err
	}

	// Start container
	if err := exec.Start(ctx); err != nil {
		return &docker.ContainerExecutionOutput{
			Name:       input.Name,
			StartedAt:  startTime,
			FinishedAt: time.Now(),
			Success:    false,
			Error:      err.Error(),
		}, err
	}

	// Ensure cleanup
	defer func() {
		if err := exec.Terminate(ctx); err != nil {
			logger.Error("Failed to terminate container", "error", err)
		}
	}()

	// Get container info
	containerID := exec.ContainerID()
	logger.Info("Container started", "containerID", containerID)

	// Wait for completion
	exitCode, err := exec.Wait(ctx)
	finishTime := time.Now()

	// Collect logs
	stdout, stdoutErr := exec.GetStdout(ctx)
	if stdoutErr != nil {
		logger.Error("Failed to get stdout", "error", stdoutErr)
	}
	stderr, stderrErr := exec.GetStderr(ctx)
	if stderrErr != nil {
		logger.Error("Failed to get stderr", "error", stderrErr)
	}

	// Get endpoint if ports exposed
	var endpoint string
	var ports map[string]string
	if len(input.Ports) > 0 {
		var endpointErr error
		endpoint, endpointErr = exec.Endpoint(ctx, input.Ports[0])
		if endpointErr != nil {
			logger.Error("Failed to get endpoint", "error", endpointErr)
		}
		var portsErr error
		ports, portsErr = exec.GetAllPorts(ctx)
		if portsErr != nil {
			logger.Error("Failed to get ports", "error", portsErr)
		}
	}

	output := &docker.ContainerExecutionOutput{
		ContainerID: containerID,
		Name:        input.Name,
		ExitCode:    int(exitCode),
		Stdout:      stdout,
		Stderr:      stderr,
		Endpoint:    endpoint,
		Ports:       ports,
		StartedAt:   startTime,
		FinishedAt:  finishTime,
		Duration:    finishTime.Sub(startTime),
		Success:     exitCode == 0 && err == nil,
	}

	if err != nil {
		output.Error = err.Error()
		logger.Error("Container execution failed", "error", err, "exitCode", exitCode)
		return output, err
	}

	logger.Info("Container completed",
		"exitCode", exitCode,
		"duration", output.Duration)

	return output, nil
}

// buildWaitStrategy converts config to docker wait strategy.
func buildWaitStrategy(cfg docker.WaitStrategyConfig) dockerpkg.WaitStrategy {
	timeout := cfg.StartupTimeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	switch cfg.Type {
	case "log":
		return dockerpkg.WaitForLog(cfg.LogMessage).WithStartupTimeout(timeout)
	case "port":
		return dockerpkg.WaitForPort(cfg.Port)
	case "http":
		status := cfg.HTTPStatus
		if status == 0 {
			status = 200
		}
		return dockerpkg.WaitForHTTP(cfg.Port, cfg.HTTPPath, status)
	case "healthy":
		return dockerpkg.WaitForHealthy()
	default:
		return dockerpkg.WaitForLog("").WithStartupTimeout(timeout)
	}
}
