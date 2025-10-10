package docker

import (
	"go.temporal.io/sdk/worker"
)

// RegisterWorkflows registers all docker workflows with a worker.
func RegisterWorkflows(w worker.Worker) {
	w.RegisterWorkflow(ExecuteContainerWorkflow)
	w.RegisterWorkflow(ContainerPipelineWorkflow)
	w.RegisterWorkflow(ParallelContainersWorkflow)
	w.RegisterWorkflow(LoopWorkflow)
	w.RegisterWorkflow(ParameterizedLoopWorkflow)
	w.RegisterWorkflow(DAGWorkflow)
	w.RegisterWorkflow(WorkflowWithParameters)
}

// RegisterActivities registers all docker activities with a worker.
func RegisterActivities(w worker.Worker) {
	w.RegisterActivity(StartContainerActivity)
}

// RegisterAll registers both workflows and activities.
func RegisterAll(w worker.Worker) {
	RegisterWorkflows(w)
	RegisterActivities(w)
}
