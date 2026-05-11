package container

import (
	"github.com/jasoet/pkg/v2/temporal/job"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	containerActivity "github.com/jasoet/go-wf/container/activity"
	wf "github.com/jasoet/go-wf/container/workflow"
)

// RegisterWorkflows registers all container workflows with a worker.
// Calling this function multiple times on the same worker is a no-op for
// subsequent calls — each workflow type is registered at most once per worker.
func RegisterWorkflows(w worker.Worker) {
	job.RegisterWorkflowOnce(w, "ExecuteContainerWorkflow", wf.ExecuteContainerWorkflow, workflow.RegisterOptions{Name: "ExecuteContainerWorkflow"})
	job.RegisterWorkflowOnce(w, "ContainerPipelineWorkflow", wf.ContainerPipelineWorkflow, workflow.RegisterOptions{Name: "ContainerPipelineWorkflow"})
	job.RegisterWorkflowOnce(w, "ParallelContainersWorkflow", wf.ParallelContainersWorkflow, workflow.RegisterOptions{Name: "ParallelContainersWorkflow"})
	job.RegisterWorkflowOnce(w, "LoopWorkflow", wf.LoopWorkflow, workflow.RegisterOptions{Name: "LoopWorkflow"})
	job.RegisterWorkflowOnce(w, "ParameterizedLoopWorkflow", wf.ParameterizedLoopWorkflow, workflow.RegisterOptions{Name: "ParameterizedLoopWorkflow"})
	job.RegisterWorkflowOnce(w, "DAGWorkflow", wf.DAGWorkflow, workflow.RegisterOptions{Name: "DAGWorkflow"})
	job.RegisterWorkflowOnce(w, "WorkflowWithParameters", wf.WorkflowWithParameters, workflow.RegisterOptions{Name: "WorkflowWithParameters"})
}

// RegisterActivities registers all container activities with a worker.
// The activity is wrapped with OTel instrumentation that activates
// when otel.Config is present in the activity context.
// Calling this function multiple times on the same worker is a no-op for
// subsequent calls — each activity type is registered at most once per worker.
func RegisterActivities(w worker.Worker) {
	instrumented := containerActivity.InstrumentedStartContainerActivity(containerActivity.StartContainerActivity)
	job.RegisterActivityOnce(w, "StartContainerActivity", instrumented, activity.RegisterOptions{
		Name: "StartContainerActivity",
	})
}

// RegisterAll registers both workflows and activities.
func RegisterAll(w worker.Worker) {
	RegisterWorkflows(w)
	RegisterActivities(w)
}
