# ── Stage 1: Build Go backend ─────────────────────────────────────────────
FROM golang:1.26-alpine AS go-builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY app/server/go.mod app/server/go.sum ./
RUN go mod download

COPY app/server/ ./
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /gospeak .

# ── Stage 2: Build frontend SPA ──────────────────────────────────────────
FROM node:22-alpine AS web-builder

RUN corepack enable && corepack prepare pnpm@latest --activate

WORKDIR /build
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY app/web/package.json app/web/package.json
RUN pnpm install --frozen-lockfile --filter @gospeak/web

COPY app/web/ app/web/
RUN cd app/web && pnpm build

# ── Stage 3: Production ──────────────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache sqlite-libs ca-certificates tzdata wget

# 非 root 运行
RUN addgroup -S app && adduser -S -G app app

WORKDIR /app

# Go backend + 环境
COPY --from=go-builder /gospeak /app/gospeak
COPY --from=go-builder /build/.env* /app/app/server/
RUN mkdir -p /app/app/server/db /app/logs \
    && chown -R app:app /app

# Frontend static files
COPY --from=web-builder /build/app/web/dist /app/static

ENV GIN_MODE=release
ENV PORT=8998

EXPOSE 8998

# 数据 + 日志卷
VOLUME ["/app/app/server/db", "/app/logs"]

# 健康检查 (GET /ping, alpine wget)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8998/ping || exit 1

USER app

ENTRYPOINT ["/app/gospeak"]
