// Package function provides Temporal workflow activities for dispatching
// arbitrary Go functions.
//
// It implements the workflow.TaskInput and workflow.TaskOutput interfaces with
// function-specific payload types and uses a [Registry] to map function names
// to their implementations at runtime.  Registered functions are executed as
// Temporal activities and can be composed using the generic orchestration
// patterns in the workflow package (pipeline, parallel, loop, DAG).
package function
