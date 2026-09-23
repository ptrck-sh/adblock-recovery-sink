# Chat Export

_Date: 2026-09-23_

---

## User

is there any solution for website trying to force their tracking and ad bs on me by gaslighitng users via the "error-reporting" into disabling adblock?
I layered DNS level blocking.

## Assistant

Yes. DNS blocking alone is the wrong layer for this specific annoyance: the site's JavaScript can notice that ad/tracker resources failed and then throw the fake “something went wrong / disable your blocker” UI.

Best stack:

- Firefox + uBlock Origin. Full uBO still gives you cosmetic filtering, scriptlets, per-site JS control, etc.
- Keep your DNS blocking underneath it.
- In uBO, enable annoyance lists such as `uBlock filters – Annoyances` and optionally AdGuard Annoyances.
- For stubborn sites, block/neutralize the anti-adblock script rather than allowing the ad domain.
- If the “error reporting” overlay itself is just DOM garbage, use uBO's element picker and permanently hide it.

So conceptually:

```text
DNS blocker → network-wide trackers/ads
uBO → browser requests + anti-adblock JS + cosmetic garbage
```

The second layer is what defeats most of this crap.

One warning: don't blindly enable 15 overlapping filter lists. uBO explicitly notes that this increases site breakage.

---

## User

[first screenshot attached]

[second screenshot attached]

[third screenshot attached]

first is breaks, looks like the js is literally deleting all CSS and JS loaded, then alterts about error-reporting

## Assistant

Yes. Your reading is basically correct.

This is an anti-adblock system, apparently tied to Raptive/Ad-Shield. It deliberately detects that domains such as `html-load.com` failed, then sabotages the page and redirects/alerts via `error-report.com`. `report.error-report.com` itself confirms the goal is “Please allow ads,” and AdGuard tracks it specifically as an anti-adblock script.

So DNS-only blocking creates exactly the failure mode they want.

Best fix: keep DNS blocking, but add browser-side neutralization. There is even a purpose-built OSS extension called Ad‑Shield Defuser that intercepts the loader, prevents stylesheet removal, suppresses the dialogs, and blocks redirects to `error-report.com` / `*-load.com`.

For Firefox/uBO, the other practical approach is: let the anti-adblock loader resolve at DNS level, then have uBO replace/block it inside the browser. Community reports use roughly:

```text
@@||html-load.com^
||html-load.com^$redirect=empty

@@||error-report.com^
||error-report.com^$redirect=empty
```

The important bit is architectural: DNS must not NXDOMAIN the “canary” domain, because that is what triggers the retaliation. Let the browser receive the request, then neutralize the response/script locally.

For that specific site, disabling JavaScript also likely works, because the destructive part is JS; downside is obvious if the site actually needs JS.

---

## User

oss extension would only work on desktops, I need that to work on tablet and phones aswell. (that is the whole reason for network level ad/tracking blocking via adguardhome and nextdns

## Assistant

Correct. If the requirement is “works for every browser/device with no extension,” then this has to be solved at the network layer — and DNS alone is too coarse for this specific anti-adblock trick.

Exactly. An extension is not the solution for your requirement.

For “AdGuard Home / NextDNS protects every device,” this is a fundamental DNS limitation: DNS sees only `html-load.com`, not which script/path is requested or what that script does. Ad-Shield deliberately exploits that. Blocking `html-load.com` causes the site to self-destruct; HaGeZi ultimately treats these domains as allowlist candidates because DNS filtering cannot fix the breakage cleanly.

You basically have three options:

1. Allow the anti-adblock infrastructure:

```text
@@||html-load.com^
@@||content-loader.com^
@@||error-report.com^
```

The sites work, while AGH/NextDNS still block the other ad/tracker domains. But some advertising payload gets through these domains. This is currently the pragmatic DNS-only solution.

2. Keep blocking them and accept broken sites. There isn't a magic DNS response such as NXDOMAIN vs `0.0.0.0` vs a block page that reliably fixes this; their JS verifies that the expected resource actually loaded.

3. If you want actual network-wide defeat of this crap, move one layer above DNS:

```text
clients
  ↓
transparent HTTPS filtering proxy
  ↓
AdGuard Home / NextDNS
  ↓
internet
```

Something like Squid `ssl_bump` or mitmproxy can selectively neutralize the Ad-Shield script while leaving the rest blocked. But that requires installing your own CA certificate on phones/tablets and handling QUIC/pinning. Much heavier.

So for your setup I'd do #1: narrowly allow `html-load.com`, `content-loader.com`, and possibly their subdomains, keep `error-report.com` blocked unless required for page operation. Don't globally weaken the rest of your DNS lists.

And yes: the behavior in your screenshots is intentional anti-adblock retaliation, not a genuine loading error.

---

## User

could I redirect the domains to a local nerfed version?

## Assistant

Yes — technically. But not with DNS rewrite alone.

You could do:

```text
html-load.com ──DNS──> 10.10.x.y
content-loader.com ──DNS──> 10.10.x.y

                     local HTTPS server
                     └── /app.js → harmless stub
```

The problem is HTTPS. The browser requests `https://html-load.com/app.js`, so your local server needs a certificate valid for `html-load.com`. You cannot obtain a publicly trusted one because you don't own that domain.

So the workable architecture is:

```text
AdGuard Home DNS rewrite
        ↓
local nginx/Caddy/mitmproxy
        ↓
certificate signed by YOUR CA
        ↓
your CA installed/trusted on phones/tablets/desktops
```

Then you can serve a nerfed `app.js` that satisfies whatever Ad-Shield expects without loading its advertising payload.

This actually has some advantages over generic HTTPS MITM: you only intercept a tiny known set of hostile domains rather than all traffic.

Main complications:

- Ad-Shield rotates across `html-load.com`, `content-loader.com`, `js-loader.com`, `css-load.com`, etc.
- Your stub needs to behave sufficiently like their loader; an empty `200 OK` may still trigger their inline watchdog.
- CSP/SRI could occasionally complicate substitution.
- Apps using certificate pinning won't accept your CA.
- Away from home, NextDNS cannot redirect to a private `10.x` address unless the device VPNs back home.

For your managed devices, though: yes. I'd seriously consider a tiny dedicated “anti-Ad-Shield sink” behind AdGuard Home, with your internal CA installed on the family devices. Much cleaner than allowing the domains outright.

---

## User

**(Recipes only) Use [JustTheRecipe](https://www.justtherecipe.com/) or [DrizzleLemons](https://www.drizzlelemons.com/):** Both of these sites strip away all the ads and fluff from online recipes, with slightly different monetization methods. These seem like great alternatives for people who don’t want to self-host something like Mealie, but as with all hosted web apps they’ll probably be prone to enshittification at some point.

## Assistant

Yes. For recipe sites specifically, this is probably the cleaner answer than fighting their anti-adblock JS.

Both services fetch the recipe server-side and return a stripped recipe view, so your phone/tablet never has to execute the hostile page JS. DrizzleLemons explicitly supports mobile and even Android sharing into its installed web app.

Given your preference for self-hosting, Mealie is still the better long-term version of this pattern: paste/share URL → server imports recipe → you never interact with the original garbage page again. The hosted options are nicer for zero-maintenance use, but they add an external dependency and monetization risk.

For your use case, I'd rank the workflow:

```text
Share recipe URL → Mealie
fallback: Share URL → JustTheRecipe / DrizzleLemons
avoid opening recipe site directly whenever possible
```

---

## User

the local sync anti-ad-shield, what feature set would it need? is there already a oss version of this?

## Assistant

Not as a turnkey OSS network service, from what I can find.

The closest existing OSS project is tinyShield. It is actively maintained, specifically targets Ad‑Shield, and even publishes a machine-readable, daily-updated Ad‑Shield domain collection. But the actual bypass is still client-side JavaScript/userscript logic.

A local network sink would therefore be a new implementation. I’d keep its scope very small:

1. Domain feed
   - consume `tinyShield-domain-collection`
   - generate AdGuard Home DNS rewrites automatically
   - cover rotating `*.html-load.com`, `*.content-loader.com`, etc.

2. Local HTTPS endpoint
   - DNS redirects those domains to one local IP
   - locally trusted CA issues certificates dynamically
   - probably use an OSS proxy primitive such as mitmproxy rather than writing TLS handling yourself

3. Ad-Shield response emulator
   - handle known paths such as `/app.js`, `/loader.min.js`
   - return valid JS, not merely empty `200`
   - create whatever globals/callback state the embedding page expects
   - make `<script onload>` succeed
   - never load/reinsert ads

For example, potentially something conceptually like:

```js
window.adshield ??= {};
window.adshield.loaded = true;
window.adshield.isAdBlockerDetected = false;
window.adshield.cmd ??= [];
```

But the exact contract has to be learned from their current runtime. Ad-Shield explicitly uses integrity checks and changes its implementation to resist blockers, so a static stub will eventually break.

4. Compatibility layer
   - correct MIME/CORS/cache headers
   - preserve expected HTTP status/redirect behaviour
   - wildcard host support
   - optionally block UDP/443 so clients fall back to HTTPS/TCP if HTTP/3 becomes troublesome

5. Auto-update + regression tests
   - periodically ingest the tinyShield domain list
   - fetch/analyse current Ad-Shield loader behaviour
   - test against several known Ad-Shield sites
   - alert rather than silently pushing a broken emulator

That last part is the important one. Their bootstrap has inline fallback code that can remove CSS and trigger `error-report.com` when the expected loader behaviour fails. That's why tinyShield/Ad-Shield Defuser intervene inside the page rather than merely returning an empty resource.

So I'd structure it:

```text
tinyShield domain collection
          ↓
     sync/controller
       ↙       ↘
AdGuard rewrite   local TLS sink
                       ↓
               Ad-Shield emulator
```

Existing OSS pieces:

- tinyShield — maintained bypass logic + knowledge base.
- tinyShield-domain-collection — excellent domain feed, already regenerated daily.
- microShield — predecessor, now obsolete/archived.
- Ad-Shield Defuser — useful reference implementation, but browser-side.

I found no existing OSS project that combines those into the specific **DNS rewrite → local TLS endpoint → fake Ad-Shield runtime** architecture.

The interesting part is that ~80% of the plumbing already exists. The genuinely new component is probably only the server-side `app.js`/loader emulator plus synchronization glue.

---

## User

then lets make this a statement.
we make this a project, and opensource is.
it'll be available as binary and container, and we offer a helm chart as usual. coanf is used to have 3 levels of config file < env < cli flags
@Deep research BE CONCISE.
it should expose the self-generated CA and Certs sothat with an easy download, devices can install them.

comeup with an implemationtion plan and rollout plan and integration plan. I'll hand it to claude then to create workitems and coordinate the implementation

## Assistant

Deep Research has started. It will first produce a research plan, then the concise implementation, integration, and rollout plan suitable for handing to Claude.

---

## User

so?

## Assistant

The research run stopped after producing only its plan. I’m finishing the actual deliverable now: current OSS landscape, then the concrete architecture, implementation, integration, and rollout plan.

The research run failed to return the report, so here is the actual handoff-ready plan.

## Project statement

Build an OSS network-level anti-anti-adblock service.

It intercepts only known hostile loader domains, terminates HTTPS with a locally trusted CA, and returns harmless compatibility responses so websites continue working while ads/tracking remain blocked.

Targets: desktops, phones, tablets, TVs/browser devices where extensions are unavailable.

Distributions:
- static Go binaries
- OCI container
- Helm chart
- config via koanf: `file < env < CLI`
- Linux first; amd64 + arm64

Existing projects do not currently provide this complete network-level appliance. tinyShield handles Ad-Shield client-side; its domain collection is MIT-licensed and useful as upstream intelligence. tinyShield itself is MPL-2.0, so avoid copying implementation unless deliberately accepting those obligations.

## Architecture

```text
tinyShield domain feed
        │
        ▼
 domain controller
        │
        ├── AdGuard Home rewrites
        │
clients ─DNS─► sink IP
               │
               ▼
        TLS/SNI termination
               │
        host/path matcher
               │
               ▼
        compatibility emulator
```

Single Go application.

Modules:

```text
cmd/
internal/config
internal/domainfeed
internal/dns
internal/pki
internal/tls
internal/emulator
internal/http
internal/admin
internal/metrics
```

Do not use mitmproxy in production. It is excellent OSS tooling, but introduces Python/runtime complexity and much broader interception capability than required. Implement narrowly in Go using `crypto/tls` + `crypto/x509`; Go supports certificate generation directly.

## PKI

First startup:

```text
Root CA
  └── Intermediate CA
       └── short-lived host certificates
```

Requirements:
- root key generated once
- intermediate signs leaf certificates
- cache leaf certificates by hostname
- only issue for domains present in active domain set
- never operate as arbitrary MITM proxy
- persistent `/data/pki`
- filesystem permissions `0700`
- Kubernetes Secret/PVC support
- optional externally supplied CA later

Public enrollment endpoints:

```text
GET /ca.crt
GET /ca.pem
GET /ca-chain.pem
GET /install
```

`/install` provides platform-specific instructions/download buttons.

Never expose:
- root key
- intermediate key
- leaf private keys

Potential later:
- Apple `.mobileconfig`
- Android-specific certificate download
- QR code pointing at `/install`

## Emulator

Do not start with a generic JavaScript emulator.

Use explicit signatures:

```yaml
emulators:
  adshield:
    hosts:
      - "*.html-load.com"
      - "*.content-loader.com"

    routes:
      - match: "/app.js"
        response: adshield-loader-v1
```

Each compatibility profile controls:
- status
- MIME type
- body
- CORS
- cache headers
- required globals
- callbacks/events

Unknown host/path:

```text
fail closed
```

Do not proxy upstream.

This is important: the service must never accidentally become an advertising proxy.

## Domain intelligence

Primary upstream:

`FilteringDev/tinyShield-domain-collection`

It is MIT licensed and maintained specifically for Ad-Shield domains.

Controller should:
- periodically fetch feed
- validate format
- retain previous known-good version
- diff before activation
- reject malformed/empty updates
- expose feed age/version
- optionally pin commit/version

Never blindly activate a new feed.

## Configuration

Example:

```yaml
server:
  https: ":443"
  admin: ":8080"

data:
  path: /data

pki:
  organization: Local Anti-AdShield
  leafTTL: 24h

feed:
  url: ...
  interval: 6h

dns:
  provider: adguardhome
  target: 10.10.8.250

adguard:
  url: https://adguard.local
  username: ...
  passwordFile: /run/secrets/adguard-password

metrics:
  enabled: true
```

Precedence:

```text
config.yaml
↓ overwritten by
APP_*
↓ overwritten by
CLI flags
```

Koanf explicitly supports file, environment and flag providers and merges later loads over earlier ones.

## AdGuard Home integration

Two modes.

Preferred:

```text
service → AdGuard Home API → DNS rewrites
```

AGH supports exact and wildcard rewrites through `/control/rewrite/*`.

Alternative:

```text
generate rewrite/filter file
```

Controller owns only entries carrying its own state marker. Never delete unrelated AGH rewrites.

Desired reconciliation:

```text
upstream domains
      ∩
enabled profiles
      ↓
desired rewrites
      ↓
compare AGH
      ↓
add/remove delta
```

## NextDNS

Do not make NextDNS the primary integration.

NextDNS can perform rewrites, including private addresses, but remote devices cannot reach the RFC1918 sink unless connected home through VPN/split DNS. Its rewrite management is also less suitable for a frequently changing generated domain set.

Recommended:

```text
LAN/VPN:
device → AdGuard Home → sink

away without VPN:
device → NextDNS → normal blocking
```

Later: generate NextDNS rewrites as optional integration.

## HTTP protocols

MVP:
- TLS 1.2/1.3
- HTTP/1.1
- HTTP/2

HTTP/3:
- not required initially
- do not advertise `Alt-Svc`
- optionally reject/drop UDP/443 so clients use TCP/443
- add native H3 only if real compatibility testing proves necessary

## API

Minimal:

```text
GET /healthz
GET /readyz
GET /metrics

GET /api/v1/status
GET /api/v1/domains
POST /api/v1/sync

GET /ca.crt
GET /ca.pem
GET /install
```

Admin API/UI must bind separately from intercepted HTTPS traffic.

## Observability

Prometheus:

```text
requests_total{host,profile,result}
tls_cert_issued_total
tls_cert_cache_entries
domain_feed_entries
domain_feed_last_success_timestamp
dns_rewrites_managed
emulator_unknown_path_total
```

Never log:
- query strings
- cookies
- request bodies
- client browsing history

Default logs should contain only operational information.

## Container

- distroless/scratch
- non-root
- read-only root filesystem
- writable `/data`
- no shell
- drop all capabilities
- bind 8443 internally if required
- Service maps 443 → 8443

No privileged container.

## Helm

Chart provides:

```text
Deployment
Service
PVC
Secret/config
ServiceMonitor optional
NetworkPolicy
PodDisruptionBudget optional
```

Security defaults:

```yaml
runAsNonRoot: true
readOnlyRootFilesystem: true
allowPrivilegeEscalation: false
capabilities:
  drop: ["ALL"]
```

CA/PKI data must persist across pod replacement.

Single replica initially because PKI/feed state is local.

## Repository

```text
/
├── cmd/sink/
├── internal/
├── profiles/
│   └── adshield/
├── charts/project/
├── config/example.yaml
├── docs/
│   ├── architecture.md
│   ├── install-ca.md
│   └── integrations/
├── Dockerfile
├── go.mod
└── .github/
```

Compatibility profiles should be data/templates where possible, not hard-coded throughout the application.

## CI/CD

Every PR:
- `go test`
- `go vet`
- lint
- profile tests
- TLS tests
- container build
- Helm lint/template

Release:

```text
tag vX.Y.Z
 ├── linux-amd64
 ├── linux-arm64
 ├── checksums
 ├── SBOM
 ├── OCI image
 └── Helm chart
```

Sign image/artifacts with Cosign.

## Implementation phases

### Phase 0 — Research harness

Build a test harness against representative Ad-Shield sites.

Capture:
- hostname
- requested path
- expected globals
- callbacks
- failure behaviour
- CSS removal behaviour
- redirects

Acceptance: exact minimum compatibility contract documented.

### Phase 1 — MVP

Implement:
- Go daemon
- koanf config
- root/intermediate PKI
- dynamic SNI leaf certs
- CA download
- static domain config
- one Ad-Shield emulator profile
- health endpoints
- container

Manual AGH rewrites.

Acceptance:

```text
DNS rewrite
→ trusted TLS
→ loader satisfied
→ page remains usable
→ ads remain blocked
```

Desktop + Android browser test.

### Phase 2 — Integration

Add:
- tinyShield domain feed
- safe updater
- AGH API reconciliation
- `/install`
- metrics
- arm64
- Helm chart

Acceptance:
- clean install from Helm
- CA survives restart/upgrade
- AGH rules converge automatically
- feed failure does not affect existing configuration

### Phase 3 — Beta

Compatibility matrix:
- Firefox
- Chromium
- Android Chrome
- Android Firefox
- Safari/iOS if available

Test:
- CSP
- CORS
- SRI
- cached loaders
- offline feed
- expired leaf
- CA rotation
- malformed upstream feed
- rollback

Run opt-in in home network.

### Phase 4 — Stable

Gate:
- ≥ several weeks without page-breaking regression
- deterministic rollback
- documented CA recovery
- signed releases
- reproducible container
- SBOM
- security policy
- compatibility tests running continuously

## Upgrade / rollback

Application rollback:

```text
Helm rollback / previous binary
```

Profile/feed rollback:

```text
current.json
previous.json
```

Never couple CA rotation to application upgrades.

CA rotation must be a separate explicit operation because replacing it immediately breaks every enrolled client.

## Important failure behaviour

These defaults matter most:

```text
feed unavailable       → keep old feed
new feed malformed     → reject
unknown hostname       → reject TLS / no interception
unknown path           → safe configured fallback, never proxy
AGH unavailable        → retain current state
emulator regression    → profile can be disabled independently
CA unavailable         → readiness fails
```

## Non-goals

Do not:
- intercept arbitrary websites
- inspect user traffic
- proxy normal HTTPS
- remove arbitrary website scripts
- replace uBlock
- become a generic filtering proxy
- bypass certificate pinning
- support applications that reject user-installed CAs

This keeps the trust boundary substantially smaller than a traditional HTTPS MITM proxy.

## Main technical risk

The difficult component is not TLS or DNS. It is keeping the compatibility profile aligned with Ad-Shield as their loader changes.

Therefore the core design should treat:

```text
engine = stable
domain intelligence = updateable
compatibility profiles = updateable/versioned
```

rather than requiring a new application release every time Ad-Shield changes.

That separation should be the central design decision Claude preserves when decomposing this into work items.

---

## User

@Deep research lets bundle that in a proper brief. DO NOT OVERKILL EVERYTHING. assume that at least a LAN level option exists to rewrite DNS. (otherwise this makes no sense anyway)
the app should not manage the DNS rewrites, they are manged by the user (however they chose)

container spec is good though, thourrough and secure, i like it.
chart looks also good, but we default to traefik ingress, with optin gateway api or legacy ingress.
we also add hpa and vpa and opt in cert manager resources

A volume should not be needed at all

CI for the oss app and chart is already scaffoleded in my gitlab-component lib. no need to go into detail there, claude will know what to do

if possible, I would therefore drop the tineShield domain feed and just mention and shoutout in docs

the rest is nicely outlined

## Assistant

Deep Research has started on the revised project brief. It will incorporate these constraints and return the finished brief in the research report.

### Deep Research report

# Existing Bypass Projects and Domain Lists

- **Userscripts:** e.g. *List‑KR/microShield* (Apache‑2.0) and *FilteringDev/tinyShield* (MPL‑2.0) are open-source anti–AdShield scripts that neutralize the in-page loader logic. Seia‑soto’s *AdShield Defuser* library (TypeScript) contains the core decoder used by these scripts.
- **Domain feeds:** *FilteringDev/tinyShield-domain-collection* publishes daily snapshots of known Ad‑Shield domains (MIT license). Examples include `html-load.com`, `content-loader.com`, `error-report.com`, `css-load.com`, etc.

# PKI Design (Local CA + Leaf Certs)

- **Root and Intermediate:** Generate a single *root CA* and use it to sign an *intermediate CA*, then issue short-lived leaf TLS certificates on-the-fly for each request’s SNI.
- **Leaf certs:** Use Go’s `tls.Config.GetCertificate` hook to issue a leaf cert per hostname. Use a strong key and appropriate SANs. Keep leaf certs ephemeral.
- **Storage:** Persist CA identity in a Kubernetes Secret; no PVC is required. Allow an externally supplied CA/keys for migration.
- **HTTPS:** Serve all intercepted domains over HTTPS using this CA. Clients must trust the root CA.

# Loader-Script Emulation Contract

- Intercept only explicitly configured AdShield loader domains.
- For JS requests, serve a harmless script that establishes the expected globals/state.
- For CSS or image endpoints, return minimal valid content as needed.
- For `error-report` or similar endpoints, return harmless successful responses where required.
- Preserve correct MIME types, status codes, CORS and cache behavior.

# Kubernetes Deployment

- Minimal non-root container.
- Read-only root filesystem.
- Drop all capabilities.
- No privilege escalation.
- Static Go binary.
- No PVC.
- Kubernetes Secret for stable CA identity.
- Traefik as default ingress option.
- Opt-in Gateway API and legacy Ingress resources.
- Optional HPA/VPA resources.
- Optional cert-manager resources for normal admin/UI ingress TLS where appropriate.
- Interception TLS still uses the locally trusted CA, not public ingress certificates.

# Rollout & Device Enrollment

- Expose a CA download/install page.
- Document Android, iOS/iPadOS, Windows, macOS and Linux trust installation.
- Optionally support MDM/profile-based rollout later.

# References / Prior Art

- microShield
- tinyShield
- tinyShield-domain-collection
- AdShield Defuser
- Caddy local CA behavior as architectural reference

---

## User

export this chat to markdown
