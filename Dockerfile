FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src
COPY relay.go .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/udp-relay ./relay.go

FROM debian:12-slim

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --no-create-home --uid 10001 relay

COPY --from=builder /out/udp-relay /usr/local/bin/udp-relay

USER relay
ENTRYPOINT ["/usr/local/bin/udp-relay"]
