#!/usr/bin/env bash
# 一键全量验证入口（Round 2 固化新增）。
# 依次跑：gofmt、go vet、架构分层检查、Go 单元测试、前端测试、前端构建（含 TS2304 与 UI 纯度门禁）。
# 任一步失败即退出并返回非零码；改完代码后跑它，通过后再提交。
# 注意：/tmp 只有 512MB，Go 构建缓存目录固定用 $HOME/.cache/go-tmp。
set -euo pipefail
cd "$(dirname "$0")/.."

export PATH="$HOME/.local/go/bin:$PATH"
export TMPDIR="${TMPDIR:-$HOME/.cache/go-tmp}"
mkdir -p "$TMPDIR"

echo "== [1/6] gofmt =="
unformatted=$(gofmt -l . 2>/dev/null | grep -v -e '^vendor/' -e 'node_modules' || true)
if [ -n "$unformatted" ]; then
	echo "❌ 以下文件未通过 gofmt，请执行 'make fmt'："
	echo "$unformatted"
	exit 1
fi
echo "    ✅ gofmt：全部合规"

echo "== [2/6] go vet =="
go vet ./...
echo "    ✅ go vet 通过"

echo "== [3/6] check-deps（架构分层） =="
bash scripts/check-deps.sh

echo "== [4/6] go test ./...（单元测试，排除 integration） =="
go test ./... -run "^Test" -skip "integration"

echo "== [5/6] npm test（前端） =="
(cd frontend && npm test)

echo "== [6/6] npm run build（前端构建，含 TS2304 与 UI 纯度门禁） =="
(cd frontend && npm run build)

echo ""
echo "✅ ALL GREEN：gofmt / vet / check-deps / go test / npm test / npm run build 全部通过"
