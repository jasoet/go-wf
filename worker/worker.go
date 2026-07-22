// Package worker provides helpers for creating Temporal workers with go-wf
// conventions, including opt-in Worker Versioning (Worker Deployments).
package worker

import (
	"os"

	"go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
	temporalwf "go.temporal.io/sdk/workflow"
)

// Environment variables enabling Worker Versioning. Both must be set.
const (
	// EnvDeploymentName names the Temporal Worker Deployment (e.g. "go-wf").
	EnvDeploymentName = "TEMPORAL_DEPLOYMENT_NAME"
	// EnvBuildID identifies this build of the worker (e.g. a git SHA).
	EnvBuildID = "TEMPORAL_BUILD_ID"
)

// VersionedOptions returns opts with Worker Versioning enabled. The default
// versioning behavior is Pinned: an execution stays on the worker build it
// started with, so deploying new workflow code never breaks in-flight replays.
func VersionedOptions(opts sdkworker.Options, deploymentName, buildID string) sdkworker.Options {
	opts.DeploymentOptions = sdkworker.DeploymentOptions{
		UseVersioning: true,
		Version: sdkworker.WorkerDeploymentVersion{
			DeploymentName: deploymentName,
			BuildID:        buildID,
		},
		DefaultVersioningBehavior: temporalwf.VersioningBehaviorPinned,
	}
	return opts
}

// OptionsFromEnv returns VersionedOptions when both EnvDeploymentName and
// EnvBuildID are set; otherwise it returns opts unchanged.
func OptionsFromEnv(opts sdkworker.Options) sdkworker.Options {
	name, id := os.Getenv(EnvDeploymentName), os.Getenv(EnvBuildID)
	if name == "" || id == "" {
		return opts
	}
	return VersionedOptions(opts, name, id)
}

// New creates a Temporal worker on taskQueue. When TEMPORAL_DEPLOYMENT_NAME
// and TEMPORAL_BUILD_ID are set, Worker Versioning is enabled automatically.
func New(c client.Client, taskQueue string, opts sdkworker.Options) sdkworker.Worker {
	return sdkworker.New(c, taskQueue, OptionsFromEnv(opts))
}
