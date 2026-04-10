# syntax=ghcr.io/docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/lazyops-agent ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata iptables \
    && addgroup -S lazyops \
    && adduser -S -D -H -h /var/lib/lazyops-agent -G lazyops lazyops \
    && mkdir -p /var/lib/lazyops-agent \
    && chown -R lazyops:lazyops /var/lib/lazyops-agent

COPY --from=builder /out/lazyops-agent /usr/local/bin/lazyops-agent

USER lazyops

VOLUME ["/var/lib/lazyops-agent"]

ENTRYPOINT ["/usr/local/bin/lazyops-agent"]
