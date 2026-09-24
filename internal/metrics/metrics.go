package metrics

import (
	"net/url"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	requests     *prometheus.CounterVec
	siteRequests *prometheus.CounterVec
	handshake    *prometheus.CounterVec
	cache        prometheus.Gauge
	expiry       prometheus.Gauge
	profiles     map[string]bool
	sitesEnabled bool
	sitesMax     int
	sites        map[string]bool
	mutex        sync.Mutex
	registry     *prometheus.Registry
}

func New(profiles []string, sitesEnabled bool, sitesMax int) *Metrics {
	registry := prometheus.NewRegistry()
	requests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ars_requests_total"}, []string{"profile", "result"})
	siteRequests := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ars_site_requests_total"}, []string{"site"})
	handshake := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "ars_tls_handshake_failures_total"}, []string{"reason"})
	cache := prometheus.NewGauge(prometheus.GaugeOpts{Name: "ars_cert_cache_entries"})
	expiry := prometheus.NewGauge(prometheus.GaugeOpts{Name: "ars_issuer_expiry_timestamp_seconds"})
	info := prometheus.NewGaugeVec(prometheus.GaugeOpts{Name: "ars_profile_info"}, []string{"profile", "version"})
	registry.MustRegister(requests, siteRequests, handshake, cache, expiry, info)
	known := map[string]bool{}
	for _, profile := range profiles {
		known[profile] = true
		info.WithLabelValues(profile, "1").Set(1)
	}
	return &Metrics{requests: requests, siteRequests: siteRequests, handshake: handshake, cache: cache, expiry: expiry, profiles: known, sitesEnabled: sitesEnabled, sitesMax: sitesMax, sites: map[string]bool{}, registry: registry}
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
func (m *Metrics) SiteRequest(referer, result string) {
	if m == nil || !m.sitesEnabled || result != "matched" {
		return
	}
	site := refererHost(referer)
	m.mutex.Lock()
	if site != "unknown" && site != "other" && !m.sites[site] {
		if len(m.sites) >= m.sitesMax {
			site = "other"
		} else {
			m.sites[site] = true
		}
	}
	m.mutex.Unlock()
	m.siteRequests.WithLabelValues(site).Inc()
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

func refererHost(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "unknown"
	}
	return strings.ToLower(parsed.Hostname())
}
