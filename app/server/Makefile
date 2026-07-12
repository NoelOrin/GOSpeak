# GOSpeak — 前后端一体化跨平台打包
#
# 每个平台产物 = 服务端二进制 + 前端静态资源 (static/)，打进同一个压缩包。
# 服务端运行时从同目录 ./static 托管前端 SPA (见 internal/router/router.go serveSPA)。
#
# CGO_ENABLED=1 因为 gorm.io/driver/sqlite 需要 mattn/go-sqlite3 (CGO)。
# 无交叉工具链的目标会 WARN 并跳过，而非 fatal。
#
# 用法:
#   make all                # 前端 + 全部平台，打包
#   make linux              # linux/amd64 + linux/arm64，打包
#   make linux-docker       # 容器内交叉编译 linux 目标，宿主机打包
#   make darwin             # darwin/amd64 + darwin/arm64，打包
#   make windows            # windows/amd64，打包
#   make web                # 仅构建前端产物
#   make linux-amd64-bin    # 仅编译单平台二进制 (不打包)

VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILDDIR  ?= build
LDFLAGS   := -ldflags="-s -w -X main.version=$(VERSION)"

REPO_ROOT := $(abspath $(CURDIR)/../..)
WEB_DIR   := $(REPO_ROOT)/app/web
WEB_DIST  := $(WEB_DIR)/dist

# ── 打包助手 ──────────────────────────────────────────────────
# $(1)=目标名 (linux-amd64)  $(2)=BUILDDIR 内的二进制文件名
# 产物: $(BUILDDIR)/gospeak-<target>/{gospeak-server, static/} + 压缩包
define pkg_tar
	@if [ -f "$(BUILDDIR)/$(2)" ]; then \
		echo ">> Packaging $(1) …"; \
		rm -rf "$(BUILDDIR)/gospeak-$(1)"; \
		mkdir -p "$(BUILDDIR)/gospeak-$(1)/static"; \
		cp "$(BUILDDIR)/$(2)" "$(BUILDDIR)/gospeak-$(1)/gospeak-server"; \
		cp -R "$(WEB_DIST)/." "$(BUILDDIR)/gospeak-$(1)/static/"; \
		( cd "$(BUILDDIR)" && tar czf "gospeak-$(1).tar.gz" "gospeak-$(1)" ); \
		echo "✓  $(BUILDDIR)/gospeak-$(1).tar.gz"; \
	else \
		echo "WARN: $(BUILDDIR)/$(2) not built; skipping package $(1)"; \
	fi
endef

define pkg_zip
	@if [ -f "$(BUILDDIR)/$(2)" ]; then \
		echo ">> Packaging $(1) …"; \
		rm -rf "$(BUILDDIR)/gospeak-$(1)"; \
		mkdir -p "$(BUILDDIR)/gospeak-$(1)/static"; \
		cp "$(BUILDDIR)/$(2)" "$(BUILDDIR)/gospeak-$(1)/gospeak-server.exe"; \
		cp -R "$(WEB_DIST)/." "$(BUILDDIR)/gospeak-$(1)/static/"; \
		( cd "$(BUILDDIR)" && zip -qr "gospeak-$(1).zip" "gospeak-$(1)" ); \
		echo "✓  $(BUILDDIR)/gospeak-$(1).zip"; \
	else \
		echo "WARN: $(BUILDDIR)/$(2) not built; skipping package $(1)"; \
	fi
endef

all: clean web linux windows darwin

clean:
	rm -rf $(BUILDDIR)
	mkdir -p $(BUILDDIR)

$(BUILDDIR):
	mkdir -p $(BUILDDIR)

# ── 前端 ──────────────────────────────────────────────────────
web:
	@echo ">> Building web frontend …"
	cd $(REPO_ROOT) && pnpm install --frozen-lockfile
	cd $(REPO_ROOT) && pnpm --filter @gospeak/web build
	@echo "✓  $(WEB_DIST)"

# ── Linux ─────────────────────────────────────────────────────
linux: linux-amd64 linux-arm64

linux-amd64: web linux-amd64-bin
	$(call pkg_tar,linux-amd64,server-linux-amd64)

linux-arm64: web linux-arm64-bin
	$(call pkg_tar,linux-arm64,server-linux-arm64)

linux-amd64-bin: | $(BUILDDIR)
	@echo ">> Building linux/amd64 …"
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-amd64 main.go
	@echo "✓  $(BUILDDIR)/server-linux-amd64"

linux-arm64-bin: | $(BUILDDIR)
	@if command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then \
		echo ">> Building linux/arm64 …"; \
		CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
			go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-arm64 main.go; \
		echo "✓  $(BUILDDIR)/server-linux-arm64"; \
	else \
		echo "WARN: aarch64-linux-gnu-gcc not found; skipping linux/arm64"; \
	fi

# 容器内交叉编译 linux 二进制 (glibc)，宿主机负责前端与打包。
linux-docker: web | $(BUILDDIR)
	@echo ">> Building linux/* via Docker …"
	docker run --rm -v $(REPO_ROOT):/src -w /src/app/server \
		-e CGO_ENABLED=1 \
		golang:1.26 \
		sh -c 'set -e; \
			apt-get update && apt-get install -y gcc gcc-aarch64-linux-gnu libsqlite3-dev; \
			CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc \
				go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-amd64 main.go && echo "✓ amd64"; \
			CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
				go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-arm64 main.go && echo "✓ arm64"'
	$(call pkg_tar,linux-amd64,server-linux-amd64)
	$(call pkg_tar,linux-arm64,server-linux-arm64)

# ── macOS ─────────────────────────────────────────────────────
darwin: darwin-amd64 darwin-arm64

darwin-amd64: web darwin-amd64-bin
	$(call pkg_tar,darwin-amd64,server-darwin-amd64)

darwin-arm64: web darwin-arm64-bin
	$(call pkg_tar,darwin-arm64,server-darwin-arm64)

darwin-amd64-bin: | $(BUILDDIR)
	@echo ">> Building darwin/amd64 …"
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-amd64 main.go
	@echo "✓  $(BUILDDIR)/server-darwin-amd64"

darwin-arm64-bin: | $(BUILDDIR)
	@echo ">> Building darwin/arm64 …"
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 \
		go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-arm64 main.go
	@echo "✓  $(BUILDDIR)/server-darwin-arm64"

# ── Windows ───────────────────────────────────────────────────
windows: windows-amd64

windows-amd64: web windows-amd64-bin
	$(call pkg_zip,windows-amd64,server-windows-amd64.exe)

windows-amd64-bin: | $(BUILDDIR)
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		echo ">> Building windows/amd64 …"; \
		CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
			go build $(LDFLAGS) -o $(BUILDDIR)/server-windows-amd64.exe main.go; \
		echo "✓  $(BUILDDIR)/server-windows-amd64.exe"; \
	else \
		echo "WARN: x86_64-w64-mingw32-gcc not found; skipping windows/amd64"; \
	fi

.PHONY: all clean web \
	linux linux-amd64 linux-arm64 linux-docker \
	linux-amd64-bin linux-arm64-bin \
	darwin darwin-amd64 darwin-arm64 \
	darwin-amd64-bin darwin-arm64-bin \
	windows windows-amd64 windows-amd64-bin
