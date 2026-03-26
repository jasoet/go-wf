package datasync

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteResult_Add(t *testing.T) {
	tests := []struct {
		name     string
		base     WriteResult
		other    WriteResult
		expected WriteResult
	}{
		{
			name:     "add to zero",
			base:     WriteResult{},
			other:    WriteResult{Inserted: 5, Updated: 3, Skipped: 2},
			expected: WriteResult{Inserted: 5, Updated: 3, Skipped: 2},
		},
		{
			name:     "add to existing",
			base:     WriteResult{Inserted: 1, Updated: 1, Skipped: 1},
			other:    WriteResult{Inserted: 2, Updated: 3, Skipped: 4},
			expected: WriteResult{Inserted: 3, Updated: 4, Skipped: 5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.base.Add(tt.other)
			assert.Equal(t, tt.expected, tt.base)
		})
	}
}

func TestWriteResult_Total(t *testing.T) {
	tests := []struct {
		name     string
		result   WriteResult
		expected int
	}{
		{"zero", WriteResult{}, 0},
		{"all fields", WriteResult{Inserted: 5, Updated: 3, Skipped: 2}, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Total())
		})
	}
}
