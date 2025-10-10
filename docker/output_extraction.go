package docker

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

// ExtractOutput extracts a value from container output based on the definition.
func ExtractOutput(def OutputDefinition, containerOutput *ContainerExecutionOutput) (string, error) {
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
		rawValue, err = readFile(def.Path)
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
		rawValue, err = extractJSONPath(rawValue, def.JSONPath)
		if err != nil {
			if def.Default != "" {
				return def.Default, nil
			}
			return "", fmt.Errorf("failed to extract JSONPath %s: %w", def.JSONPath, err)
		}
	}

	// Apply regex extraction if specified
	if def.Regex != "" {
		rawValue, err = extractRegex(rawValue, def.Regex)
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
func ExtractOutputs(definitions []OutputDefinition, containerOutput *ContainerExecutionOutput) (map[string]string, error) {
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

// extractJSONPath extracts a value from JSON using a simple JSONPath expression.
// Supports basic paths like "$.field", "$.field.nested", "$.array[0]"
func extractJSONPath(jsonStr, path string) (string, error) {
	// Parse JSON
	var data interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %w", err)
	}

	// Remove leading $. if present
	path = strings.TrimPrefix(path, "$.")
	path = strings.TrimPrefix(path, "$")

	// Split path by dots
	parts := strings.Split(path, ".")
	current := data

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Handle array indexing
		if strings.Contains(part, "[") {
			// Extract field name and index
			re := regexp.MustCompile(`^(\w+)\[(\d+)\]$`)
			matches := re.FindStringSubmatch(part)
			if len(matches) != 3 {
				return "", fmt.Errorf("invalid array syntax: %s", part)
			}

			fieldName := matches[1]
			index, _ := strconv.Atoi(matches[2])

			// Navigate to field
			if m, ok := current.(map[string]interface{}); ok {
				current = m[fieldName]
			} else {
				return "", fmt.Errorf("expected object at %s", fieldName)
			}

			// Access array index
			if arr, ok := current.([]interface{}); ok {
				if index < 0 || index >= len(arr) {
					return "", fmt.Errorf("array index out of bounds: %d", index)
				}
				current = arr[index]
			} else {
				return "", fmt.Errorf("expected array at %s", fieldName)
			}
		} else {
			// Simple field access
			if m, ok := current.(map[string]interface{}); ok {
				var exists bool
				current, exists = m[part]
				if !exists {
					return "", fmt.Errorf("field %s not found", part)
				}
			} else {
				return "", fmt.Errorf("cannot navigate to %s", part)
			}
		}
	}

	// Convert result to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "", nil
	default:
		// For complex types, return JSON representation
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("failed to marshal result: %w", err)
		}
		return string(b), nil
	}
}

// extractRegex extracts a value from text using a regular expression.
// If the regex has a capturing group, returns the first group.
// Otherwise, returns the entire match.
func extractRegex(text, pattern string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	matches := re.FindStringSubmatch(text)
	if len(matches) == 0 {
		return "", fmt.Errorf("no match found for pattern: %s", pattern)
	}

	// If there are capturing groups, return the first one
	if len(matches) > 1 {
		return matches[1], nil
	}

	// Otherwise return the full match
	return matches[0], nil
}

// readFile reads a file and returns its contents as a string.
func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SubstituteInputs applies input mappings to container environment variables.
// It resolves step outputs and substitutes them into the container input.
func SubstituteInputs(containerInput *ContainerExecutionInput, inputs []InputMapping, stepOutputs map[string]map[string]string) error {
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

// resolveInputMapping resolves an input mapping from step outputs.
// Format: "step-name.output-name"
func resolveInputMapping(mapping InputMapping, stepOutputs map[string]map[string]string) (string, error) {
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
