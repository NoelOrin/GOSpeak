# ── Stage 1: Frontend SPA ────────────────────────────────────────────────
FROM node:22-alpine AS web-builder
WORKDIR /src
RUN corepack enable && corepack prepare pnpm@10.11.0 --activate
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY app/web/package.json app/web/
COPY packages/sfu-client/package.json packages/sfu-client/
RUN pnpm install --frozen-lockfile --filter @gospeak/web...
COPY app/web app/web
COPY packages/sfu-client packages/sfu-client
RUN pnpm --filter @gospeak/web build

# ── Stage 2: Go backend (embed frontend) ─────────────────────────────────
FROM golang:1.26-alpine AS go-builder
WORKDIR /build
COPY app/server/go.mod app/server/go.sum ./
RUN go mod download
COPY app/server/ ./
# 将前端产物同步到 go:embed 目录，打进二进制
COPY --from=web-builder /src/app/web/dist/ ./internal/webui/dist/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gospeak .

# ── Stage 3: Production ──────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=go-builder /gospeak /app/gospeak

RUN mkdir -p /app/db /app/logs /app/uploads \
    && chown -R app:app /app

ENV GIN_MODE=release
ENV SERVER_PORT=8998
ENV DB_PATH=/app/db/app.db

EXPOSE 8998

VOLUME ["/app/db", "/app/logs", "/app/uploads"]

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8998/ping || exit 1

USER app

# 运行时配置走 env_file / 环境变量，不把密钥打进镜像
ENTRYPOINT ["/app/gospeak", "server", "-e", "prod"]
