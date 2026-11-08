#!/usr/bin/env bash
# =============================================================================
# GOSpeak 部署引导脚本 — 生成 deploy/.env（含随机密钥）
# =============================================================================
# 用法:
#   ./init.sh                     # 生成 LiveKit 档位的 .env（默认）
#   ./init.sh --force             # 覆盖已存在的 .env
# =============================================================================
set -euo pipefail

cd "$(dirname "$0")"

FORCE=0

usage() { sed -n '2,8p' "$0"; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    livekit) shift ;;            # 兼容旧用法：仅 LiveKit 一种档位
    --force) FORCE=1; shift ;;
    -h|--help) usage ;;
    *) echo "未知参数: $1" >&2; usage ;;
  esac
done

if [[ -f .env && $FORCE -ne 1 ]]; then
  echo "deploy/.env 已存在，如需重新生成: ./init.sh --force" >&2
  exit 1
fi

rand() { openssl rand -hex 32; }

echo "==> 生成随机密钥"
JWT_KEY=$(rand)
STORAGE_ENCRYPT_KEY=$(rand)
POSTGRES_PASSWORD=$(rand)
LIVEKIT_KEY="API$(openssl rand -hex 6)"
LIVEKIT_SECRET=$(openssl rand -base64 48 | tr -d '/+=' | cut -c1-43)

echo "==> 写出 deploy/.env (LiveKit 档位)"
cat > .env <<EOF
# 由 init.sh 生成于 $(date '+%Y-%m-%d %H:%M:%S') — SFU: livekit
COMPOSE_PROFILES=livekit,app

GOSPEAK_PORT=8998
HTTP_PORT=80

APP_ENV=prod
DB_TYPE=SQLite
DB_PATH=/app/db/app.db
DB_WAL=true

JWT_KEY=$JWT_KEY
JWT_KEY_TTL=24h

SFU_PROVIDER=livekit
LIVEKIT_HOST=ws://livekit:7880
LIVEKIT_KEY=$LIVEKIT_KEY
LIVEKIT_SECRET=$LIVEKIT_SECRET

POSTGRES_USER=gospeak
POSTGRES_PASSWORD=$POSTGRES_PASSWORD
POSTGRES_DB=gospeak
POSTGRES_PORT=5432

STORAGE_TYPE=local
STORAGE_ENCRYPT_KEY=$STORAGE_ENCRYPT_KEY
EMAIL_ENABLED=false

CORS_ORIGIN=*
EOF

echo "==> 轮换 livekit/livekit.yaml 密钥（与 .env 保持一致）"
sed -i.bak \
  -e "/^keys:/,/^$/ s|^  [A-Za-z0-9]*: .*$|  $LIVEKIT_KEY: $LIVEKIT_SECRET|" \
  livekit/livekit.yaml
rm -f livekit/livekit.yaml.bak

cat <<'EOF'

完成。后续命令（在 deploy/ 目录下执行，或任意目录加 -f deploy/docker-compose.yml）:

  docker compose up -d --build     # 首次启动
  docker compose logs -f gospeak   # 看日志
  curl http://localhost:8998/ping  # 健康检查

提示:
  - 生产环境务必配置 HTTPS（Caddy/Nginx TLS），浏览器麦克风需要安全上下文
  - 数据备份: docker run --rm -v gospeak_gospeak-db:/data -v "$PWD":/bak alpine \
      cp /data/app.db /bak/app-backup-$(date +%F).db
EOF
