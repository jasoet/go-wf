package activity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

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

// otelContext creates a context with a minimal OTel config using real SDK providers.
func otelContext() context.Context {
	cfg := pkgotel.NewConfig("test-service").
		WithTracerProvider(sdktrace.NewTracerProvider()).
		WithMeterProvider(sdkmetric.NewMeterProvider()).
		WithoutLogging()
	return pkgotel.ContextWithConfig(context.Background(), cfg)
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_Success(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("echo", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: map[string]string{"echoed": "true"},
		}, nil
	})

	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "echo",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, output)
	assert.True(t, output.Success)
	assert.Equal(t, "echo", output.Name)
	assert.Equal(t, "true", output.Result["echoed"])
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_HandlerError(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("fail", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		return nil, fmt.Errorf("handler failed")
	})

	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "fail",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	// Handler errors are captured in output, not returned as activity errors
	require.NoError(t, err)
	require.NotNil(t, output)
	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "handler failed")
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_ActivityError(t *testing.T) {
	// Activity-level errors (validation, registry lookup) ARE returned as errors
	registry := fn.NewRegistry()
	activityFn := NewExecuteFunctionActivity(registry)
	wrapped := InstrumentedExecuteFunctionActivity(activityFn)

	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "missing",
		WorkDir: "/tmp",
	}

	output, err := wrapped(ctx, input)

	require.Error(t, err)
	require.NotNil(t, output)
	assert.False(t, output.Success)
}

func TestInstrumentedExecuteFunctionActivity_WithOTelConfig_NilOutput(t *testing.T) {
	// Test with an inner function that returns nil output (edge case)
	inner := func(_ context.Context, _ payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		return nil, nil
	}

	wrapped := InstrumentedExecuteFunctionActivity(inner)
	ctx := otelContext()
	input := payload.FunctionExecutionInput{
		Name:    "noop",
		WorkDir: "/tmp",
	}

	assert.NotPanics(t, func() {
		output, err := wrapped(ctx, input)
		assert.NoError(t, err)
		assert.Nil(t, output)
	})
}

func TestRecordFunctionMetrics_WithConfig(t *testing.T) {
	ctx := otelContext()
	assert.NotPanics(t, func() {
		recordFunctionMetrics(ctx, "test-fn", "success", 5*time.Second)
	})
	assert.NotPanics(t, func() {
		recordFunctionMetrics(ctx, "test-fn", "failure", time.Duration(0))
	})
}
