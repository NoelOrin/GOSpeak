# GOSpeak Server — 跨平台编译
#
# CGO_ENABLED=1 因为 gorm.io/driver/sqlite 需要 mattn/go-sqlite3 (CGO).
# 各脚本会在本地可用的范围跳过无法编译的平台，报 WARN 而非 fatal。
#
# 用法:
#   make linux              # linux/amd64 + linux/arm64 (默认 CC)
#   make linux-docker       # 在容器内编译 linux 目标 (无需交叉工具链)
#   make darwin             # darwin/amd64 + darwin/arm64
#   make windows            # windows/amd64 (需要 mingw)

VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
BUILDDIR  ?= build
LDFLAGS   := -ldflags="-s -w -X main.version=$(VERSION)"

all: clean linux windows darwin

clean:
	rm -rf $(BUILDDIR)
	mkdir -p $(BUILDDIR)

# ── Linux ─────────────────────────────────────────────────────
linux: linux-amd64 linux-arm64

linux-amd64:
	@echo ">> Building linux/amd64 …"
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="" \
		go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-amd64 main.go
	@echo "✓  $(BUILDDIR)/server-linux-amd64"

linux-arm64:
	@if command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then \
		echo ">> Building linux/arm64 …"; \
		CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
			go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-arm64 main.go; \
		echo "✓  $(BUILDDIR)/server-linux-arm64"; \
	else \
		echo "WARN: aarch64-linux-gnu-gcc not found; skipping linux/arm64"; \
	fi

linux-docker:
	@echo ">> Building linux/* via Docker …"
	docker run --rm -v $(PWD):/src -w /src \
		-e CGO_ENABLED=1 \
		golang:1.26-alpine \
		sh -c "\
			apk add --no-cache gcc musl-dev sqlite-dev aarch64-linux-gnu-gcc; \
			CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=gcc \
				go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-amd64 main.go && echo '✓ amd64'; \
			CGO_ENABLED=1 GOOS=linux GOARCH=arm64 CC=aarch64-linux-gnu-gcc \
				go build $(LDFLAGS) -o $(BUILDDIR)/server-linux-arm64 main.go && echo '✓ arm64'; \
		"
	@echo "✓  linux targets in $(BUILDDIR)/"

# ── macOS ─────────────────────────────────────────────────────
darwin: darwin-amd64 darwin-arm64

darwin-amd64:
	@echo ">> Building darwin/amd64 …"
	CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 CC="" \
		go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-amd64 main.go
	@echo "✓  $(BUILDDIR)/server-darwin-amd64"

darwin-arm64:
	@echo ">> Building darwin/arm64 …"
	CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 CC="" \
		go build $(LDFLAGS) -o $(BUILDDIR)/server-darwin-arm64 main.go
	@echo "✓  $(BUILDDIR)/server-darwin-arm64"

# ── Windows ───────────────────────────────────────────────────
windows: windows-amd64

windows-amd64:
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		echo ">> Building windows/amd64 …"; \
		CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
			go build $(LDFLAGS) -o $(BUILDDIR)/server-windows-amd64.exe main.go; \
		echo "✓  $(BUILDDIR)/server-windows-amd64.exe"; \
	else \
		echo "WARN: x86_64-w64-mingw32-gcc not found; skipping windows/amd64"; \
	fi

.PHONY: all clean linux darwin windows \
	linux-amd64 linux-arm64 linux-docker \
	darwin-amd64 darwin-arm64 \
	windows-amd64
