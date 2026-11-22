# docker-warp-relay

A Cloudflare WARP relay running in a Docker container

## Usage (Docker Compose)

```yaml
services:
  warp-relay:
    image: ghcr.io/sh1shd/docker-warp-relay:latest
    container_name: warp-relay
    restart: unless-stopped
    cap_add:
      - NET_ADMIN
    sysctls:
      net.ipv4.ip_forward: "1"
    ports:
      - "4500:4500/udp"
```