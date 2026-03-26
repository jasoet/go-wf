package datasync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapperFunc(t *testing.T) {
	fn := MapperFunc[int, string](func(_ context.Context, records []int) ([]string, error) {
		result := make([]string, len(records))
		for i, r := range records {
			result[i] = fmt.Sprintf("%d", r)
		}
		return result, nil
	})

	result, err := fn.Map(context.Background(), []int{1, 2, 3})
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2", "3"}, result)
}

func TestIdentityMapper(t *testing.T) {
	mapper := IdentityMapper[string]()
	input := []string{"a", "b", "c"}
	result, err := mapper.Map(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}
