package datasync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSource implements Source[T] for testing.
type mockSource[T any] struct {
	name    string
	records []T
	err     error
}

func (m *mockSource[T]) Name() string                         { return m.name }
func (m *mockSource[T]) Fetch(_ context.Context) ([]T, error) { return m.records, m.err }

// mockSink implements Sink[U] for testing.
type mockSink[U any] struct {
	name    string
	result  WriteResult
	err     error
	written []U
}

func (m *mockSink[U]) Name() string { return m.name }
func (m *mockSink[U]) Write(_ context.Context, records []U) (WriteResult, error) {
	m.written = records
	return m.result, m.err
}

func TestRunner_Run_Success(t *testing.T) {
	source := &mockSource[int]{name: "src", records: []int{1, 2, 3}}
	mapper := IdentityMapper[int]()
	sink := &mockSink[int]{name: "dst", result: WriteResult{Inserted: 3}}

	runner := NewRunner(source, mapper, sink)
	result, err := runner.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, result.TotalFetched)
	assert.Equal(t, 3, result.WriteResult.Inserted)
	assert.True(t, result.ProcessingTime > 0)
}

func TestRunner_Run_EmptySource(t *testing.T) {
	source := &mockSource[int]{name: "src", records: []int{}}
	mapper := IdentityMapper[int]()
	sink := &mockSink[int]{name: "dst"}

	runner := NewRunner(source, mapper, sink)
	result, err := runner.Run(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalFetched)
	assert.Equal(t, 0, result.WriteResult.Total())
}

func TestRunner_Run_FetchError(t *testing.T) {
	source := &mockSource[int]{name: "src", err: fmt.Errorf("fetch failed")}
	mapper := IdentityMapper[int]()
	sink := &mockSink[int]{name: "dst"}

	runner := NewRunner(source, mapper, sink)
	_, err := runner.Run(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "source src fetch failed")
}

func TestRunner_Run_MapError(t *testing.T) {
	source := &mockSource[int]{name: "src", records: []int{1}}
	mapper := MapperFunc[int, int](func(_ context.Context, _ []int) ([]int, error) {
		return nil, fmt.Errorf("map failed")
	})
	sink := &mockSink[int]{name: "dst"}

	runner := NewRunner(source, mapper, sink)
	_, err := runner.Run(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "mapper failed")
}

func TestRunner_Run_WriteError(t *testing.T) {
	source := &mockSource[int]{name: "src", records: []int{1}}
	mapper := IdentityMapper[int]()
	sink := &mockSink[int]{name: "dst", err: fmt.Errorf("write failed")}

	runner := NewRunner(source, mapper, sink)
	_, err := runner.Run(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sink dst write failed")
}
