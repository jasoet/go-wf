package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncExecutionInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   SyncExecutionInput
		wantErr bool
	}{
		{
			name:    "valid input",
			input:   SyncExecutionInput{JobName: "test-job", SourceName: "src", SinkName: "dst"},
			wantErr: false,
		},
		{
			name:    "missing job name",
			input:   SyncExecutionInput{SourceName: "src", SinkName: "dst"},
			wantErr: true,
		},
		{
			name:    "missing source name",
			input:   SyncExecutionInput{JobName: "test-job", SinkName: "dst"},
			wantErr: true,
		},
		{
			name:    "missing sink name",
			input:   SyncExecutionInput{JobName: "test-job", SourceName: "src"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSyncExecutionInput_ActivityName(t *testing.T) {
	input := &SyncExecutionInput{JobName: "attendee-sync"}
	assert.Equal(t, "attendee-sync.SyncData", input.ActivityName())
}

func TestSyncExecutionOutput_IsSuccess(t *testing.T) {
	assert.True(t, SyncExecutionOutput{Success: true}.IsSuccess())
	assert.False(t, SyncExecutionOutput{Success: false}.IsSuccess())
}

func TestSyncExecutionOutput_GetError(t *testing.T) {
	assert.Equal(t, "", SyncExecutionOutput{}.GetError())
	assert.Equal(t, "something failed", SyncExecutionOutput{Error: "something failed"}.GetError())
}
