import { ref, onUnmounted } from 'vue'
import type { ChatMessage, StreamChunk } from './types'

/**
 * 流式会话状态机
 * 职责：
 * - 维护当前会话 ID（用于过滤过期流消息）
 * - 缓冲区与流式生命周期管理
 * - 格式化定时器（节流渲染与 Markdown 包装）
 * - 提供统一的 `startStream` / `feedChunk` / `finalize` / `cancel` / `getSessionId` 接口
 */
export function useChatStream() {
  let sessionIdCounter = 0
  const sessionIdRef = ref<string>(`session_${Date.now()}_${++sessionIdCounter}`)
  const isStreamingRef = ref<boolean>(false)
  const currentMsgRef = ref<ChatMessage | null>(null)

  let formatTimer: ReturnType<typeof setInterval> | null = null
  const FORMAT_INTERVAL = 300 // ms

  function getSessionId(): string {
    return sessionIdRef.value
  }

  function startStream(msg: ChatMessage) {
    cancel()
    isStreamingRef.value = true
    currentMsgRef.value = msg
    sessionIdRef.value = `session_${Date.now()}_${++sessionIdCounter}`

    // 启动节流格式化定时器
    formatTimer = setInterval(() => {
      if (currentMsgRef.value) {
        if (currentMsgRef.value.rawContent && !currentMsgRef.value.content) {
          currentMsgRef.value.content = currentMsgRef.value.rawContent
        }
        if (currentMsgRef.value.rawReasoning && !currentMsgRef.value.reasoning) {
          currentMsgRef.value.reasoning = currentMsgRef.value.rawReasoning
        }
      }
    }, FORMAT_INTERVAL)
  }

  function feedChunk(chunk: StreamChunk) {
    const chunkSessionId = chunk.sessionId || chunk.session_id
    if (chunkSessionId && chunkSessionId !== sessionIdRef.value) {
      return
    }

    if (!currentMsgRef.value) return

    if (chunk.reasoning_content) {
      currentMsgRef.value.rawReasoning = (currentMsgRef.value.rawReasoning || '') + chunk.reasoning_content
    }

    if (chunk.content) {
      if (chunk.content === 'agent-DONE') {
        if (chunk.metrics) {
          currentMsgRef.value.metrics = chunk.metrics
        }
      } else {
        currentMsgRef.value.rawContent = (currentMsgRef.value.rawContent || '') + chunk.content
      }
    }

    if (chunk.tool_calls && chunk.tool_calls.length > 0) {
      if (!currentMsgRef.value.toolCalls) {
        currentMsgRef.value.toolCalls = []
      }
      currentMsgRef.value.toolCalls.push(...chunk.tool_calls)
    }

    if (chunk.metrics) {
      currentMsgRef.value.metrics = chunk.metrics
    }
  }

  function finalize(): ChatMessage | null {
    if (formatTimer !== null) {
      clearInterval(formatTimer)
      formatTimer = null
    }

    isStreamingRef.value = false
    const msg = currentMsgRef.value
    if (msg) {
      if (msg.rawContent) {
        msg.content = msg.rawContent
      }
      if (msg.rawReasoning) {
        msg.reasoning = msg.rawReasoning
      }
    }
    currentMsgRef.value = null
    return msg
  }

  function cancel() {
    if (formatTimer !== null) {
      clearInterval(formatTimer)
      formatTimer = null
    }
    isStreamingRef.value = false
    currentMsgRef.value = null
    sessionIdRef.value = `session_${Date.now()}_${++sessionIdCounter}`
  }

  onUnmounted(() => {
    if (formatTimer !== null) {
      clearInterval(formatTimer)
      formatTimer = null
    }
  })

  return {
    isStreaming: isStreamingRef,
    sessionId: sessionIdRef,
    currentMsg: currentMsgRef,
    startStream,
    feedChunk,
    finalize,
    cancel,
    getSessionId,
  }
}
