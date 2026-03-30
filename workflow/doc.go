// Package workflow provides generic orchestration primitives built on Temporal.
//
// It defines the [TaskInput] and [TaskOutput] interfaces that all workflow task
// types must satisfy, and supplies reusable, fully-generic workflow patterns:
//
//   - [ExecuteTaskWorkflow] — run a single task as a Temporal activity.
//   - [PipelineWorkflow] — execute tasks sequentially with optional stop-on-error.
//   - [ParallelWorkflow] — execute tasks concurrently with a configurable concurrency limit.
//   - [LoopWorkflow] — repeat a task on a schedule until cancelled.
//   - [DAGWorkflow] — execute tasks as a directed acyclic graph respecting dependency edges.
//
// Each pattern is parameterised on [I TaskInput, O TaskOutput] so concrete
// packages (container, function, datasync) can plug in their own payload types
// without losing type safety.
package workflow
