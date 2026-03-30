// Package workflow provides generic orchestration primitives built on Temporal.
//
// It defines the [TaskInput] and [TaskOutput] interfaces that all workflow task
// types must satisfy, and supplies reusable, fully-generic workflow patterns:
//
//   - [ExecuteTaskWorkflow] — run a single task as a Temporal activity.
//   - [PipelineWorkflow] — execute tasks sequentially with optional stop-on-error.
//   - [ParallelWorkflow] — execute tasks concurrently with a configurable concurrency limit.
//   - [LoopWorkflow] / [ParameterizedLoopWorkflow] — execute a task template for each item in a list, sequentially or in parallel.
//
// The package also defines [DAGInput], [DAGNode], and [DAGOutput] types for directed
// acyclic graph execution with dependency edges. Concrete DAG workflow implementations
// live in the container/ and function/ packages.
//
// Each pattern is parameterized on [I TaskInput, O TaskOutput] so concrete
// packages (container, function, datasync) can plug in their own payload types
// without losing type safety.
package workflow
