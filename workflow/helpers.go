package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	wf "go.temporal.io/sdk/workflow"
)

const (
	// FailureStrategyFailFast stops execution on the first failure.
	FailureStrategyFailFast = "fail_fast"
	// FailureStrategyContinue continues execution despite failures.
	FailureStrategyContinue = "continue"
)

// DefaultActivityOptions returns the standard activity options used by workflow helpers.
func DefaultActivityOptions() wf.ActivityOptions {
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

// SubstituteTemplate replaces template variables in a string.
// Supports: {{item}}, {{index}}, and {{.paramName}}/{{paramName}} syntax.
func SubstituteTemplate(tmpl, item string, index int, params map[string]string) string {
	result := tmpl

	// Replace {{item}}
	result = strings.ReplaceAll(result, "{{item}}", item)

	// Replace {{index}}
	result = strings.ReplaceAll(result, "{{index}}", fmt.Sprintf("%d", index))

	// Replace {{.paramName}} with parameter values
	for key, value := range params {
		result = strings.ReplaceAll(result, fmt.Sprintf("{{.%s}}", key), value)
		result = strings.ReplaceAll(result, fmt.Sprintf("{{%s}}", key), value)
	}

	return result
}

// GenerateParameterCombinations generates all combinations of parameter values (cartesian product).
func GenerateParameterCombinations(params map[string][]string) []map[string]string {
	if len(params) == 0 {
		return nil
	}

	// Convert map to ordered slices for consistent iteration
	keys := make([]string, 0, len(params))
	values := make([][]string, 0, len(params))

	for key, vals := range params {
		keys = append(keys, key)
		values = append(values, vals)
	}

	// Generate cartesian product
	var result []map[string]string
	var generate func(int, map[string]string)

	generate = func(depth int, current map[string]string) {
		if depth == len(keys) {
			// Make a copy of current combination
			combo := make(map[string]string, len(current))
			for k, v := range current {
				combo[k] = v
			}
			result = append(result, combo)
			return
		}

		key := keys[depth]
		for _, value := range values[depth] {
			current[key] = value
			generate(depth+1, current)
		}
	}

	generate(0, make(map[string]string))
	return result
}

// ExtractJSONPath extracts a value from JSON using a simple JSONPath expression,
// supporting basic paths like "$.field", "$.field.nested", "$.array[0]".
func ExtractJSONPath(jsonStr, path string) (string, error) {
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
			index, err := strconv.Atoi(matches[2])
			if err != nil {
				return "", fmt.Errorf("invalid array index %s: %w", matches[2], err)
			}

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

// ExtractRegex extracts a value from text using a regular expression.
// If the regex has a capturing group, returns the first group.
// Otherwise, returns the entire match.
func ExtractRegex(text, pattern string) (string, error) {
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

// ShellEscape wraps a string in single quotes for safe shell interpolation.
// Single quotes inside the string are escaped with the '\” idiom.
func ShellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// ReadFile reads a file and returns its contents as a string.
func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path) //#nosec G304 -- path comes from workflow configuration
	if err != nil {
		return "", err
	}
	return string(data), nil
}
