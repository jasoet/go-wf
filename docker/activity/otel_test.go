package activity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/jasoet/go-wf/docker/payload"
)

func TestInstrumentedStartContainerActivity_NilConfig(t *testing.T) {
	// When no OTel config is in context, the instrumented wrapper should
	// still call the underlying activity and return the same result.
	// We can't test real container execution in unit tests, but we can
	// verify the wrapper function signature and nil-config behavior.
	wrapped := InstrumentedStartContainerActivity(StartContainerActivity)
	assert.NotNil(t, wrapped)
}

func TestRecordDockerMetrics_NilConfig(t *testing.T) {
	// Verify metrics recording doesn't panic with nil config
	ctx := context.Background()
	assert.NotPanics(t, func() {
		recordDockerMetrics(ctx, "alpine", "success", 0, time.Second)
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
