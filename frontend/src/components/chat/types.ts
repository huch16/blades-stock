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
  /**
   * 流式收尾异常提示（输出被截断 / 内容安全拦截 / 长时间静默被看门狗收尾）。
   * 必须在**流式结束后依然可见** —— 这些情况下正文可能不完整，
   * 只让回复“突然结束”而不说明原因，用户无从判断是模型问题还是程序问题。
   */
  streamNotice?: string
  /**
   * 看门狗超时标记：为 true 时在消息下方渲染**真正的按钮**
   * （重新发送 / 继续等待 / 停止本轮）。
   *
   * 为什么不用 HTML 字符串（踩过的坑）：曾经把 `<br><button @click=...>`
   * 直接拼进 streamNotice，而模板用的是 `{{ streamNotice }}` 文本插值 ——
   * 结果标签被**原样当文字渲染**，用户看到一串 `<button class="retry-btn">`
   * 源码且点击无效。按钮必须走真实的模板 + 事件，不能注入 HTML。
   */
  streamTimeout?: boolean
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
