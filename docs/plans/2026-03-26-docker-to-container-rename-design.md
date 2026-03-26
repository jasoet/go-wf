# Design: Rename `docker/` to `container/`

**Date:** 2026-03-26
**Status:** Approved
**Type:** Branding rename

## Motivation

The `docker/` package works with any OCI container runtime (Docker, Podman) via the Docker-compatible API. The name "docker" is misleading since the project convention is to use Podman. Renaming to `container/` better reflects the runtime-agnostic nature of the code.

## Scope

- **go-wf only** — `github.com/jasoet/pkg/v2/docker` stays as-is
- Full branding rename: dirs, imports, types, task queues, OTel metrics, examples, docs

## What Gets Renamed

| Category | From | To |
|----------|------|----|
| Package dir | `docker/` | `container/` |
| Examples dir | `examples/docker/` | `examples/container/` |
| Import paths | `go-wf/docker/...` | `go-wf/container/...` |
| Task queue | `"docker-tasks"` | `"container-tasks"` |
| OTel scope | `"go-wf/docker/activity"` | `"go-wf/container/activity"` |
| OTel metrics | `"go_wf.docker.task.*"` | `"go_wf.container.task.*"` |
| Function names | `recordDockerMetrics` | `recordContainerMetrics` |
| Workflow ID prefix | `"docker-workflow-"` | `"container-workflow-"` |
| Comments/docs | "docker" package refs | "container" package refs |
| compose.yml | `docker-worker` service | `container-worker` service |
| Taskfile.yml | docker task names | container task names |
| INSTRUCTION.md / README.md | docker references | container references |

## What Stays the Same

- Import alias for `pkg/v2/docker` (still `dockerpkg`)
- References to actual Docker/Podman daemon/runtime (e.g., "docker daemon not running")
- The underlying Docker client SDK usage
- Type names already using "Container" prefix (e.g., `ContainerExecutionInput`)

## Approach

Script-assisted big bang (Approach C):
1. Python script automates: directory move + file content replacements
2. Verify with `task test:unit` + `task lint`
3. Single commit
