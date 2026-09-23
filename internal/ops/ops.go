package ops

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New(ready func() error, extra http.Handler, registry *prometheus.Registry) http.Handler {
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
