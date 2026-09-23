package config

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/posflag"
	koanf "github.com/knadh/koanf/v2"
	"github.com/spf13/pflag"
	"gitlab.com/ptrck-sh/adblock-recovery-sink/internal/profile"
)

type Config struct {
	Sink struct {
		Addr  string
		HTTP2 bool
	}
	Ops         struct{ Addr string }
	Hosts       []string
	Profiles    []string
	ProfilesDir string
	Enrollment  struct{ Host string }
	Log         struct {
		Level  string
		Format string
	}
	PKI struct {
		RootCert             string
		IntermediateCert     string
		IntermediateKey      string
		RootCertFile         string
		IntermediateCertFile string
		IntermediateKeyFile  string
	}
	Limits struct {
		MaxHeaderBytes    int
		ReadHeaderTimeout time.Duration
		IdleTimeout       time.Duration
		ShutdownTimeout   time.Duration
		ShutdownDelay     time.Duration
		CertCacheSize     int
	}
	items map[string]profile.Profile
}

func Load(args []string, environ []string) (Config, error) {
	if environ == nil {
		environ = os.Environ()
	}
	flags := pflag.NewFlagSet("sink", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.SetInterspersed(false)
	flags.String("config", "", "")
	flags.String("sink-addr", ":8443", "")
	flags.Bool("sink-http2", true, "")
	flags.String("ops-addr", "127.0.0.1:8080", "")
	flags.String("hosts", "", "")
	flags.String("profiles", "adshield", "")
	flags.String("profiles-dir", "", "")
	flags.String("enrollment-host", "", "")
	if err := flags.Parse(args); err != nil {
		return Config{}, errors.New("invalid command line")
	}
	configPath, _ := flags.GetString("config")
	if !flags.Changed("config") {
		configPath = envValue(environ, "ARS_CONFIG")
	}
	ko := koanf.New(".")
	if err := ko.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return Config{}, err
	}
	if configPath != "" {
		if err := ko.Load(file.Provider(configPath), yaml.Parser()); err != nil {
			return Config{}, err
		}
	}
	if err := ko.Load(env.Provider(".", env.Opt{Prefix: "ARS_", EnvironFunc: func() []string { return environ }, TransformFunc: envTransform}), nil); err != nil {
		return Config{}, err
	}
	if err := ko.Load(posflag.ProviderWithFlag(flags, ".", ko, flagTransform), nil); err != nil {
		return Config{}, err
	}
	cfg := fromKoanf(ko)
	items, err := profile.Load(cfg.ProfilesDir)
	if err != nil {
		return Config{}, err
	}
	cfg.items = items
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func defaults() map[string]interface{} {
	return map[string]interface{}{
		"sink":  map[string]interface{}{"addr": ":8443", "http2": true},
		"ops":   map[string]interface{}{"addr": "127.0.0.1:8080"},
		"hosts": []string{}, "profiles": []string{"adshield"}, "profiles_dir": "",
		"enrollment": map[string]interface{}{"host": ""},
		"log":        map[string]interface{}{"level": "info", "format": "json"},
		"pki":        map[string]interface{}{"root_cert": "", "intermediate_cert": "", "intermediate_key": "", "root_cert_file": "", "intermediate_cert_file": "", "intermediate_key_file": ""},
		"limits":     map[string]interface{}{"max_header_bytes": 16384, "read_header_timeout": "5s", "idle_timeout": "60s", "shutdown_timeout": "10s", "shutdown_delay": "5s", "cert_cache_size": 256},
	}
}

func envTransform(key, value string) (string, any) {
	keys := map[string]string{
		"ARS_SINK_ADDR": "sink.addr", "ARS_SINK_HTTP2": "sink.http2", "ARS_OPS_ADDR": "ops.addr", "ARS_HOSTS": "hosts", "ARS_PROFILES": "profiles", "ARS_PROFILES_DIR": "profiles_dir", "ARS_ENROLLMENT_HOST": "enrollment.host", "ARS_LOG_LEVEL": "log.level", "ARS_LOG_FORMAT": "log.format", "ARS_PKI_ROOT_CERT": "pki.root_cert", "ARS_PKI_INTERMEDIATE_CERT": "pki.intermediate_cert", "ARS_PKI_INTERMEDIATE_KEY": "pki.intermediate_key", "ARS_PKI_ROOT_CERT_FILE": "pki.root_cert_file", "ARS_PKI_INTERMEDIATE_CERT_FILE": "pki.intermediate_cert_file", "ARS_PKI_INTERMEDIATE_KEY_FILE": "pki.intermediate_key_file", "ARS_LIMITS_MAX_HEADER_BYTES": "limits.max_header_bytes", "ARS_LIMITS_READ_HEADER_TIMEOUT": "limits.read_header_timeout", "ARS_LIMITS_IDLE_TIMEOUT": "limits.idle_timeout", "ARS_LIMITS_SHUTDOWN_TIMEOUT": "limits.shutdown_timeout", "ARS_LIMITS_SHUTDOWN_DELAY": "limits.shutdown_delay", "ARS_LIMITS_CERT_CACHE_SIZE": "limits.cert_cache_size",
	}
	result := keys[key]
	if result == "" {
		return "", nil
	}
	if result == "hosts" || result == "profiles" {
		return result, commaList(value)
	}
	return result, value
}

func flagTransform(flag *pflag.Flag) (string, interface{}) {
	keys := map[string]string{"sink-addr": "sink.addr", "sink-http2": "sink.http2", "ops-addr": "ops.addr", "hosts": "hosts", "profiles": "profiles", "profiles-dir": "profiles_dir", "enrollment-host": "enrollment.host"}
	key := keys[flag.Name]
	if key == "" {
		return "", nil
	}
	if key == "hosts" || key == "profiles" {
		return key, commaList(flag.Value.String())
	}
	if flag.Value.Type() == "bool" {
		value, _ := strconv.ParseBool(flag.Value.String())
		return key, value
	}
	return key, flag.Value.String()
}

func fromKoanf(ko *koanf.Koanf) Config {
	var cfg Config
	cfg.Sink.Addr, cfg.Sink.HTTP2 = ko.String("sink.addr"), ko.Bool("sink.http2")
	cfg.Ops.Addr, cfg.Hosts, cfg.Profiles, cfg.ProfilesDir = ko.String("ops.addr"), ko.Strings("hosts"), ko.Strings("profiles"), ko.String("profiles_dir")
	cfg.Enrollment.Host = ko.String("enrollment.host")
	cfg.Log.Level, cfg.Log.Format = ko.String("log.level"), ko.String("log.format")
	cfg.PKI.RootCert, cfg.PKI.IntermediateCert, cfg.PKI.IntermediateKey = ko.String("pki.root_cert"), ko.String("pki.intermediate_cert"), ko.String("pki.intermediate_key")
	cfg.PKI.RootCertFile, cfg.PKI.IntermediateCertFile, cfg.PKI.IntermediateKeyFile = ko.String("pki.root_cert_file"), ko.String("pki.intermediate_cert_file"), ko.String("pki.intermediate_key_file")
	cfg.Limits.MaxHeaderBytes, cfg.Limits.ReadHeaderTimeout, cfg.Limits.IdleTimeout, cfg.Limits.ShutdownTimeout, cfg.Limits.CertCacheSize = ko.Int("limits.max_header_bytes"), ko.Duration("limits.read_header_timeout"), ko.Duration("limits.idle_timeout"), ko.Duration("limits.shutdown_timeout"), ko.Int("limits.cert_cache_size")
	cfg.Limits.ShutdownDelay = ko.Duration("limits.shutdown_delay")
	return cfg
}

func (cfg *Config) Validate() error {
	if err := validAddr(cfg.Sink.Addr); err != nil {
		return fmt.Errorf("sink address: %w", err)
	}
	if err := validAddr(cfg.Ops.Addr); err != nil {
		return fmt.Errorf("ops address: %w", err)
	}
	if cfg.Log.Level != "debug" && cfg.Log.Level != "info" && cfg.Log.Level != "warn" && cfg.Log.Level != "error" {
		return errors.New("invalid log level")
	}
	if cfg.Log.Format != "json" && cfg.Log.Format != "text" {
		return errors.New("invalid log format")
	}
	if len(cfg.Profiles) == 0 {
		return errors.New("no enabled profiles")
	}
	for _, name := range cfg.Profiles {
		if _, ok := cfg.items[name]; !ok {
			return fmt.Errorf("unknown profile %q", name)
		}
	}
	if len(cfg.Hosts) == 0 {
		set := map[string]bool{}
		for _, name := range cfg.Profiles {
			for _, route := range cfg.items[name].Routes {
				for _, host := range route.Hosts {
					set[host] = true
				}
			}
		}
		for host := range set {
			cfg.Hosts = append(cfg.Hosts, host)
		}
		sort.Strings(cfg.Hosts)
	}
	if len(cfg.Hosts) == 0 {
		return errors.New("no hosts")
	}
	for i, host := range cfg.Hosts {
		normalized, err := profile.NormalizeHost(host)
		if err != nil {
			return err
		}
		cfg.Hosts[i] = normalized
		found := false
		for _, name := range cfg.Profiles {
			if profile.ProfileMatchesHost(cfg.items[name], normalized) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("host not provided by enabled profile")
		}
	}
	if cfg.Limits.MaxHeaderBytes <= 0 || cfg.Limits.ReadHeaderTimeout <= 0 || cfg.Limits.IdleTimeout <= 0 || cfg.Limits.ShutdownTimeout <= 0 || cfg.Limits.ShutdownDelay < 0 || cfg.Limits.CertCacheSize <= 0 {
		return errors.New("invalid limits")
	}
	if (cfg.PKI.RootCert != "" && cfg.PKI.RootCertFile != "") || (cfg.PKI.IntermediateCert != "" && cfg.PKI.IntermediateCertFile != "") || (cfg.PKI.IntermediateKey != "" && cfg.PKI.IntermediateKeyFile != "") {
		return errors.New("pki contents and file cannot both be set")
	}
	return nil
}

func (cfg Config) Routes() (*profile.Router, error) {
	return profile.NewRouter(cfg.items, cfg.Profiles)
}

func (cfg Config) RequirePKI() error {
	if (cfg.PKI.RootCert == "" && cfg.PKI.RootCertFile == "") || (cfg.PKI.IntermediateCert == "" && cfg.PKI.IntermediateCertFile == "") || (cfg.PKI.IntermediateKey == "" && cfg.PKI.IntermediateKeyFile == "") {
		return errors.New("pki material is required")
	}
	return nil
}

func RedactedYAML(cfg Config) ([]byte, error) {
	secret := func(value string) string {
		if value == "" {
			return "<unset>"
		}
		return "<set>"
	}
	return yaml.Parser().Marshal(map[string]interface{}{
		"sink": map[string]interface{}{"addr": cfg.Sink.Addr, "http2": cfg.Sink.HTTP2}, "ops": map[string]interface{}{"addr": cfg.Ops.Addr}, "hosts": cfg.Hosts, "profiles": cfg.Profiles, "profiles_dir": cfg.ProfilesDir, "enrollment": map[string]interface{}{"host": cfg.Enrollment.Host}, "log": map[string]interface{}{"level": cfg.Log.Level, "format": cfg.Log.Format},
		"pki":    map[string]interface{}{"root_cert": secret(cfg.PKI.RootCert), "intermediate_cert": secret(cfg.PKI.IntermediateCert), "intermediate_key": secret(cfg.PKI.IntermediateKey), "root_cert_file": secret(cfg.PKI.RootCertFile), "intermediate_cert_file": secret(cfg.PKI.IntermediateCertFile), "intermediate_key_file": secret(cfg.PKI.IntermediateKeyFile)},
		"limits": map[string]interface{}{"max_header_bytes": cfg.Limits.MaxHeaderBytes, "read_header_timeout": cfg.Limits.ReadHeaderTimeout.String(), "idle_timeout": cfg.Limits.IdleTimeout.String(), "shutdown_timeout": cfg.Limits.ShutdownTimeout.String(), "shutdown_delay": cfg.Limits.ShutdownDelay.String(), "cert_cache_size": cfg.Limits.CertCacheSize},
	})
}

func commaList(value string) []string {
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if item := strings.TrimSpace(part); item != "" {
			result = append(result, item)
		}
	}
	return result
}
func envValue(environ []string, key string) string {
	for _, item := range environ {
		if strings.HasPrefix(item, key+"=") {
			return strings.TrimPrefix(item, key+"=")
		}
	}
	return ""
}
func validAddr(value string) error { _, _, err := net.SplitHostPort(value); return err }
