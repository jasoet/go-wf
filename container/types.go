package container

import (
	"github.com/jasoet/go-wf/container/payload"
)

// Type aliases re-exported from container/payload for convenience.
// This allows consumers to use container.ContainerExecutionInput instead of
// container/payload.ContainerExecutionInput.
type (
	ContainerExecutionInput  = payload.ContainerExecutionInput
	WaitStrategyConfig       = payload.WaitStrategyConfig
	ContainerExecutionOutput = payload.ContainerExecutionOutput
)

// ValidateVolumes re-exports the volume validation function.
var ValidateVolumes = payload.ValidateVolumes
