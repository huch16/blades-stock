import { EventsOn } from '../../wailsjs/runtime'

export interface AgentAction {
  type:
    | 'NAVIGATE_VIEW'
    | 'OPEN_DIAGNOSIS'
    | 'OPEN_MARKET_INTELLIGENCE'
    | 'SHOW_KLINE'
    | 'DISCIPLINE_ALERT'
    | 'FILTER_WATCHLIST'
    | 'CONSULT_AGENT'
    | 'SAVE_THESIS'
  payload: Record<string, any>
}

type ActionHandler = (action: AgentAction) => void

const handlers: ActionHandler[] = []

export function registerAgentActionHandler(handler: ActionHandler) {
  handlers.push(handler)
  return () => {
    const idx = handlers.indexOf(handler)
    if (idx !== -1) handlers.splice(idx, 1)
  }
}

export function dispatchAgentAction(action: AgentAction) {
  for (const h of handlers) {
    try {
      h(action)
    } catch (e) {
      console.error('[AgentControlBus] handler error:', e)
    }
  }
}

export interface ConsultPayload {
  id?: string
  question: string
  stockCode?: string
  stockName?: string
  context?: string
  mode?: 'auto' | 'react' | 'plan_execute' | 'direct'
  autoSend?: boolean
  timestamp?: number
}

let pendingConsult: ConsultPayload | null = null

export function setPendingConsult(payload: ConsultPayload) {
  const consultId = payload.id || `consult_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  pendingConsult = { ...payload, id: consultId, timestamp: Date.now() }
}

export function consumePendingConsult(): ConsultPayload | null {
  const p = pendingConsult
  pendingConsult = null
  return p
}

export function getPendingConsult(): ConsultPayload | null {
  return pendingConsult
}

export function clearPendingConsult() {
  pendingConsult = null
}

/**
 * 快速呼叫智能体发起深度推演（跨组件统一入口）
 */
export function consultAgent(opts: {
  question: string
  stockCode?: string
  stockName?: string
  context?: string
  mode?: 'auto' | 'react' | 'plan_execute' | 'direct'
  autoSend?: boolean
}) {
  const consultId = `consult_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
  const payload: ConsultPayload = {
    id: consultId,
    question: opts.question,
    stockCode: opts.stockCode || '',
    stockName: opts.stockName || '',
    context: opts.context || '',
    mode: opts.mode || 'plan_execute',
    autoSend: opts.autoSend !== false,
    timestamp: Date.now(),
  }
  
  // 先落盘挂起缓存，防止异步路由加载延迟导致丢消息
  setPendingConsult(payload)

  dispatchAgentAction({
    type: 'CONSULT_AGENT',
    payload,
  })
}

let isInitialized = false

export function initAgentControlBus() {
  if (isInitialized) return
  isInitialized = true

  // 监听来自后端的统一动作事件
  EventsOn('agent:action', (actions: AgentAction[] | AgentAction) => {
    if (!actions) return
    const list = Array.isArray(actions) ? actions : [actions]
    for (const a of list) {
      if (a && a.type) {
        dispatchAgentAction(a)
      }
    }
  })

  // 浏览器原生事件通道兼容
  if (typeof window !== 'undefined') {
    window.addEventListener('agent:action', (e: any) => {
      if (e.detail) {
        const list = Array.isArray(e.detail) ? e.detail : [e.detail]
        for (const a of list) {
          if (a && a.type) dispatchAgentAction(a)
        }
      }
    })
  }
}
