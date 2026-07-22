package secrets

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolve(t *testing.T) {
	ctx := context.Background()
	t.Run("plain value passes through", func(t *testing.T) {
		v, err := Resolve(ctx, "hello")
		require.NoError(t, err)
		assert.Equal(t, "hello", v)
	})
	t.Run("secret ref resolves from default env resolver", func(t *testing.T) {
		t.Setenv("SECRET_PGPASS", "s3cr3t")
		v, err := Resolve(ctx, "secret://PGPASS")
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", v)
	})
	t.Run("missing ref errors", func(t *testing.T) {
		_, err := Resolve(ctx, "secret://NOPE_MISSING")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NOPE_MISSING")
	})
}

func TestResolveMap(t *testing.T) {
	t.Setenv("SECRET_TOKEN", "tok")
	in := map[string]string{
		"PLAIN": "x", "AUTH": "secret://TOKEN",
	}
	out, err := ResolveMap(context.Background(), in)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"PLAIN": "x", "AUTH": "tok"}, out)
	// input map not mutated
	assert.Equal(t, "secret://TOKEN", in["AUTH"])
}

func TestSetDefault(t *testing.T) {
	defer SetDefault(nil)
	SetDefault(ResolverFunc(func(_ context.Context, ref string) (string, error) {
		return "custom-" + ref, nil
	}))
	v, err := Resolve(context.Background(), "secret://A")
	require.NoError(t, err)
	assert.Equal(t, "custom-A", v)
}
