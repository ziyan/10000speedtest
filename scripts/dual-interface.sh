#!/usr/bin/env bash
#
# Run 10000speedtest across several network interfaces in parallel and report
# the combined throughput. This only increases the total when the interfaces
# have independent bandwidth up to a shared uplink that is faster than one NIC.
#
# Each interface must be usable for internet traffic on its own — i.e. source
# routing must send that interface's IP out that interface. See the README.
#
# Usage:   scripts/dual-interface.sh <iface1> <iface2> [iface3 ...]
# Env:     SPEEDTEST_BIN (default: 10000speedtest), MODE (both|download|upload),
#          DURATION (default: 10s), CONNECTIONS (default: 8)
set -euo pipefail

binary=${SPEEDTEST_BIN:-10000speedtest}
mode=${MODE:-both}
duration=${DURATION:-10s}
connections=${CONNECTIONS:-8}

if [ "$#" -lt 1 ]; then
	echo "usage: $0 <iface1> <iface2> [iface3 ...]" >&2
	exit 2
fi

workDirectory=$(mktemp -d)
trap 'rm -rf "$workDirectory"' EXIT

for networkInterface in "$@"; do
	"$binary" --interface "$networkInterface" --mode "$mode" \
		--duration "$duration" --connections "$connections" --json \
		>"$workDirectory/$networkInterface.json" &
done

exitCode=0
wait || exitCode=1

python3 - "$workDirectory" "$@" <<'PYTHON'
import json, os, sys

workDirectory = sys.argv[1]
interfaces = sys.argv[2:]
downloadTotal = uploadTotal = 0.0

print(f"{'interface':<12}{'download':>16}{'upload':>16}")
for networkInterface in interfaces:
    with open(os.path.join(workDirectory, networkInterface + ".json")) as handle:
        result = json.load(handle)
    download = (result.get("download") or {}).get("mbps", 0.0)
    upload = (result.get("upload") or {}).get("mbps", 0.0)
    downloadTotal += download
    uploadTotal += upload
    print(f"{networkInterface:<12}{download:>11.2f} Mbps{upload:>11.2f} Mbps")

print(f"{'COMBINED':<12}{downloadTotal:>11.2f} Mbps{uploadTotal:>11.2f} Mbps")
PYTHON

exit "$exitCode"
