package metrics

import (
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Records processed metrics
	recordsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parser_records_processed_total",
			Help: "Total number of records processed",
		},
		[]string{"type", "status"}, // type: event_log/tech_log, status: success/failed
	)

	// Records written metrics
	recordsWrittenTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parser_records_written_total",
			Help: "Total number of records written to ClickHouse",
		},
		[]string{"type"}, // type: event_log/tech_log
	)

	// Batch size metrics
	batchSizeHistogram = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "parser_batch_size",
			Help:    "Size of batches written to ClickHouse",
			Buckets: []float64{10, 50, 100, 500, 1000, 5000, 10000},
		},
		[]string{"type"}, // type: event_log/tech_log
	)

	// Processing duration metrics
	processingDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "parser_processing_duration_seconds",
			Help:    "Duration of processing operations",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10},
		},
		[]string{"operation"}, // operation: parse/write/flush
	)

	// ClickHouse connection metrics
	clickhouseConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "clickhouse_connections_active",
			Help: "Number of active ClickHouse connections",
		},
	)

	// Error metrics
	errorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parser_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type", "component"}, // type: error/warning, component: parser/writer/clickhouse
	)

	// Dead letter queue metrics
	dlqRecordsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parser_dlq_records_total",
			Help: "Total number of records added to dead letter queue",
		},
		[]string{"type"}, // type: event_log/tech_log
	)

	// Circuit breaker metrics
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "circuit_breaker_state",
			Help: "Circuit breaker state (0=closed, 1=open, 2=half-open)",
		},
		[]string{"component"}, // component: clickhouse
	)

	// Rate limiter metrics
	rateLimitRejectedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "rate_limit_rejected_total",
			Help: "Total number of requests rejected by rate limiter",
		},
	)
)

// MetricsCollector collects and exposes metrics
type MetricsCollector struct {
	mu sync.RWMutex
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

// RecordProcessed increments processed records counter
func (m *MetricsCollector) RecordProcessed(recordType, status string) {
	recordsProcessedTotal.WithLabelValues(recordType, status).Inc()
}

// RecordWritten increments written records counter
func (m *MetricsCollector) RecordWritten(recordType string) {
	recordsWrittenTotal.WithLabelValues(recordType).Inc()
}

// RecordBatchSize records batch size
func (m *MetricsCollector) RecordBatchSize(recordType string, size int) {
	batchSizeHistogram.WithLabelValues(recordType).Observe(float64(size))
}

// RecordProcessingDuration records processing duration
func (m *MetricsCollector) RecordProcessingDuration(operation string, durationSeconds float64) {
	processingDuration.WithLabelValues(operation).Observe(durationSeconds)
}

// SetClickHouseConnections sets active connections count
func (m *MetricsCollector) SetClickHouseConnections(count int) {
	clickhouseConnections.Set(float64(count))
}

// RecordError increments error counter
func (m *MetricsCollector) RecordError(errorType, component string) {
	errorsTotal.WithLabelValues(errorType, component).Inc()
}

// RecordDLQ increments dead letter queue counter
func (m *MetricsCollector) RecordDLQ(recordType string) {
	dlqRecordsTotal.WithLabelValues(recordType).Inc()
}

// SetCircuitBreakerState sets circuit breaker state
func (m *MetricsCollector) SetCircuitBreakerState(component string, state int) {
	circuitBreakerState.WithLabelValues(component).Set(float64(state))
}

// RecordRateLimitRejected increments rate limit rejected counter
func (m *MetricsCollector) RecordRateLimitRejected() {
	rateLimitRejectedTotal.Inc()
}

// Thread-safe counters for internal use
var (
	successRecordsCounter int64
	failedRecordsCounter   int64
)

// IncrementSuccessRecords atomically increments success records counter
func IncrementSuccessRecords() {
	atomic.AddInt64(&successRecordsCounter, 1)
}

// IncrementFailedRecords atomically increments failed records counter
func IncrementFailedRecords() {
	atomic.AddInt64(&failedRecordsCounter, 1)
}

// GetSuccessRecords returns current success records count
func GetSuccessRecords() int64 {
	return atomic.LoadInt64(&successRecordsCounter)
}

// GetFailedRecords returns current failed records count
func GetFailedRecords() int64 {
	return atomic.LoadInt64(&failedRecordsCounter)
}

// HTTPHandler returns HTTP handler for Prometheus metrics endpoint
func HTTPHandler() http.Handler {
	return promhttp.Handler()
}

// Global functions for direct metric access (wrapper functions)
// These allow using metrics without creating a MetricsCollector instance

// RecordProcessingDuration records processing duration (global function)
func RecordProcessingDuration(operation string, durationSeconds float64) {
	processingDuration.WithLabelValues(operation).Observe(durationSeconds)
}

// RecordBatchSize records batch size (global function)
func RecordBatchSize(recordType string, size int) {
	batchSizeHistogram.WithLabelValues(recordType).Observe(float64(size))
}

// RecordRateLimitRejected increments rate limit rejected counter (global function)
func RecordRateLimitRejected() {
	rateLimitRejectedTotal.Inc()
}

// RecordProcessed increments processed records counter (global function)
func RecordProcessed(recordType, status string) {
	recordsProcessedTotal.WithLabelValues(recordType, status).Inc()
}

// RecordWritten increments written records counter (global function)
func RecordWritten(recordType string) {
	recordsWrittenTotal.WithLabelValues(recordType).Inc()
}

// RecordError increments error counter (global function)
func RecordError(errorType, component string) {
	errorsTotal.WithLabelValues(errorType, component).Inc()
}

// RecordDLQ increments dead letter queue counter (global function)
func RecordDLQ(recordType string) {
	dlqRecordsTotal.WithLabelValues(recordType).Inc()
}

// SetCircuitBreakerState sets circuit breaker state (global function)
func SetCircuitBreakerState(component string, state int) {
	circuitBreakerState.WithLabelValues(component).Set(float64(state))
}

