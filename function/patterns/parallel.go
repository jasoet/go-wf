package patterns

import (
	"fmt"

	"github.com/jasoet/go-wf/function/builder"
	"github.com/jasoet/go-wf/function/payload"
)

// FanOutFanIn creates a parallel workflow that executes one function per name concurrently.
//
// Example:
//
//	input, err := patterns.FanOutFanIn([]string{"task-1", "task-2", "task-3"})
func FanOutFanIn(functionNames []string) (*payload.ParallelInput, error) {
	if len(functionNames) == 0 {
		return nil, fmt.Errorf("at least one function name is required")
	}

	wb := builder.NewWorkflowBuilder("fan-out-fan-in").Parallel(true)

	for _, name := range functionNames {
		wb.AddInput(payload.FunctionExecutionInput{
			Name: name,
		})
	}

	return wb.BuildParallel()
}

// ParallelDataFetch creates a parallel workflow that fetches data from three
// hardcoded sources: fetch-users, fetch-orders, and fetch-inventory.
//
// Example:
//
//	input, err := patterns.ParallelDataFetch()
func ParallelDataFetch() (*payload.ParallelInput, error) {
	return FanOutFanIn([]string{"fetch-users", "fetch-orders", "fetch-inventory"})
}

// ParallelHealthCheck creates a parallel workflow that runs a health-check function
// for each service in the given environment. FailFast is enabled so the workflow
// stops on the first failure.
//
// Example:
//
//	input, err := patterns.ParallelHealthCheck(
//	    []string{"api", "database", "cache"}, "production")
func ParallelHealthCheck(services []string, env string) (*payload.ParallelInput, error) {
	if len(services) == 0 {
		return nil, fmt.Errorf("at least one service is required")
	}

	wb := builder.NewWorkflowBuilder("health-check").
		Parallel(true).
		FailFast(true)

	for _, service := range services {
		wb.AddInput(payload.FunctionExecutionInput{
			Name: "health-check",
			Args: map[string]string{
				"service":     service,
				"environment": env,
			},
		})
	}

	return wb.BuildParallel()
}
