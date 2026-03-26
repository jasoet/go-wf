package datasync

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testRecord struct {
	ID   string
	Name string
}

func TestInsertIfAbsentSink_Name(t *testing.T) {
	sink := NewInsertIfAbsentSink[testRecord, string](
		"test-sink",
		func(r *testRecord) string { return r.ID },
		func(_ context.Context, _ string) (*testRecord, error) { return nil, nil },
		func(_ context.Context, _ *testRecord) error { return nil },
	)
	assert.Equal(t, "test-sink", sink.Name())
}

func TestInsertIfAbsentSink_Write_AllNew(t *testing.T) {
	var created []string
	sink := NewInsertIfAbsentSink[testRecord, string](
		"test-sink",
		func(r *testRecord) string { return r.ID },
		func(_ context.Context, _ string) (*testRecord, error) { return nil, nil },
		func(_ context.Context, r *testRecord) error {
			created = append(created, r.ID)
			return nil
		},
	)

	records := []testRecord{{ID: "1", Name: "a"}, {ID: "2", Name: "b"}}
	result, err := sink.Write(context.Background(), records)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Inserted)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, []string{"1", "2"}, created)
}

func TestInsertIfAbsentSink_Write_AllExisting(t *testing.T) {
	sink := NewInsertIfAbsentSink[testRecord, string](
		"test-sink",
		func(r *testRecord) string { return r.ID },
		func(_ context.Context, id string) (*testRecord, error) {
			return &testRecord{ID: id}, nil
		},
		func(_ context.Context, _ *testRecord) error {
			t.Fatal("create should not be called")
			return nil
		},
	)

	records := []testRecord{{ID: "1"}, {ID: "2"}}
	result, err := sink.Write(context.Background(), records)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Inserted)
	assert.Equal(t, 2, result.Skipped)
}

func TestInsertIfAbsentSink_Write_Mixed(t *testing.T) {
	existing := map[string]bool{"2": true}
	sink := NewInsertIfAbsentSink[testRecord, string](
		"test-sink",
		func(r *testRecord) string { return r.ID },
		func(_ context.Context, id string) (*testRecord, error) {
			if existing[id] {
				return &testRecord{ID: id}, nil
			}
			return nil, nil
		},
		func(_ context.Context, _ *testRecord) error { return nil },
	)

	records := []testRecord{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	result, err := sink.Write(context.Background(), records)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Inserted)
	assert.Equal(t, 1, result.Skipped)
}

func TestInsertIfAbsentSink_Write_FindError(t *testing.T) {
	sink := NewInsertIfAbsentSink[testRecord, string](
		"test-sink",
		func(r *testRecord) string { return r.ID },
		func(_ context.Context, _ string) (*testRecord, error) {
			return nil, fmt.Errorf("db error")
		},
		func(_ context.Context, _ *testRecord) error { return nil },
	)

	_, err := sink.Write(context.Background(), []testRecord{{ID: "1"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "find record")
}

func TestInsertIfAbsentSink_Write_CreateError(t *testing.T) {
	sink := NewInsertIfAbsentSink[testRecord, string](
		"test-sink",
		func(r *testRecord) string { return r.ID },
		func(_ context.Context, _ string) (*testRecord, error) { return nil, nil },
		func(_ context.Context, _ *testRecord) error { return fmt.Errorf("create failed") },
	)

	_, err := sink.Write(context.Background(), []testRecord{{ID: "1"}})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create record")
}
