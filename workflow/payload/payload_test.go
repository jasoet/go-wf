package payload

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockInput struct {
	Name         string `json:"name" validate:"required"`
	Image        string `json:"image" validate:"required"`
	activityName string
}

func (m *mockInput) Validate() error {
	if m.Image == "" {
		return fmt.Errorf("image is required")
	}
	return nil
}

func (m *mockInput) ActivityName() string { return m.activityName }

type mockOutput struct {
	Success  bool   `json:"success"`
	ErrorMsg string `json:"error"`
}

func (m *mockOutput) IsSuccess() bool  { return m.Success }
func (m *mockOutput) GetError() string { return m.ErrorMsg }

func TestPipelineInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   PipelineInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid pipeline input",
			input: PipelineInput[*mockInput]{
				Tasks: []*mockInput{
					{Name: "task1", Image: "alpine", activityName: "run"},
					{Name: "task2", Image: "ubuntu", activityName: "build"},
				},
				StopOnError: true,
			},
			wantErr: false,
		},
		{
			name: "empty tasks",
			input: PipelineInput[*mockInput]{
				Tasks: []*mockInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid task in pipeline",
			input: PipelineInput[*mockInput]{
				Tasks: []*mockInput{
					{Name: "task1", Image: "", activityName: "run"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParallelInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParallelInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid parallel input",
			input: ParallelInput[*mockInput]{
				Tasks: []*mockInput{
					{Name: "task1", Image: "alpine", activityName: "run"},
				},
				MaxConcurrency:  2,
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name: "empty tasks",
			input: ParallelInput[*mockInput]{
				Tasks:           []*mockInput{},
				FailureStrategy: "continue",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoopInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   LoopInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid loop input",
			input: LoopInput[*mockInput]{
				Items:    []string{"item1", "item2"},
				Template: &mockInput{Name: "tpl", Image: "alpine", activityName: "run"},
			},
			wantErr: false,
		},
		{
			name: "empty items",
			input: LoopInput[*mockInput]{
				Items:    []string{},
				Template: &mockInput{Name: "tpl", Image: "alpine", activityName: "run"},
			},
			wantErr: true,
		},
		{
			name: "invalid template",
			input: LoopInput[*mockInput]{
				Items:    []string{"item1"},
				Template: &mockInput{Name: "tpl", Image: "", activityName: "run"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParameterizedLoopInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParameterizedLoopInput[*mockInput]
		wantErr bool
	}{
		{
			name: "valid parameterized loop input",
			input: ParameterizedLoopInput[*mockInput]{
				Parameters: map[string][]string{
					"env": {"dev", "staging"},
				},
				Template: &mockInput{Name: "tpl", Image: "alpine", activityName: "run"},
			},
			wantErr: false,
		},
		{
			name: "empty parameters",
			input: ParameterizedLoopInput[*mockInput]{
				Parameters: map[string][]string{},
				Template:   &mockInput{Name: "tpl", Image: "alpine", activityName: "run"},
			},
			wantErr: true,
		},
		{
			name: "empty parameter array",
			input: ParameterizedLoopInput[*mockInput]{
				Parameters: map[string][]string{
					"env": {},
				},
				Template: &mockInput{Name: "tpl", Image: "alpine", activityName: "run"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
