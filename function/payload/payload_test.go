package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFunctionExecutionInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   FunctionExecutionInput
		wantErr bool
	}{
		{
			name:    "valid input with name only",
			input:   FunctionExecutionInput{Name: "my-func"},
			wantErr: false,
		},
		{
			name: "valid input with all fields",
			input: FunctionExecutionInput{
				Name:    "my-func",
				Args:    map[string]string{"key": "value"},
				Data:    []byte("hello"),
				Env:     map[string]string{"FOO": "bar"},
				WorkDir: "/tmp",
				Labels:  map[string]string{"env": "test"},
			},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   FunctionExecutionInput{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFunctionExecutionInput_ActivityName(t *testing.T) {
	input := &FunctionExecutionInput{Name: "my-func"}
	assert.Equal(t, "ExecuteFunctionActivity", input.ActivityName())
}

func TestFunctionExecutionOutput_IsSuccess(t *testing.T) {
	assert.True(t, FunctionExecutionOutput{Success: true}.IsSuccess())
	assert.False(t, FunctionExecutionOutput{Success: false}.IsSuccess())
}

func TestFunctionExecutionOutput_GetError(t *testing.T) {
	assert.Equal(t, "fail", FunctionExecutionOutput{Error: "fail"}.GetError())
	assert.Equal(t, "", FunctionExecutionOutput{}.GetError())
}

func TestPipelineInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   PipelineInput
		wantErr bool
	}{
		{
			name: "valid pipeline",
			input: PipelineInput{
				Functions: []FunctionExecutionInput{
					{Name: "step1"},
					{Name: "step2"},
				},
				StopOnError: true,
			},
			wantErr: false,
		},
		{
			name: "invalid - empty functions",
			input: PipelineInput{
				Functions: []FunctionExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid - nil functions",
			input: PipelineInput{
				Functions: nil,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PipelineInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParallelInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParallelInput
		wantErr bool
	}{
		{
			name: "valid parallel with continue",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}, {Name: "b"}},
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name: "valid parallel with fail_fast",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}},
				FailureStrategy: "fail_fast",
			},
			wantErr: false,
		},
		{
			name: "valid parallel with empty strategy",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}},
				FailureStrategy: "",
			},
			wantErr: false,
		},
		{
			name: "invalid - empty functions",
			input: ParallelInput{
				Functions: []FunctionExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "invalid - bad failure strategy",
			input: ParallelInput{
				Functions:       []FunctionExecutionInput{{Name: "a"}},
				FailureStrategy: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParallelInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   LoopInput
		wantErr bool
	}{
		{
			name: "valid loop",
			input: LoopInput{
				Items:    []string{"a", "b"},
				Template: FunctionExecutionInput{Name: "process"},
			},
			wantErr: false,
		},
		{
			name: "invalid - empty items",
			input: LoopInput{
				Items:    []string{},
				Template: FunctionExecutionInput{Name: "process"},
			},
			wantErr: true,
		},
		{
			name: "invalid - missing template name",
			input: LoopInput{
				Items:    []string{"a"},
				Template: FunctionExecutionInput{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoopInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParameterizedLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   ParameterizedLoopInput
		wantErr bool
	}{
		{
			name: "valid parameterized loop",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{"os": {"linux", "darwin"}},
				Template:   FunctionExecutionInput{Name: "build"},
			},
			wantErr: false,
		},
		{
			name: "invalid - nil parameters",
			input: ParameterizedLoopInput{
				Template: FunctionExecutionInput{Name: "build"},
			},
			wantErr: true,
		},
		{
			name: "invalid - empty parameter array",
			input: ParameterizedLoopInput{
				Parameters: map[string][]string{"os": {}},
				Template:   FunctionExecutionInput{Name: "build"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParameterizedLoopInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
