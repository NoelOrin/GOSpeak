#!/usr/bin/env bash
# =============================================================================
# GOSpeak 部署引导脚本
# =============================================================================
# 职责：
#   1. 以 deploy/.env.example 为模板生成 deploy/.env（含随机密钥）
#   2. 归一 deploy/livekit/livekit.yaml，使其不含任何密钥
#   3. 校验 .env 与 livekit.yaml 的一致性，不一致就报错退出
#
# 用法:
#   ./init.sh                          # 生成 .env 并归一 livekit.yaml
#   ./init.sh --force                  # 覆盖已存在的 .env
#   ./init.sh --check                  # 只做一致性校验，不写任何文件
#   ./init.sh --livekit-url wss://lk.example.com
#                                      # 同时设置浏览器侧 LiveKit 地址
#
# 环境变量覆盖（等价于上面的参数）：
#   LIVEKIT_PUBLIC_URL=wss://lk.example.com ./init.sh
# =============================================================================
set -euo pipefail

cd "$(dirname "$0")"

FORCE=0
CHECK_ONLY=0
LIVEKIT_URL_ARG=""

usage() { sed -n '2,22p' "$0"; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    livekit) shift ;;            # 兼容旧用法：仅 LiveKit 一种档位
    --force) FORCE=1; shift ;;
    --check) CHECK_ONLY=1; shift ;;
    --livekit-url)
      [[ $# -ge 2 ]] || { echo "--livekit-url 需要一个值" >&2; exit 1; }
      LIVEKIT_URL_ARG="$2"; shift 2 ;;
    -h|--help) usage ;;
    *) echo "未知参数: $1" >&2; usage ;;
  esac
done

ENV_FILE=".env"
ENV_TPL=".env.example"
LIVEKIT_YAML="livekit/livekit.yaml"

[[ -f "$ENV_TPL" ]] || { echo "缺少模板 $ENV_TPL" >&2; exit 1; }
[[ -f "$LIVEKIT_YAML" ]] || { echo "缺少 $LIVEKIT_YAML" >&2; exit 1; }

# ── 读取 .env 中某个键的值（去掉引号与行尾空白）─────────────────────────────
env_get() {
  local key="$1" file="$2"
  [[ -f "$file" ]] || return 0
  sed -n "s/^${key}=//p" "$file" | tail -1 | sed -e 's/[[:space:]]*$//' -e 's/^["'\'']//' -e 's/["'\'']$//'
}

# ── 一致性校验 ──────────────────────────────────────────────────────────────
# 校验分两类：
#   A. 仓库结构合规（CI 也跑，不依赖 .env 存在）
#      - livekit.yaml 的 keys 必须为空（{}），不得硬编码密钥
#      - livekit.yaml 的 webhook 不得含明文 api_key
#      - compose 的注入表达式与 .env 键名匹配
#   B. 本地部署就绪（仅当 .env 存在时校验）
#      - .env 的 LIVEKIT_KEY / LIVEKIT_SECRET 非空且非占位值
#      - LIVEKIT_PUBLIC_URL 建议显式配置
check_consistency() {
  local rc=0

  # ── A. 仓库结构合规 ──
  if grep -qE '^\s+API[0-9a-zA-Z]+:\s*\S+' "$LIVEKIT_YAML"; then
    echo "  [x] $LIVEKIT_YAML 仍含硬编码 keys，请改为 keys: {}" >&2; rc=1
  else
    echo "  [ok] $LIVEKIT_YAML 不含硬编码密钥"
  fi

  if grep -qE '^keys:\s*\{\}\s*$' "$LIVEKIT_YAML"; then
    echo "  [ok] $LIVEKIT_YAML 的 keys 为空，由环境变量注入"
  else
    echo "  [x] $LIVEKIT_YAML 的 keys 应为 'keys: {}'" >&2; rc=1
  fi

  if grep -qE '^\s+api_key:\s*\S+' "$LIVEKIT_YAML"; then
    echo "  [x] $LIVEKIT_YAML 的 webhook 仍含明文 api_key，应删除" >&2; rc=1
  else
    echo "  [ok] $LIVEKIT_YAML 的 webhook 不含明文 api_key"
  fi

  if grep -q 'LIVEKIT_KEYS: "\${LIVEKIT_KEY}: \${LIVEKIT_SECRET}"' docker-compose.yml &&
     grep -q 'LIVEKIT_WEBHOOK_API_KEY: "\${LIVEKIT_KEY}"' docker-compose.yml; then
    echo "  [ok] docker-compose.yml 已按 .env 的键名注入 LiveKit 密钥"
  else
    echo "  [x] docker-compose.yml 的 LiveKit 密钥注入表达式与 .env 不匹配" >&2; rc=1
  fi

  # ── B. 本地部署就绪（CI 无 .env，跳过）──
  if [[ ! -f "$ENV_FILE" ]]; then
    echo "  [-] 未找到 $ENV_FILE，跳过部署就绪检查（CI 场景正常）"
    return $rc
  fi

  local key secret
  key="$(env_get LIVEKIT_KEY "$ENV_FILE")"
  secret="$(env_get LIVEKIT_SECRET "$ENV_FILE")"
  if [[ -z "$key" || -z "$secret" ]]; then
    echo "  [x] $ENV_FILE 缺少 LIVEKIT_KEY / LIVEKIT_SECRET" >&2; rc=1
  elif [[ "$key" == "change-me" || "$secret" == "change-me" ]]; then
    echo "  [x] $ENV_FILE 的 LiveKit 密钥仍是占位值 change-me" >&2; rc=1
  else
    echo "  [ok] $ENV_FILE 已配置 LiveKit 密钥（key=${key:0:12}...）"
  fi

  local public_url
  public_url="$(env_get LIVEKIT_PUBLIC_URL "$ENV_FILE")"
  if [[ -z "$public_url" ]]; then
    echo "  [!] LIVEKIT_PUBLIC_URL 为空，将回退到 LIVEKIT_HOST；容器部署下浏览器可能连不上"
  else
    echo "  [ok] LIVEKIT_PUBLIC_URL=$public_url"
  fi

  return $rc
}

# ── 归一 livekit.yaml：剥离密钥，保留静态配置 ───────────────────────────────
normalize_livekit_yaml() {
  local tmp
  tmp="$(mktemp)"

  # 1. keys: 下所有缩进条目 → 替换为 keys: {}
  # 2. webhook 下的 api_key 行 → 删除
  awk '
    # keys 块：把 "keys:" 之后的缩进条目全部吞掉，输出 keys: {}
    /^keys:[[:space:]]*$/ { print "keys: {}"; in_keys=1; next }
    in_keys && /^[[:space:]]+/ { next }
    in_keys && !/^[[:space:]]+/ { in_keys=0 }

    # webhook 块内的 api_key 行删除
    /^webhook:[[:space:]]*$/ { in_webhook=1; print; next }
    in_webhook && /^[[:space:]]+api_key:/ { next }
    in_webhook && /^[^[:space:]]/ { in_webhook=0 }

    { print }
  ' "$LIVEKIT_YAML" > "$tmp"

  if cmp -s "$tmp" "$LIVEKIT_YAML"; then
    echo "  [ok] $LIVEKIT_YAML 已是无密钥形态"
    rm -f "$tmp"
    return 0
  fi

  mv "$tmp" "$LIVEKIT_YAML"
  echo "  [ok] 已归一 $LIVEKIT_YAML（移除硬编码 keys / webhook.api_key）"
}

# ── --check：只校验 ─────────────────────────────────────────────────────────
if [[ $CHECK_ONLY -eq 1 ]]; then
  echo "==> 校验 .env 与 livekit.yaml 一致性"
  if check_consistency; then
    echo "一致性校验通过。"
    exit 0
  fi
  echo "一致性校验失败。" >&2
  exit 1
fi

if [[ -f "$ENV_FILE" && $FORCE -ne 1 ]]; then
  echo "deploy/.env 已存在，如需重新生成: ./init.sh --force" >&2
  echo "（只做一致性校验: ./init.sh --check）" >&2
  exit 1
fi

rand() { openssl rand -hex 32; }

echo "==> 生成随机密钥"
JWT_KEY=$(rand)
STORAGE_ENCRYPT_KEY=$(rand)
POSTGRES_PASSWORD=$(rand)
LIVEKIT_KEY="API$(openssl rand -hex 6)"
LIVEKIT_SECRET=$(openssl rand -base64 48 | tr -d '/+=' | cut -c1-43)

# --force 重新生成时保留已有的浏览器侧地址，避免每次重置都要重填。
PREV_PUBLIC_URL=""
if [[ -f "$ENV_FILE" ]]; then
  PREV_PUBLIC_URL="$(env_get LIVEKIT_PUBLIC_URL "$ENV_FILE")"
fi

echo "==> 写出 $ENV_FILE（以 $ENV_TPL 为模板，替换全部密钥）"
# 用模板生成而非内联 heredoc，保证 .env 与 .env.example 字段永远一致；
# 新增配置项只需改模板，不会出现 init.sh 漏字段。
sed \
  -e "s|^JWT_KEY=.*|JWT_KEY=$JWT_KEY|" \
  -e "s|^STORAGE_ENCRYPT_KEY=.*|STORAGE_ENCRYPT_KEY=$STORAGE_ENCRYPT_KEY|" \
  -e "s|^POSTGRES_PASSWORD=.*|POSTGRES_PASSWORD=$POSTGRES_PASSWORD|" \
  -e "s|^LIVEKIT_KEY=.*|LIVEKIT_KEY=$LIVEKIT_KEY|" \
  -e "s|^LIVEKIT_SECRET=.*|LIVEKIT_SECRET=$LIVEKIT_SECRET|" \
  "$ENV_TPL" > "$ENV_FILE"

# 浏览器侧地址优先级：命令行参数 > 环境变量 > 已有 .env 的值
PUBLIC_URL="${LIVEKIT_URL_ARG:-${LIVEKIT_PUBLIC_URL:-$PREV_PUBLIC_URL}}"
if [[ -n "$PUBLIC_URL" ]]; then
  sed -i.bak "s|^LIVEKIT_PUBLIC_URL=.*|LIVEKIT_PUBLIC_URL=$PUBLIC_URL|" "$ENV_FILE"
  rm -f "$ENV_FILE.bak"
  echo "  [ok] LIVEKIT_PUBLIC_URL=$PUBLIC_URL"
fi

echo "==> 归一 $LIVEKIT_YAML"
normalize_livekit_yaml

echo "==> 校验一致性"
if ! check_consistency; then
  echo "" >&2
  echo "一致性校验失败，请检查上面的 [x] 项。" >&2
  exit 1
fi

echo ""
echo "完成。后续命令（在 deploy/ 目录下执行，或任意目录加 -f deploy/docker-compose.yml）:"
cat <<'EOF'

  docker compose up -d --build     # 首次启动
  docker compose logs -f gospeak   # 看日志
  curl http://localhost:8998/ping  # 健康检查

提示:
  - 生产环境务必配置 HTTPS（Caddy/Nginx TLS），浏览器麦克风需要安全上下文
  - LiveKit 浏览器侧地址必须公网可达：./init.sh --livekit-url wss://lk.example.com
  - WebRTC 媒体走 UDP，反代域名需放通 7882/udp、3478/udp，且不能用只代理 HTTP 的 CDN
  - 数据备份: docker run --rm -v gospeak_gospeak-db:/data -v "$PWD":/bak alpine \
      cp /data/app.db /bak/app-backup-$(date +%F).db
EOF
