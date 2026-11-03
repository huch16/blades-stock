/**
 * 聊天模块核心类型定义
 */

export interface ToolCall {
  id?: string
  function: {
    name: string
    arguments: string
  }
  status: 'pending' | 'success' | 'error'
  duration?: number
  result?: string
}

export interface ExecutionMetrics {
  total_duration?: number
  first_chunk_latency?: number
  tool_call_count?: number
  retry_count?: number
  tokens?: {
    total?: number
    prompt?: number
    completion?: number
  }
  cost_usd?: number
}

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  rawContent?: string
  reasoning?: string
  rawReasoning?: string
  jsonMarkdown?: string
  statusText?: string
  steps?: string[]
  toolCalls?: ToolCall[]
  actions?: any[]
  metrics?: ExecutionMetrics
  datetime: string
  avatar?: string
  name?: string
  error?: boolean
  modelName?: string
}

export interface StreamChunk {
  role?: string
  content?: string
  reasoning_content?: string
  sessionId?: string
  session_id?: string
  actions?: any[]
  finish_reason?: string
  metrics?: ExecutionMetrics
  tool_calls?: ToolCall[]
  response_meta?: {
    finish_reason?: string
  }
}

let msgCounter = 0
export function genMsgId(prefix = 'msg'): string {
  return `${prefix}_${Date.now()}_${++msgCounter}`
}

export function formatDuration(val?: number): string {
  if (!val || val < 0) return '0ms'
  // 兼顾 Go 序列化 time.Duration 为纳秒 (ns) 与标准毫秒 (ms)
  // 如果数值 > 1e8（即单次请求超过 100,000 秒，在交互式对话中必定是 Go 纳秒序列化残留），自动除以 1e6 转为毫秒
  let ms = val
  if (ms > 1e8) {
    ms = ms / 1e6
  }
  if (ms < 1000) return `${Math.round(ms)}ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)}s`
  const m = Math.floor(s / 60)
  const remS = Math.floor(s % 60)
  return `${m}m ${remS}s`
}

export function getToolCallStatusText(status?: string): string {
  switch (status) {
    case 'pending': return '执行中'
    case 'success': return '成功'
    case 'error': return '失败'
    default: return '未知'
  }
}
