package activity

import (
	"context"
	"fmt"
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

type mockSource[T any] struct {
	name    string
	records []T
	err     error
	sleep   time.Duration
}

func (m *mockSource[T]) Name() string { return m.name }
func (m *mockSource[T]) Fetch(ctx context.Context) ([]T, error) {
	if m.sleep > 0 {
		select {
		case <-time.After(m.sleep):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return m.records, m.err
}

type mockSink[U any] struct {
	name   string
	result datasync.WriteResult
	err    error
	sleep  time.Duration
}

func (m *mockSink[U]) Name() string { return m.name }
func (m *mockSink[U]) Write(ctx context.Context, _ []U) (datasync.WriteResult, error) {
	if m.sleep > 0 {
		select {
		case <-time.After(m.sleep):
		case <-ctx.Done():
			return datasync.WriteResult{}, ctx.Err()
		}
	}
	return m.result, m.err
}

func TestActivities_SyncData_Success(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", records: []string{"a", "b", "c"}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst", result: datasync.WriteResult{Inserted: 3}}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	result, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	var output ActivityOutput
	require.NoError(t, result.Get(&output))
	assert.Equal(t, 3, output.TotalFetched)
	assert.Equal(t, 3, output.Inserted)
}

func TestActivities_SyncData_EmptySource(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", records: []string{}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst"}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	result, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	var output ActivityOutput
	require.NoError(t, result.Get(&output))
	assert.Equal(t, 0, output.TotalFetched)
}

func TestActivities_SyncData_FetchError(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", err: fmt.Errorf("api down")}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst"}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source src fetch failed")
}

func TestActivities_SyncData_WriteError(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	source := &mockSource[string]{name: "src", records: []string{"a"}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst", err: fmt.Errorf("db down")}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sink dst write failed")
}

func TestToSyncExecutionOutput_Success(t *testing.T) {
	ao := &ActivityOutput{TotalFetched: 10, Inserted: 5, Updated: 3, Skipped: 2}
	output := ToSyncExecutionOutput("test-job", ao, 100, nil)
	assert.True(t, output.Success)
	assert.Equal(t, "test-job", output.JobName)
	assert.Equal(t, 10, output.TotalFetched)
	assert.Equal(t, 5, output.Inserted)
}

func TestToSyncExecutionOutput_Error(t *testing.T) {
	output := ToSyncExecutionOutput("test-job", nil, 0, fmt.Errorf("something failed"))
	assert.False(t, output.Success)
	assert.Equal(t, "something failed", output.Error)
}

func TestActivities_SyncData_HeartbeatsDuringSlowFetch(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	var (
		mu       sync.Mutex
		captured []string
	)
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, details converter.EncodedValues) {
		var payload string
		// assert (not require) — listener runs from the heartbeat goroutine; require's runtime.Goexit would terminate the wrong goroutine.
		assert.NoError(t, details.Get(&payload))
		mu.Lock()
		captured = append(captured, payload)
		mu.Unlock()
	})
	// Headroom for the 11s Fetch sleep plus goroutine scheduling jitter.
	testEnv.SetTestTimeout(60 * time.Second)
	// HeartbeatTimeout is unset in TestActivityEnvironment, so heartbeatInterval falls back to 10s; sleep > 10s guarantees at least one tick fires before Fetch returns.
	source := &mockSource[string]{
		name:    "src",
		records: []string{"a"},
		sleep:   11 * time.Second,
	}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{name: "dst", result: datasync.WriteResult{Inserted: 1}}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, captured, "expected at least one periodic heartbeat during slow Fetch")

	found := false
	for _, p := range captured {
		if strings.Contains(p, "fetching") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one heartbeat payload to contain 'fetching', got: %v", captured)
}

// heartbeatCaptureOutbound is an activity outbound interceptor that records
// every RecordHeartbeat call before the SDK's throttle batching can suppress it.
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

// heartbeatCaptureInbound wires the outbound interceptor into the activity chain.
type heartbeatCaptureInbound struct {
	sdkinterceptor.ActivityInboundInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (h *heartbeatCaptureInbound) Init(next sdkinterceptor.ActivityOutboundInterceptor) error {
	h.outbound.Next = next
	return h.Next.Init(h.outbound)
}

// heartbeatCaptureWorkerInterceptor is a WorkerInterceptor that injects heartbeat capture.
type heartbeatCaptureWorkerInterceptor struct {
	sdkinterceptor.InterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (w *heartbeatCaptureWorkerInterceptor) InterceptActivity(
	ctx context.Context,
	next sdkinterceptor.ActivityInboundInterceptor,
) sdkinterceptor.ActivityInboundInterceptor {
	return &heartbeatCaptureInbound{
		ActivityInboundInterceptorBase: sdkinterceptor.ActivityInboundInterceptorBase{Next: next},
		outbound:                       w.outbound,
	}
}

func TestActivities_SyncData_HeartbeatsDuringSlowWrite(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	// Use an activity outbound interceptor to capture every RecordHeartbeat call
	// before the SDK's throttle can batch/drop them. The default throttle window is
	// 30s; SyncData sends an explicit "fetched N records" heartbeat at t≈0 which
	// starts that window, so the periodic goroutine tick at t=10s never reaches the
	// SetOnActivityHeartbeatListener (it's batched and dropped on success). Capturing
	// at the interceptor layer bypasses throttling entirely.
	capturer := &heartbeatCaptureOutbound{}
	workerInterceptor := &heartbeatCaptureWorkerInterceptor{outbound: capturer}
	testEnv.SetWorkerOptions(worker.Options{
		Interceptors: []sdkinterceptor.WorkerInterceptor{workerInterceptor},
	})

	// Headroom for the 11s Write sleep plus goroutine scheduling jitter.
	testEnv.SetTestTimeout(60 * time.Second)
	// HeartbeatTimeout is unset in TestActivityEnvironment, so heartbeatInterval falls back to 10s; sleep > 10s guarantees at least one tick fires before Write returns.
	source := &mockSource[string]{name: "src", records: []string{"a"}}
	mapper := datasync.IdentityMapper[string]()
	sink := &mockSink[string]{
		name:   "dst",
		result: datasync.WriteResult{Inserted: 1},
		sleep:  11 * time.Second,
	}

	activities := NewActivities(source, mapper, sink)
	testEnv.RegisterActivity(activities.SyncData)

	input := ActivityInput{JobName: "test", SourceName: "src", SinkName: "dst"}
	_, err := testEnv.ExecuteActivity(activities.SyncData, input)
	require.NoError(t, err)

	capturer.mu.Lock()
	defer capturer.mu.Unlock()
	require.NotEmpty(t, capturer.captured, "expected at least one periodic heartbeat during slow Write")

	found := false
	for _, p := range capturer.captured {
		if strings.Contains(p, "writing") {
			found = true
			break
		}
	}
	assert.True(t, found, "expected at least one heartbeat payload to contain 'writing', got: %v", capturer.captured)
}
