package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requests  *prometheus.CounterVec
	handshake *prometheus.CounterVec
	cache     prometheus.Gauge
	expiry    prometheus.Gauge
	profiles  map[string]bool
	registry  *prometheus.Registry
}

func New(profiles []string) *Metrics {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ars_requests_total"}, []string{"profile", "result"})
	handshake := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ars_tls_handshake_failures_total"}, []string{"reason"})
	cache := prometheus.NewGauge(prometheus.GaugeOpts{Name: "ars_cert_cache_entries"})
	expiry := prometheus.NewGauge(prometheus.GaugeOpts{Name: "ars_issuer_expiry_timestamp_seconds"})
	info := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "ars_profile_info"}, []string{"profile", "version"})
	registry.MustRegister(requests, handshake, cache, expiry, info)
	known := map[string]bool{}
	for _, profile := range profiles {
		known[profile] = true
		info.WithLabelValues(profile, "1").Set(1)
	}
	return &Metrics{requests: requests, handshake: handshake, cache: cache, expiry: expiry, profiles: known, registry: registry}
}

func (m *Metrics) Request(profile, result string) {
	if m == nil {
		return
	}
	if !m.profiles[profile] {
		profile = "unknown"
	}
	if result != "matched" && result != "unknown_host" && result != "unknown_path" && result != "method_not_allowed" {
		return
	}
	m.requests.WithLabelValues(profile, result).Inc()
}
func (m *Metrics) HandshakeFailure(reason string) {
	if m != nil && (reason == "unknown_sni" || reason == "other") {
		m.handshake.WithLabelValues(reason).Inc()
	}
}
func (m *Metrics) SetCertCacheEntries(value float64) {
	if m != nil {
		m.cache.Set(value)
	}
}
func (m *Metrics) SetIssuerExpiry(value float64) {
	if m != nil {
		m.expiry.Set(value)
	}
}
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }
