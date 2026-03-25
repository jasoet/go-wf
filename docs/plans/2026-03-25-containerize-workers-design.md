# Containerize Example Workers Design

**Date:** 2026-03-25
**Status:** Approved

## Problem

Workers and trigger run via `go run` with PID file management. This requires a local Go toolchain, creates stale process issues, and doesn't match production deployment patterns.

## Decisions

- Single multi-stage Containerfile at repo root, distroless runtime images
- 3 new compose services: docker-worker, function-worker, trigger
- Docker worker mounts host container socket for container execution
- Trigger is a one-shot service with `run-all` subcommand (runs workflows + creates schedules, then exits)
- Workers read config from env vars (TEMPORAL_HOST_PORT, MINIO_ENDPOINT, etc.) with localhost fallbacks
- Taskfile simplified to use compose for everything
- Keep backward-compat aliases (local:start → local:up, local:stop → local:down)

## Changes

### Containerfile (repo root)

Multi-stage build:
- Shared builder stage (golang:1.26, go mod download, copy source)
- 3 build targets: docker-worker, function-worker, trigger
- 3 runtime images: distroless/static with single binary each
- Build with `-tags example` and `CGO_ENABLED=0`

### compose.yml — 3 new services

**docker-worker:** depends on temporal (healthy), mounts /var/run/docker.sock, env TEMPORAL_HOST_PORT=temporal:7233

**function-worker:** depends on temporal (healthy) + minio (healthy), env TEMPORAL_HOST_PORT=temporal:7233, MINIO_ENDPOINT=minio:9000, MINIO_ACCESS_KEY/SECRET_KEY=minioadmin

**trigger:** depends on docker-worker + function-worker (started), command: run-all, env TEMPORAL_HOST_PORT=temporal:7233. One-shot, exits after submitting workflows + creating schedules.

### Worker code changes

- `examples/docker/worker/main.go` — read TEMPORAL_HOST_PORT from env
- `examples/function/worker/main.go` — read TEMPORAL_HOST_PORT, MINIO_ENDPOINT, MINIO_ACCESS_KEY, MINIO_SECRET_KEY from env
- `examples/trigger/main.go` — read TEMPORAL_HOST_PORT from env, add `run-all` subcommand

### Taskfile updates

- `local:up` → `podman-compose up -d` (starts everything)
- `local:down` → `podman-compose down`
- `local:clean` → `podman-compose down -v`
- `local:build` → `podman-compose build` (new)
- `local:logs` → `podman-compose logs -f` (new)
- `local:trigger` → `podman-compose run --rm trigger run`
- `local:schedule` → `podman-compose run --rm trigger schedule`
- `local:schedule:clean` → `podman-compose run --rm trigger clean`
- `local:start` → alias for local:up
- `local:stop` → alias for local:down
- Remove: local:workers, local:workers:stop (PID-based management gone)

## Out of Scope

- CI image builds / registry push
- Production deployment configs
