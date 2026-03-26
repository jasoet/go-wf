package activity

import (
	"context"
	"strings"
	"time"

	pkgotel "github.com/jasoet/pkg/v2/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/jasoet/go-wf/container/payload"
)

const (
	containerMeterScope   = "go-wf/container/activity"
	containerTaskTotal    = "go_wf.container.task.total"
	containerTaskDuration = "go_wf.container.task.duration"
)

// InstrumentedStartContainerActivity wraps a container activity with OTel spans and metrics.
// When OTel config is not in context, the wrapper is a transparent pass-through with zero overhead.
func InstrumentedStartContainerActivity(
	inner func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error),
) func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
	return func(ctx context.Context, input payload.ContainerExecutionInput) (*payload.ContainerExecutionOutput, error) {
		cfg := pkgotel.ConfigFromContext(ctx)
		if cfg == nil {
			return inner(ctx, input)
		}

		lc := pkgotel.Layers.StartService(ctx, "docker", "StartContainer",
			pkgotel.F("container.image", input.Image),
			pkgotel.F("container.name", input.Name),
			pkgotel.F("container.auto_remove", input.AutoRemove),
		)
		defer lc.End()

		if len(input.Command) > 0 {
			lc.Span.AddAttribute("container.command", strings.Join(input.Command, " "))
		}
		if input.WorkDir != "" {
			lc.Span.AddAttribute("container.work_dir", input.WorkDir)
		}

		output, err := inner(lc.Context(), input)
		if err != nil {
			//nolint:errcheck,gosec // we return the original error, not lc.Error's return
			lc.Error(err, "container execution failed")
			recordContainerMetrics(lc.Context(), input.Image, "failure", 0, time.Duration(0))
			return output, err
		}

		if output != nil {
			lc.Span.AddAttributes(
				pkgotel.F("container.id", output.ContainerID),
				pkgotel.F("container.exit_code", output.ExitCode),
				pkgotel.F("container.duration", output.Duration.String()),
			)
			if output.Endpoint != "" {
				lc.Span.AddAttribute("container.endpoint", output.Endpoint)
			}

			status := "success"
			if !output.Success {
				status = "failure"
			}
			recordContainerMetrics(lc.Context(), input.Image, status, output.ExitCode, output.Duration)

			if output.Success {
				lc.Success("container completed")
			} else {
				//nolint:errcheck,gosec // error is nil; we only set span status here
				lc.Error(nil, "container exited with error",
					pkgotel.F("container.exit_code", output.ExitCode),
					pkgotel.F("container.error", output.Error),
				)
			}
		}

		return output, nil
	}
}

// imageBaseName extracts the image name without tag for low-cardinality metrics.
func imageBaseName(image string) string {
	// Remove tag (after last colon, but not in registry port)
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		// Check if this colon is a tag separator (not a port in registry URL)
		afterColon := image[idx+1:]
		if !strings.Contains(afterColon, "/") {
			return image[:idx]
		}
	}
	return image
}

// recordContainerMetrics records counter and histogram metrics for docker task execution.
func recordContainerMetrics(ctx context.Context, image, status string, exitCode int, duration time.Duration) {
	cfg := pkgotel.ConfigFromContext(ctx)
	if cfg == nil {
		return
	}

	meter := cfg.GetMeter(containerMeterScope)

	counter, err := meter.Int64Counter(containerTaskTotal,
		metric.WithDescription("Total number of docker task executions"),
	)
	if err == nil {
		counter.Add(ctx, 1,
			metric.WithAttributes(
				attribute.String("status", status),
				attribute.String("image", imageBaseName(image)),
			),
		)
	}

	histogram, err := meter.Float64Histogram(containerTaskDuration,
		metric.WithDescription("Duration of docker task executions in seconds"),
		metric.WithUnit("s"),
	)
	if err == nil {
		histogram.Record(ctx, duration.Seconds(),
			metric.WithAttributes(
				attribute.String("image", imageBaseName(image)),
				attribute.Int("exit_code", exitCode),
			),
		)
	}
}
