# docker-warp-relay

The project is based on the `simple.sh` script from the repository https://github.com/openwarpkit/warp-relay. The script has been reworked so that the iptables rules live inside the container's own network namespace rather than in the host's network space. This is necessary because Docker on the host rewrites its own iptables rules and wipes out relay rules written to the host.

## 1. Purpose

The relay accepts UDP traffic on the container port `SRC_PORT` and forwards it to the external server `DST_HOST:DST_PORT` (a corporate WireGuard server or Cloudflare WARP). Clients connect to the host's address, not to the external server directly.

Traffic path:

```
client → host:PORT_HOST/udp → (Docker DNAT on the host) → container:SRC_PORT
       → (DNAT to DST_HOST:DST_PORT + MASQUERADE in the container)
       → (Docker MASQUERADE on the host) → DST_HOST:DST_PORT
```

The double NAT (container and host) is required. Without MASQUERADE inside the container, replies will never return.

## 2. Project files

All files are in one directory (referred to below as `<project>`):

- `Dockerfile` — a `debian:12-slim` image with the `iptables` package; copies `relay.sh` to `/usr/local/bin/relay.sh` and makes it executable; `ENTRYPOINT` is this script.
- `docker-compose.yml` — definition of the `warp-relay` service: capability, sysctl, ports, and environment variables. All parameters are set here.
- `relay.sh` — the entrypoint. It resolves `DST_HOST`, checks `ip_forward`, flushes the `nat` and `FORWARD` tables inside the container, applies the rules, then loops on `sleep` and shuts down cleanly on SIGTERM.

`docker-compose.yml` is the single source of truth for parameters. Do not hardcode values in `relay.sh`.

## 3. Current configuration (reference)

```yaml
ports: "2288:2288/udp"
environment:
  SRC_PORT: "2288"
  DST_HOST: "40.30.20.10"
  DST_PORT: "51820"
```

Clients connect to `HOST_IP:2288/udp`. If these values have changed, check `docker-compose.yml` first rather than relying on this section.

## 4. Hard rules (never violate)

1. Do not run `iptables`, `nft`, or `sysctl -w` on the host to configure the relay. Relay rules must exist only inside the container.
2. Do not use `network_mode: host`. This would put the rules back into the host's netns, and Docker would wipe them again.
3. Do not run `iptables -F`, `iptables -t nat -F`, or similar commands on the host. This breaks Docker's networking and has nothing to do with the relay.
4. Do not change `ip_forward` with `sysctl -w` inside the container, and do not write to `/etc/sysctl.d` on the host. Forwarding inside the container is enabled only through `sysctls:` in compose. `/proc/sys` inside the container is read-only.
5. Do not remove `cap_add: NET_ADMIN`. Without it, `iptables` inside the container does not work.
6. Do not add `-d <IP>` to the DNAT rule in `relay.sh`. The container does not know the host's public IP, and the rule must catch all UDP on `SRC_PORT`.
7. Do not add package installation, `ifconfig.me`, `netfilter-persistent`, or writes to the host's `/etc` to `relay.sh`. Package installation is already handled in the `Dockerfile`, and rule persistence is unnecessary: the rules are re-applied whenever the container restarts.
8. Any change to `relay.sh` requires a rebuild: `docker compose up -d --build`. Changing only `environment` requires recreating the container: `docker compose up -d`. A plain `docker compose restart` will not pick up new variables.
9. Save files with LF line endings. CRLF breaks the `#!/bin/bash` shebang and prevents the script from running.
10. Do not change the base image version without a reason to verify it. `debian:12-slim` was chosen because its `iptables` uses the nft backend, which the host kernel supports.

## 5. Environment variables

| Variable | Default (in relay.sh) | Description |
|---|---|---|
| `SRC_PORT` | `4500` | UDP port inside the container that DNAT catches clients on |
| `DST_PORT` | `4500` | UDP port of the target server |
| `DST_HOST` | `engage.cloudflareclient.com` | IPv4 address or hostname of the target server. If an IP is given, no resolution is performed |

In `docker-compose.yml`, the host publish port (left side of `ports`) and `SRC_PORT` (container port) may differ, but `SRC_PORT` must match the right side of `ports`.

## 6. Procedures

### 6.1. First start or update
```bash
cd <project>
docker compose up -d --build
docker compose logs --tail=50
```
Expected log lines: `udp/<SRC_PORT> -> <DST_IP>:<DST_PORT> is set up.` (configured), followed by the output of `iptables -t nat -S` and `iptables -S FORWARD`.

### 6.2. Changing the target server or port
1. Change `DST_HOST`, `DST_PORT`, or `SRC_PORT` in `docker-compose.yml`.
2. If `SRC_PORT` changed, update the right side of `ports` to match.
3. Run `docker compose up -d` (recreates the container).
4. Check the logs and rules (section 6.3).

### 6.3. Checking the state
```bash
docker compose ps
docker compose exec warp-relay iptables -t nat -S
docker compose exec warp-relay iptables -S FORWARD
docker compose exec warp-relay cat /proc/sys/net/ipv4/ip_forward     # must be 1
docker compose exec warp-relay iptables -t nat -L -n -v              # counters grow with traffic
```

### 6.4. Stopping and removing
```bash
docker compose down
```
The rules inside the container disappear along with it. Nothing needs to be rolled back on the host, because the host was never modified. If the user asks to "roll back" the setup, stopping the container is enough. Clearing iptables on the host is neither needed nor safe.

### 6.5. Autostart after a host reboot
Handled by `restart: unless-stopped` in compose. No separate systemd units or changes under `/etc` are needed.

## 7. Troubleshooting

| Symptom | Cause | Action |
|---|---|---|
| Logs show `ERROR: net.ipv4.ip_forward=0` | `sysctls` not set in compose | Add `sysctls: net.ipv4.ip_forward: "1"`, run `up -d` |
| Logs show `ERROR: failed resolve resolve` (cannot resolve) | No DNS in the container or wrong name | Set the IP in `DST_HOST`, check the host's DNS |
| `docker compose up` fails on the port | Host port already in use (e.g., IPsec NAT-T on 4500) | Change the left side of `ports` and the client endpoint |
| Counters in `iptables -t nat -L -n -v` do not grow | Client is not sending traffic to the right host port | Check the client endpoint and the publish port |
| Traffic goes out but no reply | Outbound traffic blocked, or DNAT did not match | Check `iptables -S FORWARD` and the target server's firewall |
| Rules disappeared after Docker restart | Container recreated without the entrypoint running | Run `docker compose up -d`; rules are re-applied |
| Exit code 127 or `bad interpreter` in logs | CRLF line endings in `relay.sh` | Convert the file to LF and rebuild the image |
| Docker on the host has `"iptables": false` | Host does not masquerade the container's outbound traffic | Configure NAT/forwarding on the host manually (outside the relay's scope) |

## 8. What not to do

- Do not diagnose the relay through host rules (`iptables -t nat -L` without the container). Inspect inside the container (`docker compose exec warp-relay ...`).
- Do not move relay rules into `/etc/iptables` or `netfilter-persistent` on the host.
- Do not run several containers that publish the same host port.
- Do not commit private IPs or keys to the repository if the project is published. `DST_HOST` and `DST_PORT` are set in the local `docker-compose.yml`.

## 9. Action sequence for an AI agent

1. Read this file in full.
2. Read `docker-compose.yml` and `relay.sh` in `<project>`. Do not rely on the values in this document.
3. Before changing network configuration, verify that host rules are not affected (section 4).
4. After any change, run the checks in section 6.3 and report the result to the user.
