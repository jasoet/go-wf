package chunk

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"github.com/jasoet/go-wf/datasync"
)

// heartbeatCaptureOutbound mirrors the pattern in datasync/activity/sync_test.go.
type heartbeatCaptureOutbound struct {
	sdkinterceptor.ActivityOutboundInterceptorBase
	mu       sync.Mutex
	captured []string
}

func (h *heartbeatCaptureOutbound) RecordHeartbeat(ctx context.Context, details ...interface{}) {
	if len(details) > 0 {
		if s, ok := details[0].(string); ok {
			h.mu.Lock()
			h.captured = append(h.captured, s)
			h.mu.Unlock()
		}
	}
	h.Next.RecordHeartbeat(ctx, details...)
}

func (h *heartbeatCaptureOutbound) snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.captured))
	copy(out, h.captured)
	return out
}

type capturingInbound struct {
	sdkinterceptor.ActivityInboundInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInbound) Init(o sdkinterceptor.ActivityOutboundInterceptor) error {
	c.outbound.Next = o
	return c.ActivityInboundInterceptorBase.Init(c.outbound)
}

type capturingInterceptor struct {
	sdkinterceptor.WorkerInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInterceptor) InterceptActivity(_ context.Context, next sdkinterceptor.ActivityInboundInterceptor) sdkinterceptor.ActivityInboundInterceptor {
	in := &capturingInbound{outbound: c.outbound}
	in.Next = next
	return in
}

type stubMapper struct{}

func (stubMapper) Map(_ context.Context, recs []string) ([]string, error) {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = strings.ToUpper(r)
	}
	return out, nil
}

type stubSink struct {
	name  string
	sleep time.Duration
	err   error
}

func (s *stubSink) Name() string { return s.name }
func (s *stubSink) Write(ctx context.Context, recs []string) (datasync.WriteResult, error) {
	if s.sleep > 0 {
		select {
		case <-ctx.Done():
			return datasync.WriteResult{}, ctx.Err()
		case <-time.After(s.sleep):
		}
	}
	if s.err != nil {
		return datasync.WriteResult{}, s.err
	}
	return datasync.WriteResult{Inserted: len(recs)}, nil
}

func TestRunPartition_HappyPath(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return []string{"a", "b"}, nil
	}
	mapper := stubMapper{}
	sink := &stubSink{name: "sink"}

	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, mapper, sink)
	}
	testEnv.RegisterActivity(act)

	val, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.NoError(t, err)
	var got PartitionResult[int64]
	require.NoError(t, val.Get(&got))
	assert.Equal(t, int64(0), got.Start)
	assert.Equal(t, int64(100), got.End)
	assert.Equal(t, 2, got.Fetched)
	assert.Equal(t, 2, got.Inserted)
}

func TestRunPartition_EmptyFetchSkipsMapAndWrite(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return nil, nil
	}
	mapperCalled := false
	mapperFn := datasync.MapperFunc[string, string](func(_ context.Context, recs []string) ([]string, error) {
		mapperCalled = true
		return recs, nil
	})
	sink := &stubSink{name: "sink"}

	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, mapperFn, sink)
	}
	testEnv.RegisterActivity(act)

	val, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.NoError(t, err)
	var got PartitionResult[int64]
	require.NoError(t, val.Get(&got))
	assert.Equal(t, 0, got.Fetched)
	assert.False(t, mapperCalled, "mapper should be skipped when fetcher returns no records")
}

func TestRunPartition_FetcherError_Wrapped(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	boom := errors.New("fetch boom")
	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return nil, boom
	}
	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, stubMapper{}, &stubSink{name: "s"})
	}
	testEnv.RegisterActivity(act)

	_, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.Error(t, err)
}

func TestRunPartition_HeartbeatsDuringSlowWrite(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	cap := &heartbeatCaptureOutbound{}
	testEnv.SetWorkerOptions(worker.Options{
		Interceptors: []sdkinterceptor.WorkerInterceptor{&capturingInterceptor{outbound: cap}},
	})

	fetcher := func(_ context.Context, _, _ int64) ([]string, error) {
		return []string{"a"}, nil
	}
	// HeartbeatTimeout is unset in TestActivityEnvironment, so heartbeat.Interval falls back to 10s; sleep > 10s guarantees at least one tick fires before Write returns.
	sink := &stubSink{name: "sink", sleep: 11 * time.Second}

	act := func(ctx context.Context, in runPartitionInput[int64]) (PartitionResult[int64], error) {
		return runPartition[string, string, int64](ctx, in, fetcher, stubMapper{}, sink)
	}
	testEnv.RegisterActivity(act)
	// Headroom for the 11s Write sleep plus goroutine scheduling jitter.
	testEnv.SetTestTimeout(60 * time.Second)

	_, err := testEnv.ExecuteActivity(act, runPartitionInput[int64]{
		Partition: Partition[int64]{Start: 0, End: 100},
		JobName:   "job-x",
	})
	require.NoError(t, err)

	got := cap.snapshot()
	require.NotEmpty(t, got)
	foundWriting := false
	for _, m := range got {
		if strings.Contains(m, ": writing") {
			foundWriting = true
			break
		}
	}
	assert.True(t, foundWriting, "expected 'writing' phase in heartbeats; got %v", got)
}
