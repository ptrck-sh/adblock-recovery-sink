+++
title = "adblock-recovery-sink"
description = "A TLS sink for a supported anti-adblock loader resource."
weight = 1
+++

`adblock-recovery-sink` serves a harmless replacement for a supported anti-adblock loader resource. It lets a page remain usable when a network-level blocker causes that loader to fail.

The request flow is DNS rewrite, TLS connection to the sink, then a small stub loader response. The sink uses a certificate chain from a CA you create, constrained to the intercepted name.

It does not block advertisements, manage DNS, or provide block lists or feeds. You choose and maintain the DNS rewrite separately.

Only run an instance you control and only install a root CA that you generated. A trusted root can impersonate websites within its allowed scope, and this service sends JavaScript that executes in visited pages.

## Documentation

- [How it works](@/how-it-works.md)
- [Quick start](@/quick-start.md)
- [Configuration](@/configuration.md)
- [PKI and enrollment](@/pki-and-enrollment.md)
- [Kubernetes and Helm](@/kubernetes-helm.md)
- [DNS setup](@/dns-setup.md)
- [Chrome Local Network Access](@/chrome-local-network-access.md)
- [Operations](@/operations.md)
- [Troubleshooting](@/troubleshooting.md)
- [Compatibility](@/compatibility.md)
- [Validation](@/validation.md)
- [Contributing](@/contributing.md)
- [Prior art](@/prior-art.md)
