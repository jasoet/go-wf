package function

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	sdkactivity "go.temporal.io/sdk/activity"
)

// TestSetActivityInstrumenter verifies the instrumenter hook is installed
// at most once (sync.Once) and that RegisterActivity applies the installed
// instrumenter to typed activities.
//
// Under the integration build tag, function/activity may already have
// installed the OTel instrumenter before this test runs; the test therefore
// asserts the Once semantics and the wrapping behavior, not which specific
// wrapper wins.
func TestSetActivityInstrumenter(t *testing.T) {
	alreadyInstalled := instrumentActivity != nil

	var wrapped bool
	SetActivityInstrumenter(func(inner activityType) activityType {
		return func(ctx context.Context, in FunctionExecutionInput) (*FunctionExecutionOutput, error) {
			wrapped = true
			return inner(ctx, in)
		}
	})

	// Subsequent calls must be ignored (sync.Once).
	SetActivityInstrumenter(func(inner activityType) activityType {
		return func(context.Context, FunctionExecutionInput) (*FunctionExecutionOutput, error) {
			t.Fatal("second instrumenter must not be installed")
			return nil, nil
		}
	})

	mw := new(mockWorker)
	var registered any
	mw.On("RegisterActivityWithOptions", mock.Anything, sdkactivity.RegisterOptions{
		Name: "ExecuteFunctionActivity",
	}).Run(func(args mock.Arguments) {
		registered = args.Get(0)
	}).Return()

	stub := func(_ context.Context, _ FunctionExecutionInput) (*FunctionExecutionOutput, error) {
		return &FunctionExecutionOutput{Success: true}, nil
	}
	RegisterActivity(mw, stub)

	mw.AssertExpectations(t)

	// The registered activity must keep the typed signature.
	typed, ok := registered.(func(context.Context, FunctionExecutionInput) (*FunctionExecutionOutput, error))
	assert.True(t, ok, "registered activity must keep the typed signature")

	out, err := typed(context.Background(), FunctionExecutionInput{})
	assert.NoError(t, err)
	assert.True(t, out.Success)

	if alreadyInstalled {
		t.Log("instrumenter was installed before this test (integration build); skipping wrapper-identity assertion")
	} else {
		assert.True(t, wrapped, "instrumenter wrapper must wrap the registered activity")
	}
}
