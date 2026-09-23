package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/config"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/enroll"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/metrics"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/ops"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/pki"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/sink"
)

var version = "dev"

type certSource struct {
	issuer  *pki.Issuer
	metrics *metrics.Metrics
}

func (c certSource) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cert, err := c.issuer.GetCertificate(hello)
	c.metrics.SetCertCacheEntries(float64(c.issuer.CacheLen()))
	return cert, err
}

func loadIssuer(cfg config.Config) (*pki.Issuer, error) {
	root, err := material(cfg.PKI.RootCert, cfg.PKI.RootCertFile)
	if err != nil {
		return nil, err
	}
	intermediate, err := material(cfg.PKI.IntermediateCert, cfg.PKI.IntermediateCertFile)
	if err != nil {
		return nil, err
	}
	key, err := material(cfg.PKI.IntermediateKey, cfg.PKI.IntermediateKeyFile)
	if err != nil {
		return nil, err
	}
	return pki.Load(root, intermediate, key, pki.IssuerOptions{Hosts: cfg.Hosts, CacheSize: cfg.Limits.CertCacheSize})
}

func material(value, file string) ([]byte, error) {
	if value != "" {
		return []byte(value), nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("read pki material: %w", err)
	}
	return data, nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sink serve|config validate|pki init|version")
	}
	switch args[0] {
	case "version":
		fmt.Println(version)
		return nil
	case "config":
		if len(args) < 2 || args[1] != "validate" {
			return errors.New("usage: sink config validate")
		}
		cfg, err := config.Load(args[2:], os.Environ())
		if err != nil {
			return err
		}
		out, err := config.RedactedYAML(cfg)
		if err != nil {
			return err
		}
		fmt.Print(string(out))
		return nil
	case "pki":
		if len(args) < 2 || args[1] != "init" {
			return errors.New("usage: sink pki init")
		}
		return pki.RunInit(args[2:], os.Stdout)
	case "serve":
		return serve(args[1:])
	default:
		return errors.New("usage: sink serve|config validate|pki init|version")
	}
}

func serve(args []string) error {
	cfg, err := config.Load(args, os.Environ())
	if err != nil {
		return err
	}
	if err := cfg.RequirePKI(); err != nil {
		return err
	}
	level := new(slog.LevelVar)
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		return err
	}
	var handler slog.Handler
	if cfg.Log.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	logger := slog.New(handler)
	issuer, err := loadIssuer(cfg)
	if err != nil {
		return err
	}
	m := metrics.New(cfg.Profiles)
	m.SetIssuerExpiry(float64(issuer.ExpiresAt().Unix()))
	certs := certSource{issuer: issuer, metrics: m}
	ready := issuer.Ready
	enrollment := enroll.Handler(issuer, cfg.Enrollment.Host)
	logger.Info("issuer loaded", "root_fingerprint", pki.Fingerprint(issuer.Root()), "expires", issuer.ExpiresAt())
	routes, err := cfg.Routes()
	if err != nil {
		return err
	}
	sinkHandler := sink.NewHandler(routes, cfg.Hosts, m, logger)
	tlsConfig := sink.TLSConfig(cfg.Hosts, certs, cfg.Sink.HTTP2, m)
	sinkListener, err := net.Listen("tcp", cfg.Sink.Addr)
	if err != nil {
		return err
	}
	opsListener, err := net.Listen("tcp", cfg.Ops.Addr)
	if err != nil {
		sinkListener.Close()
		return err
	}
	sinkServer := &http.Server{Handler: sinkHandler, TLSConfig: tlsConfig, MaxHeaderBytes: cfg.Limits.MaxHeaderBytes, ReadHeaderTimeout: cfg.Limits.ReadHeaderTimeout, IdleTimeout: cfg.Limits.IdleTimeout}
	opsServer := &http.Server{Handler: ops.New(ready, enrollment, m.Registry()), MaxHeaderBytes: cfg.Limits.MaxHeaderBytes, ReadHeaderTimeout: cfg.Limits.ReadHeaderTimeout, IdleTimeout: cfg.Limits.IdleTimeout}
	errCh := make(chan error, 2)
	go func() { errCh <- sinkServer.Serve(tls.NewListener(sinkListener, tlsConfig)) }()
	go func() { errCh <- opsServer.Serve(opsListener) }()
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-signalCh:
		logger.Info("shutdown", "signal", sig.String())
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Limits.ShutdownTimeout)
	defer cancel()
	if err := sinkServer.Shutdown(ctx); err != nil {
		return err
	}
	if err := opsServer.Shutdown(ctx); err != nil {
		return err
	}
	return nil
}
