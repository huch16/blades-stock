import { EventsOn, EventsEmit } from '../../wailsjs/runtime'

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
  dispatchAgentAction({
    type: 'CONSULT_AGENT',
    payload: {
      question: opts.question,
      stockCode: opts.stockCode || '',
      stockName: opts.stockName || '',
      context: opts.context || '',
      mode: opts.mode || 'plan_execute',
      autoSend: opts.autoSend !== false,
    },
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
