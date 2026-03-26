package datasync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordMapper_Map(t *testing.T) {
	mapper := NewRecordMapper[int, string]("test", func(r *int) (string, error) {
		return fmt.Sprintf("val-%d", *r), nil
	})

	result, err := mapper.Map(context.Background(), []int{1, 2, 3})
	require.NoError(t, err)
	assert.Equal(t, []string{"val-1", "val-2", "val-3"}, result)
}

func TestRecordMapper_MapDetailed_WithSkips(t *testing.T) {
	mapper := NewRecordMapper[int, string]("test", func(r *int) (string, error) {
		if *r%2 == 0 {
			return "", fmt.Errorf("even numbers not allowed")
		}
		return fmt.Sprintf("val-%d", *r), nil
	})

	result := mapper.MapDetailed(context.Background(), []int{1, 2, 3, 4, 5})
	assert.Equal(t, []string{"val-1", "val-3", "val-5"}, result.Records)
	assert.Equal(t, 2, result.Skipped)
	assert.Len(t, result.SkipReasons, 2)
	assert.Contains(t, result.SkipReasons[0], "record 1")
	assert.Contains(t, result.SkipReasons[1], "record 3")
}

func TestRecordMapper_MapDetailed_AllSkipped(t *testing.T) {
	mapper := NewRecordMapper[int, string]("test", func(r *int) (string, error) {
		return "", fmt.Errorf("skip all")
	})

	result := mapper.MapDetailed(context.Background(), []int{1, 2})
	assert.Empty(t, result.Records)
	assert.Equal(t, 2, result.Skipped)
}

func TestRecordMapper_MapDetailed_EmptyInput(t *testing.T) {
	mapper := NewRecordMapper[int, string]("test", func(r *int) (string, error) {
		return "", nil
	})

	result := mapper.MapDetailed(context.Background(), []int{})
	assert.Empty(t, result.Records)
	assert.Equal(t, 0, result.Skipped)
}
