package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	hosts := flag.String("hosts", "html-load.com", "")
	sinkAddr := flag.String("sink", "127.0.0.1:8443", "")
	proxyAddr := flag.String("proxy", "127.0.0.1:3128", "")
	caOut := flag.String("ca", "root.pem", "")
	profile := flag.String("profile", "profiles/adshield/loader.min.js", "")
	flag.Parse()

	stub, err := os.ReadFile(*profile)
	if err != nil {
		log.Fatal(err)
	}

	allowed := map[string]bool{}
	for _, h := range strings.Split(*hosts, ",") {
		allowed[strings.ToLower(strings.TrimSpace(h))] = true
	}

	rootKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "ars test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
		MaxPathLenZero:        true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &rootKey.PublicKey, rootKey)
	if err != nil {
		log.Fatal(err)
	}
	root, _ := x509.ParseCertificate(rootDER)
	if err := os.WriteFile(*caOut, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: rootDER}), 0o644); err != nil {
		log.Fatal(err)
	}

	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			name := strings.ToLower(hello.ServerName)
			if !allowed[name] {
				return nil, errors.New("unknown sni")
			}
			key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
			tmpl := &x509.Certificate{
				SerialNumber: serial,
				Subject:      pkix.Name{CommonName: name},
				DNSNames:     []string{name},
				NotBefore:    time.Now().Add(-time.Hour),
				NotAfter:     time.Now().Add(12 * time.Hour),
				KeyUsage:     x509.KeyUsageDigitalSignature,
				ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			}
			der, err := x509.CreateCertificate(rand.Reader, tmpl, root, &key.PublicKey, rootKey)
			if err != nil {
				return nil, err
			}
			return &tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, nil
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/loader.min.js", func(w http.ResponseWriter, r *http.Request) {
		host, _, _ := net.SplitHostPort(r.Host)
		if host == "" {
			host = r.Host
		}
		if !allowed[strings.ToLower(host)] || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		log.Printf("sink hit %s %s proto=%s", r.Host, r.URL.Path, r.Proto)
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Write(stub)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("sink 404 %s %s", r.Host, r.URL.Path)
		http.NotFound(w, r)
	})
	sink := &http.Server{Addr: *sinkAddr, Handler: mux, TLSConfig: cfg, ReadHeaderTimeout: 5 * time.Second}
	go func() { log.Fatal(sink.ListenAndServeTLS("", "")) }()

	proxy := &http.Server{Addr: *proxyAddr, ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			r.RequestURI = ""
			resp, err := http.DefaultTransport.RoundTrip(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			defer resp.Body.Close()
			for k, v := range resp.Header {
				w.Header()[k] = v
			}
			w.WriteHeader(resp.StatusCode)
			io.Copy(w, resp.Body)
			return
		}
		host, port, _ := net.SplitHostPort(r.Host)
		target := r.Host
		if allowed[strings.ToLower(host)] && port == "443" {
			target = *sinkAddr
		}
		up, err := net.DialTimeout("tcp", target, 10*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		hj, _ := w.(http.Hijacker)
		down, _, err := hj.Hijack()
		if err != nil {
			up.Close()
			return
		}
		down.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		go func() { io.Copy(up, down); up.Close() }()
		io.Copy(down, up)
		down.Close()
	})}
	log.Printf("sink %s proxy %s hosts %s", *sinkAddr, *proxyAddr, *hosts)
	log.Fatal(proxy.ListenAndServe())
}
