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

// OutputDefinition defines how to capture output from a container.
type OutputDefinition struct {
	// Name is the output identifier
	Name string `json:"name" validate:"required"`

	// ValueFrom specifies where to extract the value from
	// Options: "stdout", "stderr", "exitCode", "file"
	ValueFrom string `json:"value_from" validate:"required,oneof=stdout stderr exitCode file"`

	// Path is the file path to read (required when ValueFrom is "file")
	Path string `json:"path,omitempty"`

	// JSONPath for extracting specific values from JSON output
	// Example: "$.build.id" to extract build.id from JSON
	JSONPath string `json:"json_path,omitempty"`

	// Regex pattern to extract value using regex
	Regex string `json:"regex,omitempty"`

	// Default value if extraction fails
	Default string `json:"default,omitempty"`
}

// InputMapping defines how to map outputs from previous steps to inputs.
type InputMapping struct {
	// Name is the environment variable or parameter name
	Name string `json:"name" validate:"required"`

	// From specifies the source in format "step-name.output-name"
	From string `json:"from" validate:"required"`

	// Default value if the source is not available
	Default string `json:"default,omitempty"`

	// Required indicates if this input must be present
	Required bool `json:"required"`
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

	// Output definitions for capturing container outputs
	Outputs []OutputDefinition `json:"outputs,omitempty"`

	// Input mappings from previous step outputs
	Inputs []InputMapping `json:"inputs,omitempty"`
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

	// ArtifactStore is the artifact storage backend (optional)
	// If provided, artifacts will be automatically uploaded/downloaded
	ArtifactStore interface{} `json:"-"`
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

// NodeResult represents the execution result of a single DAG node.
type NodeResult struct {
	// NodeName is the name of the node
	NodeName string `json:"node_name"`

	// Result is the container execution output
	Result *ContainerExecutionOutput `json:"result,omitempty"`

	// StartTime is when the node started executing
	StartTime time.Time `json:"start_time"`

	// Success indicates if the node executed successfully
	Success bool `json:"success"`

	// Error contains error information if the node failed
	Error error `json:"error,omitempty"`
}

// DAGWorkflowOutput defines the output of a DAG workflow execution.
type DAGWorkflowOutput struct {
	// Results is a map of node name to execution output
	Results map[string]*ContainerExecutionOutput `json:"results"`

	// NodeResults is a list of all node results in execution order
	NodeResults []NodeResult `json:"node_results"`

	// StepOutputs contains extracted outputs from each step
	StepOutputs map[string]map[string]string `json:"step_outputs,omitempty"`

	// TotalSuccess is the count of successful nodes
	TotalSuccess int `json:"total_success"`

	// TotalFailed is the count of failed nodes
	TotalFailed int `json:"total_failed"`

	// TotalDuration is the total execution time
	TotalDuration time.Duration `json:"total_duration"`
}
