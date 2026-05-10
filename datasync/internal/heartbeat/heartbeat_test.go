package heartbeat

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	sdkinterceptor "go.temporal.io/sdk/interceptor"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

func TestInterval(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{"zero falls back to 10s default", 0, 10 * time.Second},
		{"100ms hits 1s floor", 100 * time.Millisecond, 1 * time.Second},
		{"1s hits 1s floor", 1 * time.Second, 1 * time.Second},
		{"3s yields 1s (floor exact)", 3 * time.Second, 1 * time.Second},
		{"4s yields ~1333ms (non-round, above floor)", 4 * time.Second, 4 * time.Second / 3},
		{"6s yields 2s", 6 * time.Second, 2 * time.Second},
		{"30s yields 10s (default-config case)", 30 * time.Second, 10 * time.Second},
		{"1m yields 20s", 1 * time.Minute, 20 * time.Second},
		{"5m yields 100s", 5 * time.Minute, 100 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Interval(tc.timeout)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestPhaseMessage_DefaultsToStarting(t *testing.T) {
	var phase atomic.Pointer[string]
	msg := PhaseMessage("syncing job-x", &phase)
	assert.Equal(t, "syncing job-x: starting", msg())
}

func TestPhaseMessage_ReadsCurrentPhase(t *testing.T) {
	var phase atomic.Pointer[string]
	msg := PhaseMessage("syncing job-x", &phase)
	p := "writing"
	phase.Store(&p)
	assert.Equal(t, "syncing job-x: writing", msg())
}

// heartbeatCaptureOutbound is an activity outbound interceptor that records
// every RecordHeartbeat call (bypasses Temporal's throttling/dedup that the
// SetOnActivityHeartbeatListener test hook applies).
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
	h.ActivityOutboundInterceptorBase.RecordHeartbeat(ctx, details...)
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
	c.outbound.ActivityOutboundInterceptorBase.Next = o
	return c.ActivityInboundInterceptorBase.Init(c.outbound)
}

type capturingInterceptor struct {
	sdkinterceptor.WorkerInterceptorBase
	outbound *heartbeatCaptureOutbound
}

func (c *capturingInterceptor) InterceptActivity(_ context.Context, next sdkinterceptor.ActivityInboundInterceptor) sdkinterceptor.ActivityInboundInterceptor {
	in := &capturingInbound{outbound: c.outbound}
	in.ActivityInboundInterceptorBase.Next = next
	return in
}

func TestLoop_TicksAndStopsOnDone(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	// Sanity: SetOnActivityHeartbeatListener captures at least the first heartbeat,
	// but the interceptor below captures all calls.
	testEnv.SetOnActivityHeartbeatListener(func(_ *activity.Info, _ converter.EncodedValues) {})

	cap := &heartbeatCaptureOutbound{}
	testEnv.SetWorkerOptions(worker.Options{
		Interceptors: []sdkinterceptor.WorkerInterceptor{&capturingInterceptor{outbound: cap}},
	})

	act := func(ctx context.Context) error {
		var phase atomic.Pointer[string]
		p := "running"
		phase.Store(&p)
		done := make(chan struct{})
		go Loop(ctx, 50*time.Millisecond, PhaseMessage("syncing test", &phase), done)
		time.Sleep(180 * time.Millisecond)
		close(done)
		return nil
	}
	testEnv.RegisterActivity(act)

	_, err := testEnv.ExecuteActivity(act)
	require.NoError(t, err)

	got := cap.snapshot()
	require.GreaterOrEqual(t, len(got), 1, "expected at least one heartbeat, got %v", got)
	for _, m := range got {
		assert.Equal(t, "syncing test: running", m)
	}
}
