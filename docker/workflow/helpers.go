package workflow

import (
	"fmt"
	"strings"

	"github.com/jasoet/go-wf/docker"
)

// substituteTemplate replaces template variables in a string.
// Supports: {{item}}, {{index}}, and {{.paramName}} syntax.
func substituteTemplate(template string, item string, index int, params map[string]string) string {
	result := template

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

// substituteContainerInput creates a new container input with substituted values.
func substituteContainerInput(template docker.ContainerExecutionInput, item string, index int, params map[string]string) docker.ContainerExecutionInput {
	result := template

	// Substitute in image
	result.Image = substituteTemplate(template.Image, item, index, params)

	// Substitute in command
	if len(template.Command) > 0 {
		result.Command = make([]string, len(template.Command))
		for i, cmd := range template.Command {
			result.Command[i] = substituteTemplate(cmd, item, index, params)
		}
	}

	// Substitute in entrypoint
	if len(template.Entrypoint) > 0 {
		result.Entrypoint = make([]string, len(template.Entrypoint))
		for i, entry := range template.Entrypoint {
			result.Entrypoint[i] = substituteTemplate(entry, item, index, params)
		}
	}

	// Substitute in environment variables
	if len(template.Env) > 0 {
		result.Env = make(map[string]string, len(template.Env))
		for key, value := range template.Env {
			newKey := substituteTemplate(key, item, index, params)
			newValue := substituteTemplate(value, item, index, params)
			result.Env[newKey] = newValue
		}
	}

	// Substitute in name
	if template.Name != "" {
		result.Name = substituteTemplate(template.Name, item, index, params)
	}

	// Substitute in work directory
	if template.WorkDir != "" {
		result.WorkDir = substituteTemplate(template.WorkDir, item, index, params)
	}

	// Substitute in volumes
	if len(template.Volumes) > 0 {
		result.Volumes = make(map[string]string, len(template.Volumes))
		for key, value := range template.Volumes {
			newKey := substituteTemplate(key, item, index, params)
			newValue := substituteTemplate(value, item, index, params)
			result.Volumes[newKey] = newValue
		}
	}

	return result
}

// generateParameterCombinations generates all combinations of parameter values (cartesian product).
func generateParameterCombinations(params map[string][]string) []map[string]string {
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
