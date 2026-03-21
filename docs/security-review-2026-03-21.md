# Security Review — go-wf

**Date:** 2026-03-21
**Scope:** Full codebase (workflow core, docker, function, examples, infrastructure)
**Focus:** Security vulnerabilities, code quality, operational risks

---

## Executive Summary

The codebase demonstrates solid engineering: clean architecture, good use of Go generics, proper Temporal SDK integration, comprehensive test coverage (85%+), and thoughtful OTel instrumentation. However, the review identified **5 critical**, **16 important**, and **12 suggestion-level** findings across security, reliability, and operational concerns.

The most severe issues are path traversal in the artifact store, shell injection via template substitution, missing panic recovery in function handlers, DAG cycle detection gaps, and unsanitized volume mount paths.

---

## Critical Issues

### C1. Path Traversal in LocalFileStore

**Files:** `workflow/artifacts/store.go:63-64`, `workflow/artifacts/local.go:35,67,82,96,111`

`StorageKey()` concatenates `WorkflowID`, `RunID`, `StepName`, and `Name` with no sanitization. Setting `WorkflowID` to `../../etc` and `Name` to `passwd` causes `filepath.Join(BasePath, key)` to resolve outside `BasePath`. The `#nosec` annotations claim "path is built from validated metadata" but no validation occurs.

**Impact:** Arbitrary file read/write/delete on the worker host.

**Recommendation:** Add path containment validation:
```go
func (s *LocalFileStore) safePath(metadata ArtifactMetadata) (string, error) {
    fullPath := filepath.Join(s.BasePath, metadata.StorageKey())
    absPath, _ := filepath.Abs(fullPath)
    absBase, _ := filepath.Abs(s.BasePath)
    if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) {
        return "", fmt.Errorf("path traversal detected")
    }
    return absPath, nil
}
```

### C2. Path Traversal / Object Key Injection in MinioStore

**File:** `workflow/artifacts/minio.go:205-210`

Same unsanitized `StorageKey()` used as S3 object key. An attacker controlling metadata fields can overwrite other workflows' artifacts (cross-tenant data access).

**Recommendation:** Validate that metadata fields contain only `[a-zA-Z0-9_.-]`.

### C3. Shell Injection via Template Substitution

**Files:** `docker/template/script.go:96`, `docker/template/http.go:120-128`, `workflow/helpers.go:38-54`

`SubstituteTemplate` performs raw string replacement. When items are substituted into `sh -c` commands, attacker-controlled values can inject shell commands. Example: item `; rm -rf /` in `process.sh {{item}}` becomes `process.sh ; rm -rf /`.

The HTTP template's single-quote escaping is incomplete — arguments are not actually quoted in the final curl command string.

**Impact:** Arbitrary command execution inside containers.

**Recommendation:** Pass user values exclusively through environment variables (Docker handles them safely), or apply proper shell escaping. For HTTP template, wrap each argument in single quotes.

### C4. DAG Cycle Detection Missing — Infinite Recursion

**Files:** `docker/payload/payloads_extended.go:186-206`, `docker/workflow/dag.go:63-70`

`DAGWorkflowInput.Validate()` only checks that dependency names reference existing nodes — it does NOT detect cycles. A→B→A causes infinite recursion in `executeDAGNode` (stack overflow / worker crash).

**Recommendation:** Implement topological sort or DFS-based cycle detection in `Validate()`.

### C5. No Panic Recovery in Function Handler Dispatch

**File:** `function/activity/function.go:53`

A registered handler that panics crashes the entire Temporal worker, affecting all concurrent activities and workflows. No `recover()` guard exists anywhere in the function package.

**Impact:** Single misbehaving handler causes denial-of-service for all workflows.

**Recommendation:** Wrap handler calls in deferred `recover()`.

---

## Important Issues

### I1. Volume Mount Path Traversal

**Files:** `docker/payload/payloads.go:30`, `docker/activity/container.go:51-53`

Volume paths are passed through with zero validation. Attacker-controlled inputs can mount `/etc`, `/var/run/docker.sock`, or `/root/.ssh` into containers.

**Recommendation:** Enforce allowlist of permitted base directories; reject sensitive paths.

### I2. MaxConcurrency Declared But Never Enforced

**Files:** `workflow/types.go:42,73,91`, `workflow/parallel.go:21-24`

All futures are launched simultaneously regardless of `MaxConcurrency` value. 10,000 tasks with `MaxConcurrency: 5` schedules all 10,000 activities at once.

**Impact:** Resource exhaustion on Temporal worker and downstream services.

**Recommendation:** Implement concurrency limiting or remove the misleading field.

### I3. No Stdout/Stderr Size Limits

**File:** `docker/activity/container.go:116-123`

Container output is captured entirely into strings with no size limit. A container producing gigabytes of output causes OOM on the worker.

**Recommendation:** Truncate to a configurable maximum (e.g., 1MB).

### I4. No Upload Size Limit in Artifact Store

**File:** `workflow/artifacts/local.go:54`

`Upload` copies entire `io.Reader` to disk with no size limit. Archive extraction has a 1GB cap, but regular uploads do not.

**Recommendation:** Add `io.LimitReader` wrapper.

### I5. Directory Archive Buffered Entirely in Memory

**File:** `workflow/artifacts/activities.go:87-101`

`var buf bytes.Buffer` holds the entire tar.gz in memory before uploading. Large directories cause OOM.

**Recommendation:** Use `io.Pipe()` to stream directly to the store.

### I6. ArchiveDirectory Follows Symlinks

**File:** `workflow/artifacts/local.go:176-217`

`filepath.Walk` follows symlinks. A symlink to `/etc/shadow` in the source directory gets included in the archive, leaking sensitive data.

**Recommendation:** Skip symlinks or use `filepath.WalkDir` with explicit symlink handling.

### I7. Predictable Workflow IDs

**File:** `docker/operations.go:33`

`fmt.Sprintf("docker-workflow-%d", time.Now().Unix())` — IDs are predictable (second precision) and collide if two workflows start in the same second.

**Recommendation:** Use UUID or add random component.

### I8. ResourceLimits Never Applied to Containers

**Files:** `docker/activity/container.go:26-67`, `docker/payload/payloads_extended.go:23-39`

`ResourceLimits` struct exists in payloads but is never wired into container execution. No CPU/memory limits, no `--no-new-privileges`, no capability dropping.

**Recommendation:** Wire `ResourceLimits` into Docker config; set safe defaults.

### I9. Registry Allows Silent Handler Overwrite

**File:** `function/registry.go:41-44`

`Register()` silently replaces existing handlers. Accidental or malicious name collision can redirect execution.

**Recommendation:** Return error on duplicate name or add `RegisterOrError` variant.

### I10. Global Mutable OTel Instrumenter

**File:** `function/worker.go:17-24`

`SetActivityInstrumenter` is a public setter for a package-level variable with no synchronization. Any code can replace the instrumenter at any time.

**Recommendation:** Use `sync.Once` to allow setting exactly once.

### I11. Mutable Sentinel Error Variables

**File:** `workflow/errors/errors.go:52-76`

Error sentinels are `var` pointer types — any consumer can mutate them (e.g., `ErrInvalidInput.Message = "changed"`).

**Recommendation:** Return fresh copies from accessor functions or make unexported.

### I12. Pipeline/Parallel StopOnError Wraps Nil Error

**Files:** `workflow/pipeline.go:37`, `workflow/parallel.go:39`

When `err == nil` but `result.IsSuccess() == false`, wrapping nil with `%w` produces `"pipeline stopped at step 2: <nil>"`.

**Recommendation:** Handle the two failure cases separately.

### I13. Compose Services Bound to 0.0.0.0

**File:** `compose.yml:4,20,40,50-51`

All services (PostgreSQL, Temporal, UI, MinIO) are accessible from any network interface without authentication.

**Recommendation:** Bind to localhost: `"127.0.0.1:5432:5432"` for all services.

### I14. Temporal Dev Server Bound to 0.0.0.0

**File:** `Taskfile.yml:257,347`

`temporal server start-dev --ip 0.0.0.0` exposes unauthenticated Temporal to the network.

**Recommendation:** Change to `--ip 127.0.0.1`.

### I15. Unpinned Container Image Tags

**File:** `compose.yml:18,38,49`

`temporalio/auto-setup:latest`, `temporalio/ui:latest`, `minio/minio` (no tag). Non-reproducible builds; supply chain risk.

**Recommendation:** Pin to specific versions.

### I16. CI Workflow Missing Permissions Block

**File:** `.github/workflows/ci.yml`

No `permissions:` block — gets default (potentially broad) repository permissions.

**Recommendation:** Add `permissions: { contents: read }`.

---

## Suggestions

| # | Area | Issue | Location |
|---|------|-------|----------|
| S1 | Artifact Store | `CleanupWorkflowArtifacts` stops on first error despite comment saying "continue" | `workflow/artifacts/activities.go:184-191` |
| S2 | Artifact Store | `downloadFile` silently swallows close errors (no named returns) | `workflow/artifacts/activities.go:129-153` |
| S3 | Artifact Store | Error messages leak filesystem paths | `workflow/artifacts/local.go:39,74,103` |
| S4 | Performance | Validator instance created per call (should reuse) | `workflow/types.go:19,48,79,97` |
| S5 | OTel | Wasted Cartesian product computation for log message | `workflow/otel.go:105` |
| S6 | Function | No validation on function name content (accepts `../`, null bytes) | `function/payload/payload.go:25` |
| S7 | Function | Template substitution in Name field can redirect handler dispatch | `function/workflow/loop.go:23` |
| S8 | Function | `BuildSingle` returns pointer to internal slice element | `function/builder/builder.go:152` |
| S9 | Docker | `WatchWorkflow` goroutine leak if consumer stops reading | `docker/operations.go:172-196` |
| S10 | Docker | Custom `replaceAll`/`indexOf` duplicate stdlib functions | `docker/workflow/dag.go:383-405` |
| S11 | Infra | `.gitignore` missing `.env` pattern | `.gitignore` |
| S12 | Infra | No `SECURITY.md` for vulnerability reporting | Project root |

---

## Positive Observations

- **Archive extraction is properly guarded** against path traversal with `filepath.Clean` and `HasPrefix` checks
- **Decompression bomb mitigation** in place with 1GB `LimitReader`
- **File permissions** are restrictive: directories `0750`, files `0600`
- **OTel zero-overhead bypass** — no allocations when disabled
- **Thread-safe registry** with proper `sync.RWMutex` usage
- **Clean error semantics** — activity errors (Temporal retries) vs handler errors (captured, no retry) well-designed
- **Build tag isolation** — all examples gated with `//go:build example`
- **CI release permissions** properly scoped with `persist-credentials: false`
- **Worker concurrency limits** set in examples (good practice to model)
- **Type safety through generics** prevents misuse at compile time
- **Comprehensive test coverage** including error paths and edge cases

---

## Priority Remediation Order

1. **C1+C2** — Path traversal in artifact store (add path containment validation)
2. **C3** — Shell injection in template substitution (use env vars or proper escaping)
3. **C5** — Panic recovery in function handlers (add `recover()`)
4. **C4** — DAG cycle detection (implement topological sort)
5. **I13+I14** — Bind services to localhost
6. **I1** — Volume mount validation
7. **I2** — MaxConcurrency enforcement or removal
8. **I3+I4+I5** — Size limits (stdout, upload, archive buffer)
9. **I15+I16** — Pin images, add CI permissions
10. Remaining important and suggestion items
