# Validation: service, PKI and chart

Date: 2026-09-23. Covers milestones 2 to 5 of the [implementation plan](anti-adshield-claude-plan.md) on branch builds; nothing was released.

## Setup

- Image `registry.gitlab.com/ptrck-sh/adblock-recovery-sink:feat-service-core` (linux/amd64 and linux/arm64) from the branch pipeline.
- Chart branch `feat/chart-baseline` in a test namespace of a k3s cluster (Raspberry Pi nodes), default Traefik mode, with `routing.traefik.annotations` set to the cluster's `kubernetes.io/ingress.class`.
- Test PKI: a 30-day root and intermediate, both name-constrained to `html-load.com`, stored as an externally created Secret and referenced through `pki.existingSecret`.
- Clients: headless Chromium and Firefox through a CONNECT proxy that sends `html-load.com:443` to the Traefik load balancer, and Chrome 153 on Android 16 with a host-resolver mapping to the same address.

## Results

| Check | Result |
| --- | --- |
| `pki init` | four `0600` files, optional Secret manifest without `root.key`, refuses to overwrite |
| TLS passthrough | Traefik forwards SNI `html-load.com` untouched; the pod serves a 7-day leaf from the intermediate |
| Unknown SNI, host or path | handshake rejected, 421, 404 |
| Hostname | `/install`, `/ca.crt`, `/ca.pem`, `/ca-chain.pem`, `/fingerprint` and `/status` served; `/healthz`, `/readyz` and `/metrics` return 404 externally |
| Restart, two replicas, upgrade | same root fingerprint throughout; each replica issues its own leaf |
| Single-replica restart under load | 1939 of 1939 requests succeeded with the shutdown drain |
| Pod security | no volumes or mounts, token not mounted, UID 65532, read-only root, all capabilities dropped, `RuntimeDefault` seccomp, PKI via `secretKeyRef` only |
| NetworkPolicy | a pod with the sink's labels cannot reach LAN, internet or DNS; an unlabeled control pod can |
| Optional resources | ServiceMonitor, VPA, PDB, HPA and Certificate pass a server-side dry run |
| Browsers | pinchofyum, eatatmaudes and hot-thai-kitchen recover in Chromium, Firefox and Android Chrome; an untrusted root reproduces the original breakage |

## Findings fixed during validation

- The image started without a subcommand; it now defaults to `serve`.
- A single-replica restart dropped a few TLS connections while Traefik still routed to the terminating pod. `serve` now fails readiness and keeps serving for `limits.shutdown_delay` (default 5s) before closing listeners.
- Traefik on the test cluster only reads routes carrying its ingress class annotation; the chart gained `routing.traefik.annotations`.

## Not covered

- A real DNS rewrite instead of a proxy or host mapping.
- The Local Network Access prompt on a private address was granted through DevTools rather than tapped.
- iOS/iPadOS, Safari and Firefox for Android.
