# Docker Workflow Examples

This directory contains example code demonstrating how to use the docker package for Temporal workflows.

## Prerequisites

1. **Temporal Server** - You need a running Temporal server. Options:
   - Local development: `temporal server start-dev`
   - Docker: `docker run -p 7233:7233 temporalio/auto-setup:latest`
   - Temporal Cloud: Configure client with cloud credentials

2. **Docker** - Docker daemon must be running for container execution

## Running Examples

All examples are tagged with `//go:build example` to exclude them from normal builds.

### Worker

Start a worker to process workflows:

```bash
go run -tags=example ./docker/examples/worker/main.go
```

Keep this running in a separate terminal.

### Basic Container Execution

Execute a single PostgreSQL container:

```bash
go run -tags=example ./docker/examples/basic.go
```

This demonstrates:
- Single container execution
- Wait strategy (log-based)
- Environment variables
- Port mapping
- Auto-removal after completion

### Pipeline Execution

Run a sequential build pipeline:

```bash
go run -tags=example ./docker/examples/pipeline.go
```

This demonstrates:
- Sequential container execution
- Named steps
- Stop-on-error behavior
- Pipeline results aggregation

### Parallel Execution

Run multiple test suites in parallel:

```bash
go run -tags=example ./docker/examples/parallel.go
```

This demonstrates:
- Concurrent container execution
- Concurrency limits
- Failure strategies (continue vs fail-fast)
- Parallel results collection

## Example Workflow

1. Terminal 1: Start Temporal server
   ```bash
   temporal server start-dev
   ```

2. Terminal 2: Start worker
   ```bash
   go run -tags=example ./docker/examples/worker/main.go
   ```

3. Terminal 3: Run any example
   ```bash
   go run -tags=example ./docker/examples/basic.go
   ```

## Customization

Each example can be modified to suit your needs:

- Change Docker images
- Modify commands and arguments
- Adjust wait strategies
- Configure timeouts and retries
- Add volume mounts
- Set environment variables

## Monitoring

View workflow execution in Temporal Web UI:
- Local: http://localhost:8233
- Cloud: https://cloud.temporal.io

## Troubleshooting

**Worker not picking up tasks:**
- Ensure worker is running
- Check task queue name matches (`docker-tasks`)
- Verify Temporal server is accessible

**Container execution fails:**
- Ensure Docker daemon is running
- Check Docker image availability
- Verify container resource requirements

**Workflow timeout:**
- Increase workflow timeout in StartWorkflowOptions
- Check activity execution time
- Review Docker container startup time
