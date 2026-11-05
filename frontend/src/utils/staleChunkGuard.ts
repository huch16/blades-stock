/**
 * staleChunkGuard —— 部署热更新韧性守卫 (Stale Chunk Resilience Guard)
 *
 * ── 为什么需要它 ──────────────────────────────────────────────────────────────
 * Vite 生产构建为每个路由/组件生成带内容哈希的异步 chunk（如
 * `agent-chat-AmcV2WRK.js`）。当运维执行一次 `docker compose up -d --build`
 * 时，容器内的 `frontend/dist` 被整体替换，**所有旧哈希 chunk 立刻消失**。
 *
 * 此时任何「已经打开着旧页面」的用户，只要点击懒加载路由，浏览器就会去请求
 * 一个已经不存在的 chunk 文件 → 404 → 动态 `import()` 返回 rejected Promise。
 *
 * 这条失败链路发生在**组件挂载之前**，因此 Vue 的 `onErrorCaptured` /
 * 视图级 ErrorBoundary **完全无法捕获**，表现为：点击菜单无反应或整页白屏。
 *
 * ── 三层兜底 ─────────────────────────────────────────────────────────────────
 *  1. `vite:preloadError`（Vite 官方在 `__vitePreload` 失败时派发的事件）
 *     → 自动执行一次「静默整页刷新」，把用户无缝带到最新版本。
 *  2. 冷却窗口（默认 15s）+ sessionStorage 标记
 *     → 严格防止「刷新 → 又失败 → 再刷新」的无限重载风暴。
 *  3. 冷却期内仍然失败 → 降级为页面顶部的可关闭提示条，明确告知用户手动刷新，
 *     而不是让用户面对一片空白。
 *
 * 额外覆盖 `unhandledrejection`：部分旧版浏览器 / 非 Vite preload 路径下
 * （例如组件内自行调用的 `import()`），错误只会落到全局 Promise 拒绝，不会
 * 派发 `vite:preloadError`。
 */

const RELOAD_STAMP_KEY = 'blades_stock_chunk_recover_at'
const RELOAD_COOLDOWN_MS = 15_000
const BANNER_ID = 'blades-stock-stale-chunk-banner'

/** 识别各类「模块加载失败」错误，避免把业务异常误判为 chunk 失效 */
const STALE_CHUNK_PATTERN =
  /(Failed to fetch dynamically imported module|Importing a module script failed|error loading dynamically imported module|dynamically imported module|Unable to preload CSS|Load failed)/i

function extractMessage(payload: unknown): string {
  if (!payload) return ''
  if (typeof payload === 'string') return payload
  const anyPayload = payload as { message?: unknown; reason?: unknown; error?: unknown }
  for (const candidate of [anyPayload.message, anyPayload.reason, anyPayload.error]) {
    if (typeof candidate === 'string') return candidate
    if (candidate && typeof (candidate as Error).message === 'string') {
      return (candidate as Error).message
    }
  }
  return String(payload)
}

function isStaleChunkError(payload: unknown): boolean {
  return STALE_CHUNK_PATTERN.test(extractMessage(payload))
}

function readLastReloadAt(): number {
  try {
    return Number(sessionStorage.getItem(RELOAD_STAMP_KEY) || 0) || 0
  } catch {
    return 0
  }
}

function markReloaded(): void {
  try {
    sessionStorage.setItem(RELOAD_STAMP_KEY, String(Date.now()))
  } catch {
    /* 隐私模式下 sessionStorage 不可写，忽略即可 */
  }
}

/**
 * 尝试自动恢复。返回 true 表示已发起刷新（调用方应立即终止后续处理）。
 */
function tryAutoRecover(): boolean {
  if (typeof window === 'undefined') return false

  const now = Date.now()
  const last = readLastReloadAt()

  if (now - last <= RELOAD_COOLDOWN_MS) {
    // 冷却窗口内 —— 说明刷新一次并未解决问题（可能是服务端异常或网络故障），
    // 必须停手，否则会把用户拖入无限刷新风暴。
    return false
  }

  markReloaded()
  forceReload()
  return true
}

/**
 * 强制整页刷新。
 *
 * ⚠️ 绝不能写成 `window.location.replace(window.location.href)`：
 * 本应用使用 `createWebHashHistory()`（hash 路由），URL 中只有 `#/xxx` 部分不同时，
 * 任何形式的 `location.replace/assign(同一 href)` 都会被浏览器按「仅片段导航」处理
 * ——**不会重新加载文档**，于是自动恢复静默失效（已由 E2E 实测捕获）。
 * `location.reload()` 会真正重新拉取文档，且不会新增历史记录，是本场景的正确做法。
 */
function forceReload(): void {
  try {
    window.location.reload()
  } catch {
    // 某些沙箱/WebView 环境下 reload 受限，退化为带时间戳的显式导航
    const url = new URL(window.location.href)
    url.searchParams.set('_r', String(Date.now()))
    window.location.replace(url.toString())
  }
}

/** 降级提示条：不依赖任何 UI 库，确保在最坏情况下依然可见 */
function showManualRefreshBanner(): void {
  if (typeof document === 'undefined') return
  if (document.getElementById(BANNER_ID)) return

  const banner = document.createElement('div')
  banner.id = BANNER_ID
  banner.setAttribute('role', 'alert')
  banner.style.cssText = [
    'position:fixed',
    'top:0',
    'left:0',
    'right:0',
    'z-index:2147483647',
    'display:flex',
    'align-items:center',
    'justify-content:center',
    'gap:12px',
    'padding:10px 16px',
    'background:#ad6800',
    'color:#fff',
    'font-size:13px',
    'line-height:1.5',
    'font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Microsoft YaHei",sans-serif',
    'box-shadow:0 2px 12px rgba(0,0,0,.28)'
  ].join(';')

  const text = document.createElement('span')
  text.textContent = '检测到系统已发布新版本，当前页面资源已过期，请手动刷新页面以继续使用。'

  const button = document.createElement('button')
  button.type = 'button'
  button.textContent = '立即刷新'
  button.style.cssText = [
    'cursor:pointer',
    'border:1px solid rgba(255,255,255,.8)',
    'background:transparent',
    'color:#fff',
    'border-radius:4px',
    'padding:3px 12px',
    'font-size:13px'
  ].join(';')
  button.addEventListener('click', () => {
    markReloaded()
    forceReload()
  })

  const close = document.createElement('button')
  close.type = 'button'
  close.textContent = '×'
  close.setAttribute('aria-label', '关闭提示')
  close.style.cssText = [
    'cursor:pointer',
    'border:none',
    'background:transparent',
    'color:#fff',
    'font-size:18px',
    'line-height:1',
    'padding:0 4px'
  ].join(';')
  close.addEventListener('click', () => banner.remove())

  banner.appendChild(text)
  banner.appendChild(button)
  banner.appendChild(close)

  const mount = () => document.body && document.body.appendChild(banner)
  if (document.body) mount()
  else document.addEventListener('DOMContentLoaded', mount, { once: true })
}

/**
 * 单独暴露给 router 的兜底钩子：路由级 chunk 加载失败时同样走这套恢复逻辑。
 * 返回 true 表示「已接管」该错误（调用方不应再抛错）。
 */
export function handleStaleChunkFailure(payload?: unknown): boolean {
  if (tryAutoRecover()) return true
  // 冷却窗口内已放弃自动刷新：把原始错误留给控制台，便于运维定位是哪一个 chunk 缺失
  console.warn('[StaleChunkGuard] 自动刷新已熔断（15s 冷却），转为手动刷新提示:', extractMessage(payload))
  showManualRefreshBanner()
  return true
}

let installed = false

/** 安装全局守卫（幂等，重复调用无副作用） */
export function installStaleChunkGuard(): void {
  if (installed || typeof window === 'undefined') return
  installed = true

  // ① Vite 官方事件：预加载 / 动态导入失败
  window.addEventListener(
    'vite:preloadError',
    (event: Event) => {
      const payload = (event as Event & { payload?: unknown }).payload
      if (!isStaleChunkError(payload)) {
        // 非 chunk 失效类错误交还给原有处理链（例如 React/Vue 生态的其它 preload 错误）
        return
      }
      // 阻止 Vite 默认的 `throw` 行为，否则错误会冒泡成 unhandledrejection
      event.preventDefault()
      const msg = extractMessage(payload)
      console.warn('[StaleChunkGuard] 检测到过期 chunk，正在尝试自动恢复:', msg)
      handleStaleChunkFailure(payload)
    },
    true
  )

  // ② 全局 Promise 拒绝兜底：覆盖不经由 Vite preload helper 的动态导入
  window.addEventListener('unhandledrejection', (event: PromiseRejectionEvent) => {
    if (!isStaleChunkError(event.reason)) return
    event.preventDefault()
    const msg = extractMessage(event.reason)
    console.warn('[StaleChunkGuard] 捕获到动态模块加载拒绝，正在尝试自动恢复:', msg)
    handleStaleChunkFailure(event.reason)
  })

  // ③ 成功加载后立即清理标记，确保「下一次真实部署」仍能获得一次自动刷新机会
  window.addEventListener('load', () => {
    window.setTimeout(() => {
      try {
        sessionStorage.removeItem(RELOAD_STAMP_KEY)
      } catch {
        /* ignore */
      }
    }, 0)
  })
}

/**
 * 计算「当前页面 chunk 是否已过期」的探测函数（供监控/自检使用）。
 * 通过比对 document 中已注入的入口脚本与服务器 index.html 是否一致实现。
 */
export async function probeDeployedBundleHash(): Promise<string | null> {
  try {
    const res = await fetch(`${window.location.origin}/?t=${Date.now()}`, {
      cache: 'no-store',
      headers: { 'Cache-Control': 'no-cache' }
    })
    if (!res.ok) return null
    const html = await res.text()
    const match = html.match(/assets\/index-([A-Za-z0-9_-]+)\.js/)
    return match ? match[1] : null
  } catch {
    return null
  }
}
