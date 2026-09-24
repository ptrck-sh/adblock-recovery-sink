+++
title = "DNS setup"
description = "Rewrite the supported host to the sink and roll back safely."
weight = 7
+++

Create resolver rewrites for `html-load.com`, `*.html-load.com`, `content-loader.com`, `*.content-loader.com`, `js-loader.com`, `*.js-loader.com`, `css-load.com`, `*.css-load.com`, `d37j8pfxu2iogi.cloudfront.net`, and `dkyerkk91s4fa.cloudfront.net` after the sink is reachable and devices trust its root CA.

In AdGuard Home, add a DNS rewrite for each entry to the sink's IPv4 or IPv6 address; `*.html-load.com` matches every subdomain but not `html-load.com` itself, so keep both. If your resolver has no wildcard rewrites, list the subdomains you need individually. In NextDNS, add a rewrite for each host to the sink address or to a CNAME such as `sink.example.com`.

When using direct addresses, keep A and AAAA answers consistent: every address the client can select must reach a sink that presents the constrained certificate. When using a CNAME, the target must resolve consistently as well.

Resolver, browser, and operating-system caches can retain the old answer or a previously loaded script. Wait for TTL expiry or clear applicable site data before judging the change. Secure DNS or Private DNS can bypass the resolver that holds the rewrite.

To roll back, remove the rewrite. Clients will return to the normal DNS answer after caches expire.
