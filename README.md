# docker-warp-relay

A Cloudflare WARP relay running in a Docker container.

## Usage (Docker Compose)

```yaml
services:
  warp-relay:
    image: ghcr.io/sh1shd/docker-warp-relay:latest
    restart: unless-stopped
    ports:
      - "4500:4500/udp"
    environment:
      SRC_PORT: "4500"
      DST_HOST: engage.cloudflareclient.com
      DST_PORT: "4500"
```