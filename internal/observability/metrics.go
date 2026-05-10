package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns all application Prometheus collectors.
type Metrics struct {
	registry *prometheus.Registry

	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
	httpInflight *prometheus.GaugeVec

	avatarUploads      *prometheus.CounterVec
	avatarUploadDur    *prometheus.HistogramVec
	avatarDeletes      *prometheus.CounterVec
	avatarStorageBytes *prometheus.GaugeVec
	workerMessages     *prometheus.CounterVec
	workerProcessing   *prometheus.HistogramVec
	workerThumbnails   *prometheus.CounterVec
	dependencyOps      *prometheus.CounterVec
	dependencyDur      *prometheus.HistogramVec
}

// NewMetrics creates and registers application collectors in the supplied registry.
func NewMetrics(reg *prometheus.Registry) *Metrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	m := &Metrics{
		registry: reg,
		httpRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests.",
		}, []string{"method", "route", "status"}),
		httpDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "route", "status"}),
		httpInflight: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "http_inflight_requests",
			Help: "In-flight HTTP requests.",
		}, []string{"method", "route"}),
		avatarUploads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "avatars_uploads_total",
			Help: "Total avatar uploads.",
		}, []string{"status", "mime_type"}),
		avatarUploadDur: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "avatars_upload_duration_seconds",
			Help:    "Avatar upload duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"status", "mime_type"}),
		avatarDeletes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "avatars_deletes_total",
			Help: "Total avatar delete operations.",
		}, []string{"status"}),
		avatarStorageBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "avatars_storage_bytes",
			Help: "Observed avatar storage bytes by object kind.",
		}, []string{"kind"}),
		workerMessages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "avatar_worker_messages_total",
			Help: "Total worker messages.",
		}, []string{"routing_key", "status"}),
		workerProcessing: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "avatar_worker_processing_duration_seconds",
			Help:    "Worker message processing duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"routing_key", "status"}),
		workerThumbnails: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "avatar_worker_thumbnail_generation_total",
			Help: "Total worker thumbnail generation attempts.",
		}, []string{"size", "status"}),
		dependencyOps: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "avatar_dependency_operations_total",
			Help: "Total dependency operations.",
		}, []string{"component", "operation", "status"}),
		dependencyDur: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "avatar_dependency_operation_duration_seconds",
			Help:    "Dependency operation duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"component", "operation", "status"}),
	}
	reg.MustRegister(m.httpRequests, m.httpDuration, m.httpInflight, m.avatarUploads, m.avatarUploadDur, m.avatarDeletes, m.avatarStorageBytes, m.workerMessages, m.workerProcessing, m.workerThumbnails, m.dependencyOps, m.dependencyDur)
	return m
}

// Handler exposes registered metrics in Prometheus text format.
func (m *Metrics) Handler() http.Handler {
	if m == nil {
		return promhttp.HandlerFor(prometheus.NewRegistry(), promhttp.HandlerOpts{})
	}
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// ObserveHTTP records one HTTP request.
func (m *Metrics) ObserveHTTP(method, route string, status int, duration time.Duration) {
	if m == nil {
		return
	}
	statusLabel := strconv.Itoa(status)
	m.httpRequests.WithLabelValues(method, route, statusLabel).Inc()
	m.httpDuration.WithLabelValues(method, route, statusLabel).Observe(duration.Seconds())
}

// IncHTTPInflight updates the in-flight HTTP gauge.
func (m *Metrics) IncHTTPInflight(method, route string) func() {
	if m == nil {
		return func() {}
	}
	m.httpInflight.WithLabelValues(method, route).Inc()
	return func() { m.httpInflight.WithLabelValues(method, route).Dec() }
}

// IncAvatarUpload records an avatar upload outcome.
func (m *Metrics) IncAvatarUpload(status, mimeType string) {
	if m != nil {
		m.avatarUploads.WithLabelValues(status, mimeType).Inc()
	}
}

// ObserveAvatarUpload records an avatar upload outcome and duration.
func (m *Metrics) ObserveAvatarUpload(status, mimeType string, duration time.Duration) {
	if m != nil {
		m.avatarUploads.WithLabelValues(status, mimeType).Inc()
		m.avatarUploadDur.WithLabelValues(status, mimeType).Observe(duration.Seconds())
	}
}

// IncAvatarDelete records an avatar delete outcome.
func (m *Metrics) IncAvatarDelete(status string) {
	if m != nil {
		m.avatarDeletes.WithLabelValues(status).Inc()
	}
}

// IncWorkerMessage records a worker message outcome.
func (m *Metrics) IncWorkerMessage(routingKey, status string) {
	if m != nil {
		m.workerMessages.WithLabelValues(routingKey, status).Inc()
	}
}

// ObserveWorkerMessage records a worker message outcome and duration.
func (m *Metrics) ObserveWorkerMessage(routingKey, status string, duration time.Duration) {
	if m != nil {
		m.workerMessages.WithLabelValues(routingKey, status).Inc()
		m.workerProcessing.WithLabelValues(routingKey, status).Observe(duration.Seconds())
	}
}

// IncWorkerThumbnail records thumbnail generation by size and status.
func (m *Metrics) IncWorkerThumbnail(size, status string) {
	if m != nil {
		m.workerThumbnails.WithLabelValues(size, status).Inc()
	}
}

// SetStorageBytes records observed storage bytes by kind.
func (m *Metrics) SetStorageBytes(kind string, bytes int64) {
	if m != nil {
		m.avatarStorageBytes.WithLabelValues(kind).Set(float64(bytes))
	}
}

// IncDependencyOperation records an adapter operation outcome.
func (m *Metrics) IncDependencyOperation(component, operation, status string) {
	if m != nil {
		m.dependencyOps.WithLabelValues(component, operation, status).Inc()
	}
}

// ObserveDependencyOperation records an adapter operation outcome and duration.
func (m *Metrics) ObserveDependencyOperation(component, operation, status string, duration time.Duration) {
	if m != nil {
		m.dependencyOps.WithLabelValues(component, operation, status).Inc()
		m.dependencyDur.WithLabelValues(component, operation, status).Observe(duration.Seconds())
	}
}
