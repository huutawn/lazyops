FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/lazyops-agent ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata iptables docker-cli caddy \
    && mkdir -p /var/lib/lazyops-agent

COPY --from=builder /out/lazyops-agent /usr/local/bin/lazyops-agent

VOLUME ["/var/lib/lazyops-agent"]

ENTRYPOINT ["/usr/local/bin/lazyops-agent"]
