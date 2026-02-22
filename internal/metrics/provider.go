package metrics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const (
	serviceName        = "dzsa_sync"
	meterName          = "dzsa-sync"
	requestCount       = "request_count"
	requestLatency     = "request_latency_seconds"
	serverPlayerCount  = "server_player_count"
)

// Provider sets up OpenTelemetry metrics and Prometheus exposition.
type Provider struct {
	provider *sdkmetric.MeterProvider
}

// NewProvider creates a new metrics provider. Call Start before using the returned HTTPRecorder.
func NewProvider() (*Provider, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname: %w", err)
	}
	r := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.HostNameKey.String(hostname),
	)
	exporter, err := prometheus.New(prometheus.WithNamespace(serviceName))
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter: %w", err)
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(r),
	)
	otel.SetMeterProvider(provider)
	return &Provider{provider: provider}, nil
}

// Start is a no-op; initialization is done in NewProvider.
func (p *Provider) Start(_ context.Context) error {
	return nil
}

// Shutdown shuts down the meter provider.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p.provider != nil {
		return p.provider.Shutdown(ctx)
	}
	return nil
}

// Handler returns an http.Handler that serves Prometheus metrics at /metrics.
func (p *Provider) Handler() http.Handler {
	return promhttp.Handler()
}

// NewHTTPRecorder returns an HTTPRecorder that records RequestCount and RequestLatency.
func NewHTTPRecorder() (HTTPRecorder, error) {
	meter := otel.Meter(meterName)
	counter, err := meter.Int64Counter(requestCount)
	if err != nil {
		return nil, fmt.Errorf("request_count counter: %w", err)
	}
	histogram, err := meter.Float64Histogram(requestLatency)
	if err != nil {
		return nil, fmt.Errorf("request_latency histogram: %w", err)
	}
	return &otelRecorder{counter: counter, histogram: histogram}, nil
}

// NewPlayerCountRecorder returns a PlayerCountRecorder that records server_player_count (gauge).
func NewPlayerCountRecorder() (PlayerCountRecorder, error) {
	meter := otel.Meter(meterName)
	gauge, err := meter.Int64Gauge(serverPlayerCount)
	if err != nil {
		return nil, fmt.Errorf("server_player_count gauge: %w", err)
	}
	return &playerCountRecorder{gauge: gauge}, nil
}

type otelRecorder struct {
	counter   metric.Int64Counter
	histogram metric.Float64Histogram
}

func (r *otelRecorder) RecordRequest(ctx context.Context, host string, statusCode int, errType string, duration time.Duration) {
	attrs := attribute.NewSet(
		attribute.String("host", host),
		attribute.Int("status_code", statusCode),
		attribute.String("error", errType),
	)
	r.counter.Add(ctx, 1, metric.WithAttributeSet(attrs))
	attrsLatency := attribute.NewSet(
		attribute.String("host", host),
		attribute.Int("status_code", statusCode),
	)
	r.histogram.Record(ctx, duration.Seconds(), metric.WithAttributeSet(attrsLatency))
}

type playerCountRecorder struct {
	gauge metric.Int64Gauge
}

func (r *playerCountRecorder) RecordServerPlayerCount(ctx context.Context, serverName string, count int64) {
	attrs := attribute.NewSet(attribute.String("server", serverName))
	r.gauge.Record(ctx, count, metric.WithAttributeSet(attrs))
}
