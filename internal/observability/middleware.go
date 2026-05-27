package observability

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

// RouterOptions carries observability dependencies for HTTP routing.
type RouterOptions struct {
	Logger  *slog.Logger
	Metrics *Metrics
}

// HTTPMiddleware records access logs, metrics, and spans.
func HTTPMiddleware(opts RouterOptions) func(http.Handler) http.Handler {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	tracer := otel.Tracer("avatar-service/http")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
			r = r.WithContext(ctx)
			route := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil {
				if pattern := rc.RoutePattern(); pattern != "" {
					route = pattern
				}
			}
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = r.Header.Get("X-Correlation-ID")
			}
			ctx = WithRequestID(r.Context(), requestID)
			ctx, span := tracer.Start(ctx, r.Method+" "+route)
			defer span.End()
			span.SetAttributes(attribute.String("http.route", route), attribute.String("http.method", r.Method))
			r = r.WithContext(ctx)
			start := time.Now()
			doneInflight := opts.Metrics.IncHTTPInflight(r.Method, route)
			defer doneInflight()
			sw := &StatusWriter{ResponseWriter: w, Status: http.StatusOK}
			next.ServeHTTP(sw, r)
			route = currentRoute(r)
			span.SetName(r.Method + " " + route)
			duration := time.Since(start)
			opts.Metrics.ObserveHTTP(r.Method, route, sw.Status, duration)
			span.SetAttributes(attribute.Int("http.status_code", sw.Status), attribute.String("http.route", route))
			if sw.Status >= http.StatusInternalServerError {
				span.SetStatus(codes.Error, http.StatusText(sw.Status))
			}
			log.InfoContext(ctx, "http access", Attrs(ctx,
				"method", r.Method,
				"route", route,
				"path", r.URL.Path,
				"status", sw.Status,
				"duration", duration.String(),
			)...)
		})
	}
}

func currentRoute(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if pattern := rc.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

// StatusWriter captures the response status code.
type StatusWriter struct {
	http.ResponseWriter
	Status int
	wrote  bool
}

// WriteHeader captures the response status code.
func (w *StatusWriter) WriteHeader(code int) {
	if w.wrote {
		return
	}
	w.wrote = true
	w.Status = code
	w.ResponseWriter.WriteHeader(code)
}

// Write captures implicit 200 responses.
func (w *StatusWriter) Write(data []byte) (int, error) {
	if !w.wrote {
		if w.Status == 0 {
			w.Status = http.StatusOK
		}
		w.WriteHeader(w.Status)
	}
	return w.ResponseWriter.Write(data)
}
