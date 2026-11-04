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
interface ListenerWrapper {
  callback: Function
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

  public onMultiple(event: string, callback: Function, maxCallbacks: number = -1): void {
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

  public off(eventName: string, callbackOrNextEvent?: Function | string, ...rest: (Function | string)[]): void {
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
    } catch (err) {
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
    const url = `/api/events/stream${token ? `?token=${encodeURIComponent(token)}` : ''}`

    try {
      this.sseSource = new EventSource(url)
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
    } catch (e) {
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
  GetConfig: 10_000,           // 10s: 避免各组件 onMounted 狂轰滥炸，UpdateConfig 时立即作废
  GetVersionInfo: 60_000,      // 60s: 系统版本极少变更
  IsTradingTime: 5_000,        // 5s: 交易时段检测
  IsHKTradingTime: 5_000,      // 5s
  IsUSTradingTime: 5_000,      // 5s
  IsTradingDay: 15_000,        // 15s
  GetLatestTradingDay: 15_000, // 15s
  GetGroupList: 5_000,         // 5s: 分组列表
  GetStockList: 30_000,        // 30s: 拦截 1.15MB 全量股票列表在页面路由切换时的重复拉取
}

// 变更操作自动作废关联缓存
function invalidateQueryCacheOnMutation(method: string) {
  if (method.includes('Config')) {
    for (const key of queryCache.keys()) {
      if (key.startsWith('GetConfig:')) queryCache.delete(key)
    }
  }
  if (method.includes('Group')) {
    for (const key of queryCache.keys()) {
      if (key.startsWith('GetGroupList:')) queryCache.delete(key)
    }
  }
  if (method === 'Follow' || method === 'UnFollow') {
    for (const key of queryCache.keys()) {
      if (key.startsWith('GetFollowList:')) queryCache.delete(key)
    }
  }
}

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

  // 注入 30 秒硬超时熔断控制器，防止慢速接口无截止时间悬挂占用连接
  const controller = typeof AbortController !== 'undefined' ? new AbortController() : null
  const timer = controller ? setTimeout(() => controller.abort(), 30_000) : null

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
        const errText = await response.text()
        throw new Error(`HTTP ${response.status}: ${errText}`)
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
        console.warn(`[UniversalBridge] RPC call to "${method}" timed out after 30s.`)
        throw new Error(`请求超时：接口 "${method}" 响应耗时超过 30 秒，已执行保护性熔断`)
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
export function initUniversalBridge(): void {
  if (typeof window === 'undefined') return

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
      EventsOnMultiple: (eventName: string, callback: Function, maxCallbacks: number) => {
        webEventBus.onMultiple(eventName, callback, maxCallbacks)
      },
      EventsOn: (eventName: string, callback: Function) => {
        webEventBus.onMultiple(eventName, callback, -1)
      },
      EventsOnce: (eventName: string, callback: Function) => {
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
      WindowMinimise: () => {
        console.log('[Web stub] WindowMinimise ignored.')
      },
      WindowMaximise: () => {
        console.log('[Web stub] WindowMaximise ignored.')
      },
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
