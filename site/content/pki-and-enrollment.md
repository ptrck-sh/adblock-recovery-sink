+++
title = "PKI and enrollment"
description = "Constrained certificate authority material and device trust installation."
weight = 5
+++

`sink pki init` creates `root.crt`, `root.key`, `intermediate.crt`, and `intermediate.key`. It creates files with mode `0600`, refuses to overwrite existing files, and prints the root SHA-256 fingerprint. By default, the root is valid for ten years and the intermediate for three years; the service issues seven-day leaf certificates.

Both CA certificates have critical DNS name constraints for the supplied hosts. This limits the CA's certificate authority to the intercepted names, but a trusted root remains powerful. Run your own instance, verify its fingerprint, and retain `root.key` offline.

Initialize a CA for the bundled hosts with registrable domains so the constraints also cover their subdomains:

```sh
sink pki init --hosts html-load.com,content-loader.com,js-loader.com,css-load.com,d37j8pfxu2iogi.cloudfront.net --out ./pki
```

Existing CAs constrained to `html-load.com` keep working, but cover only that domain and its subdomains. Re-enroll with a new CA to cover the other loader hosts.

The service needs `root.crt`, `intermediate.crt`, and `intermediate.key`. It does not need `root.key`. Use the offline root to issue a replacement intermediate when rotating an intermediate. A new root requires installing the new root on every enrolled device before it can be used.

## Enrollment endpoints

The operations listener exposes these endpoints:

| Endpoint | Purpose |
| --- | --- |
| `/install` | Installation page with allowed hosts and fingerprint |
| `/ca.crt` | Downloadable root certificate in DER form |
| `/ca.pem` | Root certificate in PEM form |
| `/ca-chain.pem` | Intermediate then root certificate in PEM form |
| `/fingerprint` | Root SHA-256 fingerprint as text |

## Install and remove

Verify the fingerprint before installation. Use the device's normal trust-store procedure, then remove the same certificate to reverse enrollment.

- Android: Download the root, then install it from Security settings as a CA certificate. Remove it from Trusted credentials.
- iOS and iPadOS: Download the certificate, install the profile in Settings, then enable full trust in Certificate Trust Settings. Remove the profile in VPN and Device Management.
- Windows: Import it into Local Computer, Trusted Root Certification Authorities. Remove that certificate from the same store.
- macOS: Import it into the System keychain and set it to trust for SSL. Delete it from Keychain Access.
- Linux: Add it to the distribution CA trust store and refresh that store. Remove the file and refresh the store.
- Firefox: Firefox can use its own certificate store. Import the root into that store separately when it does not use the operating system trust store.

Chrome may also require the site-specific Local Network Access permission when the sink resolves to a private address. See [Chrome Local Network Access](@/chrome-local-network-access.md).
