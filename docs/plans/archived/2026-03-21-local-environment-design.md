# Local Environment for Manual Workflow Inspection

**Date:** 2026-03-21
**Status:** Approved
**Goal:** Run all examples locally with persistent infrastructure so workflows can be inspected via Temporal UI.

## Problem

Currently, examples are run via `task demo` which uses an ephemeral Temporal dev server (no persistence) and runs examples sequentially with no way to explore them interactively. There's no shared function worker, no compose file, and no scheduling mechanism.

## Design

### Approach: Single Compose + Separate Go Binaries + Convenience Task

Infrastructure runs in Podman via compose. Workers and triggers run as native Go binaries for fast iteration. A convenience `task local:start` wraps everything into one command.

### Infrastructure — `compose.yml`

| Service | Image | Ports | Purpose |
|---------|-------|-------|---------|
| `postgresql` | `postgres:16` | 5432 | Temporal persistence backend |
| `temporal` | `temporalio/auto-setup` | 7233 | Workflow engine (auto-creates namespace, runs migrations) |
| `temporal-ui` | `temporalio/ui` | 8233 | Web UI for workflow inspection |
| `minio` | `minio/minio` | 9000, 9001 | S3-compatible artifact storage |

- PostgreSQL and MinIO data persisted via named volumes.
- Temporal service has a healthcheck; dependent tasks wait for port 7233.

### Shared Function Worker — `examples/function/worker/main.go`

- Mirrors `examples/docker/worker/main.go` pattern.
- Registers all function workflows via `fn.RegisterWorkflows(w)`.
- Creates a consolidated registry with all handler functions from all function examples.
- Listens on `function-tasks` queue.

### Trigger CLI — `examples/trigger/main.go`

Three subcommands:

- **`run`** — Submits all workflows once with sample inputs. Each gets a unique workflow ID (`demo-<type>-<name>-<timestamp>`). Prints IDs to stdout.
- **`schedule`** — Creates Temporal schedules for a subset of workflows (pipeline every 10m, parallel every 15m, function pipeline every 10m, loop every 20m). Idempotent via fixed schedule IDs.
- **`clean`** — Removes all created schedules.

Uses lightweight inputs (alpine containers, fast handlers) for quick completion.

### Taskfile Tasks

| Task | Description |
|------|-------------|
| `local:up` | `podman-compose up -d`, wait for Temporal readiness |
| `local:down` | `podman-compose down` |
| `local:clean` | `podman-compose down -v` (removes volumes) |
| `local:workers` | Start docker + function workers in background, save PIDs |
| `local:workers:stop` | Stop background workers |
| `local:trigger` | Submit all workflows once (`trigger run`) |
| `local:schedule` | Create recurring schedules (`trigger schedule`) |
| `local:schedule:clean` | Remove schedules (`trigger clean`) |
| `local:start` | One-command: `up` + `workers` + `trigger` + `schedule` |
| `local:stop` | One-command: `schedule:clean` + `workers:stop` + `down` |

Existing `demo:*` tasks remain unchanged.

### File Layout

```
go-wf/
├── compose.yml                          # NEW
├── examples/
│   ├── docker/worker/main.go            # EXISTS
│   ├── function/worker/main.go          # NEW
│   └── trigger/main.go                  # NEW
├── Taskfile.yml                         # MODIFIED (add local:* tasks)
├── README.md                            # MODIFIED (document local:* tasks)
└── INSTRUCTION.md                       # MODIFIED (add new paths)
```

No changes to library code. All new code in `examples/`.

## Decisions

- **Podman compose** over Temporal dev server — persistent workflow history, production-like setup.
- **`temporalio/auto-setup`** over separate admin-tools container — simpler, handles namespace + schema automatically.
- **Native Go binaries** for workers/triggers over containerized — faster iteration, no Dockerfile needed.
- **Go for trigger CLI** over Python — tightly coupled to Go workflow types and payloads.
- **Named volumes** for PostgreSQL and MinIO — data survives `local:down`, cleared with `local:clean`.
