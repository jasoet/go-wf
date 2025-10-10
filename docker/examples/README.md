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

Keep this running in a separate terminal for all examples below.

## Core Examples

### Basic Container Execution

Execute a single PostgreSQL container:

```bash
go run -tags=example ./docker/examples/basic.go
```

**Demonstrates:**
- Single container execution
- Wait strategy (log-based)
- Environment variables
- Port mapping
- Auto-removal after completion

**Argo equivalent:** Container template

### Pipeline Execution

Run a sequential build pipeline:

```bash
go run -tags=example ./docker/examples/pipeline.go
```

**Demonstrates:**
- Sequential container execution
- Named steps
- Stop-on-error behavior
- Pipeline results aggregation

**Argo equivalent:** Steps template (sequential)

### Parallel Execution

Run multiple test suites in parallel:

```bash
go run -tags=example ./docker/examples/parallel.go
```

**Demonstrates:**
- Concurrent container execution
- Concurrency limits
- Failure strategies (continue vs fail-fast)
- Parallel results collection

**Argo equivalent:** Steps template (parallel)

## Argo Workflow-like Examples

### DAG Workflow

Complex CI/CD pipeline with dependency graph:

```bash
go run -tags=example ./docker/examples/dag.go
```

**Demonstrates:**
- Directed Acyclic Graph (DAG) execution
- Complex dependencies between steps
- Resource limits (CPU, memory, GPU)
- Multi-stage CI/CD pipeline
- Conditional execution based on dependencies

**Argo equivalent:** DAG template

**Pipeline stages:**
```
                checkout
                    |
                install
              /    |    \
          build  lint  security-scan
              \    |    /
               test-unit
                    |
            +-------+-------+
            |               |
    test-integration  test-e2e
            |               |
            +-------+-------+
                    |
                 deploy
                    |
              health-check
                    |
               smoke-test
```

### Builder Patterns

Comprehensive examples using the fluent builder API:

```bash
go run -tags=example ./docker/examples/builder.go
```

**Demonstrates:**
- CI/CD pipeline with builder
- Parallel data processing
- Multi-language script templates (Bash, Python, Node, Ruby)
- HTTP operations (health checks, webhooks)
- Loop-like patterns (Argo withItems equivalent)
- Exit handlers for cleanup and notifications

**Argo equivalent:** WorkflowTemplate with various template types

**Features shown:**
1. **CI/CD Pipeline** - Checkout → Build → Test → Package → Deploy
2. **Parallel Processing** - Process multiple data files concurrently
3. **Script Templates** - Execute scripts in Bash, Python, Node.js, Ruby
4. **HTTP Operations** - Health checks and API calls
5. **Loop Pattern** - Deploy to multiple environments (dev, staging, prod)
6. **Exit Handlers** - Cleanup and notification tasks

### Advanced Features

Workflow parameters, resource limits, and conditional execution:

```bash
go run -tags=example ./docker/examples/advanced.go
```

**Demonstrates:**
- Workflow parameters (template variables)
- Resource limits (CPU, memory, GPU)
- Conditional execution with when clauses
- ContinueOnFail behavior
- Multiple wait strategies (log, port, health check)
- Parameter substitution in commands and environment

**Argo equivalent:** Parameters, resource requirements, conditionals

**Features shown:**
1. **Parameterized Workflows** - Template variables ({{.version}}, {{.env}})
2. **Resource Management** - CPU/memory/GPU limits per container
3. **Conditional Logic** - Execute based on previous step results
4. **Wait Strategies** - Log messages, port availability, health checks

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
