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
      amd64) NIXPACKS_ARCH="x86_64"; BUILDX_ARCH="amd64" ;; \
      arm64) NIXPACKS_ARCH="aarch64"; BUILDX_ARCH="arm64" ;; \
      *) echo "unsupported TARGETARCH: ${TARGETARCH}" >&2; exit 1 ;; \
    esac; \
    if ! docker buildx version >/dev/null 2>&1; then \
      mkdir -p /usr/libexec/docker/cli-plugins; \
      curl -fsSL "https://github.com/docker/buildx/releases/download/v0.21.1/buildx-v0.21.1.linux-${BUILDX_ARCH}" \
        -o /usr/libexec/docker/cli-plugins/docker-buildx; \
      chmod +x /usr/libexec/docker/cli-plugins/docker-buildx; \
    fi; \
    TMP_DIR="$(mktemp -d)"; \
    curl -fsSL "https://github.com/railwayapp/nixpacks/releases/download/v1.34.1/nixpacks-v1.34.1-${NIXPACKS_ARCH}-unknown-linux-musl.tar.gz" -o /tmp/nixpacks.tgz; \
    tar xzf /tmp/nixpacks.tgz -C "${TMP_DIR}"; \
    NIXPACKS_BIN="$(find "${TMP_DIR}" -type f -name nixpacks | head -n1)"; \
    test -n "${NIXPACKS_BIN}"; \
    install -m 0755 "${NIXPACKS_BIN}" /usr/local/bin/nixpacks; \
    rm -rf "${TMP_DIR}" /tmp/nixpacks.tgz; \
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
