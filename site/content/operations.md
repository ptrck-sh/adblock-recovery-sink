+++
title = "Operations"
description = "Health, readiness, status, metrics, logging, and graceful shutdown."
weight = 9
+++

The operations listener is plain HTTP and binds to `127.0.0.1:8443` by default. Expose it deliberately if devices need enrollment access.

| Endpoint | Behaviour |
| --- | --- |
| `/healthz` | Returns `ok` while the process serves. |
| `/readyz` | Returns `ok` when the issuer is ready; returns `503` during drain or issuer failure. |
| `/status` | Returns JSON with `status`, `version`, `hostname`, `profiles`, `hosts`, and `pki`. The `pki` object contains `ready`, optional `error`, `root_fingerprint`, `root_not_after`, `intermediate_not_after`, and `leaf_cache_entries`. |
| `/metrics` | Prometheus metrics. |

The metrics registry exposes `ars_requests_total{profile,result}`, `ars_site_requests_total{site}`, `ars_upstream_requests_total{host,path,result}`, `ars_tls_handshake_failures_total{reason}`, `ars_cert_cache_entries`, `ars_issuer_expiry_timestamp_seconds`, and `ars_profile_info{profile,version}`. Request results are `matched`, `unknown_host`, `unknown_path`, or `method_not_allowed`; handshake reasons are `unknown_sni` or `other`. The upstream metric shows fallback hosts and loader hosts or paths that the profile does not serve yet.

Logging uses `json` by default or `text` when configured. Request logs are debug-level and contain only profile, result, and status; the service does not log client data.

On `SIGTERM` or interrupt, the service fails readiness, waits for `limits.shutdown_delay`, then shuts down both listeners within `limits.shutdown_timeout`. The default drain delay is five seconds.
