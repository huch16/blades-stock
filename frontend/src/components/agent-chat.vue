<template>
  <div class="chat-box">
    <t-chat
        ref="chatRef"
        :clear-history="chatList.length > 0 && isStreaming"
        :data="chatList"
        :text-loading="isStreaming"
        :is-stream-load="isStreaming"
        style="height: 100%"
        @scroll="handleChatScroll"
        @clear="clearConfirm"
    >
      <template #content="{ item, index }">
        <div v-if="item.role === 'assistant' && item.steps && item.steps.length > 0" class="agent-steps">
          <div class="agent-steps-header">📋 执行步骤 <span class="agent-steps-badge">{{ item.steps.length }}</span></div>
          <div class="agent-steps-list">
            <div v-for="(step, si) in item.steps" :key="si" class="agent-step-item">
              <div class="agent-step-dot" :class="getStepDotClass(step)"></div>
              <span class="agent-step-text">{{ step }}</span>
            </div>
          </div>
        </div>
        <div v-if="item.role === 'assistant' && isStreaming" class="agent-status">
          <span class="agent-status-text">{{ item.statusText || '处理中...' }}</span>
        </div>
        <t-chat-reasoning v-if="item.role === 'assistant'" :default-expand-all="isStreaming" expand-icon-placement="right">
          <t-chat-loading v-if="isStreaming && !item.reasoning" text="思考中..." />
          <t-chat-content v-if="item.reasoning.length > 0" :content="item.reasoning" />
        </t-chat-reasoning>
        <div v-if="item.role === 'assistant' && item.jsonMarkdown" class="agent-json-md">
          <div class="agent-json-md-header" @click="toggleJsonMd(item.id)">
            <svg :class="['agent-json-md-arrow', { 'agent-json-md-arrow-expanded': jsonMdExpandedMap[item.id] }]" viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><path d="M9 6l6 6-6 6z"/></svg>
            <span class="agent-json-md-title">📊 分析报告</span>
          </div>
          <div v-show="jsonMdExpandedMap[item.id]" class="agent-json-md-content">
            <t-chat-content :content="item.jsonMarkdown" />
          </div>
        </div>
        <t-chat-content v-if="item.content.length > 0" :content="item.content" />
      </template>
      <template #actions="{ item, index }">
        <t-chat-action
            :content="item.content"
            :operation-btn="['copy']"
            @operation="handleOperation"
        />
      </template>
      <template #footer>
          <t-chat-sender
              ref="chatSenderRef"
              v-model="inputValue"
              class="chat-sender"
              :textarea-props="{
                placeholder: '请输入消息...',
              }"
              :loading="isStreaming"
              :stop-disabled="isStreaming"
              @send="inputEnter"
              @stop="onStop"
          >
            <template #suffix>
              <t-button theme="default" variant="text" size="large" class="btn" @click="inputEnter"> 发送 </t-button>
            </template>
            <template #prefix>
              <NFlex>
                <NSelect
                    v-model:value="aiConfigStore.activeConfigId"
                    :options="aiConfigStore.configs"
                    label-field="name" value-field="ID"
                    size="tiny"
                    style="width: 200px;"
                />
                <NSelect
                    v-model:value="agentMode"
                    :options="agentModeOptions"
                    size="tiny"
                    style="width: 120px;"
                />
              </NFlex>
            </template>
          </t-chat-sender>

      </template>
    </t-chat>
    <t-button v-show="isShowToBottom" variant="text" class="bottomBtn" @click="backBottom">
      <div class="to-bottom">
        <ArrowDownIcon />
      </div>
    </t-button>
  </div>
</template>
<script setup lang="ts">
import {ref, onMounted, h, onBeforeUnmount, onBeforeMount, nextTick, type VNode} from 'vue';
import {ArrowDownIcon, CheckCircleIcon, SystemSumIcon} from 'tdesign-icons-vue-next';
const isStreaming = ref(false);

const inputValue = ref('');
let formatTimer: ReturnType<typeof setInterval> | null = null

const chatRef = ref(null);
const isShowToBottom = ref(false);

const icon = ref('');
import {darkTheme, NFlex, NImage,NSelect} from "naive-ui";
import {ChatWithAgent, GetConfig, GetVersionInfo} from "../../wailsjs/go/main/App";
import type { AIConfig } from '../../wailsjs/go/models'
import { useAiConfigStore } from '../stores/aiConfig';
import {EventsOff, EventsOn} from '../../wailsjs/runtime'
import 'tdesign-vue-next/es/style/index.css';

const aiConfigStore = useAiConfigStore()
const allowToolTip = ref(true);
const chatSenderRef = ref(null);
const agentMode = ref('auto')
const agentModeOptions = [
  { label: '🤖 自动', value: 'auto' },
  { label: '⚡ 快速', value: 'react' },
  { label: '🧠 规划', value: 'plan_execute' },
]
const jsonMdExpandedMap = ref<Record<string, boolean>>({})

// 会话 ID 计数器，用于取消过期的流消息 (F-18)
let sessionIdCounter = 0
let currentSessionId = ''

function toggleJsonMd(id: string) {
  jsonMdExpandedMap.value = {
    ...jsonMdExpandedMap.value,
    [id]: !jsonMdExpandedMap.value[id]
  }
}

function getStepDotClass(step: string) {
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('⚡') || step.includes('🧠') || step.includes('📋') || step.includes('🔄')) return 'step-active'
  return ''
}

function startFormatTimer() {
  stopFormatTimer()
  formatTimer = setInterval(() => {
    const lastItem = chatList.value[0]
    if (lastItem && lastItem.role === 'assistant') {
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent)
        lastItem.content = fmt.content
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
      }
      if (lastItem.rawReasoning) {
        const fmt = formatMarkdown(lastItem.rawReasoning)
        lastItem.reasoning = fmt.content
      }
    }
  }, 1500)
}

function stopFormatTimer() {
  if (formatTimer !== null) {
    clearInterval(formatTimer)
    formatTimer = null
  }
}

function formatMarkdown(content: string) {
  if (!content) return { content: '', jsonMarkdown: '' }

  const { content: cleaned, jsonMarkdown } = wrapInlineJson(content)

  let inCodeBlock = false
  const lines = cleaned.split('\n')
  const result: string[] = []

  for (let i = 0; i < lines.length; i++) {
    let line = lines[i]
    const trimmed = line.replace(/^[\t ]+/, '')

    if (trimmed.startsWith('```')) {
      inCodeBlock = !inCodeBlock
      if (!inCodeBlock) {
        result.push(trimmed)
        continue
      }
    }

    if (inCodeBlock) {
      result.push(line)
      continue
    }

    if (trimmed !== line && trimmed !== '') {
      line = trimmed
    }

    if (i > 0 && isBlockElement(trimmed)) {
      const prev = result.length > 0 ? result[result.length - 1] : ''
      if (prev !== '' && !isBlockElement(prev.replace(/^[\t ]+/, ''))) {
        result.push('')
      }
    }

    line = splitInlineHeading(line)

    result.push(line)
  }

  return {
    content: result.join('\n'),
    jsonMarkdown
  }
}

function hasMarkdownContent(str: string) {
  if (!str || typeof str !== 'string') return false
  return /(^|\n)\s*#{1,6}\s/.test(str) ||
    /(^|\n)\s*\|/.test(str) ||
    /(^|\n)\s*---/.test(str) ||
    /(^|\n)\s*[-*+]\s/.test(str) ||
    /(^|\n)\s*>\s/.test(str) ||
    /(^|\n)\s*```/.test(str)
}

function extractMarkdownFromJson(obj: any): string | null {
  if (typeof obj === 'string') return obj
  if (Array.isArray(obj)) {
    const items = obj.map((item: any) => typeof item === 'string' ? item : JSON.stringify(item, null, 2))
    return items.join('\n\n')
  }
  if (typeof obj === 'object' && obj !== null) {
    for (const key of ['response', 'content', 'text', 'result', 'answer', 'message', 'output']) {
      if (obj[key] != null) {
        const val = obj[key]
        if (typeof val === 'string' && hasMarkdownContent(val)) return val
        if (typeof val === 'object') {
          const extracted = extractMarkdownFromJson(val)
          if (extracted) return extracted
        }
      }
    }
    const values = Object.values(obj).filter(v => typeof v === 'string' && hasMarkdownContent(v))
    if (values.length > 0) return values.join('\n\n')
    const strValues = Object.values(obj).filter(v => typeof v === 'string')
    if (strValues.length > 0) return strValues.join('\n\n')
  }
  return null
}

function wrapInlineJson(content: string) {
  if (!content) return { content: '', jsonMarkdown: '' }
  const cleaned: string[] = []
  const jsonParts: string[] = []
  let i = 0
  const len = content.length
  let inCodeBlock = false
  const MAX_SCAN = 500000 // (F-24) 最多扫描 50 万字符

  while (i < len && i < MAX_SCAN) {
    if (content.substring(i, i + 3) === '```') {
      inCodeBlock = !inCodeBlock
      cleaned.push('```')
      i += 3
      continue
    }

    if (inCodeBlock) {
      cleaned.push(content[i])
      i++
      continue
    }

    if (content[i] === '{') {
      const end = findJsonEnd(content, i)
      if (end > i) {
        const jsonStr = content.substring(i, end + 1)
        try {
          const obj = JSON.parse(jsonStr)
          const md = extractMarkdownFromJson(obj)
          if (md) {
            jsonParts.push(md)
          } else {
            cleaned.push('\n\n```json\n' + jsonStr + '\n```\n\n')
          }
          i = end + 1
          continue
        } catch {}
      }
    }
    cleaned.push(content[i])
    i++
  }
  // 超出 MAX_SCAN 的字符直接追加 (F-24)
  if (i < len) {
    cleaned.push(content.substring(i))
  }

  return {
    content: cleaned.join(''),
    jsonMarkdown: jsonParts.join('\n\n---\n\n')
  }
}

function findJsonEnd(content: string, start: number) {
  let depth = 0
  let bracketDepth = 0
  let inStr = false
  let escape = false
  const MAX_DEPTH = 10000 // (F-24) 最多扫描 1 万字符
  const limit = Math.min(content.length, start + MAX_DEPTH)
  for (let i = start; i < limit; i++) {
    const ch = content[i]
    if (escape) { escape = false; continue }
    if (ch === '\\' && inStr) { escape = true; continue }
    if (ch === '"') { inStr = !inStr; continue }
    if (inStr) continue
    if (ch === '[') bracketDepth++
    else if (ch === ']') bracketDepth--
    else if (ch === '{') depth++
    else if (ch === '}') {
      depth--
      if (depth === 0 && bracketDepth === 0) return i
    }
  }
  return -1
}

function splitInlineHeading(line: string) {
  const match = line.match(/(#{1,6}\s+\S)/)
  if (!match) return line
  const idx = match.index!
  if (idx === 0) return line
  const prefix = line.substring(0, idx)
  if (prefix.trim() === '') return line
  return prefix + '\n\n' + line.substring(idx)
}

function isBlockElement(line: string) {
  if (!line || line.length === 0) return false
  if (line[0] === '#') return true
  if (line.startsWith('- ') || line.startsWith('* ') || line.startsWith('+ ')) return true
  if (line.startsWith('```')) return true
  if (line.startsWith('> ')) return true
  if (line.length >= 2 && line[0] >= '1' && line[0] <= '9' && line[1] === '.') return true
  if (line.startsWith('---') || line.startsWith('***') || line.startsWith('___')) return true
  if (line.startsWith('|')) return true
  return false
}

function parseStepText(text: string) {
  if (!text) return [text]
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
      const entries = Object.entries(obj)
      if (entries.length > 0) {
        return entries.map(([k, v]) => `${k}: ${typeof v === 'string' ? v : JSON.stringify(v)}`)
      }
    }
    return [text]
  } catch {
    return [text]
  }
}

let msgIdCounter = 0
function nextMsgId(): string {
  return `msg_${++msgIdCounter}_${Date.now()}`
}

const handleAgentMessage = (data: any) => {
  console.log(data)
  // 过滤过期会话消息 (F-18)
  if (data?.sessionId && data.sessionId !== currentSessionId) {
    return
  }
  if(data?.role==="assistant"){
    isStreaming.value = false;
    const lastItem = chatList.value[0]
    if (!lastItem || lastItem.role !== 'assistant') { // (F-17)
      console.warn('handleAgentMessage: no assistant placeholder found')
      return
    }
    if (data['reasoning_content']){
      const rc = data['reasoning_content']
      if (rc.startsWith('[STATUS]')) {
        lastItem.statusText = rc.replace(/^\[STATUS\]/, '').trim()
      } else if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!lastItem.steps) lastItem.steps = []
          const parsed = parseStepText(stepText)
          lastItem.steps.push(...parsed)
        }
      } else {
        lastItem.rawReasoning = (lastItem.rawReasoning || '') + rc
        lastItem.reasoning = lastItem.rawReasoning
      }
    }
    if (data['content']){
      if (data['content'] === 'agent-DONE') {
        // terminal signal, not displayable content
      } else {
        lastItem.rawContent = (lastItem.rawContent || '') + data['content']
        lastItem.content = lastItem.rawContent
      }
    }
    if(data['tool_calls']){
      for (const tool of  data['tool_calls']) {
          console.log(tool.id, tool.type, tool.function.name, tool.function.arguments);
        lastItem.reasoning += "\n```"+tool.function.name+"\n" +
            "参数："+ (tool.function.arguments?tool.function.arguments:"无")+
            "\n```\n";
      }
    }
  }
  if(data['response_meta']&&data['response_meta'].finish_reason==="stop"){
    isStreaming.value = false;
    stopFormatTimer()
    const lastItem = chatList.value[0]
    if (lastItem) {
      if (lastItem.rawContent) {
        const fmt = formatMarkdown(lastItem.rawContent)
        lastItem.content = fmt.content
        if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
      }
      if (lastItem.rawReasoning) {
        const fmt = formatMarkdown(lastItem.rawReasoning)
        lastItem.reasoning = fmt.content
      }
    }
  }
}

// (F-19/F-20) 统一注册/注销事件
onBeforeMount(() => {
  EventsOff("agent-message", handleAgentMessage)
  EventsOn("agent-message", handleAgentMessage)
})

onMounted(() => {
  aiConfigStore.loadConfigs().catch(() => {})

  GetConfig().then((res) => {
    if (res.darkTheme) {
      document.documentElement.setAttribute("theme-mode", "dark");
    } else {
      document.documentElement.removeAttribute("theme-mode");    }
  }).catch(() => {})

  GetVersionInfo().then((res) => {
    icon.value = res.icon;
  }).catch(() => {})
});

onBeforeUnmount(() => {
  stopFormatTimer()
  EventsOff("agent-message", handleAgentMessage)
})

const backBottom = () => {
  chatRef.value.scrollToBottom({
    behavior: 'smooth',
  });
};

const handleChatScroll = function ({ e }: any) {
  const scrollTop = e.target.scrollTop;
  isShowToBottom.value = scrollTop < 0;
};

const clearConfirm = function () {
  chatList.value = [];
};

const handleOperation = function (type: string, options: any) {
  console.log('handleOperation', type, options);
  if (type === 'copy' && options?.content) { // (F-28)
    navigator.clipboard.writeText(options.content).then(() => {
      // 使用 TDesign 的 MessagePlugin
      import('tdesign-vue-next').then(({ MessagePlugin }) => {
        MessagePlugin.success('已复制')
      })
    }).catch(() => {
      import('tdesign-vue-next').then(({ MessagePlugin }) => {
        MessagePlugin.error('复制失败')
      })
    })
  }
};

interface ChatMessage {
  id: number
  avatar: string | VNode
  name: string
  datetime: string
  content: string
  reasoning?: string
  rawContent?: string
  rawReasoning?: string
  jsonMarkdown?: string
  role: 'assistant' | 'user'
  duration?: number
  steps?: string[]
}

const chatList = ref<ChatMessage[]>([
  {
    id: nextMsgId(),
    avatar: '',
    name: 'Blades-Stock AI',
    datetime: '',
    reasoning: '',
    content: '我是您的AI赋能股票分析助手,您可以问我任何关于股票投资方面的问题。',
    role: 'assistant',
    duration: 10,
  },
  {
    id: nextMsgId(),
    avatar: 'https://tdesign.gtimg.com/site/avatar.jpg',
    name: '宇宙无敌大韭菜',
    datetime: '',
    content: '介绍下自己？',
    role: 'user',
    reasoning: '',
  },
]);
// 动态更新头像 (F-26)
chatList.value[0].avatar = h(NImage, { src: icon.value, height: '48px', width: '48px'})

const onStop = function () {
  currentSessionId = '' // 标记当前会话已取消 (F-18)
  isStreaming.value = false;
  stopFormatTimer()
  const lastItem = chatList.value[0]
  if (lastItem && lastItem.role === 'assistant') {
    if (lastItem.rawContent) {
      const fmt = formatMarkdown(lastItem.rawContent)
      lastItem.content = fmt.content
      if (fmt.jsonMarkdown) lastItem.jsonMarkdown = fmt.jsonMarkdown
    }
    if (lastItem.rawReasoning) {
      const fmt = formatMarkdown(lastItem.rawReasoning)
      lastItem.reasoning = fmt.content
    }
  }
};

const inputEnter = function () {
  if (isStreaming.value) return
  if (!inputValue.value) return

  // (F-21) 检查 AI 配置
  const configId = aiConfigStore.activeConfigId
  if (!configId || configId <= 0 || !aiConfigStore.activeConfig) {
    import('tdesign-vue-next').then(({ MessagePlugin }) => {
      MessagePlugin.warning('请先选择或创建 AI 配置')
    })
    return
  }

  const params = {
    id: nextMsgId(),
    avatar: 'https://tdesign.gtimg.com/site/avatar.jpg',
    name: '宇宙无敌大韭菜',
    datetime: new Date().toDateString(),
    content: inputValue.value,
    role: 'user',
  };
  chatList.value.unshift(params);

  // 创建新的会话 ID (F-18)
  currentSessionId = `session_${++sessionIdCounter}_${Date.now()}`
  const assisId = nextMsgId()
  const params2 = {
    id: assisId,
    avatar: '',
    name: 'Blades-Stock AI',
    datetime: new Date().toDateString(),
    content: '',
    rawContent: '',
    reasoning: '',
    rawReasoning: '',
    jsonMarkdown: '',
    role: 'assistant',
    steps: [],
    statusText: '',
  };
  params2.avatar = h(NImage, { src: icon.value, height: '48px', width: '48px'}) // (F-26)
  chatList.value.unshift(params2);
  isStreaming.value = true;
  startFormatTimer()
  jsonMdExpandedMap.value = { ...jsonMdExpandedMap.value, [assisId]: true } // (F-23)

  // (F-22) 确保占位已插入
  nextTick(() => {
    ChatWithAgent(inputValue.value, configId, 0, false, 0, false, agentMode.value === 'auto' ? '' : agentMode.value)
      .catch((err: any) => {
        console.error('ChatWithAgent error', err)
        isStreaming.value = false
        stopFormatTimer()
      })
  })
};
</script>
<style lang="less">
::-webkit-scrollbar-thumb {
  background-color: var(--td-scrollbar-color);
}
::-webkit-scrollbar-thumb:horizontal:hover {
  background-color: var(--td-scrollbar-hover-color);
}
::-webkit-scrollbar-track {
  background-color: var(--td-scroll-track-color);
}
.chat-box {
  position: relative;
  height: 100%;
  margin: 5px 10px 5px 10px;
  text-align: left;
  .bottomBtn {
    position: absolute;
    left: 50%;
    margin-left: -20px;
    bottom: 210px;
    padding: 0;
    border: 0;
    width: 40px;
    height: 40px;
    border-radius: 50%;
    box-shadow: 0px 8px 10px -5px rgba(0, 0, 0, 0.08), 0px 16px 24px 2px rgba(0, 0, 0, 0.04),
    0px 6px 30px 5px rgba(0, 0, 0, 0.05);
  }
  .to-bottom {
    width: 40px;
    height: 40px;
    border: 1px solid #dcdcdc;
    box-sizing: border-box;
    background: var(--td-bg-color-container);
    border-radius: 50%;
    font-size: 24px;
    line-height: 40px;
    display: flex;
    align-items: center;
    justify-content: center;
    .t-icon {
      font-size: 24px;
    }
  }
}

.model-select {
  display: flex;
  align-items: center;
  .t-select {
    width: 112px;
    height: 32px;
    margin-right: 8px;
    .t-input {
      border-radius: 32px;
      padding: 0 15px;
    }
  }
  .check-box {
    width: 112px;
    height: 32px;
    border-radius: 32px;
    border: 0;
    background: #e7e7e7;
    color: rgba(0, 0, 0, 0.9);
    box-sizing: border-box;
    flex: 0 0 auto;
    .t-button__text {
      display: flex;
      align-items: center;
      justify-content: center;
      span {
        margin-left: 4px;
      }
    }
  }
  .check-box.is-active {
    border: 1px solid #d9e1ff;
    background: #f2f3ff;
    color: var(--td-brand-color);
  }
}


.chat-sender {
  .btn {
    color: var(--td-text-color-disabled);
    border: none;
    &:hover {
      color: var(--td-brand-color-hover);
      border: none;
      background: none;
    }
  }
  .btn.t-button {
    height: var(--td-comp-size-m);
    padding: 0;
  }
  .model-select {
    display: flex;
    align-items: center;
    .t-select {
      width: 112px;
      height: var(--td-comp-size-m);
      margin-right: var(--td-comp-margin-s);
      .t-input {
        border-radius: 32px;
        padding: 0 15px;
      }
      .t-input.t-is-focused {
        box-shadow: none;
      }
    }
    .check-box {
      width: 112px;
      height: var(--td-comp-size-m);
      border-radius: 32px;
      border: 0;
      background: var(--td-bg-color-component);
      color: var(--td-text-color-primary);
      box-sizing: border-box;
      flex: 0 0 auto;
      .t-button__text {
        display: flex;
        align-items: center;
        justify-content: center;
        span {
          margin-left: var(--td-comp-margin-s);
        }
      }
    }
    .check-box.is-active {
      border: 1px solid var(--td-brand-color-focus);
      background: var(--td-brand-color-light);
      color: var(--td-text-color-brand);
    }
  }
}

.agent-steps {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.06) 0%, rgba(46, 139, 87, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
}
.agent-steps-badge {
  font-size: 11px;
  background: var(--td-brand-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.agent-steps-list {
  padding: 8px 10px 8px 14px;
  max-height: 250px;
  overflow-y: auto;
}
.agent-step-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 3px 0;
  position: relative;
  font-size: 12px;
  color: var(--td-text-color-secondary);
  line-height: 1.5;
}
.agent-step-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 16px;
  bottom: -3px;
  width: 1px;
  background: var(--td-component-border);
}
.agent-step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--td-text-color-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
}
.agent-step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 4px rgba(230, 162, 60, 0.4);
}
.agent-step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 4px rgba(64, 158, 255, 0.4);
}
.agent-step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 4px rgba(103, 194, 58, 0.4);
}
.agent-step-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  text-align: left;
}

.agent-json-md {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-json-md-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  cursor: pointer;
  user-select: none;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.06) 0%, rgba(5, 150, 105, 0.06) 100%);
  border-bottom: 1px solid var(--td-component-border);
  transition: background 0.2s;
}
.agent-json-md-header:hover {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.12) 0%, rgba(5, 150, 105, 0.12) 100%);
}
.agent-json-md-arrow {
  transition: transform 0.2s;
  flex-shrink: 0;
}
.agent-json-md-arrow-expanded {
  transform: rotate(90deg);
}
.agent-json-md-title {
  font-size: 13px;
  font-weight: 500;
}
.agent-json-md-content {
  padding: 10px;
  max-height: 500px;
  overflow-y: auto;
  text-align: left;
}

.agent-steps {
  margin-bottom: 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 6px;
  overflow: hidden;
  background: var(--td-bg-color-container-hover);
}
.agent-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-component);
  border-bottom: 1px solid var(--td-component-border);
}
.agent-steps-badge {
  background: var(--td-brand-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  font-size: 11px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.agent-steps-list {
  padding: 6px 10px;
}
.agent-step-item {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  padding: 2px 0;
  font-size: 12px;
  line-height: 1.5;
}
.agent-step-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 5px;
  flex-shrink: 0;
}
.agent-step-dot.step-done { background: #22c55e; }
.agent-step-dot.step-tool { background: #3b82f6; }
.agent-step-dot.step-active { background: #f59e0b; animation: agent-dot-pulse 1.2s infinite; }
@keyframes agent-dot-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
.agent-step-text { word-break: break-all; color: var(--td-text-color-secondary); }
.agent-status {
  background: var(--td-brand-color-light, #e8f5e9);
  border-radius: 6px;
  padding: 6px 12px;
  margin-bottom: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.agent-status::before {
  content: '';
  display: inline-block;
  width: 12px;
  height: 12px;
  border: 2px solid var(--td-brand-color, #0052d9);
  border-top-color: transparent;
  border-radius: 50%;
  animation: agent-status-spin 0.8s linear infinite;
}
.agent-status-text {
  font-size: 13px;
  color: var(--td-brand-color, #0052d9);
}
@keyframes agent-status-spin {
  to { transform: rotate(360deg); }
}
</style>