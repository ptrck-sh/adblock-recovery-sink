+++
title = "Chrome Local Network Access"
description = "Chrome permissions for a public page that reaches a private sink address."
weight = 8
+++

Chrome can classify a request as Local Network Access when a public page reaches the sink through a loopback or private address. A DNS rewrite to a LAN sink can therefore show a per-site Allow prompt.

Allow access for each affected site on each device user. If the permission is denied, the loader request fails and the page can break as it would without the sink. After a denial or ignored prompt, Chrome can embargo the site and report that it cannot ask again; resetting site permissions may not immediately remove that embargo.

To avoid the prompt, make the sink reachable through an address Chrome treats as public. Publish a DNS-only record for your own name, rewrite `html-load.com` to a CNAME for that name, and forward TCP `443` to the TLS-passthrough sink. LAN clients need NAT loopback, and the firewall must allow the looped-back sources. A TLS-terminating CDN cannot work because the TLS connection must reach the sink with `html-load.com` SNI.

Managed Chrome deployments can use the `LocalNetworkAccessAllowedForUrls` policy for the affected page URLs.
