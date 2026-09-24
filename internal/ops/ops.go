package ops

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Status struct {
	Status       string    `json:"status"`
	Version      string    `json:"version"`
	Hostname     string    `json:"hostname"`
	Profiles     []string  `json:"profiles"`
	Hosts        []string  `json:"hosts"`
	SkippedHosts []string  `json:"skipped_hosts"`
	PKI          PKIStatus `json:"pki"`
}

type PKIStatus struct {
	Ready                bool   `json:"ready"`
	Error                string `json:"error,omitempty"`
	RootFingerprint      string `json:"root_fingerprint"`
	RootNotAfter         string `json:"root_not_after"`
	IntermediateNotAfter string `json:"intermediate_not_after"`
	LeafCacheEntries     int    `json:"leaf_cache_entries"`
}

func New(ready func() error, status func() Status, extra http.Handler, registry *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = writer.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(writer http.ResponseWriter, request *http.Request) {
		if ready != nil && ready() != nil {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = writer.Write([]byte("ok"))
	})
	mux.HandleFunc("/status", func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			writer.Header().Set("Allow", "GET, HEAD")
			writer.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		response := Status{}
		if status != nil {
			response = status()
		}
		response.Status = "ok"
		if ready != nil && ready() != nil {
			response.Status = "degraded"
		}
		body, err := json.Marshal(response)
		if err != nil {
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		writer.WriteHeader(http.StatusOK)
		if request.Method == http.MethodGet {
			_, _ = writer.Write(body)
		}
	})
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	if extra != nil {
		mux.Handle("/install", extra)
		mux.Handle("/ca.crt", extra)
		mux.Handle("/ca.pem", extra)
		mux.Handle("/ca-chain.pem", extra)
		mux.Handle("/fingerprint", extra)
	}
	return mux
}
