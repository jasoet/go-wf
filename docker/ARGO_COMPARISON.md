# Argo Workflows Feature Comparison

This document compares the `go-wf/docker` package features with Argo Workflows to identify implemented features and gaps.

## Feature Matrix

| Feature | Argo Workflows | go-wf/docker | Status | Notes |
|---------|---------------|--------------|--------|-------|
| **Core Execution** |
| Single Container | ✅ | ✅ | Full | `ExecuteContainerWorkflow` |
| Sequential Steps | ✅ | ✅ | Full | `ContainerPipelineWorkflow` |
| Parallel Execution | ✅ | ✅ | Full | `ParallelContainersWorkflow` |
| DAG (Dependencies) | ✅ | ✅ | Full | `DAGWorkflow` with node dependencies |
| **Templates** |
| Container Template | ✅ | ✅ | Full | `template.NewContainer` with options |
| Script Template | ✅ | ✅ | Full | Bash, Python, Node, Ruby, Go scripts |
| Resource Template | ✅ | ❌ | Missing | K8s resource management |
| HTTP Template | ⚠️ | ✅ | Enhanced | HTTP requests, health checks, webhooks |
| **Advanced Features** |
| Conditionals (when) | ✅ | ✅ | Partial | `ConditionalBehavior` with when clauses |
| Loops (withItems) | ✅ | ❌ | **Missing** | No native loop support |
| Loops (withParam) | ✅ | ❌ | **Missing** | No dynamic parameterization |
| Parameters | ✅ | ✅ | Full | `WorkflowParameter` with template substitution |
| Artifacts | ✅ | ⚠️ | Defined | Structure exists but not fully implemented |
| Volumes | ✅ | ⚠️ | Basic | Volume mounts supported, no shared volumes |
| **Resource Management** |
| CPU Limits | ✅ | ✅ | Full | `ResourceLimits.CPURequest/Limit` |
| Memory Limits | ✅ | ✅ | Full | `ResourceLimits.MemoryRequest/Limit` |
| GPU Support | ✅ | ✅ | Full | `ResourceLimits.GPUCount` |
| **Lifecycle** |
| Retries | ✅ | ✅ | Full | Temporal retry policies |
| Timeouts | ✅ | ✅ | Full | `RunTimeout`, `StartTimeout` |
| Exit Handlers | ✅ | ✅ | Full | Builder `AddExitHandler` |
| Suspend/Resume | ✅ | ❌ | **Missing** | No manual intervention points |
| **Container Types** |
| Main Container | ✅ | ✅ | Full | Standard execution |
| Init Containers | ✅ | ⚠️ | Workaround | Use pipeline steps |
| Sidecars | ✅ | ❌ | **Missing** | No sidecar container support |
| Daemons | ✅ | ❌ | **Missing** | No daemon container support |
| **Workflow Management** |
| Submit Workflow | ✅ | ✅ | Full | `SubmitWorkflow` |
| Wait for Completion | ✅ | ✅ | Full | `SubmitAndWait` |
| Watch/Stream | ✅ | ✅ | Full | `WatchWorkflow` |
| Cancel | ✅ | ✅ | Full | `CancelWorkflow` |
| Terminate | ✅ | ✅ | Full | `TerminateWorkflow` |
| Signal | ✅ | ✅ | Full | `SignalWorkflow` |
| Query | ✅ | ✅ | Full | `QueryWorkflow` |
| **Scheduling** |
| Cron Workflows | ✅ | ❌ | **Missing** | No built-in cron support (use Temporal schedules) |
| Workflow of Workflows | ✅ | ❌ | **Missing** | No child workflow support |
| **Data Flow** |
| Input Parameters | ✅ | ✅ | Full | Workflow parameters |
| Output Parameters | ✅ | ⚠️ | Partial | Captured in `ContainerExecutionOutput` |
| Output Artifacts | ✅ | ⚠️ | Defined | Structure exists, not implemented |
| Step Outputs | ✅ | ⚠️ | Limited | Available but no explicit passing |
| **Synchronization** |
| Semaphores | ✅ | ❌ | **Missing** | No resource locking |
| Mutexes | ✅ | ❌ | **Missing** | No mutual exclusion |
| **Builder/DSL** |
| Fluent API | ⚠️ | ✅ | Enhanced | More intuitive than Argo YAML |
| Pre-built Patterns | ❌ | ✅ | **Enhanced** | CI/CD, parallel patterns out of box |
| Composable Templates | ⚠️ | ✅ | Full | `WorkflowSource` interface |
| **Observability** |
| Built-in Metrics | ✅ | ⚠️ | Inherited | Temporal provides metrics |
| Workflow Archive | ✅ | ⚠️ | Inherited | Temporal provides history |
| Event Emission | ✅ | ⚠️ | Inherited | Temporal provides events |
| **Storage** |
| Artifact Repository | ✅ | ❌ | **Missing** | No artifact storage |
| Volume Snapshots | ✅ | ❌ | **Missing** | No volume management |
| **Security** |
| Secrets | ✅ | ⚠️ | Defined | `SecretReference` structure exists |
| Pod Security | ✅ | N/A | N/A | Temporal-based, not pod-based |
| **Other** |
| Pod GC | ✅ | N/A | N/A | Docker auto-remove |
| Memoization | ✅ | ❌ | **Missing** | No result caching |
| Workflow Templates | ✅ | ⚠️ | Partial | Can create reusable functions |

## Summary

### Strengths (vs Argo Workflows)
1. **Simpler Developer Experience** - Fluent Go API vs YAML
2. **Type Safety** - Compile-time validation
3. **Pre-built Patterns** - Ready-to-use CI/CD patterns
4. **Better Local Development** - No Kubernetes required
5. **Temporal Benefits** - Built-in observability, retries, history
6. **Flexible Templates** - Composable `WorkflowSource` interface

### Critical Gaps
1. **No Loop Support** - Cannot iterate over items (withItems/withParam)
2. **No Sidecars** - Cannot run supporting containers
3. **No Suspend/Resume** - No manual intervention points
4. **Limited Data Passing** - No explicit step output -> input mapping
5. **No Synchronization** - No semaphores/mutexes for resource control
6. **No Artifact Storage** - Artifacts defined but not stored/retrieved
7. **No Workflow Nesting** - Cannot call child workflows

### Minor Gaps
1. **Secrets Not Implemented** - Structure exists, needs integration
2. **No Shared Volumes** - Each container has own volumes
3. **No Cron Support** - Need to use Temporal schedules directly
4. **No Resource Templates** - K8s-specific, not applicable

## Recommendations

### High Priority Additions
1. **Loop Support** - Add `withItems` and `withParam` equivalents
   ```go
   builder.ForEach(items, func(item string) *template.Container {
       return template.NewContainer("process", "alpine",
           template.WithEnv("ITEM", item))
   })
   ```

2. **Data Passing** - Explicit output -> input mapping
   ```go
   node := DAGNode{
       Name: "process",
       Inputs: []Input{{From: "build.output"}},
   }
   ```

3. **Sidecar Support** - Long-running supporting containers
   ```go
   container := template.NewContainer("app", "myapp",
       template.WithSidecar("proxy", "envoy:latest"))
   ```

### Medium Priority
1. **Suspend/Resume** - Manual workflow gates
2. **Artifact Implementation** - Complete artifact storage/retrieval
3. **Shared Volumes** - Volume sharing between steps
4. **Workflow Nesting** - Child workflow support

### Low Priority
1. **Synchronization** - Semaphores and mutexes
2. **Memoization** - Step result caching
3. **Pod GC Policies** - Container cleanup strategies

## Workarounds

### Loops (Current)
Use builder to create containers programmatically:
```go
wb := builder.NewWorkflowBuilder("loop-example")
for _, item := range items {
    wb.Add(template.NewContainer(item, "alpine",
        template.WithEnv("ITEM", item)))
}
input, _ := wb.BuildPipeline()
```

### Sidecars (Current)
Use parallel execution with long-running containers:
```go
// Not ideal, but functional
parallel := builder.NewWorkflowBuilder("with-sidecar").
    Parallel(true).
    Add(sidecarContainer).
    Add(mainContainer).
    BuildParallel()
```

### Suspend/Resume (Current)
Use Temporal signals:
```go
// Workflow waits for signal
workflow.GetSignalChannel(ctx, "continue").Receive(ctx, nil)

// External trigger
docker.SignalWorkflow(ctx, client, workflowID, runID, "continue", nil)
```

## Argo Workflow Examples Ported to go-wf

See [examples/](./examples/) directory for:
- `dag.go` - DAG workflow (Argo DAG equivalent)
- `builder.go` - Complex builder patterns (Argo steps equivalent)
- `basic.go` - Simple container (Argo container template)
- `pipeline.go` - Sequential execution (Argo steps)
- `parallel.go` - Parallel execution (Argo parallelism)

## Conclusion

The `go-wf/docker` package provides **~70% feature parity** with Argo Workflows, with notable advantages in developer experience and type safety. Critical gaps exist in loops, sidecars, and data passing. Most Argo Workflows can be ported with minor adaptations.

For production use cases similar to Argo Workflows, evaluate if the missing features are critical for your use case. Many can be worked around, but loops and explicit data passing are the most significant limitations.

## Roadmap

See [ROADMAP.md](./ROADMAP.md) for detailed implementation plans to close the feature gaps. Priority features:

**P0 - Critical (Next Release):**
- Loop Support (withItems/withParam)
- Explicit Data Passing Between Steps
- Artifact Storage Implementation

**P1 - High Priority:**
- Suspend/Resume Workflow (Approval Gates)
- Per-step Retry Policies

**P2-P3 - Medium/Low Priority:**
- Workflow Templates
- Synchronization Primitives
- Memoization

Target: **85-95% feature parity** with Argo Workflows (non-K8s features) by end of 2025.
