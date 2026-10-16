/**
 * Universal Bridge (通用通信适配桥接层)
 *
 * 核心职责：
 * 1. 自动探测当前运行环境（Wails 桌面客户端 VS 浏览器 Web 模式）；
 * 2. 在 Web 模式下，动态通过 Proxy 托管 window.go.main.App 与 window.runtime；
 * 3. 将原先的 Wails IPC 零侵入透明转化为标准 HTTP POST 请求与 SSE 流式推流；
 * 4. 统一处理公网鉴权 Header（Authorization Bearer Token）。
 */

// 1. 环境感知
export const isWails = (): boolean => {
  return typeof window !== 'undefined' &&
    typeof (window as any).runtime !== 'undefined' &&
    typeof (window as any).go !== 'undefined' &&
    Boolean((window as any).go?.main?.App)
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

// 3. Web 模式下的全局简易事件总线与 SSE 监听器
class WebEventBus {
  private listeners: Map<string, Set<Function>> = new Map()
  private sseSource: EventSource | null = null
  private sseConnected: boolean = false

  public on(event: string, callback: Function): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set())
    }
    this.listeners.get(event)!.add(callback)
    this.ensureSSEConnection()
  }

  public off(event: string, callback?: Function): void {
    if (!callback) {
      this.listeners.delete(event)
      return
    }
    const set = this.listeners.get(event)
    if (set) {
      set.delete(callback)
      if (set.size === 0) {
        this.listeners.delete(event)
      }
    }
  }

  public emit(event: string, ...args: any[]): void {
    const callbacks = this.listeners.get(event)
    if (callbacks) {
      callbacks.forEach((cb) => {
        try {
          cb(...args)
        } catch (e) {
          console.error(`[WebEventBus] Callback error on event "${event}":`, e)
        }
      })
    }
  }

  // 保证与后端的 SSE 长连接连通
  private ensureSSEConnection(): void {
    if (this.sseConnected || typeof window === 'undefined') return
    const token = getAuthToken()
    const url = `/api/events/stream${token ? `?token=${encodeURIComponent(token)}` : ''}`

    try {
      this.sseSource = new EventSource(url)
      this.sseConnected = true

      this.sseSource.onmessage = (e) => {
        try {
          const payload = JSON.parse(e.data)
          if (payload && payload.event) {
            this.emit(payload.event, payload.data)
          }
        } catch (err) {
          // 普通文本推流直接透传
          this.emit('message', e.data)
        }
      }

      this.sseSource.onerror = () => {
        // 网络异常或 401 断开
        this.sseConnected = false
        if (this.sseSource) {
          this.sseSource.close()
          this.sseSource = null
        }
        // 5秒后尝试自动重连
        setTimeout(() => this.ensureSSEConnection(), 5000)
      }
    } catch (e) {
      this.sseConnected = false
    }
  }
}

export const webEventBus = new WebEventBus()

// 4. Web 模式下的通用 HTTP RPC 发起器
export async function callWebApi(method: string, ...args: any[]): Promise<any> {
  const token = getAuthToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const url = `/api/App/${method}`

  try {
    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(args),
    })

    if (response.status === 401) {
      // 广播 401 未授权通知，供前端弹出授权输入弹窗
      window.dispatchEvent(new CustomEvent('blades:unauthorized'))
      throw new Error('未授权访问或访问口令已失效，请重新输入口令')
    }

    if (!response.ok) {
      const errText = await response.text()
      throw new Error(`HTTP ${response.status}: ${errText}`)
    }

    const contentType = response.headers.get('content-type') || ''
    if (contentType.includes('application/json')) {
      const json = await response.json()
      // 后端统一约定的 { code, data, error } 或直出数据结构
      if (json && typeof json === 'object' && 'code' in json && 'data' in json) {
        if (json.code !== 0) {
          throw new Error(json.error || json.message || '请求失败')
        }
        return json.data
      }
      return json
    } else {
      return await response.text()
    }
  } catch (err: any) {
    console.error(`[UniversalBridge] RPC call to "${method}" failed:`, err)
    throw err
  }
}

// 5. 全局运行时桩注入（必须在最早时刻执行）
export function initUniversalBridge(): void {
  if (typeof window === 'undefined') return

  // 如果处于真实 Wails 桌面端，直接使用原生机制，无需额外代理
  if (isWails()) {
    console.log('[UniversalBridge] Running in Native Wails Desktop mode.')
    return
  }

  console.log('[UniversalBridge] Running in Cloud-Native Web/Browser mode. Installing proxy stubs...')

  // 注入 window.go.main.App 的动态代理
  const win = window as any
  win.go = win.go || {}
  win.go.main = win.go.main || {}

  if (!win.go.main.App) {
    win.go.main.App = new Proxy({}, {
      get(_target, propKey: string) {
        return (...args: any[]) => {
          return callWebApi(propKey, ...args)
        }
      },
    })
  }

  // 注入 window.runtime 的动态代理
  if (!win.runtime) {
    win.runtime = {
      EventsOn: (event: string, callback: Function) => webEventBus.on(event, callback),
      EventsOff: (event: string, callback?: Function) => webEventBus.off(event, callback),
      EventsOnce: (event: string, callback: Function) => {
        const onceWrapper = (...args: any[]) => {
          webEventBus.off(event, onceWrapper)
          callback(...args)
        }
        webEventBus.on(event, onceWrapper)
      },
      EventsEmit: (event: string, ...args: any[]) => webEventBus.emit(event, ...args),
      BrowserOpenURL: (url: string) => {
        window.open(url, '_blank')
      },
      WindowMinimise: () => {
        console.log('[Web stub] WindowMinimise ignored in browser.')
      },
      WindowMaximise: () => {
        console.log('[Web stub] WindowMaximise ignored in browser.')
      },
      WindowToggleMaximise: () => {
        console.log('[Web stub] WindowToggleMaximise ignored in browser.')
      },
      WindowSetTitle: (title: string) => {
        document.title = title
      },
      Quit: () => {
        window.close()
      },
      SaveFileDialog: async (_options: any) => {
        // Web 模式下 SaveFileDialog 直接提示由浏览器下载流接管
        return 'browser_download'
      },
    }
  }

  // 为 body 添加 web 模式特定 class，方便样式优雅隐藏无边框窗体控制按钮
  if (typeof document !== 'undefined' && document.body) {
    document.body.classList.add('is-web-mode')
  }
}
