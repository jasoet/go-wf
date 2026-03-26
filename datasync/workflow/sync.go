package workflow

import (
	"time"

	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/activity"
	"github.com/jasoet/go-wf/datasync/payload"
)

const (
	defaultActivityTimeout      = 5 * time.Minute
	defaultHeartbeatTimeout     = 30 * time.Second
	defaultMaxRetries           = int32(3)
	defaultRetryInitialInterval = 30 * time.Second
	defaultRetryBackoffCoeff    = 2.0
	defaultRetryMaxInterval     = 5 * time.Minute
)

// TaskQueue returns the Temporal task queue name for a sync job.
func TaskQueue(jobName string) string {
	return "sync-" + jobName
}

// RegisterSyncDataOptions returns activity registration options for SyncData.
func RegisterSyncDataOptions(jobName string) workflow.RegisterOptions {
	return workflow.RegisterOptions{Name: jobName + ".SyncData"}
}

// RegisterWorkflowOptions returns workflow registration options for a job.
func RegisterWorkflowOptions(jobName string) workflow.RegisterOptions {
	return workflow.RegisterOptions{Name: jobName}
}

// BuildWorkflowInput creates a SyncExecutionInput from a Job.
func BuildWorkflowInput[T any, U any](job datasync.Job[T, U]) payload.SyncExecutionInput {
	return payload.SyncExecutionInput{
		JobName:    job.Name,
		SourceName: job.Source.Name(),
		SinkName:   job.Sink.Name(),
		Metadata:   job.Metadata,
	}
}

// BuildActivityInput creates an ActivityInput from a Job.
func BuildActivityInput[T any, U any](job datasync.Job[T, U]) activity.ActivityInput {
	input := activity.ActivityInput{
		JobName:    job.Name,
		SourceName: job.Source.Name(),
		SinkName:   job.Sink.Name(),
	}
	// If source implements ParamSource, capture params.
	type paramProvider interface {
		Params() any
	}
	if ps, ok := any(job.Source).(paramProvider); ok {
		input.Params = ps.Params()
	}
	return input
}

// FullJobRegistration holds all information needed to register and schedule a sync job
// without knowing its generic types.
type FullJobRegistration struct {
	Name          string
	TaskQueue     string
	Schedule      time.Duration
	Disabled      bool
	WorkflowInput payload.SyncExecutionInput
	Register      func(w worker.Worker)
}

// BuildJobRegistration creates a FullJobRegistration from a generic Job and a disabled flag.
func BuildJobRegistration[T any, U any](job datasync.Job[T, U], disabled bool) FullJobRegistration {
	return FullJobRegistration{
		Name:          job.Name,
		TaskQueue:     TaskQueue(job.Name),
		Schedule:      job.Schedule,
		Disabled:      disabled,
		WorkflowInput: BuildWorkflowInput(job),
		Register: func(w worker.Worker) {
			RegisterJob(w, job)
		},
	}
}

// RegisterJob registers a sync job's workflow and activities with a Temporal worker.
func RegisterJob[T any, U any](w worker.Worker, job datasync.Job[T, U]) {
	activities := activity.NewActivities(job.Source, job.Mapper, job.Sink)

	activityInput := BuildActivityInput(job)
	wf := newSyncWorkflow(job, activityInput)

	w.RegisterWorkflowWithOptions(wf, RegisterWorkflowOptions(job.Name))
	w.RegisterActivityWithOptions(activities.SyncData, sdkactivity.RegisterOptions{Name: job.Name + ".SyncData"})
}

// newSyncWorkflow returns a workflow function that executes a single SyncData activity.
func newSyncWorkflow[T any, U any](job datasync.Job[T, U], activityInput activity.ActivityInput) func(workflow.Context, payload.SyncExecutionInput) (*payload.SyncExecutionOutput, error) {
	activityTimeout := withDefault(job.ActivityTimeout, defaultActivityTimeout)
	heartbeatTimeout := withDefault(job.HeartbeatTimeout, defaultHeartbeatTimeout)
	maxRetries := job.MaxRetries
	if maxRetries == 0 {
		maxRetries = defaultMaxRetries
	}
	retryInitialInterval := withDefault(job.RetryInitialInterval, defaultRetryInitialInterval)
	retryBackoffCoeff := job.RetryBackoffCoefficient
	if retryBackoffCoeff == 0 {
		retryBackoffCoeff = defaultRetryBackoffCoeff
	}
	retryMaxInterval := withDefault(job.RetryMaxInterval, defaultRetryMaxInterval)

	return func(ctx workflow.Context, _ payload.SyncExecutionInput) (*payload.SyncExecutionOutput, error) {
		ao := workflow.ActivityOptions{
			TaskQueue:           TaskQueue(job.Name),
			StartToCloseTimeout: activityTimeout,
			HeartbeatTimeout:    heartbeatTimeout,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts:    maxRetries,
				InitialInterval:    retryInitialInterval,
				BackoffCoefficient: retryBackoffCoeff,
				MaximumInterval:    retryMaxInterval,
			},
		}
		ctx = workflow.WithActivityOptions(ctx, ao)

		startTime := workflow.Now(ctx)

		var actOutput activity.ActivityOutput
		err := workflow.ExecuteActivity(ctx, job.Name+".SyncData", activityInput).Get(ctx, &actOutput)

		processingTime := workflow.Now(ctx).Sub(startTime)
		output := activity.ToSyncExecutionOutput(job.Name, &actOutput, processingTime, err)

		if err != nil {
			return &output, err
		}
		return &output, nil
	}
}

func withDefault(val, def time.Duration) time.Duration {
	if val == 0 {
		return def
	}
	return val
}
