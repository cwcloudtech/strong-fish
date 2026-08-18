package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"strong-fish-api/internal/utils"
)

// EndpointObserver is called once per completed request with the resolved
// route pattern, status and duration, so metrics are recorded without a second
// middleware re-deriving the same three facts.
type EndpointObserver func(route, method string, status int, duration time.Duration)

// Instrument opens a span for every request, logs one line for it, and reports
// it to observe.
//
// All three use the *resolved* chi route pattern rather than the raw path. The
// span has to be opened before the handler chain runs, when the pattern isn't
// known yet, so it starts named after the path and is renamed once chi has
// settled on a match - otherwise every request for a different program id
// would be its own endpoint, and the metrics would be unaggregatable.
func Instrument(tracer trace.Tracer, logger *slog.Logger, observe EndpointObserver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(recorder, r.WithContext(ctx))
			duration := time.Since(start)

			route := r.URL.Path
			if routeCtx := chi.RouteContext(r.Context()); routeCtx != nil {
				if pattern := routeCtx.RoutePattern(); utils.IsNotBlank(pattern) {
					route = pattern
				}
			}

			span.SetName(r.Method + " " + route)
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.status_code", recorder.status),
			)
			if recorder.status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(recorder.status))
			}

			logRequest(ctx, logger, r.Method, route, recorder.status, duration)

			if observe != nil {
				observe(route, r.Method, recorder.status, duration)
			}
		})
	}
}

// logRequest writes the access line at a level that matches the outcome, so a
// log search for errors finds the failing requests without having to know the
// status codes.
func logRequest(ctx context.Context, logger *slog.Logger, method, route string, status int, duration time.Duration) {
	level := slog.LevelInfo
	switch {
	case status >= http.StatusInternalServerError:
		level = slog.LevelError
	case status >= http.StatusBadRequest:
		level = slog.LevelWarn
	}
	logger.Log(ctx, level, "http request",
		slog.String("method", method),
		slog.String("route", route),
		slog.Int("status", status),
		slog.Duration("duration", duration),
	)
}

// statusRecorder remembers the status the handler wrote. net/http offers no
// way to read it back off a ResponseWriter, and a request that never calls
// WriteHeader answered 200.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
