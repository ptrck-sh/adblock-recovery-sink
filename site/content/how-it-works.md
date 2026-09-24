+++
title = "How it works"
description = "The supported loader contract and the TLS request flow."
weight = 2
+++

The bundled Ad-Shield light profile serves the same stub as `/loader.min.js`, `/app.js`, `/vendor.js` and `/main.js` on `html-load.com`, `fb.html-load.com`, `1.s.html-load.com`, `3.s.html-load.com`, `8.s.html-load.com`, `content-loader.com`, `fb.content-loader.com`, `1.content-loader.com`, `2.content-loader.com`, `js-loader.com`, `css-load.com`, `d37j8pfxu2iogi.cloudfront.net`, and `dkyerkk91s4fa.cloudfront.net`. Sites name the loader differently, but every observed variant runs the same light handshake. See [Compatibility](@/compatibility.md) for the observed loader contract and test results.

The page's recovery code loads that script after its blocked detection path. The replacement replies to same-window request messages that the recovery code expects. It loads no additional resources.

## Request flow

1. Your DNS resolver rewrites the requested loader host to the sink.
2. The browser opens TLS with that host as SNI.
3. The sink accepts only configured SNI names and issues a short-lived leaf certificate from your intermediate CA.
4. The sink matches `GET` or `HEAD` on one of the loader paths and returns the bundled stub with `Cache-Control: no-store`.
5. The page receives the expected reply and does not enter its loader-failure path.

TLS is required because the page requests HTTPS. A public certificate cannot validate for an intercepted third-party name, so the device must trust a CA you operate. The generated root and intermediate both carry critical DNS name constraints, limiting certificates to the names you supplied.

## Host scope

The bundled profile supports the listed loader hosts. It does not serve `essential` mode or unrelated loader integrations. Unknown SNI is rejected during TLS; an allowed host with an unknown path receives `404`.
