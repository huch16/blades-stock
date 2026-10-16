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
  return (
    typeof window !== 'undefined' &&
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

  public onMultiple(event: string, callback: Function, maxCallbacks: number = -1): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, [])
    }
    this.listeners.get(event)!.push({
      callback,
      maxCallbacks,
      calledCount: 0,
    })
    this.ensureSSEConnection()
  }

  public off(eventName: string, ...additionalEventNames: string[]): void {
    const targets = [eventName, ...additionalEventNames]
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
          this.emit('message', e.data)
        }
      }

      this.sseSource.onerror = () => {
        this.sseConnected = false
        if (this.sseSource) {
          this.sseSource.close()
          this.sseSource = null
        }
        if (!this.retryTimer) {
          this.retryTimer = setTimeout(() => {
            this.retryTimer = null
            this.ensureSSEConnection()
          }, 4000)
        }
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
      clearAuthToken()
      window.dispatchEvent(new CustomEvent('blades:unauthorized'))
      throw new Error('未授权访问：请先输入有效访问口令')
    }

    if (!response.ok) {
      const errText = await response.text()
      throw new Error(`HTTP ${response.status}: ${errText}`)
    }

    const contentType = response.headers.get('content-type') || ''
    if (contentType.includes('application/json')) {
      const json = await response.json()
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

// 5. 全局运行时桩注入（最早时刻执行）
export function initUniversalBridge(): void {
  if (typeof window === 'undefined') return

  // 原生 Wails 桌面环境直接放行
  if (isWails()) {
    console.log('[UniversalBridge] Running in Native Wails Desktop mode.')
    return
  }

  console.log('[UniversalBridge] Running in Cloud-Native Web/Browser mode. Installing proxy stubs...')

  const win = window as any
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
      EventsOff: (eventName: string, ...additionalEventNames: string[]) => {
        webEventBus.off(eventName, ...additionalEventNames)
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
        document.title = title
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
      WindowSetSize: (_w: number, _h: number) => {},
      WindowGetSize: async () => {
        return { w: window.innerWidth, h: window.innerHeight }
      },
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
