package payload

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGWorkflowInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   DAGWorkflowInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid DAG with single node",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "build",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid DAG with dependencies",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "build",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
					{
						Name: "test",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
						Dependencies: []string{"build"},
					},
				},
				FailFast: true,
			},
			wantErr: false,
		},
		{
			name:    "invalid - empty nodes",
			input:   DAGWorkflowInput{},
			wantErr: true,
			errMsg:  "at least one node is required",
		},
		{
			name: "invalid - node name with spaces",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "bad name",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid node name",
		},
		{
			name: "invalid - node name starts with digit",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "1build",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid node name",
		},
		{
			name: "invalid - empty node name",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid node name",
		},
		{
			name: "valid - node name with hyphens and underscores",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "build-step_1",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid - dependency not found",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{
						Name: "test",
						Container: ExtendedContainerInput{
							ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
						},
						Dependencies: []string{"non-existent"},
					},
				},
			},
			wantErr: true,
			errMsg:  "dependency node not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("DAGWorkflowInput.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errMsg != "" && err != nil {
				assert.Contains(t, err.Error(), tt.errMsg)
			}
		})
	}
}

func TestDAGWorkflowInput_CycleDetection(t *testing.T) {
	tests := []struct {
		name    string
		input   DAGWorkflowInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "direct cycle A->B->A",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"B"}},
					{Name: "B", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "self-referencing node",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "indirect cycle A->B->C->A",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"C"}},
					{Name: "B", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
					{Name: "C", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"B"}},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "duplicate node names",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "build", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}},
					{Name: "build", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}},
				},
			},
			wantErr: true,
			errMsg:  "duplicate node name",
		},
		{
			name: "valid diamond DAG",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}},
					{Name: "B", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
					{Name: "C", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
					{Name: "D", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"B", "C"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestArtifact_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   Artifact
		wantErr bool
	}{
		{
			name:    "valid file artifact",
			input:   Artifact{Name: "output", Path: "/tmp/output.tar", Type: "file"},
			wantErr: false,
		},
		{
			name:    "valid directory artifact",
			input:   Artifact{Name: "logs", Path: "/var/log", Type: "directory"},
			wantErr: false,
		},
		{
			name:    "valid archive artifact",
			input:   Artifact{Name: "bundle", Path: "/tmp/bundle.tar.gz", Type: "archive"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   Artifact{Path: "/tmp/file", Type: "file"},
			wantErr: true,
		},
		{
			name:    "invalid - missing path",
			input:   Artifact{Name: "test", Type: "file"},
			wantErr: true,
		},
		{
			name:    "invalid - bad type",
			input:   Artifact{Name: "test", Path: "/tmp", Type: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Artifact validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSecretReference_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   SecretReference
		wantErr bool
	}{
		{
			name:    "valid secret reference",
			input:   SecretReference{Name: "db-secret", Key: "password", EnvVar: "DB_PASSWORD"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   SecretReference{Key: "password", EnvVar: "DB_PASSWORD"},
			wantErr: true,
		},
		{
			name:    "invalid - missing key",
			input:   SecretReference{Name: "db-secret", EnvVar: "DB_PASSWORD"},
			wantErr: true,
		},
		{
			name:    "invalid - missing env_var",
			input:   SecretReference{Name: "db-secret", Key: "password"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SecretReference validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOutputDefinition_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   OutputDefinition
		wantErr bool
	}{
		{
			name:    "valid stdout output",
			input:   OutputDefinition{Name: "build-id", ValueFrom: "stdout"},
			wantErr: false,
		},
		{
			name:    "valid stderr output",
			input:   OutputDefinition{Name: "errors", ValueFrom: "stderr"},
			wantErr: false,
		},
		{
			name:    "valid exitCode output",
			input:   OutputDefinition{Name: "code", ValueFrom: "exitCode"},
			wantErr: false,
		},
		{
			name:    "valid file output",
			input:   OutputDefinition{Name: "result", ValueFrom: "file", Path: "/tmp/result.json"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   OutputDefinition{ValueFrom: "stdout"},
			wantErr: true,
		},
		{
			name:    "invalid - missing value_from",
			input:   OutputDefinition{Name: "test"},
			wantErr: true,
		},
		{
			name:    "invalid - bad value_from",
			input:   OutputDefinition{Name: "test", ValueFrom: "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("OutputDefinition validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestInputMapping_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   InputMapping
		wantErr bool
	}{
		{
			name:    "valid input mapping",
			input:   InputMapping{Name: "BUILD_ID", From: "build.build-id"},
			wantErr: false,
		},
		{
			name:    "valid with default",
			input:   InputMapping{Name: "VERSION", From: "build.version", Default: "latest"},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   InputMapping{From: "build.id"},
			wantErr: true,
		},
		{
			name:    "invalid - missing from",
			input:   InputMapping{Name: "BUILD_ID"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("InputMapping validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkflowParameter_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   WorkflowParameter
		wantErr bool
	}{
		{
			name:    "valid parameter",
			input:   WorkflowParameter{Name: "env", Value: "production"},
			wantErr: false,
		},
		{
			name:    "valid with description",
			input:   WorkflowParameter{Name: "env", Value: "prod", Description: "Target environment", Required: true},
			wantErr: false,
		},
		{
			name:    "invalid - missing name",
			input:   WorkflowParameter{Value: "production"},
			wantErr: true,
		},
		{
			name:    "invalid - missing value",
			input:   WorkflowParameter{Name: "env"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("WorkflowParameter validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDAGNode_Validation(t *testing.T) {
	validate := validator.New()

	tests := []struct {
		name    string
		input   DAGNode
		wantErr bool
	}{
		{
			name: "valid node",
			input: DAGNode{
				Name: "build",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
				},
			},
			wantErr: false,
		},
		{
			name: "valid with dependencies",
			input: DAGNode{
				Name: "test",
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
				},
				Dependencies: []string{"build"},
			},
			wantErr: false,
		},
		{
			name: "invalid - missing name",
			input: DAGNode{
				Container: ExtendedContainerInput{
					ContainerExecutionInput: ContainerExecutionInput{Image: "alpine:latest"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DAGNode validation error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
