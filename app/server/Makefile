# GOSpeak — 单二进制跨平台打包
#
# 前端产物在编译前复制到 internal/webui/dist，经 go:embed 打进二进制。
# 运行时无需再附带 static/ 目录（仍可用 STATIC_DIR 覆盖外挂静态资源）。
#
# CGO_ENABLED=0 默认纯 Go 交叉编译；本地 make 也可直接使用。
#
# 用法:
#   make all                # 前端 + 全部平台二进制
#   make linux              # linux/amd64 + linux/arm64
#   make darwin             # darwin/amd64 + darwin/arm64
#   make windows            # windows/amd64
#   make web                # 仅构建前端并同步到 embed 目录
#   make linux-amd64-bin    # 仅编译单平台二进制

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

all: clean web linux windows darwin

clean:
	rm -rf $(BUILDDIR)
	mkdir -p $(BUILDDIR)
	# 保留 embed 目录结构，清理构建产物
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	touch $(EMBED_DIR)/.gitkeep

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

# ── Linux ─────────────────────────────────────────────────────
linux: linux-amd64 linux-arm64

linux-amd64: web linux-amd64-bin
linux-arm64: web linux-arm64-bin

linux-amd64-bin: | $(BUILDDIR)
	@echo ">> Building linux/amd64 (embedded web) …"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-linux-amd64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-linux-amd64"

linux-arm64-bin: | $(BUILDDIR)
	@echo ">> Building linux/arm64 (embedded web) …"
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-linux-arm64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-linux-arm64"

# ── macOS ─────────────────────────────────────────────────────
darwin: darwin-amd64 darwin-arm64

darwin-amd64: web darwin-amd64-bin
darwin-arm64: web darwin-arm64-bin

darwin-amd64-bin: | $(BUILDDIR)
	@echo ">> Building darwin/amd64 (embedded web) …"
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-darwin-amd64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-darwin-amd64"

darwin-arm64-bin: | $(BUILDDIR)
	@echo ">> Building darwin/arm64 (embedded web) …"
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-darwin-arm64 main.go
	@echo "✓  $(BUILDDIR)/gospeak-darwin-arm64"

# ── Windows ───────────────────────────────────────────────────
windows: windows-amd64

windows-amd64: web windows-amd64-bin

windows-amd64-bin: | $(BUILDDIR)
	@echo ">> Building windows/amd64 (embedded web) …"
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/gospeak-windows-amd64.exe main.go
	@echo "✓  $(BUILDDIR)/gospeak-windows-amd64.exe"

.PHONY: all clean web sync-web \
	linux linux-amd64 linux-arm64 \
	linux-amd64-bin linux-arm64-bin \
	darwin darwin-amd64 darwin-arm64 \
	darwin-amd64-bin darwin-arm64-bin \
	windows windows-amd64 windows-amd64-bin
