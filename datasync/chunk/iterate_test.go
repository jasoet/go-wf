package chunk

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIteratePartitions_ProcessesAllInOrder(t *testing.T) {
	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
	}
	var seen []Partition[int64]
	err := IteratePartitions[int64](context.Background(), parts, 0, nil,
		func(p Partition[int64]) error {
			seen = append(seen, p)
			return nil
		})
	require.NoError(t, err)
	assert.Equal(t, parts, seen)
}

func TestIteratePartitions_StopsOnError(t *testing.T) {
	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
	}
	boom := errors.New("boom")
	count := 0
	err := IteratePartitions[int64](context.Background(), parts, 0, nil,
		func(_ Partition[int64]) error {
			count++
			return boom
		})
	require.Error(t, err)
	assert.True(t, errors.Is(err, boom))
	assert.Equal(t, 1, count, "stopped after first error")
}

func TestIteratePartitions_SleepsBetweenButNotAfterLast(t *testing.T) {
	parts := []Partition[int64]{
		{Start: 0, End: 10},
		{Start: 10, End: 20},
		{Start: 20, End: 30},
	}
	sleeps := 0
	err := IteratePartitions[int64](context.Background(), parts, 5*time.Millisecond,
		func(_ context.Context, _ time.Duration) { sleeps++ },
		func(_ Partition[int64]) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, 2, sleeps, "len(parts)-1 sleeps")
}

func TestIteratePartitions_ZeroSleepSkipsCallback(t *testing.T) {
	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	sleeps := 0
	err := IteratePartitions[int64](context.Background(), parts, 0,
		func(_ context.Context, _ time.Duration) { sleeps++ },
		func(_ Partition[int64]) error { return nil })
	require.NoError(t, err)
	assert.Equal(t, 0, sleeps)
}

func TestIteratePartitions_ContextCancelDuringSleep(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	parts := []Partition[int64]{{Start: 0, End: 10}, {Start: 10, End: 20}}
	err := IteratePartitions[int64](ctx, parts, 100*time.Millisecond,
		func(_ context.Context, _ time.Duration) { cancel() },
		func(_ Partition[int64]) error { return nil })
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}
