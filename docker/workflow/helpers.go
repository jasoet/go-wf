package workflow

import (
	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// substituteContainerInput creates a new container input with substituted values.
func substituteContainerInput(template payload.ContainerExecutionInput, item string, index int, params map[string]string) payload.ContainerExecutionInput {
	result := template

	// Substitute in image
	result.Image = generic.SubstituteTemplate(template.Image, item, index, params)

	// Substitute in command
	if len(template.Command) > 0 {
		result.Command = make([]string, len(template.Command))
		for i, cmd := range template.Command {
			result.Command[i] = generic.SubstituteTemplate(cmd, item, index, params)
		}
	}

	// Substitute in entrypoint
	if len(template.Entrypoint) > 0 {
		result.Entrypoint = make([]string, len(template.Entrypoint))
		for i, entry := range template.Entrypoint {
			result.Entrypoint[i] = generic.SubstituteTemplate(entry, item, index, params)
		}
	}

	// Substitute in environment variables
	if len(template.Env) > 0 {
		result.Env = make(map[string]string, len(template.Env))
		for key, value := range template.Env {
			newKey := generic.SubstituteTemplate(key, item, index, params)
			newValue := generic.SubstituteTemplate(value, item, index, params)
			result.Env[newKey] = newValue
		}
	}

	// Substitute in name
	if template.Name != "" {
		result.Name = generic.SubstituteTemplate(template.Name, item, index, params)
	}

	// Substitute in work directory
	if template.WorkDir != "" {
		result.WorkDir = generic.SubstituteTemplate(template.WorkDir, item, index, params)
	}

	// Substitute in volumes
	if len(template.Volumes) > 0 {
		result.Volumes = make(map[string]string, len(template.Volumes))
		for key, value := range template.Volumes {
			newKey := generic.SubstituteTemplate(key, item, index, params)
			newValue := generic.SubstituteTemplate(value, item, index, params)
			result.Volumes[newKey] = newValue
		}
	}

	return result
}

// generateParameterCombinations delegates to the generic implementation.
func generateParameterCombinations(params map[string][]string) []map[string]string {
	return generic.GenerateParameterCombinations(params)
}

// substituteTemplate delegates to the generic implementation.
func substituteTemplate(tmpl, item string, index int, params map[string]string) string {
	return generic.SubstituteTemplate(tmpl, item, index, params)
}
