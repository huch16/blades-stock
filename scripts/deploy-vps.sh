#!/usr/bin/env bash
# =============================================================================
# Blades Stock — VPS 原生一键部署脚本（无需 Docker）
#
# 适用：Ubuntu 20.04+ / Debian 11+（x86_64 / arm64）
# 功能：环境自检 → 自动装 Go/Node → 拉代码 → 构建前后端 → systemd 服务 → 健康检查
#
# 用法（推荐，两行搞定）：
#     cd ~/blades-stock && git pull && sudo bash scripts/deploy-vps.sh
#
# 可选环境变量覆盖：
#     BLADES_DIR=/opt/blades-stock   安装目录（默认 /opt/blades-stock）
#     BLADES_PORT=8000               监听端口
#     BLADES_BRANCH=v3.1.3           部署的 Git 引用（分支或 tag）
#     BLADES_TOKEN=<your-token>      Web 访问口令（无默认值；未设定时脚本会
#                                    随机生成一个并在部署成功后打印一次，请妥善保存）
# =============================================================================

set -euo pipefail

# ----------------------------- 配置 -----------------------------------------
INSTALL_DIR="${BLADES_DIR:-/opt/blades-stock}"
APP_PORT="${BLADES_PORT:-8000}"
APP_HOST="${BLADES_HOST:-0.0.0.0}"
GIT_REF="${BLADES_BRANCH:-v3.1.3}"
# 访问口令没有默认值：未设定时随机生成（openssl 不可用则用 /dev/urandom），
# 部署成功后打印一次。绝不在脚本或文档里写死口令。
if [ -n "${BLADES_TOKEN:-}" ]; then
    ACCESS_TOKEN="${BLADES_TOKEN}"
    ACCESS_TOKEN_GENERATED=0
else
    if command -v openssl >/dev/null 2>&1; then
        ACCESS_TOKEN="$(openssl rand -hex 16)"
    else
        ACCESS_TOKEN="$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')"
    fi
    ACCESS_TOKEN_GENERATED=1
fi
SERVICE_NAME="blades-stock"
GO_VERSION_WANT="1.26.0"
REPO_URL="https://github.com/huch16/blades-stock.git"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; RED='\033[0;31m'; NC='\033[0m'
step() { echo -e "\n${BLUE}==> $*${NC}"; }
ok()   { echo -e "${GREEN}    ✓ $*${NC}"; }
warn() { echo -e "${YELLOW}    ! $*${NC}"; }
die()  { echo -e "${RED}    ✗ $*${NC}"; exit 1; }

[ "$(id -u)" -eq 0 ] || die "请用 root 运行：sudo bash scripts/deploy-vps.sh"

echo -e "${BLUE}"
echo "==============================================================="
echo "   Blades Stock 部署 (原生构建 · 无需 Docker)"
echo "==============================================================="
echo -e "${NC}部署目录 : ${INSTALL_DIR}"
echo "访问端口 : ${APP_PORT}"
echo "Git 引用 : ${GIT_REF}"

# ------------------------- 1. 基础依赖 ---------------------------------------
step "[1/7] 安装基础工具"
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get install -y -qq git curl wget ca-certificates build-essential sqlite3 >/dev/null
ok "git / curl / wget / gcc / make / sqlite3 就绪"

# ------------------------- 2. Go 工具链 --------------------------------------
step "[2/7] 检查 Go 工具链 (需要 ${GO_VERSION_WANT}+)"
need_go=0
if command -v go >/dev/null 2>&1; then
    cur="$(go env GOVERSION 2>/dev/null | sed 's/^go//')"
    # 简单版本比较：主版本号一致即可
    if [ "${cur%%.*}" != "${GO_VERSION_WANT%%.*}" ]; then
        warn "当前 Go ${cur} 与 go.mod 要求 1.26 不一致，将安装 ${GO_VERSION_WANT}"
        need_go=1
    else
        ok "已安装 Go ${cur}"
    fi
else
    need_go=1
fi

if [ "$need_go" -eq 1 ]; then
    case "$(uname -m)" in
        x86_64|amd64) GO_ARCH=amd64 ;;
        aarch64|arm64) GO_ARCH=arm64 ;;
        *) die "不支持的 CPU 架构: $(uname -m)" ;;
    esac
    echo "    下载 go${GO_VERSION_WANT}.linux-${GO_ARCH}.tar.gz ..."
    wget -q "https://go.dev/dl/go${GO_VERSION_WANT}.linux-${GO_ARCH}.tar.gz" -O /tmp/go.tar.gz \
        || die "Go 下载失败（检查网络/DNS）"
    rm -rf /usr/local/go
    tar -C /usr/local -xzf /tmp/go.tar.gz
    rm -f /tmp/go.tar.gz
    printf 'export PATH=$PATH:/usr/local/go/bin\n' > /etc/profile.d/go.sh
    export PATH="$PATH:/usr/local/go/bin"
    ok "Go $(go version | awk '{print $3}') 安装完成"
fi
export PATH="$PATH:/usr/local/go/bin"

# ------------------------- 3. Node 工具链 ------------------------------------
step "[3/7] 检查 Node.js (需要 18+)"
need_node=0
if command -v node >/dev/null 2>&1; then
    node_major="$(node -v | sed 's/^v//' | cut -d. -f1)"
    if [ "$node_major" -lt 18 ] 2>/dev/null; then
        warn "当前 Node $(node -v) 过低，将升级到 20.x"
        need_node=1
    else
        ok "已安装 Node $(node -v)"
    fi
else
    need_node=1
fi

if [ "$need_node" -eq 1 ]; then
    echo "    通过 NodeSource 安装 Node.js 20.x ..."
    curl -fsSL https://deb.nodesource.com/setup_20.x | bash - >/dev/null 2>&1
    apt-get install -y -qq nodejs >/dev/null
    ok "Node $(node -v) / npm $(npm -v) 安装完成"
fi

# ------------------------- 4. 同步代码 ---------------------------------------
step "[4/7] 同步代码 (${GIT_REF})"
if [ -d "${INSTALL_DIR}/.git" ]; then
    cd "${INSTALL_DIR}"
    git fetch --tags --force origin
    ok "已拉取远程更新"
else
    # 若目标是当前脚本所在仓库，则原地使用，避免重复克隆
    SCRIPT_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
    if [ -d "${SCRIPT_REPO}/.git" ]; then
        INSTALL_DIR="${SCRIPT_REPO}"
        cd "${INSTALL_DIR}"
        git fetch --tags --force origin || warn "fetch 失败，使用本地代码继续"
        ok "使用当前仓库: ${INSTALL_DIR}"
    else
        rm -rf "${INSTALL_DIR}"
        git clone "${REPO_URL}" "${INSTALL_DIR}"
        cd "${INSTALL_DIR}"
        ok "克隆完成"
    fi
fi

git checkout -f "${GIT_REF}" 2>/dev/null || warn "引用 ${GIT_REF} 不存在，保持当前代码"
ok "当前提交: $(git rev-parse --short HEAD) — $(git log -1 --pretty=%s | cut -c1-60)"

# ------------------------- 5. 构建前端 ---------------------------------------
step "[5/7] 构建前端 (Vue 3 + Vite)"

# Vite/Rollup 构建峰值内存可达 2~4GB，Node 默认堆上限（约 2GB）会直接
# OOM 崩溃（实测小内存 VPS 上表现为 “Aborted (core dumped)”）。
# 这里按宿主机内存自适应设置堆上限，并在内存偏小时补一块 SWAP 兜底。
TOTAL_MEM_MB=$(awk '/MemTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo 2048)
SWAP_MB=$(awk '/SwapTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || echo 0)

# 内存偏小时补 SWAP：<4GB 内存补 4GB，否则补 2GB
SWAP_WANT_MB=2048
if [ "$TOTAL_MEM_MB" -lt 4096 ]; then
    SWAP_WANT_MB=4096
fi
if [ "$SWAP_MB" -lt 1024 ]; then
    warn "Swap 仅 ${SWAP_MB}MB，创建 $((SWAP_WANT_MB/1024))GB 应急 SWAP 防止构建 OOM..."
    if [ ! -f /swapfile ]; then
        (fallocate -l "${SWAP_WANT_MB}M" /swapfile 2>/dev/null \
            || dd if=/dev/zero of=/swapfile bs=1M count="${SWAP_WANT_MB}" 2>/dev/null) || true
        if [ -f /swapfile ]; then
            chmod 600 /swapfile
            mkswap /swapfile >/dev/null 2>&1 || true
            swapon /swapfile 2>/dev/null || true
            grep -q '^/swapfile' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
            ok "$((SWAP_WANT_MB/1024))GB SWAP 已就绪"
        else
            warn "SWAP 创建失败（磁盘空间不足？），构建可能因内存不足失败"
        fi
    else
        swapon /swapfile 2>/dev/null || true
        ok "复用已有 /swapfile"
    fi
fi

# 堆上限取「内存的 70%」，下限 2048MB、上限 6144MB（与 Dockerfile 对齐）
NODE_HEAP=$(( TOTAL_MEM_MB * 70 / 100 ))
if [ "$NODE_HEAP" -lt 2048 ]; then NODE_HEAP=2048; fi
if [ "$NODE_HEAP" -gt 6144 ]; then NODE_HEAP=6144; fi
export NODE_OPTIONS="--max-old-space-size=${NODE_HEAP}"
echo "    内存: ${TOTAL_MEM_MB}MB / SWAP: ${SWAP_MB}MB → Node 堆上限: ${NODE_HEAP}MB"

cd "${INSTALL_DIR}/frontend"
BUILD_LOG=/tmp/blades-frontend-build.log

echo "    npm install（首次较慢，约 1-3 分钟）..."
if ! npm install --no-audit --no-fund > "${BUILD_LOG}" 2>&1; then
    echo "--- npm install 失败，日志末尾 ---"
    tail -25 "${BUILD_LOG}"
    die "npm install 失败（完整日志: ${BUILD_LOG}）"
fi
ok "依赖安装完成"

echo "    npm run build ..."
if ! npm run build > "${BUILD_LOG}" 2>&1; then
    echo "--- npm run build 失败，日志末尾 ---"
    tail -30 "${BUILD_LOG}"
    echo "-----------------------------------------"
    if grep -qiE 'heap out of memory|Aborted|Allocation failed|JavaScript heap' "${BUILD_LOG}"; then
        die "前端构建因内存不足失败。请手动加大 SWAP 后重试：\n      fallocate -l 4G /swapfile && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile\n      # 然后重新运行: sudo bash scripts/deploy-vps.sh"
    fi
    die "前端构建失败（完整日志: ${BUILD_LOG}）"
fi
[ -f dist/index.html ] || die "前端构建失败：dist/index.html 不存在"
ok "前端构建完成"

# 去掉构建告警干扰，仅回显体积概览
if [ -f dist/index.html ]; then
    echo "    dist 体积: $(du -sh dist | cut -f1)"
fi

# 发布韧性：保留上一版本产物，供服务端做旧 chunk 回退（避免部署打断在线用户）
rm -rf dist-previous
mkdir -p dist-previous
cp -r dist/. dist-previous/
ok "前端构建完成（含 dist-previous 回退副本）"

# ------------------------- 6. 构建后端 ---------------------------------------
step "[6/7] 构建后端 (纯 Go 静态编译)"
cd "${INSTALL_DIR}"
go mod download
CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w -X main.Version=$(git rev-parse --short HEAD)" \
    -o build/bin/blades-stock-server ./cmd/server
[ -f build/bin/blades-stock-server ] || die "后端构建失败"
ok "后端构建完成: $(du -h build/bin/blades-stock-server | cut -f1)"

# ------------------------- 7. 服务与数据 -------------------------------------
step "[7/7] 配置 systemd 服务与数据目录"
mkdir -p "${INSTALL_DIR}/data" "${INSTALL_DIR}/logs"

cat > "/etc/systemd/system/${SERVICE_NAME}.service" << EOF
[Unit]
Description=Blades Stock AI Investment Workstation
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
Environment="BLADES_AUTH_TOKEN=${ACCESS_TOKEN}"
Environment="BLADES_STOCK_DATA_DIR=${INSTALL_DIR}/data"
Environment="PORT=${APP_PORT}"
Environment="HOST=${APP_HOST}"
ExecStart=${INSTALL_DIR}/build/bin/blades-stock-server \\
    --data-dir=${INSTALL_DIR}/data \\
    --dist-dir=${INSTALL_DIR}/frontend/dist \\
    --previous-dist-dir=${INSTALL_DIR}/frontend/dist-previous \\
    --port=${APP_PORT} --host=${APP_HOST}
Restart=always
RestartSec=5
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}" >/dev/null 2>&1
systemctl restart "${SERVICE_NAME}"
ok "服务已启动 (${SERVICE_NAME})"

# ------------------------- 健康检查 ------------------------------------------
echo ""
echo "    等待服务就绪（最多 60 秒）..."
healthy=0
for i in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:${APP_PORT}/health" 2>/dev/null | grep -q '"healthy"'; then
        healthy=1
        break
    fi
    sleep 2
done

if [ "$healthy" -eq 1 ]; then
    PUB_IP="$(curl -fsS --max-time 5 https://api.ipify.org 2>/dev/null || hostname -I | awk '{print $1}')"
    echo ""
    echo -e "${GREEN}==============================================================="
    echo "   部署成功！"
    echo "==============================================================="
    echo -e "${NC}   访问地址 : http://${PUB_IP}:${APP_PORT}"
    if [ "${ACCESS_TOKEN_GENERATED}" = "1" ]; then
        echo "   访问口令 : ${ACCESS_TOKEN}（本次随机生成，请立即妥善保存）"
    else
        echo "   访问口令 : （使用你设定的 BLADES_TOKEN）"
    fi
    echo "   服务状态 : systemctl status ${SERVICE_NAME}"
    echo "   实时日志 : journalctl -u ${SERVICE_NAME} -f"
    echo -e "${GREEN}===============================================================${NC}"
else
    echo ""
    echo -e "${RED}服务未能在 60 秒内就绪，最近 40 行日志：${NC}"
    journalctl -u "${SERVICE_NAME}" -n 40 --no-pager || true
    die "部署失败：请根据上方日志排查（常见原因：端口被占用 / 数据目录权限）"
fi
