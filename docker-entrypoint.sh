#!/usr/bin/env bash
set -euo pipefail
shopt -s nullglob

if [[ ! -f /usr/local/zeek/etc/node.cfg ]] || [[ ! -s /usr/local/zeek/etc/node.cfg ]]; then
	echo "error: node.cfg is missing or empty. mount it into /usr/local/zeek/etc/node.cfg." >&2
	exit 1
fi

stop() {
	echo "stopping zeek..."
	zeekctl stop
	exit 0
}
trap stop SIGINT SIGTERM

# disable NIC offloading on monitored interfaces for accurate packet capture.
# parse unique interfaces from node.cfg, stripping af_packet:: prefix if present.
# errors are non-fatal since some flags may not be supported on all NICs or VMs.
declare -A seen_ifaces
while IFS='=' read -r key value; do
	[[ "$key" =~ ^[[:space:]]*interface ]] || continue
	iface="${value##af_packet::}"
	iface="${iface// /}"
	[[ -n "$iface" ]] || continue
	[[ -z "${seen_ifaces[$iface]+x}" ]] || continue
	seen_ifaces[$iface]=1
	if ethtool -K "$iface" rx off tx off sg off tso off gso off gro off lro off >/dev/null 2>&1; then
		echo "disabled NIC offloading on $iface"
	else
		echo "warning: failed to disable some offloading features on $iface (this may be expected in virtual environments)" >&2
	fi
done < /usr/local/zeek/etc/node.cfg

# build local.zeek from autoload scripts, stripping comments
autoload=(/usr/local/zeek/share/zeek/site/autoload/*.zeek)
if [[ ${#autoload[@]} -eq 0 ]]; then
	echo "error: no .zeek scripts found in autoload directory" >&2
	exit 1
fi
grep -hv '^#' "${autoload[@]}" > /usr/local/zeek/share/zeek/site/local.zeek

zeekctl check || { echo "error: zeek configuration check failed" >&2; exit 1; }
zeekctl install || { echo "error: zeekctl install failed" >&2; exit 1; }
zeekctl start || { echo "error: zeekctl start failed" >&2; exit 1; }

crond -b -L /dev/fd/1 || { echo "error: failed to start crond" >&2; exit 1; }
echo "cron enabled"

# keep the container running while remaining responsive to signals.
# backgrounding sleep and using wait allows bash to process SIGTERM traps.
# without this, bash blocks on the foreground sleep and never runs the trap.
sleep infinity &
wait $!
