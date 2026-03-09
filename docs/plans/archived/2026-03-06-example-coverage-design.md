# Example Code Coverage Improvement Design

**Date**: 2026-03-06
**Status**: Approved
**Approach**: C — Add new files, small additions to existing files

## Problem

The `examples/docker/` directory covers ~55% of the go-wf API surface. Major gaps exist in the operations API (0/8 functions), pre-built patterns (2/16 functions), and several builder/template/extended container features.

## Solution

Add 3 new example files and make small additions to 2 existing files.

## New Files

### 1. `operations.go` — Workflow Lifecycle Management

Demonstrates the operations API from `docker/operations.go`:

- `docker.SubmitWorkflow()` — fire-and-forget submission
- `docker.SubmitAndWait()` — submit with timeout
- `docker.GetWorkflowStatus()` — query status
- `docker.CancelWorkflow()` — cancel running workflow
- `docker.TerminateWorkflow()` — force terminate
- `docker.SignalWorkflow()` — signal a workflow
- `docker.QueryWorkflow()` — query workflow state
- `docker.WatchWorkflow()` — stream updates via channel

Structure: Self-contained (embeds worker). Uses `client.Dial` directly. 4 example functions:

1. `submitAndForget()` — Submit a pipeline, check status, then wait for result
2. `submitWithTimeout()` — SubmitAndWait with a timeout
3. `cancelAndTerminate()` — Start long-running container, cancel it, then start another and terminate
4. `watchWorkflowUpdates()` — Start a workflow, watch via channel, print updates

### 2. `patterns.go` — Pre-built Pattern Functions

Showcases all 16 pre-built pattern functions from `docker/patterns/`:

CI/CD patterns:
- `BuildTestDeploy()`
- `BuildTestDeployWithHealthCheck()`
- `BuildTestDeployWithNotification()`
- `MultiEnvironmentDeploy()`

Parallel patterns:
- `FanOutFanIn()`
- `ParallelDataProcessing()`
- `ParallelTestSuite()`
- `ParallelDeployment()`
- `MapReduce()`

Loop patterns:
- `ParallelLoop()` / `SequentialLoop()`
- `MultiRegionDeployment()`
- `ParameterSweep()`
- `ParallelLoopWithTemplate()`

Structure: Self-contained (embeds worker). 3 grouped example functions:

1. `runCICDPatterns()` — all 4 CI/CD patterns
2. `runParallelPatterns()` — all 5 parallel patterns
3. `runLoopPatterns()` — all 6 loop patterns

### 3. `builder-advanced.go` — Advanced Builder/Template APIs

Demonstrates remaining builder, template, and source APIs:

Builder APIs:
- `BuildSingle()` — single container execution
- `Build()` — auto-select pipeline or parallel
- `Cleanup()` — cleanup between steps
- Constructor options: `WithStopOnError()`, `WithParallelMode()`, `WithMaxConcurrency()`, `WithGlobalAutoRemove()`
- `ContainerSource` / `NewContainerSource()` — wrap payload as WorkflowSource
- `AddInput()` — add raw ContainerExecutionInput

Loop builder APIs:
- `ForEachParam()` — parameterized loop convenience
- `NewParameterizedLoopBuilder()` with `WithTemplate()` and `BuildParameterizedLoop()`

Template APIs:
- `NewGoScript()` — Go script template
- `NewHTTPWebhook()` — webhook notification
- Container options: `WithVolume()`, `WithPorts()`, `WithLabel()`, `WithWaitForLog()`, `WithWaitForPort()`

Structure: Self-contained (embeds worker). 5 example functions:

1. `runSingleContainerBuilder()` — BuildSingle + constructor options
2. `runAutoSelectBuilder()` — Build() auto-select with ContainerSource and AddInput
3. `runCleanupPipeline()` — pipeline with Cleanup(true) and rich container options
4. `runParameterizedLoopBuilder()` — ForEachParam and NewParameterizedLoopBuilder
5. `runAdditionalTemplates()` — NewGoScript, NewHTTPWebhook, more container options

## Modifications to Existing Files

### `advanced.go` — Add Example 5

New function `runRetryAndSecretsDemo()`:
- `ExtendedContainerInput.RetryAttempts` + `RetryDelay`
- `ExtendedContainerInput.Secrets` (struct showcase)
- `ExtendedContainerInput.DependsOn`
- `OutputDefinition.ValueFrom="file"` — file-based output extraction
- `OutputDefinition.Default` — default values

### `artifacts.go` — Add Example 4

New function `artifactCleanupExample()`:
- `Artifact.Type="archive"` — archive artifact type
- `artifacts.CleanupWorkflowArtifacts()` reference
- `artifacts.ArtifactConfig` struct with `EnableCleanup` and `RetentionDays`

### `README.md` — Update documentation

Add descriptions for the 3 new example files and updated existing file descriptions.

## Coverage Impact

| Category | Before | After |
|----------|--------|-------|
| Operations API (8 functions) | 0/8 | 8/8 |
| Pre-built patterns (16 functions) | 2/16 | 16/16 |
| Builder/Template APIs | ~60% | ~95% |
| Extended container features | ~40% | ~90% |
| Overall feature coverage | ~55% | ~95% |

## Intentionally Uncovered

- `ListWorkflows()` / `GetWorkflowHistory()` — not implemented in base code
- `workflow.TaskInput`/`TaskOutput` generic interfaces — internal plumbing
- `workflow.helpers` functions — used internally by workflows

## Constraints

- All files use `//go:build example` tag
- All files in `package main`
- Self-contained files embed own worker
- No function name collisions across files
- Consistent style with existing examples
