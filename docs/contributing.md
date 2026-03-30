# Contributing Guide

This guide covers development setup, conventions, and workflow for contributing to go-wf.

## Development Setup

### Prerequisites

- **Go 1.26+**
- **[Task](https://taskfile.dev/)** -- all commands run through `task <name>`
- **Podman** (or Docker) -- required for integration tests
- **Git** with access to push branches

### Option A: Nix Development Environment (Recommended)

The project includes a `flake.nix` that provides all development tools (Go, golangci-lint, gofumpt, gosec, gotools, jq).

```bash
# Enter the dev shell
nix develop

# Or use direnv for automatic activation
# Add "use flake" to .envrc
echo "use flake" > .envrc
direnv allow
```

All Taskfile commands run inside `nix develop -c` automatically, so you do not need to manually enter the shell to use `task`.

### Option B: Manual Setup

Install tools individually:

```bash
# Install Go 1.26+
# Install Task: https://taskfile.dev/installation/
# Install Podman: https://podman.io/getting-started/installation

# Install Go dev tools
task tools
```

### Verify Setup

```bash
task nix:check    # Verify all tools are available
task lint         # Verify linter works
task test:unit    # Run unit tests (no container engine needed)
```

## Available Tasks

Run `task --list` to see all commands. Key tasks:

| Task | Description |
|------|-------------|
| `task test:unit` | Run unit tests (fast, no container engine) |
| `task test:integration` | Run integration tests (container engine required) |
| `task test` | Run all tests with coverage |
| `task lint` | Run golangci-lint |
| `task fmt` | Format code (goimports + gofumpt) |
| `task check` | Run all checks (test + lint) |
| `task test:pkg -- ./path/...` | Test a specific package |
| `task test:run -- -run TestName ./path/...` | Run a specific test |
| `task test:coverage -- ./path/...` | Show coverage for a package |
| `task demo:start` | Start Temporal + workers for running examples |
| `task demo:stop` | Stop demo environment |
| `task local:up` | Start full local infrastructure via podman-compose |
| `task local:down` | Stop local infrastructure |

## Code Conventions

### Tool Requirements

- **Commands**: Always use `task <name>` -- never run raw `go test`, `golangci-lint`, etc. directly.
- **Containers**: Always use `podman` / `podman-compose` (never `docker` / `docker-compose`).
- **Python** (if needed): Always use `uv` (never pip, conda, pipenv).
- **Node.js** (if needed): Always use `bun` / `bunx` (never npm, npx).

### Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`

Examples:
```
feat(datasync): add batch processing support
fix(container): handle nil payload in pipeline workflow
test(function): add unit tests for DAG builder
docs: update contributing guide
```

### Branching and PRs

1. Create a branch per change: `feat/add-batch-support`, `fix/nil-payload-handling`
2. Make focused, atomic commits
3. Create a PR with `gh pr create`
4. Ensure CI passes (lint + unit tests)
5. Squash merge into `main`

### Code Quality

- **Zero** golangci-lint errors
- **gofumpt** formatting enforced
- **goimports** with local prefix `github.com/jasoet/go-wf` (three-group imports: stdlib, third-party, local)
- Cyclomatic complexity < 20
- Function length < 100 lines / 50 statements
- All exported functions must be documented
- Comments on declarations must end in a period
- Line length < 190 characters

Always run before pushing:

```bash
task fmt     # Auto-format code
task check   # Run all tests + lint
```

## Testing

### Unit vs Integration Tests

- **Unit tests** (`*_test.go`): No build tags, no external dependencies, fast. Use Temporal `TestWorkflowEnvironment` with mocked activities.
- **Integration tests** (`*_integration_test.go`): Use `//go:build integration` tag. Require a container engine (Podman/Docker) for testcontainers.
- **Example code** (`examples/`): Use `//go:build example` tag.

### Coverage Targets

- **Minimum**: 80%
- **Goal**: 85%+

### Running Tests

```bash
task test:unit          # Fast, no container engine needed
task test:integration   # Requires Podman/Docker
task test               # All tests with coverage report
```

For detailed testing guidance (table-driven tests, mocking patterns, testcontainers setup, best practices), see [TESTING.md](../TESTING.md).

## PR Workflow

1. **Branch**: Create from `main` with a descriptive name
   ```bash
   git checkout -b feat/my-feature main
   ```

2. **Develop**: Write code, tests, and format
   ```bash
   task fmt
   task test:unit
   task lint
   ```

3. **Push and create PR**:
   ```bash
   git push -u origin feat/my-feature
   gh pr create --title "feat(scope): description" --body "Summary of changes"
   ```

4. **CI checks**: The CI pipeline runs lint and unit tests on all PRs. Both must pass.

5. **Merge**: Squash merge into `main` once approved and CI is green.

## CI Pipeline

The GitHub Actions CI pipeline (`.github/workflows/ci.yml`) runs on:
- Pushes to `main`
- Pull requests targeting `main`

Steps:
1. **Lint** -- `task lint` via Nix
2. **Test** -- `task ci:test` (unit tests, no coverage HTML)

## Project Structure

See `INSTRUCTION.md` for the full architecture overview and key paths. The codebase follows a package-per-feature layout:

- `workflow/` -- Generic workflow core (interfaces, orchestration)
- `container/` -- Container-based workflow implementation
- `function/` -- Go function workflow implementation
- `datasync/` -- Data synchronization pipeline implementation
- `examples/` -- Example code (build tag: `example`)
