package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("sink:\n  http2: false\n  addr: 127.0.0.1:9443\nhosts: [HTML-LOAD.COM.]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		args, env []string
		http2     bool
		addr      string
	}{
		{"file false", []string{"--config", file}, nil, false, "127.0.0.1:9443"},
		{"env wins", []string{"--config", file}, []string{"ARS_SINK_HTTP2=true", "ARS_SINK_ADDR=127.0.0.1:9555"}, true, "127.0.0.1:9555"},
		{"flag wins", []string{"--config", file, "--sink-http2=false", "--sink-addr", "127.0.0.1:9666"}, []string{"ARS_SINK_HTTP2=true", "ARS_SINK_ADDR=127.0.0.1:9555"}, false, "127.0.0.1:9666"},
		{"unchanged flags", []string{"--config", file}, nil, false, "127.0.0.1:9443"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Load(test.args, test.env)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Sink.HTTP2 != test.http2 || cfg.Sink.Addr != test.addr {
				t.Fatalf("got http2=%v addr=%s", cfg.Sink.HTTP2, cfg.Sink.Addr)
			}
			if len(cfg.Hosts) != 1 || cfg.Hosts[0] != "html-load.com" {
				t.Fatalf("hosts=%v", cfg.Hosts)
			}
		})
	}
}

func TestListsAndHostValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     []string
		wantErr bool
		hosts   []string
	}{
		{"comma lists", []string{"ARS_HOSTS=HTML-LOAD.COM.", "ARS_PROFILES=adshield"}, false, []string{"html-load.com"}},
		{"unknown host", []string{"ARS_HOSTS=badexample.com"}, true, nil},
		{"non ascii", []string{"ARS_HOSTS=éxample.com"}, true, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Load(nil, test.env)
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v", err)
			}
			if !test.wantErr && len(cfg.Hosts) != len(test.hosts) {
				t.Fatalf("hosts=%v", cfg.Hosts)
			}
		})
	}
}

func TestHostname(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		env      []string
		hostname string
		wantErr  bool
	}{
		{"empty", nil, nil, "", false},
		{"environment", nil, []string{"ARS_HOSTNAME=sink.example"}, "sink.example", false},
		{"flag", []string{"--hostname", "sink.example"}, []string{"ARS_HOSTNAME=env.example"}, "sink.example", false},
		{"uppercase", nil, []string{"ARS_HOSTNAME=Sink.Example"}, "", true},
		{"trailing dot", nil, []string{"ARS_HOSTNAME=sink.example."}, "", true},
		{"invalid character", nil, []string{"ARS_HOSTNAME=sink_example"}, "", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg, err := Load(test.args, test.env)
			if (err != nil) != test.wantErr {
				t.Fatalf("err=%v", err)
			}
			if !test.wantErr && cfg.Hostname != test.hostname {
				t.Fatalf("hostname=%q", cfg.Hostname)
			}
		})
	}
}

func TestToast(t *testing.T) {
	defaultConfig, err := Load(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaultConfig.Toast.Enabled || defaultConfig.Toast.Details {
		t.Fatalf("toast=%+v", defaultConfig.Toast)
	}
	defaultRouter, err := defaultConfig.Routes()
	if err != nil {
		t.Fatal(err)
	}
	defaultRoute, _ := defaultRouter.Match("html-load.com", "GET", "/loader.min.js")
	if strings.Contains(string(defaultRoute.Body), "Adblock recovery neutralized") {
		t.Fatal("default route contains toast")
	}
	cfg, err := Load(nil, []string{"ARS_TOAST_ENABLED=true", "ARS_TOAST_DETAILS=true"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Toast.Enabled || !cfg.Toast.Details {
		t.Fatalf("toast=%+v", cfg.Toast)
	}
	router, err := cfg.Routes()
	if err != nil {
		t.Fatal(err)
	}
	route, result := router.Match("html-load.com", "GET", "/loader.min.js")
	if string(result) != "matched" || !strings.Contains(string(route.Body), `({"details":true});`) {
		t.Fatalf("route=%+v result=%s", route, result)
	}
}

func TestMetrics(t *testing.T) {
	defaults, err := Load(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Metrics.Sites.Enabled || defaults.Metrics.Sites.Max != 100 || defaults.Metrics.Upstreams.Max != 200 {
		t.Fatalf("metrics=%+v", defaults.Metrics)
	}
	cfg, err := Load(nil, []string{"ARS_METRICS_SITES_ENABLED=true", "ARS_METRICS_SITES_MAX=10", "ARS_METRICS_UPSTREAMS_MAX=20"})
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Metrics.Sites.Enabled || cfg.Metrics.Sites.Max != 10 || cfg.Metrics.Upstreams.Max != 20 {
		t.Fatalf("metrics=%+v", cfg.Metrics)
	}
	for _, environment := range [][]string{{"ARS_METRICS_SITES_MAX=0"}, {"ARS_METRICS_UPSTREAMS_MAX=0"}} {
		if _, err := Load(nil, environment); err == nil {
			t.Fatal("expected metrics validation error")
		}
	}
}
