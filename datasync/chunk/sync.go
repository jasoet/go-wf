package chunk

import (
	"cmp"
	"context"
	"fmt"
	"time"

	"github.com/jasoet/pkg/v2/temporal/job"
	"go.temporal.io/sdk/activity"
	sdkclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"github.com/jasoet/go-wf/v2/datasync"
	"github.com/jasoet/go-wf/v2/datasync/payload"
	datasyncwf "github.com/jasoet/go-wf/v2/datasync/workflow"
)

const (
	defaultStartToCloseTimeout = 20 * time.Minute
	defaultHeartbeatTimeout    = 15 * time.Minute
)

var defaultActivityRetryPolicy = temporal.RetryPolicy{
	MaximumAttempts:    3,
	InitialInterval:    60 * time.Second,
	BackoffCoefficient: 2.0,
	MaximumInterval:    5 * time.Minute,
}

// defaultCursorActivityOptions configures cursor-read and cursor-advance
// activity calls. These are lightweight metadata operations against the
// caller-supplied ProgressTracker (typically a local DB) — short timeouts
// and aggressive retries are appropriate.
var defaultCursorActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Second,
	RetryPolicy: &temporal.RetryPolicy{
		MaximumAttempts:    3,
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    8 * time.Second,
	},
}

// ChunkedSync builds a Temporal workflow + activities that walk a sequence of
// partitions and run fetch -> map -> write per partition.
type ChunkedSync[In, Out any, K cmp.Ordered] struct {
	name           string
	partitioner    Partitioner[K]
	fetcher        PartitionFetcher[In, K]
	mapper         datasync.Mapper[In, Out]
	sink           datasync.Sink[Out]
	tracker        ProgressTracker[K]
	partitionSleep time.Duration
	schedule       *job.ScheduleSpec
	rateLimitOpts  *RateLimitOpts
	activityRetry  *temporal.RetryPolicy
	startToClose   time.Duration
	heartbeat      time.Duration
	maxPerExec     int
	disabled       bool
}

// NewChunkedSync starts a builder for a job named name. The name appears in
// Temporal as the workflow type, the activity prefix, and the schedule id.
func NewChunkedSync[In, Out any, K cmp.Ordered](name string) *ChunkedSync[In, Out, K] {
	return &ChunkedSync[In, Out, K]{name: name}
}

func (c *ChunkedSync[In, Out, K]) Partitioner(p Partitioner[K]) *ChunkedSync[In, Out, K] {
	c.partitioner = p
	return c
}

func (c *ChunkedSync[In, Out, K]) Fetcher(f PartitionFetcher[In, K]) *ChunkedSync[In, Out, K] {
	c.fetcher = f
	return c
}

func (c *ChunkedSync[In, Out, K]) Mapper(m datasync.Mapper[In, Out]) *ChunkedSync[In, Out, K] {
	c.mapper = m
	return c
}

func (c *ChunkedSync[In, Out, K]) Sink(s datasync.Sink[Out]) *ChunkedSync[In, Out, K] {
	c.sink = s
	return c
}

func (c *ChunkedSync[In, Out, K]) WithTracker(t ProgressTracker[K]) *ChunkedSync[In, Out, K] {
	c.tracker = t
	return c
}

func (c *ChunkedSync[In, Out, K]) PartitionSleep(d time.Duration) *ChunkedSync[In, Out, K] {
	c.partitionSleep = d
	return c
}

// ScheduleEvery configures the workflow to fire at fixed intervals.
func (c *ChunkedSync[In, Out, K]) ScheduleEvery(d time.Duration) *ChunkedSync[In, Out, K] {
	c.schedule = &job.ScheduleSpec{Interval: d}
	return c
}

// ScheduleCron configures the workflow to fire on a cron expression.
func (c *ChunkedSync[In, Out, K]) ScheduleCron(expr string) *ChunkedSync[In, Out, K] {
	c.schedule = &job.ScheduleSpec{Cron: expr}
	return c
}

// ScheduleRaw configures the workflow with a fully customized schedule spec
// (e.g., calendar specs, overlap policy, jitter).
func (c *ChunkedSync[In, Out, K]) ScheduleRaw(spec *job.ScheduleSpec) *ChunkedSync[In, Out, K] {
	c.schedule = spec
	return c
}

func (c *ChunkedSync[In, Out, K]) RateLimitRetry(opts RateLimitOpts) *ChunkedSync[In, Out, K] {
	c.rateLimitOpts = &opts
	return c
}

func (c *ChunkedSync[In, Out, K]) ActivityRetry(p temporal.RetryPolicy) *ChunkedSync[In, Out, K] {
	c.activityRetry = &p
	return c
}

func (c *ChunkedSync[In, Out, K]) ActivityTimeouts(startToClose, hb time.Duration) *ChunkedSync[In, Out, K] {
	c.startToClose = startToClose
	c.heartbeat = hb
	return c
}

// MaxPartitionsPerExecution caps how many partitions one workflow execution
// processes before issuing ContinueAsNew with the same input. Required for
// large partition lists that would otherwise exceed Temporal's history
// budget. Must be combined with WithTracker — without a tracker, every
// execution re-processes the same prefix forever.
func (c *ChunkedSync[In, Out, K]) MaxPartitionsPerExecution(n int) *ChunkedSync[In, Out, K] {
	c.maxPerExec = n
	return c
}

func (c *ChunkedSync[In, Out, K]) Disabled(b bool) *ChunkedSync[In, Out, K] {
	c.disabled = b
	return c
}

// buildValidate panics if any required field is missing or an invalid combination is configured.
func (c *ChunkedSync[In, Out, K]) buildValidate() {
	if c.partitioner == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Partitioner is required", c.name))
	}
	if c.fetcher == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Fetcher is required", c.name))
	}
	if c.mapper == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Mapper is required", c.name))
	}
	if c.sink == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: Sink is required", c.name))
	}
	if c.maxPerExec > 0 && c.tracker == nil {
		panic(fmt.Sprintf("chunk.ChunkedSync(%q).Build: MaxPartitionsPerExecution requires WithTracker — without a tracker, the workflow re-processes the same partitions forever", c.name))
	}
}

// buildActivityOptions resolves activity options from builder fields, applying defaults.
func (c *ChunkedSync[In, Out, K]) buildActivityOptions() (partOpts, listOpts workflow.ActivityOptions) {
	startToClose := c.startToClose
	if startToClose == 0 {
		startToClose = defaultStartToCloseTimeout
	}
	hbTimeout := c.heartbeat
	if hbTimeout == 0 {
		hbTimeout = defaultHeartbeatTimeout
	}
	retry := c.activityRetry
	if retry == nil {
		p := defaultActivityRetryPolicy
		retry = &p
	}
	partOpts = workflow.ActivityOptions{
		StartToCloseTimeout: startToClose,
		HeartbeatTimeout:    hbTimeout,
		RetryPolicy:         retry,
	}
	listOpts = workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy:         retry,
	}
	return partOpts, listOpts
}

// cursorResult is the serialisable output of the ReadCursor activity.
type cursorResult[K cmp.Ordered] struct {
	Cursor K    `json:"cursor"`
	Exists bool `json:"exists"`
}

// Build constructs a *job.Definition. Panics if a required field is missing —
// caught at process startup, not in production hot paths.
func (c *ChunkedSync[In, Out, K]) Build() (*job.Definition, error) {
	c.buildValidate()

	jobName := c.name
	partitionsActName := jobName + ".Partitions"
	runPartitionActName := jobName + ".RunPartition"
	readCursorActName := jobName + ".ReadCursor"
	advanceCursorActName := jobName + ".AdvanceCursor"

	fetcher := c.fetcher
	if c.rateLimitOpts != nil {
		fetcher = WithRateLimitRetry[In, K](c.fetcher, *c.rateLimitOpts)
	}
	mapper, sink, tracker, partitioner := c.mapper, c.sink, c.tracker, c.partitioner

	partitionActivityOptions, partitionsListOptions := c.buildActivityOptions()

	// Activity closures (bind type parameters at registration time).
	partitionsActFn := func(ctx context.Context) ([]Partition[K], error) {
		return partitioner.Partitions(ctx)
	}
	runPartitionActFn := func(ctx context.Context, in runPartitionInput[K]) (PartitionResult[K], error) {
		return runPartition[In, Out, K](ctx, in, fetcher, mapper, sink)
	}

	var readCursorActFn func(ctx context.Context, jobName string) (cursorResult[K], error)
	var advanceCursorActFn func(ctx context.Context, completed K) error
	if tracker != nil {
		readCursorActFn = func(ctx context.Context, name string) (cursorResult[K], error) {
			cur, exists, err := tracker.Cursor(ctx, name)
			return cursorResult[K]{Cursor: cur, Exists: exists}, err
		}
		advanceCursorActFn = func(ctx context.Context, completed K) error {
			return tracker.Advance(ctx, jobName, completed)
		}
	}

	wfState := chunkedSyncWorkflow[In, Out, K]{
		jobName:                   jobName,
		partitionsActivityName:    partitionsActName,
		runPartitionActivityName:  runPartitionActName,
		readCursorActivityName:    readCursorActName,
		advanceCursorActivityName: advanceCursorActName,
		partitionActivityOptions:  partitionActivityOptions,
		partitionsListOptions:     partitionsListOptions,
		partitionSleep:            c.partitionSleep,
		hasTracker:                tracker != nil,
		maxPerExec:                c.maxPerExec,
	}

	schedule := c.schedule
	disabled := c.disabled

	opts := []job.Option{
		job.WithRegister(func(w worker.Worker) {
			w.RegisterWorkflowWithOptions(wfState.run, workflow.RegisterOptions{Name: jobName})
			w.RegisterActivityWithOptions(partitionsActFn, activity.RegisterOptions{Name: partitionsActName})
			w.RegisterActivityWithOptions(runPartitionActFn, activity.RegisterOptions{Name: runPartitionActName})
			if tracker != nil {
				w.RegisterActivityWithOptions(readCursorActFn, activity.RegisterOptions{Name: readCursorActName})
				w.RegisterActivityWithOptions(advanceCursorActFn, activity.RegisterOptions{Name: advanceCursorActName})
			}
		}),
		job.WithExecute(func(ctx context.Context, c sdkclient.Client, sdkOpts sdkclient.StartWorkflowOptions, in any) (sdkclient.WorkflowRun, error) {
			return c.ExecuteWorkflow(ctx, sdkOpts, jobName, in)
		}),
		job.WithNewInput(func() any { return &payload.SyncExecutionInput{} }),
	}
	if schedule != nil {
		if disabled {
			schedule.Paused = true
		}
		opts = append(opts, job.WithSchedule(schedule))
	}

	return job.New(jobName, datasyncwf.TaskQueue(jobName), opts...)
}

// chunkedSyncWorkflow holds the workflow-side configuration captured at
// Build time. The run method is the registered workflow function.
type chunkedSyncWorkflow[In, Out any, K cmp.Ordered] struct {
	jobName                   string
	partitionsActivityName    string
	runPartitionActivityName  string
	readCursorActivityName    string
	advanceCursorActivityName string
	partitionActivityOptions  workflow.ActivityOptions
	partitionsListOptions     workflow.ActivityOptions
	partitionSleep            time.Duration
	hasTracker                bool
	maxPerExec                int
}

// run is the Temporal workflow function.
func (s chunkedSyncWorkflow[In, Out, K]) run(ctx workflow.Context, input payload.SyncExecutionInput) (SyncResult[K], error) {
	summary := SyncResult[K]{JobName: s.jobName}

	listCtx := workflow.WithActivityOptions(ctx, s.partitionsListOptions)
	var parts []Partition[K]
	if err := workflow.ExecuteActivity(listCtx, s.partitionsActivityName).Get(listCtx, &parts); err != nil {
		return summary, fmt.Errorf("partitions: %w", err)
	}
	if len(parts) == 0 {
		return summary, nil
	}

	// Cursor filtering when tracker is configured.
	if s.hasTracker {
		cursorCtx := workflow.WithActivityOptions(ctx, defaultCursorActivityOptions)
		var cur cursorResult[K]
		if err := workflow.ExecuteActivity(cursorCtx, s.readCursorActivityName, s.jobName).Get(cursorCtx, &cur); err != nil {
			return summary, fmt.Errorf("read cursor: %w", err)
		}
		if cur.Exists {
			filtered := make([]Partition[K], 0, len(parts))
			for _, p := range parts {
				if p.Start >= cur.Cursor {
					filtered = append(filtered, p)
				}
			}
			parts = filtered
		}
		if len(parts) == 0 {
			return summary, nil
		}
	}

	// Truncate to MaxPartitionsPerExecution; defer the rest to a fresh execution.
	deferred := false
	if s.maxPerExec > 0 && len(parts) > s.maxPerExec {
		parts = parts[:s.maxPerExec]
		deferred = true
	}

	var cursorAdvCtx workflow.Context
	if s.hasTracker {
		cursorAdvCtx = workflow.WithActivityOptions(ctx, defaultCursorActivityOptions)
	}
	partCtx := workflow.WithActivityOptions(ctx, s.partitionActivityOptions)
	for i, p := range parts {
		var pr PartitionResult[K]
		if err := workflow.ExecuteActivity(partCtx, s.runPartitionActivityName, runPartitionInput[K]{
			Partition: p,
			JobName:   s.jobName,
		}).Get(partCtx, &pr); err != nil {
			return summary, fmt.Errorf("partition %v..%v: %w", p.Start, p.End, err)
		}
		summary.Partitions = append(summary.Partitions, pr)
		summary.TotalPartitions++
		summary.TotalFetched += pr.Fetched
		summary.TotalInserted += pr.Inserted
		summary.TotalUpdated += pr.Updated
		summary.TotalSkipped += pr.Skipped

		if s.hasTracker {
			if err := workflow.ExecuteActivity(cursorAdvCtx, s.advanceCursorActivityName, p.End).Get(cursorAdvCtx, nil); err != nil {
				return summary, fmt.Errorf("advance cursor: %w", err)
			}
		}

		if i < len(parts)-1 && s.partitionSleep > 0 {
			if err := workflow.Sleep(ctx, s.partitionSleep); err != nil {
				return summary, fmt.Errorf("partition-sleep: %w", err)
			}
		}
	}

	if deferred {
		return summary, workflow.NewContinueAsNewError(ctx, s.jobName, input)
	}
	return summary, nil
}
