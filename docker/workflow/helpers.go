package workflow

import (
	"github.com/jasoet/go-wf/docker/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

// toTaskPtrs converts a slice of ContainerExecutionInput values to a slice of pointers.
func toTaskPtrs(containers []payload.ContainerExecutionInput) []*payload.ContainerExecutionInput {
	ptrs := make([]*payload.ContainerExecutionInput, len(containers))
	for i := range containers {
		ptrs[i] = &containers[i]
	}
	return ptrs
}

// toPipelineOutput converts a generic pipeline output to a docker-specific output.
func toPipelineOutput(g *generic.PipelineOutput[payload.ContainerExecutionOutput], err error) (*payload.PipelineOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.PipelineOutput{
		Results: g.Results, TotalSuccess: g.TotalSuccess, TotalFailed: g.TotalFailed, TotalDuration: g.TotalDuration,
	}, err
}

// toParallelOutput converts a generic parallel output to a docker-specific output.
func toParallelOutput(g *generic.ParallelOutput[payload.ContainerExecutionOutput], err error) (*payload.ParallelOutput, error) {
	if g == nil {
		return nil, err
	}
	return &payload.ParallelOutput{
		Results: g.Results, TotalSuccess: g.TotalSuccess, TotalFailed: g.TotalFailed, TotalDuration: g.TotalDuration,
	}, err
}

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
