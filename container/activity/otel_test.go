package activity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	pkgotel "github.com/jasoet/pkg/v2/otel"

	"github.com/jasoet/go-wf/container/payload"
)

func TestInstrumentedStartContainerActivity_NilConfig(t *testing.T) {
	// When no OTel config is in context, the instrumented wrapper should
	// still call the underlying activity and return the same result.
	// We can't test real container execution in unit tests, but we can
	// verify the wrapper function signature and nil-config behavior.
	wrapped := InstrumentedStartContainerActivity(StartContainerActivity)
	assert.NotNil(t, wrapped)
}

func TestRecordContainerMetrics_NilConfig(t *testing.T) {
	// Verify metrics recording doesn't panic with nil config
	ctx := context.Background()
	assert.NotPanics(t, func() {
		recordContainerMetrics(ctx, "alpine", "success", 0, time.Second)
	})
}

func TestImageBaseName(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "image with tag",
			image:    "alpine:3.18",
			expected: "alpine",
		},
		{
			name:     "image without tag",
			image:    "alpine",
			expected: "alpine",
		},
		{
			name:     "image with registry and tag",
			image:    "docker.io/library/alpine:latest",
			expected: "docker.io/library/alpine",
		},
		{
			name:     "image with registry port and tag",
			image:    "registry.example.com:5000/myimage:v1",
			expected: "registry.example.com:5000/myimage",
		},
		{
			name:     "image with registry port no tag",
			image:    "registry.example.com:5000/myimage",
			expected: "registry.example.com:5000/myimage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imageBaseName(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestInstrumentedStartContainerActivity_PassThrough(t *testing.T) {
	// Verify that without OTel config, the inner function is called directly
	called := false
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "test-123",
		Success:     true,
	}

	inner := func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		called = true
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := context.Background() // no OTel config

	output, err := wrapped(ctx, payload.ContainerExecutionInput{
		Image: "alpine:latest",
	})

	assert.NoError(t, err)
	assert.True(t, called, "inner function should be called")
	assert.Equal(t, expectedOutput, output)
}

// otelContext creates a context with a fully configured OTel config (tracer + meter, no logging).
func otelContext() context.Context {
	cfg := pkgotel.NewConfig("test-service").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_Success(t *testing.T) {
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "abc-123",
		Success:     true,
		ExitCode:    0,
		Duration:    2 * time.Second,
		Endpoint:    "http://localhost:8080",
	}

	inner := func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()

	output, err := wrapped(ctx, payload.ContainerExecutionInput{
		Image:   "alpine:3.18",
		Command: []string{"echo", "hello"},
		WorkDir: "/app",
	})

	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_SuccessNoEndpoint(t *testing.T) {
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "def-456",
		Success:     true,
		ExitCode:    0,
		Duration:    time.Second,
		// Endpoint intentionally empty to test the if-Endpoint branch
	}

	inner := func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()

	output, err := wrapped(ctx, payload.ContainerExecutionInput{
		Image: "nginx:latest",
	})

	require.NoError(t, err)
	assert.Equal(t, expectedOutput, output)
	assert.Empty(t, output.Endpoint)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_Failure(t *testing.T) {
	expectedOutput := &payload.ContainerExecutionOutput{
		ContainerID: "ghi-789",
		Success:     false,
		ExitCode:    1,
		Duration:    500 * time.Millisecond,
		Error:       "process exited with code 1",
	}

	inner := func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return expectedOutput, nil
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()

	output, err := wrapped(ctx, payload.ContainerExecutionInput{
		Image: "myapp:v2",
	})

	require.NoError(t, err)
	assert.False(t, output.Success)
	assert.Equal(t, 1, output.ExitCode)
}

func TestInstrumentedStartContainerActivity_WithOTelConfig_Error(t *testing.T) {
	expectedErr := fmt.Errorf("container runtime error")

	inner := func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		return nil, expectedErr
	}

	wrapped := InstrumentedStartContainerActivity(inner)
	ctx := otelContext()

	output, err := wrapped(ctx, payload.ContainerExecutionInput{
		Image: "alpine:3.18",
	})

	require.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, output)
}

func TestRecordContainerMetrics_WithConfig(t *testing.T) {
	ctx := otelContext()
	assert.NotPanics(t, func() {
		recordContainerMetrics(ctx, "alpine:3.18", "success", 0, 2*time.Second)
	})
}

func TestImageBaseName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "empty string",
			image:    "",
			expected: "",
		},
		{
			name:     "latest tag only",
			image:    ":latest",
			expected: ":latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := imageBaseName(tt.image)
			assert.Equal(t, tt.expected, result)
		})
	}
}
