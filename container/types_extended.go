package container

import (
	"github.com/jasoet/go-wf/container/payload"
)

// Type aliases re-exported from container/payload for convenience.
type (
	ConditionalBehavior    = payload.ConditionalBehavior
	ResourceLimits         = payload.ResourceLimits
	Artifact               = payload.Artifact
	SecretReference        = payload.SecretReference
	OutputDefinition       = payload.OutputDefinition
	InputMapping           = payload.InputMapping
	ExtendedContainerInput = payload.ExtendedContainerInput
	WorkflowParameter      = payload.WorkflowParameter
	DAGNode                = payload.DAGNode
	DAGWorkflowInput       = payload.DAGWorkflowInput
	NodeResult             = payload.NodeResult
	DAGWorkflowOutput      = payload.DAGWorkflowOutput
)
