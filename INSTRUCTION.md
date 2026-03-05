# Project Instructions

<!-- This file is the single source of truth for AI assistants working on this project. -->
<!-- AI: Read this file at the start of every session. Update it when conventions, -->
<!-- architecture, or key paths change. Also keep README.md in sync. -->
<!-- If Obsidian MCP is available, check the vault for additional knowledge, best -->
<!-- practices, and reference notes. Ask the user for the vault path if needed. -->

## Project Overview

go-wf — a Go library providing reusable, production-ready Temporal workflows for common orchestration patterns. Primary focus is Docker container workflows with Argo Workflow-like capabilities. Built with Go 1.26+, uses `github.com/jasoet/pkg/v2` as the base library. Provides single container, pipeline, parallel, DAG, and loop workflows with a fluent builder API, container/script/HTTP templates, and lifecycle management.

**Repository Type:** Library (Go module)
**Module:** `github.com/jasoet/go-wf`
**Key Dependencies:** Temporal SDK, testcontainers-go, pkg/v2, validator/v10, minio-go

## ABSOLUTE RULE — Git Authorship

**NEVER add AI (Claude, Copilot, or any AI) as co-author, committer, or contributor in git commits.**
Only the user's registered email may appear in commits. This is company policy — commits with AI
authorship WILL BE REJECTED. Do not use `--author`, `Co-authored-by`, or any other mechanism to
attribute commits to AI. This applies to ALL commits, including those made by tools and subagents.

## Conventions

- **Python**: Always use `uv` (never pip, conda, pipenv). All scripts must be Python, no bash scripts.
- **Node.js**: Always use `bun`/`bunx` (never node, npm, npx).
- **Commands**: Always use `task <name>` to run commands. Run `task --list` to discover available tasks. If a command is important or repeated but has no task, suggest adding it to `Taskfile.yml` with a proper description.
- **Brainstorming**: When the user starts a new topic or plans something, always use the brainstorming skill first. If unsure whether to brainstorm, ask the user.
- **Superpowers**: Ensure superpowers skills are installed and use them. Brainstorm before features, TDD for implementation, systematic-debugging for bugs.
- **README.md**: Human-readable project documentation. Update when user-facing behavior changes.
- **INSTRUCTION.md**: AI-readable project context (this file). Update when project conventions, architecture, or key paths change.
- **Scripts**: For complex automation, create proper UV projects (Python) or Bun projects (TypeScript) with `pyproject.toml`/`package.json`. Always `.gitignore` generated files (`.venv/`, `node_modules/`, `__pycache__/`).
- **Containers**: Always use Podman (never Docker). Use `podman` and `podman-compose` instead of `docker` and `docker-compose`. Migrate any existing Docker references to Podman.
- **Commits**: Use [Conventional Commits](https://www.conventionalcommits.org/). Format: `<type>(<scope>): <description>`. Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`.
- **Branching**: Create a new branch for each feature or fix (`feat/...`, `fix/...`). Create PR (GitHub, use `gh`) or MR (GitLab, use `glab`) when ready. Always squash commits on merge. Use `gh`/`glab` to check PR/MR status and CI pipeline results before merging.

## Key Paths

| Path | Purpose |
|------|---------|
| `docker/` | Docker container workflows (main package) |
| `docker/activity/` | Temporal activities for container execution |
| `docker/artifacts/` | Artifact store (local + MinIO) |
| `docker/builder/` | Fluent builder API for workflows |
| `docker/errors/` | Error types and handling |
| `docker/patterns/` | Pre-built patterns (CI/CD, loop, parallel) |
| `docker/payload/` | Type-safe payload structs |
| `docker/template/` | Container, script, and HTTP templates |
| `docker/workflow/` | Workflow implementations (container, pipeline, parallel, DAG, loop) |
| `examples/docker/` | Example code (build tag: `//go:build example`) |
| `Taskfile.yml` | All project commands |
| `.claude/` | Claude Code hooks, scripts, and skills |
| `.github/workflows/` | GitHub Actions CI/CD |
| `INSTRUCTION.md` | AI context (this file) |
| `README.md` | Human documentation |

## Taskfile Commands

| Task | Description |
|------|-------------|
| `task` | List all available tasks |
| `task test` | Run all tests (unit + integration) with coverage — Docker required |
| `task test:unit` | Run unit tests only (fast, no Docker required) |
| `task lint` | Run golangci-lint |
| `task check` | Run all checks (test + lint) |
| `task tools` | Install development tools (golangci-lint, gofumpt, goimports) |
| `task fmt` | Format all Go files (goimports with local prefix + gofumpt) |
| `task clean` | Clean build artifacts |

## Architecture

Go module-based library organized as package-per-feature:

- **Workflows** register with Temporal workers via `docker.RegisterAll(w)`
- **Activities** wrap `github.com/jasoet/pkg/v2/docker` for container execution
- **Payloads** are validated structs (`go-playground/validator`) for workflow I/O
- **Builder** provides a fluent API to compose container → pipeline → parallel → DAG
- **Templates** (container, script, HTTP) generate payload structs from higher-level config
- **Patterns** are pre-built workflow compositions (CI/CD pipelines, fan-out/fan-in, etc.)

## Testing Strategy

- **Coverage target**: 85%+
- **Unit tests**: `*_test.go` — no build tags, no external dependencies, fast
- **Integration tests**: `*_integration_test.go` — `//go:build integration` tag, uses testcontainers, requires Docker/Podman
- **Example code**: `//go:build example` tag in `examples/`
- **Assertion library**: `github.com/stretchr/testify` (assert + require)
- **Pattern**: Table-driven tests preferred
- **Run unit**: `task test:unit`
- **Run all**: `task test` (includes integration, Docker required)

## Quality Standards

- Zero golangci-lint errors
- gofumpt formatting enforced
- goimports with local prefix `github.com/jasoet/go-wf` (3-group imports: stdlib, third-party, local)
- Cyclomatic complexity < 20
- Function length < 100 lines / 50 statements (`funlen`)
- Code duplication threshold: 100 tokens (`dupl`)
- Line length < 190 characters
- All exported functions documented
- Comments on declarations must end in a period (`godot`)
- Security: gosec scanning, no hardcoded secrets, directory permissions ≤ 0o750, file permissions ≤ 0o600

## Go: Base Library (pkg/v2)

All Go projects use `github.com/jasoet/pkg/v2` as the base library.
Read the following files from `~/Documents/Go/pkg/` for full details:
- `README.md`: Available packages and quick start
- `CLAUDE.md`: Architecture patterns, testing strategy, development commands
- `PROJECT_TEMPLATE.md`: Recommended project structure and wiring patterns

Key patterns:
- Functional Options for all configuration (`WithXxx()` functions)
- OTelConfig injected via functional options, never serialized (`yaml:"-" mapstructure:"-"`)
- LayerContext for automatic span + logger correlation
- No-op provider pattern: OTel gracefully defaults to no-op when nil
