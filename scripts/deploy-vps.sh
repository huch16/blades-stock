#!/usr/bin/env bash
# ==============================================================================
# Blades Stock — Ubuntu 24.04 (VPS) 工业级云原生 Docker 一键部署脚本 (v1.8.0)
# 硬件环境自适应引擎：
#   - 甲骨文云 (Oracle Cloud): 2核 CPU / 12G 内存
#   - Instabox 云沙箱: 4核 CPU / 8G 内存
#   - 通用 Ubuntu VPS: 1~8核 / 2~16G 内存
# 统一访问口令：2266762
# ==============================================================================

set -e

# 颜色常量
GREEN="\033[0;32m"
BLUE="\033[0;34m"
YELLOW="\033[1;33m"
CYAN="\033[0;36m"
RED="\033[0;31m"
NC="\033[0m"

echo -e "${BLUE}==================================================================${NC}"
echo -e "${GREEN}   Blades Stock v1.8.0 — 工业级智能量化投研中台 (Cloud-Native)   ${NC}"
echo -e "${BLUE}==================================================================${NC}"

# 1. 检查基础环境与依赖命令
echo -e "${BLUE}[1/6] 检查系统环境与基础工具...${NC}"
for cmd in curl git openssl awk; do
    if ! command -v "$cmd" &> /dev/null; then
        echo -e "${YELLOW}[提示] 未检测到 $cmd，正在尝试自动安装...${NC}"
        if [ -x "$(command -v apt-get)" ]; then
            sudo apt-get update -y && sudo apt-get install -y "$cmd"
        else
            echo -e "${RED}[错误] 请先手动安装 $cmd 后重试。${NC}"
            exit 1
        fi
    fi
done

# 2. 检查并安装 Docker 环境
echo -e "${BLUE}[2/6] 验证 Docker 容器运行环境...${NC}"
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}[提示] 未检测到 Docker，正在自动安装 Docker Engine...${NC}"
    curl -fsSL https://get.docker.com | bash
    sudo systemctl enable docker
    sudo systemctl start docker
fi

# 检查 Docker Compose
if ! docker compose version &> /dev/null; then
    echo -e "${YELLOW}[提示] 正在安装 docker-compose 插件...${NC}"
    sudo apt-get update -y && sudo apt-get install -y docker-compose-plugin
fi

# 3. 宿主机硬件感知与动态调优计算
echo -e "${BLUE}[3/6] 探测宿主机硬件配置并适配最佳性能方案...${NC}"
TOTAL_CORES=$(nproc 2>/dev/null || grep -c ^processor /proc/cpuinfo 2>/dev/null || echo 2)
TOTAL_RAM_KB=$(grep MemTotal /proc/meminfo 2>/dev/null | awk '{print $2}' || echo 8000000)
TOTAL_RAM_GB=$((TOTAL_RAM_KB / 1024 / 1024))

# 计算 CPU 配额 (预留 0.2 核给宿主机 SSH 与系统守护)
if [ "$TOTAL_CORES" -le 2 ]; then
    BLADES_CPU_LIMIT="1.8"
else
    BLADES_CPU_LIMIT=$(awk "BEGIN {printf \"%.1f\", $TOTAL_CORES - 0.2}")
fi

# 计算内存配额与 Go 内存限制
if [ "$TOTAL_RAM_GB" -ge 12 ]; then
    BLADES_MEM_LIMIT="8G"
    GOMEMLIMIT="7GiB"
    HW_PROFILE="甲骨文云 / 充裕大内存规格 (Oracle Cloud 2C/12G)"
elif [ "$TOTAL_RAM_GB" -ge 8 ]; then
    BLADES_MEM_LIMIT="6G"
    GOMEMLIMIT="5GiB"
    HW_PROFILE="Instabox / 高性能多核沙箱规格 (4C/8G)"
elif [ "$TOTAL_RAM_GB" -ge 4 ]; then
    BLADES_MEM_LIMIT="3G"
    GOMEMLIMIT="2500MiB"
    HW_PROFILE="标准云服务器规格 (2C/4G)"
else
    BLADES_MEM_LIMIT="1500M"
    GOMEMLIMIT="1200MiB"
    HW_PROFILE="轻量入门型规格 (1C/2G)"
fi

echo -e "   -> 宿主机硬件: ${CYAN}${TOTAL_CORES} 核 CPU / ${TOTAL_RAM_GB} GB 内存${NC}"
echo -e "   -> 适配策略集: ${GREEN}${HW_PROFILE}${NC}"
echo -e "   -> 容器资源上限: ${CYAN}CPU: ${BLADES_CPU_LIMIT} 核 | 内存: ${BLADES_MEM_LIMIT} | GOMEMLIMIT: ${GOMEMLIMIT}${NC}"

# 3.2 内存防穿透守护：检测并按需配置 Swap
SWAP_TOTAL_MB=$(free -m 2>/dev/null | awk '/Swap:/ {print $2}' || echo 0)
if [ -z "$SWAP_TOTAL_MB" ] || [ "$SWAP_TOTAL_MB" -lt 1024 ]; then
    echo -e "${YELLOW}[提示] 检测到当前 VPS Swap 虚拟内存较小 (${SWAP_TOTAL_MB}MB)，正在创建 2GB 紧急防 OOM 缓冲空间...${NC}"
    if [ -w / ] && [ ! -f /swapfile ]; then
        fallocate -l 2G /swapfile 2>/dev/null || dd if=/dev/zero of=/swapfile bs=1M count=2048 2>/dev/null || true
        if [ -f /swapfile ]; then
            chmod 600 /swapfile
            mkswap /swapfile >/dev/null 2>&1 || true
            swapon /swapfile 2>/dev/null || true
            if ! grep -q '/swapfile' /etc/fstab 2>/dev/null; then
                echo '/swapfile none swap sw 0 0' >> /etc/fstab 2>/dev/null || true
            fi
            echo -e "${GREEN}--> 2GB Swap 应急虚拟内存已就绪，全面杜绝极端并发下的系统级 OOM${NC}"
        fi
    fi
fi

# 4. 准备持久化目录与权威库初始化
echo -e "${BLUE}[4/6] 初始化数据与日志持久化资产...${NC}"
mkdir -p data logs

# 复制初始权威 SQLite 库快照（若 data 目录为空）
if [ ! -f "data/stock.db" ] && [ -f "build/bin/data/stock.db" ]; then
    echo -e "${GREEN}--> 注入出厂 SQLite 权威库快照与预设策略集...${NC}"
    cp "build/bin/data/stock.db" "data/stock.db"
fi

# 4.2 配置安全访问凭据与统一密码 (2266762)
UNIFIED_DEFAULT_PWD="2266762"

if [ ! -f ".env" ]; then
    cat <<EOF > .env
# Blades Stock 云端运行环境变量 (v1.8.0 统一标准)
BLADES_AUTH_TOKEN=${UNIFIED_DEFAULT_PWD}
PORT=8000
TZ=Asia/Shanghai
SEARXNG_URL=https://search.woha.men
BLADES_CPU_LIMIT=${BLADES_CPU_LIMIT}
BLADES_MEM_LIMIT=${BLADES_MEM_LIMIT}
GOMEMLIMIT=${GOMEMLIMIT}
EOF
    echo -e "${GREEN}--> 已自动为您配置工业级统一管理员密码: ${YELLOW}${UNIFIED_DEFAULT_PWD}${NC}"
else
    # 检查现有 .env，确保必要参数就绪
    if ! grep -q '^BLADES_AUTH_TOKEN=' .env; then
        echo "BLADES_AUTH_TOKEN=${UNIFIED_DEFAULT_PWD}" >> .env
    fi
    if grep -q '^PORT=8080' .env; then
        sed -i 's/^PORT=8080/PORT=8000/' .env
    fi
    # 补充硬件自适应参数
    if ! grep -q '^BLADES_CPU_LIMIT=' .env; then
        echo "BLADES_CPU_LIMIT=${BLADES_CPU_LIMIT}" >> .env
    fi
    if ! grep -q '^BLADES_MEM_LIMIT=' .env; then
        echo "BLADES_MEM_LIMIT=${BLADES_MEM_LIMIT}" >> .env
    fi
    if ! grep -q '^GOMEMLIMIT=' .env; then
        echo "GOMEMLIMIT=${GOMEMLIMIT}" >> .env
    fi
    echo -e "${GREEN}--> 已自动融合并更新 .env 硬件参数配置${NC}"
fi

# 同步持久化 auth.token
if [ -d "data" ] && [ ! -f "data/auth.token" ]; then
    echo -n "${UNIFIED_DEFAULT_PWD}" > "data/auth.token"
    chmod 600 "data/auth.token" 2>/dev/null || true
fi

AUTH_TOKEN=$(grep '^BLADES_AUTH_TOKEN=' .env | cut -d '=' -f2- | tr -d ' "')
APP_PORT=$(grep '^PORT=' .env | cut -d '=' -f2- | tr -d ' "')
APP_PORT=${APP_PORT:-8000}

# 5. 构建与启动 Docker 容器
echo -e "${BLUE}[5/6] 构建并启动 Docker 服务集群 (应用 ${HW_PROFILE} 优化参数)...${NC}"
docker compose up -d --build

# 6. 等待服务就绪并健康探活
echo -e "${BLUE}[6/6] 正在进行系统健康自检与端口探活...${NC}"
MAX_RETRIES=20
COUNT=0
HEALTHY=false

while [ $COUNT -lt $MAX_RETRIES ]; do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:${APP_PORT}/health || true)
    if [ "$HTTP_CODE" = "200" ]; then
        HEALTHY=true
        break
    fi
    sleep 2
    COUNT=$((COUNT + 1))
    echo -n "."
done
echo ""

if [ "$HEALTHY" = true ]; then
    # 获取公网 IP
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s https://ifconfig.me || echo "你的VPS公网IP")

    echo -e "\n${GREEN}==================================================================${NC}"
    echo -e "${GREEN}🎉 Blades Stock v1.8.0 工业级云原生投研工作站已成功部署！${NC}"
    echo -e "${GREEN}==================================================================${NC}"
    echo -e " 🌐 服务监听端口:       ${YELLOW}${APP_PORT}${NC}"
    echo -e " 🌐 网页直连访问:       ${YELLOW}http://${PUBLIC_IP}:${APP_PORT}${NC}"
    echo -e " 🔐 统一管理员登录密码: ${GREEN}${UNIFIED_DEFAULT_PWD}${NC}"
    echo -e " 🖥️ 硬件性能画像:       ${CYAN}${HW_PROFILE}${NC}"
    echo -e " ⚙️ 容器算力配额:       ${CYAN}CPU ${BLADES_CPU_LIMIT}核 / 内存 ${BLADES_MEM_LIMIT}${NC}"
    echo -e "${BLUE}------------------------------------------------------------------${NC}"
    echo -e " 📱 移动端原生体验:     手机访问支持底部 5 键原生 TabBar、单手沉浸流式布局"
    echo -e " 📱 PWA 原生应用模式:   手机 Safari / Chrome 可点击【添加到主屏幕】作为 App"
    echo -e " 💡 Nginx 反向代理提示: 若配置域名，请将 proxy_pass 指向 http://127.0.0.1:${APP_PORT};"
    echo -e "    长研报与流式防超时: 'proxy_read_timeout 600s; proxy_buffering off;'"
    echo -e "    参考配置模板位于:   scripts/nginx-blades-stock.conf"
    echo -e " 📊 数据持久化资产:     $(pwd)/data"
    echo -e " 📝 实时运行日志查看:   docker compose logs -f blades-stock"
    echo -e " 🛑 停止容器服务:       docker compose down"
    echo -e " 🔄 重启容器服务:       docker compose restart"
    echo -e "${GREEN}==================================================================${NC}\n"
else
    echo -e "${RED}[警告] 服务启动健康检查超时，请通过 'docker compose logs blades-stock' 查看详细容器启动日志。${NC}"
fi
