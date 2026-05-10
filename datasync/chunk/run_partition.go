package chunk

import (
	"cmp"
	"context"
	"fmt"
	"sync/atomic"

	"go.temporal.io/sdk/activity"

	"github.com/jasoet/go-wf/datasync"
	"github.com/jasoet/go-wf/datasync/internal/heartbeat"
)

// runPartitionInput is the activity input for a single partition.
type runPartitionInput[K cmp.Ordered] struct {
	Partition Partition[K] `json:"partition"`
	JobName   string       `json:"jobName"`
}

// runPartition is the per-partition activity body. It records a starting
// heartbeat, spawns a heartbeat goroutine that ticks every Interval(...) with
// the current phase, then runs fetch -> map -> write.
//
// Activity registration: callers wrap this in a closure that binds the
// concrete In, Out, K parameters and registers it under "<jobName>.RunPartition".
func runPartition[In, Out any, K cmp.Ordered](
	ctx context.Context,
	in runPartitionInput[K],
	fetcher PartitionFetcher[In, K],
	mapper datasync.Mapper[In, Out],
	sink datasync.Sink[Out],
) (PartitionResult[K], error) {
	result := PartitionResult[K]{Start: in.Partition.Start, End: in.Partition.End}

	var phase atomic.Pointer[string]
	setPhase := func(p string) { phase.Store(&p) }
	setPhase("starting")

	prefix := fmt.Sprintf("partition %v to %v", in.Partition.Start, in.Partition.End)
	if in.JobName != "" {
		prefix = in.JobName + " " + prefix
	}
	activity.RecordHeartbeat(ctx, prefix+": starting")

	interval := heartbeat.Interval(activity.GetInfo(ctx).HeartbeatTimeout)
	done := make(chan struct{})
	defer close(done)
	go heartbeat.Loop(ctx, interval, heartbeat.PhaseMessage(prefix, &phase), done)

	setPhase("fetching")
	records, err := fetcher(ctx, in.Partition.Start, in.Partition.End)
	if err != nil {
		return result, fmt.Errorf("fetch %v..%v: %w", in.Partition.Start, in.Partition.End, err)
	}
	result.Fetched = len(records)
	if result.Fetched == 0 {
		return result, nil
	}

	setPhase("mapping")
	mapped, err := mapper.Map(ctx, records)
	if err != nil {
		return result, fmt.Errorf("map %v..%v: %w", in.Partition.Start, in.Partition.End, err)
	}

	setPhase("writing")
	wr, err := sink.Write(ctx, mapped)
	if err != nil {
		return result, fmt.Errorf("write %v..%v: %w", in.Partition.Start, in.Partition.End, err)
	}
	result.Inserted = wr.Inserted
	result.Updated = wr.Updated
	result.Skipped = wr.Skipped
	return result, nil
}
