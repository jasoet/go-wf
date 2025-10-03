package docker

import (
	"testing"
	"time"
)

func TestContainerExecutionInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ContainerExecutionInput
		wantErr bool
	}{
		{
			name: "valid input with minimal fields",
			input: ContainerExecutionInput{
				Image: "nginx:alpine",
			},
			wantErr: false,
		},
		{
			name: "valid input with all fields",
			input: ContainerExecutionInput{
				Image:      "postgres:16-alpine",
				Command:    []string{"postgres"},
				Entrypoint: []string{"/usr/local/bin/docker-entrypoint.sh"},
				Env: map[string]string{
					"POSTGRES_PASSWORD": "test",
				},
				Ports:   []string{"5432:5432"},
				Volumes: map[string]string{"/data": "/var/lib/postgresql/data"},
				WorkDir: "/app",
				User:    "postgres",
				WaitStrategy: WaitStrategyConfig{
					Type:           "log",
					LogMessage:     "ready to accept connections",
					StartupTimeout: 30 * time.Second,
				},
				StartTimeout: 10 * time.Second,
				RunTimeout:   5 * time.Minute,
				AutoRemove:   true,
				Name:         "test-postgres",
				Labels:       map[string]string{"env": "test"},
			},
			wantErr: false,
		},
		{
			name: "invalid input - missing image",
			input: ContainerExecutionInput{
				Command: []string{"echo", "hello"},
			},
			wantErr: true,
		},
		{
			name: "valid with wait strategy port",
			input: ContainerExecutionInput{
				Image: "redis:alpine",
				WaitStrategy: WaitStrategyConfig{
					Type: "port",
					Port: "6379",
				},
			},
			wantErr: false,
		},
		{
			name: "valid with wait strategy http",
			input: ContainerExecutionInput{
				Image: "nginx:alpine",
				WaitStrategy: WaitStrategyConfig{
					Type:       "http",
					Port:       "80",
					HTTPPath:   "/health",
					HTTPStatus: 200,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ContainerExecutionInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPipelineInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   PipelineInput
		wantErr bool
	}{
		{
			name: "valid pipeline with two containers",
			input: PipelineInput{
				Containers: []ContainerExecutionInput{
					{Image: "alpine:latest", Command: []string{"echo", "step1"}},
					{Image: "alpine:latest", Command: []string{"echo", "step2"}},
				},
				StopOnError: true,
				Cleanup:     true,
			},
			wantErr: false,
		},
		{
			name: "invalid pipeline - empty containers",
			input: PipelineInput{
				Containers:  []ContainerExecutionInput{},
				StopOnError: true,
			},
			wantErr: true,
		},
		{
			name: "invalid pipeline - nil containers",
			input: PipelineInput{
				Containers:  nil,
				StopOnError: true,
			},
			wantErr: true,
		},
		{
			name: "valid pipeline with single container",
			input: PipelineInput{
				Containers: []ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
				StopOnError: false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PipelineInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParallelInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParallelInput
		wantErr bool
	}{
		{
			name: "valid parallel with continue strategy",
			input: ParallelInput{
				Containers: []ContainerExecutionInput{
					{Image: "alpine:latest"},
					{Image: "nginx:alpine"},
				},
				MaxConcurrency:  2,
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name: "valid parallel with fail_fast strategy",
			input: ParallelInput{
				Containers: []ContainerExecutionInput{
					{Image: "alpine:latest"},
					{Image: "nginx:alpine"},
				},
				MaxConcurrency:  0,
				FailureStrategy: "fail_fast",
			},
			wantErr: false,
		},
		{
			name: "valid parallel with empty strategy (default)",
			input: ParallelInput{
				Containers: []ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
				FailureStrategy: "",
			},
			wantErr: false,
		},
		{
			name: "invalid parallel - empty containers",
			input: ParallelInput{
				Containers:      []ContainerExecutionInput{},
				FailureStrategy: "continue",
			},
			wantErr: true,
		},
		{
			name: "invalid parallel - nil containers",
			input: ParallelInput{
				Containers:      nil,
				FailureStrategy: "continue",
			},
			wantErr: true,
		},
		{
			name: "invalid parallel - invalid failure strategy",
			input: ParallelInput{
				Containers: []ContainerExecutionInput{
					{Image: "alpine:latest"},
				},
				FailureStrategy: "invalid_strategy",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParallelInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWaitStrategyConfig_Validation(t *testing.T) {
	tests := []struct {
		name        string
		input       ContainerExecutionInput
		wantErr     bool
		description string
	}{
		{
			name: "valid empty wait strategy",
			input: ContainerExecutionInput{
				Image:        "alpine:latest",
				WaitStrategy: WaitStrategyConfig{},
			},
			wantErr:     false,
			description: "Empty wait strategy should be valid",
		},
		{
			name: "valid log wait strategy",
			input: ContainerExecutionInput{
				Image: "postgres:16",
				WaitStrategy: WaitStrategyConfig{
					Type:       "log",
					LogMessage: "ready",
				},
			},
			wantErr:     false,
			description: "Log wait strategy should be valid",
		},
		{
			name: "valid port wait strategy",
			input: ContainerExecutionInput{
				Image: "redis:alpine",
				WaitStrategy: WaitStrategyConfig{
					Type: "port",
					Port: "6379",
				},
			},
			wantErr:     false,
			description: "Port wait strategy should be valid",
		},
		{
			name: "valid http wait strategy",
			input: ContainerExecutionInput{
				Image: "nginx:alpine",
				WaitStrategy: WaitStrategyConfig{
					Type:     "http",
					Port:     "80",
					HTTPPath: "/",
				},
			},
			wantErr:     false,
			description: "HTTP wait strategy should be valid",
		},
		{
			name: "valid healthy wait strategy",
			input: ContainerExecutionInput{
				Image: "postgres:16",
				WaitStrategy: WaitStrategyConfig{
					Type: "healthy",
				},
			},
			wantErr:     false,
			description: "Healthy wait strategy should be valid",
		},
		{
			name: "invalid wait strategy type",
			input: ContainerExecutionInput{
				Image: "alpine:latest",
				WaitStrategy: WaitStrategyConfig{
					Type: "invalid",
				},
			},
			wantErr:     true,
			description: "Invalid wait strategy type should fail validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("WaitStrategyConfig validation: %s\nerror = %v, wantErr %v",
					tt.description, err, tt.wantErr)
			}
		})
	}
}
