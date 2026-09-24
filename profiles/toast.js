const adblockRecoveryToastScript = document.currentScript;
(function (options) {
  try {
    const script = adblockRecoveryToastScript;
    let shown = false;
    const text = (value) => document.createTextNode(value);
    const show = () => {
      try {
        const add = () => {
          try {
            if (!document.body) return;
            const host = document.createElement("div");
            const shadow = host.attachShadow({ mode: "closed" });
            const toast = document.createElement("div");
            host.style.position = "fixed";
            host.style.top = "16px";
            host.style.right = "16px";
            host.style.zIndex = "2147483647";
            host.style.pointerEvents = "auto";
            toast.style.fontFamily = "system-ui, sans-serif";
            toast.style.fontSize = "13px";
            toast.style.lineHeight = "1.4";
            toast.style.padding = "10px 12px";
            toast.style.borderRadius = "8px";
            toast.style.border = "1px solid rgba(127, 127, 127, 0.35)";
            toast.style.boxShadow = "0 3px 12px rgba(0, 0, 0, 0.2)";
            toast.style.backgroundColor = "#ffffff";
            toast.style.color = "#222222";
            toast.style.cursor = "pointer";
            toast.style.maxWidth = "280px";
            toast.role = "status";
            toast.ariaLive = "polite";
            if (matchMedia("(prefers-color-scheme: dark)").matches) {
              toast.style.backgroundColor = "#242424";
              toast.style.color = "#f4f4f4";
            }
            toast.appendChild(text("Adblock recovery neutralized"));
            if (options && options.details && script && script.src) {
              const source = new URL(script.src);
              const details = document.createElement("div");
              details.style.marginTop = "3px";
              details.style.opacity = "0.75";
              details.appendChild(text(source.host + source.pathname));
              toast.appendChild(details);
            }
            const dismiss = () => host.remove();
            toast.addEventListener("click", dismiss);
            shadow.appendChild(toast);
            document.body.appendChild(host);
            setTimeout(dismiss, 6000);
          } catch (_) {}
        };
        if (document.body) add();
        else document.addEventListener("DOMContentLoaded", add, { once: true });
      } catch (_) {}
    };
    addEventListener("message", (event) => {
      try {
        if (!shown && event.source === window && typeof event.data === "string" && event.data.endsWith("_as_req")) {
          shown = true;
          show();
        }
      } catch (_) {}
    });
  } catch (_) {}
})
