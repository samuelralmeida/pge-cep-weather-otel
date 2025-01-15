package tracing

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func InitProvider(serviceName, collectorUrl string) (func(context.Context) error, error) {
	log.Printf("Connecting to OTEL Collector at %s\n", collectorUrl)
	ctx := context.Background()

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	conn, err := grpc.NewClient(collectorUrl, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithGRPCConn(conn),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	otel.SetTracerProvider(traceProvider)

	otel.SetTextMapPropagator(propagation.TraceContext{})

	return traceExporter.Shutdown, nil
}
