#!/usr/bin/env bash
# 本地 LiveKit 开发启动脚本（宿主机二进制，不依赖 Docker）
#
#   ./scripts/dev-livekit.sh
#
# 二进制来源：go build ./cmd/server（见 .local/bin/livekit-server）。
# 如需用 Docker，见 deploy/livekit/livekit.docker.yaml 顶部注释里的 docker run 命令。
set -euo pipefail

cd "$(dirname "$0")/.."

BIN=".local/bin/livekit-server"
CONFIG="deploy/livekit/livekit.local.yaml"

if [[ ! -x "$BIN" ]]; then
	echo "未找到 $BIN，请先编译："
	echo "  git clone --depth 1 --branch v1.13.11 https://github.com/livekit/livekit.git /tmp/lk-src"
	echo "  cd /tmp/lk-src && GOPROXY=https://goproxy.cn,direct go build -o \"$PWD/$BIN\" ./cmd/server"
	exit 1
fi

if lsof -nP -iTCP:7880 -sTCP:LISTEN >/dev/null 2>&1; then
	echo "7880 已被占用，LiveKit 可能已在运行"
	exit 1
fi

# 密钥从 deploy/.env 读取，保证与后端签发 token 用的值一致。
# livekit.local.yaml 本身不含凭据（LiveKit 1.13 起 webhook.api_key 必填，
# 且必须是 keys 中存在的 key，缺失会直接启动失败）。
# 统一配置入口：仓库根的 deploy/.env
ENV_FILE="deploy/.env"
if [[ ! -f "$ENV_FILE" ]]; then
	echo "未找到 deploy/.env，请先执行：cd deploy && ./init.sh"
	exit 1
fi

LIVEKIT_KEY=$(grep -E '^LIVEKIT_KEY=' "$ENV_FILE" | tail -1 | cut -d= -f2- | tr -d '"'"'"' ')
LIVEKIT_SECRET=$(grep -E '^LIVEKIT_SECRET=' "$ENV_FILE" | tail -1 | cut -d= -f2- | tr -d '"'"'"' ')

if [[ -z "$LIVEKIT_KEY" || -z "$LIVEKIT_SECRET" ]]; then
	echo "$ENV_FILE 里缺少 LIVEKIT_KEY / LIVEKIT_SECRET"
	exit 1
fi

export LIVEKIT_KEYS="$LIVEKIT_KEY: $LIVEKIT_SECRET"
export LIVEKIT_WEBHOOK_API_KEY="$LIVEKIT_KEY"

exec "$BIN" --config "$CONFIG"
