#!/usr/bin/env bash
# ==============================================================================
# Blades Stock — Ubuntu 24.04 (VPS) 一键云原生 Docker 部署脚本
# 适用硬件：2核 CPU / 12G 内存 / 200GB SSD 云服务器
# ==============================================================================

set -e

# 颜色常量
GREEN="\033[0;32m"
BLUE="\033[0;34m"
YELLOW="\033[1;33m"
RED="\033[0;31m"
NC="\033[0m"

echo -e "${BLUE}==================================================================${NC}"
echo -e "${GREEN}   Blades Stock — 智能量化投研工作站 (Cloud-Native Web 版)   ${NC}"
echo -e "${BLUE}==================================================================${NC}"

# 1. 检查基础命令
for cmd in curl git openssl; do
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

# 2. 检查 Docker 环境
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}[提示] 未检测到 Docker，正在自动安装 Docker...${NC}"
    curl -fsSL https://get.docker.com | bash
    sudo systemctl enable docker
    sudo systemctl start docker
fi

# 检查 Docker Compose
if ! docker compose version &> /dev/null; then
    echo -e "${YELLOW}[提示] 正在安装 docker-compose 插件...${NC}"
    sudo apt-get update -y && sudo apt-get install -y docker-compose-plugin
fi

# 3. 准备持久化目录
echo -e "${BLUE}[1/4] 初始化数据与日志持久化目录...${NC}"
mkdir -p data logs

# 复制默认初始 SQLite 权威库（如果本地 data 目录为空）
if [ ! -f "data/stock.db" ] && [ -f "build/bin/data/stock.db" ]; then
    echo -e "${GREEN}--> 正在注入默认 SQLite 权威库快照...${NC}"
    cp "build/bin/data/stock.db" "data/stock.db"
fi

# 4. 生成或读取访问口令 (.env)
echo -e "${BLUE}[2/4] 配置安全访问凭据...${NC}"
if [ ! -f ".env" ]; then
    RANDOM_TOKEN=$(openssl rand -hex 8)
    cat <<EOF > .env
# Blades Stock 云端运行环境变量
BLADES_AUTH_TOKEN=${RANDOM_TOKEN}
PORT=8000
TZ=Asia/Shanghai
SEARXNG_URL=https://search.woha.men
EOF
    echo -e "${GREEN}--> 已为您自动生成高强度安全访问口令并保存至 .env 文件${NC}"
else
    # 如果已有 .env 但未配置 PORT 或仍为 8080，则替换为 8000
    if grep -q '^PORT=8080' .env; then
        sed -i 's/^PORT=8080/PORT=8000/' .env
        echo -e "${GREEN}--> 已自动将 .env 端口更新为 8000${NC}"
    fi
    echo -e "${GREEN}--> 已检测到现有 .env 配置文件${NC}"
fi

# 提取当前 Token 与端口
AUTH_TOKEN=$(grep '^BLADES_AUTH_TOKEN=' .env | cut -d '=' -f2- | tr -d ' "')
APP_PORT=$(grep '^PORT=' .env | cut -d '=' -f2- | tr -d ' "')
APP_PORT=${APP_PORT:-8000}

# 5. 构建与启动 Docker 容器
echo -e "${BLUE}[3/4] 构建并启动 Docker 服务集群 (针对 2核/12G VPS 调优)...${NC}"
docker compose up -d --build

# 6. 等待服务就绪并健康探活
echo -e "${BLUE}[4/4] 正在进行系统健康自检...${NC}"
MAX_RETRIES=15
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
    echo -e "${GREEN}🎉 Blades Stock 云原生投研平台已成功部署并启动！${NC}"
    echo -e "${GREEN}==================================================================${NC}"
    echo -e " 🌐 服务内部监听端口: ${YELLOW}${APP_PORT}${NC}"
    echo -e " 🌐 网页直连地址:     ${YELLOW}http://${PUBLIC_IP}:${APP_PORT}${NC}"
    echo -e " 🔐 专属访问口令:     ${YELLOW}${AUTH_TOKEN}${NC}"
    echo -e "${BLUE}------------------------------------------------------------------${NC}"
    echo -e " 💡 Nginx 反向代理提示：若使用 Nginx 反代，请将 proxy_pass 指向 http://127.0.0.1:${APP_PORT};"
    echo -e "    并确保开启 SSE 流式推流配置 (proxy_buffering off;)"
    echo -e " 📊 数据持久化位置: $(pwd)/data"
    echo -e " 📝 运行日志查看:   docker compose logs -f blades-stock"
    echo -e " 🛑 停止服务:       docker compose down"
    echo -e " 🔄 重启服务:       docker compose restart"
    echo -e "${GREEN}==================================================================${NC}\n"
else
    echo -e "${RED}[警告] 服务启动健康检查超时，请通过 'docker compose logs blades-stock' 查看详细容器启动日志。${NC}"
fi
