package metrics

import "testing"

func TestBoundedLabels(t *testing.T) {
	m := New([]string{"adshield"})
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
