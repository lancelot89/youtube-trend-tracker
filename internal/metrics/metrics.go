package metrics

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all application metrics
type Metrics struct {
	// Counters
	VideosProcessed   prometheus.Counter
	APICallsTotal     *prometheus.CounterVec
	BigQueryInserts   *prometheus.CounterVec
	ErrorsTotal       *prometheus.CounterVec
	
	// Histograms for latency
	APICallDuration      *prometheus.HistogramVec
	BigQueryDuration     *prometheus.HistogramVec
	ProcessingDuration   prometheus.Histogram
	
	// Gauges
	LastRunTimestamp     prometheus.Gauge
	APIQuotaRemaining    prometheus.Gauge
	ActiveConnections    prometheus.Gauge
	
	mu sync.RWMutex
	registry *prometheus.Registry
}

// NewMetrics creates and registers all metrics
func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	
	m := &Metrics{
		registry: registry,
		
		VideosProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "ytt_videos_processed_total",
			Help: "Total number of videos processed",
		}),
		
		APICallsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ytt_api_calls_total",
				Help: "Total number of API calls",
			},
			[]string{"api", "method", "status"},
		),
		
		BigQueryInserts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ytt_bigquery_inserts_total",
				Help: "Total number of BigQuery insert operations",
			},
			[]string{"dataset", "table", "status"},
		),
		
		ErrorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "ytt_errors_total",
				Help: "Total number of errors",
			},
			[]string{"component", "type"},
		),
		
		APICallDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ytt_api_call_duration_seconds",
				Help:    "Duration of API calls in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"api", "method"},
		),
		
		BigQueryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "ytt_bigquery_operation_duration_seconds",
				Help:    "Duration of BigQuery operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "dataset", "table"},
		),
		
		ProcessingDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "ytt_processing_duration_seconds",
				Help:    "Total processing duration in seconds",
				Buckets: prometheus.ExponentialBuckets(1, 2, 10),
			},
		),
		
		LastRunTimestamp: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ytt_last_run_timestamp",
				Help: "Timestamp of the last successful run",
			},
		),
		
		APIQuotaRemaining: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ytt_api_quota_remaining",
				Help: "Remaining API quota",
			},
		),
		
		ActiveConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "ytt_active_connections",
				Help: "Number of active connections",
			},
		),
	}
	
	// Register all metrics
	registry.MustRegister(
		m.VideosProcessed,
		m.APICallsTotal,
		m.BigQueryInserts,
		m.ErrorsTotal,
		m.APICallDuration,
		m.BigQueryDuration,
		m.ProcessingDuration,
		m.LastRunTimestamp,
		m.APIQuotaRemaining,
		m.ActiveConnections,
	)
	
	// Register default Go metrics
	registry.MustRegister(prometheus.NewGoCollector())
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	
	return m
}

// Handler returns the HTTP handler for metrics endpoint
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// RecordAPICall records an API call with its duration
func (m *Metrics) RecordAPICall(api, method, status string, duration time.Duration) {
	m.APICallsTotal.WithLabelValues(api, method, status).Inc()
	m.APICallDuration.WithLabelValues(api, method).Observe(duration.Seconds())
}

// RecordBigQueryOp records a BigQuery operation with its duration
func (m *Metrics) RecordBigQueryOp(operation, dataset, table, status string, duration time.Duration) {
	m.BigQueryInserts.WithLabelValues(dataset, table, status).Inc()
	m.BigQueryDuration.WithLabelValues(operation, dataset, table).Observe(duration.Seconds())
}

// RecordError records an error occurrence
func (m *Metrics) RecordError(component, errorType string) {
	m.ErrorsTotal.WithLabelValues(component, errorType).Inc()
}

// RecordVideosProcessed increments the videos processed counter
func (m *Metrics) RecordVideosProcessed(count int) {
	m.VideosProcessed.Add(float64(count))
}

// SetLastRunTimestamp updates the last run timestamp
func (m *Metrics) SetLastRunTimestamp() {
	m.LastRunTimestamp.SetToCurrentTime()
}

// SetAPIQuotaRemaining updates the remaining API quota
func (m *Metrics) SetAPIQuotaRemaining(quota float64) {
	m.APIQuotaRemaining.Set(quota)
}

// Timer is a helper for timing operations
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// ObserveDuration returns the duration since timer creation
func (t *Timer) ObserveDuration() time.Duration {
	return time.Since(t.start)
}

// StartMetricsServer starts the metrics HTTP server
func StartMetricsServer(ctx context.Context, port int, metrics *Metrics) error {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	})
	
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("Failed to shutdown metrics server", "error", err)
		}
	}()
	
	slog.Info("Starting metrics server", "port", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("metrics server error: %w", err)
	}
	
	return nil
}