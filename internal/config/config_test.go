package config

import (
	"os"
	"path/filepath"
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
