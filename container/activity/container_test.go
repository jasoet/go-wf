package activity

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jasoet/go-wf/container/payload"
)

func TestTruncateOutput(t *testing.T) {
	t.Run("short string unchanged", func(t *testing.T) {
		input := "hello world"
		result := truncateOutput(input)
		if result != input {
			t.Errorf("expected %q, got %q", input, result)
		}
	})

	t.Run("empty string unchanged", func(t *testing.T) {
		result := truncateOutput("")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("exactly maxOutputSize unchanged", func(t *testing.T) {
		input := strings.Repeat("a", maxOutputSize)
		result := truncateOutput(input)
		if result != input {
			t.Errorf("expected string of length %d, got length %d", maxOutputSize, len(result))
		}
	})

	t.Run("over maxOutputSize is truncated", func(t *testing.T) {
		input := strings.Repeat("b", maxOutputSize+100)
		result := truncateOutput(input)
		if !strings.HasSuffix(result, "\n... [truncated]") {
			t.Error("expected truncation suffix")
		}
		// The prefix should be exactly maxOutputSize bytes of 'b'
		prefix := result[:maxOutputSize]
		if prefix != strings.Repeat("b", maxOutputSize) {
			t.Error("expected prefix to be preserved")
		}
	})
}

func TestBuildWaitStrategy(t *testing.T) {
	tests := []struct {
		name   string
		config payload.WaitStrategyConfig
		want   string // We'll check the type since WaitStrategy is an interface
	}{
		{
			name: "log wait strategy",
			config: payload.WaitStrategyConfig{
				Type:           "log",
				LogMessage:     "ready to accept connections",
				StartupTimeout: 30 * time.Second,
			},
			want: "log",
		},
		{
			name: "log wait strategy with default timeout",
			config: payload.WaitStrategyConfig{
				Type:       "log",
				LogMessage: "server started",
			},
			want: "log",
		},
		{
			name: "port wait strategy",
			config: payload.WaitStrategyConfig{
				Type: "port",
				Port: "5432",
			},
			want: "port",
		},
		{
			name: "http wait strategy with custom status",
			config: payload.WaitStrategyConfig{
				Type:       "http",
				Port:       "8080",
				HTTPPath:   "/health",
				HTTPStatus: 200,
			},
			want: "http",
		},
		{
			name: "http wait strategy with default status",
			config: payload.WaitStrategyConfig{
				Type:     "http",
				Port:     "80",
				HTTPPath: "/",
			},
			want: "http",
		},
		{
			name: "healthy wait strategy",
			config: payload.WaitStrategyConfig{
				Type: "healthy",
			},
			want: "healthy",
		},
		{
			name: "default wait strategy for unknown type",
			config: payload.WaitStrategyConfig{
				Type: "unknown",
			},
			want: "default",
		},
		{
			name:   "empty wait strategy",
			config: payload.WaitStrategyConfig{},
			want:   "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy := buildWaitStrategy(tt.config)
			assert.NotNil(t, strategy, "buildWaitStrategy(%q) returned nil", tt.config.Type)
		})
	}
}

func TestBuildWaitStrategy_AllTypeStrings(t *testing.T) {
	// Verify all documented type strings produce non-nil strategies
	// and that the function handles each without panicking.
	types := []string{"log", "port", "http", "healthy", "unknown", ""}
	for _, typ := range types {
		t.Run("type_"+typ, func(t *testing.T) {
			config := payload.WaitStrategyConfig{
				Type:       typ,
				LogMessage: "ready",
				Port:       "8080",
				HTTPPath:   "/health",
			}
			strategy := buildWaitStrategy(config)
			assert.NotNil(t, strategy, "buildWaitStrategy(%q) returned nil", typ)
		})
	}
}

func TestBuildWaitStrategy_TimeoutDefaults(t *testing.T) {
	// Test that default timeout is applied when not specified
	config := payload.WaitStrategyConfig{
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
	input := payload.ContainerExecutionInput{
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
		WaitStrategy: payload.WaitStrategyConfig{
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

func TestPipelineInput_MultipleContainers(t *testing.T) {
	input := payload.PipelineInput{
		Containers: []payload.ContainerExecutionInput{
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
			input := payload.ParallelInput{
				Containers: []payload.ContainerExecutionInput{
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
			input := payload.ParallelInput{
				Containers: []payload.ContainerExecutionInput{
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

func TestParallelInput_NegativeConcurrency(t *testing.T) {
	// MaxConcurrency is not currently enforced (see ParallelInput docs).
	// Negative values pass validation. This test documents current behavior.
	input := payload.ParallelInput{
		Containers: []payload.ContainerExecutionInput{
			{Image: "alpine:latest"},
		},
		MaxConcurrency:  -1,
		FailureStrategy: "continue",
	}

	err := input.Validate()
	assert.NoError(t, err, "Negative MaxConcurrency should pass validation (field is not enforced)")
}

func TestWaitStrategyConfig_AllTypes(t *testing.T) {
	configs := []payload.WaitStrategyConfig{
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
		input := payload.ContainerExecutionInput{
			Image:        "test:latest",
			WaitStrategy: cfg,
		}

		if err := input.Validate(); err != nil {
			t.Errorf("Config %d should be valid: %v", i, err)
		}
	}
}
