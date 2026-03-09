# Demo Task Design

## Goal

Single `task demo` command that starts Temporal dev server, starts docker worker, runs all 16 examples sequentially, and cleans up. User watches workflows in Temporal UI at http://localhost:8233.

## Flow

1. Start `temporal server start-dev` in background (UI at http://localhost:8233)
2. Wait for port 7233 to be ready
3. Set `DOCKER_HOST` for podman socket
4. Start docker worker (`examples/docker/worker/main.go`) in background
5. Wait briefly for worker registration
6. Run function examples (5 files)
7. Run standalone docker examples (5 files: basic, pipeline, parallel, loop, builder)
8. Run worker-dependent docker examples (6 files: advanced, builder-advanced, dag, data-passing, operations, patterns-demo)
9. Clean up: kill worker, kill Temporal server
10. Trap SIGINT/EXIT so cleanup always runs

## Skipped

- `artifacts.go` — requires MinIO server

## Implementation

Single shell script in Taskfile `demo` task with trap-based cleanup.
