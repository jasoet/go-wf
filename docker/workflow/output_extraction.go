package workflow

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// ExtractOutput extracts a value from container output based on the definition.
func ExtractOutput(def payload.OutputDefinition, containerOutput *payload.ContainerExecutionOutput) (string, error) {
	var rawValue string
	var err error

	// Extract based on ValueFrom
	switch def.ValueFrom {
	case "stdout":
		rawValue = containerOutput.Stdout
	case "stderr":
		rawValue = containerOutput.Stderr
	case "exitCode":
		rawValue = strconv.Itoa(containerOutput.ExitCode)
	case "file":
		if def.Path == "" {
			return def.Default, fmt.Errorf("path is required when value_from is 'file'")
		}
		rawValue, err = generic.ReadFile(def.Path)
		if err != nil {
			if def.Default != "" {
				return def.Default, nil
			}
			return "", fmt.Errorf("failed to read file %s: %w", def.Path, err)
		}
	default:
		return def.Default, fmt.Errorf("unknown value_from: %s", def.ValueFrom)
	}

	// Apply JSONPath extraction if specified
	if def.JSONPath != "" {
		rawValue, err = generic.ExtractJSONPath(rawValue, def.JSONPath)
		if err != nil {
			if def.Default != "" {
				return def.Default, nil
			}
			return "", fmt.Errorf("failed to extract JSONPath %s: %w", def.JSONPath, err)
		}
	}

	// Apply regex extraction if specified
	if def.Regex != "" {
		rawValue, err = generic.ExtractRegex(rawValue, def.Regex)
		if err != nil {
			if def.Default != "" {
				return def.Default, nil
			}
			return "", fmt.Errorf("failed to extract regex %s: %w", def.Regex, err)
		}
	}

	// Trim whitespace
	rawValue = strings.TrimSpace(rawValue)

	// Return default if empty
	if rawValue == "" && def.Default != "" {
		return def.Default, nil
	}

	return rawValue, nil
}

// ExtractOutputs extracts all outputs defined in the list.
func ExtractOutputs(definitions []payload.OutputDefinition, containerOutput *payload.ContainerExecutionOutput) (map[string]string, error) {
	outputs := make(map[string]string, len(definitions))

	for _, def := range definitions {
		value, err := ExtractOutput(def, containerOutput)
		if err != nil {
			return nil, fmt.Errorf("failed to extract output %s: %w", def.Name, err)
		}
		outputs[def.Name] = value
	}

	return outputs, nil
}

// SubstituteInputs applies input mappings to container environment variables.
// It resolves step outputs and substitutes them into the container input.
func SubstituteInputs(containerInput *payload.ContainerExecutionInput, inputs []payload.InputMapping, stepOutputs map[string]map[string]string) error {
	if containerInput.Env == nil {
		containerInput.Env = make(map[string]string)
	}

	for _, input := range inputs {
		value, err := resolveInputMapping(input, stepOutputs)
		if err != nil {
			if input.Required {
				return fmt.Errorf("failed to resolve required input %s: %w", input.Name, err)
			}
			// Use default if not required
			if input.Default != "" {
				containerInput.Env[input.Name] = input.Default
			}
			continue
		}

		containerInput.Env[input.Name] = value
	}

	return nil
}

// resolveInputMapping resolves an input mapping from step outputs,
// using the format: "step-name.output-name".
func resolveInputMapping(mapping payload.InputMapping, stepOutputs map[string]map[string]string) (string, error) {
	// Parse "step-name.output-name"
	parts := strings.SplitN(mapping.From, ".", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid input mapping format: %s (expected step-name.output-name)", mapping.From)
	}

	stepName := parts[0]
	outputName := parts[1]

	// Get step outputs
	outputs, exists := stepOutputs[stepName]
	if !exists {
		return mapping.Default, fmt.Errorf("step %s not found in outputs", stepName)
	}

	// Get specific output
	value, exists := outputs[outputName]
	if !exists {
		return mapping.Default, fmt.Errorf("output %s not found in step %s", outputName, stepName)
	}

	return value, nil
}
