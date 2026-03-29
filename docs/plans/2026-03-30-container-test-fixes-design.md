# Container Test Fixes & Integration Test CI Workflow

**Date:** 2026-03-30
**Status:** Approved

## Overview

Two concerns addressed in this plan:
1. Fix issues found during container test code review (moderate scope)
2. Add optional GitHub Actions workflow for integration tests on local self-hosted runner with Podman

## Part 1: Test Code Fixes

### Changes

| # | File | Change | Rationale |
|---|------|--------|-----------|
| 1 | `container/container_test.go` | Remove `integrationMockWorker` and `TestWorkflowRegistration` | Duplicate — `worker_test.go` already tests this with proper mock assertions |
| 2 | `container/activity/container_test.go` | Remove `TestContainerExecutionOutput_Fields` | Tests Go struct assignment, not our code |
| 3 | `container/workflow/dag_test.go` | Remove `TestHelperFunctions` | Tests `strings.ReplaceAll` and `strings.Index` from stdlib |
| 4 | `container/workflow/dag_test.go` | Add `TestDAGWorkflowValidation` case for cyclic dependencies (A→B→A) | Missing coverage for cycle detection |
| 5 | `container/activity/container_test.go` | Strengthen `TestBuildWaitStrategy` — verify returned strategy is non-nil and test observable timeout behavior where possible | Currently only checks `!= nil` |
| 6 | `container/integration_test.go` | Append `t.Name()` or UUID suffix to workflow IDs | Prevent collisions on concurrent runs |
| 7 | `container/activity/container_test.go` | Add `TestParallelInput_NegativeConcurrency` validation test | Missing negative value validation |
| 8 | `container/workflow/loop_test.go` | Make `callCount` in sequential tests use `atomic.Int32` for consistency | Inconsistent pattern, fragile if tests are refactored to parallel |

### Out of Scope

- Timeout/context-deadline workflow tests (needs behavioral design decisions)
- Pinning `TestStartContainerActivity_NonZeroExit` behavior (ambiguous by design)

## Part 2: Integration Test CI Workflow

### Trigger

`workflow_dispatch` only (manual). Integration tests are slow and resource-heavy on the local runner.

### Input

`scope` dropdown with choices:
- `all` (default) — `./...`
- `container` — `./container/...`
- `function` — `./function/...`
- `datasync` — `./datasync/...`
- `store` — `./workflow/store/... ./workflow/artifacts/...`

### Runner

`[self-hosted, local, macOS, ARM64]` — same as existing CI.

### Workflow Steps

1. Checkout
2. `nix develop --command task container:check` — validate Podman, set `DOCKER_HOST` and `TESTCONTAINERS_RYUK_DISABLED`
3. Map scope input to Go package path(s)
4. Run: `nix develop --command go test -race -count=1 -tags=integration -timeout=15m <package-path>`

### Key Details

- Reuses existing `container:check` task for Podman detection (no duplicate logic)
- Environment variables from `container:check` exported to subsequent steps via `$GITHUB_ENV`
- No new Taskfile tasks needed — workflow composes existing primitives
- Workflow-level timeout: 20 minutes (buffer over 15m test timeout)

### File

`.github/workflows/integration-test.yml`
