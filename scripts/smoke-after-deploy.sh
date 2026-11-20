#!/usr/bin/env bash
# ==============================================================================
# Blades Stock — 部署后冒烟自检 (Post-Deploy Smoke Check)
#
# 目的：直接回应「改 A 坏 B」这类协同缺口 —— 每次部署后花 20 秒把关键不变量验一遍，
#       而不是等用户点开某个页面才发现坏了。
#
# 用法：
#   bash scripts/smoke-after-deploy.sh                  # 默认 http://127.0.0.1:8000
#   bash scripts/smoke-after-deploy.sh http://host:port
#   BLADES_AUTH_TOKEN=xxxx bash scripts/smoke-after-deploy.sh
#
# 退出码：0 = 全部通过；1 = 存在失败项（可直接用于 CI / 部署脚本闸门）
# ==============================================================================

set -uo pipefail

BASE="${1:-http://127.0.0.1:8000}"
TOKEN="${BLADES_AUTH_TOKEN:-}"
if [ -z "$TOKEN" ] && [ -f .env ]; then
    TOKEN="$(grep '^BLADES_AUTH_TOKEN=' .env 2>/dev/null | cut -d '=' -f2- | tr -d ' "')"
fi
if [ -z "$TOKEN" ]; then
    echo -e "${RED}[错误] 未找到访问口令：请设置环境变量 BLADES_AUTH_TOKEN 再运行，${NC}"
    echo -e "${RED}       或在部署目录的 .env 中配置 BLADES_AUTH_TOKEN。本脚本不再内置任何默认口令。${NC}"
    exit 1
fi

GREEN="\033[0;32m"; RED="\033[0;31m"; YELLOW="\033[1;33m"; BLUE="\033[0;34m"; NC="\033[0m"
PASS=0; FAIL=0

ok()   { PASS=$((PASS+1)); echo -e "  ${GREEN}✅${NC} $1"; }
bad()  { FAIL=$((FAIL+1)); echo -e "  ${RED}❌${NC} $1"; }
warn() { echo -e "  ${YELLOW}⚠️ ${NC} $1"; }
hdr()  { echo -e "\n${BLUE}[$1]${NC}"; }

# 取 HTTP 状态码
# 注意：绝不能写成 `curl ... || echo "000"` —— SSE 等长连接会因超时返回非 0 退出码
# （curl exit 28），此时 curl **已经**打印了 200，`||` 会再追加 "000"，得到 "200000"
# 与 "200" 比较失败，把正常的 SSE 误判为故障。
code() {
    local out
    out="$(curl -s -o /dev/null -w '%{http_code}' -m 15 "$@" 2>/dev/null)"
    printf '%s' "${out:-000}"
}

# 取首个响应头字段值（不区分大小写）
hdrval() {
    local want full
    want="$(echo "$1" | tr 'A-Z' 'a-z')"
    full="$(curl -s -o /dev/null -D - -m 15 "$2" 2>/dev/null | tr -d '\r' | awk -F': ' -v k="$want" 'tolower($1)==k {print $2; exit}')"
    printf '%s' "$full"
}

# 子串判定：绝不用 `echo "$BIG" | grep -q pat`
# 在 set -o pipefail 下，grep -q 命中即退出会让 echo 收到 SIGPIPE（141），
# 整个管道被判为非 0 —— 即使真正命中了也会走 else 分支。
# 对「禁止项」检查而言这是致命的假阴性（首屏出现 codemirror 也会报 ✅）。
has() { case "$1" in *"$2"*) return 0 ;; *) return 1 ;; esac; }

echo -e "${BLUE}==================================================================${NC}"
echo -e "${GREEN}   Blades Stock — 部署后冒烟自检${NC}"
echo -e "   目标: ${YELLOW}${BASE}${NC}"
echo -e "${BLUE}==================================================================${NC}"

# ──────────────────────────────────────────────────────────────────────────────
hdr "1/8 健康探针"
[ "$(code "${BASE}/health")" = "200" ] && ok "/health = 200" || bad "/health 非 200"
[ "$(code "${BASE}/live")" = "200" ]   && ok "/live = 200"   || bad "/live 非 200"
[ "$(code "${BASE}/ready")" = "200" ]  && ok "/ready = 200"  || bad "/ready 非 200"

# ──────────────────────────────────────────────────────────────────────────────
hdr "2/8 发布韧性（部署不得打断在线用户）"
HEALTH_JSON="$(curl -s -m 15 "${BASE}/health" || echo '')"
if has "$HEALTH_JSON" '"previous_dist_armed":true'; then
    ok "previous_dist_armed = true（旧 chunk 可回退）"
else
    bad "previous_dist_armed 非 true —— 部署仍会打断已打开页面的用户"
    warn "排查步骤："
    warn "  1) 宿主机目录必须存在且非空:  frontend/dist-previous/index.html"
    warn "  2) compose 必须挂载该目录:    ./frontend/dist-previous:/app/frontend/dist-previous:ro"
    warn "  3) 目录是容器启动时解析并缓存的 —— 补齐后需重启容器: docker compose restart blades-stock"
    warn "  4) deploy-vps.sh 步骤 4.6 会在替换容器前自动快照旧版产物；"
    warn "     若仍为空，直接重跑: bash scripts/deploy-vps.sh"
fi
MISS="$(echo "$HEALTH_JSON" | grep -o '"stale_chunk_miss":[0-9]*' | grep -o '[0-9]*$')"
if [ -z "$MISS" ]; then warn "未取到 stale_chunk_miss 指标"
elif [ "$MISS" = "0" ]; then ok "stale_chunk_miss = 0（无旧 chunk 真实缺失）"
else bad "stale_chunk_miss = ${MISS} —— 有用户正在白屏，回退链路可能断了"; fi

# ──────────────────────────────────────────────────────────────────────────────
hdr "3/8 首屏体积基线（关键 chunk 不得回到首屏）"
INDEX_HTML="$(curl -s -m 15 "${BASE}/" || echo '')"
PRELOADED="$(echo "$INDEX_HTML" | grep -o 'assets/[^"]*' | sort -u)"
COUNT="$(echo "$PRELOADED" | grep -c . || true)"
echo "     预加载资源数: ${COUNT}"
for forbidden in codemirror markdown tdesign; do
    if has "$(printf '%s' "$PRELOADED" | tr 'A-Z' 'a-z')" "$forbidden"; then
        bad "首屏预加载列表出现 '${forbidden}' chunk —— 首屏体积已回退"
    else
        ok "首屏无 '${forbidden}' chunk"
    fi
done
if [ "$COUNT" -le 10 ]; then ok "预加载资源数 ${COUNT} ≤ 10（基线 8）"; else warn "预加载资源数 ${COUNT} 高于基线 8，请核对"; fi

# ──────────────────────────────────────────────────────────────────────────────
hdr "4/8 静态资源缓存策略"
ENTRY="$(echo "$PRELOADED" | grep -E '^assets/index-.*\.js$' | head -1)"
if [ -n "$ENTRY" ]; then
    CC="$(curl -s -o /dev/null -D - -m 15 -H "Authorization: Bearer ${TOKEN}" "${BASE}/${ENTRY}" | tr -d '\r' | grep -i '^cache-control:' | cut -d' ' -f2- | head -1)"
    case "$CC" in
        "")          warn "未取到 /assets/ 的 Cache-Control（可能被中间代理剥离）" ;;
        *immutable*) ok "/assets/*.js → Cache-Control: ${CC}" ;;
        *)           bad "/assets/*.js 缓存策略异常: ${CC}（期望含 immutable）" ;;
    esac
else
    warn "未在 index.html 中找到入口 assets/index-*.js"
fi

# ──────────────────────────────────────────────────────────────────────────────
hdr "5/8 敏感路径必须 404（消除扫描误报 + 恢复 404 监控）"
for p in "/.git/config" "/Dockerfile" "/Makefile" "/package.json" "/.env" "/node_modules/x.js"; do
    c="$(code -H "Authorization: Bearer ${TOKEN}" "${BASE}${p}")"
    if [ "$c" = "404" ]; then ok "${p} = 404"; else bad "${p} = ${c}（期望 404，SPA 兜底把敏感路径伪装成 200）"; fi
done
[ "$(code -H "Authorization: Bearer ${TOKEN}" "${BASE}/api/docs")" = "404" ] \
    && ok "/api/docs = 404 (JSON)" || bad "/api/docs 非 404"

# ──────────────────────────────────────────────────────────────────────────────
hdr "6/8 SSE 实时推送链路"
[ "$(code -m 5 "${BASE}/api/events/stream")" = "401" ] \
    && ok "SSE 无凭证 = 401（鉴权有效）" || bad "SSE 无凭证未返回 401"
[ "$(code -m 5 -H "Authorization: Bearer ${TOKEN}" "${BASE}/api/events/stream")" = "200" ] \
    && ok "SSE 带 Bearer = 200" || bad "SSE 带 Bearer 非 200"
CT="$(curl -s -o /dev/null -D - -m 5 -H "Authorization: Bearer ${TOKEN}" "${BASE}/api/events/stream" 2>/dev/null | tr -d '\r' | awk 'tolower($1)=="content-type:" {print $2; exit}')"
if has "$CT" 'text/event-stream'; then ok "SSE Content-Type = ${CT}"; else bad "SSE Content-Type 异常: ${CT}"; fi

# ──────────────────────────────────────────────────────────────────────────────
hdr "7/8 安全响应头"
HDRS="$(curl -s -o /dev/null -D - -m 15 "${BASE}/" 2>/dev/null | tr -d '\r' | tr 'A-Z' 'a-z')"
for h in "strict-transport-security" "x-content-type-options" "x-frame-options" "content-security-policy"; do
    if has "$HDRS" "${h}:"; then ok "${h} 已注入"; else bad "${h} 缺失"; fi
done
CSP="$(curl -s -o /dev/null -D - -m 15 "${BASE}/" 2>/dev/null | tr -d '\r' | awk 'tolower($1)=="content-security-policy:" {$1=""; print; exit}')"
if has "$CSP" "unsafe-eval"; then bad "CSP 含 'unsafe-eval'"; else ok "CSP 不含 'unsafe-eval'"; fi

# ──────────────────────────────────────────────────────────────────────────────
hdr "8/8 产物内不得有被 CSP 阻断的资源引用"
if [ -n "$ENTRY" ]; then
    BUNDLE="$(curl -s -m 30 -H "Authorization: Bearer ${TOKEN}" "${BASE}/${ENTRY}" 2>/dev/null)"
    HTTP_EXT="$(printf '%s' "$BUNDLE" | grep -o 'http://[a-z0-9.-]*' 2>/dev/null | grep -vE 'localhost|127\.0\.0\.1|www\.w3\.org' 2>/dev/null | sort -u | head -5 || true)"
    if [ -z "$HTTP_EXT" ]; then ok "入口 bundle 无 http:// 外部资源"; else bad "入口 bundle 含被 CSP 阻断的 http:// 资源: ${HTTP_EXT}"; fi
    if has "$BUNDLE" "vite:preloadError"; then ok "staleChunkGuard 已打进产物"; else bad "产物中未见 vite:preloadError 监听"; fi
else
    warn "跳过（未找到入口 bundle）"
fi

# ──────────────────────────────────────────────────────────────────────────────
echo -e "\n${BLUE}==================================================================${NC}"
if [ "$FAIL" -eq 0 ]; then
    echo -e " ${GREEN}🎉 冒烟自检全部通过（${PASS} 项）${NC}"
    echo -e "${BLUE}==================================================================${NC}"
    exit 0
else
    echo -e " ${RED}❌ 冒烟自检失败：${FAIL} 项失败 / ${PASS} 项通过${NC}"
    echo -e " ${YELLOW}请先修复上述失败项，再宣布发布成功。${NC}"
    echo -e "${BLUE}==================================================================${NC}"
    exit 1
fi
