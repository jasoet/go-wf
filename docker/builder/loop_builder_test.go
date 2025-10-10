package builder

import (
	"testing"

	"github.com/jasoet/go-wf/docker/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoopBuilder(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  *LoopBuilder
	}{
		{
			name:  "with items",
			items: []string{"item1", "item2", "item3"},
			want: &LoopBuilder{
				items:    []string{"item1", "item2", "item3"},
				parallel: false,
				failFast: false,
			},
		},
		{
			name:  "with single item",
			items: []string{"single"},
			want: &LoopBuilder{
				items:    []string{"single"},
				parallel: false,
				failFast: false,
			},
		},
		{
			name:  "with empty items",
			items: []string{},
			want: &LoopBuilder{
				items:    []string{},
				parallel: false,
				failFast: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewLoopBuilder(tt.items)
			assert.Equal(t, tt.want.items, got.items)
			assert.Equal(t, tt.want.parallel, got.parallel)
			assert.Equal(t, tt.want.failFast, got.failFast)
		})
	}
}

func TestNewParameterizedLoopBuilder(t *testing.T) {
	tests := []struct {
		name       string
		parameters map[string][]string
	}{
		{
			name: "with multiple parameters",
			parameters: map[string][]string{
				"env":    {"dev", "staging", "prod"},
				"region": {"us-west", "us-east"},
			},
		},
		{
			name: "with single parameter",
			parameters: map[string][]string{
				"version": {"1.0", "2.0"},
			},
		},
		{
			name:       "with empty parameters",
			parameters: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewParameterizedLoopBuilder(tt.parameters)
			assert.Equal(t, tt.parameters, got.parameters)
			assert.False(t, got.parallel)
			assert.False(t, got.failFast)
		})
	}
}

func TestLoopBuilder_WithTemplate(t *testing.T) {
	builder := NewLoopBuilder([]string{"item1", "item2"})

	template := payload.ContainerExecutionInput{
		Image:   "processor:v1",
		Command: []string{"process", "{{item}}"},
		Env: map[string]string{
			"KEY": "value",
		},
	}

	result := builder.WithTemplate(template)

	assert.Equal(t, template.Image, result.template.Image)
	assert.Equal(t, template.Command, result.template.Command)
	assert.Equal(t, template.Env, result.template.Env)
	assert.Equal(t, builder, result, "should return same builder for chaining")
}

func TestLoopBuilder_WithSource(t *testing.T) {
	tests := []struct {
		name        string
		source      WorkflowSource
		expectError bool
	}{
		{
			name: "valid source",
			source: NewContainerSource(payload.ContainerExecutionInput{
				Image:   "alpine:latest",
				Command: []string{"echo", "test"},
			}),
			expectError: false,
		},
		{
			name:        "nil source",
			source:      nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := NewLoopBuilder([]string{"item1"})
			result := builder.WithSource(tt.source)

			if tt.expectError {
				assert.NotEmpty(t, result.errors)
			} else {
				assert.Empty(t, result.errors)
				assert.Equal(t, "alpine:latest", result.template.Image)
			}
		})
	}
}

func TestLoopBuilder_Parallel(t *testing.T) {
	builder := NewLoopBuilder([]string{"item1", "item2"})

	result := builder.Parallel(true)
	assert.True(t, result.parallel)
	assert.Equal(t, builder, result, "should return same builder for chaining")

	result = builder.Parallel(false)
	assert.False(t, result.parallel)
}

func TestLoopBuilder_MaxConcurrency(t *testing.T) {
	builder := NewLoopBuilder([]string{"item1", "item2"})

	result := builder.MaxConcurrency(5)
	assert.Equal(t, 5, result.maxConcurrency)
	assert.Equal(t, builder, result, "should return same builder for chaining")

	result = builder.MaxConcurrency(0)
	assert.Equal(t, 0, result.maxConcurrency)
}

func TestLoopBuilder_FailFast(t *testing.T) {
	builder := NewLoopBuilder([]string{"item1", "item2"})

	result := builder.FailFast(true)
	assert.True(t, result.failFast)
	assert.Equal(t, builder, result, "should return same builder for chaining")

	result = builder.FailFast(false)
	assert.False(t, result.failFast)
}

func TestLoopBuilder_BuildLoop(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *LoopBuilder
		expectError bool
		validate    func(t *testing.T, input *payload.LoopInput)
	}{
		{
			name: "valid loop",
			setupFunc: func() *LoopBuilder {
				return NewLoopBuilder([]string{"item1", "item2", "item3"}).
					WithTemplate(payload.ContainerExecutionInput{
						Image:   "processor:v1",
						Command: []string{"process", "{{item}}"},
					})
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.LoopInput) {
				assert.Len(t, input.Items, 3)
				assert.Equal(t, "processor:v1", input.Template.Image)
				assert.False(t, input.Parallel)
				assert.Equal(t, "continue", input.FailureStrategy)
			},
		},
		{
			name: "parallel loop with fail fast",
			setupFunc: func() *LoopBuilder {
				return NewLoopBuilder([]string{"item1", "item2"}).
					WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"}).
					Parallel(true).
					FailFast(true).
					MaxConcurrency(5)
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.LoopInput) {
				assert.True(t, input.Parallel)
				assert.Equal(t, "fail_fast", input.FailureStrategy)
				assert.Equal(t, 5, input.MaxConcurrency)
			},
		},
		{
			name: "empty items",
			setupFunc: func() *LoopBuilder {
				return NewLoopBuilder([]string{}).
					WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: true,
			validate:    nil,
		},
		{
			name: "nil items",
			setupFunc: func() *LoopBuilder {
				return NewLoopBuilder(nil).
					WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: true,
			validate:    nil,
		},
		{
			name: "error from nil source",
			setupFunc: func() *LoopBuilder {
				return NewLoopBuilder([]string{"item1"}).
					WithSource(nil)
			},
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.BuildLoop()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				if tt.validate != nil {
					tt.validate(t, input)
				}
			}
		})
	}
}

func TestLoopBuilder_BuildParameterizedLoop(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func() *LoopBuilder
		expectError bool
		validate    func(t *testing.T, input *payload.ParameterizedLoopInput)
	}{
		{
			name: "valid parameterized loop",
			setupFunc: func() *LoopBuilder {
				return NewParameterizedLoopBuilder(map[string][]string{
					"env":    {"dev", "staging", "prod"},
					"region": {"us-west", "us-east"},
				}).WithTemplate(payload.ContainerExecutionInput{
					Image:   "deployer:v1",
					Command: []string{"deploy", "--env={{.env}}", "--region={{.region}}"},
				})
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.ParameterizedLoopInput) {
				assert.Len(t, input.Parameters, 2)
				assert.Equal(t, "deployer:v1", input.Template.Image)
				assert.False(t, input.Parallel)
				assert.Equal(t, "continue", input.FailureStrategy)
			},
		},
		{
			name: "parallel with fail fast",
			setupFunc: func() *LoopBuilder {
				return NewParameterizedLoopBuilder(map[string][]string{
					"version": {"1.0", "2.0"},
				}).
					WithTemplate(payload.ContainerExecutionInput{Image: "builder:v1"}).
					Parallel(true).
					FailFast(true).
					MaxConcurrency(10)
			},
			expectError: false,
			validate: func(t *testing.T, input *payload.ParameterizedLoopInput) {
				assert.True(t, input.Parallel)
				assert.Equal(t, "fail_fast", input.FailureStrategy)
				assert.Equal(t, 10, input.MaxConcurrency)
			},
		},
		{
			name: "empty parameters",
			setupFunc: func() *LoopBuilder {
				return NewParameterizedLoopBuilder(map[string][]string{}).
					WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: true,
			validate:    nil,
		},
		{
			name: "nil parameters",
			setupFunc: func() *LoopBuilder {
				return NewParameterizedLoopBuilder(nil).
					WithTemplate(payload.ContainerExecutionInput{Image: "alpine:latest"})
			},
			expectError: true,
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := tt.setupFunc()
			input, err := builder.BuildParameterizedLoop()

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				if tt.validate != nil {
					tt.validate(t, input)
				}
			}
		})
	}
}

func TestForEach(t *testing.T) {
	items := []string{"file1.csv", "file2.csv", "file3.csv"}
	template := payload.ContainerExecutionInput{
		Image:   "processor:v1",
		Command: []string{"process", "{{item}}"},
	}

	builder := ForEach(items, template)

	require.NotNil(t, builder)
	assert.Equal(t, items, builder.items)
	assert.Equal(t, template.Image, builder.template.Image)
	assert.Equal(t, template.Command, builder.template.Command)
}

func TestForEachParam(t *testing.T) {
	params := map[string][]string{
		"env":    {"dev", "staging", "prod"},
		"region": {"us-west", "us-east"},
	}
	template := payload.ContainerExecutionInput{
		Image:   "deployer:v1",
		Command: []string{"deploy", "--env={{.env}}", "--region={{.region}}"},
	}

	builder := ForEachParam(params, template)

	require.NotNil(t, builder)
	assert.Equal(t, params, builder.parameters)
	assert.Equal(t, template.Image, builder.template.Image)
	assert.Equal(t, template.Command, builder.template.Command)
}

func TestLoopBuilder_ChainedCalls(t *testing.T) {
	// Test fluent API with chained calls
	input, err := NewLoopBuilder([]string{"batch1.json", "batch2.json", "batch3.json"}).
		WithTemplate(payload.ContainerExecutionInput{
			Image:   "data-processor:v1",
			Command: []string{"process", "--file={{item}}"},
		}).
		Parallel(true).
		MaxConcurrency(3).
		FailFast(false).
		BuildLoop()

	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Len(t, input.Items, 3)
	assert.True(t, input.Parallel)
	assert.Equal(t, 3, input.MaxConcurrency)
	assert.Equal(t, "continue", input.FailureStrategy)
}

func TestParameterizedLoopBuilder_ChainedCalls(t *testing.T) {
	// Test fluent API with chained calls
	input, err := NewParameterizedLoopBuilder(map[string][]string{
		"go_version": {"1.21", "1.22", "1.23"},
		"platform":   {"linux", "darwin", "windows"},
	}).
		WithTemplate(payload.ContainerExecutionInput{
			Image:   "builder:v1",
			Command: []string{"build", "--go={{.go_version}}", "--platform={{.platform}}"},
		}).
		Parallel(true).
		MaxConcurrency(6).
		FailFast(true).
		BuildParameterizedLoop()

	require.NoError(t, err)
	assert.NotNil(t, input)
	assert.Len(t, input.Parameters, 2)
	assert.True(t, input.Parallel)
	assert.Equal(t, 6, input.MaxConcurrency)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}
