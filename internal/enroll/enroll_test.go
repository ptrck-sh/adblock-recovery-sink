package enroll_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/enroll"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/pki"
)

func TestHandler(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if err := pki.Init(pki.InitOptions{Hosts: []string{"allowed.example"}, OutDir: directory, Now: func() time.Time { return now }}); err != nil {
		t.Fatal(err)
	}
	root, err := os.ReadFile(filepath.Join(directory, "root.crt"))
	if err != nil {
		t.Fatal(err)
	}
	intermediate, err := os.ReadFile(filepath.Join(directory, "intermediate.crt"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(filepath.Join(directory, "intermediate.key"))
	if err != nil {
		t.Fatal(err)
	}
	issuer, err := pki.Load(root, intermediate, key, pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	handler := enroll.Handler(issuer, "enroll.example")
	checks := []struct {
		path        string
		contentType string
		contains    string
	}{
		{"/ca.crt", "application/x-x509-ca-cert", ""},
		{"/ca.pem", "application/x-pem-file", "BEGIN CERTIFICATE"},
		{"/ca-chain.pem", "application/x-pem-file", "BEGIN CERTIFICATE"},
		{"/fingerprint", "text/plain", pki.Fingerprint(issuer.Root())},
		{"/install", "text/html", pki.Fingerprint(issuer.Root())},
	}
	for _, check := range checks {
		request := httptest.NewRequest(http.MethodGet, check.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s returned %d", check.path, response.Code)
		}
		if !strings.HasPrefix(response.Header().Get("Content-Type"), check.contentType) {
			t.Fatalf("%s content type is %q", check.path, response.Header().Get("Content-Type"))
		}
		if !strings.Contains(response.Body.String(), check.contains) {
			t.Fatalf("%s body is missing %q", check.path, check.contains)
		}
		for _, header := range []string{"X-Content-Type-Options", "Cache-Control", "Referrer-Policy"} {
			if response.Header().Get(header) == "" {
				t.Fatalf("%s is missing %s", check.path, header)
			}
		}
	}
	install := httptest.NewRecorder()
	handler.ServeHTTP(install, httptest.NewRequest(http.MethodGet, "/install", nil))
	if install.Header().Get("Content-Security-Policy") != "default-src 'none'; style-src 'unsafe-inline'; img-src data:" {
		t.Fatal("install page is missing its content security policy")
	}
	if strings.Contains(strings.ToLower(install.Body.String()), "http") {
		t.Fatal("install page has an external URL")
	}
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatal("unknown endpoint did not return 404")
	}
	method := httptest.NewRecorder()
	handler.ServeHTTP(method, httptest.NewRequest(http.MethodPost, "/ca.pem", nil))
	if method.Code != http.StatusMethodNotAllowed {
		t.Fatal("POST did not return 405")
	}
}
