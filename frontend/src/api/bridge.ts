/**
 * Universal Bridge (通用通信适配桥接层)
 *
 * 核心职责：
 * 1. 自动探测运行环境（Wails 桌面客户端 VS 浏览器 Web 模式）；
 * 2. 在 Web 模式下通过 Proxy 托管 window.go.main.App 与 window.runtime；
 * 3. 完整实现 Wails Runtime 运行时的全套事件接口（EventsOnMultiple、EventsOffAll、全屏控制、剪贴板等）；
 * 4. 透明将 Wails IPC 转化为 HTTP RPC 与 Server-Sent Events (SSE) 长连接；
 * 5. 统一公网安全令牌鉴权与 401 拦截处理。
 */

// 1. 环境感知
export const isWails = (): boolean => {
  if (typeof window === 'undefined') return false
  // 核心判断：原生 Wails 客户端必定存在宿主注入的 window.WailsInvoke 或 window._wails
  if (typeof (window as any).WailsInvoke !== 'undefined' || typeof (window as any)._wails !== 'undefined') {
    return true
  }
  // 若已显式标记为 Web Universal Bridge 代理模式，则必定是 Web 浏览器模式
  if ((window as any).__BLADES_WEB_BRIDGE__) {
    return false
  }
  return (
    typeof (window as any).runtime !== 'undefined' &&
    typeof (window as any).go !== 'undefined' &&
    Boolean((window as any).go?.main?.App)
  )
}

// 2. 鉴权 Token 管理
const AUTH_TOKEN_KEY = 'blades_stock_auth_token'

export const getAuthToken = (): string => {
  if (typeof window === 'undefined') return ''
  return localStorage.getItem(AUTH_TOKEN_KEY) || ''
}

export const setAuthToken = (token: string): void => {
  if (typeof window === 'undefined') return
  localStorage.setItem(AUTH_TOKEN_KEY, token.trim())
}

export const clearAuthToken = (): void => {
  if (typeof window === 'undefined') return
  localStorage.removeItem(AUTH_TOKEN_KEY)
}

export const getAuthStatus = (): { authenticated: boolean; token: string; isWeb: boolean } => {
  const token = getAuthToken()
  return {
    authenticated: Boolean(token),
    token,
    isWeb: !isWails(),
  }
}

/**
 * 唤起安全鉴权弹窗（支持用户在设置或顶部主动打开管理口令）
 */
export const openAuthModal = (mode: 'login' | 'manage' = 'manage'): void => {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent('blades:open-auth-modal', { detail: { mode } }))
}

/**
 * 注销当前 Web 访问口令并清理环境
 */
export const logoutWebAuth = (): void => {
  clearAuthToken()
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('blades:unauthorized'))
  }
}

// 3. Web 模式下的全局事件总线（完全对齐 Wails Events 语义）

/**
 * Wails 事件回调签名。
 *
 * 修复（@typescript-eslint/no-unsafe-function-type）：此前统一写作内置 `Function`，
 * 它会吞掉所有参数与返回值类型（`Function` 等价于「任意可调用值」），
 * 既无法静态校验调用点，也掩盖了真实的类型错误。
 * 事件总线本身对载荷不做约束，因此统一使用 `unknown[]` 而非 `any[]`，
 * 使调用方必须显式收窄类型。
 */
type EventCallback = (...args: unknown[]) => void

interface ListenerWrapper {
  callback: EventCallback
  maxCallbacks: number
  calledCount: number
}

class WebEventBus {
  private listeners: Map<string, ListenerWrapper[]> = new Map()
  private sseSource: EventSource | null = null
  private sseConnected: boolean = false
  private retryTimer: any = null
  private attachedEvents: Set<string> = new Set()
  private retryCount: number = 0
  private readonly maxRetryDelay: number = 30000
  private readonly baseRetryDelay: number = 2000

  constructor() {
    if (typeof window !== 'undefined') {
      window.addEventListener('online', () => {
        if (!isWails()) {
          this.reconnectSSE()
        }
      })
      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible' && !this.sseConnected && !isWails()) {
          this.reconnectSSE()
        }
      })
    }
  }

  public onMultiple(event: string, callback: EventCallback, maxCallbacks: number = -1): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, [])
    }
    const list = this.listeners.get(event)!
    // 幂等防重守卫：同一回调函数绝不重复添加，彻底杜绝多次调用 EventsOn 引发的流式打字机重复回显！
    if (list.some(item => item.callback === callback)) {
      return
    }
    list.push({
      callback,
      maxCallbacks,
      calledCount: 0,
    })
    this.ensureSSEConnection()
    this.attachSSEListener(event)
  }

  public off(eventName: string, callbackOrNextEvent?: EventCallback | string, ...rest: (EventCallback | string)[]): void {
    if (typeof callbackOrNextEvent === 'function') {
      const list = this.listeners.get(eventName)
      if (list) {
        this.listeners.set(eventName, list.filter(item => item.callback !== callbackOrNextEvent))
      }
      return
    }

    const targets = [eventName]
    if (typeof callbackOrNextEvent === 'string') {
      targets.push(callbackOrNextEvent)
    }
    for (const item of rest) {
      if (typeof item === 'string') {
        targets.push(item)
      }
    }
    for (const name of targets) {
      this.listeners.delete(name)
    }
  }

  public offAll(): void {
    this.listeners.clear()
  }

  public emit(event: string, ...args: any[]): void {
    const list = this.listeners.get(event)
    if (!list || list.length === 0) return

    // 复制数组防止遍历中修改自身
    const toExecute = [...list]
    for (let i = 0; i < toExecute.length; i++) {
      const item = toExecute[i]
      try {
        item.callback(...args)
      } catch (e) {
        console.error(`[WebEventBus] Callback error on event "${event}":`, e)
      }
      item.calledCount++
      if (item.maxCallbacks > 0 && item.calledCount >= item.maxCallbacks) {
        // 从原始列表中移除
        const origList = this.listeners.get(event)
        if (origList) {
          const idx = origList.indexOf(item)
          if (idx !== -1) {
            origList.splice(idx, 1)
          }
        }
      }
    }
  }

  private handleSSEMessage(e: MessageEvent, explicitEvent?: string): void {
    try {
      const payload = JSON.parse(e.data)
      const eventName = explicitEvent || (payload && payload.event) || 'message'
      const eventData = (payload && typeof payload === 'object' && 'data' in payload) ? payload.data : payload
      this.emit(eventName, eventData)
    } catch (_err) {
      this.emit(explicitEvent || 'message', e.data)
    }
  }

  private attachSSEListener(event: string): void {
    if (!this.sseSource || this.attachedEvents.has(event)) return
    this.attachedEvents.add(event)
    this.sseSource.addEventListener(event, (e: any) => {
      this.handleSSEMessage(e, event)
    })
  }

  // 保证与后端的 SSE 长连接连通
  public ensureSSEConnection(): void {
    if (this.sseConnected || typeof window === 'undefined') return
    const token = getAuthToken()
    // 在 Web 模式下，未提供访问 Token 前严禁盲目建立 SSE 连接，避免引发 401 死循环与服务器长连接雪崩
    if (!token && !isWails()) {
      return
    }
    // 修复 P0-1: 严禁在 URL Query 中明文拼接 Token，杜绝其泄露至 Nginx / Cloudflare 边缘访问日志与 Referer
    // 现代浏览器在同源或跨域白名单环境下通过安全 HttpOnly Cookie (blades_token) 自动隐式传递鉴权会话
    // 同时配置 withCredentials: true，确保在跨端口/反向代理开发与沙箱集成中凭据不丢失
    const url = '/api/events/stream'

    try {
      this.sseSource = new EventSource(url, { withCredentials: true } as any)
      this.sseConnected = true
      this.attachedEvents.clear()

      this.sseSource.onopen = () => {
        this.sseConnected = true
        this.retryCount = 0
      }

      this.sseSource.onmessage = (e) => {
        this.retryCount = 0
        this.handleSSEMessage(e)
      }

      // 为当前已注册的所有前端监听事件挂载 addEventListener，提供双重保障
      for (const evt of this.listeners.keys()) {
        this.attachSSEListener(evt)
      }
      const defaultEvents = ['newChatStream', 'agent-message', 'marketIntelligenceReport', 'stockPriceAlertTriggered', 'refreshFollowList', 'loadingDone']
      for (const evt of defaultEvents) {
        this.attachSSEListener(evt)
      }

      this.sseSource.onerror = () => {
        this.sseConnected = false
        this.attachedEvents.clear()
        if (this.sseSource) {
          this.sseSource.close()
          this.sseSource = null
        }
        if (!this.retryTimer) {
          const delay = Math.min(
            this.maxRetryDelay,
            Math.round(this.baseRetryDelay * Math.pow(1.5, Math.min(this.retryCount, 8)) * (0.8 + Math.random() * 0.4))
          )
          this.retryCount++
          this.retryTimer = setTimeout(() => {
            this.retryTimer = null
            this.ensureSSEConnection()
          }, delay)
        }
      }
    } catch (_e) {
      this.sseConnected = false
    }
  }

  // 认证成功后强制重连 SSE
  public reconnectSSE(): void {
    if (this.sseSource) {
      this.sseSource.close()
      this.sseSource = null
    }
    this.sseConnected = false
    this.retryCount = 0
    if (this.retryTimer) {
      clearTimeout(this.retryTimer)
      this.retryTimer = null
    }
    this.ensureSSEConnection()
  }
}

export const webEventBus = new WebEventBus()

let lastUnauthorizedDispatch = 0

// ==========================================
// 4. 高性能请求聚合、去重与微缓存引擎 (Request Coalescing & Micro-Cache)
// ==========================================

// 正在飞行中的并发请求池 (用于拦截同瞬时重复请求，返回同一个 Promise)
const inFlightRequests = new Map<string, Promise<any>>()

// 高频幂等只读查询的短期内存缓存 (TTL 毫秒)
interface CacheEntry {
  data: any
  expiresAt: number
}
const queryCache = new Map<string, CacheEntry>()

// 允许短期微缓存的 RPC 方法配置及 TTL (毫秒)
const TTL_CACHE_RULES: Record<string, number> = {
  GetConfig: 15_000,           // 15s: 避免各组件 onMounted 狂轰滥炸，UpdateConfig 时立即作废
  GetAiConfigs: 30_000,        // 30s: AI 模型列表
  GetVersionInfo: 60_000,      // 60s: 系统版本极少变更
  IsTradingTime: 5_000,        // 5s: 交易时段检测
  IsHKTradingTime: 5_000,      // 5s
  IsUSTradingTime: 5_000,      // 5s
  IsTradingDay: 15_000,        // 15s
  GetLatestTradingDay: 15_000, // 15s
  GetGroupList: 10_000,        // 10s: 分组列表
  GetFollowList: 15_000,       // 15s: 自选股列表 (修复 P1-2: 补齐自选股缓存)
  GetStockList: 86_400_000,    // 24小时: 修复 P1-1，防止 1.15MB 全量股票字典在交易时段重复下发消耗数百兆流量
}

// 修复 P1-2: 严禁使用子串 includes("Config") 模糊匹配（否则会导致 GetAiConfigs 误伤清空 GetConfig 缓存）
// 改用精确白名单映射字典，仅在发生真实写操作时定向失效关联只读查询
const MUTATION_INVALIDATION_MAP: Record<string, string[]> = {
  UpdateConfig: ['GetConfig', 'GetAiConfigs'],
  AddGroup: ['GetGroupList'],
  RemoveGroup: ['GetGroupList'],
  UpdateGroup: ['GetGroupList'],
  UpdateGroupSort: ['GetGroupList'],
  AddStockGroup: ['GetGroupList'],
  RemoveStockGroup: ['GetGroupList'],
  Follow: ['GetFollowList'],
  UnFollow: ['GetFollowList'],
  FollowFund: ['GetFollowedFund', 'GetFollowedFundPaged'],
  SaveStockThesis: ['GetStockThesis', 'GetAllStockTheses', 'GetStockThesisHistory'],
  DeleteStockThesis: ['GetStockThesis', 'GetAllStockTheses', 'GetStockThesisHistory'],
}

// 变更操作自动作废关联缓存
function invalidateQueryCacheOnMutation(method: string) {  const targets = MUTATION_INVALIDATION_MAP[method]
  if (targets && targets.length > 0) {
    for (const target of targets) {
      for (const key of queryCache.keys()) {
        if (key.startsWith(`${target}:`)) {
          queryCache.delete(key)
        }
      }
    }
  }
}

/**
 * 将非 2xx 的 HTTP 响应格式化为**简洁可读**的错误信息。
 *
 * 背景：反向代理 / Cloudflare 在上游超时或宕机时会返回一整页 HTML
 *（典型如 “502 Bad Gateway”）。旧实现直接 `HTTP 502: <html>...</html>` 抛给上层，
 * 最终会被原样渲染到用户界面与通知弹窗里：既泄露内部网关信息，
 * 也让用户完全看不懂到底发生了什么（实测线上出现过整页 HTML 弹窗）。
 *
 * 现在：用户只看到简洁的中文诊断；HTML 全文仅进控制台日志供排查。
 * 注意：错误文本中保留 “HTTP <status>” 前缀，上层按状态码分支的逻辑仍然有效。
 */
function formatHttpError(method: string, status: number, rawText: string): string {
  const raw = (rawText || '').trim()
  const looksLikeHtml = /^\s*<(!doctype|html|\?xml)/i.test(raw) || /<html[\s>]/i.test(raw.slice(0, 500))

  // 必须说清楚**这是本系统的网关**返回的，不是你填的模型服务商。
  // 历史文案把两者混为一谈，用户看到 502 就去怪 DeepSeek/网关，方向完全错了。
  const gatewayHints: Record<number, string> = {
    502: `本系统网关错误 (Bad Gateway)：请求未能完整送达/送达后未及时响应，通常由反向代理或隧道瞬时中断、或调用耗时超出网关容忍度引起。这是**本系统入口**返回的，不代表你配置的模型服务商有问题。请稍后重试；若反复出现，请检查部署入口（nginx / Cloudflare 隧道）日志。`,
    503: `本系统服务暂不可用 (Service Unavailable)：容器正在重启或过载。这是**本系统入口**返回的，与模型服务商无关，请稍后重试。`,
    504: `本系统网关超时 (Gateway Timeout)：调用耗时超出网关容忍度已被中断。这是**本系统入口**返回的，请稍后重试。`,
  }
  const hint = gatewayHints[status]
  if (hint) {
    console.error(`[UniversalBridge] ${method} HTTP ${status} raw body:`, raw)
    return `HTTP ${status} [接口 ${method}] ${hint}`
  }
  if (looksLikeHtml) {
    console.error(`[UniversalBridge] ${method} HTTP ${status} HTML body:`, raw)
    return `HTTP ${status}: 服务返回了非预期的 HTML 页面（通常是反向代理错误页），详细原因请查看服务端日志。`
  }
  const detail = raw.length > 300 ? raw.slice(0, 300) + '…' : raw
  return `HTTP ${status}: ${detail || '请求失败'}`
}

// 默认 RPC 超时预算（普通查询/写入接口足够）
const DEFAULT_RPC_TIMEOUT_MS = 30_000

// 长耗时生成类接口的超时预算：与服务端 10 分钟后台推演 context 对齐
const LONG_RUNNING_TIMEOUT_MS = 600_000

/**
 * 这些接口在服务端会真正跑一次完整的大模型长文推演（各自持有独立的
 * 10 分钟 context），耗时 40~90s 是常态，绝不能套用 30s 熔断。
 * 依据：backend/agent/market_lifecycle.go 中 4 个方法均使用
 * context.WithTimeout(context.Background(), 10*time.Minute)。
 */
const LONG_RUNNING_METHODS = new Set([
  'GeneratePreMarketBriefing',
  'GeneratePostMarketReview',
  'GenerateETFRotationReport',
  'GenerateSerenityResearchReport'
  // 注：曾列有 'TriggerSectorAnomalySerenityAnalysis'，但它**未注册为 RPC**
  //（它是内部触发路径，由价格异动监视器调用；其能力已通过
  //  GenerateSerenityResearchReport 对外提供）。留在超时表里属于死配置，已移除。
])

// 4. Web 模式下的通用 HTTP RPC 发起器
export async function callWebApi(method: string, ...args: any[]): Promise<any> {
  const token = getAuthToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  // 注入链路追踪 Request-ID
  const requestId = `req-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
  headers['X-Request-ID'] = requestId

  const serializedArgs = JSON.stringify(args)
  const cacheKey = `${method}:${serializedArgs}`

  // 1. 若命中短期内存微缓存，直接同步返回副本，零网络开销
  const cached = queryCache.get(cacheKey)
  if (cached && Date.now() < cached.expiresAt) {
    return cached.data
  }

  // 2. 判定该请求是否支持并发合并（仅针对查询类、幂等类接口启用）
  const isQueryMethod =
    method.startsWith('Get') ||
    method.startsWith('Is') ||
    method.startsWith('Search') ||
    method.startsWith('Filter') ||
    method.startsWith('Calculate')

  // 若当前已有完全相同的网络请求正在飞行中，直接复用该 Promise，彻底根除请求瀑布
  if (isQueryMethod && inFlightRequests.has(cacheKey)) {
    return inFlightRequests.get(cacheKey)!
  }

  const url = `/api/App/${method}`

  // 超时预算：默认 30s 硬熔断，防止慢速接口无截止时间悬挂占用连接。
  //
  // 但**长耗时生成类接口必须给足预算**：它们在服务端用独立的 10 分钟
  // context 在后台推演（market_lifecycle.go），nginx 侧 proxy_read_timeout
  // 也是 600s。实测 GeneratePreMarketBriefing 正常耗时约 41s（40~90s 是常态）。
  // 统一 30s 熔断会在报告生成完成前就提前掐断，用户看到的是
  // 「研报生成失败」—— 而报告其实还在后台正常生成。这正是
  // 「盘前推演等失败」的根因（与模型好坏无关）。
  const timeoutMs = LONG_RUNNING_METHODS.has(method)
    ? LONG_RUNNING_TIMEOUT_MS
    : DEFAULT_RPC_TIMEOUT_MS
  const controller = typeof AbortController !== 'undefined' ? new AbortController() : null
  const timer = controller ? setTimeout(() => controller.abort(), timeoutMs) : null

  const fetchPromise = (async () => {
    try {
      const response = await fetch(url, {
        method: 'POST',
        headers,
        body: serializedArgs,
        signal: controller ? controller.signal : undefined,
      })
      if (timer) clearTimeout(timer)

      if (response.status === 401) {
        clearAuthToken()
        const now = Date.now()
        // 节流派发 blades:unauthorized 事件，防止数十个并发请求瞬间引爆事件总线
        if (now - lastUnauthorizedDispatch > 2000) {
          lastUnauthorizedDispatch = now
          window.dispatchEvent(new CustomEvent('blades:unauthorized'))
        }
        throw new Error('未授权访问：请先输入有效访问口令')
      }

      if (!response.ok) {
        const errText = await response.text().catch(() => '')
        throw new Error(formatHttpError(method, response.status, errText))
      }

      const contentType = response.headers.get('content-type') || ''
      let result: any
      if (contentType.includes('application/json')) {
        const json = await response.json()
        if (json && typeof json === 'object') {
          if ('code' in json && json.code !== 0) {
            throw new Error(json.error || json.message || `请求失败 (code: ${json.code})`)
          }
          if ('data' in json) {
            result = json.data
          } else {
            result = json
          }
        } else {
          result = json
        }
      } else {
        result = await response.text()
      }

      // 3. 成功返回后，处理微缓存与写操作失效
      invalidateQueryCacheOnMutation(method)

      const ttl = TTL_CACHE_RULES[method]
      if (ttl && ttl > 0) {
        queryCache.set(cacheKey, {
          data: result,
          expiresAt: Date.now() + ttl,
        })
      }

      return result
    } catch (err: any) {
      if (timer) clearTimeout(timer)
      if (err.name === 'AbortError') {
        const secs = Math.round(timeoutMs / 1000)
        console.warn(`[UniversalBridge] RPC call to "${method}" timed out after ${secs}s.`)
        // 附带 cause，保留原始 AbortError 以供上层排查（preserve-caught-error）
        // 文案必须包含中文「超时 / 熔断」，因为长耗时生成类页面（如市场研报抽屉）
        // 靠关键词判断是否切入「后台持续生成中」的轮询兜底；
        // 旧文案是纯中文而抽屉只匹配英文 'timeout' → 永远匹配不上，直接报失败。
        throw new Error(
          `请求超时：接口 "${method}" 响应耗时超过 ${secs} 秒，已执行保护性熔断（timeout）`,
          { cause: err }
        )
      }
      console.error(`[UniversalBridge] RPC call to "${method}" (${requestId}) failed:`, err)
      throw err
    } finally {
      // 4. 请求结束，从正在飞行队列中移除
      inFlightRequests.delete(cacheKey)
    }
  })()

  if (isQueryMethod) {
    inFlightRequests.set(cacheKey, fetchPromise)
  }

  return fetchPromise
}

// 5. 全局运行时桩注入（最早时刻执行）
export function initUniversalBridge(): void {  if (typeof window === 'undefined') return

  const win = window as any
  // 原生 Wails 桌面环境判断（支持原生 WailsInvoke、_wails 或原生 runtime.EventsOn）：
  // 命中原生桌面特征时绝对放行，绝不可误注入 Web 代理桩！
  const isNativeWails =
    typeof win.WailsInvoke !== 'undefined' ||
    typeof win._wails !== 'undefined' ||
    (typeof win.runtime !== 'undefined' && typeof win.runtime.EventsOn === 'function' && !win.__BLADES_WEB_BRIDGE__) ||
    (typeof win.go !== 'undefined' && Boolean(win.go?.main?.App) && !win.__BLADES_WEB_BRIDGE__)

  if (isNativeWails) {
    console.log('[UniversalBridge] Running in Native Wails Desktop mode.')
    delete win.__BLADES_WEB_BRIDGE__
    return
  }

  // 显式标记当前处于 Web Universal Bridge 模式，防止代理对象被误判为原生 Wails
  win.__BLADES_WEB_BRIDGE__ = true
  console.log('[UniversalBridge] Running in Cloud-Native Web/Browser mode. Installing proxy stubs...')

  win.go = win.go || {}
  win.go.main = win.go.main || {}

  // 1) 注入 window.go.main.App 的动态代理
  if (!win.go.main.App) {
    win.go.main.App = new Proxy({}, {
      get(_target, propKey: string) {
        return (...args: any[]) => {
          return callWebApi(propKey, ...args)
        }
      },
    })
  }

  // 2) 注入 window.runtime 的完整方法桩（完全对齐 wailsjs/runtime/runtime.js）
  if (!win.runtime) {
    win.runtime = {
      // 事件系统
      EventsOnMultiple: (eventName: string, callback: EventCallback, maxCallbacks: number) => {
        webEventBus.onMultiple(eventName, callback, maxCallbacks)
      },
      EventsOn: (eventName: string, callback: EventCallback) => {
        webEventBus.onMultiple(eventName, callback, -1)
      },
      EventsOnce: (eventName: string, callback: EventCallback) => {
        webEventBus.onMultiple(eventName, callback, 1)
      },
      EventsOff: (eventName: string, ...additionalArgs: any[]) => {
        webEventBus.off(eventName, additionalArgs[0], ...additionalArgs.slice(1))
      },
      EventsOffAll: () => {
        webEventBus.offAll()
      },
      EventsEmit: (eventName: string, ...args: any[]) => {
        webEventBus.emit(eventName, ...args)
      },

      // 浏览器与外链
      BrowserOpenURL: (url: string) => {
        window.open(url, '_blank')
      },

      // 运行环境信息探测（对齐 Wails Environment 返回结构）
      Environment: async () => {
        let platform = 'web'
        if (typeof navigator !== 'undefined') {
          const ua = (navigator.userAgent || '').toLowerCase()
          if (ua.includes('win')) platform = 'windows'
          else if (ua.includes('mac')) platform = 'darwin'
          else if (ua.includes('linux')) platform = 'linux'
        }
        return {
          buildType: 'production',
          platform,
          arch: 'amd64'
        }
      },

      // 窗口与全屏控制（利用 HTML5 规范 API 实现）
      WindowFullscreen: () => {
        if (!document.fullscreenElement) {
          document.documentElement.requestFullscreen().catch(() => {})
        }
      },
      WindowUnfullscreen: () => {
        if (document.fullscreenElement) {
          document.exitFullscreen().catch(() => {})
        }
      },
      WindowIsFullscreen: async () => {
        return Boolean(document.fullscreenElement)
      },
      WindowReload: () => {
        window.location.reload()
      },
      WindowReloadApp: () => {
        window.location.reload()
      },
      WindowSetTitle: (title: string) => {
        // 网页端标签页标题精简化处理，避免超长免责声明撑爆浏览器 Tab
        if (title.length > 40) {
          const parts = title.split('  ')
          document.title = parts[0] || title.slice(0, 40)
        } else {
          document.title = title
        }
      },
      WindowMinimise: () => {},
      WindowMaximise: () => {},
      WindowToggleMaximise: () => {
        if (!document.fullscreenElement) {
          document.documentElement.requestFullscreen().catch(() => {})
        } else {
          document.exitFullscreen().catch(() => {})
        }
      },
      WindowIsMaximised: async () => Boolean(document.fullscreenElement),
      WindowIsMinimised: async () => false,
      WindowIsNormal: async () => !document.fullscreenElement,
      WindowSetSize: (_w: number, _h: number) => {},
      WindowGetSize: async () => {
        return { w: window.innerWidth, h: window.innerHeight }
      },
      WindowSetPosition: (_x: number, _y: number) => {},
      WindowGetPosition: async () => ({ x: 0, y: 0 }),
      WindowSetMaxSize: (_w: number, _h: number) => {},
      WindowSetMinSize: (_w: number, _h: number) => {},
      WindowHide: () => {},
      WindowShow: () => {},
      Hide: () => {},
      Show: () => {},
      ScreenGetAll: async () => [
        { isCurrent: true, isPrimary: true, width: window.screen?.width || window.innerWidth, height: window.screen?.height || window.innerHeight }
      ],
      OnFileDrop: (_callback: any, _useDropTarget?: boolean) => {},
      OnFileDropOff: () => {},
      Quit: () => {
        window.close()
      },

      // 剪贴板支持（兼容非 HTTPS/HTTP 裸 IP 访问）
      ClipboardGetText: async () => {
        try {
          if (navigator.clipboard && navigator.clipboard.readText) {
            return await navigator.clipboard.readText()
          }
        } catch {
          // 非安全上下文或权限被拒
        }
        return ''
      },
      ClipboardSetText: async (text: string) => {
        try {
          if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(text)
            return true
          }
        } catch {
          // 降级兜底
        }

        // 非 HTTPS (如 http://<ip>:8080) 降级方案：创建隐藏 textarea 复制
        try {
          const textArea = document.createElement('textarea')
          textArea.value = text
          textArea.style.position = 'fixed'
          textArea.style.left = '-9999px'
          textArea.style.top = '-9999px'
          textArea.setAttribute('readonly', '')
          document.body.appendChild(textArea)
          textArea.select()
          const successful = document.execCommand('copy')
          document.body.removeChild(textArea)
          return successful
        } catch (e) {
          console.error('[UniversalBridge] Clipboard fallback failed:', e)
          return false
        }
      },

      // 日志系统
      LogPrint: (msg: string) => console.log('[Backend]', msg),
      LogTrace: (msg: string) => console.trace('[Backend]', msg),
      LogDebug: (msg: string) => console.debug('[Backend]', msg),
      LogInfo: (msg: string) => console.info('[Backend]', msg),
      LogWarning: (msg: string) => console.warn('[Backend]', msg),
      LogError: (msg: string) => console.error('[Backend]', msg),
      LogFatal: (msg: string) => console.error('[Backend FATAL]', msg),

      // 保存文件对话框 Web 降级
      SaveFileDialog: async (_options: any) => {
        return 'browser_download'
      },
    }
  }

  if (typeof document !== 'undefined' && document.body) {
    document.body.classList.add('is-web-mode')
  }
}
