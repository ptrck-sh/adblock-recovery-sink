+++
title = "Quick start"
description = "Create a constrained CA, start the sink, enroll a device, and add a rewrite."
weight = 3
+++

Generate CA material once, offline. This example constrains both CA certificates to the bundled profile host.

```sh
sink pki init --hosts html-load.com --out ./ars-pki
```

To use the published image:

```sh
mkdir -p ars-pki
docker run --rm --network none --read-only --user "$(id -u):$(id -g)" -v "$PWD/ars-pki:/out" registry.gitlab.com/ptrck-sh/adblock-recovery-sink:0.1.0 pki init --hosts html-load.com --out /out
```

On an SELinux host, add `:Z` to the Podman volume mount.

Start the container with the three files required by the service. The sink listener is TLS on port `443`; the operations and enrollment listener is plain HTTP on `8443` by default.

```sh
docker run --rm --publish 443:443 --publish 8443:8443 --sysctl net.ipv4.ip_unprivileged_port_start=443 -v "$PWD/ars-pki:/pki:ro" -e ARS_PKI_ROOT_CERT_FILE=/pki/root.crt -e ARS_PKI_INTERMEDIATE_CERT_FILE=/pki/intermediate.crt -e ARS_PKI_INTERMEDIATE_KEY_FILE=/pki/intermediate.key registry.gitlab.com/ptrck-sh/adblock-recovery-sink:0.1.0 serve
```

Binding port `443` as a non-root container user requires the shown namespaced sysctl or `CAP_NET_BIND_SERVICE`.

Open `http://sink.example.com:8443/install` from the device, verify the displayed fingerprint against the `pki init` output, and install the root certificate. Then add a DNS rewrite for `html-load.com` to the sink address or CNAME. Keep the root private key offline; the running service does not need it.

See [PKI and enrollment](@/pki-and-enrollment.md) and [DNS setup](@/dns-setup.md) for device and resolver details.
