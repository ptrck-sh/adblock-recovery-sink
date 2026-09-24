package ops

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/metrics"
)

func TestEndpoints(t *testing.T) {
	handler := New(func() error { return errors.New("not ready") }, func() Status { return Status{} }, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) }), metrics.New([]string{"adshield"}).Registry())
	tests := []struct {
		path   string
		status int
	}{{"/healthz", 200}, {"/readyz", 503}, {"/metrics", 200}, {"/install", 201}, {"/missing", 404}}
	for _, test := range tests {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, test.path, nil))
		if recorder.Code != test.status {
			t.Fatalf("%s got %d", test.path, recorder.Code)
		}
	}
}

func TestStatus(t *testing.T) {
	secret := "private-key-value"
	handler := New(func() error { return errors.New("draining") }, func() Status {
		return Status{
			Version:  "v1.2.3",
			Hostname: "sink.example",
			Profiles: []string{"adshield"},
			Hosts:    []string{"html-load.com"},
			PKI: PKIStatus{
				Ready:                true,
				RootFingerprint:      "AA:BB",
				RootNotAfter:         "2030-01-02T03:04:05Z",
				IntermediateNotAfter: "2029-01-02T03:04:05Z",
				LeafCacheEntries:     2,
			},
		}
	}, nil, metrics.New([]string{"adshield"}).Registry())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type=%q", response.Header().Get("Content-Type"))
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache control=%q", response.Header().Get("Cache-Control"))
	}
	var status Status
	if err := json.Unmarshal(response.Body.Bytes(), &status); err != nil {
		t.Fatal(err)
	}
	if status.Status != "degraded" || status.Version != "v1.2.3" || status.Hostname != "sink.example" || len(status.Profiles) != 1 || status.Profiles[0] != "adshield" || len(status.Hosts) != 1 || status.Hosts[0] != "html-load.com" {
		t.Fatalf("status=%+v", status)
	}
	if !status.PKI.Ready || status.PKI.RootFingerprint != "AA:BB" || status.PKI.RootNotAfter != "2030-01-02T03:04:05Z" || status.PKI.IntermediateNotAfter != "2029-01-02T03:04:05Z" || status.PKI.LeafCacheEntries != 2 {
		t.Fatalf("pki=%+v", status.PKI)
	}
	if strings.Contains(response.Body.String(), secret) || strings.Contains(response.Body.String(), "PRIVATE KEY") || strings.Contains(response.Body.String(), "error") {
		t.Fatalf("status body exposed unexpected data: %s", response.Body.String())
	}
	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/status", nil))
	if head.Code != http.StatusOK || head.Body.Len() != 0 || head.Header().Get("Content-Type") != "application/json" || head.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("head status=%d body=%q headers=%v", head.Code, head.Body.String(), head.Header())
	}
}

func TestStatusPKIError(t *testing.T) {
	handler := New(func() error { return nil }, func() Status {
		return Status{PKI: PKIStatus{Ready: false, Error: "certificate expired"}}
	}, nil, metrics.New([]string{"adshield"}).Registry())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) || !strings.Contains(response.Body.String(), `"error":"certificate expired"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
