+++
title = "Configuration"
description = "Configuration sources, keys, limits, and profile overrides."
weight = 4
+++

Configuration is loaded in this order: built-in defaults, YAML from `--config` or `ARS_CONFIG`, environment, then explicitly supplied flags. A flag left at its default does not override a previous source. The three PKI values must be supplied either as contents or as files, never both.

| Key | Default | Environment | Flag |
| --- | --- | --- | --- |
| configuration file | none | `ARS_CONFIG` | `--config` |
| `sink.addr` | `:443` | `ARS_SINK_ADDR` | `--sink-addr` |
| `sink.http2` | `true` | `ARS_SINK_HTTP2` | `--sink-http2` |
| `ops.addr` | `127.0.0.1:8443` | `ARS_OPS_ADDR` | `--ops-addr` |
| `hosts` | profile hosts | `ARS_HOSTS` | `--hosts` |
| `profiles` | `adshield` | `ARS_PROFILES` | `--profiles` |
| `profiles_dir` | empty | `ARS_PROFILES_DIR` | `--profiles-dir` |
| `hostname` | empty | `ARS_HOSTNAME` | `--hostname` |
| `log.level` | `info` | `ARS_LOG_LEVEL` | |
| `log.format` | `json` | `ARS_LOG_FORMAT` | |
| `pki.root_cert` | empty | `ARS_PKI_ROOT_CERT` | |
| `pki.intermediate_cert` | empty | `ARS_PKI_INTERMEDIATE_CERT` | |
| `pki.intermediate_key` | empty | `ARS_PKI_INTERMEDIATE_KEY` | |
| `pki.root_cert_file` | empty | `ARS_PKI_ROOT_CERT_FILE` | |
| `pki.intermediate_cert_file` | empty | `ARS_PKI_INTERMEDIATE_CERT_FILE` | |
| `pki.intermediate_key_file` | empty | `ARS_PKI_INTERMEDIATE_KEY_FILE` | |

`hosts` and `profiles` are comma-separated in environment variables and flags. When `hosts` is unset, the service derives it from enabled profiles. Every configured host must be provided by an enabled profile. `hostname` is an optional public name shown on `/install`.

## Limits

| Key | Default | Environment | Flag |
| --- | --- | --- | --- |
| `limits.max_header_bytes` | `16384` | `ARS_LIMITS_MAX_HEADER_BYTES` | — |
| `limits.read_header_timeout` | `5s` | `ARS_LIMITS_READ_HEADER_TIMEOUT` | — |
| `limits.idle_timeout` | `60s` | `ARS_LIMITS_IDLE_TIMEOUT` | — |
| `limits.shutdown_timeout` | `10s` | `ARS_LIMITS_SHUTDOWN_TIMEOUT` | — |
| `limits.shutdown_delay` | `5s` | `ARS_LIMITS_SHUTDOWN_DELAY` | — |
| `limits.cert_cache_size` | `256` | `ARS_LIMITS_CERT_CACHE_SIZE` | — |

All timeout values must be positive except `shutdown_delay`, which may be zero. `max_header_bytes` and `cert_cache_size` must be positive.

## Profiles

Profiles define only the responses that the sink may serve. Bundled profiles are embedded in the binary. `profiles_dir` adds or replaces profiles by name: each discovered `profile.yaml` is loaded, and an operator profile with the same name replaces the bundled one.

```yaml
version: 1
name: example
routes:
  - hosts: [example.com]
    path: /loader.js
    methods: [GET, HEAD]
    status: 200
    content_type: "text/javascript; charset=utf-8"
    cache_control: no-store
    cors_origin: "*"
    body_file: loader.js
```

Each profile needs a non-empty `name`, `version: 1`, and at least one route. Routes require `hosts`, an absolute `path`, supported methods (`GET`, `HEAD`, `POST`, or `OPTIONS`), a status from `100` through `599`, and a relative `body_file` in the profile directory. Response bodies are limited to 1 MiB.
