# Local Environment Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Create a local environment with persistent Temporal infrastructure (via Podman compose) and tooling to run, inspect, and schedule all example workflows via the Temporal UI.

**Architecture:** Podman compose runs PostgreSQL + Temporal + Temporal UI + MinIO. Native Go binaries serve as workers (docker-tasks, function-tasks queues). A Go trigger CLI submits workflows and creates schedules. Taskfile tasks orchestrate everything.

**Tech Stack:** Podman Compose, Temporal, PostgreSQL, MinIO, Go 1.26+

---

### Task 1: Create `compose.yml`

**Files:**
- Create: `compose.yml`

**Step 1: Write the compose file**

```yaml
services:
  postgresql:
    image: postgres:16
    environment:
      POSTGRES_USER: temporal
      POSTGRES_PASSWORD: temporal
      POSTGRES_DB: temporal
    ports:
      - "5432:5432"
    volumes:
      - postgresql-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U temporal"]
      interval: 5s
      timeout: 5s
      retries: 10

  temporal:
    image: temporalio/auto-setup:latest
    depends_on:
      postgresql:
        condition: service_healthy
    environment:
      - DB=postgres12
      - DB_PORT=5432
      - POSTGRES_USER=temporal
      - POSTGRES_PWD=temporal
      - POSTGRES_SEEDS=postgresql
      - DYNAMIC_CONFIG_FILE_PATH=config/dynamicconfig/development-sql.yaml
    ports:
      - "7233:7233"
    healthcheck:
      test: ["CMD", "temporal", "operator", "cluster", "health"]
      interval: 10s
      timeout: 5s
      retries: 20
      start_period: 30s

  temporal-ui:
    image: temporalio/ui:latest
    depends_on:
      temporal:
        condition: service_healthy
    environment:
      - TEMPORAL_ADDRESS=temporal:7233
      - TEMPORAL_CORS_ORIGINS=http://localhost:8233
    ports:
      - "8233:8080"

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio-data:/data
    healthcheck:
      test: ["CMD", "mc", "ready", "local"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgresql-data:
  minio-data:
```

**Step 2: Validate compose file**

Run: `podman-compose config`
Expected: Parsed YAML output with no errors.

**Step 3: Test services start**

Run: `podman-compose up -d`
Then wait: `podman-compose exec temporal temporal operator cluster health` (retry up to 60s)
Expected: Temporal cluster responds healthy, UI accessible at http://localhost:8233

Run: `podman-compose down`

**Step 4: Commit**

```bash
git add compose.yml
git commit -m "feat: add compose.yml for local Temporal infrastructure"
```

---

### Task 2: Create shared function worker — `examples/function/worker/main.go`

**Files:**
- Create: `examples/function/worker/main.go`

**Step 1: Write the shared function worker**

This consolidates all 18 handler functions from the 5 function examples into a single registry.

```go
//go:build example

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jasoet/pkg/v2/temporal"
	"go.temporal.io/sdk/worker"

	fn "github.com/jasoet/go-wf/function"
	fnactivity "github.com/jasoet/go-wf/function/activity"
	fnpayload "github.com/jasoet/go-wf/function/payload"
)

func main() {
	c, closer, err := temporal.NewClient(temporal.DefaultConfig())
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()
	if closer != nil {
		defer closer.Close()
	}

	log.Println("Starting Function Temporal Worker...")

	w := worker.New(c, "function-tasks", worker.Options{
		MaxConcurrentActivityExecutionSize:     10,
		MaxConcurrentWorkflowTaskExecutionSize: 10,
	})

	registry := fn.NewRegistry()
	registerAllHandlers(registry)

	fn.RegisterWorkflows(w)
	fn.RegisterActivity(w, fnactivity.NewExecuteFunctionActivity(registry))

	log.Println("Registered workflows:")
	log.Println("  - ExecuteFunctionWorkflow")
	log.Println("  - FunctionPipelineWorkflow")
	log.Println("  - ParallelFunctionsWorkflow")
	log.Println("  - LoopWorkflow")
	log.Println("  - ParameterizedLoopWorkflow")
	log.Println()
	log.Println("Registered activities:")
	log.Println("  - ExecuteFunctionActivity")
	log.Println()
	log.Println("Worker listening on task queue: function-tasks")

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}

	log.Println("Worker stopped")
}

func registerAllHandlers(registry *fn.Registry) {
	// --- basic.go handlers ---
	registry.Register("greet", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		name := input.Args["name"]
		if name == "" {
			name = "World"
		}
		return &fnpayload.FunctionOutput{
			Result: map[string]string{
				"greeting": fmt.Sprintf("Hello, %s!", name),
			},
		}, nil
	})

	// --- pipeline.go handlers ---
	registry.Register("validate", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		email := input.Args["email"]
		name := input.Args["name"]
		if email == "" || name == "" {
			return nil, fmt.Errorf("validation failed: email and name are required")
		}
		return &fnpayload.FunctionOutput{
			Result: map[string]string{
				"status":    "valid",
				"name":      name,
				"email":     email,
				"validated": time.Now().Format(time.RFC3339),
			},
		}, nil
	})

	registry.Register("transform", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		name := input.Args["name"]
		email := input.Args["email"]
		data := map[string]interface{}{
			"display_name": strings.Title(name),
			"slug":         strings.ToLower(strings.ReplaceAll(name, " ", "-")),
			"email":        email,
			"tier":         "standard",
		}
		jsonData, _ := json.Marshal(data)
		return &fnpayload.FunctionOutput{
			Result: map[string]string{
				"display_name": strings.Title(name),
				"slug":         strings.ToLower(strings.ReplaceAll(name, " ", "-")),
				"tier":         "standard",
			},
			Data: string(jsonData),
		}, nil
	})

	registry.Register("notify", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		channel := input.Args["channel"]
		if channel == "" {
			channel = "email"
		}
		name := input.Args["name"]
		log.Printf("Notification sent via %s to %s", channel, name)
		return &fnpayload.FunctionOutput{
			Result: map[string]string{
				"channel":   channel,
				"status":    "sent",
				"sent_at":   time.Now().Format(time.RFC3339),
				"recipient": name,
			},
		}, nil
	})

	// --- parallel.go handlers ---
	registry.Register("fetch-users", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		time.Sleep(500 * time.Millisecond)
		users := []map[string]string{
			{"id": "1", "name": "Alice"},
			{"id": "2", "name": "Bob"},
			{"id": "3", "name": "Charlie"},
		}
		jsonData, _ := json.Marshal(users)
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"count": "3", "source": "user-service"},
			Data:   string(jsonData),
		}, nil
	})

	registry.Register("fetch-orders", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		time.Sleep(700 * time.Millisecond)
		orders := []map[string]string{
			{"id": "101", "total": "$99.99"},
			{"id": "102", "total": "$149.99"},
		}
		jsonData, _ := json.Marshal(orders)
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"count": "2", "source": "order-service"},
			Data:   string(jsonData),
		}, nil
	})

	registry.Register("fetch-inventory", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		time.Sleep(300 * time.Millisecond)
		inventory := map[string]int{"widget-a": 150, "widget-b": 0, "widget-c": 75}
		jsonData, _ := json.Marshal(inventory)
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"total_skus": "3", "out_of_stock": "1", "source": "warehouse-service"},
			Data:   string(jsonData),
		}, nil
	})

	// --- loop.go handlers ---
	registry.Register("process-csv", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		file := input.Args["file"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"file": file, "status": "processed"},
		}, nil
	})

	registry.Register("run-migration", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		migration := input.Args["migration"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"migration": migration, "status": "applied"},
		}, nil
	})

	registry.Register("deploy-service", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		env := input.Args["environment"]
		region := input.Args["region"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"environment": env, "region": region, "status": "deployed"},
		}, nil
	})

	registry.Register("sync-tenant", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		tenant := input.Args["tenant"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"tenant": tenant, "status": "synced"},
		}, nil
	})

	registry.Register("health-check", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		service := input.Args["service"]
		env := input.Args["environment"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"service": service, "environment": env, "healthy": "true"},
		}, nil
	})

	// --- builder.go handlers ---
	registry.Register("extract", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		source := input.Args["source"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"records": "1500", "source": source},
		}, nil
	})

	// "transform" already registered from pipeline.go — builder.go uses a different transform.
	// Register as "etl-transform" to avoid conflict. The trigger will reference this name.
	registry.Register("etl-transform", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		format := input.Args["format"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"format": format, "records": "1480", "dropped": "20"},
		}, nil
	})

	registry.Register("load", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		target := input.Args["target"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"target": target, "loaded": "1480"},
		}, nil
	})

	registry.Register("validate-config", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		env := input.Args["env"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"env": env, "valid": "true"},
		}, nil
	})

	registry.Register("check-deps", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		service := input.Args["service"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"service": service, "healthy": "true"},
		}, nil
	})

	registry.Register("run-smoke-tests", func(ctx context.Context, input fnpayload.FunctionInput) (*fnpayload.FunctionOutput, error) {
		target := input.Args["target"]
		return &fnpayload.FunctionOutput{
			Result: map[string]string{"target": target, "passed": "12", "failed": "0"},
		}, nil
	})

	log.Printf("Registered %d handler functions", 18)
}
```

**Step 2: Test compilation**

Run: `cd examples/function/worker && go build -tags example -o /dev/null .`
Expected: Compiles successfully.

**Step 3: Commit**

```bash
git add examples/function/worker/main.go
git commit -m "feat: add shared function worker with all example handlers"
```

---

### Task 3: Create trigger CLI — `examples/trigger/main.go`

**Files:**
- Create: `examples/trigger/main.go`

**Step 1: Write the trigger CLI**

The trigger CLI has three subcommands: `run`, `schedule`, `clean`.

It needs to:
- Connect to Temporal at localhost:7233
- For `run`: submit one instance of each workflow type with sample inputs
- For `schedule`: create Temporal schedules for a subset of workflows
- For `clean`: delete all created schedules

**Docker workflow submissions** use `docker.SubmitWorkflow()` from `docker/operations.go`.
**Function workflow submissions** use the Temporal client directly with `client.ExecuteWorkflow()`.

Use lightweight inputs:
- Docker: `alpine` image with simple `echo` commands (fast, no external deps)
- Functions: reference handlers registered in the shared function worker

Each workflow gets a descriptive ID like `demo-docker-pipeline-<timestamp>` for easy identification in the UI.

**Schedule IDs** are fixed strings like `schedule-docker-pipeline`, `schedule-fn-pipeline`, etc. for idempotency.

The code is substantial (~400 lines). Key structure:

```go
//go:build example

package main

import (
	// temporal client, docker payloads, function payloads, patterns, templates, os, fmt, time
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: trigger <run|schedule|clean>")
		os.Exit(1)
	}
	// Connect to Temporal
	// Switch on os.Args[1]
}

func runAll(c client.Client) {
	ts := time.Now().Format("20060102-150405")
	// Submit docker workflows: basic, pipeline, parallel, loop, parameterized-loop, dag
	// Submit function workflows: basic, pipeline, parallel, loop, parameterized-loop
	// Use docker.SubmitWorkflow for docker, c.ExecuteWorkflow for functions
	// Print each workflow ID as submitted
}

func createSchedules(c client.Client) {
	// Create 4 schedules with fixed IDs
	// Use client.ScheduleClient().Create()
	// Skip if schedule already exists
}

func cleanSchedules(c client.Client) {
	// Delete schedules by fixed IDs
	// Ignore "not found" errors
}
```

Docker inputs use `template.NewContainer()` and `template.NewBashScript()` for clean payload creation. Function inputs use `fnpayload.FunctionExecutionInput`, `fnpayload.PipelineInput`, etc.

**Step 2: Test compilation**

Run: `cd examples/trigger && go build -tags example -o /dev/null .`
Expected: Compiles successfully.

**Step 3: Test with running infrastructure**

Run: `podman-compose up -d` (from Task 1)
Wait for Temporal readiness.
Start workers: `go run -tags example examples/docker/worker/main.go &` and `go run -tags example examples/function/worker/main.go &`
Run: `go run -tags example examples/trigger/main.go run`
Expected: All workflow IDs printed, workflows visible in Temporal UI at http://localhost:8233

Run: `go run -tags example examples/trigger/main.go schedule`
Expected: Schedules created, visible in Temporal UI schedules tab.

Run: `go run -tags example examples/trigger/main.go clean`
Expected: Schedules removed.

Stop workers and `podman-compose down`.

**Step 4: Commit**

```bash
git add examples/trigger/main.go
git commit -m "feat: add trigger CLI for submitting and scheduling workflows"
```

---

### Task 4: Add Taskfile `local:*` tasks

**Files:**
- Modify: `Taskfile.yml`

**Step 1: Add all local:* tasks**

Add the following tasks after the existing `demo:stop` task:

```yaml
  local:up:
    desc: Start local infrastructure (Temporal, PostgreSQL, MinIO)
    silent: true
    cmds:
      - |
        set -e
        echo "Starting local infrastructure..."
        podman-compose up -d
        echo "Waiting for Temporal (port 7233)..."
        for i in $(seq 1 60); do
          if nc -z localhost 7233 2>/dev/null; then
            echo "Temporal ready"
            echo ""
            echo "Services:"
            echo "  Temporal UI: http://localhost:8233"
            echo "  Temporal gRPC: localhost:7233"
            echo "  MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"
            echo "  PostgreSQL: localhost:5432 (temporal/temporal)"
            exit 0
          fi
          sleep 2
        done
        echo "Temporal failed to start within 120s"
        exit 1

  local:down:
    desc: Stop local infrastructure
    silent: true
    cmds:
      - podman-compose down

  local:clean:
    desc: Stop local infrastructure and remove volumes
    silent: true
    cmds:
      - podman-compose down -v

  local:workers:
    desc: Start docker and function workers in background
    silent: true
    env:
      DOCKER_HOST: '{{.CONTAINER_HOST}}'
    cmds:
      - |
        set -e
        PIDFILE="/tmp/go-wf-local.pids"

        if [ -f "$PIDFILE" ]; then
          echo "Workers already running (PID file exists). Run 'task local:workers:stop' first."
          exit 1
        fi

        echo "Starting docker worker..."
        cd examples/docker/worker && go run -tags example main.go > /tmp/go-wf-docker-worker.log 2>&1 &
        DOCKER_PID=$!
        cd - > /dev/null

        echo "Starting function worker..."
        cd examples/function/worker && go run -tags example main.go > /tmp/go-wf-function-worker.log 2>&1 &
        FUNCTION_PID=$!
        cd - > /dev/null

        sleep 3
        echo "$DOCKER_PID $FUNCTION_PID" > "$PIDFILE"

        echo "Workers started:"
        echo "  Docker worker PID: $DOCKER_PID (queue: docker-tasks)"
        echo "  Function worker PID: $FUNCTION_PID (queue: function-tasks)"
        echo "  Logs: /tmp/go-wf-docker-worker.log, /tmp/go-wf-function-worker.log"

  local:workers:stop:
    desc: Stop background workers
    silent: true
    cmds:
      - |
        PIDFILE="/tmp/go-wf-local.pids"

        if [ ! -f "$PIDFILE" ]; then
          echo "No workers running (no PID file found)."
          exit 0
        fi

        read DOCKER_PID FUNCTION_PID < "$PIDFILE"

        echo "Stopping workers..."
        [ -n "$DOCKER_PID" ] && kill $DOCKER_PID 2>/dev/null && echo "  Stopped docker worker (PID $DOCKER_PID)"
        [ -n "$FUNCTION_PID" ] && kill $FUNCTION_PID 2>/dev/null && echo "  Stopped function worker (PID $FUNCTION_PID)"

        rm -f "$PIDFILE"
        echo "Workers stopped."

  local:trigger:
    desc: Submit all example workflows once
    silent: true
    dir: examples/trigger
    cmds:
      - go run -tags example main.go run

  local:schedule:
    desc: Create recurring workflow schedules
    silent: true
    dir: examples/trigger
    cmds:
      - go run -tags example main.go schedule

  local:schedule:clean:
    desc: Remove all workflow schedules
    silent: true
    dir: examples/trigger
    cmds:
      - go run -tags example main.go clean

  local:start:
    desc: Start everything (infrastructure + workers + trigger + schedules)
    silent: true
    cmds:
      - task: local:up
      - task: local:workers
      - task: local:trigger
      - task: local:schedule
      - |
        echo ""
        echo "Local environment ready!"
        echo "  Temporal UI: http://localhost:8233"
        echo "  MinIO Console: http://localhost:9001"
        echo ""
        echo "Stop with: task local:stop"

  local:stop:
    desc: Stop everything (schedules + workers + infrastructure)
    silent: true
    cmds:
      - task: local:schedule:clean
      - task: local:workers:stop
      - task: local:down
```

**Step 2: Verify tasks are listed**

Run: `task --list`
Expected: All `local:*` tasks appear with descriptions.

**Step 3: Commit**

```bash
git add Taskfile.yml
git commit -m "feat: add local:* tasks for local environment management"
```

---

### Task 5: Update documentation

**Files:**
- Modify: `README.md` — add Local Environment section documenting `local:*` tasks
- Modify: `INSTRUCTION.md` — add new paths (`compose.yml`, `examples/function/worker/`, `examples/trigger/`), add `local:*` tasks to Taskfile Commands table

**Step 1: Update INSTRUCTION.md**

Add to Key Paths table:
```
| `compose.yml` | Podman compose for local Temporal infrastructure |
| `examples/function/worker/` | Shared function worker for all examples |
| `examples/trigger/` | Trigger CLI for submitting and scheduling workflows |
```

Add to Taskfile Commands table:
```
| `task local:up` | Start local infrastructure (Temporal, PostgreSQL, MinIO) |
| `task local:down` | Stop local infrastructure |
| `task local:clean` | Stop infrastructure and remove volumes |
| `task local:workers` | Start docker and function workers in background |
| `task local:workers:stop` | Stop background workers |
| `task local:trigger` | Submit all example workflows once |
| `task local:schedule` | Create recurring workflow schedules |
| `task local:schedule:clean` | Remove all workflow schedules |
| `task local:start` | Start everything (infra + workers + trigger + schedules) |
| `task local:stop` | Stop everything |
```

**Step 2: Update README.md**

Add a "Local Environment" section after the existing "Quick Demo" section, documenting:
- Prerequisites (Podman, podman-compose)
- `task local:start` / `task local:stop` one-liners
- Individual task breakdown for advanced usage
- Service URLs (Temporal UI, MinIO Console)

**Step 3: Commit**

```bash
git add INSTRUCTION.md README.md
git commit -m "docs: add local environment setup to README and INSTRUCTION"
```

---

### Task 6: End-to-end validation

**Step 1: Full lifecycle test**

Run: `task local:start`
Expected:
1. Podman containers start (postgresql, temporal, temporal-ui, minio)
2. Both workers start in background
3. All workflows submitted (IDs printed)
4. Schedules created
5. Final message with URLs

**Step 2: Verify in Temporal UI**

Open http://localhost:8233
- Check "Recent Workflows" — all submitted workflows should be visible
- Check individual workflow details — inputs, outputs, activity history
- Check "Schedules" tab — 4 recurring schedules visible
- Wait 10+ minutes — verify scheduled workflows execute

**Step 3: Clean shutdown**

Run: `task local:stop`
Expected: Schedules removed, workers stopped, containers stopped.

**Step 4: Verify persistence**

Run: `task local:up` then check Temporal UI — previous workflow history should still be visible.
Run: `task local:down`

**Step 5: Clean everything**

Run: `task local:clean`
Expected: Volumes removed, fresh state on next start.
