package payload

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputMapping_Fields(t *testing.T) {
	om := OutputMapping{
		Name:      "build_id",
		ResultKey: "id",
		Default:   "unknown",
	}
	assert.Equal(t, "build_id", om.Name)
	assert.Equal(t, "id", om.ResultKey)
	assert.Equal(t, "unknown", om.Default)
}

func TestInputMapping_Fields(t *testing.T) {
	im := FunctionInputMapping{
		Name:     "SOURCE_ID",
		From:     "build.build_id",
		Default:  "none",
		Required: true,
	}
	assert.Equal(t, "SOURCE_ID", im.Name)
	assert.Equal(t, "build.build_id", im.From)
	assert.Equal(t, "none", im.Default)
	assert.True(t, im.Required)
}

func TestDataMapping_Fields(t *testing.T) {
	dm := DataMapping{
		FromNode: "step-1",
		Optional: true,
	}
	assert.Equal(t, "step-1", dm.FromNode)
	assert.True(t, dm.Optional)
}

func TestDAGWorkflowInput_ValidInput(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []FunctionDAGNode{
			{
				Name: "step-1",
				Function: FunctionExecutionInput{
					Name: "build",
				},
			},
			{
				Name: "step-2",
				Function: FunctionExecutionInput{
					Name: "test",
				},
				Dependencies: []string{"step-1"},
			},
		},
		FailFast:    true,
		MaxParallel: 2,
	}

	err := input.Validate()
	require.NoError(t, err)
}

func TestDAGWorkflowInput_EmptyNodes(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []FunctionDAGNode{},
	}

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one node is required")
}

func TestDAGWorkflowInput_MissingDependency(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []FunctionDAGNode{
			{
				Name: "step-1",
				Function: FunctionExecutionInput{
					Name: "build",
				},
				Dependencies: []string{"nonexistent"},
			},
		},
	}

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "dependency node not found")
}

func TestDAGWorkflowInput_CircularDependency(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []FunctionDAGNode{
			{
				Name: "A",
				Function: FunctionExecutionInput{
					Name: "funcA",
				},
				Dependencies: []string{"B"},
			},
			{
				Name: "B",
				Function: FunctionExecutionInput{
					Name: "funcB",
				},
				Dependencies: []string{"A"},
			},
		},
	}

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circular dependency")
}

func TestDAGWorkflowInput_DuplicateNodeNames(t *testing.T) {
	input := DAGWorkflowInput{
		Nodes: []FunctionDAGNode{
			{
				Name: "step-1",
				Function: FunctionExecutionInput{
					Name: "build",
				},
			},
			{
				Name: "step-1",
				Function: FunctionExecutionInput{
					Name: "test",
				},
			},
		},
	}

	err := input.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate node name")
}

func TestFunctionDAGNode_Fields(t *testing.T) {
	node := FunctionDAGNode{
		Name: "build",
		Function: FunctionExecutionInput{
			Name: "build-func",
			Args: map[string]string{"version": "1.0"},
		},
		Dependencies: []string{"setup"},
		Outputs: []OutputMapping{
			{Name: "artifact", ResultKey: "path"},
		},
		Inputs: []FunctionInputMapping{
			{Name: "CONFIG", From: "setup.config_path"},
		},
		DataInput: &DataMapping{
			FromNode: "setup",
			Optional: false,
		},
	}

	assert.Equal(t, "build", node.Name)
	assert.Equal(t, "build-func", node.Function.Name)
	assert.Equal(t, []string{"setup"}, node.Dependencies)
	assert.Len(t, node.Outputs, 1)
	assert.Len(t, node.Inputs, 1)
	assert.NotNil(t, node.DataInput)
	assert.Equal(t, "setup", node.DataInput.FromNode)
}

func TestFunctionNodeResult_Fields(t *testing.T) {
	now := time.Now()
	nr := FunctionNodeResult{
		NodeName:  "step-1",
		Result:    &FunctionExecutionOutput{Name: "step-1", Success: true},
		StartTime: now,
		Success:   true,
		Error:     nil,
	}

	assert.Equal(t, "step-1", nr.NodeName)
	assert.True(t, nr.Success)
	assert.Nil(t, nr.Error)
	assert.Equal(t, now, nr.StartTime)
	assert.NotNil(t, nr.Result)
}

func TestFunctionDAGWorkflowOutput_Fields(t *testing.T) {
	output := FunctionDAGWorkflowOutput{
		Results: map[string]*FunctionExecutionOutput{
			"step-1": {Name: "step-1", Success: true},
		},
		NodeResults: []FunctionNodeResult{
			{NodeName: "step-1", Success: true},
		},
		StepOutputs: map[string]map[string]string{
			"step-1": {"artifact": "/tmp/out"},
		},
		TotalSuccess:  1,
		TotalFailed:   0,
		TotalDuration: 5 * time.Second,
	}

	assert.Len(t, output.Results, 1)
	assert.Len(t, output.NodeResults, 1)
	assert.Len(t, output.StepOutputs, 1)
	assert.Equal(t, "/tmp/out", output.StepOutputs["step-1"]["artifact"])
	assert.Equal(t, 1, output.TotalSuccess)
	assert.Equal(t, 0, output.TotalFailed)
	assert.Equal(t, 5*time.Second, output.TotalDuration)
}
