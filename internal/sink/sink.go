package sink

import (
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/metrics"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/profile"
)

type CertSource interface {
	GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
}

func TLSConfig(hosts []string, source CertSource, http2 bool, metrics *metrics.Metrics) *tls.Config {
	allowed := hostSet(hosts)
	next := []string{"http/1.1"}
	if http2 {
		next = []string{"h2", "http/1.1"}
	}
	return &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: next, GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		host, err := profile.NormalizeHost(hello.ServerName)
		if err != nil || !allowedHost(allowed, host) {
			metrics.HandshakeFailure("unknown_sni")
			return nil, errors.New("unknown sni")
		}
		certificate, err := source.GetCertificate(hello)
		if err != nil {
			metrics.HandshakeFailure("other")
		}
		return certificate, err
	}}
}

func NewHandler(router *profile.Router, hosts []string, metrics *metrics.Metrics, logger *slog.Logger) http.Handler {
	return &handler{router: router, hosts: hostSet(hosts), metrics: metrics, logger: logger}
}

type handler struct {
	router  *profile.Router
	hosts   map[string]bool
	metrics *metrics.Metrics
	logger  *slog.Logger
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 0)
	io.Copy(io.Discard, request.Body)
	request.Body.Close()
	host := requestHost(request.Host)
	if !allowedHost(h.hosts, host) {
		h.respond(writer, request, host, "unknown", profile.UnknownHost, http.StatusMisdirectedRequest)
		return
	}
	route, result := h.router.Match(host, request.Method, request.URL.Path)
	if request.Method == http.MethodOptions && result == profile.MethodNotAllowed && route != nil && route.CORSOrigin != "" {
		h.setRouteHeaders(writer, route)
		writer.Header().Set("Allow", allow(route))
		h.respond(writer, request, host, route.Profile, profile.Matched, http.StatusNoContent)
		return
	}
	switch result {
	case profile.UnknownPath:
		h.respond(writer, request, host, "unknown", result, http.StatusNotFound)
	case profile.MethodNotAllowed:
		writer.Header().Set("Allow", allow(route))
		h.respond(writer, request, host, route.Profile, result, http.StatusMethodNotAllowed)
	case profile.Matched:
		h.setRouteHeaders(writer, route)
		h.metrics.Request(route.Profile, string(result))
		h.metrics.SiteRequest(request.Referer(), string(result))
		h.metrics.UpstreamRequest(host, request.URL.Path, string(result))
		writer.WriteHeader(route.Status)
		if request.Method != http.MethodHead {
			_, _ = writer.Write(route.Body)
		}
		h.log(route.Profile, result, route.Status)
	default:
		h.respond(writer, request, host, "unknown", profile.UnknownHost, http.StatusMisdirectedRequest)
	}
}

func (h *handler) setRouteHeaders(writer http.ResponseWriter, route *profile.Route) {
	if route.ContentType != "" {
		writer.Header().Set("Content-Type", route.ContentType)
	}
	if route.CacheControl != "" {
		writer.Header().Set("Cache-Control", route.CacheControl)
	}
	if route.CORSOrigin != "" {
		writer.Header().Set("Access-Control-Allow-Origin", route.CORSOrigin)
	}
	writer.Header().Set("X-Content-Type-Options", "nosniff")
}
func (h *handler) respond(writer http.ResponseWriter, request *http.Request, host, name string, result profile.Result, status int) {
	h.metrics.Request(name, string(result))
	h.metrics.SiteRequest(request.Referer(), string(result))
	h.metrics.UpstreamRequest(host, request.URL.Path, string(result))
	writer.WriteHeader(status)
	h.log(name, result, status)
}
func (h *handler) log(name string, result profile.Result, status int) {
	if h.logger != nil {
		h.logger.Debug("request", "profile", name, "result", result, "status", status)
	}
}
func hostSet(hosts []string) map[string]bool {
	set := map[string]bool{}
	for _, host := range hosts {
		normalized, err := profile.NormalizeHost(host)
		if err == nil {
			set[normalized] = true
		}
	}
	return set
}
func allowedHost(hosts map[string]bool, host string) bool {
	if hosts[host] {
		return true
	}
	for pattern := range hosts {
		if strings.HasPrefix(pattern, "*.") && strings.HasSuffix(host, "."+strings.TrimPrefix(pattern, "*.")) {
			return true
		}
	}
	return false
}
func requestHost(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		value = host
	}
	normalized, err := profile.NormalizeHost(value)
	if err != nil {
		return ""
	}
	return normalized
}
func allow(route *profile.Route) string {
	methods := append([]string(nil), route.Methods...)
	hasOptions := false
	for _, method := range methods {
		if method == http.MethodOptions {
			hasOptions = true
		}
	}
	if route.CORSOrigin != "" && !hasOptions {
		methods = append(methods, http.MethodOptions)
	}
	sort.Strings(methods)
	return strings.Join(methods, ", ")
}
