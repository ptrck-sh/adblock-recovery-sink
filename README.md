# Adblock Recovery Sink

Serves harmless replacements for known anti-adblock loader resources, so pages stay usable while network-level ad and tracker blocking remains active. Works for desktop, phone and tablet browsers without extensions.

The sink sits behind DNS rewrites that you manage on your LAN resolver (AdGuard Home, NextDNS, or similar). The application never changes DNS.

## Status

Scaffold only. The substitution approach is proven in [docs/compatibility.md](docs/compatibility.md); the service itself is not implemented yet.

## Trust model

Run your own instance and never enroll devices in someone else's. Installing an instance's root certificate lets its operator impersonate any website to your devices, and the sink delivers JavaScript that runs inside the pages you visit. Only trust a root you generated yourself.

## Configuration

Configuration is loaded in this order: built-in defaults, YAML from `--config` or `ARS_CONFIG`, environment, then explicitly supplied flags. Unchanged flag defaults do not override other sources.

| Key | Environment | Flag |
| --- | --- | --- |
| `sink.addr` | `ARS_SINK_ADDR` | `--sink-addr` |
| `sink.http2` | `ARS_SINK_HTTP2` | `--sink-http2` |
| `ops.addr` | `ARS_OPS_ADDR` | `--ops-addr` |
| `hosts` | `ARS_HOSTS` | `--hosts` |
| `profiles` | `ARS_PROFILES` | `--profiles` |
| `profiles_dir` | `ARS_PROFILES_DIR` | `--profiles-dir` |
| `enrollment.host` | `ARS_ENROLLMENT_HOST` | `--enrollment-host` |
| `log.level`, `log.format` | `ARS_LOG_LEVEL`, `ARS_LOG_FORMAT` | |
| `pki.root_cert`, `pki.intermediate_cert`, `pki.intermediate_key` | `ARS_PKI_ROOT_CERT`, `ARS_PKI_INTERMEDIATE_CERT`, `ARS_PKI_INTERMEDIATE_KEY` | |
| `pki.root_cert_file`, `pki.intermediate_cert_file`, `pki.intermediate_key_file` | `ARS_PKI_ROOT_CERT_FILE`, `ARS_PKI_INTERMEDIATE_CERT_FILE`, `ARS_PKI_INTERMEDIATE_KEY_FILE` | |
| `limits.max_header_bytes`, `limits.read_header_timeout`, `limits.idle_timeout`, `limits.shutdown_timeout`, `limits.cert_cache_size` | matching `ARS_LIMITS_*` names | |

## Prior art

- [tinyShield](https://github.com/FilteringDev/tinyShield)

## License

MIT
