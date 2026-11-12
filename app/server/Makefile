# GOSpeak — 单二进制跨平台打包
#
# 前端产物在编译前复制到 internal/webui/dist，经 go:embed 打进二进制。
# Bot 基础插件（bot-base）源码 + assets 同样经 go:embed 打进二进制，
# 启动后端时由插件 Registry 同步注册并启动，无需额外 sidecar 文件。
# 运行时无需再附带 static/ 目录（仍可用 STATIC_DIR 覆盖外挂静态资源）。
#
# CGO_ENABLED=0 默认纯 Go 交叉编译；本地 make 也可直接使用。
#
# 用法:
#   make all                # 前端 + 全部平台二进制
#   make linux              # linux/amd64 + linux/arm64
#   make darwin             # darwin/amd64 + darwin/arm64
#   make windows            # windows/amd64
#   make edge               # ARMv7/v6/v5 + RISC-V
#   make web                # 仅构建前端并同步到 embed 目录
#   make linux-amd64-bin    # 仅编译单平台二进制
#   make freebsd            # freebsd/amd64 + arm64
#   make windows-arm64      # windows/arm64 (Snapdragon X)

VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILDDIR  ?= build
LDFLAGS   := -ldflags="-s -w -X main.version=$(VERSION)"

REPO_ROOT := $(abspath $(CURDIR)/../..)
WEB_DIR   := $(REPO_ROOT)/app/web
WEB_DIST  := $(WEB_DIR)/dist
EMBED_DIR := internal/webui/dist

# ── 同步前端到 go:embed 目录 ──────────────────────────────────
define sync_web
	@if [ ! -f "$(WEB_DIST)/index.html" ]; then \
		echo "ERROR: $(WEB_DIST)/index.html missing; run 'make web' first"; \
		exit 1; \
	fi
	@rm -rf "$(EMBED_DIR)"
	@mkdir -p "$(EMBED_DIR)"
	@cp -R "$(WEB_DIST)/." "$(EMBED_DIR)/"
	@echo "✓  synced frontend → $(EMBED_DIR)"
endef

all: clean web linux windows darwin freebsd openbsd

lint: vet

vet:
	go vet ./...

clean:
	rm -rf $(BUILDDIR)
	mkdir -p $(BUILDDIR)
	# 保留 embed 目录结构，清理构建产物
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	printf '%s\n' \
		'This file keeps go:embed valid when frontend assets are not synced yet.' \
		'Release/Docker builds replace this directory with app/web/dist contents.' \
		> $(EMBED_DIR)/EMBED_PLACEHOLDER

$(BUILDDIR):
	mkdir -p $(BUILDDIR)

# ── 前端 ──────────────────────────────────────────────────────
web:
	@echo ">> Building web frontend …"
	cd $(REPO_ROOT) && pnpm install --frozen-lockfile
	cd $(REPO_ROOT) && pnpm --filter @gospeak/web build
	$(call sync_web)
	@echo "✓  $(WEB_DIST) → $(EMBED_DIR)"

sync-web:
	$(call sync_web)

# Bot 基础插件 assets 已纳入源码 go:embed（internal/plugin/builtin/botbase/assets）。
# 无需额外 sync 步骤；编译二进制时自动嵌入，启动时同步拉起。
bot-embed-check:
	@test -f internal/plugin/builtin/botbase/assets/manifest.json
	@test -f internal/plugin/builtin/botbase/assets/default_config.json
	@echo "✓  bot-base embed assets present"

# ── Linux ─────────────────────────────────────────────────────
linux: linux-amd64 linux-arm64

linux-amd64: web linux-amd64-bin
linux-arm64: web linux-arm64-bin

linux-amd64-bin: bot-embed-check | $(BUILDDIR)
	@echo ">> Building linux/amd64 (embedded web) …"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-linux-amd64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-linux-amd64"

linux-arm64-bin: bot-embed-check | $(BUILDDIR)
	@echo ">> Building linux/arm64 (embedded web) …"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-linux-arm64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-linux-arm64"

linux-armv7: web linux-armv7-bin

linux-armv7-bin: | $(BUILDDIR)
	@echo ">> Building linux/arm/v7 (ARMv7, 树莓派2/3 32位, 全志H3) …"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-linux-armv7 main.go
	@echo "✓  $(BUILDDIR)/gospeak-linux-armv7"

# ── macOS ─────────────────────────────────────────────────────
darwin: darwin-amd64 darwin-arm64

darwin-amd64: web darwin-amd64-bin
darwin-arm64: web darwin-arm64-bin

darwin-amd64-bin: bot-embed-check | $(BUILDDIR)
	@echo ">> Building darwin/amd64 (embedded web) …"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-darwin-amd64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-darwin-amd64"

darwin-arm64-bin: bot-embed-check | $(BUILDDIR)
	@echo ">> Building darwin/arm64 (embedded web) …"
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-darwin-arm64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-darwin-arm64"

# ── FreeBSD ──────────────────────────────────────────────────
freebsd: freebsd-amd64 freebsd-arm64

freebsd-amd64: web freebsd-amd64-bin

freebsd-amd64-bin: | $(BUILDDIR)
	@echo ">> Building freebsd/amd64 …"
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-freebsd-amd64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-freebsd-amd64"

freebsd-arm64: web freebsd-arm64-bin

freebsd-arm64-bin: | $(BUILDDIR)
	@echo ">> Building freebsd/arm64 (FreeBSD ARM, NAS) …"
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-freebsd-arm64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-freebsd-arm64"

# ── OpenBSD ──────────────────────────────────────────────────
openbsd: openbsd-amd64

openbsd-amd64: web openbsd-amd64-bin

openbsd-amd64-bin: | $(BUILDDIR)
	@echo ">> Building openbsd/amd64 (OpenBSD, 防火墙/路由) …"
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-openbsd-amd64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-openbsd-amd64"
# ── Windows ───────────────────────────────────────────────────


windows: windows-amd64 windows-arm64

windows-amd64: web windows-amd64-bin

windows-amd64-bin: | $(BUILDDIR)
	@echo ">> Building windows/amd64 (embedded web) …"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-windows-amd64.exe main.go
	@echo "✓  $(BUILDDIR)/gospeak-windows-amd64.exe"

windows-arm64: web windows-arm64-bin

windows-arm64-bin: | $(BUILDDIR)
	@echo ">> Building windows/arm64 (Snapdragon X / Surface Pro) …"
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-windows-arm64.exe main.go
	@echo "✓  $(BUILDDIR)/gospeak-windows-arm64.exe"

.PHONY: all clean web sync-web \
	linux linux-amd64 linux-arm64 \
	linux-amd64-bin linux-arm64-bin \
	darwin darwin-amd64 darwin-arm64 \
	darwin-amd64-bin darwin-arm64-bin \
	windows windows-amd64 windows-amd64-bin \
	openbsd openbsd-amd64 openbsd-amd64-bin \
	freebsd-arm64 freebsd-arm64-bin \
	linux-armv7 linux-armv7-bin \
	windows-arm64 windows-arm64-bin \
	freebsd freebsd-amd64 freebsd-amd64-bin
