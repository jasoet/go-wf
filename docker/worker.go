package docker

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"

	containerActivity "github.com/jasoet/go-wf/docker/activity"
	wf "github.com/jasoet/go-wf/docker/workflow"
)

// RegisterWorkflows registers all docker workflows with a worker.
func RegisterWorkflows(w worker.Worker) {
	w.RegisterWorkflow(wf.ExecuteContainerWorkflow)
	w.RegisterWorkflow(wf.ContainerPipelineWorkflow)
	w.RegisterWorkflow(wf.ParallelContainersWorkflow)
	w.RegisterWorkflow(wf.LoopWorkflow)
	w.RegisterWorkflow(wf.ParameterizedLoopWorkflow)
	w.RegisterWorkflow(wf.DAGWorkflow)
	w.RegisterWorkflow(wf.WorkflowWithParameters)
}

// RegisterActivities registers all docker activities with a worker.
func RegisterActivities(w worker.Worker) {
	w.RegisterActivityWithOptions(containerActivity.StartContainerActivity, activity.RegisterOptions{
		Name: "StartContainerActivity",
	})
}

// RegisterAll registers both workflows and activities.
func RegisterAll(w worker.Worker) {
	RegisterWorkflows(w)
	RegisterActivities(w)
}
