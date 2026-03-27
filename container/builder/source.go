package builder

import (
	"github.com/jasoet/go-wf/container/payload"
	"github.com/jasoet/go-wf/workflow"
)

// GenericSource is a generic interface for composable workflow components.
// It can generate any type of task input.
type GenericSource[I workflow.TaskInput] interface {
	// ToTaskInput converts the source into a task input
	ToTaskInput() I
}

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

// GenericSourceFunc is a generic function adapter for GenericSource.
type GenericSourceFunc[I workflow.TaskInput] func() I

// ToTaskInput implements GenericSource interface.
func (f GenericSourceFunc[I]) ToTaskInput() I {
	return f()
}

// TaskInputSource wraps any value as a GenericSource.
type TaskInputSource[I workflow.TaskInput] struct {
	input I
}

// NewTaskInputSource creates a new generic source from a task input.
func NewTaskInputSource[I workflow.TaskInput](input I) *TaskInputSource[I] {
	return &TaskInputSource[I]{input: input}
}

// ToTaskInput implements GenericSource interface.
func (s *TaskInputSource[I]) ToTaskInput() I {
	return s.input
}
