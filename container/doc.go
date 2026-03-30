// Package container provides Temporal workflow activities for executing
// container workloads via Docker or Podman.
//
// It implements the workflow.TaskInput and workflow.TaskOutput interfaces with
// container-specific payload types (image, command, environment, volumes, etc.)
// and registers them as Temporal activities so they can be composed using the
// generic orchestration patterns in the workflow package (pipeline, parallel,
// loop, DAG).
//
// Typical usage involves calling [RegisterAll] to register all container
// workflows and activities with a Temporal worker.
package container
