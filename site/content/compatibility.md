+++
title = "Compatibility"
description = "Compatibility evidence for the Ad-Shield light loader profile."
weight = 11
+++

# Compatibility report: Ad-Shield light loader

Date: 2026-09-23. Milestone 1 of the [implementation plan](https://gitlab.com/ptrck-sh/adblock-recovery-sink/-/blob/main/docs/anti-adshield-claude-plan.md).

## Verdict

Resource substitution works. Serving one small script as `https://html-load.com/loader.min.js` keeps all three tested sites usable under existing DNS blocking. The page's own inline code defines the contract, so the replacement needs no invented API. Dependent implementation can proceed.

Chrome on desktop and Android asks for Local Network Access on each affected site when the sink sits on a private address, and a denied or suppressed prompt leaves the page broken. The pilot must either document the per-site Allow step or place the sink on an address Chrome treats as public.

## Tested sites

| Site | Integration |
| --- | --- |
| `pinchofyum.com/6-ingredient-espresso-brownies` | Raptive (`adthrive`), recovery script inline in HTML |
| `eatatmaudes.com/the-ultimate-brownie-loaf-with-espresso-brown-butter-frosting/` | Raptive, recovery script injected later by the ad stack |
| `hot-thai-kitchen.com/red-curry-chicken-squash/` | Raptive, behind a Cloudflare bot check |

## Observed loader contract

Raptive ships two inline scripts, `adblock-detection-*` and `adblock-recovery-*` (`data-abr-mode="light"`).

1. The detection script requests `https://ads.adthrive.com/abd/abd.js` with XHR unless the `__adblocker` cookie exists. When the request fails, it sets `__adblocker=true` for five minutes. When it succeeds, `abd.js` runs bait-element checks and sets the cookie itself.
2. The recovery script polls the cookie every 50 ms for up to five seconds. On `true` it injects `<script id="Tqgkgu" src="https://html-load.com/loader.min.js">` with inline `onload` and `onerror` handlers.
3. `onload` posts a random string `X` as `X_as_req` to its own window and expects `X_as_res` back. It retries after 100, 500 and 1000 ms, then reports the loader as "tainted".
4. `onerror` retries the same path on a base64 host list embedded in the page: `fb.html-load.com`, `d37j8pfxu2iogi.cloudfront.net`, `content-loader.com`, `fb.content-loader.com`. Live runs also requested `rule.evenyippee.com`, so the list rotates.
5. After the last host fails, it posts the page URL to `error-report.com/report?type=loader_light`, overlays a full-screen iframe from `report.error-report.com/modal`, and checks every second that the iframe stays visible. If the iframe fails or disappears, `confirm()` offers a redirect to the report page; dismissing it calls `location.reload()`, so the page loops.
6. A second inline guard checks that `script#Tqgkgu` exists and that a `data:` script executes within 251 ms.
7. The `essential` mode injects a different obfuscated inline script tagged `html-load.cc`. None of the tested sites use it; it is unsupported.

The replacement in [`profiles/adshield/loader.min.js`](https://gitlab.com/ptrck-sh/adblock-recovery-sink/-/blob/main/profiles/adshield/loader.min.js) answers step 3 and nothing else: it replies to `*_as_req` messages from the same window. It loads no further resources.

## Results

Headless Playwright on a LAN host behind the existing resolvers. The proof sink terminated TLS with a throwaway root trusted in each browser profile's NSS database; certificate errors were not ignored. A CONNECT proxy routed only `html-load.com:443` to the sink, standing in for a DNS rewrite, so every other host went through the LAN resolvers unchanged.

| Site | Engine | Without sink | With sink |
| --- | --- | --- | --- |
| pinchofyum | Chromium 1243 | all loader hosts fail, 7 dialogs, 16 reloads, stylesheets drop to 0, modal iframe | 1 loader request, 0 dialogs, 29 stylesheets kept |
| pinchofyum | Firefox 155 | 7 dialogs, 14 reloads, stylesheets drop to 0 | 1 loader request, 0 dialogs, 29 stylesheets kept |
| eatatmaudes | Chromium 1243 | 6 dialogs, 14 reloads, stylesheets drop to 0 | 1 loader request, 0 dialogs, 41 stylesheets kept |
| eatatmaudes | Firefox 155 | 6 dialogs, 14 reloads | 1 loader request, 0 dialogs, 41 stylesheets kept |
| hot-thai-kitchen | Firefox 155 | loader failure, dialog, reload | 1 loader request, 0 dialogs, 42 stylesheets kept |
| hot-thai-kitchen | Chromium 1243 | Cloudflare bot check blocks headless Chromium | not tested |

With the sink, no fallback host was requested and no host appeared that the blocked run did not already contact. The substitution adds no advertising or tracking payloads.

### Android

Chrome 153 on Android 16 (Honor MagicPad 2), driven over wireless debugging. The test root was installed as a user CA certificate. A Chrome command-line flag mapped `html-load.com` to the sink through an `adb reverse` tunnel on `127.0.0.1:8443`; every other host used the tablet's own resolver.

| Case | Result |
| --- | --- |
| No mapping | 4 dialogs, 10 reloads, stylesheets drop to 0, modal iframe |
| Mapping, root not installed | `ERR_CERT_AUTHORITY_INVALID`, same breakage |
| Mapping, root installed, Local Network Access blocked | `html-load.com` fails with `ERR_FAILED` before reaching the sink, same breakage |
| Mapping, root installed, Local Network Access prompt pending | loader request hangs; neither `onload` nor `onerror` fires, so the page happens to stay styled |
| Mapping, root installed, Local Network Access granted | pinchofyum (29 stylesheets), eatatmaudes (41) and hot-thai-kitchen (42) recover: 1 loader request, 0 dialogs, sink hit over HTTP/2 |

Hot-thai-kitchen passes its Cloudflare check on the real device.

## Local Network Access

Chrome treats a public page requesting a private or loopback address as local network access and asks per site: "*site* wants to access other apps and services on this device" for loopback, and a local-network variant for private addresses. A DNS rewrite to a LAN sink triggers this for every affected site.

- Denying the prompt leaves the page as broken as without the sink.
- After blocks or ignored prompts, Chrome embargoes the site and shows "This site can't ask for your permission"; resetting site permissions did not lift it during testing. The granted case above used a DevTools permission override for that reason.
- Each device user has to allow each affected site once.

Chrome classifies by the resolved IP address only, so the sink avoids the prompt when it answers on an address Chrome treats as public, even if the traffic never leaves the LAN. Managed desktops can also use the `LocalNetworkAccessAllowedForUrls` policy. Firefox showed no equivalent prompt in headless testing.

Decision: the default deployment tolerates the prompt, and the setup guide documents the one-time Allow per site and device. Operators who want no prompt can publish the sink on their own public address: rewrite `html-load.com` to a CNAME of their own hostname, point that hostname at their WAN address, and forward TCP 443 to the TLS-passthrough entrypoint. The record must be DNS-only; a TLS-terminating proxy such as Cloudflare's orange cloud cannot pass `html-load.com` through to the sink.

Verified 2026-09-24: with the sink reached through the operator's public address, Chrome 153 on Android loaded all three sites through the sink with no Local Network Access permission granted and no prompt. Two network conditions apply. LAN clients connect to their own public address, so the router in front of the passthrough entrypoint must loop that traffic back (NAT loopback). A firewall that only admits a CDN's address ranges on port 443 must also admit the looped-back sources.

## Host set

On the Raptive sites the primary `html-load.com/loader.min.js` is enough: fallback hosts are requested only after it fails.

Other integrations name the loader differently. `tomshardware.com` (2026-09-24) runs the same light SDK (`data-sdk="l/1.2.10"`) but starts at `html-load.com/app.js` and then tries `fb.html-load.com/vendor.js`, `dkyerkk91s4fa.cloudfront.net/main.js`, `content-loader.com/app.js` and `fb.content-loader.com/vendor.js`. Success is the same `*_as_req`/`*_as_res` handshake. Serving the unchanged stub on those paths kept the page intact in Chromium and Firefox: no dialogs, no `error-report.com` request, all stylesheets present.

A host that answers `404` still advances the fallback chain, but a host that answers `200` with any script does not: the page then waits for the handshake and fails without trying fallbacks. A resolver block page with a trusted certificate (verified with the NextDNS block page on Android, 2026-09-24) is such a host, so every loader host needs a rewrite to the sink. Because of the first case, so the bundled profile serves the stub on `/loader.min.js`, `/app.js`, `/vendor.js` and `/main.js` for every known domain and all of its subdomains, including `js-loader.com`, `css-load.com` and the numbered subdomains from [Jacob Desforges' research](https://jacobdesforges.com/adshield-ad-reinsertion/).

## Browser restrictions

- CSP: `pinchofyum.com` sends `frame-ancestors 'self'` and a meta policy of `upgrade-insecure-requests; block-all-mixed-content`. Neither restricts script sources. `eatatmaudes.com` sends no CSP.
- SRI: the loader element is created without an `integrity` attribute.
- Caching: the sink sends `Cache-Control: no-store`. A browser that cached the real loader before the rewrite may keep running it until the cache expires; clear site data after enabling the rewrite.
- Chrome Local Network Access: prompts per site; see [Local Network Access](#local-network-access).

## Resolver behaviour before the sink

On the test LAN, `ads.adthrive.com` and `content-loader.com` resolve to `0.0.0.0`. `html-load.com` and `report.error-report.com` resolve to a block-page address that presents an untrusted certificate. Both end states trigger the failure path above.

## Not yet tested

- Android through a real DNS rewrite to a private address, after a local deployment.
- iOS/iPadOS and Safari.
- Headed desktop Chrome.
- Firefox for Android.
- `essential` mode.

## Reproduce

Requirements: Go, `uv`, `certutil` (`libnss3-tools`) and Playwright browsers (`uv run --with playwright playwright install chromium firefox`).

```sh
go run ./test/smoke/proofsink -ca test/smoke/.work/root.pem
test/smoke/run.sh chromium "file://$PWD/test/fixtures/adshield-light/index.html" fixture
test/smoke/run.sh firefox https://pinchofyum.com/6-ingredient-espresso-brownies pinchofyum
test/smoke/run.sh chromium https://pinchofyum.com/6-ingredient-espresso-brownies control notrust
```

The fixture page sets its title to `ok` when the handshake succeeds. Live-site runs print loader hosts, dialog count, navigations and the final stylesheet count. These are manual smoke tests. The CI job `e2e:browser` runs `test/e2e/run.sh`, which starts the real `sink` binary with the toast enabled and checks the handshake and toast in Chromium for seven loader URLs, two of them numbered subdomains matched only by a wildcard.

## Prior art

[tinyShield](https://github.com/FilteringDev/tinyShield) documents the same Ad-Shield domains and defuses the loader inside the browser. This project uses no code or data from it.
