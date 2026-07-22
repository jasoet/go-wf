package activity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

func TestExecuteFunctionActivity_Success(t *testing.T) {
	registry := fn.NewRegistry()
	_ = registry.Register("greet", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		name := input.Args["name"]
		return &fn.FunctionOutput{
			Result: map[string]string{"greeting": "hello " + name},
		}, nil
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{
		Name: "greet",
		Args: map[string]string{"name": "world"},
	}

	output, err := activity(context.Background(), input)
	require.NoError(t, err)
	require.NotNil(t, output)

	assert.True(t, output.Success)
	assert.Equal(t, "greet", output.Name)
	assert.Equal(t, "hello world", output.Result["greeting"])
	assert.NotZero(t, output.Duration)
	assert.NotZero(t, output.StartedAt)
	assert.NotZero(t, output.FinishedAt)
	assert.Empty(t, output.Error)
}

func TestExecuteFunctionActivity_HandlerError(t *testing.T) {
	registry := fn.NewRegistry()
	_ = registry.Register("fail", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		return nil, fmt.Errorf("something went wrong")
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{Name: "fail"}

	output, err := activity(context.Background(), input)
	require.NoError(t, err) // Activity itself succeeds, but output captures the error
	require.NotNil(t, output)

	assert.False(t, output.Success)
	assert.Equal(t, "fail", output.Name)
	assert.Contains(t, output.Error, "something went wrong")
}

func TestExecuteFunctionActivity_NotFound(t *testing.T) {
	registry := fn.NewRegistry()

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{Name: "missing"}

	output, err := activity(context.Background(), input)
	require.Error(t, err)
	require.NotNil(t, output)

	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "missing")
}

func TestExecuteFunctionActivity_ValidationError(t *testing.T) {
	registry := fn.NewRegistry()

	activity := NewExecuteFunctionActivity(registry)

	// Missing required Name field
	input := payload.FunctionExecutionInput{}

	output, err := activity(context.Background(), input)
	require.Error(t, err)
	require.NotNil(t, output)

	assert.False(t, output.Success)
}

func TestExecuteFunctionActivity_PanicRecovery(t *testing.T) {
	registry := fn.NewRegistry()
	_ = registry.Register("panic-handler", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		panic("unexpected nil pointer")
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{Name: "panic-handler"}

	// Should NOT panic — should return graceful error
	output, err := activity(context.Background(), input)
	require.NoError(t, err) // Activity returns nil error (business logic failure)
	require.NotNil(t, output)

	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "panic")
	assert.Contains(t, output.Error, "unexpected nil pointer")
	assert.NotZero(t, output.Duration)
}

func TestExecuteFunctionActivity_WithData(t *testing.T) {
	registry := fn.NewRegistry()
	_ = registry.Register("echo-data", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Data: input.Data,
		}, nil
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{
		Name: "echo-data",
		Data: []byte("raw bytes"),
	}

	output, err := activity(context.Background(), input)
	require.NoError(t, err)

	assert.True(t, output.Success)
	assert.Equal(t, []byte("raw bytes"), output.Data)
}

func TestExecuteFunctionActivity_SecretRefEnv(t *testing.T) {
	t.Setenv("SECRET_API_KEY", "resolved-key")
	registry := fn.NewRegistry()
	_ = registry.Register("env-reader", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{
			Result: map[string]string{"key": input.Env["API_KEY"], "plain": input.Env["PLAIN"]},
		}, nil
	})

	activity := NewExecuteFunctionActivity(registry)

	output, err := activity(context.Background(), payload.FunctionExecutionInput{
		Name: "env-reader",
		Env:  map[string]string{"API_KEY": "secret://API_KEY", "PLAIN": "x"},
	})
	require.NoError(t, err)
	assert.True(t, output.Success)
	assert.Equal(t, "resolved-key", output.Result["key"])
	assert.Equal(t, "x", output.Result["plain"])
}

func TestExecuteFunctionActivity_SecretRefMissing(t *testing.T) {
	registry := fn.NewRegistry()
	_ = registry.Register("noop", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		return &fn.FunctionOutput{}, nil
	})

	activity := NewExecuteFunctionActivity(registry)

	output, err := activity(context.Background(), payload.FunctionExecutionInput{
		Name: "noop",
		Env:  map[string]string{"MISSING": "secret://DEFINITELY_NOT_SET"},
	})
	require.Error(t, err)
	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "DEFINITELY_NOT_SET")
}
