+++
title = "Kubernetes and Helm"
description = "Deploy the sink with the companion Helm chart."
weight = 6
+++

The companion chart is maintained at [adblock-recovery-sink-chart](https://gitlab.com/ptrck-sh/adblock-recovery-sink-chart). Read its README for the complete values reference and install commands.

The values are organized around the workload and service, TLS-passthrough routing, PKI material, and network isolation. Key settings include:

- `ingress.kind`: choose `IngressRoute` or `Ingress`.
- `gateway`: configure the gateway or entry point for TLS passthrough.
- `interception.hosts`: declare the names accepted by the sink and routing layer.
- `pki.existingSecret`: reference a pre-created Secret containing the required CA and intermediate material.
- `networkPolicy`: enable or configure network restrictions for the workload.

TLS must pass through to the sink because it selects and serves a certificate for the intercepted SNI. Do not terminate TLS before the sink for `html-load.com`.
