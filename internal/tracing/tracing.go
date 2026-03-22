// Package tracing installs OpenTelemetry globals (tracer provider, propagators) for OTLP export.
package tracing

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config controls OTLP export. Endpoint is the gRPC host:port (no scheme), e.g. "localhost:4317" or "tempo:4317".
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
	Insecure       bool
	SampleRatio    float64
}

// Install configures the global TracerProvider and text map propagator. If cfg.OTLPEndpoint is empty,
// it returns (nil, nil) and leaves OTel defaults (noop) in place.
func Install(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if cfg.OTLPEndpoint == "" {
		return nil, nil
	}
	if cfg.ServiceName == "" {
		cfg.ServiceName = "helloworld"
	}
	if cfg.SampleRatio <= 0 || cfg.SampleRatio > 1 {
		cfg.SampleRatio = 1.0
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithTimeout(10 * time.Second),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exp, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("otlp trace exporter: %w", err)
	}

	attrs := []resource.Option{
		resource.WithAttributes(semconv.ServiceName(cfg.ServiceName)),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.ServiceVersion(cfg.ServiceVersion)))
	}
	res, err := resource.New(ctx, attrs...)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
