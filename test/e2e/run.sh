#!/usr/bin/env bash
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
work="${E2E_WORKDIR:-$here/.work}"
port="${E2E_PORT:-18443}"
ops="${E2E_OPS_PORT:-18080}"
sink="${SINK_BIN:-$work/sink}"
urls=(
	https://html-load.com/loader.min.js
	https://fb.html-load.com/vendor.js
	https://7.s.html-load.com/loader.min.js
	https://3.content-loader.com/vendor.js
	https://html-load.com/app.js
	https://content-loader.com/app.js
	https://dkyerkk91s4fa.cloudfront.net/main.js
)

rm -rf "$work"
mkdir -p "$work"
if [ -z "${SINK_BIN:-}" ]; then
	(cd "$repo" && go build -o "$sink" ./cmd/sink)
fi
"$sink" pki init --hosts html-load.com,content-loader.com,dkyerkk91s4fa.cloudfront.net --out "$work/pki" >/dev/null

rules=""
for url in "${urls[@]}"; do
	host="${url#https://}"
	host="${host%%/*}"
	rules="${rules:+$rules, }MAP $host:443 127.0.0.1:$port"
done

ARS_SINK_ADDR="127.0.0.1:$port" \
	ARS_OPS_ADDR="127.0.0.1:$ops" \
	ARS_TOAST_ENABLED=true \
	ARS_TOAST_DETAILS=true \
	ARS_PKI_ROOT_CERT_FILE="$work/pki/root.crt" \
	ARS_PKI_INTERMEDIATE_CERT_FILE="$work/pki/intermediate.crt" \
	ARS_PKI_INTERMEDIATE_KEY_FILE="$work/pki/intermediate.key" \
	"$sink" serve >"$work/sink.log" 2>&1 &
pid=$!
trap 'kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null || true' EXIT
for _ in $(seq 50); do
	kill -0 "$pid" 2>/dev/null || { cat "$work/sink.log" >&2; exit 1; }
	curl -fs "http://127.0.0.1:$ops/readyz" >/dev/null && break
	sleep 0.1
done

profile="$work/profile"
mkdir -p "$profile" "$profile-home/.pki/nssdb"
certutil -N -d "sql:$profile-home/.pki/nssdb" --empty-password
certutil -A -d "sql:$profile-home/.pki/nssdb" -n ars-e2e-root -t "C,," -i "$work/pki/root.crt"

status=0
for url in "${urls[@]}"; do
	uv run -q --with playwright python "$here/check.py" "$repo/test/fixtures/adshield-light/index.html" "$profile" "$rules" "$url" toast || status=1
done
exit "$status"
