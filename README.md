# Adblock Recovery Sink

Serves harmless replacements for known anti-adblock loader resources, so pages stay usable while network-level ad and tracker blocking remains active. Works for desktop, phone and tablet browsers without extensions.

The sink sits behind DNS rewrites that you manage on your LAN resolver (AdGuard Home, NextDNS, or similar). The application never changes DNS.

## Status

Release candidates. The substitution approach is proven in [docs/compatibility.md](docs/compatibility.md) and the service, PKI and chart are validated in [docs/validation.md](docs/validation.md).

## Generate the CA

Run `pki init` once, offline. It writes `root.crt`, `root.key`, `intermediate.crt` and `intermediate.key` (mode `0600`), with root and intermediate both name-constrained to the given hosts, prints the root SHA-256 fingerprint, and refuses to overwrite existing files. It never talks to a cluster or creates Kubernetes objects; getting the files into your secret store is up to you.

With the release binary:

```sh
sink pki init --hosts html-load.com --out ./ars-pki
```

With the published image:

```sh
mkdir -p ars-pki
docker run --rm --network none --read-only --user "$(id -u):$(id -g)" \
  -v "$PWD/ars-pki:/out" \
  registry.gitlab.com/ptrck-sh/adblock-recovery-sink:0.1.0-rc.2 \
  pki init --hosts html-load.com --out /out
```

With Podman, use the same command and add `:Z` to the volume on SELinux hosts.

The running service needs only `root.crt`, `intermediate.crt` and `intermediate.key`. Store `root.key` offline; it is only needed to issue a new intermediate.

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
| `hostname` | `ARS_HOSTNAME` | `--hostname` |
| `log.level`, `log.format` | `ARS_LOG_LEVEL`, `ARS_LOG_FORMAT` | |
| `pki.root_cert`, `pki.intermediate_cert`, `pki.intermediate_key` | `ARS_PKI_ROOT_CERT`, `ARS_PKI_INTERMEDIATE_CERT`, `ARS_PKI_INTERMEDIATE_KEY` | |
| `pki.root_cert_file`, `pki.intermediate_cert_file`, `pki.intermediate_key_file` | `ARS_PKI_ROOT_CERT_FILE`, `ARS_PKI_INTERMEDIATE_CERT_FILE`, `ARS_PKI_INTERMEDIATE_KEY_FILE` | |
| `limits.max_header_bytes`, `limits.read_header_timeout`, `limits.idle_timeout`, `limits.shutdown_timeout`, `limits.shutdown_delay`, `limits.cert_cache_size` | matching `ARS_LIMITS_*` names | |

`hostname` is the optional public hostname of this instance, displayed on the install page.

The sink serves intercepted hosts over TLS on `:443`. The ops listener serves plain HTTP on `127.0.0.1:8443` by default: `/healthz`, `/readyz`, `/metrics`, `/status` and the enrollment pages. Binding 443 without root needs the namespaced sysctl `net.ipv4.ip_unprivileged_port_start=443` (for example `docker run --sysctl net.ipv4.ip_unprivileged_port_start=443`) or `CAP_NET_BIND_SERVICE`.

## Prior art

- [tinyShield](https://github.com/FilteringDev/tinyShield)

## License

MIT
