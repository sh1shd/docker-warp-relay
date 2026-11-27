FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src
COPY go.mod ./
COPY . .

ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/udp-relay .

FROM golang:1.24-alpine

RUN apk add --no-cache ca-certificates \
 && adduser -D -H -u 10001 relay

COPY --from=builder /out/udp-relay /usr/local/bin/udp-relay

USER relay
ENTRYPOINT ["/usr/local/bin/udp-relay"]