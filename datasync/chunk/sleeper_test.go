package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.temporal.io/sdk/testsuite"
)

func TestHeartbeatSleeper_RespectsContextCancel(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	act := func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		start := time.Now()
		HeartbeatSleeper(ctx, 10*time.Second)
		elapsed := time.Since(start)
		assert.Less(t, elapsed, 5*time.Second, "should return promptly after cancel")
		return nil
	}
	testEnv.RegisterActivity(act)
	_, err := testEnv.ExecuteActivity(act)
	assert.NoError(t, err)
}

func TestHeartbeatSleeper_Returns_OnDeadline(t *testing.T) {
	env := &testsuite.WorkflowTestSuite{}
	testEnv := env.NewTestActivityEnvironment()

	act := func(ctx context.Context) error {
		start := time.Now()
		HeartbeatSleeper(ctx, 80*time.Millisecond)
		assert.GreaterOrEqual(t, time.Since(start), 80*time.Millisecond)
		return nil
	}
	testEnv.RegisterActivity(act)
	_, err := testEnv.ExecuteActivity(act)
	assert.NoError(t, err)
}
