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
	registry.Register("greet", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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
	registry.Register("fail", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
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

func TestExecuteFunctionActivity_WithData(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("echo-data", func(_ context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
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
