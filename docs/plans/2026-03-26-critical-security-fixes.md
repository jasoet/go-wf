# Critical Security Fixes Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the 5 critical security vulnerabilities (C1-C5) identified in the security review.

**Architecture:** Each fix is isolated to a specific package. C1+C2 share a common validation function in `workflow/artifacts/`. C3 adds shell escaping to `workflow/helpers.go` and fixes HTTP template quoting. C4 ports DFS cycle detection from `function/payload/` to `container/payload/`. C5 adds `recover()` in `function/activity/`.

**Tech Stack:** Go 1.26+, testify (assert+require), Temporal SDK

---

### Task 1: C1+C2 — Path Traversal Protection in Artifact Store

**Files:**
- Modify: `workflow/artifacts/store.go` (add `ValidateMetadata` and `SafeStorageKey`)
- Modify: `workflow/artifacts/local.go` (add `safePath` method, update all methods)
- Modify: `workflow/artifacts/minio.go` (add key validation in `objectName`)
- Test: `workflow/artifacts/local_test.go` (add path traversal tests)

**Step 1: Write failing tests for path traversal detection**

Add to `workflow/artifacts/local_test.go`:

```go
func TestValidateMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata ArtifactMetadata
		wantErr  bool
	}{
		{
			name: "valid metadata",
			metadata: ArtifactMetadata{
				Name: "output.tar", WorkflowID: "wf-123", RunID: "run-456", StepName: "build",
			},
			wantErr: false,
		},
		{
			name: "path traversal in WorkflowID",
			metadata: ArtifactMetadata{
				Name: "file", WorkflowID: "../../etc", RunID: "run", StepName: "step",
			},
			wantErr: true,
		},
		{
			name: "path traversal in RunID",
			metadata: ArtifactMetadata{
				Name: "file", WorkflowID: "wf", RunID: "../..", StepName: "step",
			},
			wantErr: true,
		},
		{
			name: "path traversal in StepName",
			metadata: ArtifactMetadata{
				Name: "file", WorkflowID: "wf", RunID: "run", StepName: "../../secret",
			},
			wantErr: true,
		},
		{
			name: "path traversal in Name",
			metadata: ArtifactMetadata{
				Name: "../passwd", WorkflowID: "wf", RunID: "run", StepName: "step",
			},
			wantErr: true,
		},
		{
			name: "null byte in Name",
			metadata: ArtifactMetadata{
				Name: "file\x00.txt", WorkflowID: "wf", RunID: "run", StepName: "step",
			},
			wantErr: true,
		},
		{
			name: "empty WorkflowID",
			metadata: ArtifactMetadata{
				Name: "file", WorkflowID: "", RunID: "run", StepName: "step",
			},
			wantErr: true,
		},
		{
			name: "dots and hyphens allowed",
			metadata: ArtifactMetadata{
				Name: "output-v1.0.tar.gz", WorkflowID: "wf-123", RunID: "run-456", StepName: "build-step",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMetadata(tt.metadata)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLocalFileStore_PathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewLocalFileStore(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	malicious := ArtifactMetadata{
		Name: "passwd", WorkflowID: "../../etc", RunID: "run", StepName: "step",
	}

	t.Run("upload rejects traversal", func(t *testing.T) {
		err := store.Upload(ctx, malicious, bytes.NewReader([]byte("evil")))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid metadata")
	})

	t.Run("download rejects traversal", func(t *testing.T) {
		_, err := store.Download(ctx, malicious)
		assert.Error(t, err)
	})

	t.Run("delete rejects traversal", func(t *testing.T) {
		err := store.Delete(ctx, malicious)
		assert.Error(t, err)
	})

	t.Run("exists rejects traversal", func(t *testing.T) {
		_, err := store.Exists(ctx, malicious)
		assert.Error(t, err)
	})
}
```

**Step 2: Run tests to verify they fail**

Run: `task test:pkg -- ./workflow/artifacts/...`
Expected: FAIL — `ValidateMetadata` undefined, no validation in store methods

**Step 3: Implement ValidateMetadata in store.go**

Add to `workflow/artifacts/store.go`:

```go
import (
	"fmt"
	"regexp"
	"strings"
)

// safeNamePattern allows alphanumeric, hyphens, underscores, dots.
var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// ValidateMetadata checks that metadata fields are safe for use as path/key components.
// Rejects path traversal sequences, null bytes, and empty required fields.
func ValidateMetadata(m ArtifactMetadata) error {
	fields := map[string]string{
		"WorkflowID": m.WorkflowID,
		"RunID":      m.RunID,
		"StepName":   m.StepName,
		"Name":       m.Name,
	}
	for name, value := range fields {
		if value == "" {
			return fmt.Errorf("invalid metadata: %s must not be empty", name)
		}
		if strings.ContainsAny(value, "/\\\x00") {
			return fmt.Errorf("invalid metadata: %s contains forbidden characters", name)
		}
		if strings.Contains(value, "..") {
			return fmt.Errorf("invalid metadata: %s contains path traversal sequence", name)
		}
		if !safeNamePattern.MatchString(value) {
			return fmt.Errorf("invalid metadata: %s contains invalid characters", name)
		}
	}
	return nil
}
```

**Step 4: Add validation to LocalFileStore methods**

In `workflow/artifacts/local.go`, add validation at the start of Upload, Download, Delete, Exists:

```go
func (s *LocalFileStore) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}
	// ... existing code
}
```

Apply the same pattern to Download, Delete, Exists. Remove the unjustified `#nosec` annotations.

**Step 5: Add validation to MinioStore objectName**

In `workflow/artifacts/minio.go`, add validation to Upload, Download, Delete, Exists (same pattern as LocalFileStore).

**Step 6: Run tests to verify they pass**

Run: `task test:pkg -- ./workflow/artifacts/...`
Expected: PASS

**Step 7: Run full unit test suite to check for regressions**

Run: `task test:unit`
Expected: PASS

**Step 8: Commit**

```
git add workflow/artifacts/store.go workflow/artifacts/local.go workflow/artifacts/minio.go workflow/artifacts/local_test.go
git commit -m "fix(artifacts): add path traversal protection to artifact store

Validate metadata fields before use as filesystem paths or object keys.
Rejects path traversal sequences, null bytes, and invalid characters.
Removes unjustified #nosec annotations."
```

---

### Task 2: C3 — Shell Injection Protection in Template Substitution

**Files:**
- Modify: `workflow/helpers.go` (add `ShellEscape` function)
- Modify: `container/template/http.go` (quote curl arguments properly)
- Test: `workflow/helpers_test.go` (add shell escape tests)
- Test: `container/template/template_test.go` (add injection tests)

**Step 1: Write failing tests for shell escaping**

Add to `workflow/helpers_test.go`:

```go
func TestShellEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple string", "hello", "'hello'"},
		{"string with spaces", "hello world", "'hello world'"},
		{"string with single quotes", "it's here", "'it'\\''s here'"},
		{"injection attempt", "; rm -rf /", "'; rm -rf /'"},
		{"command substitution", "$(whoami)", "'$(whoami)'"},
		{"backtick substitution", "`whoami`", "'`whoami`'"},
		{"empty string", "", "''"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShellEscape(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `task test:pkg -- ./workflow/...`
Expected: FAIL — `ShellEscape` undefined

**Step 3: Implement ShellEscape**

Add to `workflow/helpers.go`:

```go
// ShellEscape wraps a string in single quotes for safe shell interpolation.
// Single quotes inside the string are escaped with the '\'' idiom.
func ShellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./workflow/...`
Expected: PASS

**Step 5: Write failing test for HTTP template quoting**

Add to `container/template/template_test.go`:

```go
func TestHTTP_CurlArgsAreQuoted(t *testing.T) {
	http := NewHTTP("injection-test",
		WithHTTPURL("http://example.com"),
		WithHTTPHeader("Authorization", "Bearer $(whoami)"),
		WithHTTPBody(`{"key": "value"}`))

	input := http.ToInput()

	// The script (last element of Command) should have quoted arguments
	script := input.Command[len(input.Command)-1]

	// The malicious header value should be inside single quotes, not bare
	assert.NotContains(t, script, "$(whoami)")
	assert.Contains(t, script, "'$(whoami)'")
}
```

**Step 6: Fix HTTP template to quote curl arguments**

In `container/template/http.go`, update `buildValidationScript`:

```go
func (h *HTTP) buildValidationScript(curlArgs []string) string {
	// Quote each curl argument in single quotes for safe shell interpolation
	quotedArgs := make([]string, len(curlArgs))
	for i, arg := range curlArgs {
		quotedArgs[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
	}

	curlCmd := "curl " + strings.Join(quotedArgs, " ")
	// ... rest unchanged
}
```

**Step 7: Run tests**

Run: `task test:pkg -- ./container/template/...`
Expected: PASS

**Step 8: Run full unit tests**

Run: `task test:unit`
Expected: PASS

**Step 9: Commit**

```
git add workflow/helpers.go workflow/helpers_test.go container/template/http.go container/template/template_test.go
git commit -m "fix(security): add shell escaping and fix HTTP template injection

Add ShellEscape function for safe shell interpolation.
Quote all curl arguments in HTTP template to prevent injection."
```

---

### Task 3: C4 — DAG Cycle Detection in Docker Module

**Files:**
- Modify: `container/payload/payloads_extended.go` (add cycle detection + duplicate check)
- Test: `container/payload/payloads_extended_test.go` (add cycle tests)

**Step 1: Write failing tests for cycle detection**

Add to `container/payload/payloads_extended_test.go`:

```go
func TestDAGWorkflowInput_CycleDetection(t *testing.T) {
	tests := []struct {
		name    string
		input   DAGWorkflowInput
		wantErr bool
		errMsg  string
	}{
		{
			name: "direct cycle A->B->A",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"B"}},
					{Name: "B", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "self-referencing node",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "indirect cycle A->B->C->A",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"C"}},
					{Name: "B", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
					{Name: "C", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"B"}},
				},
			},
			wantErr: true,
			errMsg:  "circular dependency",
		},
		{
			name: "duplicate node names",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "build", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}},
					{Name: "build", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}},
				},
			},
			wantErr: true,
			errMsg:  "duplicate node name",
		},
		{
			name: "valid diamond DAG (not a cycle)",
			input: DAGWorkflowInput{
				Nodes: []DAGNode{
					{Name: "A", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}},
					{Name: "B", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
					{Name: "C", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"A"}},
					{Name: "D", Container: ExtendedContainerInput{ContainerExecutionInput: ContainerExecutionInput{Image: "alpine"}}, Dependencies: []string{"B", "C"}},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
```

**Step 2: Run tests to verify failures**

Run: `task test:pkg -- ./container/payload/...`
Expected: FAIL — cycles and duplicates not detected

**Step 3: Implement cycle detection (port from function module)**

Replace `Validate()` in `container/payload/payloads_extended.go`:

```go
import "fmt"

// Validate validates DAG workflow input including cycle detection.
func (i *DAGWorkflowInput) Validate() error {
	if len(i.Nodes) == 0 {
		return errors.ErrInvalidInput.Wrap("at least one node is required")
	}

	// Check for duplicate node names.
	nodeMap := make(map[string]bool, len(i.Nodes))
	for _, node := range i.Nodes {
		if nodeMap[node.Name] {
			return errors.ErrInvalidInput.Wrap(fmt.Sprintf("duplicate node name: %s", node.Name))
		}
		nodeMap[node.Name] = true
	}

	// Check that all dependencies reference existing nodes.
	for _, node := range i.Nodes {
		for _, dep := range node.Dependencies {
			if !nodeMap[dep] {
				return errors.ErrInvalidInput.Wrap("dependency node not found: " + dep)
			}
		}
	}

	// DFS-based cycle detection.
	if err := detectDAGCycles(i.Nodes); err != nil {
		return err
	}

	return nil
}

// detectDAGCycles uses DFS to find circular dependencies in the DAG.
func detectDAGCycles(nodes []DAGNode) error {
	deps := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		deps[node.Name] = node.Dependencies
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)

	state := make(map[string]int, len(nodes))

	var dfs func(name string) error
	dfs = func(name string) error {
		state[name] = visiting
		for _, dep := range deps[name] {
			switch state[dep] {
			case visiting:
				return errors.ErrInvalidInput.Wrap(
					fmt.Sprintf("circular dependency detected involving node: %s", dep),
				)
			case unvisited:
				if err := dfs(dep); err != nil {
					return err
				}
			}
		}
		state[name] = visited
		return nil
	}

	for _, node := range nodes {
		if state[node.Name] == unvisited {
			if err := dfs(node.Name); err != nil {
				return err
			}
		}
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `task test:pkg -- ./container/payload/...`
Expected: PASS

**Step 5: Run full unit tests**

Run: `task test:unit`
Expected: PASS

**Step 6: Commit**

```
git add container/payload/payloads_extended.go container/payload/payloads_extended_test.go
git commit -m "fix(docker): add DAG cycle detection and duplicate node validation

Port DFS-based cycle detection from function module to docker module.
Add duplicate node name check. Prevents infinite recursion on cyclic
dependency graphs that would crash the Temporal worker."
```

---

### Task 4: C5 — Panic Recovery in Function Handler Dispatch

**Files:**
- Modify: `function/activity/function.go` (add `recover()`)
- Test: `function/activity/function_test.go` (add panic test)

**Step 1: Write failing test for panic recovery**

Add to `function/activity/function_test.go`:

```go
func TestExecuteFunctionActivity_PanicRecovery(t *testing.T) {
	registry := fn.NewRegistry()
	registry.Register("panic-handler", func(_ context.Context, _ fn.FunctionInput) (*fn.FunctionOutput, error) {
		panic("unexpected nil pointer")
	})

	activity := NewExecuteFunctionActivity(registry)

	input := payload.FunctionExecutionInput{Name: "panic-handler"}

	// Should NOT panic — should return graceful error
	output, err := activity(context.Background(), input)
	require.NoError(t, err) // Activity returns nil error (business logic failure)
	require.NotNil(t, output)

	assert.False(t, output.Success)
	assert.Contains(t, output.Error, "panic")
	assert.Contains(t, output.Error, "unexpected nil pointer")
	assert.NotZero(t, output.Duration)
}
```

**Step 2: Run test to verify it fails (panic crashes the test)**

Run: `task test:run -- -run TestExecuteFunctionActivity_PanicRecovery ./function/activity/...`
Expected: FAIL — test panics

**Step 3: Add panic recovery**

In `function/activity/function.go`, wrap the handler call:

```go
// Call handler with panic recovery
var fnOutput *fn.FunctionOutput
var handlerErr error

func() {
	defer func() {
		if r := recover(); r != nil {
			handlerErr = fmt.Errorf("handler panic: %v", r)
		}
	}()
	fnOutput, handlerErr = handler(ctx, fnInput)
}()
```

Add `"fmt"` to imports.

**Step 4: Run test to verify it passes**

Run: `task test:run -- -run TestExecuteFunctionActivity_PanicRecovery ./function/activity/...`
Expected: PASS

**Step 5: Run full unit tests**

Run: `task test:unit`
Expected: PASS

**Step 6: Commit**

```
git add function/activity/function.go function/activity/function_test.go
git commit -m "fix(function): add panic recovery in handler dispatch

Wrap handler calls with recover() to prevent a panicking handler
from crashing the entire Temporal worker. Panics are captured as
business logic failures (Success=false) without triggering retries."
```

---

### Task 5: Final Verification

**Step 1: Run full unit test suite**

Run: `task test:unit`
Expected: PASS with no regressions

**Step 2: Run linter**

Run: `task lint`
Expected: PASS

**Step 3: Run formatter**

Run: `task fmt`
Expected: No changes (or apply formatting)
