# Docker → Container Rename Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rename the `container/` package to `container/` across the entire go-wf codebase for runtime-agnostic branding.

**Architecture:** Script-assisted big bang — a Python script handles directory moves and text replacements, followed by manual verification and a single commit.

**Tech Stack:** Python (via uv), Go (task test:unit, task lint)

**Design Doc:** `docs/plans/2026-03-26-docker-to-container-rename-design.md`

---

### Task 1: Create the rename script

**Files:**
- Create: `scripts/rename-docker-to-container/main.py`
- Create: `scripts/rename-docker-to-container/pyproject.toml`

**Step 1: Create the Python project**

Create `scripts/rename-docker-to-container/pyproject.toml`:
```toml
[project]
name = "rename-docker-to-container"
version = "0.1.0"
requires-python = ">=3.12"
```

**Step 2: Write the rename script**

Create `scripts/rename-docker-to-container/main.py` that does the following in order:

1. **Move directories** (using `shutil.move`):
   - `container/` → `container/`
   - `examples/container/` → `examples/container/`

2. **Rename file** (the test file that references the package name):
   - `container/docker_test.go` → `container/container_test.go`

3. **Text replacements in all `.go`, `.yml`, `.md` files** (excluding `.git/`, `go.sum`, and the design doc). Order matters — longer/more-specific patterns first to avoid double-replacing:

   | Pattern | Replacement | Context |
   |---------|-------------|---------|
   | `go-wf/container/` | `go-wf/container/` | Import paths |
   | `go-wf/docker"` | `go-wf/container"` | Import paths (no trailing slash) |
   | `"docker-tasks"` | `"container-tasks"` | Task queue name |
   | `"docker-queue"` | `"container-queue"` | Task queue name (tests) |
   | `"docker-workflow-"` | `"container-workflow-"` | Workflow ID prefix |
   | `go_wf.docker.task` | `go_wf.container.task` | OTel metric names |
   | `go-wf/container/activity` | `go-wf/container/activity` | OTel meter scope (as string literal) |
   | `recordDockerMetrics` | `recordContainerMetrics` | Function name |
   | `TestRecordDockerMetrics` | `TestRecordContainerMetrics` | Test function name |
   | `dockerMeterScope` | `containerMeterScope` | Constant name |
   | `dockerTaskTotal` | `containerTaskTotal` | Constant name |
   | `dockerTaskDuration` | `containerTaskDuration` | Constant name |

4. **Containerfile replacements:**

   | Pattern | Replacement |
   |---------|-------------|
   | `docker-worker-build` | `container-worker-build` |
   | `docker-worker` | `container-worker` |
   | `/docker-worker` | `/container-worker` |
   | `./examples/container/worker/` | `./examples/container/worker/` |

5. **compose.yml replacements:**

   | Pattern | Replacement |
   |---------|-------------|
   | `docker-worker:` | `container-worker:` |
   | `target: docker-worker` | `target: container-worker` |
   | `- docker` (profile) | `- container` |

6. **Taskfile.yml replacements:**

   | Pattern | Replacement |
   |---------|-------------|
   | `example:docker:worker` | `example:container:worker` |
   | `example:docker` | `example:container` |
   | `docker example` | `container example` |
   | `Docker examples` | `Container examples` |
   | `Docker worker` | `Container worker` |
   | `--profile docker` | `--profile container` |

7. **Documentation replacements** (INSTRUCTION.md, README.md, container/README.md → container/README.md):
   - `container/` (as path reference) → `container/`
   - `examples/container/` → `examples/container/`
   - But **preserve** references to:
     - `pkg/v2/docker` (external dependency)
     - Docker/Podman as runtime names (e.g., "Docker daemon", "docker socket")
     - `DOCKER_HOST` environment variable
     - `/var/run/docker.sock` socket path

**Important exclusions the script must handle:**
- Do NOT replace `pkg/v2/docker` imports (the external dependency)
- Do NOT replace `DOCKER_HOST` env var references
- Do NOT replace `/var/run/docker.sock` socket path references
- Do NOT replace `dockerpkg` import alias
- Do NOT replace `docker.sock` references
- Do NOT modify files in `.git/` or `go.sum`
- Do NOT modify the design doc itself

**Step 3: Run the script**

```bash
cd /Users/jasoet/Documents/Go/go-wf
uv run scripts/rename-docker-to-container/main.py
```

---

### Task 2: Verify the rename

**Step 1: Check directories exist**

```bash
ls container/ examples/container/
# Should exist
ls container/ examples/container/
# Should NOT exist (error expected)
```

**Step 2: Check no stale import paths remain**

```bash
grep -r "go-wf/docker" --include="*.go" .
# Should return nothing (or only go.sum which we skip)
```

**Step 3: Check external dep references preserved**

```bash
grep -r "pkg/v2/docker" --include="*.go" .
# Should still have hits (these should NOT have been renamed)
grep -r "DOCKER_HOST" .
# Should still have hits
grep -r "docker.sock" .
# Should still have hits
```

**Step 4: Run unit tests**

```bash
task test:unit
```

Expected: All tests pass.

**Step 5: Run linter**

```bash
task lint
```

Expected: No errors.

---

### Task 3: Update documentation

**Step 1: Verify INSTRUCTION.md**

Read INSTRUCTION.md and ensure:
- Key paths table references `container/` not `container/`
- Task descriptions reference `container` not `container`
- No broken references

**Step 2: Verify README.md**

Read README.md and ensure:
- Import paths show `go-wf/container/...`
- Examples reference `examples/container/`
- Task queue name is `container-tasks`

**Step 3: Manual fixes**

Fix any references the script missed or over-replaced. The script handles most cases, but documentation prose may need manual review for sentences like "The docker package..." → "The container package...".

---

### Task 4: Update .gitignore and cleanup

**Step 1: Add script artifacts to .gitignore**

Ensure `scripts/rename-docker-to-container/.venv/` and `__pycache__/` are gitignored.

**Step 2: Clean up the script**

After successful rename, the script is a one-time tool. Either:
- Delete `scripts/rename-docker-to-container/` before committing
- Or keep it for reference

---

### Task 5: Commit

**Step 1: Stage all changes**

```bash
git add -A
git status
```

Review the diff to ensure:
- `container/` deleted, `container/` added
- `examples/container/` deleted, `examples/container/` added
- All import paths updated
- No accidental changes to `pkg/v2/docker` references

**Step 2: Commit**

```bash
git commit -m "refactor: rename docker package to container for runtime-agnostic branding"
```

**Step 3: Verify**

```bash
task test:unit
task lint
```

Expected: All pass.
