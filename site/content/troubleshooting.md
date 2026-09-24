+++
title = "Troubleshooting"
description = "Common DNS, certificate, Local Network Access, and routing failures."
weight = 10
+++

| Symptom | Cause | Fix |
| --- | --- | --- |
| Certificate authority error | The device does not trust this instance's root. | Enroll the device from `/install` and verify the fingerprint first. |
| The page still breaks | DNS or browser cache has the old result, or Secure DNS or Private DNS bypasses the rewrite. | Confirm the resolver answer, wait for expiry or clear site data, and use a resolver path that applies the rewrite. |
| Chrome shows or blocks a Local Network Access prompt | The sink resolves to a private or loopback address. | Allow the affected site, or use the public-address design in [Chrome Local Network Access](@/chrome-local-network-access.md). |
| TLS handshake fails | The SNI name is not configured. | Use only a host supplied by an enabled profile and include it in the generated CA constraints. |
| HTTP `421` | The HTTP Host is not configured or accepted. | Check the DNS rewrite and configured `hosts`. |
| HTTP `404` | The host is accepted but the request path has no profile route. | The bundled profile serves `/loader.min.js`, `/app.js`, `/vendor.js` and `/main.js` only. |
| HTTP `405` | The profile does not permit the request method. | Use a supported method or correct the profile. |

Test TLS routing directly with a trusted CA file:

```sh
curl --cacert root.crt --resolve html-load.com:443:203.0.113.10 https://html-load.com/loader.min.js
```

Replace `203.0.113.10` with the sink address. Inspect the local operations listener for configuration, enabled profiles, accepted hosts, CA expiry, and the root fingerprint:

```sh
curl http://127.0.0.1:8443/status
```
