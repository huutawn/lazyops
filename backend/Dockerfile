FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/lazyops-backend ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S lazyops \
    && adduser -S -D -H -h /app -G lazyops lazyops

WORKDIR /app

COPY --from=builder /out/lazyops-backend /app/lazyops-backend

USER lazyops

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/api/v1/health >/dev/null || exit 1

ENTRYPOINT ["/app/lazyops-backend"]
