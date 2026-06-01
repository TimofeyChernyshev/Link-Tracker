package botmetrics

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
	commandRequestsTotal *prometheus.CounterVec

	commandDuration *prometheus.HistogramVec

	sentNotificationTotal prometheus.Counter

	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge

	memoryUsage prometheus.Gauge
}

func NewMetrics(memoryMetricTick time.Duration) *Metrics {
	m := &Metrics{
		commandRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "command_requests_total",
				Help: "Total number of processed commands",
			},
			[]string{"command"},
		),
		commandDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "command_duration_ms_total",
				Help:    "Duration of command processing in milliseconds",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000},
			},
			[]string{"scope", "scope_type"},
		),
		sentNotificationTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "sent_notification_total",
				Help: "Total number of sent notifications",
			},
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

// Реализация application.MetricsCollector
func (m *Metrics) RecordCommand(ctx context.Context, command string) {
	m.commandRequestsTotal.WithLabelValues(command).Inc()
}

func (m *Metrics) RecordCommandDuration(ctx context.Context, scope, scopeType string, durationMs float64) {
	m.commandDuration.WithLabelValues(scope, scopeType).Observe(durationMs)
}

func (m *Metrics) RecordNotificationSent(ctx context.Context) {
	m.sentNotificationTotal.Inc()
}

// Реализация bothttp.MetricsCollector
func (m *Metrics) RecordHTTPRequest(ctx context.Context, method, endpoint, status string) {
	m.httpRequestsTotal.WithLabelValues(method, endpoint, status).Inc()
}

func (m *Metrics) RecordHTTPRequestDuration(ctx context.Context, method, endpoint string, durationSeconds float64) {
	m.httpRequestDuration.WithLabelValues(method, endpoint).Observe(durationSeconds)
}

func (m *Metrics) RecordHTTPRequestsInFlight(ctx context.Context, delta int) {
	m.httpRequestsInFlight.Add(float64(delta))
}

// HTTP middleware для RED метрик
func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.RecordHTTPRequestsInFlight(r.Context(), 1)
		defer m.RecordHTTPRequestsInFlight(r.Context(), -1)

		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)

		duration := time.Since(start).Seconds()
		statusText := http.StatusText(rw.statusCode)
		if statusText == "" {
			statusText = "unknown"
		}
		m.RecordHTTPRequest(r.Context(), r.Method, r.URL.Path, statusText)
		m.RecordHTTPRequestDuration(r.Context(), r.Method, r.URL.Path, duration)
	})
}

func (m *Metrics) RunMetricsServer(port string) (shutdown func(ctx context.Context) error, err error) {
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

	shutdown = func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	}
	return shutdown, nil
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (m *Metrics) UpdateMemoryUsage(bytes uint64) {
	m.memoryUsage.Set(float64(bytes))
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
