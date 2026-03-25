# Function Module Feature Parity Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add DAG workflow and patterns package to the function module, achieving feature parity with the docker module.

**Architecture:** Two vertical slices — Slice 1 delivers DAG end-to-end (payload → workflow → OTel → builder → registration → worker → trigger), Slice 2 delivers the patterns package (pipeline, parallel, loop, DAG patterns). Each task follows TDD.

**Tech Stack:** Go 1.26+, Temporal SDK (testsuite for unit tests), testify, go-playground/validator, pkgotel

---

## Slice 1: DAG Workflow

### Task 1: DAG Payload Types

**Files:**
- Create: `function/payload/payload_extended.go`
- Test: `function/payload/payload_extended_test.go`

**Step 1: Write the failing test**

```go
// function/payload/payload_extended_test.go
package payload

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGWorkflowInput_Validate_Valid(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{Name: "build", Function: FunctionExecutionInput{Name: "compile"}},
			{Name: "test", Function: FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"build"}},
		},
		FailFast: true,
	}
	assert.NoError(t, input.Validate())
}

func TestDAGWorkflowInput_Validate_EmptyNodes(t *testing.T) {
	input := DAGWorkflowInput{Nodes: []DAGNode{}}
	assert.Error(t, input.Validate())
}

func TestDAGWorkflowInput_Validate_MissingDependency(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{Name: "build", Function: FunctionExecutionInput{Name: "compile"}, Dependencies: []string{"nonexistent"}},
		},
	}
	assert.Error(t, input.Validate())
}

func TestDAGWorkflowInput_Validate_CircularDependency(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{Name: "a", Function: FunctionExecutionInput{Name: "fn-a"}, Dependencies: []string{"b"}},
			{Name: "b", Function: FunctionExecutionInput{Name: "fn-b"}, Dependencies: []string{"a"}},
		},
	}
	assert.Error(t, input.Validate())
}

func TestDAGWorkflowInput_Validate_DuplicateNodeNames(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []DAGNode{
			{Name: "build", Function: FunctionExecutionInput{Name: "compile"}},
			{Name: "build", Function: FunctionExecutionInput{Name: "compile2"}},
		},
	}
	assert.Error(t, input.Validate())
}

func TestOutputMapping_Defaults(t *testing.T) {
	om := OutputMapping{Name: "version", ResultKey: "ver", Default: "0.0.0"}
	assert.Equal(t, "version", om.Name)
	assert.Equal(t, "ver", om.ResultKey)
	assert.Equal(t, "0.0.0", om.Default)
}

func TestInputMapping_Fields(t *testing.T) {
	im := InputMapping{Name: "build_id", From: "build.id", Default: "unknown", Required: true}
	assert.Equal(t, "build_id", im.Name)
	assert.Equal(t, "build.id", im.From)
	assert.True(t, im.Required)
}

func TestDataMapping_Fields(t *testing.T) {
	dm := DataMapping{FromNode: "extract", Optional: false}
	assert.Equal(t, "extract", dm.FromNode)
	assert.False(t, dm.Optional)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/payload/...`
Expected: FAIL — types not defined

**Step 3: Write minimal implementation**

```go
// function/payload/payload_extended.go
package payload

import (
	"fmt"
	"time"

	"github.com/jasoet/go-wf/workflow/errors"
)

// OutputMapping exposes a key from a function's Result map as a named output.
type OutputMapping struct {
	Name      string `json:"name" validate:"required"`
	ResultKey string `json:"result_key" validate:"required"`
	Default   string `json:"default,omitempty"`
}

// InputMapping maps a dependency's output to an arg in the current node.
type InputMapping struct {
	Name     string `json:"name" validate:"required"`
	From     string `json:"from" validate:"required"`
	Default  string `json:"default,omitempty"`
	Required bool   `json:"required"`
}

// DataMapping controls passing Data bytes from one node to another.
type DataMapping struct {
	FromNode string `json:"from_node" validate:"required"`
	Optional bool   `json:"optional"`
}

// DAGNode represents a node in a DAG workflow.
type DAGNode struct {
	Name         string                 `json:"name" validate:"required"`
	Function     FunctionExecutionInput `json:"function" validate:"required"`
	Dependencies []string               `json:"dependencies,omitempty"`
	Outputs      []OutputMapping        `json:"outputs,omitempty"`
	Inputs       []InputMapping         `json:"inputs,omitempty"`
	DataInput    *DataMapping           `json:"data_input,omitempty"`
}

// DAGWorkflowInput defines a DAG (Directed Acyclic Graph) workflow.
type DAGWorkflowInput struct {
	Nodes       []DAGNode `json:"nodes" validate:"required,min=1"`
	FailFast    bool      `json:"fail_fast"`
	MaxParallel int       `json:"max_parallel,omitempty"`
}

// Validate validates DAG workflow input.
func (i *DAGWorkflowInput) Validate() error {
	if len(i.Nodes) == 0 {
		return errors.ErrInvalidInput.Wrap("at least one node is required")
	}

	nodeMap := make(map[string]bool)
	for _, node := range i.Nodes {
		if nodeMap[node.Name] {
			return errors.ErrInvalidInput.Wrap(fmt.Sprintf("duplicate node name: %s", node.Name))
		}
		nodeMap[node.Name] = true
	}

	for _, node := range i.Nodes {
		for _, dep := range node.Dependencies {
			if !nodeMap[dep] {
				return errors.ErrInvalidInput.Wrap("dependency node not found: " + dep)
			}
		}
	}

	if hasCycle(i.Nodes) {
		return errors.ErrInvalidInput.Wrap("circular dependency detected")
	}

	return nil
}

// hasCycle detects circular dependencies using DFS.
func hasCycle(nodes []DAGNode) bool {
	deps := make(map[string][]string)
	for _, n := range nodes {
		deps[n.Name] = n.Dependencies
	}

	const (
		white = 0 // unvisited
		gray  = 1 // in progress
		black = 2 // done
	)
	color := make(map[string]int)

	var visit func(string) bool
	visit = func(name string) bool {
		color[name] = gray
		for _, dep := range deps[name] {
			if color[dep] == gray {
				return true
			}
			if color[dep] == white && visit(dep) {
				return true
			}
		}
		color[name] = black
		return false
	}

	for _, n := range nodes {
		if color[n.Name] == white {
			if visit(n.Name) {
				return true
			}
		}
	}
	return false
}

// NodeResult represents the execution result of a single DAG node.
type NodeResult struct {
	NodeName  string                   `json:"node_name"`
	Result    *FunctionExecutionOutput `json:"result,omitempty"`
	StartTime time.Time                `json:"start_time"`
	Success   bool                     `json:"success"`
	Error     error                    `json:"error,omitempty"`
}

// DAGWorkflowOutput defines the output of a DAG workflow execution.
type DAGWorkflowOutput struct {
	Results       map[string]*FunctionExecutionOutput `json:"results"`
	NodeResults   []NodeResult                        `json:"node_results"`
	StepOutputs   map[string]map[string]string        `json:"step_outputs,omitempty"`
	TotalSuccess  int                                 `json:"total_success"`
	TotalFailed   int                                 `json:"total_failed"`
	TotalDuration time.Duration                       `json:"total_duration"`
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/payload/...`
Expected: PASS

**Step 5: Commit**

```
git add function/payload/payload_extended.go function/payload/payload_extended_test.go
git commit -m "feat(function): add DAG payload types"
```

---

### Task 2: DAG Workflow Implementation

**Files:**
- Create: `function/workflow/dag.go`
- Test: `function/workflow/dag_test.go`

**Step 1: Write the failing test**

```go
// function/workflow/dag_test.go
package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestDAGWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{
			Success:  true,
			Duration: 1 * time.Second,
		}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "build", Function: payload.FunctionExecutionInput{Name: "compile"}},
			{Name: "test", Function: payload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"build"}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, 0, result.TotalFailed)
}

func TestDAGWorkflow_ValidationError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{Nodes: []payload.DAGNode{}}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_FailFast(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: false, Error: "build failed"}, nil).Once()

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "build", Function: payload.FunctionExecutionInput{Name: "compile"}},
			{Name: "test", Function: payload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"build"}},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_DiamondDependency(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: true, Duration: 1 * time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "A", Function: payload.FunctionExecutionInput{Name: "fn-a"}},
			{Name: "B", Function: payload.FunctionExecutionInput{Name: "fn-b"}, Dependencies: []string{"A"}},
			{Name: "C", Function: payload.FunctionExecutionInput{Name: "fn-c"}, Dependencies: []string{"A"}},
			{Name: "D", Function: payload.FunctionExecutionInput{Name: "fn-d"}, Dependencies: []string{"B", "C"}},
		},
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 4, result.TotalSuccess)
}

func TestDAGWorkflow_AlreadyExecutedGuard(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	callCount := 0
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		func(_ context.Context, in payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
			callCount++
			return &payload.FunctionExecutionOutput{Success: true, Name: in.Name, Duration: 1 * time.Second}, nil
		})

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "A", Function: payload.FunctionExecutionInput{Name: "fn-a"}},
			{Name: "B", Function: payload.FunctionExecutionInput{Name: "fn-b"}, Dependencies: []string{"A"}},
			{Name: "C", Function: payload.FunctionExecutionInput{Name: "fn-c"}, Dependencies: []string{"A"}},
		},
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 3, callCount, "A should only execute once despite being dep of both B and C")
	assert.Equal(t, 3, result.TotalSuccess)
}

func TestDAGWorkflow_InputMapping(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	// Build returns result with "artifact" key
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(in payload.FunctionExecutionInput) bool {
		return in.Name == "compile"
	})).Return(&payload.FunctionExecutionOutput{
		Success: true,
		Result:  map[string]string{"artifact": "app-binary", "version": "1.0"},
	}, nil)

	// Deploy should receive artifact_path in args
	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(in payload.FunctionExecutionInput) bool {
		return in.Name == "publish-artifact"
	})).Return(&payload.FunctionExecutionOutput{
		Success: true,
		Result:  map[string]string{"published": "true"},
	}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{
				Name:     "build",
				Function: payload.FunctionExecutionInput{Name: "compile"},
				Outputs: []payload.OutputMapping{
					{Name: "artifact", ResultKey: "artifact"},
				},
			},
			{
				Name:         "deploy",
				Function:     payload.FunctionExecutionInput{Name: "publish-artifact", Args: map[string]string{}},
				Dependencies: []string{"build"},
				Inputs: []payload.InputMapping{
					{Name: "artifact_path", From: "build.artifact", Required: true},
				},
			},
		},
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
	assert.Equal(t, "app-binary", result.StepOutputs["build"]["artifact"])
}

func TestDAGWorkflow_DataMapping(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	testData := []byte(`{"records": [1, 2, 3]}`)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(in payload.FunctionExecutionInput) bool {
		return in.Name == "extract"
	})).Return(&payload.FunctionExecutionOutput{
		Success: true,
		Data:    testData,
	}, nil)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(in payload.FunctionExecutionInput) bool {
		return in.Name == "transform"
	})).Return(&payload.FunctionExecutionOutput{
		Success: true,
	}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "extract", Function: payload.FunctionExecutionInput{Name: "extract"}},
			{
				Name:         "transform",
				Function:     payload.FunctionExecutionInput{Name: "transform"},
				Dependencies: []string{"extract"},
				DataInput:    &payload.DataMapping{FromNode: "extract"},
			},
		},
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestDAGWorkflow_ActivityError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		nil, fmt.Errorf("activity crashed"))

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "broken", Function: payload.FunctionExecutionInput{Name: "crash"}},
		},
		FailFast: true,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}

func TestDAGWorkflow_ContinueOnFailure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(in payload.FunctionExecutionInput) bool {
		return in.Name == "fail-fn"
	})).Return(&payload.FunctionExecutionOutput{Success: false, Error: "failed"}, nil)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.MatchedBy(func(in payload.FunctionExecutionInput) bool {
		return in.Name == "pass-fn"
	})).Return(&payload.FunctionExecutionOutput{Success: true}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "failing", Function: payload.FunctionExecutionInput{Name: "fail-fn"}},
			{Name: "passing", Function: payload.FunctionExecutionInput{Name: "pass-fn"}},
		},
		FailFast: false,
	}

	env.ExecuteWorkflow(DAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 1, result.TotalSuccess)
	assert.Equal(t, 1, result.TotalFailed)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/workflow/...`
Expected: FAIL — DAGWorkflow not defined

**Step 3: Write minimal implementation**

```go
// function/workflow/dag.go
package workflow

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
)

// dagState holds shared mutable state for DAG execution.
type dagState struct {
	mu          sync.Mutex
	executed    map[string]bool
	results     map[string]*payload.FunctionExecutionOutput
	stepOutputs map[string]map[string]string
	stepData    map[string][]byte
}

func newDAGState() *dagState {
	return &dagState{
		executed:    make(map[string]bool),
		results:     make(map[string]*payload.FunctionExecutionOutput),
		stepOutputs: make(map[string]map[string]string),
		stepData:    make(map[string][]byte),
	}
}

// DAGWorkflow executes functions in a DAG (Directed Acyclic Graph) pattern.
func DAGWorkflow(ctx wf.Context, input payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("Starting DAG workflow", "nodes", len(input.Nodes))

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("invalid DAG input: %w", err)
	}

	startTime := wf.Now(ctx)
	output := &payload.DAGWorkflowOutput{
		Results:     make(map[string]*payload.FunctionExecutionOutput),
		NodeResults: make([]payload.NodeResult, 0, len(input.Nodes)),
		StepOutputs: make(map[string]map[string]string),
	}

	ctx = wf.WithActivityOptions(ctx, dagActivityOptions())
	state := newDAGState()
	nodeMap := buildFnNodeMap(input.Nodes)

	var executeNode func(string) error
	executeNode = func(nodeName string) error {
		return executeFnDAGNode(ctx, nodeName, nodeMap, &input, state, output, executeNode)
	}

	for _, node := range input.Nodes {
		if err := executeNode(node.Name); err != nil {
			output.TotalDuration = wf.Now(ctx).Sub(startTime)
			return output, err
		}
	}

	output.Results = state.results
	output.StepOutputs = state.stepOutputs
	output.TotalDuration = wf.Now(ctx).Sub(startTime)

	logger.Info("DAG workflow completed",
		"success", output.TotalSuccess,
		"failed", output.TotalFailed,
		"duration", output.TotalDuration)

	return output, nil
}

func buildFnNodeMap(nodes []payload.DAGNode) map[string]*payload.DAGNode {
	nodeMap := make(map[string]*payload.DAGNode, len(nodes))
	for i := range nodes {
		nodeMap[nodes[i].Name] = &nodes[i]
	}
	return nodeMap
}

func executeFnDAGNode(ctx wf.Context, nodeName string, nodeMap map[string]*payload.DAGNode, input *payload.DAGWorkflowInput, state *dagState, output *payload.DAGWorkflowOutput, executeNode func(string) error) error {
	logger := wf.GetLogger(ctx)

	state.mu.Lock()
	if state.executed[nodeName] {
		state.mu.Unlock()
		return nil
	}
	state.mu.Unlock()

	node, ok := nodeMap[nodeName]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeName)
	}

	if err := executeFnDependencies(executeNode, node, input, state); err != nil {
		return err
	}

	logger.Info("Executing node", "name", nodeName)

	fnInput := node.Function
	if err := applyFnInputMappings(&fnInput, node, state); err != nil {
		return err
	}
	applyFnDataMapping(&fnInput, node, state)

	var result payload.FunctionExecutionOutput
	err := wf.ExecuteActivity(ctx, fnInput.ActivityName(), fnInput).Get(ctx, &result)

	extractFnOutputs(node, &result, state)

	state.mu.Lock()
	state.results[nodeName] = &result
	state.executed[nodeName] = true
	if result.Data != nil {
		state.stepData[nodeName] = result.Data
	}
	state.mu.Unlock()

	recordFnNodeResult(nodeName, &result, err, ctx, output, logger)

	if (err != nil || !result.Success) && input.FailFast {
		if err != nil {
			return err
		}
		return fmt.Errorf("node %s failed", nodeName)
	}
	return nil
}

func executeFnDependencies(executeNode func(string) error, node *payload.DAGNode, input *payload.DAGWorkflowInput, state *dagState) error {
	for _, dep := range node.Dependencies {
		if err := executeNode(dep); err != nil {
			return err
		}

		if input.FailFast {
			state.mu.Lock()
			depResult := state.results[dep]
			state.mu.Unlock()

			if depResult != nil && !depResult.Success {
				return fmt.Errorf("dependency %s failed", dep)
			}
		}
	}
	return nil
}

func applyFnInputMappings(fnInput *payload.FunctionExecutionInput, node *payload.DAGNode, state *dagState) error {
	if len(node.Inputs) == 0 {
		return nil
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	if fnInput.Args == nil {
		fnInput.Args = make(map[string]string)
	}

	for _, im := range node.Inputs {
		parts := strings.SplitN(im.From, ".", 2)
		if len(parts) != 2 {
			if im.Required {
				return fmt.Errorf("invalid input mapping format for node %s: %s", node.Name, im.From)
			}
			if im.Default != "" {
				fnInput.Args[im.Name] = im.Default
			}
			continue
		}

		stepName, outputName := parts[0], parts[1]
		outputs, ok := state.stepOutputs[stepName]
		if !ok || outputs == nil {
			if im.Required {
				return fmt.Errorf("required input %s: source step %s has no outputs", im.Name, stepName)
			}
			if im.Default != "" {
				fnInput.Args[im.Name] = im.Default
			}
			continue
		}

		value, ok := outputs[outputName]
		if !ok {
			if im.Required {
				return fmt.Errorf("required input %s: output %s not found in step %s", im.Name, outputName, stepName)
			}
			if im.Default != "" {
				fnInput.Args[im.Name] = im.Default
			}
			continue
		}

		fnInput.Args[im.Name] = value
	}

	return nil
}

func applyFnDataMapping(fnInput *payload.FunctionExecutionInput, node *payload.DAGNode, state *dagState) {
	if node.DataInput == nil {
		return
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	data, ok := state.stepData[node.DataInput.FromNode]
	if ok {
		fnInput.Data = data
	}
}

func extractFnOutputs(node *payload.DAGNode, result *payload.FunctionExecutionOutput, state *dagState) {
	if len(node.Outputs) == 0 || !result.Success {
		return
	}

	outputs := make(map[string]string)
	for _, om := range node.Outputs {
		value, ok := result.Result[om.ResultKey]
		if !ok {
			if om.Default != "" {
				value = om.Default
			} else {
				continue
			}
		}
		outputs[om.Name] = value
	}

	state.mu.Lock()
	state.stepOutputs[node.Name] = outputs
	state.mu.Unlock()
}

func recordFnNodeResult(nodeName string, result *payload.FunctionExecutionOutput, err error, ctx wf.Context, output *payload.DAGWorkflowOutput, logger interface {
	Info(string, ...interface{})
	Error(string, ...interface{})
}) {
	nodeResult := payload.NodeResult{
		NodeName:  nodeName,
		Result:    result,
		StartTime: wf.Now(ctx),
	}

	if err != nil || !result.Success {
		nodeResult.Success = false
		nodeResult.Error = err
		output.TotalFailed++
		logger.Error("Node failed", "name", nodeName, "error", err)
	} else {
		nodeResult.Success = true
		output.TotalSuccess++
		logger.Info("Node completed", "name", nodeName)
	}

	output.NodeResults = append(output.NodeResults, nodeResult)
}

func dagActivityOptions() wf.ActivityOptions {
	return wf.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/workflow/...`
Expected: PASS

**Step 5: Commit**

```
git add function/workflow/dag.go function/workflow/dag_test.go
git commit -m "feat(function): add DAG workflow implementation"
```

---

### Task 3: DAG OTel Instrumentation

**Files:**
- Create: `function/workflow/dag_otel.go`
- Test: `function/workflow/dag_otel_test.go`

**Step 1: Write the failing test**

```go
// function/workflow/dag_otel_test.go
package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/jasoet/go-wf/function/payload"
)

func TestInstrumentedDAGWorkflow_Success(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	registerFunctionActivity(env)

	env.OnActivity("ExecuteFunctionActivity", mock.Anything, mock.Anything).Return(
		&payload.FunctionExecutionOutput{Success: true, Duration: 1 * time.Second}, nil)

	input := payload.DAGWorkflowInput{
		Nodes: []payload.DAGNode{
			{Name: "build", Function: payload.FunctionExecutionInput{Name: "compile"}},
			{Name: "test", Function: payload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"build"}},
		},
	}

	env.ExecuteWorkflow(InstrumentedDAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var result payload.DAGWorkflowOutput
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.Equal(t, 2, result.TotalSuccess)
}

func TestInstrumentedDAGWorkflow_Failure(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	input := payload.DAGWorkflowInput{Nodes: []payload.DAGNode{}}

	env.ExecuteWorkflow(InstrumentedDAGWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/workflow/...`
Expected: FAIL — InstrumentedDAGWorkflow not defined

**Step 3: Write minimal implementation**

```go
// function/workflow/dag_otel.go
package workflow

import (
	wf "go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/function/payload"
)

// InstrumentedDAGWorkflow wraps DAGWorkflow with structured logging at boundaries.
func InstrumentedDAGWorkflow(ctx wf.Context, input payload.DAGWorkflowInput) (*payload.DAGWorkflowOutput, error) {
	logger := wf.GetLogger(ctx)
	logger.Info("dag.start",
		"node_count", len(input.Nodes),
		"fail_fast", input.FailFast,
		"max_parallel", input.MaxParallel,
	)

	startTime := wf.Now(ctx)
	result, err := DAGWorkflow(ctx, input)

	duration := wf.Now(ctx).Sub(startTime)

	if err != nil {
		logger.Error("dag.failed",
			"error", err,
			"duration", duration,
		)
		return result, err
	}

	logger.Info("dag.complete",
		"node_count", len(input.Nodes),
		"success_count", result.TotalSuccess,
		"failure_count", result.TotalFailed,
		"duration", duration,
	)

	return result, nil
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/workflow/...`
Expected: PASS

**Step 5: Commit**

```
git add function/workflow/dag_otel.go function/workflow/dag_otel_test.go
git commit -m "feat(function): add OTel instrumentation for DAG workflow"
```

---

### Task 4: DAG Builder

**Files:**
- Create: `function/builder/dag_builder.go`
- Test: `function/builder/dag_builder_test.go`

**Step 1: Write the failing test**

```go
// function/builder/dag_builder_test.go
package builder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/function/payload"
)

func TestDAGBuilder_BuildDAG_Simple(t *testing.T) {
	input, err := NewDAGBuilder("simple").
		AddNodeWithInput("build", payload.FunctionExecutionInput{Name: "compile"}).
		AddNodeWithInput("test", payload.FunctionExecutionInput{Name: "run-tests"}, "build").
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Nodes, 2)
	assert.Equal(t, "build", input.Nodes[0].Name)
	assert.Equal(t, []string{"build"}, input.Nodes[1].Dependencies)
}

func TestDAGBuilder_BuildDAG_WithSource(t *testing.T) {
	source := NewFunctionSource(payload.FunctionExecutionInput{Name: "compile"})

	input, err := NewDAGBuilder("with-source").
		AddNode("build", source).
		BuildDAG()

	require.NoError(t, err)
	assert.Len(t, input.Nodes, 1)
	assert.Equal(t, "compile", input.Nodes[0].Function.Name)
}

func TestDAGBuilder_BuildDAG_WithMappings(t *testing.T) {
	input, err := NewDAGBuilder("mapped").
		AddNodeWithInput("build", payload.FunctionExecutionInput{Name: "compile"}).
		AddNodeWithInput("deploy", payload.FunctionExecutionInput{Name: "publish"}, "build").
		WithOutputMapping("build", payload.OutputMapping{Name: "artifact", ResultKey: "path"}).
		WithInputMapping("deploy", payload.InputMapping{Name: "artifact_path", From: "build.artifact"}).
		BuildDAG()

	require.NoError(t, err)
	require.Len(t, input.Nodes, 2)
	assert.Len(t, input.Nodes[0].Outputs, 1)
	assert.Equal(t, "artifact", input.Nodes[0].Outputs[0].Name)
	assert.Len(t, input.Nodes[1].Inputs, 1)
	assert.Equal(t, "build.artifact", input.Nodes[1].Inputs[0].From)
}

func TestDAGBuilder_BuildDAG_WithDataMapping(t *testing.T) {
	input, err := NewDAGBuilder("data-mapped").
		AddNodeWithInput("extract", payload.FunctionExecutionInput{Name: "extract"}).
		AddNodeWithInput("transform", payload.FunctionExecutionInput{Name: "transform"}, "extract").
		WithDataMapping("transform", "extract").
		BuildDAG()

	require.NoError(t, err)
	require.NotNil(t, input.Nodes[1].DataInput)
	assert.Equal(t, "extract", input.Nodes[1].DataInput.FromNode)
}

func TestDAGBuilder_BuildDAG_FailFast(t *testing.T) {
	input, err := NewDAGBuilder("ff").
		AddNodeWithInput("a", payload.FunctionExecutionInput{Name: "fn-a"}).
		FailFast(true).
		BuildDAG()

	require.NoError(t, err)
	assert.True(t, input.FailFast)
}

func TestDAGBuilder_BuildDAG_MaxParallel(t *testing.T) {
	input, err := NewDAGBuilder("mp").
		AddNodeWithInput("a", payload.FunctionExecutionInput{Name: "fn-a"}).
		MaxParallel(3).
		BuildDAG()

	require.NoError(t, err)
	assert.Equal(t, 3, input.MaxParallel)
}

func TestDAGBuilder_BuildDAG_Empty(t *testing.T) {
	_, err := NewDAGBuilder("empty").BuildDAG()
	assert.Error(t, err)
}

func TestDAGBuilder_BuildDAG_NilSource(t *testing.T) {
	_, err := NewDAGBuilder("nil").
		AddNode("broken", nil).
		BuildDAG()
	assert.Error(t, err)
}

func TestDAGBuilder_BuildDAG_MappingUnknownNode(t *testing.T) {
	_, err := NewDAGBuilder("unknown").
		AddNodeWithInput("a", payload.FunctionExecutionInput{Name: "fn-a"}).
		WithOutputMapping("nonexistent", payload.OutputMapping{Name: "x", ResultKey: "y"}).
		BuildDAG()
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/builder/...`
Expected: FAIL — NewDAGBuilder not defined

**Step 3: Write minimal implementation**

```go
// function/builder/dag_builder.go
package builder

import (
	"fmt"

	"github.com/jasoet/go-wf/function/payload"
)

// DAGBuilder provides a fluent API for constructing DAG workflow inputs.
type DAGBuilder struct {
	name        string
	nodes       []payload.DAGNode
	nodeIndex   map[string]int
	failFast    bool
	maxParallel int
	errors      []error
}

// NewDAGBuilder creates a new DAG builder.
func NewDAGBuilder(name string) *DAGBuilder {
	return &DAGBuilder{
		name:      name,
		nodes:     make([]payload.DAGNode, 0),
		nodeIndex: make(map[string]int),
	}
}

// AddNode adds a node from a WorkflowSource with optional dependencies.
func (b *DAGBuilder) AddNode(name string, source WorkflowSource, deps ...string) *DAGBuilder {
	if source == nil {
		b.errors = append(b.errors, fmt.Errorf("cannot add nil source for node %s", name))
		return b
	}
	return b.AddNodeWithInput(name, source.ToInput(), deps...)
}

// AddNodeWithInput adds a node from a raw FunctionExecutionInput with optional dependencies.
func (b *DAGBuilder) AddNodeWithInput(name string, input payload.FunctionExecutionInput, deps ...string) *DAGBuilder {
	b.nodeIndex[name] = len(b.nodes)
	b.nodes = append(b.nodes, payload.DAGNode{
		Name:         name,
		Function:     input,
		Dependencies: deps,
	})
	return b
}

// WithOutputMapping adds output mappings to a node.
func (b *DAGBuilder) WithOutputMapping(nodeName string, mappings ...payload.OutputMapping) *DAGBuilder {
	idx, ok := b.nodeIndex[nodeName]
	if !ok {
		b.errors = append(b.errors, fmt.Errorf("node %s not found for output mapping", nodeName))
		return b
	}
	b.nodes[idx].Outputs = append(b.nodes[idx].Outputs, mappings...)
	return b
}

// WithInputMapping adds input mappings to a node.
func (b *DAGBuilder) WithInputMapping(nodeName string, mappings ...payload.InputMapping) *DAGBuilder {
	idx, ok := b.nodeIndex[nodeName]
	if !ok {
		b.errors = append(b.errors, fmt.Errorf("node %s not found for input mapping", nodeName))
		return b
	}
	b.nodes[idx].Inputs = append(b.nodes[idx].Inputs, mappings...)
	return b
}

// WithDataMapping sets data mapping for a node.
func (b *DAGBuilder) WithDataMapping(nodeName string, fromNode string) *DAGBuilder {
	idx, ok := b.nodeIndex[nodeName]
	if !ok {
		b.errors = append(b.errors, fmt.Errorf("node %s not found for data mapping", nodeName))
		return b
	}
	b.nodes[idx].DataInput = &payload.DataMapping{FromNode: fromNode}
	return b
}

// FailFast configures fail-fast behavior.
func (b *DAGBuilder) FailFast(ff bool) *DAGBuilder {
	b.failFast = ff
	return b
}

// MaxParallel sets maximum parallel node executions.
func (b *DAGBuilder) MaxParallel(max int) *DAGBuilder {
	b.maxParallel = max
	return b
}

// BuildDAG creates the DAG workflow input.
func (b *DAGBuilder) BuildDAG() (*payload.DAGWorkflowInput, error) {
	if len(b.errors) > 0 {
		return nil, b.errors[0]
	}

	if len(b.nodes) == 0 {
		return nil, fmt.Errorf("DAG workflow requires at least one node")
	}

	input := &payload.DAGWorkflowInput{
		Nodes:       b.nodes,
		FailFast:    b.failFast,
		MaxParallel: b.maxParallel,
	}

	if err := input.Validate(); err != nil {
		return nil, fmt.Errorf("DAG validation failed: %w", err)
	}

	return input, nil
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/builder/...`
Expected: PASS

**Step 5: Commit**

```
git add function/builder/dag_builder.go function/builder/dag_builder_test.go
git commit -m "feat(function): add DAG builder with fluent API"
```

---

### Task 5: Register DAG Workflow

**Files:**
- Modify: `function/worker.go:33-39` (add DAGWorkflow registration)
- Modify: `function/worker_test.go` (verify registration)

**Step 1: Update RegisterWorkflows**

Add `wf.InstrumentedDAGWorkflow` to the `RegisterWorkflows` function in `function/worker.go`:

```go
func RegisterWorkflows(w WorkflowRegistrar) {
	w.RegisterWorkflow(wf.ExecuteFunctionWorkflow)
	w.RegisterWorkflow(wf.FunctionPipelineWorkflow)
	w.RegisterWorkflow(wf.ParallelFunctionsWorkflow)
	w.RegisterWorkflow(wf.LoopWorkflow)
	w.RegisterWorkflow(wf.ParameterizedLoopWorkflow)
	w.RegisterWorkflow(wf.InstrumentedDAGWorkflow)
}
```

**Step 2: Run existing tests to verify nothing breaks**

Run: `task test:pkg -- ./function/...`
Expected: PASS

**Step 3: Commit**

```
git add function/worker.go
git commit -m "feat(function): register DAG workflow with Temporal worker"
```

---

### Task 6: Worker Handlers + Trigger Updates

**Files:**
- Modify: `examples/function/worker/main.go` (add 3 handlers)
- Modify: `examples/trigger/main.go` (add DAG workflows + schedule)

**Step 1: Add new handlers to worker**

Add to `registerAllHandlers` in `examples/function/worker/main.go`, before the final `log.Printf`:

```go
	// --- DAG-specific handlers (3 handlers) ---

	registry.Register("compile", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[compile] Compiling application...")
		time.Sleep(300 * time.Millisecond) // Simulate compilation
		return &fn.FunctionOutput{
			Result: map[string]string{"artifact": "app-binary", "status": "compiled", "version": "1.0.0"},
		}, nil
	})

	registry.Register("run-tests", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		log.Println("[run-tests] Running test suite...")
		time.Sleep(500 * time.Millisecond) // Simulate test execution
		return &fn.FunctionOutput{
			Result: map[string]string{"passed": "42", "failed": "0", "skipped": "3"},
		}, nil
	})

	registry.Register("publish-artifact", func(ctx context.Context, input fn.FunctionInput) (*fn.FunctionOutput, error) {
		artifactPath := input.Args["artifact_path"]
		if artifactPath == "" {
			artifactPath = "default-artifact"
		}
		log.Printf("[publish-artifact] Publishing artifact: %s", artifactPath)
		return &fn.FunctionOutput{
			Result: map[string]string{"published": "true", "registry": "artifacts.example.com", "artifact": artifactPath},
		}, nil
	})
```

Update the handler count: `log.Printf("Registered %d handler functions", 21)`

**Step 2: Add DAG submissions to trigger**

Add imports at top of `examples/trigger/main.go` (if not already present — the existing imports cover the needed packages).

Add to `runAll()` after the existing function workflows:

```go
	// 6. DAG — ETL with validation
	track(submit(ctx, c, fmt.Sprintf("demo-fn-dag-etl-%s", ts), fnQueue,
		fnwf.InstrumentedDAGWorkflow,
		fnpayload.DAGWorkflowInput{
			Nodes: []fnpayload.DAGNode{
				{Name: "validate-config", Function: fnpayload.FunctionExecutionInput{
					Name: "validate-config", Args: map[string]string{"env": "production"},
				}},
				{Name: "extract", Function: fnpayload.FunctionExecutionInput{
					Name: "extract", Args: map[string]string{"source": "database"},
				}},
				{Name: "transform", Function: fnpayload.FunctionExecutionInput{
					Name: "etl-transform", Args: map[string]string{"format": "parquet"},
				}, Dependencies: []string{"validate-config", "extract"}},
				{Name: "load", Function: fnpayload.FunctionExecutionInput{
					Name: "load", Args: map[string]string{"target": "warehouse"},
				}, Dependencies: []string{"transform"}},
			},
			FailFast: true,
		}))

	// 7. DAG — CI Pipeline
	track(submit(ctx, c, fmt.Sprintf("demo-fn-dag-ci-%s", ts), fnQueue,
		fnwf.InstrumentedDAGWorkflow,
		fnpayload.DAGWorkflowInput{
			Nodes: []fnpayload.DAGNode{
				{
					Name:     "compile",
					Function: fnpayload.FunctionExecutionInput{Name: "compile"},
					Outputs:  []fnpayload.OutputMapping{{Name: "artifact", ResultKey: "artifact"}},
				},
				{Name: "unit-test", Function: fnpayload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"compile"}},
				{Name: "lint", Function: fnpayload.FunctionExecutionInput{Name: "validate-config", Args: map[string]string{"env": "ci"}}, Dependencies: []string{"compile"}},
				{
					Name:         "publish",
					Function:     fnpayload.FunctionExecutionInput{Name: "publish-artifact", Args: map[string]string{}},
					Dependencies: []string{"unit-test", "lint"},
					Inputs:       []fnpayload.InputMapping{{Name: "artifact_path", From: "compile.artifact"}},
				},
			},
			FailFast: true,
		}))
```

**Step 3: Add DAG schedule**

Add to `schedules` slice in `createSchedules()`:

```go
		{
			ID:           "schedule-fn-dag-ci",
			Interval:     15 * time.Minute,
			WorkflowID:   "scheduled-fn-dag-ci",
			WorkflowFunc: fnwf.InstrumentedDAGWorkflow,
			TaskQueue:    "function-tasks",
			Input: fnpayload.DAGWorkflowInput{
				Nodes: []fnpayload.DAGNode{
					{
						Name:     "compile",
						Function: fnpayload.FunctionExecutionInput{Name: "compile"},
						Outputs:  []fnpayload.OutputMapping{{Name: "artifact", ResultKey: "artifact"}},
					},
					{Name: "unit-test", Function: fnpayload.FunctionExecutionInput{Name: "run-tests"}, Dependencies: []string{"compile"}},
					{Name: "lint", Function: fnpayload.FunctionExecutionInput{Name: "validate-config", Args: map[string]string{"env": "ci"}}, Dependencies: []string{"compile"}},
					{
						Name:         "publish",
						Function:     fnpayload.FunctionExecutionInput{Name: "publish-artifact", Args: map[string]string{}},
						Dependencies: []string{"unit-test", "lint"},
						Inputs:       []fnpayload.InputMapping{{Name: "artifact_path", From: "compile.artifact"}},
					},
				},
				FailFast: true,
			},
		},
```

**Step 4: Add to cleanSchedules**

Add `"schedule-fn-dag-ci"` to the `scheduleIDs` slice in `cleanSchedules()`.

**Step 5: Build and verify**

Run: `task test:unit`
Expected: PASS

**Step 6: Commit**

```
git add examples/function/worker/main.go examples/trigger/main.go
git commit -m "feat(function): add DAG examples to worker and trigger CLI"
```

---

## Slice 2: Patterns Package

### Task 7: Pipeline Patterns

**Files:**
- Create: `function/patterns/pipeline.go`
- Test: `function/patterns/pipeline_test.go`

**Step 1: Write the failing test**

```go
// function/patterns/pipeline_test.go
package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestETLPipeline(t *testing.T) {
	input, err := ETLPipeline("database", "parquet", "warehouse")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Functions, 3)
	assert.True(t, input.StopOnError)
	assert.Equal(t, "extract", input.Functions[0].Name)
	assert.Equal(t, "etl-transform", input.Functions[1].Name)
	assert.Equal(t, "load", input.Functions[2].Name)
}

func TestValidateTransformNotify(t *testing.T) {
	input, err := ValidateTransformNotify("user@example.com", "Demo", "slack")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Functions, 3)
	assert.True(t, input.StopOnError)
	assert.Equal(t, "validate", input.Functions[0].Name)
	assert.Equal(t, "transform", input.Functions[1].Name)
	assert.Equal(t, "notify", input.Functions[2].Name)
}

func TestMultiEnvironmentDeploy(t *testing.T) {
	input, err := MultiEnvironmentDeploy("v1.2.3", []string{"staging", "production"})
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Functions, 2)
	assert.True(t, input.StopOnError)
}

func TestMultiEnvironmentDeploy_Empty(t *testing.T) {
	_, err := MultiEnvironmentDeploy("v1.0", []string{})
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/patterns/...`
Expected: FAIL — package doesn't exist

**Step 3: Write minimal implementation**

```go
// function/patterns/pipeline.go
package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// ETLPipeline creates an extract-transform-load pipeline.
func ETLPipeline(source, format, target string) (*payload.PipelineInput, error) {
	return builder.NewWorkflowBuilder("etl-pipeline").
		AddInput(payload.FunctionExecutionInput{
			Name: "extract",
			Args: map[string]string{"source": source},
		}).
		AddInput(payload.FunctionExecutionInput{
			Name: "etl-transform",
			Args: map[string]string{"format": format},
		}).
		AddInput(payload.FunctionExecutionInput{
			Name: "load",
			Args: map[string]string{"target": target},
		}).
		StopOnError(true).
		BuildPipeline()
}

// ValidateTransformNotify creates a validate-transform-notify pipeline.
func ValidateTransformNotify(email, name, channel string) (*payload.PipelineInput, error) {
	return builder.NewWorkflowBuilder("validate-transform-notify").
		AddInput(payload.FunctionExecutionInput{
			Name: "validate",
			Args: map[string]string{"email": email, "name": name},
		}).
		AddInput(payload.FunctionExecutionInput{
			Name: "transform",
			Args: map[string]string{"name": name, "email": email},
		}).
		AddInput(payload.FunctionExecutionInput{
			Name: "notify",
			Args: map[string]string{"name": name, "channel": channel},
		}).
		StopOnError(true).
		BuildPipeline()
}

// MultiEnvironmentDeploy creates a sequential deployment to multiple environments.
func MultiEnvironmentDeploy(version string, environments []string) (*payload.PipelineInput, error) {
	if len(environments) == 0 {
		return nil, fmt.Errorf("at least one environment is required")
	}

	wb := builder.NewWorkflowBuilder("multi-env-deploy")
	for _, env := range environments {
		wb.AddInput(payload.FunctionExecutionInput{
			Name: "deploy-service",
			Args: map[string]string{"environment": env, "version": version},
		})
	}

	return wb.StopOnError(true).BuildPipeline()
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/patterns/...`
Expected: PASS

**Step 5: Commit**

```
git add function/patterns/pipeline.go function/patterns/pipeline_test.go
git commit -m "feat(function): add pipeline patterns"
```

---

### Task 8: Parallel Patterns

**Files:**
- Create: `function/patterns/parallel.go`
- Test: `function/patterns/parallel_test.go`

**Step 1: Write the failing test**

```go
// function/patterns/parallel_test.go
package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFanOutFanIn(t *testing.T) {
	input, err := FanOutFanIn([]string{"fetch-users", "fetch-orders", "fetch-inventory"})
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Functions, 3)
}

func TestFanOutFanIn_Empty(t *testing.T) {
	_, err := FanOutFanIn([]string{})
	assert.Error(t, err)
}

func TestParallelDataFetch(t *testing.T) {
	input, err := ParallelDataFetch()
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Functions, 3)
	assert.Equal(t, "fetch-users", input.Functions[0].Name)
	assert.Equal(t, "fetch-orders", input.Functions[1].Name)
	assert.Equal(t, "fetch-inventory", input.Functions[2].Name)
}

func TestParallelHealthCheck(t *testing.T) {
	input, err := ParallelHealthCheck([]string{"api", "db", "cache"}, "production")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Functions, 3)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestParallelHealthCheck_Empty(t *testing.T) {
	_, err := ParallelHealthCheck([]string{}, "prod")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/patterns/...`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// function/patterns/parallel.go
package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// FanOutFanIn runs multiple named functions in parallel.
func FanOutFanIn(functionNames []string) (*payload.ParallelInput, error) {
	if len(functionNames) == 0 {
		return nil, fmt.Errorf("at least one function is required")
	}

	wb := builder.NewWorkflowBuilder("fan-out-fan-in").Parallel(true)
	for _, name := range functionNames {
		wb.AddInput(payload.FunctionExecutionInput{Name: name})
	}

	return wb.BuildParallel()
}

// ParallelDataFetch fetches users, orders, and inventory in parallel.
func ParallelDataFetch() (*payload.ParallelInput, error) {
	return builder.NewWorkflowBuilder("data-fetch").
		Parallel(true).
		AddInput(payload.FunctionExecutionInput{Name: "fetch-users"}).
		AddInput(payload.FunctionExecutionInput{Name: "fetch-orders"}).
		AddInput(payload.FunctionExecutionInput{Name: "fetch-inventory"}).
		BuildParallel()
}

// ParallelHealthCheck runs health checks for multiple services concurrently.
func ParallelHealthCheck(services []string, env string) (*payload.ParallelInput, error) {
	if len(services) == 0 {
		return nil, fmt.Errorf("at least one service is required")
	}

	wb := builder.NewWorkflowBuilder("health-check").Parallel(true).FailFast(true)
	for _, service := range services {
		wb.AddInput(payload.FunctionExecutionInput{
			Name: "health-check",
			Args: map[string]string{"service": service, "environment": env},
		})
	}

	return wb.BuildParallel()
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/patterns/...`
Expected: PASS

**Step 5: Commit**

```
git add function/patterns/parallel.go function/patterns/parallel_test.go
git commit -m "feat(function): add parallel patterns"
```

---

### Task 9: Loop Patterns

**Files:**
- Create: `function/patterns/loop.go`
- Test: `function/patterns/loop_test.go`

**Step 1: Write the failing test**

```go
// function/patterns/loop_test.go
package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBatchProcess(t *testing.T) {
	input, err := BatchProcess([]string{"file1.csv", "file2.csv"}, "process-csv")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, []string{"file1.csv", "file2.csv"}, input.Items)
	assert.Equal(t, "process-csv", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, "continue", input.FailureStrategy)
}

func TestBatchProcess_Empty(t *testing.T) {
	_, err := BatchProcess([]string{}, "process-csv")
	assert.Error(t, err)
}

func TestSequentialMigration(t *testing.T) {
	input, err := SequentialMigration([]string{"001_create_users", "002_add_email"})
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, "run-migration", input.Template.Name)
	assert.False(t, input.Parallel)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestSequentialMigration_Empty(t *testing.T) {
	_, err := SequentialMigration([]string{})
	assert.Error(t, err)
}

func TestMultiRegionDeploy(t *testing.T) {
	input, err := MultiRegionDeploy([]string{"dev", "staging"}, []string{"us-east-1", "eu-west-1"}, "v1.2.3")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, []string{"dev", "staging"}, input.Parameters["environment"])
	assert.Equal(t, []string{"us-east-1", "eu-west-1"}, input.Parameters["region"])
	assert.Equal(t, "deploy-service", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, "fail_fast", input.FailureStrategy)
}

func TestMultiRegionDeploy_EmptyEnvironments(t *testing.T) {
	_, err := MultiRegionDeploy([]string{}, []string{"us-east-1"}, "v1.0")
	assert.Error(t, err)
}

func TestMultiRegionDeploy_EmptyRegions(t *testing.T) {
	_, err := MultiRegionDeploy([]string{"prod"}, []string{}, "v1.0")
	assert.Error(t, err)
}

func TestParameterSweep(t *testing.T) {
	params := map[string][]string{
		"learning_rate": {"0.001", "0.01"},
		"batch_size":    {"32", "64"},
	}
	input, err := ParameterSweep(params, "train-model", 3)
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Equal(t, params, input.Parameters)
	assert.Equal(t, "train-model", input.Template.Name)
	assert.True(t, input.Parallel)
	assert.Equal(t, 3, input.MaxConcurrency)
	assert.Equal(t, "continue", input.FailureStrategy)
}

func TestParameterSweep_Empty(t *testing.T) {
	_, err := ParameterSweep(map[string][]string{}, "train", 1)
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/patterns/...`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// function/patterns/loop.go
package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// BatchProcess creates a parallel loop over items with the given function.
func BatchProcess(items []string, functionName string) (*payload.LoopInput, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("at least one item is required")
	}

	return builder.NewLoopBuilder(items).
		WithTemplate(payload.FunctionExecutionInput{
			Name: functionName,
			Args: map[string]string{"file": "{{item}}"},
		}).
		Parallel(true).
		BuildLoop()
}

// SequentialMigration creates a sequential, fail-fast loop for database migrations.
func SequentialMigration(migrations []string) (*payload.LoopInput, error) {
	if len(migrations) == 0 {
		return nil, fmt.Errorf("at least one migration is required")
	}

	return builder.NewLoopBuilder(migrations).
		WithTemplate(payload.FunctionExecutionInput{
			Name: "run-migration",
			Args: map[string]string{"migration": "{{item}}"},
		}).
		FailFast(true).
		BuildLoop()
}

// MultiRegionDeploy creates a parameterized deploy across environments and regions.
func MultiRegionDeploy(environments, regions []string, version string) (*payload.ParameterizedLoopInput, error) {
	if len(environments) == 0 {
		return nil, fmt.Errorf("at least one environment is required")
	}
	if len(regions) == 0 {
		return nil, fmt.Errorf("at least one region is required")
	}

	return builder.NewParameterizedLoopBuilder(map[string][]string{
		"environment": environments,
		"region":      regions,
	}).
		WithTemplate(payload.FunctionExecutionInput{
			Name: "deploy-service",
			Args: map[string]string{"version": version, "environment": "{{.environment}}", "region": "{{.region}}"},
		}).
		Parallel(true).
		FailFast(true).
		BuildParameterizedLoop()
}

// ParameterSweep creates a parameterized loop for experimentation.
func ParameterSweep(params map[string][]string, functionName string, maxConcurrency int) (*payload.ParameterizedLoopInput, error) {
	if len(params) == 0 {
		return nil, fmt.Errorf("at least one parameter is required")
	}

	return builder.NewParameterizedLoopBuilder(params).
		WithTemplate(payload.FunctionExecutionInput{Name: functionName}).
		Parallel(true).
		MaxConcurrency(maxConcurrency).
		BuildParameterizedLoop()
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/patterns/...`
Expected: PASS

**Step 5: Commit**

```
git add function/patterns/loop.go function/patterns/loop_test.go
git commit -m "feat(function): add loop patterns"
```

---

### Task 10: DAG Patterns

**Files:**
- Create: `function/patterns/dag.go`
- Test: `function/patterns/dag_test.go`

**Step 1: Write the failing test**

```go
// function/patterns/dag_test.go
package patterns

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestETLWithValidation(t *testing.T) {
	input, err := ETLWithValidation("database", "parquet", "warehouse")
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Nodes, 4) // validate-config, extract, transform, load
	assert.True(t, input.FailFast)

	// validate-config and extract have no deps
	nodeMap := make(map[string][]string)
	for _, n := range input.Nodes {
		nodeMap[n.Name] = n.Dependencies
	}
	assert.Empty(t, nodeMap["validate-config"])
	assert.Empty(t, nodeMap["extract"])
	assert.ElementsMatch(t, []string{"validate-config", "extract"}, nodeMap["transform"])
	assert.Equal(t, []string{"transform"}, nodeMap["load"])
}

func TestCIPipeline(t *testing.T) {
	input, err := CIPipeline()
	require.NoError(t, err)
	require.NotNil(t, input)
	assert.Len(t, input.Nodes, 4) // compile, unit-test, lint, publish
	assert.True(t, input.FailFast)

	nodeMap := make(map[string][]string)
	for _, n := range input.Nodes {
		nodeMap[n.Name] = n.Dependencies
	}
	assert.Empty(t, nodeMap["compile"])
	assert.Equal(t, []string{"compile"}, nodeMap["unit-test"])
	assert.Equal(t, []string{"compile"}, nodeMap["lint"])
	assert.ElementsMatch(t, []string{"unit-test", "lint"}, nodeMap["publish"])
}

func TestCIPipeline_HasOutputMapping(t *testing.T) {
	input, err := CIPipeline()
	require.NoError(t, err)

	// compile node should have output mapping
	var compileNode, publishNode *struct{ outputs, inputs int }
	for _, n := range input.Nodes {
		if n.Name == "compile" {
			compileNode = &struct{ outputs, inputs int }{len(n.Outputs), len(n.Inputs)}
		}
		if n.Name == "publish" {
			publishNode = &struct{ outputs, inputs int }{len(n.Outputs), len(n.Inputs)}
		}
	}
	require.NotNil(t, compileNode)
	require.NotNil(t, publishNode)
	assert.Equal(t, 1, compileNode.outputs)
	assert.Equal(t, 1, publishNode.inputs)
}
```

**Step 2: Run test to verify it fails**

Run: `task test:pkg -- ./function/patterns/...`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// function/patterns/dag.go
package patterns

import (
	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// ETLWithValidation creates a DAG where validate-config and extract run in parallel,
// both feed into transform, then load.
func ETLWithValidation(source, format, target string) (*payload.DAGWorkflowInput, error) {
	return builder.NewDAGBuilder("etl-with-validation").
		AddNodeWithInput("validate-config", payload.FunctionExecutionInput{
			Name: "validate-config",
			Args: map[string]string{"env": "production"},
		}).
		AddNodeWithInput("extract", payload.FunctionExecutionInput{
			Name: "extract",
			Args: map[string]string{"source": source},
		}).
		AddNodeWithInput("transform", payload.FunctionExecutionInput{
			Name: "etl-transform",
			Args: map[string]string{"format": format},
		}, "validate-config", "extract").
		AddNodeWithInput("load", payload.FunctionExecutionInput{
			Name: "load",
			Args: map[string]string{"target": target},
		}, "transform").
		FailFast(true).
		BuildDAG()
}

// CIPipeline creates a DAG: compile → (unit-test ∥ lint) → publish,
// with output mapping from compile to publish.
func CIPipeline() (*payload.DAGWorkflowInput, error) {
	return builder.NewDAGBuilder("ci-pipeline").
		AddNodeWithInput("compile", payload.FunctionExecutionInput{Name: "compile"}).
		AddNodeWithInput("unit-test", payload.FunctionExecutionInput{Name: "run-tests"}, "compile").
		AddNodeWithInput("lint", payload.FunctionExecutionInput{
			Name: "validate-config",
			Args: map[string]string{"env": "ci"},
		}, "compile").
		AddNodeWithInput("publish", payload.FunctionExecutionInput{
			Name: "publish-artifact",
			Args: map[string]string{},
		}, "unit-test", "lint").
		WithOutputMapping("compile", payload.OutputMapping{Name: "artifact", ResultKey: "artifact"}).
		WithInputMapping("publish", payload.InputMapping{Name: "artifact_path", From: "compile.artifact"}).
		FailFast(true).
		BuildDAG()
}
```

**Step 4: Run test to verify it passes**

Run: `task test:pkg -- ./function/patterns/...`
Expected: PASS

**Step 5: Commit**

```
git add function/patterns/dag.go function/patterns/dag_test.go
git commit -m "feat(function): add DAG patterns"
```

---

### Task 11: Final Verification

**Step 1: Run full unit test suite**

Run: `task test:unit`
Expected: PASS — all existing + new tests pass

**Step 2: Run linter**

Run: `task lint`
Expected: PASS

**Step 3: Run fmt**

Run: `task fmt`

**Step 4: Commit any formatting changes**

```
git add -A
git commit -m "style(function): format new files"
```

(Only if fmt made changes)

**Step 5: Update INSTRUCTION.md and README.md**

Add to INSTRUCTION.md key paths table:
- `function/patterns/` — Pre-built patterns (pipeline, parallel, loop, DAG)

Add to README.md function module section: mention DAG workflow and patterns.

**Step 6: Commit docs**

```
git add INSTRUCTION.md README.md
git commit -m "docs: update INSTRUCTION and README with function DAG and patterns"
```
