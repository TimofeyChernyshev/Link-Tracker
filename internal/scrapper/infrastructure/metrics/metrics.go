package scrappermetrics

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	linksOnTrack     *prometheus.GaugeVec
	requestDuration  *prometheus.HistogramVec
	apiRequestsTotal *prometheus.CounterVec

	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge

	memoryUsage prometheus.Gauge
}

func NewMetrics(memoryMetricTick time.Duration) *Metrics {
	m := &Metrics{
		linksOnTrack: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "links_on_track_total",
				Help: "Total number of tracked links",
			},
			[]string{"tracked_source"},
		),

		requestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "request_duration_ms_total",
				Help:    "Duration of operations in milliseconds",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
			},
			[]string{"scope", "scope_type"},
		),

		apiRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "api_requests_total",
				Help: "Total number of API requests",
			},
			[]string{"source"},
		),

		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status"},
		),

		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		httpRequestsInFlight: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_in_flight",
				Help: "Current number of HTTP requests being processed",
			},
		),

		memoryUsage: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "memory_usage_bytes",
				Help: "Current memory usage in bytes",
			},
		),
	}

	go m.collectMemoryMetrics(memoryMetricTick)

	return m
}

func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.httpRequestsInFlight.Inc()
		defer m.httpRequestsInFlight.Dec()

		start := time.Now()

		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()

		m.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, http.StatusText(rw.statusCode)).Inc()
		m.httpRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration)
	})
}

func (m *Metrics) UpdateMemoryUsage(bytes uint64) {
	m.memoryUsage.Set(float64(bytes))
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (m *Metrics) RecordLinksTracked(ctx context.Context, domain string, count int) {
	m.linksOnTrack.WithLabelValues(domain).Set(float64(count))
}

func (m *Metrics) RecordRequestDuration(ctx context.Context, scope, scopeType string, durationMs float64) {
	m.requestDuration.WithLabelValues(scope, scopeType).Observe(durationMs)
}

func (m *Metrics) RecordAPIRequest(ctx context.Context, source string) {
	m.apiRequestsTotal.WithLabelValues(source).Inc()
}

func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, endpoint, status string) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}

func (m *Metrics) RecordHTTPRequestDuration(ctx context.Context, method, endpoint string, durationSeconds float64) {
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(durationSeconds)
}

func (m *Metrics) RecordHTTPRequestsInFlight(ctx context.Context, delta int) {
	m.httpRequestsInFlight.Add(float64(delta))
}

func (m *Metrics) RecordMemoryUsage(ctx context.Context, bytes uint64) {
	m.memoryUsage.Set(float64(bytes))
}

func (m *Metrics) RunMetricsServer(port string) (func(ctx context.Context) error, error) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go func() {
		slog.Info("metrics server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("metrics server error", "error", err)
		}
	}()

	shutdown := func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}
	return shutdown, nil
}

func (m *Metrics) collectMemoryMetrics(memoryMetricTick time.Duration) {
	ticker := time.NewTicker(memoryMetricTick)
	defer ticker.Stop()
	for range ticker.C {
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		m.UpdateMemoryUsage(memStats.Alloc)
	}
}
