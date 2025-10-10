package builder

import "github.com/jasoet/go-wf/docker/payload"

// WorkflowSource represents a composable workflow component that can generate
// container execution inputs. This allows building complex workflows from reusable parts.
//
// Example:
//
//	type DeploymentStep struct {
//	    image string
//	    env string
//	}
//
//	func (d *DeploymentStep) ToInput() payload.ContainerExecutionInput {
//	    return payload.ContainerExecutionInput{
//	        Image: d.image,
//	        Env: map[string]string{"ENV": d.env},
//	    }
//	}
type WorkflowSource interface {
	// ToInput converts the source into a ContainerExecutionInput
	ToInput() payload.ContainerExecutionInput
}

// WorkflowSourceFunc is a function adapter for WorkflowSource interface.
type WorkflowSourceFunc func() payload.ContainerExecutionInput

// ToInput implements WorkflowSource interface.
func (f WorkflowSourceFunc) ToInput() payload.ContainerExecutionInput {
	return f()
}

// ContainerSource creates a WorkflowSource from a ContainerExecutionInput.
// This allows treating existing inputs as sources in the builder pattern.
//
// Example:
//
//	input := payload.ContainerExecutionInput{Image: "alpine:latest"}
//	source := builder.ContainerSource(input)
type ContainerSource struct {
	input payload.ContainerExecutionInput
}

// NewContainerSource creates a new container source.
func NewContainerSource(input payload.ContainerExecutionInput) *ContainerSource {
	return &ContainerSource{input: input}
}

// ToInput implements WorkflowSource interface.
func (c *ContainerSource) ToInput() payload.ContainerExecutionInput {
	return c.input
}
