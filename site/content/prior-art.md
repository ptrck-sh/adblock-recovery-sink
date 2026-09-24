+++
title = "Prior art"
description = "Related work that informed the supported loader scope."
weight = 14
+++

[tinyShield](https://github.com/FilteringDev/tinyShield) documents the same Ad-Shield domains and defuses the loader in the browser. This project uses no tinyShield code or data.

[Recipe Blogs Are A Test Bed for Aggressive Ad-Blocker Defeating Techniques](https://jacobdesforges.com/adshield-ad-reinsertion/) by Jacob Desforges traces Ad-Shield's rotating domains (`html-load.com`, `content-loader.com`, `js-loader.com`, `css-load.com` and numbered subdomains), the `error-report.com` dialogs and the ad reinsertion that follows. The host list in the bundled profile starts from it.

[Websites crash, blame ad blockers: a hidden war in the digital world](https://adguard.com/en/blog/ad-blockers-website-crash-blame.html) by Ekaterina Kachalova at AdGuard describes the stylesheet removal (`document.querySelectorAll('link,style').forEach((e)=>e.remove())`) and the messages that blame the ad blocker for the breakage.
