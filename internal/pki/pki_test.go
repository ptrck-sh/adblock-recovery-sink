package pki_test

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/pki"
)

func TestInitWritesOnlyConstrainedPEMFiles(t *testing.T) {
	directory := t.TempDir()
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	err := pki.Init(pki.InitOptions{
		Hosts:  []string{"Allowed.Example", "other.example"},
		OutDir: directory,
		Now:    func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"root.crt", "root.key", "intermediate.crt", "intermediate.key"} {
		info, err := os.Stat(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Fatalf("%s permissions are %o", name, info.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected exactly four files, got %d", len(entries))
	}
	rootPEM, err := os.ReadFile(filepath.Join(directory, "root.crt"))
	if err != nil {
		t.Fatal(err)
	}
	intermediatePEM, err := os.ReadFile(filepath.Join(directory, "intermediate.crt"))
	if err != nil {
		t.Fatal(err)
	}
	root := certificate(t, rootPEM)
	intermediate := certificate(t, intermediatePEM)
	for _, cert := range []*x509.Certificate{root, intermediate} {
		if !cert.PermittedDNSDomainsCritical || len(cert.PermittedDNSDomains) != 2 {
			t.Fatal("name constraints are missing or non-critical")
		}
	}
	if err := pki.Init(pki.InitOptions{Hosts: []string{"allowed.example"}, OutDir: directory}); err == nil {
		t.Fatal("Init overwrote existing files")
	}
}

func TestLoadValidationAndRestart(t *testing.T) {
	directory, now := initFiles(t, []string{"allowed.example"}, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	root, intermediate, key := readIssuerFiles(t, directory)
	issuer, err := pki.Load(root, intermediate, key, pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := pki.Load(root, intermediate, key, pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if pki.Fingerprint(issuer.Root()) != pki.Fingerprint(restarted.Root()) {
		t.Fatal("restart changed root identity")
	}
	if _, err := pki.Load(root, intermediate, rootKey(t, directory), pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now }}); err == nil {
		t.Fatal("accepted mismatched intermediate key")
	}
	otherDirectory, _ := initFiles(t, []string{"allowed.example"}, now)
	otherRoot, err := os.ReadFile(filepath.Join(otherDirectory, "root.crt"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pki.Load(otherRoot, intermediate, key, pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now }}); err == nil {
		t.Fatal("accepted intermediate from another root")
	}
	if _, err := pki.Load(root, intermediate, key, pki.IssuerOptions{Hosts: []string{"outside.example"}, Now: func() time.Time { return now }}); err == nil {
		t.Fatal("accepted host outside constraints")
	}
	if _, err := pki.Load([]byte("not pem"), intermediate, key, pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now }}); err == nil {
		t.Fatal("accepted garbage PEM")
	}
	if _, err := pki.Load(root, intermediate, key, pki.IssuerOptions{Hosts: []string{"allowed.example"}, Now: func() time.Time { return now.Add(11 * 365 * 24 * time.Hour) }}); err == nil {
		t.Fatal("accepted expired issuer")
	}
}

func TestFilterHosts(t *testing.T) {
	directory, _ := initFiles(t, []string{"allowed.example"}, time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC))
	root, intermediate, _ := readIssuerFiles(t, directory)
	allowed, skipped, err := pki.FilterHosts(root, intermediate, []string{"allowed.example", "outside.example"})
	if err != nil {
		t.Fatal(err)
	}
	if len(allowed) != 1 || allowed[0] != "allowed.example" || len(skipped) != 1 || skipped[0] != "outside.example" {
		t.Fatalf("allowed=%v skipped=%v", allowed, skipped)
	}
}

func TestIssuerCertificatesCacheRenewalAndReadiness(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	directory, _ := initFiles(t, []string{"a.example", "b.example", "c.example"}, now)
	root, intermediate, key := readIssuerFiles(t, directory)
	issuer, err := pki.Load(root, intermediate, key, pki.IssuerOptions{
		Hosts:        []string{"a.example", "b.example", "c.example"},
		CacheSize:    2,
		LeafValidity: 24 * time.Hour,
		Now:          func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	first := get(t, issuer, "a.example")
	if same := get(t, issuer, "a.example"); same != first {
		t.Fatal("cache miss for unchanged certificate")
	}
	verifyLeaf(t, first, issuer.Root(), "a.example", now)
	if first.Leaf.NotAfter.After(issuer.Intermediate().NotAfter) {
		t.Fatal("leaf outlives intermediate")
	}
	if _, err := issuer.GetCertificate(&tls.ClientHelloInfo{ServerName: "unknown.example"}); err == nil {
		t.Fatal("accepted unknown SNI")
	}
	get(t, issuer, "b.example")
	get(t, issuer, "a.example")
	get(t, issuer, "c.example")
	if issuer.CacheLen() != 2 {
		t.Fatal("cache is not bounded")
	}
	if fresh := get(t, issuer, "b.example"); fresh == nil {
		t.Fatal("LRU eviction did not issue certificate")
	}
	beforeRenewal := get(t, issuer, "a.example")
	now = now.Add(17 * time.Hour)
	renewed := get(t, issuer, "a.example")
	if renewed == beforeRenewal || string(renewed.Certificate[0]) == string(beforeRenewal.Certificate[0]) {
		t.Fatal("certificate was not renewed")
	}
	now = issuer.ExpiresAt().Add(time.Second)
	if err := issuer.Ready(); err == nil {
		t.Fatal("expired issuer is ready")
	}
}

func TestIssuerTLS(t *testing.T) {
	now := time.Now()
	directory, _ := initFiles(t, []string{"allowed.example"}, now)
	rootPEM, intermediatePEM, keyPEM := readIssuerFiles(t, directory)
	issuer, err := pki.Load(rootPEM, intermediatePEM, keyPEM, pki.IssuerOptions{Hosts: []string{"allowed.example"}})
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(issuer.Root())
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("local listener unavailable: %v", err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.Listener = listener
	server.TLS = &tls.Config{GetCertificate: issuer.GetCertificate, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	defer server.Close()
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "allowed.example", MinVersion: tls.VersionTLS12}}}
	response, err := client.Get(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	badClient := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, ServerName: "other.example", MinVersion: tls.VersionTLS12}}}
	if _, err := badClient.Get(server.URL); err == nil {
		t.Fatal("TLS accepted an unconfigured SNI")
	}
}

func initFiles(t *testing.T, hosts []string, now time.Time) (string, time.Time) {
	t.Helper()
	directory := t.TempDir()
	if err := pki.Init(pki.InitOptions{Hosts: hosts, OutDir: directory, Now: func() time.Time { return now }}); err != nil {
		t.Fatal(err)
	}
	return directory, now
}

func readIssuerFiles(t *testing.T, directory string) ([]byte, []byte, []byte) {
	t.Helper()
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
	return root, intermediate, key
}

func rootKey(t *testing.T, directory string) []byte {
	t.Helper()
	key, err := os.ReadFile(filepath.Join(directory, "root.key"))
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func certificate(t *testing.T, data []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(data)
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func get(t *testing.T, issuer *pki.Issuer, host string) *tls.Certificate {
	t.Helper()
	cert, err := issuer.GetCertificate(&tls.ClientHelloInfo{ServerName: host})
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func verifyLeaf(t *testing.T, certificate *tls.Certificate, root *x509.Certificate, host string, now time.Time) {
	t.Helper()
	pool := x509.NewCertPool()
	pool.AddCert(root)
	intermediates := x509.NewCertPool()
	intermediate, err := x509.ParseCertificate(certificate.Certificate[1])
	if err != nil {
		t.Fatal(err)
	}
	intermediates.AddCert(intermediate)
	if _, err := certificate.Leaf.Verify(x509.VerifyOptions{DNSName: host, Roots: pool, Intermediates: intermediates, CurrentTime: now}); err != nil {
		t.Fatal(err)
	}
}
