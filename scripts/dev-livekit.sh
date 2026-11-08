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

exec "$BIN" --config "$CONFIG"
