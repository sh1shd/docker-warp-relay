# docker-warp-relay

A Cloudflare WARP UDP relay running in a Docker container. The relay uses a userspace UDP proxy, so it does not configure firewall or routing rules inside the container.

## Usage (Docker Compose)

```yaml
services:
  warp-relay:
    image: ghcr.io/sh1shd/docker-warp-relay:latest
    container_name: warp-relay
    restart: unless-stopped
    ports:
      - "4500:4500/udp"
    environment:
      SRC_PORT: "4500"
      DST_HOST: engage.cloudflareclient.com
      DST_PORT: "4500"
```

Only the UDP port publication is required. `cap_add: NET_ADMIN`, `sysctls`, `privileged`, and host IP forwarding are not needed. The container forwards datagrams received on `SRC_PORT` to `DST_HOST:DST_PORT` and sends replies back to the originating client. Each client gets a temporary upstream UDP socket, which expires after two minutes without traffic.

The defaults are `SRC_PORT=4500`, `DST_HOST=engage.cloudflareclient.com`, and `DST_PORT=4500`; the `environment` section can be omitted when using those defaults. `SRC_PORT` must match the published container port.
