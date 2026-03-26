# Artifact Storage for Function Module Design

**Date:** 2026-03-25
**Status:** Approved
**Approach:** All-at-once (core + docker refactor + function support + examples)

## Problem

- Function module has no artifact storage support — functions can't pass large data (files, reports, datasets) between DAG nodes
- Docker DAG uses `interface{}` for `ArtifactStore` with runtime type assertions — should use proper typed field
- No byte-based convenience helpers for in-memory data
- No examples demonstrate artifact flow

## Decisions

- Add artifact support to function DAG, matching docker's declarative pattern (InputArtifacts/OutputArtifacts on nodes)
- Both byte-based (for in-memory) and file-based (for large data) artifact operations
- Byte helpers as standalone functions (no interface change)
- Refactor docker `interface{}` to `artifacts.ArtifactStore` proper type
- Shared `ArtifactRef` type in `workflow/artifacts` (used by both docker and function)
- Worker-injected store (not serialized through Temporal) — same pattern as docker
- Examples with both LocalFile and MinIO backends
- `ArtifactStore` is optional (nil = skip artifacts)
- Artifacts for non-DAG patterns deferred to future work

## Changes

### Core — Byte Helpers + Shared Types

**`workflow/artifacts/helpers.go`**
- `UploadBytes(ctx, store ArtifactStore, metadata ArtifactMetadata, data []byte) error`
- `DownloadBytes(ctx, store ArtifactStore, metadata ArtifactMetadata) ([]byte, error)`

**`workflow/artifacts/store.go`**
- Add `ArtifactRef` struct: Name, Path, Type, Optional — shared reference type for artifact declarations on nodes

### Docker Refactor

**`container/payload/payloads_extended.go`**
- `ArtifactStore interface{}` → `ArtifactStore artifacts.ArtifactStore` (json:"-")

**`container/workflow/dag.go`**
- Remove `store, ok := input.ArtifactStore.(artifacts.ArtifactStore)` type assertions
- Use `input.ArtifactStore` directly

### Function DAG Artifacts

**`function/payload/payload_extended.go`**
- Add `ArtifactStore artifacts.ArtifactStore` (json:"-") to `DAGWorkflowInput`
- Add `InputArtifacts []artifacts.ArtifactRef` and `OutputArtifacts []artifacts.ArtifactRef` to `FunctionDAGNode`

**`function/workflow/dag.go`**
- Before activity execution: download input artifacts (skip if store nil or no input artifacts)
- After activity execution: upload output artifacts (skip if store nil, no output artifacts, or node failed)

### Examples

**Worker handlers** (3 new):
- `generate-report` — produces []byte data (simulated report)
- `process-report` — reads Data, transforms, outputs new Data
- `archive-report` — reads Data, returns summary

**Worker setup:**
- Create LocalFile and MinIO stores
- Register wrapped DAG workflows that inject the stores

**Trigger:**
- `demo-fn-dag-artifact-local-{ts}` — DAG with LocalFile store
- `demo-fn-dag-artifact-minio-{ts}` — DAG with MinIO store
- `schedule-fn-dag-artifact` (2 min) — MinIO-backed schedule

## Out of Scope

- Artifacts for non-DAG patterns (pipeline, parallel, loop)
- New storage backends (Redis, database, document storage)
- Docker DAG examples with artifacts
