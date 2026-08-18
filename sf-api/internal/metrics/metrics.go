// Package metrics wires OpenTelemetry metrics up to two readers sharing one
// set of instruments: a Prometheus exporter behind GET /v1/metrics, and - when
// an endpoint is configured - a periodic OTLP push to the same collector that
// receives the traces and logs.
//
// Ported from ~/cwclock's metrics package. The business gauges differ, because
// what is worth watching in a powerlifting app is not what is worth watching in
// a time tracker.
package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprometheus "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"

	"strong-fish-api/internal/middleware"
	"strong-fish-api/internal/store"
	"strong-fish-api/internal/telemetry"
	"strong-fish-api/internal/utils"
)

const meterName = "strong-fish-api"

// Config is the same SF_OTEL_ENDPOINT/SF_OTEL_PROTO pair the traces and logs
// use: one collector for all three.
type Config struct {
	Endpoint string
	Proto    string
	Version  string
}

// Metrics bundles the /v1/metrics handler, the per-request observer the
// tracing middleware feeds, and a shutdown hook.
type Metrics struct {
	Handler  http.Handler
	Observe  middleware.EndpointObserver
	Shutdown func(context.Context) error
}

// Setup registers the Go/process collectors, the HTTP instruments and the
// business gauges.
func Setup(ctx context.Context, cfg Config, users *store.UserStore, clubs *store.ClubStore,
	programs *store.ProgramStore) (*Metrics, error) {
	res := resource.NewSchemaless(
		attribute.String("service.name", telemetry.ServiceName),
		attribute.String("service.version", cfg.Version),
	)

	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	promExporter, err := otelprometheus.New(otelprometheus.WithRegisterer(registry))
	if err != nil {
		return nil, fmt.Errorf("metrics: building the prometheus exporter: %w", err)
	}

	readerOpts := []sdkmetric.Option{sdkmetric.WithResource(res), sdkmetric.WithReader(promExporter)}
	var shutdownFuncs []func(context.Context) error

	if utils.IsNotBlank(cfg.Endpoint) {
		exporter, err := buildExporter(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("metrics: building the OTLP exporter: %w", err)
		}
		periodic := sdkmetric.NewPeriodicReader(exporter)
		readerOpts = append(readerOpts, sdkmetric.WithReader(periodic))
		shutdownFuncs = append(shutdownFuncs, periodic.Shutdown)
	}

	provider := sdkmetric.NewMeterProvider(readerOpts...)
	otel.SetMeterProvider(provider)
	shutdownFuncs = append(shutdownFuncs, provider.Shutdown)

	meter := provider.Meter(meterName)

	requests, err := meter.Int64Counter("http_server_requests_total",
		metric.WithDescription("Count of HTTP requests per endpoint"))
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram("http_server_request_duration_seconds",
		metric.WithDescription("Duration of HTTP requests per endpoint"),
		metric.WithUnit("s"))
	if err != nil {
		return nil, err
	}

	if err := registerGauges(meter, users, clubs, programs); err != nil {
		return nil, err
	}

	observe := func(route, method string, status int, elapsed time.Duration) {
		attrs := metric.WithAttributes(
			attribute.String("route", route),
			attribute.String("method", method),
			attribute.Int("status", status),
		)
		requests.Add(context.Background(), 1, attrs)
		duration.Record(context.Background(), elapsed.Seconds(), attrs)
	}

	return &Metrics{
		Handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry}),
		Observe: observe,
		Shutdown: func(ctx context.Context) error {
			var errs []error
			for _, fn := range shutdownFuncs {
				if err := fn(ctx); err != nil {
					errs = append(errs, err)
				}
			}
			if len(errs) > 0 {
				return fmt.Errorf("metrics shutdown: %v", errs)
			}
			return nil
		},
	}, nil
}

// registerGauges wires the app's own counts to a single callback, so one scrape
// costs three cheap queries rather than one per gauge - and so a scrape can
// never observe a half-updated set.
func registerGauges(meter metric.Meter, users *store.UserStore, clubs *store.ClubStore,
	programs *store.ProgramStore) error {
	usersGauge, err := meter.Int64ObservableGauge("strong_fish_users_total",
		metric.WithDescription("Number of accounts per role"))
	if err != nil {
		return err
	}
	clubsGauge, err := meter.Int64ObservableGauge("strong_fish_clubs_total",
		metric.WithDescription("Number of clubs"))
	if err != nil {
		return err
	}
	programsGauge, err := meter.Int64ObservableGauge("strong_fish_programs_total",
		metric.WithDescription("Number of programs"))
	if err != nil {
		return err
	}

	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		// A failing count is skipped rather than failing the scrape: a gauge
		// that disappears for one interval is better than losing every other
		// metric in the same collection.
		if byRole, err := users.CountByRole(ctx); err == nil {
			for role, count := range byRole {
				observer.ObserveInt64(usersGauge, count, metric.WithAttributes(attribute.String("role", role)))
			}
		}
		if count, err := clubs.Count(ctx); err == nil {
			observer.ObserveInt64(clubsGauge, count)
		}
		if count, err := programs.Count(ctx); err == nil {
			observer.ObserveInt64(programsGauge, count)
		}
		return nil
	}, usersGauge, clubsGauge, programsGauge)
	return err
}

func buildExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
	if cfg.Proto == telemetry.ProtoHTTP {
		return otlpmetrichttp.New(ctx, otlpmetrichttp.WithEndpoint(cfg.Endpoint), otlpmetrichttp.WithInsecure())
	}
	return otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithEndpoint(cfg.Endpoint), otlpmetricgrpc.WithInsecure())
}
