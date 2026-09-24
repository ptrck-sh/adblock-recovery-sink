+++
title = "Why a Go service"
description = "Why this is a small Go program and not an nginx configuration."
weight = 15
+++

The response itself is trivial: a few hundred bytes of JavaScript over HTTPS. Any web server can return that, and nginx, Caddy or HAProxy with a static file would work for a single host with a certificate you manage by hand. The work is everything around that response, and in a web server config each part needs its own tool, script or module.

## Certificates

A browser only accepts the replacement if the TLS certificate for `html-load.com` chains to a root it trusts. With nginx you would:

- create a root and an intermediate with name constraints using `openssl`, `step` or `cfssl`
- issue a leaf per intercepted host, or one leaf with every host as a SAN
- renew the leaves before they expire, reload nginx, and repeat whenever the host list changes

The sink loads only the root certificate, the intermediate certificate and the intermediate key. It issues a leaf for the requested SNI in memory, the first time a host is requested, and renews it before it expires. Leaves live seven days and never touch disk. Adding a host is a config change, not a certificate job.

## Fail-closed checks

nginx serves whatever certificate and key it is given. The sink refuses to start when the intermediate key does not match its certificate, the chain does not verify, a certificate is expired, or a configured host is outside the CA name constraints. During the handshake it rejects any SNI it is not configured for, so it cannot be turned into a general-purpose interception proxy by a DNS mistake.

## Exact routing

Each profile route is an exact host, path and method, served with fixed headers. Everything else gets a TLS failure, `421`, `404` or `405`. In nginx that is a `server` block per host, `location =` blocks, `return 200` with an escaped JavaScript string or a file, `if` checks on the method, and CORS handling written by hand. The profile YAML states the same thing as data, and the unit tests check it.

## Enrollment

Devices need the root certificate in a form they can install. The ops listener derives `/install`, `/ca.crt` (DER), `/ca.pem`, `/ca-chain.pem` and `/fingerprint` from the loaded CA, so the download always matches what the sink signs with. With nginx you would convert and publish these files yourself and keep them in sync with every CA rotation.

## Options that change the response

The optional toast is appended to JavaScript responses at startup from two settings. nginx would need `sub_filter`, `njs` or a templating step before startup for the same result.

## Observability

`/metrics` counts requests by profile and result, TLS handshake failures by reason, cached leaves, and the issuer expiry time. `/readyz` fails when the issuer is not valid. An nginx setup needs an exporter for request counts and has no signal for an intermediate that is about to expire.

## Packaging

The result is one static binary with three direct dependencies besides the standard library: koanf for config, pflag for flags and the Prometheus client. The image is distroless, runs as non-root, has no shell and needs no writable filesystem or volumes. PKI material comes from environment variables or files, so the Helm chart passes it in from a Secret without any volume. An nginx image needs config files, certificate files and a way to reload after renewal.

## When nginx is enough

If you intercept one host, already run nginx, and are happy to issue and renew the certificate yourself, a static file behind a `server` block does the job. This project exists because the host list grows, certificates have to be short-lived and constrained, and devices have to be enrolled, all of which the sink handles in one place.
