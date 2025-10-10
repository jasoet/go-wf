# Docker Workflow Examples

This directory contains comprehensive examples demonstrating the capabilities of the `go-wf/docker` package for orchestrating Docker container workflows with Temporal.

## Prerequisites

Before running these examples, ensure you have:

1. **Temporal Server** running locally:
   ```bash
   # Using Docker Compose (recommended)
   git clone https://github.com/temporalio/docker-compose.git
   cd docker-compose
   docker-compose up -d

   # Or using Temporal CLI
   temporal server start-dev
   ```

   The Temporal Web UI will be available at: http://localhost:8233

2. **Docker** installed and running:
   ```bash
   docker --version
   # Docker version 20.10.0 or higher recommended
   ```

3. **Go 1.23+** installed:
   ```bash
   go version
   # go version go1.23 or higher
   ```

## Running Examples

All examples in this directory use the `//go:build example` build tag to prevent accidental execution. There are two ways to run them:

### Method 1: Using the Worker (Recommended)

1. Start the worker in a separate terminal:
   ```bash
   cd examples/docker/worker
   go run -tags example main.go
   ```

   The worker will:
   - Register all Docker workflows and activities
   - Listen on the `docker-tasks` task queue
   - Process workflow executions

2. In another terminal, run any example:
   ```bash
   cd examples/docker
   go run -tags example basic.go
   go run -tags example pipeline.go
   go run -tags example parallel.go
   # ... etc
   ```

### Method 2: Self-Contained Examples

Some examples include their own embedded worker and can be run directly:

```bash
cd examples/docker
go run -tags example advanced.go
go run -tags example builder.go
go run -tags example dag.go
go run -tags example loop.go
go run -tags example data-passing.go
go run -tags example artifacts.go
```

These examples start a worker internally, execute the workflow, and clean up automatically.

## Example Descriptions

### 1. Basic Container Execution (`basic.go`)

**Purpose**: Demonstrates single container execution with wait strategies.

**Features**:
- Single container workflow
- PostgreSQL container with log-based wait strategy
- Environment variable configuration
- Port exposure
- Auto-removal after completion

**Run**:
```bash
go run -tags example basic.go
```

**Use Case**: Testing container startup, database initialization, or service health checks.

**Argo Workflow Equivalent**: Container template

---

### 2. Pipeline Workflow (`pipeline.go`)

**Purpose**: Sequential container execution pipeline (Build → Test → Deploy).

**Features**:
- Sequential step execution
- Stop-on-error behavior
- Automatic cleanup
- Duration tracking per step
- Result aggregation

**Run**:
```bash
go run -tags example pipeline.go
```

**Use Case**: CI/CD pipelines where each step depends on the previous one.

**Argo Workflow Equivalent**: Steps template (sequential)

---

### 3. Parallel Execution (`parallel.go`)

**Purpose**: Concurrent container execution with controlled concurrency.

**Features**:
- Parallel container execution
- Max concurrency limiting (3 containers at a time)
- Failure strategy: continue on errors
- Success/failure tracking
- Result collection from all containers

**Run**:
```bash
go run -tags example parallel.go
```

**Use Case**: Running test suites, batch processing, or independent tasks concurrently.

**Argo Workflow Equivalent**: Steps template (parallel)

---

### 4. DAG Workflow (`dag.go`)

**Purpose**: Complex dependency graph orchestration (similar to Argo Workflows DAG).

**Features**:
- Multi-node dependency graph
- Parallel execution where possible
- Resource limits (CPU, memory, GPU)
- Fail-fast mode
- Execution order tracking
- Conditional execution based on dependencies

**Workflow Graph**:
```
                    checkout
                       |
                    install
                  /    |    \
              build  lint  security-scan
                  \    |    /
                   test-unit
                       |
              +--------+--------+
              |                 |
        test-integration  test-e2e
              |                 |
              +--------+--------+
                       |
                    deploy
                       |
                 health-check
                       |
                   smoke-test
```

**Run**:
```bash
go run -tags example dag.go
```

**Use Case**: Complex CI/CD pipelines with parallel stages and conditional execution.

**Argo Workflow Equivalent**: DAG template

---

### 5. Loop Workflows (`loop.go`)

**Purpose**: Demonstrates loop patterns for iterative container execution.

**Features**:
- Simple parallel loops (`withItems`)
- Sequential loops
- Parameterized loops (`withParam`) for matrix builds
- Loop builder API with fluent interface
- Pattern functions (matrix builds, batch processing)
- Concurrency control with max concurrent tasks
- Failure strategies (fail-fast vs continue)

**Examples Included**:
1. **Simple Parallel Loop**: Process multiple files concurrently
2. **Sequential Loop**: Deploy to regions one by one (rate limiting)
3. **Parameterized Loop**: Matrix deployment (env × region combinations)
4. **Loop Builder**: Fluent API for loop construction
5. **Pattern Functions**: Pre-built patterns for common scenarios
6. **Batch Processing**: Process items with concurrency limits

**Run**:
```bash
go run -tags example loop.go
```

**Use Case**: Matrix builds, batch data processing, multi-region deployments, iterative testing.

**Argo Workflow Equivalent**: withItems, withParam templates

---

### 6. Data Passing Between Steps (`data-passing.go`)

**Purpose**: Demonstrates explicit data passing between workflow steps.

**Features**:
- Output capture from containers (stdout, stderr, exit code)
- JSONPath extraction from JSON outputs
- Regex extraction from text outputs
- Input mapping from previous step outputs
- Required and optional inputs with defaults
- Automatic data flow between dependent steps

**Examples Included**:
1. **Build → Test → Deploy Pipeline**: Pass version and build ID between steps
2. **JSON Output Extraction**: Extract configuration values using JSONPath
3. **Regex Extraction**: Extract version, build number, artifact name using regex
4. **Multiple Outputs/Inputs**: Complex data flow with multiple values from stdout/stderr/exitCode

**Run**:
```bash
go run -tags example data-passing.go
```

**Use Case**: Passing build artifacts metadata, configuration values, version numbers between pipeline stages.

**Argo Workflow Equivalent**: Outputs and parameters passing

---

### 7. Artifact Storage (`artifacts.go`)

**Purpose**: Demonstrates artifact storage and retrieval between workflow steps.

**Features**:
- Local file store for artifacts
- Minio (S3-compatible) object storage integration
- File and directory artifact types
- Artifact upload after step completion
- Artifact download before step execution
- Optional artifacts (don't fail if missing)
- Workflow-scoped artifact namespace

**Examples Included**:
1. **Build → Test Pipeline**: Upload binary from build, download in test
2. **Build → Test → Deploy**: Multiple artifacts (binary, metadata, test results)
3. **Minio Storage**: Using S3-compatible storage for artifacts with remote persistence

**Run**:
```bash
# Requires /tmp/workflow-artifacts directory
mkdir -p /tmp/workflow-artifacts

# For Minio example, start Minio first:
docker run -d -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=minioadmin \
  -e MINIO_ROOT_PASSWORD=minioadmin \
  minio/minio server /data --console-address ":9001"

go run -tags example artifacts.go
```

**Use Case**: Passing build artifacts, test results, deployment packages, logs between stages.

**Argo Workflow Equivalent**: Artifacts with S3/GCS/Artifactory

---

### 8. Builder API (`builder.go`)

**Purpose**: Demonstrates the fluent builder API for constructing workflows programmatically.

**Features**:
- Workflow builder pattern with method chaining
- Script templates (Bash, Python, Node.js, Ruby)
- HTTP operations (health checks, webhooks)
- Container templates with options
- Exit handlers for cleanup
- Parallel and pipeline workflow construction
- Concurrency control

**Examples Included**:
1. **CI/CD Pipeline**: Checkout → Build → Test → Package → Deploy
2. **Parallel Data Processing**: Process multiple files with concurrency limits
3. **Multi-Language Scripts**: Bash, Python, Node.js, Ruby script execution
4. **HTTP Operations**: Health checks and API calls
5. **Loop Pattern**: Programmatic container creation (withItems equivalent)
6. **Exit Handlers**: Cleanup and notification tasks that always run

**Run**:
```bash
go run -tags example builder.go
```

**Use Case**: Building complex workflows programmatically with clean, readable, maintainable code.

**Argo Workflow Equivalent**: WorkflowTemplate with template types

---

### 9. Advanced Features (`advanced.go`)

**Purpose**: Demonstrates advanced workflow features for production use.

**Features**:
- Workflow parameters (template variables)
- Resource limits (CPU, memory, GPU)
- Conditional execution with when expressions
- ContinueOnFail and ContinueOnError behaviors
- Multiple wait strategies (log, port, HTTP, healthy)
- Parameter substitution in commands and environment

**Examples Included**:
1. **Parameterized Workflow**: Template variables ({{.version}}, {{.environment}}, {{.repo}})
2. **Resource Limits**: CPU/memory limits per container, GPU requests for ML workloads
3. **Conditional Execution**: Deploy only if tests pass, rollback on failure
4. **Wait Strategies**: Different container readiness strategies for various use cases

**Run**:
```bash
go run -tags example advanced.go
```

**Use Case**: Production-grade workflows with resource management, conditional logic, complex orchestration, and multi-environment deployments.

**Argo Workflow Equivalent**: Parameters, resource requirements, conditionals, retryStrategy

---

## Task Queue

All examples use the **`docker-tasks`** task queue. Make sure your worker is listening on this queue:

```go
w := worker.New(c, "docker-tasks", worker.Options{})
docker.RegisterAll(w)
```

## Viewing Workflow Execution

### 1. Temporal Web UI

Visit http://localhost:8233
- Navigate to "Workflows" in the sidebar
- Search by Workflow ID (shown in example output)
- View execution history, events, results, and timing
- Inspect input/output payloads
- Debug failures with stack traces

### 2. Using `tctl` CLI

```bash
# List recent workflows
tctl workflow list

# Show workflow details
tctl workflow show -w <workflow-id>

# Show workflow history
tctl workflow showid <workflow-id>

# Describe workflow
tctl workflow describe -w <workflow-id>
```

## Troubleshooting

### Worker Not Connecting to Temporal

**Error**: `Unable to create Temporal client` or `connection refused`

**Solution**:
```bash
# Check if Temporal server is running
docker ps | grep temporal

# Check Temporal logs
docker logs <temporal-container-id>

# Restart Temporal server
cd docker-compose && docker-compose restart

# Or restart dev server
temporal server start-dev
```

### Docker Permission Issues

**Error**: `permission denied while trying to connect to Docker daemon`

**Solution**:
```bash
# Add user to docker group (Linux)
sudo usermod -aG docker $USER
newgrp docker

# Verify Docker is running
docker ps

# Or run with sudo (not recommended for development)
sudo go run -tags example basic.go
```

### Build Tag Not Recognized

**Error**: `no Go files in /path/to/examples/docker`

**Solution**: Always use `-tags example` when building or running:
```bash
go run -tags example basic.go
# Not: go run basic.go

# For building
go build -tags example -o myexample basic.go
```

### Port Already in Use

**Error**: `bind: address already in use` or `port is already allocated`

**Solution**:
```bash
# Find processes using the port
lsof -i :5432  # or your port number
netstat -tulpn | grep :5432

# Stop conflicting containers
docker ps
docker stop <container-id>

# Or use different ports in your example
```

### Workflow Execution Timeout

**Error**: `workflow execution timeout` or `deadline exceeded`

**Solution**: Increase workflow execution timeout:
```go
client.StartWorkflowOptions{
    ID:                       "my-workflow",
    TaskQueue:                "docker-tasks",
    WorkflowExecutionTimeout: 10 * time.Minute, // Increase this
    WorkflowTaskTimeout:      1 * time.Minute,
}
```

### Missing Artifacts Directory

**Error**: `failed to create artifact store: no such file or directory`

**Solution**:
```bash
# Create artifacts directory
mkdir -p /tmp/workflow-artifacts
chmod 755 /tmp/workflow-artifacts

# Or use a different path in your example
store, err := artifacts.NewLocalFileStore("/path/to/your/artifacts")
```

### Container Image Pull Failures

**Error**: `failed to pull image` or `image not found`

**Solution**:
```bash
# Pull image manually first
docker pull alpine:latest
docker pull golang:1.23-alpine

# Check image availability
docker images

# Use explicit tags
Image: "postgres:16-alpine"  # Instead of "postgres:latest"
```

### Task Queue Mismatch

**Error**: Workflow starts but never completes, stuck in "Running" state

**Solution**: Ensure task queue names match:
```go
// In worker
w := worker.New(c, "docker-tasks", worker.Options{})

// In workflow execution
client.StartWorkflowOptions{
    TaskQueue: "docker-tasks",  // Must match worker
}
```

## Best Practices

1. **Always start the worker first** before running examples
2. **Use unique workflow IDs** to avoid conflicts (add timestamps if needed)
3. **Clean up resources** - examples use `AutoRemove: true` by default
4. **Monitor in Temporal Web UI** to understand execution flow and debug issues
5. **Use appropriate failure strategies**:
   - `fail_fast` for critical pipelines
   - `continue` for best-effort processing
6. **Set resource limits** for production workflows to prevent resource exhaustion
7. **Use artifacts** for passing files between steps (binaries, packages, reports)
8. **Use data passing** for passing values between steps (versions, IDs, flags)
9. **Use builder API** for complex workflows to improve readability
10. **Add wait strategies** to ensure containers are ready before proceeding

## Common Patterns

### Pattern 1: Build-Test-Deploy Pipeline
```go
// Use pipeline.go or dag.go as a template
// Sequential: pipeline.go
// With dependencies: dag.go
```

### Pattern 2: Matrix Builds
```go
// Use loop.go parameterized example
// Build for all combinations of OS, Go version, architecture
```

### Pattern 3: Batch Processing
```go
// Use loop.go batch processing example
// Process files with concurrency limits
```

### Pattern 4: Multi-Stage Deployment
```go
// Use advanced.go conditional execution
// Deploy with rollback on failure
```

## Example Workflow

### Complete Flow

1. **Terminal 1**: Start Temporal server
   ```bash
   cd /path/to/docker-compose
   docker-compose up
   # Or: temporal server start-dev
   ```

2. **Terminal 2**: Start worker
   ```bash
   cd examples/docker/worker
   go run -tags example main.go
   ```

3. **Terminal 3**: Run example
   ```bash
   cd examples/docker
   go run -tags example dag.go
   ```

4. **Browser**: Monitor execution
   ```
   Open http://localhost:8233
   Navigate to Workflows
   Find your workflow by ID
   Explore execution history
   ```

## Next Steps

- Review the [main docker package README](../../docker/README.md) for API documentation
- Explore the [source code](../../docker/) to understand implementation details
- Adapt examples to your specific use case
- Build custom workflows using the builder API
- Integrate with your CI/CD pipeline
- Contribute new examples or improvements via pull requests

## Support

For issues, questions, or contributions:
- **GitHub Issues**: https://github.com/jasoet/go-wf/issues
- **Temporal Documentation**: https://docs.temporal.io
- **Docker Documentation**: https://docs.docker.com

## License

See the main repository LICENSE file for licensing information.
