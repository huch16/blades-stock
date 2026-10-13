<template>
  <div class="chat-container">
    <div class="chat-header" v-if="!isCompact">
      <div class="header-left">
        <h2 class="chat-title">💬 AI 投研助手</h2>
        <span class="chat-subtitle">基于多模型协作的智能股票分析</span>
      </div>
      <div class="header-right">
        <n-select
          v-model:value="activeConfigId"
          :options="configOptions"
          :placeholder="configOptions.length ? '' : '请先在设置中添加 AI 配置'"
          size="small"
          style="width: 220px;"
          clearable
          @update:value="onConfigChange"
        />
        <n-space vertical-align="center" size="small">
          <n-select
            v-model:value="agentMode"
            :options="agentModeOptions"
            size="small"
            style="width: 110px;"
          />
          <n-button
            variant="text"
            size="small"
            @click="toggleCompact"
            :circle="true"
          >
            <template v-if="isCompact">
              <FullscreenIcon />
            </template>
            <template v-else>
              <FullscreenExitIcon />
            </template>
          </n-button>
          <n-button
            variant="text"
            size="small"
            @click="clearHistory"
            :circle="true"
          >
            <DeleteIcon />
          </n-button>
        </n-space>
      </div>
    </div>

    <div class="chat-messages" ref="messagesContainer">
      <!-- 消息锚点导航（右侧边栏） -->
      <MessageAnchorNav
        :items="anchorNavItems"
        :show="messages.length > 0"
        @scroll="scrollToMessage"
      />

      <ChatMessage
        v-for="msg in messages"
        :key="msg.id"
        :message="msg"
        :is-streaming="isStreaming && msg.id === streamingMsgId"
        :current-user-avatar="userAvatar"
        :current-user-name="userName"
        @copy="handleCopy"
        @retry="handleRetry"
        @reference="handleReference"
        @edit="handleEdit"
        @resend="handleResend"
      />

      <!-- 流式占位消息 -->
      <ChatMessage
        v-if="isStreaming && streamingPlaceholder"
        :key="streamingPlaceholder.id"
        :message="streamingPlaceholder"
        :is-streaming="true"
        :current-user-avatar="userAvatar"
        :current-user-name="userName"
      />

      <!-- 空状态 -->
      <div v-if="messages.length === 0 && !isStreaming" class="empty-state">
        <div class="empty-icon">💬</div>
        <h3>开始你的投研对话</h3>
        <p>输入股票代码、名称或投资问题，AI 将为你提供专业分析</p>
        <div class="empty-suggestions">
          <n-tag
            v-for="s in suggestions"
            :key="s"
            size="small"
            @click="fillInput(s)"
            class="suggestion-tag"
          >
            {{ s }}
          </n-tag>
        </div>
      </div>

      <!-- 滚动到底部按钮 -->
      <n-button
        v-show="showScrollBottom"
        class="scroll-bottom-btn"
        variant="text"
        circle
        @click="scrollToBottom"
      >
        <ArrowDownIcon />
      </n-button>
    </div>

    <!-- 输入区域 -->
    <div class="chat-input-area">
      <t-chat-sender
        ref="chatSenderRef"
        v-model="inputValue"
        :textarea-props="{
          placeholder: configOptions.length ? '请输入消息... (Shift+Enter 换行)' : '请先在设置中配置 AI 模型',
          rows: 1,
          autosize: { minRows: 1, maxRows: 6 },
        }"
        :loading="isStreaming"
        :stop-disabled="isStreaming"
        @send="handleSend"
        @stop="handleStop"
      >
        <template #prefix v-if="!isCompact">
          <n-space vertical-align="center" size="small" class="input-prefix">
            <n-tooltip placement="top" :content="'选择 AI 配置'">
              <n-select
                v-model:value="activeConfigId"
                :options="configOptions"
                :placeholder="configOptions.length ? '' : '无可用配置'"
                size="small"
                style="width: 200px;"
                clearable
                @update:value="onConfigChange"
              />
            </n-tooltip>
            <n-tooltip placement="top" :content="'对话模式'">
              <n-select
                v-model:value="agentMode"
                :options="agentModeOptions"
                size="small"
                style="width: 110px;"
              />
            </n-tooltip>
          </n-space>
        </template>
        <template #suffix>
          <n-space vertical-align="center" size="small">
            <n-button
              v-if="!isCompact"
              variant="text"
              size="small"
              @click="insertTemplate"
            >
              📝 模板
            </n-button>
            <n-button
              theme="primary"
              :loading="isStreaming"
              @click="handleSend"
            >
              {{ isStreaming ? '停止' : '发送' }}
            </n-button>
          </n-space>
        </template>
      </t-chat-sender>

      <div v-if="!configOptions.length" class="config-hint">
        ⚠️ 未检测到 AI 配置，请前往 <router-link to="/settings">设置页面</router-link> 添加配置后再使用
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch, provide, h, type VNode } from 'vue'
import { NSelect, NSpace, NButton, NTag, NTooltip, NImage } from 'naive-ui'
import { ArrowDownIcon, DeleteIcon, FullscreenIcon, FullscreenExitIcon } from 'tdesign-icons-vue-next'
import { ChatWithAgent, GetConfig, GetVersionInfo } from '../../wailsjs/go/main/App'
import { useAiConfigStore } from '@/stores/aiConfig'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { cleanFinalContent } from '@/utils/cleanText'
import {
  useChatStream,
  useMessageAnchor,
  type ChatMessage as ChatMessageType,
  type StreamChunk,
  genMsgId,
  formatDuration
} from './chat'
import ChatMessageComp from './chat/ChatMessage.vue'
import MessageAnchorNav from './chat/MessageAnchorNav.vue'

// ========== 状态 ==========
const aiConfigStore = useAiConfigStore()
const messages = ref<ChatMessage[]>([])
const inputValue = ref('')
const isStreaming = ref(false)
const streamingMsgId = ref<string | null>(null)
const streamingPlaceholder = ref<ChatMessage | null>(null)
const showScrollBottom = ref(false)
const isCompact = ref(false)
const agentMode = ref<'auto' | 'react' | 'plan_execute'>('auto')
const userAvatar = ref('https://tdesign.gtimg.com/site/avatar.jpg')
const userName = ref('用户')

const activeConfigId = computed({
  get: () => aiConfigStore.activeConfigId,
  set: (val) => { aiConfigStore.setActiveConfigId(val) }
})

const configOptions = computed(() => aiConfigStore.configs.map(c => ({
  label: c.name || `配置 #${c.ID}`,
  value: c.ID,
  description: c.modelName ? `模型: ${c.modelName}` : '',
})))

const agentModeOptions = [
  { label: '🤖 自动', value: 'auto', description: '根据问题复杂度自动选择策略' },
  { label: '⚡ 快速', value: 'react', description: 'ReAct 单步推理，适合简单问答' },
  { label: '🧠 规划', value: 'plan_execute', description: 'Plan-and-Execute 多步规划，适合复杂分析' },
]

const suggestions = [
  '分析 000001 平安银行',
  '对比 贵州茅台 vs 五粮液',
  '筛选 低估值高分红股票',
  '解释 什么是 PE TTM',
  '回测 双均线策略 最近一年',
]

// ========== 流式状态机 ==========
const {
  isStreaming: streamIsStreaming,
  startStream,
  feedChunk,
  finalize,
  cancel,
  getSessionId,
} = useChatStream()

// ========== 锚点导航 ==========
const {
  activeAnchor,
  anchorNavItems,
  registerElement,
  unregisterElement,
  scrollToMessage,
  scrollToNext,
  scrollToPrev,
  scrollToLatest,
  scrollToOldest,
} = useMessageAnchor(messages)

// 同步流式状态到组件
watch(streamIsStreaming, (val) => { isStreaming.value = val })

// 暴露注册函数供子组件使用
provide('registerMessageElement', registerElement)
provide('unregisterMessageElement', unregisterElement)

// ========== 事件处理 ==========
function handleAgentMessage(data: StreamChunk) {
  // 会话过滤
  if (data.sessionId && data.sessionId !== getSessionId()) return

  if (data.role === 'assistant') {
    if (!streamingPlaceholder.value) return

    if (data.reasoning_content) {
      const rc = data.reasoning_content
      if (rc.startsWith('[STATUS]')) {
        streamingPlaceholder.value.statusText = rc.replace(/^\[STATUS\]/, '').trim()
      } else if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!streamingPlaceholder.value.steps) streamingPlaceholder.value.steps = []
          const parsed = parseStepText(stepText)
          streamingPlaceholder.value.steps.push(...parsed)
        }
      } else {
        streamingPlaceholder.value.rawReasoning = (streamingPlaceholder.value.rawReasoning || '') + rc
      }
    }

    if (data.content) {
      if (data.content === 'agent-DONE') {
        if (data.metrics) streamingPlaceholder.value.metrics = data.metrics
      } else {
        streamingPlaceholder.value.rawContent = (streamingPlaceholder.value.rawContent || '') + data.content
      }
    }

    if (data.tool_calls?.length) {
      if (!streamingPlaceholder.value.toolCalls) streamingPlaceholder.value.toolCalls = []
      streamingPlaceholder.value.toolCalls.push(...data.tool_calls)
    }

    if (data.response_meta?.finish_reason === 'stop') {
      const finalMsg = finalize()
      if (finalMsg) {
        messages.value.unshift(finalMsg)
        streamingPlaceholder.value = null
        streamingMsgId.value = null
      }
    }
  }
}

// 解析步骤文本
function parseStepText(text: string): string[] {
  const trimmed = text.trim()
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return [text]
  try {
    const obj = JSON.parse(trimmed)
    if (Array.isArray(obj)) {
      return obj.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
    }
    if (typeof obj === 'object' && obj !== null) {
      const steps = obj.steps || obj.step || obj.plan || obj.items || obj.list
      if (Array.isArray(steps)) {
        return steps.map((item, i) => `${i + 1}. ${typeof item === 'string' ? item : JSON.stringify(item)}`)
      }
      return Object.entries(obj).map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
    }
    return [text]
  } catch {
    return [text]
  }
}

// ========== 生命周期 ==========
onBeforeMount(() => {
  EventsOff('agent-message', handleAgentMessage)
  EventsOn('agent-message', handleAgentMessage)
})

onMounted(async () => {
  await aiConfigStore.loadConfigs()

  GetConfig().then((res) => {
    if (res.darkTheme) {
      document.documentElement.setAttribute('theme-mode', 'dark')
    } else {
      document.documentElement.removeAttribute('theme-mode')
    }
  }).catch(() => {})

  GetVersionInfo().then((res) => {
    if (res.icon) {
      // 更新 AI 头像
      messages.value.forEach(m => {
        if (m.role === 'assistant') m.avatar = res.icon
      })
    }
  }).catch(() => {})
})

onBeforeUnmount(() => {
  EventsOff('agent-message', handleAgentMessage)
})

// ========== 交互方法 ==========
function handleSend() {
  if (isStreaming.value) {
    handleStop()
    return
  }
  if (!inputValue.value.trim()) return

  const configId = activeConfigId.value
  if (!configId || configId <= 0 || !aiConfigStore.activeConfig) {
    import('tdesign-vue-next').then(({ MessagePlugin }) => {
      MessagePlugin.warning('请先选择或创建 AI 配置')
    })
    return
  }

  // 用户消息
  const userMsg: ChatMessage = {
    id: genMsgId(),
    avatar: userAvatar.value,
    name: userName.value,
    datetime: new Date().toISOString(),
    content: inputValue.value,
    role: 'user',
  }
  messages.value.unshift(userMsg)

  // AI 占位消息
  const assistantId = genMsgId()
  const assistantMsg: ChatMessage = {
    id: assistantId,
    avatar: '',
    name: 'AI 助手',
    datetime: new Date().toISOString(),
    content: '',
    rawContent: '',
    reasoning: '',
    rawReasoning: '',
    jsonMarkdown: '',
    role: 'assistant',
    steps: [],
    statusText: '',
    toolCalls: [],
  }
  streamingMsgId.value = assistantId
  streamingPlaceholder.value = assistantMsg
  messages.value.unshift(assistantMsg) // 先加入列表，组件会根据 isStreaming 渲染骨架屏

  // 开始流式
  startStream(assistantMsg)

  nextTick(() => {
    scrollToBottom()
    ChatWithAgent(
      inputValue.value,
      configId,
      0,
      false,
      0,
      false,
      agentMode.value === 'auto' ? '' : agentMode.value,
      ''
    ).catch((err: any) => {
      console.error('ChatWithAgent error', err)
      cancel()
      const errMsg: ChatMessage = {
        id: genMsgId(),
        avatar: '',
        name: 'AI 助手',
        datetime: new Date().toISOString(),
        content: `❌ 调用失败：${err?.message || err}`,
        role: 'assistant',
      }
      messages.value.unshift(errMsg)
    })
  })

  inputValue.value = ''
}

function handleStop() {
  cancel()
  if (streamingPlaceholder.value) {
    messages.value = messages.value.filter(m => m.id !== streamingPlaceholder.value!.id)
    streamingPlaceholder.value = null
    streamingMsgId.value = null
  }
}

function handleCopy(content: string) {
  navigator.clipboard.writeText(content).then(() => {
    import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.success('已复制'))
  }).catch(() => {
    import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.error('复制失败'))
  })
}

function handleRetry(msgId: string) {
  const idx = messages.value.findIndex(m => m.id === msgId)
  if (idx === -1) return
  const msg = messages.value[idx]
  if (msg.role === 'user') {
    // 重发用户消息
    inputValue.value = msg.content
    messages.value = messages.value.slice(0, idx) // 移除该消息及之后的回复
    handleSend()
  }
}

function handleReference(msgId: string) {
  const msg = messages.value.find(m => m.id === msgId)
  if (msg) {
    inputValue.value += `\n\n> 引用：${msg.content.slice(0, 200)}...`
    import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.success('已引用到输入框'))
  }
}

function handleEdit(msg: ChatMessage) {
  if (msg.role === 'user') {
    inputValue.value = msg.content
    const idx = messages.value.findIndex(m => m.id === msg.id)
    if (idx !== -1) messages.value.splice(idx, 1)
  }
}

function handleResend(msg: ChatMessage) {
  handleEdit(msg)
  handleSend()
}

function onConfigChange(val: number) {
  activeConfigId.value = val
  aiConfigStore.setActiveConfigId(val)
}

function fillInput(text: string) {
  inputValue.value = text
}

function clearHistory() {
  import('tdesign-vue-next').then(({ DialogPlugin }) => {
    DialogPlugin.warning({
      title: '清空对话历史',
      content: '确定要清空当前对话历史吗？此操作不可恢复。',
      positiveText: '清空',
      negativeText: '取消',
      onPositiveAction: () => {
        messages.value = []
        import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.success('已清空'))
      }
    })
  })
}

function toggleCompact() {
  isCompact.value = !isCompact.value
}

function insertTemplate() {
  import('tdesign-vue-next').then(({ DialogPlugin }) => {
    DialogPlugin.info({
      title: '快速模板',
      content: () => h('div', { style: 'display:flex;flex-direction:column;gap:8px' }, [
        h(NButton, { size: 'small', block: true, onClick: () => { fillInput('分析 {{stockCode}} 的基本面、技术面、资金面、消息面'); DialogPlugin.close() }}, { default: () => '📊 全维度个股分析' }),
        h(NButton, { size: 'small', block: true, onClick: () => { fillInput('对比 {{stockCode1}} 和 {{stockCode2}} 的优劣势'); DialogPlugin.close() }}, { default: () => '⚖️ 两只股票对比分析' }),
        h(NButton, { size: 'small', block: true, onClick: () => { fillInput('筛选市盈率<15、股息率>3%、ROE>15% 的股票'); DialogPlugin.close() }}, { default: () => '🔍 价值型选股筛选' }),
        h(NButton, { size: 'small', block: true, onClick: () => { fillInput('回测 双均线策略 MA5/MA20 最近 1 年'); DialogPlugin.close() }}, { default: () => '📉 策略回测' }),
        h(NButton, { size: 'small', block: true, onClick: () => { fillInput('解释 MACD、RSI、布林带 的实战用法'); DialogPlugin.close() }}, { default: () => '📚 技术指标教学' }),
      ]),
      positiveText: '关闭',
      onPositiveAction: () => {},
    })
  })
}

function scrollToBottom() {
  nextTick(() => {
    const container = document.querySelector('.chat-messages')
    if (container) {
      container.scrollTo({ top: 0, behavior: 'smooth' })
    }
  })
}

// 监听滚动显示/隐藏底部按钮
function handleScroll() {
  const container = document.querySelector('.chat-messages')
  if (container) {
    showScrollBottom.value = container.scrollTop > 200
  }
}

onMounted(() => {
  const container = document.querySelector('.chat-messages')
  container?.addEventListener('scroll', handleScroll)
})

onBeforeUnmount(() => {
  const container = document.querySelector('.chat-messages')
  container?.removeEventListener('scroll', handleScroll)
})
</script>

<style scoped lang="less">
.chat-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: var(--td-bg-color-container);
}

.chat-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--td-component-border);
  background: var(--td-bg-color-container);
  flex-shrink: 0;

  .header-left {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .chat-title {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  .chat-subtitle {
    margin: 0;
    font-size: 12px;
    color: var(--td-text-color-disabled);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column-reverse; // 最新消息在上
  position: relative;

  /* 滚动条美化 */
  ::-webkit-scrollbar {
    width: 6px;
  }
  ::-webkit-scrollbar-track {
    background: transparent;
  }
  ::-webkit-scrollbar-thumb {
    background: var(--td-border-level-2-color);
    border-radius: 3px;
  }
  ::-webkit-scrollbar-thumb:hover {
    background: var(--td-border-level-3-color);
  }
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 300px;
  text-align: center;
  color: var(--td-text-color-disabled);

  .empty-icon {
    font-size: 48px;
    margin-bottom: 12px;
    opacity: 0.5;
  }

  h3 {
    margin: 0 0 8px;
    font-size: 18px;
    font-weight: 500;
    color: var(--td-text-color-secondary);
  }

  p {
    margin: 0 0 20px;
    font-size: 14px;
  }
}

.empty-suggestions {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 8px;
  max-width: 400px;
}

.suggestion-tag {
  cursor: pointer;
  transition: all 0.2s;
  &:hover {
    background: var(--td-brand-color-light);
    border-color: var(--td-brand-color);
    color: var(--td-brand-color);
  }
}

.scroll-bottom-btn {
  position: absolute;
  right: 20px;
  bottom: 20px;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 10;
}

.chat-input-area {
  padding: 12px 16px;
  border-top: 1px solid var(--td-component-border);
  background: var(--td-bg-color-container);
  flex-shrink: 0;

  .input-prefix {
    margin-right: 8px;
  }

  .config-hint {
    margin-top: 8px;
    padding: 8px 12px;
    background: #fff3d4;
    border: 1px solid #ffe58f;
    border-radius: 6px;
    font-size: 13px;
    color: #ad6800;
    a { color: var(--td-brand-color); }
  }
}

:deep(.t-chat-sender) {
  .t-chat-sender__textarea {
    border-radius: 8px !important;
    border: 1px solid var(--td-component-border) !important;
    background: var(--td-bg-color-container) !important;
    font-size: 14px !important;
    line-height: 1.6 !important;
  }
}
</style>