# Function Module Feature Parity Design

**Date:** 2026-03-25
**Status:** Approved
**Approach:** Vertical Slice (DAG end-to-end first, then Patterns)

## Problem

The function module is missing two features that the docker module has:
1. **DAG workflow** — dependency-based execution graph
2. **Patterns package** — pre-built workflow compositions

## Decisions

- Full feature parity with docker, adapted for function-native semantics
- Data passing between DAG nodes via Result key mapping + Data byte passthrough (no artifact store, stdout/stderr extraction — those are container-specific)
- Trigger CLI and local environment updated with DAG examples
- 2-3 new worker handlers for DAG-specific scenarios
- OTel instrumentation included from the start

## Delivery Slices

### Slice 1: DAG Workflow (end-to-end)

#### Payload Types (`function/payload/payload_extended.go`)

New types for DAG data passing and execution:

- `OutputMapping` — exposes keys from a function's `Result` map as named outputs
  - `Name string` — output identifier
  - `ResultKey string` — key in Result map to expose
  - `Default string` — fallback if key missing
- `InputMapping` — maps a dependency's output to an arg in the current node
  - `Name string` — arg key to set
  - `From string` — format: `"node-name.output-name"`
  - `Default string` — fallback if source unavailable
  - `Required bool` — fail if source missing
- `DataMapping` — passes `Data []byte` from one node to another
  - `FromNode string` — source node name
  - `Optional bool` — whether missing data is an error
- `DAGNode` — a node in the DAG
  - `Name string`
  - `Function FunctionExecutionInput`
  - `Dependencies []string`
  - `Outputs []OutputMapping`
  - `Inputs []InputMapping`
  - `DataInput *DataMapping`
- `DAGWorkflowInput` — top-level input
  - `Nodes []DAGNode`
  - `FailFast bool`
  - `MaxParallel int`
  - `Validate()` — checks at least 1 node, no unknown deps, no circular deps
- `NodeResult` — per-node result
  - `NodeName string`
  - `Result *FunctionExecutionOutput`
  - `StartTime time.Time`
  - `Success bool`
  - `Error error`
- `DAGWorkflowOutput` — aggregated output
  - `Results map[string]*FunctionExecutionOutput`
  - `NodeResults []NodeResult`
  - `StepOutputs map[string]map[string]string`
  - `TotalSuccess int`, `TotalFailed int`, `TotalDuration time.Duration`

#### Workflow (`function/workflow/dag.go`)

Follows docker's recursive execution pattern:

- `DAGWorkflow(ctx wf.Context, input payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error)`
- `dagState` struct — `executed`, `results`, `stepOutputs`, `stepData` maps
- `executeDAGNode` recursive function:
  1. Skip if already executed
  2. Execute dependencies recursively
  3. Apply input mappings — resolve `"node.output"` from `stepOutputs` into `Args`
  4. Apply data mapping — copy `[]byte` from referenced node into `Data`
  5. Execute activity via `wf.ExecuteActivity`
  6. Extract outputs — map `Result` keys via `OutputMapping` into `stepOutputs`
  7. Store result + data in `dagState`
  8. Record `NodeResult`
- Helper functions: `buildNodeMap`, `applyInputMappings`, `applyDataMapping`, `extractAndStoreOutputs`, `defaultActivityOptions`

#### OTel Instrumentation (`function/workflow/dag.go`)

Following existing patterns in `workflow/otel.go` and `function/activity/otel.go`:

- `InstrumentedDAGWorkflow(ctx, input)` — wraps `DAGWorkflow` with root span `fn.dag.workflow`
  - Span attributes: `dag.name`, `dag.node_count`, `dag.fail_fast`, `dag.max_parallel`
  - On completion: `dag.total_success`, `dag.total_failed`, `dag.total_duration_ms`
- Per-node child span `fn.dag.node` inside `executeDAGNode`
  - Attributes: `node.name`, `node.dependency_count`, `node.has_input_mappings`, `node.has_data_mapping`
  - On completion: `node.success`, `node.duration_ms`
- Metrics:
  - Counter: `fn.dag.node.executions` (labels: `node_name`, `success`)
  - Histogram: `fn.dag.node.duration`
  - Counter: `fn.dag.workflow.executions` (labels: `success`)
  - Histogram: `fn.dag.workflow.duration`
- Registered workflow is `InstrumentedDAGWorkflow` (not raw `DAGWorkflow`)

#### Builder (`function/builder/dag_builder.go`)

Fluent API for constructing DAG inputs:

- `NewDAGBuilder(name string) *DAGBuilder`
- `AddNode(name string, fn WorkflowSource, deps ...string) *DAGBuilder`
- `AddNodeWithInput(name string, input FunctionExecutionInput, deps ...string) *DAGBuilder`
- `WithOutputMapping(nodeName string, mappings ...OutputMapping) *DAGBuilder`
- `WithInputMapping(nodeName string, mappings ...InputMapping) *DAGBuilder`
- `WithDataMapping(nodeName string, fromNode string) *DAGBuilder`
- `FailFast(ff bool) *DAGBuilder`
- `MaxParallel(max int) *DAGBuilder`
- `BuildDAG() (*DAGWorkflowInput, error)`

Example usage:
```go
dag, err := builder.NewDAGBuilder("ci-pipeline").
    AddNode("compile", compileSource).
    AddNode("unit-test", testSource, "compile").
    AddNode("lint", lintSource, "compile").
    AddNode("publish", publishSource, "unit-test", "lint").
    WithOutputMapping("compile", payload.OutputMapping{Name: "artifact", ResultKey: "path"}).
    WithInputMapping("publish", payload.InputMapping{Name: "artifact_path", From: "compile.artifact"}).
    FailFast(true).
    BuildDAG()
```

#### Worker Handlers (`examples/function/worker/main.go`)

3 new handlers (total: 21):
- `compile` — returns `{"artifact": "app-binary", "status": "compiled"}`
- `run-tests` — returns `{"passed": "42", "failed": "0"}`
- `publish-artifact` — reads `artifact_path` from args, returns `{"published": "true", "registry": "artifacts.example.com"}`

#### Trigger Updates (`examples/trigger/main.go`)

- `runAll()`: add `demo-fn-dag-etl-{ts}` and `demo-fn-dag-ci-{ts}`
- `createSchedules()`: add `schedule-fn-dag-ci` (every 15 min)
- `cleanSchedules()`: include `schedule-fn-dag-ci`

#### Registration

Register `InstrumentedDAGWorkflow` with Temporal worker in `function/register.go` or equivalent.

### Slice 2: Patterns Package

#### Pipeline Patterns (`function/patterns/pipeline.go`)
- `ETLPipeline(source, format, target string) (*PipelineInput, error)` — extract -> transform -> load
- `ValidateTransformNotify(email, name, channel string) (*PipelineInput, error)` — validate -> transform -> notify
- `MultiEnvironmentDeploy(version string, environments []string) (*PipelineInput, error)` — sequential deploy

#### Parallel Patterns (`function/patterns/parallel.go`)
- `FanOutFanIn(functionNames []string) (*ParallelInput, error)` — run named functions concurrently
- `ParallelDataFetch() (*ParallelInput, error)` — fetch-users + fetch-orders + fetch-inventory
- `ParallelHealthCheck(services []string, env string) (*ParallelInput, error)` — concurrent health checks

#### Loop Patterns (`function/patterns/loop.go`)
- `BatchProcess(items []string, functionName string) (*LoopInput, error)` — parallel loop
- `SequentialMigration(migrations []string) (*LoopInput, error)` — sequential, fail-fast
- `MultiRegionDeploy(environments, regions []string, version string) (*ParameterizedLoopInput, error)` — parameterized deploy
- `ParameterSweep(params map[string][]string, functionName string, maxConcurrency int) (*ParameterizedLoopInput, error)` — parameterized experimentation

#### DAG Patterns (`function/patterns/dag.go`)
- `ETLWithValidation(source, format, target string) (*DAGWorkflowInput, error)` — validate-config || extract -> transform -> load
- `CIPipeline() (*DAGWorkflowInput, error)` — compile -> (unit-test || lint) -> publish

All patterns use the builder API internally.

## Out of Scope

- Artifact storage for functions (container-specific)
- Resource limits / secrets (container-specific)
- Docker module changes
