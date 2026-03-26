package function

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	require.NotNil(t, r)
	assert.False(t, r.Has("nonexistent"))
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()

	handler := func(_ context.Context, input FunctionInput) (*FunctionOutput, error) {
		return &FunctionOutput{Result: map[string]string{"key": "value"}}, nil
	}

	err := r.Register("my-func", handler)
	require.NoError(t, err)
	assert.True(t, r.Has("my-func"))

	got, err := r.Get("my-func")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Call the handler to verify it works
	out, err := got(context.Background(), FunctionInput{})
	require.NoError(t, err)
	assert.Equal(t, "value", out.Result["key"])
}

func TestRegistry_RegisterDuplicate(t *testing.T) {
	r := NewRegistry()

	handler := func(_ context.Context, _ FunctionInput) (*FunctionOutput, error) {
		return &FunctionOutput{}, nil
	}

	err := r.Register("dup", handler)
	require.NoError(t, err)

	err = r.Register("dup", handler)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()

	_, err := r.Get("missing")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()

	_ = r.Register("exists", func(_ context.Context, _ FunctionInput) (*FunctionOutput, error) {
		return &FunctionOutput{}, nil
	})

	assert.True(t, r.Has("exists"))
	assert.False(t, r.Has("nope"))
}
