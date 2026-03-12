package activity

import (
	"context"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	fn "github.com/jasoet/go-wf/function"
	"github.com/jasoet/go-wf/function/payload"
)

// init registers the OTel instrumentation wrapper with the parent function package.
// This hook pattern avoids an import cycle (function -> function/activity -> function).
func init() {
	fn.SetActivityInstrumenter(InstrumentedExecuteFunctionActivity)
}

const (
	functionMeterScope   = "go-wf/function/activity"
	functionTaskTotal    = "go_wf.function.task.total"
	functionTaskDuration = "go_wf.function.task.duration"
)

// InstrumentedExecuteFunctionActivity wraps a function activity with OTel spans and metrics.
// When OTel config is not in context, the wrapper is a transparent pass-through with zero overhead.
func InstrumentedExecuteFunctionActivity(
	inner func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error),
) func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
	return func(ctx context.Context, input payload.FunctionExecutionInput) (*payload.FunctionExecutionOutput, error) {
		cfg := pkgotel.ConfigFromContext(ctx)
		if cfg == nil {
			return inner(ctx, input)
		}

		lc := pkgotel.Layers.StartService(ctx, "function", "Execute",
			pkgotel.F("function.name", input.Name),
			pkgotel.F("function.has_data", len(input.Data) > 0),
			pkgotel.F("function.work_dir", input.WorkDir),
		)
		defer lc.End()

		output, err := inner(lc.Context(), input)
		if err != nil {
			//nolint:errcheck,gosec // we return the original err, not lc.Error's return
			lc.Error(err, "function execution failed")
			recordFunctionMetrics(lc.Context(), input.Name, "failure", time.Duration(0))
			return output, err
		}

		if output != nil {
			lc.Span.AddAttributes(
				pkgotel.F("function.duration", output.Duration.String()),
				pkgotel.F("function.has_result", len(output.Result) > 0),
			)

			status := "success"
			if !output.Success {
				status = "failure"
			}
			recordFunctionMetrics(lc.Context(), input.Name, status, output.Duration)

			if output.Success {
				lc.Success("function completed")
			} else {
				//nolint:errcheck,gosec // error is nil; we only set span status here
				lc.Error(nil, "function execution returned error",
					pkgotel.F("function.error", output.Error),
				)
			}
		}

		return output, nil
	}
}

// recordFunctionMetrics records counter and histogram metrics for function task execution.
func recordFunctionMetrics(ctx context.Context, functionName, status string, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(functionMeterScope)

	counter, err := meter.Int64Counter(functionTaskTotal,
		metric.WithDescription("Total number of function task executions"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("status", status),
				attribute.String("function_name", functionName),
			),
		)
	}

	histogram, err := meter.Float64Histogram(functionTaskDuration,
		metric.WithDescription("Duration of function task executions in seconds"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("function_name", functionName),
			),
		)
	}
}
