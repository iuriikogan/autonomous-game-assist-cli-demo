package trace

import (
	"context"
	"fmt"

	texporter "github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracerProvider initializes an OpenTelemetry TracerProvider backed by Google Cloud Trace.
// It returns a cleanup function that will flush and shut down the provider, or an error.
func InitTracerProvider(ctx context.Context, projectID string) (func(context.Context) error, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project ID is required to initialize Cloud Trace exporter")
	}

	// 1. Create the Cloud Trace exporter
	exporter, err := texporter.New(texporter.WithProjectID(projectID))
	if err != nil {
		return nil, fmt.Errorf("failed to create Cloud Trace exporter: %w", err)
	}

	// 2. Define the Resource describing our service
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("autonomous-game-assist-cli"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace resource: %w", err)
	}

	// 3. Set up TracerProvider with Batcher, Sampler (100%), and Resource
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)

	// 4. Set global OpenTelemetry providers
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Return clean shutdown handler to flush buffered spans on exit
	return tp.Shutdown, nil
}
