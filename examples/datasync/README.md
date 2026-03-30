# DataSync Workflow Examples

This directory contains examples demonstrating the `go-wf/datasync` package for orchestrating typed data synchronization workflows with Temporal.

## Prerequisites

1. **Temporal Server** running locally:
   ```bash
   # Using Temporal CLI (recommended)
   temporal server start-dev

   # Or using Podman Compose
   git clone https://github.com/temporalio/docker-compose.git
   cd docker-compose
   podman-compose up -d
   ```

   Temporal Web UI: http://localhost:8233

2. **Go 1.26+** installed:
   ```bash
   go version
   ```

## Running Examples

All examples use the `//go:build example` build tag. Run with:

```bash
task example:datasync -- basic.go
task example:datasync -- pipeline.go
task example:datasync -- parallel.go
task example:datasync -- builder.go
```

Each example is self-contained: it creates a datasync job with source/mapper/sink, registers it with a Temporal worker, executes the workflow, and prints results.

## How It Works

The datasync module follows a source-mapper-sink pattern:

```
1. Define Source[T] to fetch records of type T
2. Define RecordMapper[T] to transform records (optional, identity by default)
3. Define Sink[T] to write records
4. Create a SyncJob[T] combining source + mapper + sink
5. Register the job with a Temporal worker
6. Execute the SyncWorkflow which orchestrates fetch -> map -> write
```

## Example Descriptions

### 1. Basic Sync (`basic.go`)

Single datasync job: fetch users from an in-memory source, identity-map, write to an in-memory sink.

**Demonstrates:** SyncJob creation, Source/Sink interfaces, worker registration, SyncWorkflow execution.

### 2. Pipeline (`pipeline.go`)

Sequential orchestration of multiple sync jobs. Two independent jobs (users and orders) execute one after the other using standard Temporal client calls.

**Demonstrates:** Multiple SyncJob registrations, sequential workflow execution, per-job result inspection.

### 3. Parallel Execution (`parallel.go`)

Concurrent execution of multiple sync jobs. Three independent data sources are synced in parallel.

**Demonstrates:** Parallel workflow submission, concurrent SyncJob execution, result aggregation.

### 4. Builder API (`builder.go`)

Fluent builder API for constructing sync jobs with custom record mapping. Transforms User records into UserDTO using a RecordMapper.

**Demonstrates:** Builder pattern, custom RecordMapper, typed transformations, SyncJobBuilder API.

## Worker Setup

For long-running scenarios, use the shared worker:

```bash
cd examples/datasync/worker
go run -tags example main.go
```

The worker registers multiple datasync jobs and listens for workflow executions. Trigger workflows via Temporal CLI or UI once the worker is running.

## Task Queue

All examples use the **`datasync-tasks`** task queue:

```go
w := worker.New(c, "datasync-tasks", worker.Options{})
```

## Troubleshooting

### Worker Not Connecting

```bash
temporal server start-dev
temporal workflow list
```

### Build Tag Error

Always use `-tags example`:
```bash
go run -tags example basic.go
```

### Workflow Stuck in Running

Ensure task queue names match between worker and workflow execution.

## Next Steps

- Review the [datasync package source](../../datasync/) for implementation details
- See the [function examples](../function/) for Go function orchestration
- See the [container examples](../container/) for container orchestration patterns
- Build custom Source/Sink implementations for your data stores
