package docker

import "time"

// ConditionalBehavior defines how containers behave based on conditions.
type ConditionalBehavior struct {
	// When specifies a condition for executing this container.
	// Supports Temporal workflow expressions.
	// Example: "{{steps.test.exitCode}} == 0"
	When string `json:"when,omitempty"`

	// ContinueOnFail allows the workflow to continue even if this container fails.
	ContinueOnFail bool `json:"continue_on_fail"`

	// ContinueOnError allows the workflow to continue on errors.
	ContinueOnError bool `json:"continue_on_error"`
}

// ResourceLimits defines resource constraints for a container.
type ResourceLimits struct {
	// CPURequest is the CPU request (e.g., "500m")
	CPURequest string `json:"cpu_request,omitempty"`

	// CPULimit is the CPU limit (e.g., "1000m")
	CPULimit string `json:"cpu_limit,omitempty"`

	// MemoryRequest is the memory request (e.g., "256Mi")
	MemoryRequest string `json:"memory_request,omitempty"`

	// MemoryLimit is the memory limit (e.g., "512Mi")
	MemoryLimit string `json:"memory_limit,omitempty"`

	// GPUCount is the number of GPUs to allocate
	GPUCount int `json:"gpu_count,omitempty"`
}

// Artifact defines input or output artifacts for containers.
type Artifact struct {
	// Name is the artifact identifier
	Name string `json:"name" validate:"required"`

	// Path is the file/directory path
	Path string `json:"path" validate:"required"`

	// Type can be "file", "directory", or "archive"
	Type string `json:"type" validate:"oneof=file directory archive"`

	// Optional indicates if the artifact is optional
	Optional bool `json:"optional"`
}

// SecretReference defines a reference to a secret.
type SecretReference struct {
	// Name is the secret name
	Name string `json:"name" validate:"required"`

	// Key is the key within the secret
	Key string `json:"key" validate:"required"`

	// EnvVar is the environment variable name to inject
	EnvVar string `json:"env_var" validate:"required"`
}

// ExtendedContainerInput extends ContainerExecutionInput with advanced features.
type ExtendedContainerInput struct {
	ContainerExecutionInput

	// Conditional behavior
	Conditional *ConditionalBehavior `json:"conditional,omitempty"`

	// Resource limits
	Resources *ResourceLimits `json:"resources,omitempty"`

	// Input artifacts
	InputArtifacts []Artifact `json:"input_artifacts,omitempty"`

	// Output artifacts
	OutputArtifacts []Artifact `json:"output_artifacts,omitempty"`

	// Secret references
	Secrets []SecretReference `json:"secrets,omitempty"`

	// Retry configuration
	RetryAttempts int           `json:"retry_attempts,omitempty"`
	RetryDelay    time.Duration `json:"retry_delay,omitempty"`

	// Dependencies on other containers
	DependsOn []string `json:"depends_on,omitempty"`
}

// WorkflowParameter defines a workflow input parameter.
type WorkflowParameter struct {
	// Name is the parameter identifier
	Name string `json:"name" validate:"required"`

	// Value is the parameter value
	Value string `json:"value" validate:"required"`

	// Description describes the parameter
	Description string `json:"description,omitempty"`

	// Required indicates if the parameter is required
	Required bool `json:"required"`
}

// DAGNode represents a node in a DAG workflow.
type DAGNode struct {
	// Name is the node identifier
	Name string `json:"name" validate:"required"`

	// Container is the container to execute
	Container ExtendedContainerInput `json:"container" validate:"required"`

	// Dependencies are the nodes that must complete before this node
	Dependencies []string `json:"dependencies,omitempty"`
}

// DAGWorkflowInput defines a DAG (Directed Acyclic Graph) workflow.
type DAGWorkflowInput struct {
	// Nodes are the workflow nodes
	Nodes []DAGNode `json:"nodes" validate:"required,min=1"`

	// Parameters are workflow parameters
	Parameters []WorkflowParameter `json:"parameters,omitempty"`

	// FailFast determines if the workflow should stop on first failure
	FailFast bool `json:"fail_fast"`

	// MaxParallel limits the number of parallel executions
	MaxParallel int `json:"max_parallel,omitempty"`
}

// Validate validates DAG workflow input.
func (i *DAGWorkflowInput) Validate() error {
	if len(i.Nodes) == 0 {
		return ErrInvalidInput.Wrap("at least one node is required")
	}

	// Check for circular dependencies (simplified check)
	nodeMap := make(map[string]bool)
	for _, node := range i.Nodes {
		nodeMap[node.Name] = true
	}

	for _, node := range i.Nodes {
		for _, dep := range node.Dependencies {
			if !nodeMap[dep] {
				return ErrInvalidInput.Wrap("dependency node not found: " + dep)
			}
		}
	}

	return nil
}
