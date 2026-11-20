#!/usr/bin/env bash
# ==============================================================================
# Blades Stock — Ubuntu 24.04 (VPS) 工业级云原生 Docker 一键部署脚本 (v3.0.0)
# 硬件环境自适应引擎：
#   - 甲骨文云 (Oracle Cloud): 2核 CPU / 12G 内存
#   - Instabox 云沙箱: 4核 CPU / 8G 内存
#   - 通用 Ubuntu VPS: 1~8核 / 2~16G 内存
# 访问口令没有默认值：未设定时本脚本随机生成一个写入 .env 并在部署成功后打印一次。
# 绝不在脚本、文档或代码里写死口令。
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
echo -e "${GREEN}   Blades Stock v3.0.0 — 工业级智能量化投研中台 (Cloud-Native)   ${NC}"
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

# 4.2 配置安全访问凭据：口令没有默认值。优先使用环境变量 BLADES_AUTH_TOKEN，
# 其次复用已有 .env 中的口令，都没有时随机生成一个并写入 .env。
gen_random_token() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex 16
    else
        head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n'
    fi
}

if [ -n "${BLADES_AUTH_TOKEN:-}" ]; then
    AUTH_TOKEN_FROM_ENV="${BLADES_AUTH_TOKEN}"
    TOKEN_GENERATED=0
else
    AUTH_TOKEN_FROM_ENV=""
    TOKEN_GENERATED=0
fi

if [ ! -f ".env" ]; then
    NEW_TOKEN="${AUTH_TOKEN_FROM_ENV:-$(gen_random_token)}"
    [ -z "$AUTH_TOKEN_FROM_ENV" ] && TOKEN_GENERATED=1
    cat <<EOF > .env
# Blades Stock 云端运行环境变量 (v3.0.0 统一标准)
BLADES_AUTH_TOKEN=${NEW_TOKEN}
PORT=8000
TZ=Asia/Shanghai
SEARXNG_URL=https://search.woha.men
BLADES_CPU_LIMIT=${BLADES_CPU_LIMIT}
BLADES_MEM_LIMIT=${BLADES_MEM_LIMIT}
GOMEMLIMIT=${GOMEMLIMIT}
EOF
    if [ "${TOKEN_GENERATED}" = "1" ]; then
        echo -e "${GREEN}--> 已生成随机访问口令并写入 .env，部署成功后会打印一次，请妥善保存${NC}"
    else
        echo -e "${GREEN}--> 已使用环境变量 BLADES_AUTH_TOKEN 写入 .env${NC}"
    fi
else
    # 检查现有 .env，确保必要参数就绪
    if ! grep -q '^BLADES_AUTH_TOKEN=' .env; then
        NEW_TOKEN="${AUTH_TOKEN_FROM_ENV:-$(gen_random_token)}"
        [ -z "$AUTH_TOKEN_FROM_ENV" ] && TOKEN_GENERATED=1
        echo "BLADES_AUTH_TOKEN=${NEW_TOKEN}" >> .env
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

AUTH_TOKEN=$(grep '^BLADES_AUTH_TOKEN=' .env | cut -d '=' -f2- | tr -d ' "')
APP_PORT=$(grep '^PORT=' .env | cut -d '=' -f2- | tr -d ' "')
APP_PORT=${APP_PORT:-8000}

# ──────────────────────────────────────────────────────────────────────────────
# 4.6 发布韧性 (P0)：快照「正在运行的上一版前端产物」
#
# 背景：部署瞬间，仍停留在旧页面的用户会去懒加载**旧哈希** chunk。若服务端拿不到
#       上一版资源，这些请求就是 404 → 用户白屏。服务端 (--previous-dist-dir)
#       会从 dist-previous 回退同名资源，所以**必须在替换容器之前**完成快照。
#       注意：服务端在启动时解析并缓存该目录，因此快照必须在 docker compose up 之前就位。
# ──────────────────────────────────────────────────────────────────────────────
PREV_DIR="$(pwd)/frontend/dist-previous"

# 原子替换目录：先把新内容放到临时位置，再整体切换，避免中途失败丢掉已有快照。
# 硬约束：**残缺的源目录绝不允许替换掉已有的完好快照**（否则回退链反而被弄断）。
install_prev_dir() {
    local src="$1"
    local old="${PREV_DIR}.old"

    # 入参校验：必须有 index.html，否则直接拒绝，不碰现有快照
    if [ ! -f "$src/index.html" ]; then
        return 1
    fi

    rm -rf "$old"
    [ -d "$PREV_DIR" ] && mv "$PREV_DIR" "$old"
    if mv "$src" "$PREV_DIR" 2>/dev/null && [ -f "$PREV_DIR/index.html" ]; then
        rm -rf "$old"
        return 0
    fi
    # 切换失败则回滚，保证原有快照不丢
    rm -rf "$PREV_DIR"
    [ -d "$old" ] && mv "$old" "$PREV_DIR"
    return 1
}

count_prev_chunks() {
    find "$PREV_DIR/assets" -type f -name '*.js' 2>/dev/null | wc -l | tr -d ' '
}

echo -e "${BLUE}[4.6/6] 构建镜像并快照旧版前端产物（发布韧性）...${NC}"
mkdir -p "$PREV_DIR"

# 先构建镜像（不启动），以便在替换容器前完成快照
docker compose build

if docker inspect -f '{{.State.Running}}' blades-stock 2>/dev/null | grep -q true; then
    TMP_PREV="$(mktemp -d)"
    if docker cp blades-stock:/app/frontend/dist/. "$TMP_PREV/" >/dev/null 2>&1 && [ -f "$TMP_PREV/index.html" ]; then
        if install_prev_dir "$TMP_PREV"; then
            echo -e "${GREEN}--> 已快照在线版本资源到 frontend/dist-previous/（$(count_prev_chunks) 个 JS chunk）${NC}"
        else
            echo -e "${YELLOW}[提示] 快照目录切换失败，已回滚至原有 dist-previous/${NC}"
        fi
    else
        rm -rf "$TMP_PREV"
        echo -e "${YELLOW}[提示] 未能从运行中容器抓取旧版产物，保留既有 dist-previous/${NC}"
    fi
else
    echo -e "${YELLOW}--> 未检测到运行中的容器（首次部署），无旧版可快照${NC}"
fi

# 首次部署播种：dist-previous 仍为空时，从刚构建好的镜像提取产物，
# 使「下一次」升级立即具备旧 chunk 回退能力。
if [ ! -f "$PREV_DIR/index.html" ]; then
    TMP_SEED="$(mktemp -d)"
    if docker create --name blades-prev-seed blades-stock:latest >/dev/null 2>&1; then
        if docker cp blades-prev-seed:/app/frontend/dist/. "$TMP_SEED/" >/dev/null 2>&1 && [ -f "$TMP_SEED/index.html" ]; then
            if install_prev_dir "$TMP_SEED"; then
                echo -e "${GREEN}--> 已用当前版本播种 frontend/dist-previous/，下次升级即受旧 chunk 回退保护${NC}"
            else
                echo -e "${YELLOW}[提示] 播种 dist-previous/ 失败，下次部署前请重跑本脚本${NC}"
            fi
        else
            rm -rf "$TMP_SEED"
        fi
        docker rm -f blades-prev-seed >/dev/null 2>&1
    else
        rm -rf "$TMP_SEED"
        echo -e "${YELLOW}[提示] 无法创建临时容器用于播种 dist-previous/${NC}"
    fi
fi

# 5. 启动 Docker 容器
echo -e "${BLUE}[5/6] 启动 Docker 服务集群 (应用 ${HW_PROFILE} 优化参数)...${NC}"
docker compose up -d

# 6. 等待服务就绪并健康探活
#
# 为什么不再固定 20×2s：首次冷启动需要完成数据库迁移、全市场标的字典种子导入
# 与分词词典加载，在 2C/4G 规格上经常明显超过 40s。旧实现会在服务其实**正常启动**
# 的情况下误报「健康检查超时」并 exit 1（紧接着的冒烟自检又全绿），
# 既让部署在自动化里假失败，也把用户训练成忽略警告。
# 现在：等待预算可通过 BLADES_HEALTH_MAX_WAIT 调整；容器已退出则提前止损；
# 真失败时直接打印容器日志，而不是让用户自己去翻。
echo -e "${BLUE}[6/6] 正在进行系统健康自检与端口探活...${NC}"
HEALTH_MAX_WAIT="${BLADES_HEALTH_MAX_WAIT:-180}"
HEALTHY=false
WAITED=0
LAST_CODE="000"
CONTAINER_DOWN=0

while [ "$WAITED" -lt "$HEALTH_MAX_WAIT" ]; do
    # 容器若已退出，再等下去毫无意义 —— 立刻止损。
    # 注意：只有 `docker compose ps` 本身成功执行时才据此判断，
    # 否则命令报错会得到一个空字符串，从而误判为“容器已退出”。
    RUNNING_SVCS=""
    if RUNNING_SVCS=$(docker compose ps --status running --services 2>/dev/null); then
        case "$RUNNING_SVCS" in
            *blades-stock*) : ;;            # 仍在运行，继续等待
            *) CONTAINER_DOWN=1 ;;          # 确实没在跑
        esac
    fi
    if [ "$CONTAINER_DOWN" = "1" ]; then
        echo -e "\n${RED}容器 blades-stock 未处于运行状态，已提前终止等待。${NC}"
        break
    fi

    LAST_CODE=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${APP_PORT}/health" || true)
    if [ "$LAST_CODE" = "200" ]; then
        HEALTHY=true
        break
    fi
    sleep 2
    WAITED=$((WAITED + 2))
    # 每 10 秒打印一次已等待秒数与当前状态码，便于区分“还没起来”和“起来了但在报错”
    if [ $((WAITED % 10)) -eq 0 ]; then
        echo -n "[${WAITED}s:${LAST_CODE}]"
    else
        echo -n "."
    fi
done
echo ""
if [ "$HEALTHY" = "true" ]; then
    echo -e "${GREEN}  ✅ 服务已就绪（耗时 ${WAITED}s，/health = 200）${NC}"
fi

if [ "$HEALTHY" = true ]; then
    # 6.5 部署后冒烟自检（防止“改 A 坏 B”：把关键不变量逐条验一遍）
    echo -e "${BLUE}[6.5/6] 执行部署后冒烟自检 (27 项关键不变量)...${NC}"
    SMOKE_FAILED=0
    if ! bash scripts/smoke-after-deploy.sh "http://127.0.0.1:${APP_PORT}"; then
        SMOKE_FAILED=1
    fi

    # 获取公网 IP
    PUBLIC_IP=$(curl -s https://api.ipify.org || curl -s https://ifconfig.me || echo "你的VPS公网IP")

    echo -e "\n${GREEN}==================================================================${NC}"
    echo -e "${GREEN}🎉 Blades Stock v3.0.0 工业级云原生投研工作站已成功部署！${NC}"
    echo -e "${GREEN}==================================================================${NC}"
    echo -e " 🌐 服务监听端口:       ${YELLOW}${APP_PORT}${NC}"
    echo -e " 🌐 网页直连访问:       ${YELLOW}http://${PUBLIC_IP}:${APP_PORT}${NC}"
    if [ "${TOKEN_GENERATED}" = "1" ]; then
        echo -e " 🔐 访问口令（本次随机生成，请立即妥善保存）: ${GREEN}${AUTH_TOKEN}${NC}"
    else
        echo -e " 🔐 访问口令: 使用你设定的 BLADES_AUTH_TOKEN（见 .env）"
    fi
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
    echo -e " 🛡️ 发布韧性自检:        curl -s http://127.0.0.1:${APP_PORT}/health"
    echo -e "    （previous_dist_armed=true 表示旧 chunk 回退已生效，部署不再打断在线用户）"
    echo -e " 🔍 冒烟自检重跑:        bash scripts/smoke-after-deploy.sh http://127.0.0.1:${APP_PORT}"
    echo -e "${GREEN}==================================================================${NC}\n"

    if [ "$SMOKE_FAILED" = "1" ]; then
        echo -e "${RED}==================================================================${NC}"
        echo -e "${RED}❌ 部署已完成，但冒烟自检存在失败项。${NC}"
        echo -e "${YELLOW}   请勿宣布发布成功 —— 请按上方 ❌ 清单逐项修复后重跑：${NC}"
        echo -e "${YELLOW}   bash scripts/smoke-after-deploy.sh http://127.0.0.1:${APP_PORT}${NC}"
        echo -e "${RED}==================================================================${NC}\n"
        exit 1
    fi
else
    echo -e "${RED}[失败] 服务在 ${HEALTH_MAX_WAIT}s 内未通过健康检查（最后一次 /health 返回 ${LAST_CODE}）。${NC}"
    echo -e "${YELLOW}-------- 最近 60 行容器日志（真正的失败原因通常就在这里）--------${NC}"
    docker compose logs --tail=60 blades-stock || true
    echo -e "${YELLOW}--------------------------------------------------------------${NC}"
    echo -e "${YELLOW}若确认是冷启动较慢（种子导入/词典加载），可跳过等待或延长预算：${NC}"
    echo -e "${YELLOW}  BLADES_HEALTH_MAX_WAIT=420 bash scripts/deploy-vps.sh${NC}"
    exit 1
fi
