#!/usr/bin/env bash
set -euo pipefail

usage() {
	echo "usage: $0 <chromium|firefox> <url> <name> [trust|notrust]" >&2
	exit 1
}

[ $# -ge 3 ] || usage
engine=$1 url=$2 name=$3 trust=${4:-trust}
here="$(cd "$(dirname "$0")" && pwd)"
work="${SMOKE_WORKDIR:-$here/.work}"
root="${SMOKE_ROOT_CA:-$work/root.pem}"
prof="$work/profile-$name"

rm -rf "$prof" "$prof-home"
mkdir -p "$prof" "$prof-home/.pki/nssdb"
if [ "$trust" = trust ]; then
	db="$prof-home/.pki/nssdb"
	[ "$engine" = firefox ] && db="$prof"
	certutil -N -d "sql:$db" --empty-password
	certutil -A -d "sql:$db" -n ars-smoke-root -t "C,," -i "$root"
fi
uv run -q --with playwright python "$here/trace.py" "$url" "$engine" "$work/$name.json" "$prof"
