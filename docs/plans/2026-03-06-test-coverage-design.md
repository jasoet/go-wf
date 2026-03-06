# Test Coverage Improvement Design

**Date:** 2026-03-06
**Goal:** Increase overall test coverage from 81.4% to 85%+
**Approach:** Broad sweep — low-hanging fruit across four packages

## Current State

| Package | Coverage | Gap Type |
|---------|----------|----------|
| `docker/payload` | 28.1% | Missing validation tests |
| `docker/activity` | 15.8% | `StartContainerActivity` untested |
| `docker` (root) | 77.0% | Error paths in operations.go |
| `workflow/artifacts` | 76.7% | Error paths in local.go + activities.go |

## Package 1: `docker/payload` (28.1% -> ~75%)

**Test type:** Unit tests (no external dependencies)

### Missing Coverage

**Validate methods:**
- `LoopInput.Validate()` — nested template validation
- `ParameterizedLoopInput.Validate()` — parameter array checks, empty values
- `DAGWorkflowInput.Validate()` — empty nodes, dependency validation

**Struct validation (validation tags untested):**
- `Artifact` — required Name/Path, type enum (file/directory/archive)
- `SecretReference` — all required fields
- `OutputDefinition` — required Name/ValueFrom, valueFrom enum
- `InputMapping` — required Name/From
- `WorkflowParameter` — required Name/Value
- `DAGNode` — required Name/Container

### Approach

- Table-driven tests in `payloads_test.go` (extend) and `payloads_extended_test.go` (new)
- Each test covers: valid input, missing required fields, invalid enum values
- Custom validation logic paths (empty parameters, dependency checks)

## Package 2: `docker` operations (77.0% -> ~90%)

**Test type:** Unit tests with Temporal mock client (existing pattern)

### Missing Coverage

**`SubmitWorkflow`:**
- Pointer type variants (`*ContainerExecutionInput`, `*PipelineInput`, `*ParallelInput`)
- Default case for unsupported input types
- `ExecuteWorkflow` error path

**`SubmitAndWait`:**
- Error propagation from `SubmitWorkflow`
- `we.Get()` failure path

**`GetWorkflowStatus`:**
- `we.Get()` error path

**`WatchWorkflow`:**
- Context cancellation (`ctx.Done()`)
- `GetWorkflowStatus` error during polling

**`QueryWorkflow`:**
- Entire function (currently skipped — needs `converter.EncodedValue` mock)

### Approach

- Extend `operations_test.go` using existing mock client pattern
- Add error-returning mock behaviors for each path
- Implement `QueryWorkflow` test with proper `EncodedValue` mock

## Package 3: `docker/activity` (15.8% -> ~65%)

**Test type:** Integration tests (`//go:build integration`) with real Podman

### Missing Coverage

- `StartContainerActivity` — entire function (151 lines, 0% covered)

### Test Cases

1. **Happy path** — alpine `echo hello`, verify stdout, exit code 0, success=true
2. **Environment variables** — pass env vars, verify available in container
3. **Command/entrypoint** — custom command and entrypoint override
4. **Container failure** — exit non-zero, verify exit code and success=false
5. **Port exposure** — expose port, verify mapping in output
6. **Wait strategy** — log-based wait strategy
7. **AutoRemove** — verify cleanup behavior

### Approach

- New file: `activity_integration_test.go`
- Use testcontainers pattern consistent with existing integration tests
- Requires Temporal activity test environment or direct function invocation

## Package 4: `workflow/artifacts` (76.7% -> ~87%)

**Test type:** Unit tests with real filesystem operations

### Missing Coverage

**`local.go` error paths:**
- `NewLocalFileStore()` — MkdirAll failure
- `Upload()` — MkdirAll, OpenFile, io.Copy failures
- `Download()` — os.Open non-NotExist error
- `Delete()` — os.Remove non-NotExist error
- `Exists()` — os.Stat non-NotExist error
- `List()` — filepath.Rel failure, Walk error
- `ArchiveDirectory()` — tar header, file open, write failures
- `ExtractArchive()` — corrupt archive, gzip error, directory traversal prevention
- `extractFileFromArchive()` — io.Copy failure, close error

**`activities.go` error paths:**
- `uploadFile/uploadDirectory` — bad source paths
- `downloadFile/downloadDirectory` — MkdirAll, OpenFile failures
- `CleanupWorkflowArtifacts` — store.List failure, store.Delete failure

### Approach

- Extend `local_test.go` and `activities_test.go`
- Trigger errors via: non-existent paths, read-only directories, corrupt archive data
- No mocking — real filesystem operations
- Skip `minio.go` (requires MinIO container, already has integration tests)

## Files to Create/Modify

| File | Action |
|------|--------|
| `docker/payload/payloads_test.go` | Extend |
| `docker/payload/payloads_extended_test.go` | Create |
| `docker/operations_test.go` | Extend |
| `docker/activity/activity_integration_test.go` | Create |
| `workflow/artifacts/local_test.go` | Extend |
| `workflow/artifacts/activities_test.go` | Extend |

## Constraints

- All unit tests: no build tags, no external dependencies
- Integration tests: `//go:build integration` tag, requires Podman
- Table-driven tests preferred
- Use `testify` (assert + require)
- Follow existing code style and import conventions
- No mocking for filesystem or Docker — use real operations
- Temporal client mocking only where existing pattern requires it (operations.go)
