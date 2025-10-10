package workflow

import (
	"testing"

	"github.com/jasoet/go-wf/docker"
	"github.com/jasoet/go-wf/docker/activity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// TestLoopInput_Validate tests loop input validation.
func TestLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   docker.LoopInput
		wantErr bool
	}{
		{
			name: "valid loop input",
			input: docker.LoopInput{
				Items: []string{"item1", "item2", "item3"},
				Template: docker.ContainerExecutionInput{
					Image: "alpine:latest",
				},
				Parallel:        true,
				FailureStrategy: "continue",
			},
			wantErr: false,
		},
		{
			name: "empty items",
			input: docker.LoopInput{
				Items: []string{},
				Template: docker.ContainerExecutionInput{
					Image: "alpine:latest",
				},
			},
			wantErr: true,
		},
		{
			name: "nil items",
			input: docker.LoopInput{
				Template: docker.ContainerExecutionInput{
					Image: "alpine:latest",
				},
			},
			wantErr: true,
		},
		{
			name: "missing image in template",
			input: docker.LoopInput{
				Items:    []string{"item1"},
				Template: docker.ContainerExecutionInput{},
			},
			wantErr: true,
		},
		{
			name: "valid with max concurrency",
			input: docker.LoopInput{
				Items: []string{"item1", "item2"},
				Template: docker.ContainerExecutionInput{
					Image: "alpine:latest",
				},
				Parallel:       true,
				MaxConcurrency: 2,
			},
			wantErr: false,
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

// TestParameterizedLoopInput_Validate tests parameterized loop input validation.
func TestParameterizedLoopInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   docker.ParameterizedLoopInput
		wantErr bool
	}{
		{
			name: "valid parameterized loop",
			input: docker.ParameterizedLoopInput{
				Parameters: map[string][]string{
					"env":    {"dev", "prod"},
					"region": {"us-west", "us-east"},
				},
				Template: docker.ContainerExecutionInput{
					Image: "deployer:v1",
				},
			},
			wantErr: false,
		},
		{
			name: "empty parameters",
			input: docker.ParameterizedLoopInput{
				Parameters: map[string][]string{},
				Template: docker.ContainerExecutionInput{
					Image: "alpine:latest",
				},
			},
			wantErr: true,
		},
		{
			name: "parameter with empty array",
			input: docker.ParameterizedLoopInput{
				Parameters: map[string][]string{
					"env":    {"dev", "prod"},
					"region": {},
				},
				Template: docker.ContainerExecutionInput{
					Image: "deployer:v1",
				},
			},
			wantErr: true,
		},
		{
			name: "missing image in template",
			input: docker.ParameterizedLoopInput{
				Parameters: map[string][]string{
					"env": {"dev"},
				},
				Template: docker.ContainerExecutionInput{},
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

// TestSubstituteTemplate tests template substitution.
func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		item     string
		index    int
		params   map[string]string
		want     string
	}{
		{
			name:     "substitute item",
			template: "process {{item}}",
			item:     "file.csv",
			index:    0,
			params:   nil,
			want:     "process file.csv",
		},
		{
			name:     "substitute index",
			template: "task-{{index}}",
			item:     "",
			index:    5,
			params:   nil,
			want:     "task-5",
		},
		{
			name:     "substitute param with dot syntax",
			template: "deploy --env={{.env}}",
			item:     "",
			index:    0,
			params:   map[string]string{"env": "production"},
			want:     "deploy --env=production",
		},
		{
			name:     "substitute param without dot",
			template: "deploy --env={{env}}",
			item:     "",
			index:    0,
			params:   map[string]string{"env": "staging"},
			want:     "deploy --env=staging",
		},
		{
			name:     "substitute multiple",
			template: "process {{item}} index={{index}} env={{.env}}",
			item:     "data.json",
			index:    3,
			params:   map[string]string{"env": "dev"},
			want:     "process data.json index=3 env=dev",
		},
		{
			name:     "no substitution",
			template: "simple command",
			item:     "",
			index:    0,
			params:   nil,
			want:     "simple command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteTemplate(tt.template, tt.item, tt.index, tt.params)
			if got != tt.want {
				t.Errorf("substituteTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSubstituteContainerInput tests container input substitution.
func TestSubstituteContainerInput(t *testing.T) {
	tests := []struct {
		name     string
		template docker.ContainerExecutionInput
		item     string
		index    int
		params   map[string]string
		validate func(*testing.T, docker.ContainerExecutionInput)
	}{
		{
			name: "substitute in image",
			template: docker.ContainerExecutionInput{
				Image: "processor:{{item}}",
			},
			item:   "v1",
			index:  0,
			params: nil,
			validate: func(t *testing.T, result docker.ContainerExecutionInput) {
				if result.Image != "processor:v1" {
					t.Errorf("Image = %v, want processor:v1", result.Image)
				}
			},
		},
		{
			name: "substitute in command",
			template: docker.ContainerExecutionInput{
				Image:   "alpine:latest",
				Command: []string{"echo", "Processing {{item}} at index {{index}}"},
			},
			item:   "file.txt",
			index:  5,
			params: nil,
			validate: func(t *testing.T, result docker.ContainerExecutionInput) {
				if len(result.Command) != 2 {
					t.Errorf("Command length = %v, want 2", len(result.Command))
				}
				if result.Command[1] != "Processing file.txt at index 5" {
					t.Errorf("Command[1] = %v, want 'Processing file.txt at index 5'", result.Command[1])
				}
			},
		},
		{
			name: "substitute in env",
			template: docker.ContainerExecutionInput{
				Image: "alpine:latest",
				Env: map[string]string{
					"ITEM":  "{{item}}",
					"INDEX": "{{index}}",
					"ENV":   "{{.env}}",
				},
			},
			item:   "data.csv",
			index:  2,
			params: map[string]string{"env": "production"},
			validate: func(t *testing.T, result docker.ContainerExecutionInput) {
				if result.Env["ITEM"] != "data.csv" {
					t.Errorf("Env[ITEM] = %v, want data.csv", result.Env["ITEM"])
				}
				if result.Env["INDEX"] != "2" {
					t.Errorf("Env[INDEX] = %v, want 2", result.Env["INDEX"])
				}
				if result.Env["ENV"] != "production" {
					t.Errorf("Env[ENV] = %v, want production", result.Env["ENV"])
				}
			},
		},
		{
			name: "substitute in name",
			template: docker.ContainerExecutionInput{
				Image: "alpine:latest",
				Name:  "container-{{index}}",
			},
			item:   "",
			index:  10,
			params: nil,
			validate: func(t *testing.T, result docker.ContainerExecutionInput) {
				if result.Name != "container-10" {
					t.Errorf("Name = %v, want container-10", result.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substituteContainerInput(tt.template, tt.item, tt.index, tt.params)
			tt.validate(t, result)
		})
	}
}

// TestGenerateParameterCombinations tests parameter combination generation.
func TestGenerateParameterCombinations(t *testing.T) {
	tests := []struct {
		name   string
		params map[string][]string
		want   int
	}{
		{
			name:   "empty parameters",
			params: map[string][]string{},
			want:   0,
		},
		{
			name: "single parameter",
			params: map[string][]string{
				"env": {"dev", "prod"},
			},
			want: 2,
		},
		{
			name: "two parameters",
			params: map[string][]string{
				"env":    {"dev", "prod"},
				"region": {"us-west", "us-east"},
			},
			want: 4, // 2 * 2
		},
		{
			name: "three parameters",
			params: map[string][]string{
				"env":    {"dev", "staging", "prod"},
				"region": {"us-west", "us-east"},
				"tier":   {"free", "premium"},
			},
			want: 12, // 3 * 2 * 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateParameterCombinations(tt.params)
			if len(got) != tt.want {
				t.Errorf("generateParameterCombinations() returned %v combinations, want %v", len(got), tt.want)
			}

			// Validate that all combinations are unique
			seen := make(map[string]bool)
			for _, combo := range got {
				// Create a unique key from the combination
				key := ""
				for k, v := range combo {
					key += k + "=" + v + ";"
				}
				if seen[key] {
					t.Errorf("Duplicate combination found: %v", combo)
				}
				seen[key] = true

				// Validate that all parameter keys are present
				for paramKey := range tt.params {
					if _, ok := combo[paramKey]; !ok {
						t.Errorf("Missing parameter %v in combination %v", paramKey, combo)
					}
				}
			}
		})
	}
}

// TestGenerateParameterCombinations_Values tests actual combination values.
func TestGenerateParameterCombinations_Values(t *testing.T) {
	params := map[string][]string{
		"env":    {"dev", "prod"},
		"region": {"us-west", "us-east"},
	}

	combinations := generateParameterCombinations(params)

	// Expected combinations (order may vary):
	// {env:dev, region:us-west}
	// {env:dev, region:us-east}
	// {env:prod, region:us-west}
	// {env:prod, region:us-east}

	expected := []map[string]string{
		{"env": "dev", "region": "us-west"},
		{"env": "dev", "region": "us-east"},
		{"env": "prod", "region": "us-west"},
		{"env": "prod", "region": "us-east"},
	}

	if len(combinations) != len(expected) {
		t.Errorf("Expected %d combinations, got %d", len(expected), len(combinations))
	}

	// Check that all expected combinations are present
	for _, exp := range expected {
		found := false
		for _, combo := range combinations {
			if combo["env"] == exp["env"] && combo["region"] == exp["region"] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected combination not found: %v", exp)
		}
	}
}

// TestLoopWorkflow tests loop workflow execution.
func TestLoopWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the container execution activity
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{
			ContainerID: "test-container",
			ExitCode:    0,
			Success:     true,
		}, nil,
	)

	input := docker.LoopInput{
		Items: []string{"item1", "item2", "item3"},
		Template: docker.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"echo", "{{item}}"},
		},
		Parallel:        true,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result docker.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, result.ItemCount)
	assert.Equal(t, 3, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

// TestLoopWorkflow_Sequential tests sequential loop execution.
func TestLoopWorkflow_Sequential(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the container execution activity
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{
			ContainerID: "test-container",
			ExitCode:    0,
			Success:     true,
		}, nil,
	)

	input := docker.LoopInput{
		Items: []string{"step1", "step2"},
		Template: docker.ContainerExecutionInput{
			Image:   "alpine:latest",
			Command: []string{"echo", "{{item}}"},
		},
		Parallel:        false,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(LoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result docker.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
}

// TestParameterizedLoopWorkflow tests parameterized loop workflow.
func TestParameterizedLoopWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the container execution activity
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{
			ContainerID: "test-container",
			ExitCode:    0,
			Success:     true,
		}, nil,
	)

	input := docker.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"env":    {"dev", "prod"},
			"region": {"us-west", "us-east"},
		},
		Template: docker.ContainerExecutionInput{
			Image:   "deployer:v1",
			Command: []string{"deploy", "--env={{.env}}", "--region={{.region}}"},
		},
		Parallel:        true,
		FailureStrategy: "fail_fast",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result docker.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.ItemCount) // 2 envs * 2 regions = 4 combinations
	assert.Equal(t, 4, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

// TestParameterizedLoopWorkflow_Sequential tests sequential parameterized loop.
func TestParameterizedLoopWorkflow_Sequential(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock the container execution activity
	env.OnActivity(activity.StartContainerActivity, mock.Anything, mock.Anything).Return(
		&docker.ContainerExecutionOutput{
			ContainerID: "test-container",
			ExitCode:    0,
			Success:     true,
		}, nil,
	)

	input := docker.ParameterizedLoopInput{
		Parameters: map[string][]string{
			"version": {"1.0", "2.0"},
		},
		Template: docker.ContainerExecutionInput{
			Image:   "builder:v1",
			Command: []string{"build", "--version={{.version}}"},
		},
		Parallel:        false,
		FailureStrategy: "continue",
	}

	env.ExecuteWorkflow(ParameterizedLoopWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result docker.LoopOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.ItemCount)
	assert.Equal(t, 2, result.TotalSuccess)
}

// Benchmark tests
func BenchmarkSubstituteTemplate(b *testing.B) {
	template := "process {{item}} at index {{index}} in env {{.env}}"
	params := map[string]string{"env": "production"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = substituteTemplate(template, "file.csv", i, params)
	}
}

func BenchmarkGenerateParameterCombinations(b *testing.B) {
	params := map[string][]string{
		"env":    {"dev", "staging", "prod"},
		"region": {"us-west", "us-east", "eu-central"},
		"tier":   {"free", "premium"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateParameterCombinations(params)
	}
}

func BenchmarkSubstituteContainerInput(b *testing.B) {
	template := docker.ContainerExecutionInput{
		Image:   "processor:{{item}}",
		Command: []string{"process", "{{item}}", "--index={{index}}"},
		Env: map[string]string{
			"ITEM":  "{{item}}",
			"INDEX": "{{index}}",
			"ENV":   "{{.env}}",
		},
	}
	params := map[string]string{"env": "production"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = substituteContainerInput(template, "file.csv", i, params)
	}
}
