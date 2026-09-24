import json
import os
import sys
from pathlib import Path

from playwright.sync_api import sync_playwright

TOAST_JS = """() => [...document.body.children].some(e => e.style.zIndex === "2147483647" && e.style.position === "fixed")"""


def main():
    fixture, profile, rules, src, toast = sys.argv[1:6]
    url = Path(fixture).resolve().as_uri() + "?src=" + src
    with sync_playwright() as p:
        ctx = p.chromium.launch_persistent_context(
            profile,
            headless=True,
            args=["--host-resolver-rules=" + rules],
            env={"HOME": profile + "-home"},
            color_scheme=os.environ.get("E2E_COLOR_SCHEME", "light"),
        )
        page = ctx.pages[0] if ctx.pages else ctx.new_page()
        page.goto(url)
        page.wait_for_function("() => document.title !== 'pending'", timeout=15000)
        result = {"src": src, "title": page.title(), "toast": None}
        if toast == "toast":
            try:
                page.wait_for_function(TOAST_JS, timeout=5000)
                result["toast"] = True
                if os.environ.get("E2E_SCREENSHOT"):
                    page.screenshot(path=os.environ["E2E_SCREENSHOT"], clip={"x": 880, "y": 0, "width": 400, "height": 120})
            except Exception:
                result["toast"] = False
        ctx.close()
    print(json.dumps(result))
    ok = result["title"] == "ok" and result["toast"] in (None, True)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
