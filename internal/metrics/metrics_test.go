package metrics

import (
	"testing"

	"github.com/prometheus/client_model/go"
)

func TestBoundedLabels(t *testing.T) {
	m := New([]string{"adshield"}, false, 100)
	m.Request("adshield", "matched")
	m.Request("anything", "unknown_path")
	m.Request("adshield", "anything")
	m.HandshakeFailure("unknown_sni")
	m.HandshakeFailure("anything")
	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() == "ars_requests_total" && len(family.Metric) != 2 {
			t.Fatalf("requests metrics=%d", len(family.Metric))
		}
	}
}

func TestRefererHost(t *testing.T) {
	tests := []struct {
		value, want string
	}{
		{"https://WWW.Example.com/path", "www.example.com"},
		{"https://example.com:8443/path", "example.com"},
		{"https://example.com/path", "example.com"},
		{"", "unknown"},
		{"://example.com", "unknown"},
		{"ftp://example.com/path", "unknown"},
	}
	for _, test := range tests {
		if got := refererHost(test.value); got != test.want {
			t.Fatalf("refererHost(%q)=%q", test.value, got)
		}
	}
}

func TestSiteRequests(t *testing.T) {
	m := New([]string{"adshield"}, true, 1)
	m.SiteRequest("", "matched")
	m.SiteRequest("https://first.example", "matched")
	m.SiteRequest("https://second.example", "matched")
	m.SiteRequest("https://third.example", "unknown_path")
	if got := counterValue(t, m, "ars_site_requests_total", map[string]string{"site": "unknown"}); got != 1 {
		t.Fatalf("unknown=%v", got)
	}
	if got := counterValue(t, m, "ars_site_requests_total", map[string]string{"site": "first.example"}); got != 1 {
		t.Fatalf("first=%v", got)
	}
	if got := counterValue(t, m, "ars_site_requests_total", map[string]string{"site": "other"}); got != 1 {
		t.Fatalf("other=%v", got)
	}
}

func TestSiteRequestsDisabled(t *testing.T) {
	m := New([]string{"adshield"}, false, 1)
	m.SiteRequest("https://example.com", "matched")
	for _, family := range metricFamilies(t, m) {
		if family.GetName() == "ars_site_requests_total" {
			t.Fatal("site metric present")
		}
	}
}

func counterValue(t *testing.T, m *Metrics, name string, labels map[string]string) float64 {
	t.Helper()
	for _, family := range metricFamilies(t, m) {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.Metric {
			if matches(metric, labels) {
				return metric.GetCounter().GetValue()
			}
		}
	}
	return 0
}

func metricFamilies(t *testing.T, m *Metrics) []*io_prometheus_client.MetricFamily {
	t.Helper()
	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	return families
}

func matches(metric *io_prometheus_client.Metric, labels map[string]string) bool {
	if len(metric.Label) != len(labels) {
		return false
	}
	for _, label := range metric.Label {
		if labels[label.GetName()] != label.GetValue() {
			return false
		}
	}
	return true
}
