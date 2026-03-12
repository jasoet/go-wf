package activity

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

func TestInstrumentedExecuteFunctionActivity_ReturnsNonNil(t *testing.T) {
	inner := func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}
	wrapped := InstrumentedExecuteFunctionActivity(inner)
	assert.NotNil(t, wrapped)
}

func TestRecordFunctionMetrics_NilConfig(t *testing.T) {
	ctx := context.Background()
	assert.NotPanics(t, func() {
		recordFunctionMetrics(ctx, "test-fn", "success", time.Second)
	})
}

func TestInstrumentedExecuteFunctionActivity_PassThrough(t *testing.T) {
	// Set up a real registry with a handler
	registry := fn.NewRegistry()
	registry.Register("echo", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: map[string]string{"echoed": "true"},
		}, nil
	})

	// Create the real activity and wrap it
	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	// Call without OTel config — should pass through to inner
	ctx := context.Background()
	input := payload.FunctionExecutionInput{
		Name:    "echo",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	assert.NoError(t, err)
	assert.NotNil(t, output)
	assert.True(t, output.Success)
	assert.Equal(t, "echo", output.Name)
	assert.Equal(t, "true", output.Result["echoed"])
}
