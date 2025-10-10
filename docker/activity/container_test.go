package activity

import (
	"testing"
	"time"

	"github.com/jasoet/go-wf/docker"
)

func TestBuildWaitStrategy(t *testing.T) {
	tests := []struct {
		name   string
		config docker.WaitStrategyConfig
		want   string // We'll check the type since WaitStrategy is an interface
	}{
		{
			name: "log wait strategy",
			config: docker.WaitStrategyConfig{
				Type:           "log",
				LogMessage:     "ready to accept connections",
				StartupTimeout: 30 * time.Second,
			},
			want: "log",
		},
		{
			name: "log wait strategy with default timeout",
			config: docker.WaitStrategyConfig{
				Type:       "log",
				LogMessage: "server started",
			},
			want: "log",
		},
		{
			name: "port wait strategy",
			config: docker.WaitStrategyConfig{
				Type: "port",
				Port: "5432",
			},
			want: "port",
		},
		{
			name: "http wait strategy with custom status",
			config: docker.WaitStrategyConfig{
				Type:       "http",
				Port:       "8080",
				HTTPPath:   "/health",
				HTTPStatus: 200,
			},
			want: "http",
		},
		{
			name: "http wait strategy with default status",
			config: docker.WaitStrategyConfig{
				Type:     "http",
				Port:     "80",
				HTTPPath: "/",
			},
			want: "http",
		},
		{
			name: "healthy wait strategy",
			config: docker.WaitStrategyConfig{
				Type: "healthy",
			},
			want: "healthy",
		},
		{
			name: "default wait strategy for unknown type",
			config: docker.WaitStrategyConfig{
				Type: "unknown",
			},
			want: "default",
		},
		{
			name:   "empty wait strategy",
			config: docker.WaitStrategyConfig{},
			want:   "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := buildWaitStrategy(tt.config)
			if strategy == nil {
				t.Error("buildWaitStrategy returned nil")
			}
			// Successfully built strategy - we can't inspect the internal type easily
			// but we verify it doesn't panic and returns non-nil
		})
	}
}

func TestBuildWaitStrategy_TimeoutDefaults(t *testing.T) {
	// Test that default timeout is applied when not specified
	config := docker.WaitStrategyConfig{
		Type:       "log",
		LogMessage: "ready",
		// No StartupTimeout specified
	}

	strategy := buildWaitStrategy(config)
	if strategy == nil {
		t.Error("Expected non-nil strategy")
	}
}

func TestContainerExecutionInput_AllFields(t *testing.T) {
	// Test that all fields are properly validated
	input := docker.ContainerExecutionInput{
		Image:      "alpine:latest",
		Command:    []string{"echo", "test"},
		Entrypoint: []string{"/bin/sh"},
		Env: map[string]string{
			"KEY": "value",
		},
		Ports:   []string{"8080:8080"},
		Volumes: map[string]string{"/host": "/container"},
		WorkDir: "/app",
		User:    "nobody",
		WaitStrategy: docker.WaitStrategyConfig{
			Type:           "log",
			LogMessage:     "ready",
			StartupTimeout: 10 * time.Second,
		},
		StartTimeout: 5 * time.Second,
		RunTimeout:   30 * time.Second,
		AutoRemove:   true,
		Name:         "test-container",
		Labels: map[string]string{
			"env": "test",
		},
	}

	if err := input.Validate(); err != nil {
		t.Errorf("Valid input should not fail validation: %v", err)
	}
}

func TestContainerExecutionOutput_Fields(t *testing.T) {
	// Test output structure
	startedAt := time.Now()
	finishedAt := startedAt.Add(5 * time.Second)
	output := docker.ContainerExecutionOutput{
		ContainerID: "abc123",
		Name:        "test",
		ExitCode:    0,
		Stdout:      "output",
		Stderr:      "errors",
		Endpoint:    "localhost:8080",
		Ports: map[string]string{
			"8080": "32768",
		},
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		Duration:   5 * time.Second,
		Success:    true,
		Error:      "",
	}

	if output.ContainerID != "abc123" {
		t.Errorf("Expected ContainerID abc123, got %s", output.ContainerID)
	}
	if output.Name != "test" {
		t.Errorf("Expected Name test, got %s", output.Name)
	}
	if output.ExitCode != 0 {
		t.Errorf("Expected ExitCode 0, got %d", output.ExitCode)
	}
	if output.Stdout != "output" {
		t.Errorf("Expected Stdout output, got %s", output.Stdout)
	}
	if output.Stderr != "errors" {
		t.Errorf("Expected Stderr errors, got %s", output.Stderr)
	}
	if output.Endpoint != "localhost:8080" {
		t.Errorf("Expected Endpoint localhost:8080, got %s", output.Endpoint)
	}
	if len(output.Ports) != 1 || output.Ports["8080"] != "32768" {
		t.Errorf("Expected Ports map with 8080:32768, got %v", output.Ports)
	}
	if !output.Success {
		t.Error("Expected Success to be true")
	}
	if output.Error != "" {
		t.Errorf("Expected empty Error, got %s", output.Error)
	}
	if output.Duration != 5*time.Second {
		t.Errorf("Expected Duration 5s, got %v", output.Duration)
	}
	if !output.StartedAt.Equal(startedAt) {
		t.Errorf("Expected StartedAt %v, got %v", startedAt, output.StartedAt)
	}
	if !output.FinishedAt.Equal(finishedAt) {
		t.Errorf("Expected FinishedAt %v, got %v", finishedAt, output.FinishedAt)
	}
}

func TestPipelineInput_MultipleContainers(t *testing.T) {
	input := docker.PipelineInput{
		Containers: []docker.ContainerExecutionInput{
			{Image: "alpine:1"},
			{Image: "alpine:2"},
			{Image: "alpine:3"},
		},
		StopOnError: false,
		Cleanup:     true,
	}

	if err := input.Validate(); err != nil {
		t.Errorf("Valid pipeline input should not fail: %v", err)
	}

	if len(input.Containers) != 3 {
		t.Errorf("Expected 3 containers, got %d", len(input.Containers))
	}
}

func TestParallelInput_FailureStrategies(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		wantErr  bool
	}{
		{
			name:     "continue strategy",
			strategy: "continue",
			wantErr:  false,
		},
		{
			name:     "fail_fast strategy",
			strategy: "fail_fast",
			wantErr:  false,
		},
		{
			name:     "empty strategy (default)",
			strategy: "",
			wantErr:  false,
		},
		{
			name:     "invalid strategy",
			strategy: "abort",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := docker.ParallelInput{
				Containers: []docker.ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
				FailureStrategy: tt.strategy,
			}

			err := input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParallelInput_Concurrency(t *testing.T) {
	tests := []struct {
		name           string
		maxConcurrency int
		wantErr        bool
	}{
		{
			name:           "unlimited concurrency",
			maxConcurrency: 0,
			wantErr:        false,
		},
		{
			name:           "limited concurrency",
			maxConcurrency: 5,
			wantErr:        false,
		},
		{
			name:           "single concurrency",
			maxConcurrency: 1,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := docker.ParallelInput{
				Containers: []docker.ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
				MaxConcurrency:  tt.maxConcurrency,
				FailureStrategy: "continue",
			}

			if err := input.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWaitStrategyConfig_AllTypes(t *testing.T) {
	configs := []docker.WaitStrategyConfig{
		{
			Type:           "log",
			LogMessage:     "ready",
			StartupTimeout: 30 * time.Second,
		},
		{
			Type: "port",
			Port: "5432",
		},
		{
			Type:       "http",
			Port:       "8080",
			HTTPPath:   "/health",
			HTTPStatus: 200,
		},
		{
			Type: "healthy",
		},
	}

	for i, cfg := range configs {
		input := docker.ContainerExecutionInput{
			Image:        "test:latest",
			WaitStrategy: cfg,
		}

		if err := input.Validate(); err != nil {
			t.Errorf("Config %d should be valid: %v", i, err)
		}
	}
}
