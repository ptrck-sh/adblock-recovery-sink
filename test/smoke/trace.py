import json
import os
import sys
import time
from urllib.parse import urlsplit

from playwright.sync_api import sync_playwright

STATE_JS = """() => ({
  sheets: document.styleSheets.length,
  cookie: (document.cookie.match(/__adblocker=([^;]*)/) || [])[1] || null,
  iframes: [...document.querySelectorAll('iframe')].map(f => f.src).filter(s => /error-report|html-load|content-load/.test(s)).length,
  title: document.title.slice(0, 60),
})"""


def main():
    url, engine, out, profile = sys.argv[1:5]
    seconds = int(os.environ.get("SMOKE_SECONDS", "25"))
    proxy = os.environ.get("SMOKE_PROXY", "http://127.0.0.1:3128")
    events = []
    start = time.time()

    def log(kind, **fields):
        events.append({"t": round(time.time() - start, 2), "kind": kind, **fields})

    with sync_playwright() as p:
        ctx = getattr(p, engine).launch_persistent_context(
            profile,
            headless=True,
            viewport={"width": 1280, "height": 900},
            proxy={"server": proxy},
            env={**os.environ, "HOME": profile + "-home"},
        )
        page = ctx.pages[0] if ctx.pages else ctx.new_page()
        page.on("request", lambda r: log("req", url=r.url[:300], type=r.resource_type))
        page.on("requestfailed", lambda r: log("fail", url=r.url[:300], err=r.failure))
        page.on("pageerror", lambda e: log("pageerror", text=str(e)[:300]))
        page.on("dialog", lambda d: (log("dialog", type=d.type, text=d.message[:300]), d.dismiss()))
        page.on("framenavigated", lambda f: log("nav", main=f == page.main_frame, url=f.url[:300]))
        try:
            page.goto(url, wait_until="domcontentloaded", timeout=60000)
        except Exception as e:
            log("goto-error", text=str(e)[:300])
        for _ in range(seconds):
            try:
                log("state", **page.evaluate(STATE_JS))
            except Exception as e:
                log("state-error", text=str(e)[:200])
            page.wait_for_timeout(1000)
        log("final", url=page.url[:300])
        ctx.close()

    loaders = sorted({urlsplit(e["url"]).hostname for e in events if e["kind"] == "req" and urlsplit(e["url"]).path.endswith("/loader.min.js")})
    states = [e for e in events if e["kind"] == "state"]
    summary = {
        "loaderHosts": loaders,
        "dialogs": sum(e["kind"] == "dialog" for e in events),
        "mainNavigations": sum(e["kind"] == "nav" and e["main"] for e in events),
        "last": states[-1] if states else None,
    }
    with open(out, "w") as f:
        json.dump({"summary": summary, "events": events}, f, indent=1)
    print(json.dumps(summary))


if __name__ == "__main__":
    main()
