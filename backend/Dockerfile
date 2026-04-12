FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/lazyops-backend ./cmd/server \
    && CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/lazyops-build-worker ./cmd/build-worker

FROM alpine:3.21

ARG TARGETARCH

RUN set -eux; \
    apk add --no-cache ca-certificates tzdata git docker-cli curl; \
    case "${TARGETARCH:-amd64}" in \
      amd64) NIXPACKS_ARCH="x86_64" ;; \
      arm64) NIXPACKS_ARCH="aarch64" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    curl -fsSL "https://github.com/railwayapp/nixpacks/releases/download/v1.34.1/nixpacks-v1.34.1-${NIXPACKS_ARCH}-unknown-linux-musl.tar.gz" \
      | tar xzf - -C /usr/local/bin --strip-components=1 nixpacks; \
    chmod +x /usr/local/bin/nixpacks; \
    addgroup -S lazyops; \
    adduser -S -D -H -h /app -G lazyops lazyops

WORKDIR /app

COPY --from=builder /out/lazyops-backend /app/lazyops-backend
COPY --from=builder /out/lazyops-build-worker /app/lazyops-build-worker

USER lazyops

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null || exit 1

ENTRYPOINT ["/app/lazyops-backend"]
