# ── Stage 1: Build Go backend ─────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder


WORKDIR /build
COPY app/server/go.mod app/server/go.sum ./
RUN go mod download

COPY app/server/ ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gospeak .

# ── Stage 2: Build frontend SPA ──────────────────────────────────────────
FROM scratch AS web-builder
COPY app/web/dist /dist

# ── Stage 3: Production ──────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=go-builder /gospeak /app/gospeak
COPY --from=web-builder /dist /app/static

RUN mkdir -p /app/db /app/logs /app/uploads \
    && chown -R app:app /app

ENV GIN_MODE=release
ENV SERVER_PORT=8998
ENV DB_PATH=/app/db/app.db

EXPOSE 8998

VOLUME ["/app/db", "/app/logs", "/app/uploads", "/app/static"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8998/ping || exit 1

USER app

# 运行时配置走 env_file / 环境变量，不把密钥打进镜像
ENTRYPOINT ["/app/gospeak", "server", "-e", "prod"]
