package sink

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"
	"time"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/metrics"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/profile"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/profiles"
)

func bundledRouter(t *testing.T) *profile.Router {
	t.Helper()
	items, err := profile.Load("")
	if err != nil {
		t.Fatal(err)
	}
	router, err := profile.NewRouter(items, []string{"adshield"})
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func TestHandler(t *testing.T) {
	handler := NewHandler(bundledRouter(t), []string{"html-load.com"}, metrics.New([]string{"adshield"}, false, 100, 200), nil)
	body, err := profiles.FS.ReadFile("adshield/loader.min.js")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, host, method, path string
		status                   int
		body                     bool
	}{
		{"unknown host", "other.example", "GET", "/loader.min.js", 421, false},
		{"unknown path", "html-load.com", "GET", "/other", 404, false},
		{"method", "html-load.com", "POST", "/loader.min.js", 405, false},
		{"head", "html-load.com:443", "HEAD", "/loader.min.js", 200, false},
		{"options", "html-load.com", "OPTIONS", "/loader.min.js", 204, false},
		{"body", "HTML-LOAD.COM.", "GET", "/loader.min.js", 200, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, "https://example"+test.path, nil)
			request.Host = test.host
			handler.ServeHTTP(recorder, request)
			if recorder.Code != test.status {
				t.Fatalf("got %d", recorder.Code)
			}
			if recorder.Header().Get("Alt-Svc") != "" {
				t.Fatal("Alt-Svc present")
			}
			if test.method == "OPTIONS" && recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
				t.Fatal("missing cors")
			}
			if test.method == "POST" && recorder.Header().Get("Allow") == "" {
				t.Fatal("missing allow")
			}
			if test.body && string(recorder.Body.Bytes()) != string(body) {
				t.Fatal("wrong bundled body")
			}
			if !test.body && recorder.Body.Len() != 0 {
				t.Fatal("unexpected body")
			}
		})
	}
}

func TestHandlerSiteRequests(t *testing.T) {
	m := metrics.New([]string{"adshield"}, true, 100, 200)
	handler := NewHandler(bundledRouter(t), []string{"html-load.com"}, m, nil)
	matched := httptest.NewRequest(http.MethodGet, "https://example/loader.min.js", nil)
	matched.Host = "html-load.com"
	matched.Header.Set("Referer", "https://www.example.com/path")
	handler.ServeHTTP(httptest.NewRecorder(), matched)
	unmatched := httptest.NewRequest(http.MethodGet, "https://example/other", nil)
	unmatched.Host = "html-load.com"
	unmatched.Header.Set("Referer", "https://other.example.com/path")
	handler.ServeHTTP(httptest.NewRecorder(), unmatched)
	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, family := range families {
		if family.GetName() != "ars_site_requests_total" {
			continue
		}
		if len(family.Metric) != 1 || family.Metric[0].Label[0].GetValue() != "www.example.com" {
			t.Fatalf("site metrics=%+v", family.Metric)
		}
		return
	}
	t.Fatal("site metric missing")
}

func TestHandlerUpstreamRequests(t *testing.T) {
	m := metrics.New([]string{"adshield"}, false, 100, 1)
	handler := NewHandler(bundledRouter(t), []string{"html-load.com"}, m, nil)
	tests := []struct {
		host, method, path, result string
	}{
		{"HTML-LOAD.COM.:443", http.MethodGet, "/loader.min.js?query=value", "matched"},
		{"other.example", http.MethodGet, "/loader.min.js", "unknown_host"},
		{"html-load.com", http.MethodGet, "/other", "unknown_path"},
		{"html-load.com", http.MethodPost, "/loader.min.js", "method_not_allowed"},
	}
	for _, test := range tests {
		request := httptest.NewRequest(test.method, "https://example"+test.path, nil)
		request.Host = test.host
		handler.ServeHTTP(httptest.NewRecorder(), request)
	}
	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]struct{ host, path string }{
		"matched":            {"html-load.com", "/loader.min.js"},
		"unknown_host":       {"other", "other"},
		"unknown_path":       {"other", "other"},
		"method_not_allowed": {"html-load.com", "/loader.min.js"},
	}
	for _, family := range families {
		if family.GetName() != "ars_upstream_requests_total" {
			continue
		}
		if len(family.Metric) != 4 {
			t.Fatalf("upstream metrics=%+v", family.Metric)
		}
		for _, metric := range family.Metric {
			labels := map[string]string{}
			for _, label := range metric.Label {
				labels[label.GetName()] = label.GetValue()
			}
			want, ok := expected[labels["result"]]
			if !ok || labels["host"] != want.host || labels["path"] != want.path {
				t.Fatalf("upstream labels=%v", labels)
			}
			delete(expected, labels["result"])
		}
		if len(expected) != 0 {
			t.Fatalf("missing results=%v", expected)
		}
		return
	}
	t.Fatal("upstream metric missing")
}

type testCertSource struct{ certificate tls.Certificate }

func (s testCertSource) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return &s.certificate, nil
}

func TestTLS(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "html-load.com"}, DNSNames: []string{"html-load.com"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	server := newServer(t, NewHandler(bundledRouter(t), []string{"html-load.com"}, metrics.New([]string{"adshield"}, false, 100, 200), nil))
	server.TLS = TLSConfig([]string{"html-load.com"}, testCertSource{certificate: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}}, true, metrics.New([]string{"adshield"}, false, 100, 200))
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "html-load.com"}}
	client := &http.Client{Transport: transport}
	request, _ := http.NewRequest(http.MethodGet, server.URL+"/loader.min.js", nil)
	request.Host = "html-load.com"
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.ProtoMajor != 2 {
		t.Fatalf("protocol %s", response.Proto)
	}
	_, _ = io.ReadAll(response.Body)
	badTransport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "other.example"}}
	badClient := &http.Client{Transport: badTransport}
	if _, err := badClient.Get(server.URL); err == nil {
		t.Fatal("unknown sni succeeded")
	}
	server.Close()
	plainServer := newServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	plainServer.TLS = TLSConfig([]string{"html-load.com"}, testCertSource{certificate: tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}}, false, metrics.New([]string{"adshield"}, false, 100, 200))
	plainServer.StartTLS()
	defer plainServer.Close()
	plainClient := &http.Client{Transport: &http.Transport{ForceAttemptHTTP2: true, TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "html-load.com"}}}
	plainResponse, err := plainClient.Get(plainServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	plainResponse.Body.Close()
	if plainResponse.ProtoMajor != 1 {
		t.Fatalf("protocol %s", plainResponse.Proto)
	}
}

func newServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if errors.Is(err, syscall.EPERM) {
		t.Skip("loopback listeners are unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	return server
}
