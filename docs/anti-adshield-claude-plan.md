# Anti-AdShield — implementation handoff for Claude

## Objective and constraints

Build an OSS service that serves harmless replacements for known Ad-Shield resources, keeping supported websites usable while existing ad/tracker blocking remains active. Target desktop, phone and tablet browsers without extensions.

The user's final decisions supersede earlier proposals in the exported thread:

- One Go application; static Linux amd64/arm64 binaries, OCI image and Helm chart.
- Koanf configuration: defaults → file → environment → explicitly supplied CLI flags.
- DNS rewrites already exist as a LAN capability. Users manage them; the application never changes DNS.
- No domain-feed dependency, updater or synchronization API. Credit tinyShield and related prior art in documentation.
- No runtime volumes, PVC, database or writable application state. CA identity is supplied externally and survives restarts.
- Traefik by default; opt-in Gateway API and legacy Ingress integration, HPA, VPA and cert-manager resources.
- Reuse the existing GitLab component library for app/chart CI. Do not redesign CI.

Read repository instructions and existing project conventions first. Create the six work items below with their dependencies and acceptance criteria. Keep scope small; milestone 1 decides whether the proposed bypass is viable.

## 1. Prove resource substitution works

**Deliver:** one working compatibility profile, minimal reproducible fixtures and a short compatibility report.

Inspect current loader behavior on two or three representative affected sites. The export contains no concrete affected-site URLs; recover them from available project context or obtain a failing example before claiming success.

Record requested hosts/paths, execution order, required globals/callbacks, inline watchdogs, CSS deletion and redirects. Test replacement through real DNS rewriting and trusted HTTPS—not only browser request interception.

Include desktop Chromium/Firefox and a real Android browser. Check CSP/SRI, cached scripts, and public-page requests to the private sink. Chrome's Local Network Access permission can affect this architecture; record prompts and behavior when permission is denied. Headers alone must not be assumed to solve this.[1]

**Acceptance:** a site that breaks under existing DNS blocking remains usable with the replacement; styles survive, error redirects disappear, and the replacement loads no advertising/tracking payloads. No extensions, modified publisher HTML or disabled browser security. Document any required per-site permissions and unsupported cases.

**Gate:** if substitution cannot satisfy the inline watchdog or browser restrictions, report the evidence and stop dependent implementation. Do not invent an API such as `window.adshield.loaded` from the thread's illustrative pseudocode.

## 2. Implement the service and profile engine

**Depends on:** 1.

- Commands: `serve`, `config validate`, `pki init`, `version`.
- Separate interception listener (`8443`) and enrollment/operations listener (`8080`). Bind the latter to loopback for standalone use; explicitly configure it for containers.
- Small modules: configuration, PKI, profiles, HTTP and metrics. Use Go's standard TLS/X.509 implementation.
- Versioned profiles define explicit hosts, paths, methods, response bodies, status, MIME, CORS and cache behavior. Bundle the initial profile; support operator configuration overrides without a remote updater.
- Normalize hostnames and define exact/wildcard matching consistently. Wildcards never authorize unrelated suffixes. Distinguish apex domains from subdomains.
- Unknown SNI: reject TLS. Unknown path on an allowed host: 404. Reject unauthorized HTTP hosts. No upstream proxy or arbitrary certificate issuance.
- Only emulate CSS, images or reporting endpoints when the observed contract requires them. Never reflect request content into executable responses.
- TLS 1.2/1.3, HTTP/1.1 and HTTP/2; no HTTP/3 or `Alt-Svc` initially. Bound request sizes, timeouts and certificate-cache size; shut down gracefully.

**Acceptance:** the profile passes the milestone-1 fixture; config precedence works, including explicit `false` values and flags whose defaults must not override file/environment settings.[2] The daemon operates without outbound network access.

## 3. Add stable PKI and device enrollment

**Depends on:** 2.

- `pki init` generates a root and intermediate once and writes owner-readable bootstrap files to a selected location. Offer Kubernetes Secret output; never log private keys.
- Keep the root private key outside the running service. Runtime receives the root certificate, intermediate certificate and intermediate signing key.
- Kubernetes: reference an existing, externally managed Secret through `secretKeyRef` environment variables. Generate ordinary config environment values from chart values/ConfigMap. No mounted Secrets or config volumes are required.[3]
- Binary/container: accept equivalent injected values, with file configuration available for standalone deployments.
- Generate short-lived hostname certificates in a bounded memory cache; renew before expiry and never exceed the issuer's validity. All replicas use the same CA identity; independent leaf caches are fine.
- Validate key/certificate matching, CA constraints and validity. Missing or invalid credentials fail startup; expiry makes readiness fail. Never silently generate a new CA during `serve`, upgrades or Helm rendering.
- Enrollment exposes `/install`, `/ca.crt` (DER), `/ca.pem`, `/ca-chain.pem` and the root SHA-256 fingerprint. Download public certificates only; users install the root, not leaf certificates.
- Serve enrollment through a user-controlled hostname with independently trusted TLS, or a documented LAN bootstrap path with fingerprint verification. Keep the page self-contained; use Catppuccin Latte styling.

**Acceptance:** restart, upgrade and two replicas preserve the trusted root. Document installation/removal for Android, iOS/iPadOS, Windows, macOS and Linux, including browser-specific trust limitations. Secret changes require an explicit rollout; root rotation is a separate enrollment operation.

## 4. Package the binary and secure container

**Depends on:** 2–3.

- Static binary and multi-architecture scratch/distroless image; no shell.
- Fixed non-root UID/GID, read-only root filesystem, no privilege escalation, all capabilities dropped, `seccompProfile: RuntimeDefault`.
- Unprivileged application ports; publish/map HTTPS 443 to 8443. No host networking or privileged containers.
- No runtime volumes or Kubernetes API permissions; disable service-account token automount.
- `/healthz`, `/readyz`, `/metrics`; route only enrollment paths through the enrollment ingress. Keep operations endpoints private.
- Operational logs only: no client IPs, referrers, cookies, query strings or request bodies. Metrics use bounded profile/result labels, never raw host/path/client labels.
- Expose request/failure totals, unknown-route counts, certificate-cache size, issuer expiry and profile version. Avoid a browsing-history dashboard.

**Acceptance:** run as non-root with read-only filesystem, no mounts and outbound access denied; health, enrollment and a real substituted request succeed.

## 5. Build the Helm integration

**Depends on:** 3–4.

Provide Deployment, separate sink/operations Services, configuration, probes, resource requests/limits and NetworkPolicy. Support optional ServiceMonitor, PDB, HPA and VPA. Default to one replica; default VPA to recommendation-only (`Off`) when enabled, avoiding concurrent HPA/VPA resource adjustment.

| Mode | Intercepted traffic | Enrollment |
| --- | --- | --- |
| Traefik — default | `IngressRouteTCP`, scoped SNI matches, `tls.passthrough: true` | HTTP `IngressRoute` on the user's hostname |
| Gateway API — opt-in | `TLSRoute` attached to a compatible `Passthrough` listener | `HTTPRoute` |
| Legacy Ingress — opt-in | Direct TCP Service, or explicitly documented/tested controller-specific passthrough | Standard Kubernetes `Ingress` |

The application must terminate interception TLS. Standard Ingress does not provide portable TLS passthrough; do not silently route this traffic through ordinary HTTP termination.[4–6] Generate application and route host scopes from the same chart values; never install a catch-all SNI route on a shared entrypoint.

Optional cert-manager `Certificate` resources secure the normal enrollment hostname through an existing issuer. Do not request public certificates for intercepted third-party domains or make cert-manager mandatory.

Allow ingress from the selected controller/LAN path and monitoring source; deny outbound traffic by default. Expose controller/CRD prerequisites and fail clearly for unsupported enabled options. Do not install cluster-wide controllers or CRDs from this chart.

**Acceptance:** render all modes; deploy the default mode, confirm the browser sees the application's CA-issued leaf, then verify restart and two-replica behavior without volumes. Use existing GitLab app/chart components for releases.

## 6. Integrate, pilot and release

**Depends on:** 1–5.

1. Bootstrap/back up CA material; deploy the sink before changing client DNS.
2. Enroll one desktop and one Android device. Verify trust and any local-network permissions.
3. User adds rewrites for the validated host set only. Document AdGuard Home/NextDNS examples, apex/wildcard coverage, A/AAAA consistency and resolver/rebinding exceptions where needed. The application receives no DNS credentials.
4. Run the affected-site checks using actual DNS resolution. Off-LAN access requires a route/VPN to the sink; document that prerequisite.
5. Pilot on those devices, then expand deliberately. Publish a dated site/browser compatibility matrix; mark untested combinations explicitly, including iOS/Safari until tested.
6. Release binary, image and chart through existing CI conventions. Add concise setup, troubleshooting, contribution, security-reporting and prior-art documentation; follow existing OSS licensing conventions.

Rollback application/profile configuration while retaining the CA. To withdraw interception, users restore prior DNS rules and clear relevant caches; original blocking-related breakage may return. Disabling a profile alone does not remove its DNS rewrites. Document explicit root rotation and trust removal.

**Done means:** demonstrated page recovery on supported targets, stable trust across restarts/scaling, working downloads, secure volume-free deployment, and a rehearsed rollback. Keep automated tests focused on profile behavior, config precedence, host authorization/PKI and chart rendering; use live-site checks as manual release smoke tests, not flaky mandatory CI.

## Technical references

1. [Chrome Local Network Access](https://developer.chrome.com/blog/local-network-access)
2. [Koanf configuration providers](https://github.com/knadh/koanf)
3. [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/)
4. [Traefik IngressRouteTCP](https://doc.traefik.io/traefik/reference/routing-configuration/kubernetes/crd/tcp/ingressroutetcp/)
5. [Gateway API TLS configuration](https://gateway-api.sigs.k8s.io/guides/user-guides/tls/)
6. [Kubernetes Ingress TLS semantics](https://kubernetes.io/docs/concepts/services-networking/ingress/)

Prior art: [tinyShield](https://github.com/FilteringDev/tinyShield). Credit relevant projects; verify provenance and license before reusing code or fixtures. No runtime feed integration.
