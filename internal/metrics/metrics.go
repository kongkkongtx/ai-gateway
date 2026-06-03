package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Collector struct {
	requestsTotal        *prometheus.CounterVec
	requestDuration      *prometheus.HistogramVec
	tokensTotal          *prometheus.CounterVec
	activeRequests       prometheus.Gauge
	upstreamHealth       *prometheus.GaugeVec
	rateLimitExceeded    *prometheus.CounterVec
	experimentRequests   *prometheus.CounterVec
	experimentLatency    *prometheus.HistogramVec
	registry             *prometheus.Registry
}

func NewCollector(reg *prometheus.Registry) *Collector {
	c := &Collector{registry: reg}
	c.requestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gateway_requests_total", Help: "Total number of requests processed"},
		[]string{"method", "path", "status", "model", "provider"},
	)
	c.requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "model", "provider"},
	)
	c.tokensTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gateway_tokens_total", Help: "Total number of tokens processed"},
		[]string{"type", "model", "provider"},
	)
	c.activeRequests = prometheus.NewGauge(
		prometheus.GaugeOpts{Name: "gateway_active_requests", Help: "Number of requests currently being processed"},
	)
	c.upstreamHealth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gateway_upstream_health",
			Help: "Health status of upstreams (1 = healthy, 0 = unhealthy)",
		},
		[]string{"upstream", "provider"},
	)
	c.rateLimitExceeded = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gateway_rate_limit_exceeded_total", Help: "Total number of rate-limited requests"},
		[]string{"type"},
	)
	c.experimentRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "gateway_experiment_requests_total", Help: "Total A/B experiment requests"},
		[]string{"experiment", "variant", "status"},
	)
	c.experimentLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gateway_experiment_latency_seconds",
			Help:    "A/B experiment request latencies",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"experiment", "variant"},
	)
	reg.MustRegister(c.requestsTotal, c.requestDuration, c.tokensTotal,
		c.activeRequests, c.upstreamHealth, c.rateLimitExceeded,
		c.experimentRequests, c.experimentLatency)
	return c
}

func (c *Collector) RecordRequest(method, path, status, model, provider string, duration time.Duration) {
	c.requestsTotal.WithLabelValues(method, path, status, model, provider).Inc()
	c.requestDuration.WithLabelValues(method, path, model, provider).Observe(duration.Seconds())
}

func (c *Collector) RecordTokens(tokenType, model, provider string, count int) {
	c.tokensTotal.WithLabelValues(tokenType, model, provider).Add(float64(count))
}

func (c *Collector) IncActiveRequests()  { c.activeRequests.Inc() }
func (c *Collector) DecActiveRequests()  { c.activeRequests.Dec() }

func (c *Collector) SetUpstreamHealth(upstream, provider string, healthy bool) {
	val := 0.0
	if healthy { val = 1.0 }
	c.upstreamHealth.WithLabelValues(upstream, provider).Set(val)
}

func (c *Collector) IncRateLimitExceeded(limitType string) {
	c.rateLimitExceeded.WithLabelValues(limitType).Inc()
}

func (c *Collector) RecordExperimentRequest(experimentID, variant, status string) {
	c.experimentRequests.WithLabelValues(experimentID, variant, status).Inc()
}

func (c *Collector) RecordExperimentLatency(experimentID, variant string, duration time.Duration) {
	c.experimentLatency.WithLabelValues(experimentID, variant).Observe(duration.Seconds())
}

func (c *Collector) Handler() http.Handler {
	return promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{})
}

func FormatStatus(code int) string { return strconv.Itoa(code) }