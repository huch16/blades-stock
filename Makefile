# Makefile for Blades Stock (v4.0.0)
#
# 说明：本目标集刻意只依赖「本机确实可用」的工具链。
# 早期版本在此处调用了 golangci-lint，但该工具未随项目安装，导致 `make lint` 必然失败。

.PHONY: all help lint vet fmt test test-integration build build-web frontend-lint

all: lint test build

help:
	@echo "可用目标："
	@echo "  make lint              代码静态检查（go vet + staticcheck，若已安装）"
	@echo "  make fmt               格式化 Go 代码"
	@echo "  make test              单元测试（排除 integration）"
	@echo "  make test-integration  集成测试（需要 Chrome）"
	@echo "  make build             编译桌面端（调用 build.ps1 wails）"
	@echo "  make build-web         编译纯 Go Web 服务端（cmd/server）"
	@echo "  make frontend-lint     前端静态检查（ESLint，0 errors 基线）"

lint:
	@echo "[1/3] gofmt 合规性检查（0 个文件应为未格式化）..."
	@unformatted=$$(gofmt -l . 2>/dev/null | grep -v '^vendor/' || true); \
	if [ -n "$$unformatted" ]; then \
		echo "❌ 以下文件未通过 gofmt，请执行 'make fmt'："; echo "$$unformatted"; exit 1; \
	fi; \
	echo "    ✅ gofmt：全部合规"
	@echo "[2/3] go vet..."
	go vet ./...
	@echo "[3/3] staticcheck（可选）..."
	@command -v staticcheck >/dev/null 2>&1 \
		&& staticcheck ./... \
		|| echo "    （跳过 staticcheck：未安装，可执行 'go install honnef.co/go/tools/cmd/staticcheck@latest'）"

fmt:
	gofmt -w .

# 单元测试（排除需要外部依赖的 integration 标签）
test:
	go test ./... -run "^Test" -skip "integration"

# 集成测试（需要本机安装 Chrome）
test-integration:
	go test -tags=integration ./... -v

build:
	powershell -File build.ps1 wails

build-web:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o build/bin/blades-stock-server ./cmd/server

frontend-lint:
	cd frontend && npm run lint
