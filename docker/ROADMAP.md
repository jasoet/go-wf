# go-wf/docker Feature Roadmap

This document outlines the plan to close feature gaps with Argo Workflows, excluding Kubernetes-specific features.

## Priority Classification

### P0 - Critical (Next Release)
Features that significantly limit workflow expressiveness and are frequently needed.

### P1 - High Priority (Following Release)
Important features that enhance usability but have workarounds.

### P2 - Medium Priority (Future Releases)
Nice-to-have features that improve specific use cases.

### P3 - Low Priority (Backlog)
Features that provide incremental value.

---

## P0 - Critical Features

### 1. Loop Support (withItems/withParam)

**Current State:** Must programmatically create containers in Go code
**Target State:** Declarative loop syntax similar to Argo

**Implementation Plan:**

#### Phase 1: withItems Support (Simple Iteration)
```go
// API Design
type LoopInput struct {
    Items    []string                     // List of items to iterate
    Template ContainerExecutionInput      // Template for each item
    Parallel bool                         // Run in parallel or sequential
    MaxConcurrency int                   // Max parallel executions
}

// Usage Example
loop := docker.LoopInput{
    Items: []string{"file1.csv", "file2.csv", "file3.csv"},
    Template: docker.ContainerExecutionInput{
        Image: "processor:v1",
        Command: []string{"process", "{{item}}"},
        Env: map[string]string{
            "INPUT_FILE": "{{item}}",
            "INDEX": "{{index}}",
        },
    },
    Parallel: true,
    MaxConcurrency: 3,
}
```

#### Phase 2: withParam Support (Dynamic Parameters)
```go
// API Design
type ParameterizedLoopInput struct {
    Parameters map[string][]string         // Multiple parameter arrays
    Template   ContainerExecutionInput     // Template with multiple {{.param}}
    Parallel   bool
}

// Usage Example
loop := docker.ParameterizedLoopInput{
    Parameters: map[string][]string{
        "env":     {"dev", "staging", "prod"},
        "region":  {"us-west", "us-east", "eu-central"},
    },
    Template: docker.ContainerExecutionInput{
        Image: "deployer:v1",
        Command: []string{"deploy", "--env={{.env}}", "--region={{.region}}"},
    },
}
```

**Implementation Steps:**
1. Add `LoopInput` and `ParameterizedLoopInput` to payloads.go
2. Create `LoopWorkflow` in workflows.go
3. Implement template variable substitution (extend existing parameter logic)
4. Add builder support: `builder.ForEach(items, template)`
5. Add pattern: `patterns.ParallelLoop(items, template)`
6. Write comprehensive tests
7. Add examples/loop.go

**Estimated Effort:** 3-4 days
**Files to Modify:**
- docker/payloads.go (new types)
- docker/workflows.go (new workflow)
- docker/builder/builder.go (ForEach method)
- docker/patterns/parallel.go (loop pattern)
- docker/examples/loop.go (new example)

---

### 2. Explicit Data Passing Between Steps

**Current State:** No built-in mechanism to pass outputs to inputs
**Target State:** Explicit output capture and input mapping

**Implementation Plan:**

#### Phase 1: Output Capture
```go
// API Design - Extend ExtendedContainerInput
type ExtendedContainerInput struct {
    ContainerExecutionInput

    // New fields
    Outputs []OutputDefinition `json:"outputs,omitempty"`
}

type OutputDefinition struct {
    Name     string `json:"name" validate:"required"`
    Path     string `json:"path"`           // File path to read
    ValueFrom string `json:"value_from"`     // Extract from: stdout, stderr, file, exitCode
    JSONPath  string `json:"json_path"`      // For JSON extraction
}

// Example
outputs := []docker.OutputDefinition{
    {Name: "version", ValueFrom: "stdout"},
    {Name: "build_id", Path: "/output/build.json", JSONPath: "$.build.id"},
}
```

#### Phase 2: Input Mapping
```go
// API Design
type InputMapping struct {
    Name  string `json:"name"`
    From  string `json:"from"`  // Format: "step-name.output-name"
}

type ExtendedContainerInput struct {
    // ... existing fields
    Inputs []InputMapping `json:"inputs,omitempty"`
}

// Example
inputs := []docker.InputMapping{
    {Name: "BUILD_VERSION", From: "build.version"},
    {Name: "BUILD_ID", From: "build.build_id"},
}
```

#### Phase 3: DAG Integration
```go
// Modify DAGNode to support data flow
type DAGNode struct {
    Name         string
    Container    ExtendedContainerInput
    Dependencies []string

    // Data dependencies - more explicit than just dependencies
    DataFrom     []string `json:"data_from,omitempty"`  // Nodes to wait for data from
}
```

**Implementation Steps:**
1. Add OutputDefinition and InputMapping to payloads_extended.go
2. Implement output extraction in activities.go
3. Create workflow context for storing step outputs
4. Implement input substitution from previous step outputs
5. Update DAGWorkflow to handle data dependencies
6. Add validation for circular data dependencies
7. Write comprehensive tests
8. Add examples/data-passing.go

**Estimated Effort:** 5-6 days
**Files to Modify:**
- docker/payloads_extended.go (new types)
- docker/activities.go (output extraction)
- docker/dag.go (data flow handling)
- docker/workflows.go (context management)
- docker/examples/data-passing.go (new example)

---

### 3. Artifact Storage Implementation

**Current State:** Artifact structures defined but not implemented
**Target State:** Full artifact upload/download with storage backends

**Implementation Plan:**

#### Phase 1: Storage Interface
```go
// API Design
type ArtifactStore interface {
    Upload(ctx context.Context, artifact Artifact, data io.Reader) error
    Download(ctx context.Context, artifact Artifact) (io.ReadCloser, error)
    Delete(ctx context.Context, artifact Artifact) error
}

// Built-in implementations
type LocalFileStore struct {
    BasePath string
}

type S3Store struct {
    Bucket string
    Prefix string
    Client *s3.Client
}

type MinioStore struct {
    Endpoint string
    Bucket   string
    Client   *minio.Client
}
```

#### Phase 2: Artifact Activities
```go
// New activities
func UploadArtifactActivity(ctx context.Context, input ArtifactUploadInput) error
func DownloadArtifactActivity(ctx context.Context, input ArtifactDownloadInput) error

// Integration with container execution
type ArtifactConfig struct {
    Store          ArtifactStore
    WorkflowID     string
    RunID          string
}
```

#### Phase 3: Workflow Integration
```go
// Usage in DAG
node := docker.DAGNode{
    Name: "build",
    Container: docker.ExtendedContainerInput{
        OutputArtifacts: []docker.Artifact{
            {Name: "binary", Path: "/output/app", Type: "file"},
        },
    },
}

// Automatic download for dependent nodes
node2 := docker.DAGNode{
    Name: "test",
    Container: docker.ExtendedContainerInput{
        InputArtifacts: []docker.Artifact{
            {Name: "binary", Path: "/app/app", Type: "file"},
        },
    },
    Dependencies: []string{"build"},
}
```

**Implementation Steps:**
1. Create docker/artifacts package
2. Implement ArtifactStore interface
3. Implement LocalFileStore (default)
4. Add optional S3Store and MinioStore
5. Create artifact upload/download activities
6. Integrate with container execution workflow
7. Add artifact cleanup on workflow completion
8. Write comprehensive tests
9. Add examples/artifacts.go

**Estimated Effort:** 6-7 days
**Files to Modify:**
- docker/artifacts/store.go (new package)
- docker/artifacts/local.go (local storage)
- docker/artifacts/s3.go (S3 storage)
- docker/activities.go (artifact handling)
- docker/dag.go (artifact integration)
- docker/examples/artifacts.go (new example)

---

## P1 - High Priority Features

### 4. Suspend/Resume Workflow

**Current State:** No manual intervention points
**Target State:** Workflows can pause for approvals or manual triggers

**Implementation Plan:**

#### Phase 1: Approval Gates
```go
// API Design
type ApprovalGate struct {
    Name        string        `json:"name"`
    Timeout     time.Duration `json:"timeout"`
    Approvers   []string      `json:"approvers,omitempty"`
    Message     string        `json:"message"`
    AutoApprove bool          `json:"auto_approve"`
}

// Usage
gate := docker.ApprovalGate{
    Name: "production-deployment",
    Timeout: 24 * time.Hour,
    Approvers: []string{"ops-team", "tech-lead"},
    Message: "Approve deployment to production?",
}
```

#### Phase 2: Workflow Integration
```go
// Add to DAGNode
type DAGNode struct {
    // ... existing fields
    ApprovalGate *ApprovalGate `json:"approval_gate,omitempty"`
}

// Signal-based approval
func ApproveWorkflow(ctx context.Context, client client.Client, workflowID, gateName string) error {
    return SignalWorkflow(ctx, client, workflowID, "", "approve", gateName)
}

func RejectWorkflow(ctx context.Context, client client.Client, workflowID, gateName string, reason string) error {
    return SignalWorkflow(ctx, client, workflowID, "", "reject", map[string]string{
        "gate": gateName,
        "reason": reason,
    })
}
```

#### Phase 3: Manual Tasks
```go
// API Design
type ManualTask struct {
    Name         string        `json:"name"`
    Instructions string        `json:"instructions"`
    Timeout      time.Duration `json:"timeout"`
    RequireSignal bool         `json:"require_signal"`
}

// Usage
task := docker.ManualTask{
    Name: "verify-deployment",
    Instructions: "Manually verify the deployment in staging environment",
    Timeout: 2 * time.Hour,
    RequireSignal: true,
}
```

**Implementation Steps:**
1. Add ApprovalGate and ManualTask to payloads_extended.go
2. Implement signal-based approval in workflows
3. Add approval workflow activities
4. Create helper functions for approve/reject
5. Add timeout handling for approvals
6. Write comprehensive tests
7. Add examples/approval.go

**Estimated Effort:** 4-5 days
**Files to Modify:**
- docker/payloads_extended.go (approval types)
- docker/dag.go (approval integration)
- docker/operations.go (approve/reject helpers)
- docker/examples/approval.go (new example)

---

### 5. Retry Policies Per Step

**Current State:** Global retry policy only
**Target State:** Per-step retry configuration

**Implementation Plan:**

```go
// API Design - Extend ExtendedContainerInput
type RetryPolicy struct {
    MaxAttempts       int           `json:"max_attempts"`
    InitialInterval   time.Duration `json:"initial_interval"`
    BackoffCoefficient float64      `json:"backoff_coefficient"`
    MaxInterval       time.Duration `json:"max_interval"`
    RetryableErrors   []string      `json:"retryable_errors,omitempty"`
}

type ExtendedContainerInput struct {
    // ... existing fields
    RetryPolicy *RetryPolicy `json:"retry_policy,omitempty"`
}

// Usage
node := docker.DAGNode{
    Name: "flaky-api-call",
    Container: docker.ExtendedContainerInput{
        ContainerExecutionInput: docker.ContainerExecutionInput{
            Image: "api-client:v1",
        },
        RetryPolicy: &docker.RetryPolicy{
            MaxAttempts: 5,
            InitialInterval: 2 * time.Second,
            BackoffCoefficient: 2.0,
            MaxInterval: 30 * time.Second,
        },
    },
}
```

**Implementation Steps:**
1. Add RetryPolicy to payloads_extended.go
2. Modify workflow to use per-step retry policies
3. Add retry policy builder methods
4. Write comprehensive tests
5. Update examples

**Estimated Effort:** 2-3 days
**Files to Modify:**
- docker/payloads_extended.go (retry policy)
- docker/workflows.go (per-step retry)
- docker/dag.go (retry handling)

---

## P2 - Medium Priority Features

### 6. Workflow Templates (Reusable Workflows)

**Current State:** Can create Go functions, but not runtime templates
**Target State:** Define reusable workflow templates that can be instantiated

**Implementation Plan:**

```go
// API Design
type WorkflowTemplate struct {
    Name       string                    `json:"name"`
    Parameters []WorkflowParameter       `json:"parameters"`
    Workflow   interface{}               // PipelineInput, ParallelInput, or DAGWorkflowInput
}

type WorkflowTemplateInstance struct {
    TemplateName string                 `json:"template_name"`
    Arguments    map[string]string       `json:"arguments"`
}

// Usage
template := docker.WorkflowTemplate{
    Name: "deploy-service",
    Parameters: []docker.WorkflowParameter{
        {Name: "service", Required: true},
        {Name: "version", Required: true},
        {Name: "environment", Required: true},
    },
    Workflow: /* DAGWorkflowInput with {{.service}}, etc. */,
}

// Instantiate
instance := docker.WorkflowTemplateInstance{
    TemplateName: "deploy-service",
    Arguments: map[string]string{
        "service": "api",
        "version": "v1.2.3",
        "environment": "production",
    },
}
```

**Estimated Effort:** 5-6 days

---

### 7. Workflow Metrics and Observability

**Current State:** Basic Temporal metrics only
**Target State:** Custom workflow metrics and step timing

**Implementation Plan:**

```go
// API Design
type WorkflowMetrics struct {
    StepDurations    map[string]time.Duration
    StepSuccessCount map[string]int
    StepFailureCount map[string]int
    TotalDuration    time.Duration
    ContainerCount   int
}

// Expose metrics
func GetWorkflowMetrics(ctx context.Context, client client.Client, workflowID string) (*WorkflowMetrics, error)
```

**Estimated Effort:** 3-4 days

---

### 8. Conditional Retry (Retry on Specific Exit Codes)

**Current State:** Retry on any failure
**Target State:** Retry only on specific conditions

**Implementation Plan:**

```go
// Extend RetryPolicy
type RetryPolicy struct {
    // ... existing fields
    RetryOnExitCodes []int    `json:"retry_on_exit_codes,omitempty"`
    SkipOnExitCodes  []int    `json:"skip_on_exit_codes,omitempty"`
    RetryOnError     string   `json:"retry_on_error,omitempty"`  // Regex pattern
}
```

**Estimated Effort:** 2-3 days

---

## P3 - Low Priority Features

### 9. Synchronization Primitives

**Current State:** None
**Target State:** Semaphores and mutexes for resource control

**Implementation Plan:**

```go
// API Design
type Semaphore struct {
    Name      string `json:"name"`
    Limit     int    `json:"limit"`
}

type DAGNode struct {
    // ... existing fields
    Semaphores []Semaphore `json:"semaphores,omitempty"`
}

// Usage - limit concurrent deployments
node := docker.DAGNode{
    Name: "deploy",
    Semaphores: []docker.Semaphore{
        {Name: "deployment-lock", Limit: 1},
    },
}
```

**Estimated Effort:** 4-5 days

---

### 10. Memoization (Result Caching)

**Current State:** No caching
**Target State:** Cache step results based on inputs

**Implementation Plan:**

```go
// API Design
type MemoizationConfig struct {
    Enabled bool          `json:"enabled"`
    Key     string        `json:"key"`      // Cache key template
    TTL     time.Duration `json:"ttl"`
}

type ExtendedContainerInput struct {
    // ... existing fields
    Memoization *MemoizationConfig `json:"memoization,omitempty"`
}

// Usage
node := docker.DAGNode{
    Name: "expensive-build",
    Container: docker.ExtendedContainerInput{
        Memoization: &docker.MemoizationConfig{
            Enabled: true,
            Key: "build-{{.git_sha}}",
            TTL: 24 * time.Hour,
        },
    },
}
```

**Estimated Effort:** 5-6 days

---

## Implementation Timeline

### Phase 1 (P0 - Critical) - Q1 2025
**Duration:** 3-4 weeks

Week 1-2:
- ✅ Loop Support (withItems/withParam)
- ✅ Loop examples and documentation

Week 2-3:
- ✅ Explicit Data Passing Between Steps
- ✅ Data passing examples

Week 3-4:
- ✅ Artifact Storage Implementation
- ✅ Artifact examples and documentation
- ✅ Release v0.2.0

### Phase 2 (P1 - High Priority) - Q2 2025
**Duration:** 2-3 weeks

Week 1:
- ✅ Suspend/Resume Workflow
- ✅ Approval gate examples

Week 2:
- ✅ Per-step Retry Policies
- ✅ Retry policy examples

Week 3:
- ✅ Documentation updates
- ✅ Release v0.3.0

### Phase 3 (P2 - Medium Priority) - Q3 2025
**Duration:** 3-4 weeks

- Workflow Templates
- Metrics and Observability
- Conditional Retry
- Release v0.4.0

### Phase 4 (P3 - Low Priority) - Q4 2025
**Duration:** 2-3 weeks

- Synchronization Primitives
- Memoization
- Release v0.5.0

---

## Testing Strategy

### For Each Feature:

1. **Unit Tests**
   - Test all new types and functions
   - Test edge cases and error conditions
   - Achieve 85%+ coverage

2. **Integration Tests**
   - Test with real Docker containers
   - Test with Temporal workflows
   - Test failure scenarios

3. **Examples**
   - Create working example for each feature
   - Document use cases
   - Include in examples directory

4. **Documentation**
   - Update README.md
   - Update ARGO_COMPARISON.md
   - Add feature-specific docs

---

## API Stability Considerations

### Versioning Strategy:
- v0.x.y - Pre-1.0, breaking changes allowed with minor version bump
- v1.x.y - Post-1.0, breaking changes require major version bump

### Deprecation Policy:
- Mark deprecated features in v0.x
- Provide migration path
- Remove in v0.x+2

---

## Success Metrics

### Coverage Goals:
- **P0 Features:** 85%+ feature parity with Argo Workflows (non-K8s)
- **P1 Features:** 90%+ feature parity
- **P2+P3 Features:** 95%+ feature parity

### Quality Goals:
- Zero breaking changes without major version bump (post v1.0)
- 85%+ test coverage on all new code
- All examples compile and run successfully
- Documentation complete for all features

---

## Migration Path from Current Version

### For Loop Support:
```go
// Before (programmatic)
for _, env := range environments {
    wb.Add(template.NewContainer(env, "alpine"))
}

// After (declarative)
wb.ForEach(environments, template.NewContainer("deploy", "alpine"))
```

### For Data Passing:
```go
// Before (no explicit passing)
// Manual file sharing via volumes

// After (explicit)
build := DAGNode{
    Outputs: []OutputDefinition{{Name: "version", ValueFrom: "stdout"}},
}
deploy := DAGNode{
    Inputs: []InputMapping{{Name: "VERSION", From: "build.version"}},
}
```

---

## Community Feedback Integration

### Feedback Channels:
- GitHub Issues for feature requests
- Discussions for design proposals
- Pull Requests welcome

### Decision Process:
1. Feature request submitted
2. Team evaluates against roadmap
3. Design discussion if accepted
4. Implementation
5. Release

---

## Conclusion

This roadmap prioritizes the most impactful features while maintaining backwards compatibility and code quality. The phased approach allows for iterative development and community feedback integration.

**Next Steps:**
1. Create GitHub issues for P0 features
2. Begin implementation of Loop Support
3. Gather community feedback on design proposals
