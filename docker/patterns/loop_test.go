//go:build !integration

package patterns

import (
	"testing"

	"github.com/jasoet/go-wf/docker/template"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParallelLoop(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		image   string
		command string
		wantErr bool
	}{
		{
			name:    "valid parallel loop",
			items:   []string{"file1.csv", "file2.csv", "file3.csv"},
			image:   "processor:v1",
			command: "process.sh {{item}}",
			wantErr: false,
		},
		{
			name:    "single item",
			items:   []string{"file1.csv"},
			image:   "processor:v1",
			command: "process.sh {{item}}",
			wantErr: false,
		},
		{
			name:    "empty items",
			items:   []string{},
			image:   "processor:v1",
			command: "process.sh {{item}}",
			wantErr: true,
		},
		{
			name:    "nil items",
			items:   nil,
			image:   "processor:v1",
			command: "process.sh {{item}}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := ParallelLoop(tt.items, tt.image, tt.command)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.items, input.Items)
				assert.Equal(t, tt.image, input.Template.Image)
				assert.True(t, input.Parallel)
				assert.Equal(t, "continue", input.FailureStrategy)
				assert.Contains(t, input.Template.Env, "ITEM")
				assert.Contains(t, input.Template.Env, "INDEX")
			}
		})
	}
}

func TestSequentialLoop(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		image   string
		command string
		wantErr bool
	}{
		{
			name:    "valid sequential loop",
			items:   []string{"step1", "step2", "step3"},
			image:   "deployer:v1",
			command: "deploy.sh {{item}}",
			wantErr: false,
		},
		{
			name:    "single item",
			items:   []string{"step1"},
			image:   "deployer:v1",
			command: "deploy.sh {{item}}",
			wantErr: false,
		},
		{
			name:    "empty items",
			items:   []string{},
			image:   "deployer:v1",
			command: "deploy.sh {{item}}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := SequentialLoop(tt.items, tt.image, tt.command)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.items, input.Items)
				assert.Equal(t, tt.image, input.Template.Image)
				assert.False(t, input.Parallel)
				assert.Equal(t, "fail_fast", input.FailureStrategy)
			}
		})
	}
}

func TestBatchProcessing(t *testing.T) {
	tests := []struct {
		name           string
		dataFiles      []string
		processorImage string
		maxConcurrency int
		wantErr        bool
	}{
		{
			name:           "valid batch processing",
			dataFiles:      []string{"batch1.json", "batch2.json", "batch3.json"},
			processorImage: "data-processor:v1",
			maxConcurrency: 3,
			wantErr:        false,
		},
		{
			name:           "single batch",
			dataFiles:      []string{"batch1.json"},
			processorImage: "data-processor:v1",
			maxConcurrency: 1,
			wantErr:        false,
		},
		{
			name:           "empty data files",
			dataFiles:      []string{},
			processorImage: "data-processor:v1",
			maxConcurrency: 3,
			wantErr:        true,
		},
		{
			name:           "zero concurrency",
			dataFiles:      []string{"batch1.json"},
			processorImage: "data-processor:v1",
			maxConcurrency: 0,
			wantErr:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := BatchProcessing(tt.dataFiles, tt.processorImage, tt.maxConcurrency)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.dataFiles, input.Items)
				assert.Equal(t, tt.processorImage, input.Template.Image)
				assert.True(t, input.Parallel)
				assert.Equal(t, tt.maxConcurrency, input.MaxConcurrency)
				assert.Equal(t, "continue", input.FailureStrategy)
				assert.Contains(t, input.Template.Env, "INPUT_FILE")
				assert.Contains(t, input.Template.Env, "BATCH_INDEX")
			}
		})
	}
}

func TestMultiRegionDeployment(t *testing.T) {
	tests := []struct {
		name         string
		environments []string
		regions      []string
		deployImage  string
		wantErr      bool
	}{
		{
			name:         "valid multi-region deployment",
			environments: []string{"dev", "staging", "prod"},
			regions:      []string{"us-west", "us-east", "eu-central"},
			deployImage:  "deployer:v1",
			wantErr:      false,
		},
		{
			name:         "single environment and region",
			environments: []string{"prod"},
			regions:      []string{"us-west"},
			deployImage:  "deployer:v1",
			wantErr:      false,
		},
		{
			name:         "empty environments",
			environments: []string{},
			regions:      []string{"us-west"},
			deployImage:  "deployer:v1",
			wantErr:      true,
		},
		{
			name:         "empty regions",
			environments: []string{"prod"},
			regions:      []string{},
			deployImage:  "deployer:v1",
			wantErr:      true,
		},
		{
			name:         "both empty",
			environments: []string{},
			regions:      []string{},
			deployImage:  "deployer:v1",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := MultiRegionDeployment(tt.environments, tt.regions, tt.deployImage)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.environments, input.Parameters["env"])
				assert.Equal(t, tt.regions, input.Parameters["region"])
				assert.Equal(t, tt.deployImage, input.Template.Image)
				assert.True(t, input.Parallel)
				assert.Equal(t, "fail_fast", input.FailureStrategy)
				assert.Contains(t, input.Template.Env, "ENVIRONMENT")
				assert.Contains(t, input.Template.Env, "REGION")
			}
		})
	}
}

func TestMatrixBuild(t *testing.T) {
	tests := []struct {
		name        string
		buildMatrix map[string][]string
		buildImage  string
		wantErr     bool
	}{
		{
			name: "valid matrix build",
			buildMatrix: map[string][]string{
				"go_version": {"1.21", "1.22", "1.23"},
				"platform":   {"linux", "darwin", "windows"},
			},
			buildImage: "builder:v1",
			wantErr:    false,
		},
		{
			name: "single parameter",
			buildMatrix: map[string][]string{
				"version": {"1.0", "2.0"},
			},
			buildImage: "builder:v1",
			wantErr:    false,
		},
		{
			name:        "empty matrix",
			buildMatrix: map[string][]string{},
			buildImage:  "builder:v1",
			wantErr:     true,
		},
		{
			name:        "nil matrix",
			buildMatrix: nil,
			buildImage:  "builder:v1",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := MatrixBuild(tt.buildMatrix, tt.buildImage)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.buildMatrix, input.Parameters)
				assert.Equal(t, tt.buildImage, input.Template.Image)
				assert.True(t, input.Parallel)
				assert.Equal(t, "fail_fast", input.FailureStrategy)
				assert.Contains(t, input.Template.Env, "BUILD_INDEX")
				// Check that all matrix keys are in the environment
				for key := range tt.buildMatrix {
					assert.Contains(t, input.Template.Env, key)
				}
			}
		})
	}
}

func TestParameterSweep(t *testing.T) {
	tests := []struct {
		name           string
		parameters     map[string][]string
		trainerImage   string
		maxConcurrency int
		wantErr        bool
	}{
		{
			name: "valid parameter sweep",
			parameters: map[string][]string{
				"learning_rate": {"0.001", "0.01", "0.1"},
				"batch_size":    {"32", "64", "128"},
			},
			trainerImage:   "ml-trainer:v1",
			maxConcurrency: 5,
			wantErr:        false,
		},
		{
			name: "single parameter",
			parameters: map[string][]string{
				"learning_rate": {"0.001", "0.01"},
			},
			trainerImage:   "ml-trainer:v1",
			maxConcurrency: 2,
			wantErr:        false,
		},
		{
			name:           "empty parameters",
			parameters:     map[string][]string{},
			trainerImage:   "ml-trainer:v1",
			maxConcurrency: 5,
			wantErr:        true,
		},
		{
			name:           "nil parameters",
			parameters:     nil,
			trainerImage:   "ml-trainer:v1",
			maxConcurrency: 5,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := ParameterSweep(tt.parameters, tt.trainerImage, tt.maxConcurrency)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.parameters, input.Parameters)
				assert.Equal(t, tt.trainerImage, input.Template.Image)
				assert.True(t, input.Parallel)
				assert.Equal(t, tt.maxConcurrency, input.MaxConcurrency)
				assert.Equal(t, "continue", input.FailureStrategy)
				assert.Contains(t, input.Template.Env, "EXPERIMENT_INDEX")
				// Check that all parameters are in the environment
				for key := range tt.parameters {
					assert.Contains(t, input.Template.Env, key)
				}
			}
		})
	}
}

func TestParallelLoopWithTemplate(t *testing.T) {
	tests := []struct {
		name    string
		items   []string
		wantErr bool
	}{
		{
			name:    "valid loop with template",
			items:   []string{"a", "b", "c"},
			wantErr: false,
		},
		{
			name:    "single item",
			items:   []string{"x"},
			wantErr: false,
		},
		{
			name:    "empty items",
			items:   []string{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateSrc := template.NewContainer("process", "alpine:latest",
				template.WithCommand("echo", "Processing {{item}}"))

			input, err := ParallelLoopWithTemplate(tt.items, templateSrc)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, input)
			} else {
				require.NoError(t, err)
				require.NotNil(t, input)
				assert.Equal(t, tt.items, input.Items)
				assert.True(t, input.Parallel)
				assert.Equal(t, "continue", input.FailureStrategy)
			}
		})
	}
}
