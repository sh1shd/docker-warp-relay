#!/bin/bash
set -euo pipefail

SRC_PORT="${SRC_PORT:-4500}"
DST_PORT="${DST_PORT:-4500}"
DST_HOST="${DST_HOST:-engage.cloudflareclient.com}"

log() { echo "[docker-warp-relay] $*"; }

if [[ "$DST_HOST" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    DST_IP="$DST_HOST"
else
    DST_IP="$(getent ahostsv4 "$DST_HOST" | awk '{print $1; exit}')"
fi

if [[ -z "$DST_IP" ]]; then
    log "ERROR: failed resolve $DST_HOST"
    exit 1
fi

if [[ "$(cat /proc/sys/net/ipv4/ip_forward)" != "1" ]]; then
    log "ERROR: net.ipv4.ip_forward is 0, add sysctls in compose file"
    exit 1
fi

iptables -t nat -F
iptables -F FORWARD

iptables -t nat -A PREROUTING -p udp --dport "$SRC_PORT" \
    -j DNAT --to-destination "${DST_IP}:${DST_PORT}"

iptables -t nat -A POSTROUTING -p udp -d "$DST_IP" --dport "$DST_PORT" \
    -j MASQUERADE

iptables -A FORWARD -p udp -d "$DST_IP" --dport "$DST_PORT" -j ACCEPT
iptables -A FORWARD -p udp -s "$DST_IP" --sport "$DST_PORT" -j ACCEPT

log "udp/${SRC_PORT} -> ${DST_IP}:${DST_PORT} is set up."
iptables -t nat -S
iptables -S FORWARD

trap 'log "stopping container"; exit 0' TERM INT
while true; do
    sleep 3600 &
    wait $!
done