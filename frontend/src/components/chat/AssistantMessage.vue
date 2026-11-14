<template>
  <div class="msg-wrapper assistant">
    <div class="msg-avatar">
      <n-avatar :size="40" :src="avatar" round />
    </div>
    <div class="msg-content">
      <div class="msg-header">
        <span class="msg-name">{{ name }}</span>
        <span class="msg-time">{{ formatTime(datetime) }}</span>
        <span v-if="isStreaming" class="streaming-badge">生成中…</span>
      </div>

      <!-- 1. 思考过程（可折叠，默认折叠；流式时自动展开） -->
      <CollapsibleSection
        v-if="reasoning || rawReasoning"
        title="🧠 思考过程"
        :expanded="expandedReasoning"
        @toggle="expandedReasoning = !expandedReasoning"
        class="section-reasoning"
      >
        <ChatMarkdown :content="reasoning || rawReasoning || ''" />
      </CollapsibleSection>

      <!-- 2. 执行步骤（可折叠，默认折叠） -->
      <CollapsibleSection
        v-if="steps && steps.length > 0"
        :title="`📋 执行步骤 (${steps.length})`"
        :expanded="expandedSteps"
        @toggle="expandedSteps = !expandedSteps"
        class="section-steps"
      >
        <div class="steps-list">
          <div
            v-for="(step, si) in steps"
            :key="si"
            class="step-item"
            :class="{ 'step-done': step.includes('✅'), 'step-tool': step.includes('🔧'), 'step-active': step.includes('⚡') || step.includes('🔄') }"
          >
            <span class="step-dot" :class="getStepDotClass(step)"></span>
            <span class="step-text">{{ step }}</span>
          </div>
        </div>
      </CollapsibleSection>

      <!-- 3. 工具调用时间轴（新增动画版） -->
      <ToolTimeline
        v-if="toolCalls && toolCalls.length > 0"
        :calls="toolCalls"
        :expanded="expandedTools"
        @toggle="expandedTools = !expandedTools"
        @copy-args="handleCopyToolArgs"
        @copy-result="handleCopyToolResult"
      />

      <!-- 4. 结构化报告（可折叠，默认折叠） -->
      <CollapsibleSection
        v-if="jsonMarkdown"
        title="📊 结构化报告"
        :expanded="expandedReport"
        @toggle="expandedReport = !expandedReport"
        class="section-report"
      >
        <ChatMarkdown :content="jsonMarkdown" />
      </CollapsibleSection>

      <!-- 5. 最终回答（主内容，始终展示） -->
      <div class="msg-main-content" :id="anchorId">
        <ChatMarkdown v-if="content" :content="content" />
        <div v-else-if="isStreaming" class="streaming-placeholder">
          <!-- 采用与首行文本等高的紧凑打字占位符，彻底消除原 5 行骨架屏 (150px) 瞬间坍缩为首字 1 行 (24px) 的剧烈上下震荡 -->
          <div class="compact-typing-indicator">
            <span class="typing-dot"></span>
            <span class="typing-dot"></span>
            <span class="typing-dot"></span>
            <span class="typing-text">{{ message.statusText || '正在思考并撰写投研报告...' }}</span>
          </div>
        </div>
        <!-- 空回复：绝不能只显示「（空回复）」——用户看到这四个字时完全无法
             判断是软件坏了、网络不通、还是模型问题，只能反复重试。
             这里根据**实际握有的事实**分三种情况给出可行动的说明：
               ① 有思考内容 → 把思考折叠区指出来（答案其实“藏在思考里”）
               ② 有工具过程 → 说明模型调了工具但没写结论
               ③ 什么都没有 → 给排查步骤
             后端现已把这种情况归类为「静默空答」并返回带原因的文案，
             这里是第二道防线（覆盖旧构建 / 历史消息 / 其他入口）。 -->
        <div v-else class="empty-state empty-state-actionable">
          <div class="empty-title">⚠️ 模型没有给出正文回答</div>
          <div v-if="reasoning || rawReasoning">
            本轮模型产出了<b>思考内容</b>但没有写出最终答案，思考过程已折叠在上方
            <b>「💭 思考过程」</b>中。
            <div class="empty-hint-title">怎么办：</div>
            <ol>
              <li>到「设置 → AI模型注册表」关闭该卡片的<b>「深度思考」</b>后重试（最常见原因：思考吃掉了全部输出预算）</li>
              <li>或把该卡片的<b>「最大输出长度 (maxTokens)」</b>调大</li>
              <li>或换用输出更稳定的模型（MiniMax-M3 / DeepSeek 等）</li>
            </ol>
          </div>
          <div v-else-if="toolCalls.length">
            模型执行了 <b>{{ toolCalls.length }}</b> 次工具调用，但没有写出结论。
            <div class="empty-hint-title">怎么办：</div>
            <ol>
              <li>直接重新发送（上游空答多为间歇性，重试常可恢复）</li>
              <li>若反复出现，到「设置 → AI模型注册表」点该卡片<b>「检测连通性」</b>，或切换其他模型</li>
            </ol>
          </div>
          <div v-else>
            上游模型服务<b>没有返回任何内容</b>（连接建立后即结束）。
            <div class="empty-hint-title">怎么办：</div>
            <ol>
              <li>重新发送一次（大概率为上游瞬时异常）</li>
              <li>仍无改善时，到「设置 → AI模型注册表」点该卡片<b>「检测连通性」</b>确认可用</li>
              <li>或切换为其他 AI 配置</li>
            </ol>
          </div>
        </div>

        <!-- 流式收尾异常提示：必须在流式结束后仍可见，
             否则用户只看到“回复突然结束”，无从判断原因。 -->
        <div v-if="message.streamNotice && !isStreaming" class="stream-notice">
          {{ message.streamNotice }}
          <!-- 看门狗超时：渲染**真实按钮**并向外 emit，
               绝不把 HTML 拼进 streamNotice（那会被 {{ }} 当成纯文本原样显示）。 -->
          <div v-if="message.streamTimeout" class="stream-notice-actions">
            <button type="button" class="notice-btn notice-btn-primary" @click="emit('retry')">
              🔄 重新发送
            </button>
            <button type="button" class="notice-btn" @click="emit('continueWait')">
              ⏳ 继续等待
            </button>
            <button type="button" class="notice-btn notice-btn-danger" @click="emit('stop')">
              ⏹ 停止本轮
            </button>
          </div>
        </div>
      </div>

      <!-- 5.1 伴随动作卡片 (Agent Action Chips) —— 将 raw agent-action JSON 转化为高交互性原生芯片 -->
      <div v-if="actions && actions.length > 0 && !isStreaming" class="action-chips-container">
        <div class="action-chips-header">
          <span class="action-chips-icon">⚡</span>
          <span class="action-chips-label">智能体建议系统联动：</span>
        </div>
        <div class="action-chips-list">
          <button
            v-for="(act, ai) in actions"
            :key="ai"
            class="action-chip-btn"
            @click="handleTriggerAction(act)"
            :title="getActionTooltip(act)"
          >
            <span class="action-btn-icon">{{ getActionIcon(act) }}</span>
            <span class="action-btn-text">{{ getActionLabel(act) }}</span>
            <span class="action-btn-arrow">➔</span>
          </button>
        </div>
      </div>

      <!-- 6. 性能指标胶囊（底部，低优先级） -->
      <div v-if="metrics" class="metrics-capsule">
        <span class="metric-tag" title="端到端总耗时">⏱️ {{ formatDuration(metrics.total_duration) }}</span>
        <span v-if="metrics.first_chunk_latency" class="metric-tag" title="首包延迟">⚡ {{ formatDuration(metrics.first_chunk_latency) }}</span>
        <span v-if="metrics.tool_call_count > 0" class="metric-tag" title="工具调用次数">🔧 {{ metrics.tool_call_count }}次工具</span>
        <span v-if="metrics.retry_count > 0" class="metric-tag metric-warning" title="重试次数">🔁 {{ metrics.retry_count }}次重试</span>
        <span v-if="metrics.tokens?.total" class="metric-tag" title="Token 使用">📝 {{ metrics.tokens.total }} tokens</span>
        <span v-if="metrics.cost_usd" class="metric-tag" title="预估成本">💰 $${metrics.cost_usd.toFixed(4)}</span>
      </div>

      <!-- 7. 消息操作栏 (生成中隐藏，完成后渐入，杜绝操作条随流式文本逐行下移震荡) -->
      <div v-if="!isStreaming" class="msg-actions">
        <n-button
          v-if="hasStockThesisPotential"
          size="tiny"
          text
          type="primary"
          title="将分析结论与买方假说沉淀入标的认知档案账本"
          @click="$emit('saveThesis', message)"
        >
          📥 存入买方认知账本
        </n-button>
        <n-button size="tiny" text @click="$emit('copy', content)">📋 复制</n-button>
        <n-button size="tiny" text @click="$emit('retry')">🔄 重试</n-button>
        <n-button size="tiny" text @click="$emit('reference')">🔗 引用</n-button>
        <n-button size="tiny" text title="导出为高清研报图片" @click="$emit('exportImage', message)">🖼️ 导出图片</n-button>
        <n-button size="tiny" text @click="copyAnchorLink">📍 链接</n-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, inject } from 'vue'
import { NAvatar, NButton, useMessage } from 'naive-ui'
import CollapsibleSection from './CollapsibleSection.vue'
import ToolTimeline from './ToolTimeline.vue'
import ChatMarkdown from './ChatMarkdown.vue'
import { copyToClipboard } from '@/utils/clipboard'
import { dispatchAgentAction, type AgentAction } from '@/utils/agentControlBus'
import { stripAllAgentActionBlocks } from '@/utils/cleanText'
import {
  formatDuration,
  type ChatMessage
} from './types'
import type { RegisterMessageElement, UnregisterMessageElement } from './useMessageAnchor'

const props = defineProps<{
  message: ChatMessage
  avatar: string
  name: string
  isStreaming?: boolean
}>()

// 事件全部通过模板中的 $emit 触发，无需在脚本中持有 emit 引用
const emit = defineEmits<{
  copy: [content: string]
  retry: []
  reference: []
  saveThesis: [msg: ChatMessage]
  exportImage: [msg: ChatMessage]
  // 看门狗超时后的用户选择（由 agent-chat.vue 处理）
  continueWait: []
  stop: []
}>()

// 注入锚点注册函数
const registerElement = inject<RegisterMessageElement>('registerMessageElement')
const unregisterElement = inject<UnregisterMessageElement>('unregisterMessageElement')

// 检测内容是否具有买方立论潜在特征
const hasStockThesisPotential = computed(() => {
  const c = content.value || ''
  if (!c || c.length < 20) return false
  const hasCode = /\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b/.test(c)
  const hasKeywords = /(逻辑|催化|证伪|买方|立论|研报|估值|评级|止损|仓位|趋势|支撑|阻力|买点|卖点|风险)/.test(c)
  return hasCode || hasKeywords || c.length > 50
})

// 响应式提取消息字段
const content = computed(() => stripAllAgentActionBlocks(props.message.content || ''))
const reasoning = computed(() => props.message.reasoning || '')
const rawReasoning = computed(() => props.message.rawReasoning || '')
const jsonMarkdown = computed(() => props.message.jsonMarkdown || '')
const steps = computed(() => props.message.steps || [])
const toolCalls = computed(() => props.message.toolCalls || [])
const actions = computed(() => props.message.actions || [])
const metrics = computed(() => props.message.metrics)
const datetime = computed(() => props.message.datetime)

function getActionIcon(act: any): string {
  switch (act?.type) {
    case 'OPEN_DIAGNOSIS': return '🎯'
    case 'SHOW_KLINE': return '📈'
    case 'NAVIGATE_VIEW': return '🧭'
    case 'OPEN_MARKET_INTELLIGENCE': return '🧠'
    case 'DISCIPLINE_ALERT': return '⚠️'
    default: return '⚡'
  }
}

function getActionLabel(act: any): string {
  if (!act) return '系统联动'
  const p = act.payload || {}
  switch (act.type) {
    case 'OPEN_DIAGNOSIS':
      return `调起 3D 量化体检: ${p.name || ''} (${p.code || ''})`
    case 'SHOW_KLINE':
      return `查看 K 线走势: ${p.name || ''} (${p.code || ''})`
    case 'NAVIGATE_VIEW':
      return `跳转至功能页面: ${p.path || ''}`
    case 'OPEN_MARKET_INTELLIGENCE':
      return `打开 AI 市场智能中枢`
    case 'DISCIPLINE_ALERT':
      return p.title || '风控操盘纪律提醒'
    default:
      return `执行联动: ${act.type}`
  }
}

function getActionTooltip(act: any): string {
  return `点击立即触发系统联动动作: ${act?.type || ''}`
}

function handleTriggerAction(act: any) {
  if (act) {
    dispatchAgentAction(act as AgentAction)
  }
}

// 展开状态（默认全折叠）
const expandedReasoning = ref(false)
const expandedSteps = ref(false)
const expandedTools = ref(false)
const expandedReport = ref(false)

// 如果正在流式，自动展开思考过程
watch(() => props.isStreaming, (val) => {
  if (val) {
    expandedReasoning.value = true
  }
}, { immediate: true })

// 锚点 ID（用于深度链接）
const anchorId = computed(() => `msg-${props.message.id}`)

// 注册/注销锚点元素
onMounted(() => {
  const el = document.getElementById(anchorId.value)
  if (el) registerElement?.(anchorId.value, el)
})

onUnmounted(() => {
  unregisterElement?.(anchorId.value)
})

function formatTime(dt: string): string {
  if (!dt) return ''
  const date = new Date(dt)
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

function getStepDotClass(step: string): string {
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('⚡') || step.includes('🔄')) return 'step-active'
  return ''
}

// 修复（vue/no-dupe-keys 捕获）：此前该变量命名为 `message`，与 props 的
// `message: ChatMessage` **同名**。在 <script setup> 中 setup 绑定优先级高于 props，
// 模板里的 `message` 会被编译为这个 naive-ui 消息 API（而非消息实体），导致：
//   1) 模板 `{{ message.statusText }}` 永远取到 undefined → 流式占位文案恒为默认值；
//   2) `$emit('saveThesis', message)` / `$emit('exportImage', message)` 传给父组件的
//      是消息 API 对象而非 ChatMessage → 「存入买方认知账本」「导出图片」实际失效。
const msgApi = useMessage()

async function handleCopyToolArgs(args: string) {
  const ok = await copyToClipboard(args)
  if (ok) msgApi.success('参数已复制')
  else msgApi.error('复制失败，请手动选取文本')
}

async function handleCopyToolResult(result: string) {
  const ok = await copyToClipboard(result)
  if (ok) msgApi.success('结果已复制')
  else msgApi.error('复制失败，请手动选取文本')
}

async function copyAnchorLink() {
  const url = `${window.location.origin}${window.location.pathname}#/agent?msg=${props.message.id}`
  const ok = await copyToClipboard(url)
  if (ok) msgApi.success('消息链接已复制到剪贴板')
  else msgApi.error('复制失败，请手动复制浏览器地址')
}
</script>

<style scoped lang="less">
.stream-notice {
  margin-top: 10px;
  padding: 8px 12px;
  border-left: 3px solid #d29922;
  border-radius: 4px;
  background: rgba(210, 153, 34, 0.12);
  color: #d29922;
  font-size: 12.5px;
  line-height: 1.6;
}

/* 看门狗超时后的操作按钮（真实按钮，非注入 HTML） */
.stream-notice-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.notice-btn {
  padding: 5px 12px;
  font-size: 12.5px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.25);
  background: transparent;
  color: inherit;
  cursor: pointer;
  transition: all 0.15s ease;
}

.notice-btn:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.45);
}

.notice-btn-primary {
  background: #18a058;
  border-color: #18a058;
  color: #fff;
}

.notice-btn-primary:hover {
  background: #36ad6a;
  border-color: #36ad6a;
}

.notice-btn-danger {
  color: #e88080;
  border-color: rgba(232, 128, 128, 0.5);
}

.notice-btn-danger:hover {
  background: rgba(232, 128, 128, 0.15);
}

.msg-wrapper {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  align-items: flex-start;

  &.assistant {
    flex-direction: row;
  }
}

.msg-avatar {
  flex-shrink: 0;
}

.msg-content {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 85%;
  width: 100%;
}

.msg-header {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--bs-text-secondary);
}

.msg-name {
  font-weight: 600;
  color: var(--bs-text-primary);
}

.msg-time {
  color: var(--bs-text-disabled);
}

.streaming-badge {
  font-size: 11px;
  background: var(--bs-brand-light, #e8f5e9);
  color: var(--bs-brand, #0052d9);
  padding: 2px 8px;
  border-radius: 10px;
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

/* 折叠面板通用样式 */
.section-reasoning,
.section-steps,
.section-tools,
.section-report {
  border: 1px solid var(--bs-border-component);
  border-radius: 8px;
  background: var(--bs-bg-container-hover);
  overflow: hidden;
}

:deep(.collapsible-section) {
  border: none;
  background: transparent;
}

:deep(.collapsible-header) {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  color: var(--bs-text-secondary);
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.06) 0%, rgba(46, 139, 87, 0.06) 100%);
  border-bottom: 1px solid var(--bs-border-component);
  user-select: none;
  transition: background 0.2s;

  &:hover {
    background: linear-gradient(135deg, rgba(56, 173, 169, 0.12) 0%, rgba(46, 139, 87, 0.12) 100%);
  }
}

:deep(.collapsible-arrow) {
  transition: transform 0.2s;
  flex-shrink: 0;
  color: var(--bs-text-disabled);
}

:deep(.collapsible-arrow.expanded) {
  transform: rotate(90deg);
}

:deep(.collapsible-content) {
  padding: 10px 14px;
  max-height: 380px;
  overflow-y: auto;
  font-size: 12.5px !important;
  line-height: 1.58 !important;
  color: var(--bs-text-secondary, rgba(220, 226, 235, 0.78)) !important;

  .t-chat__text {
    font-size: 12.5px !important;
    line-height: 1.58 !important;
    padding: 0 !important;
  }

  .t-chat__text__content {
    font-size: 12.5px !important;
    line-height: 1.58 !important;

    p {
      font-size: 12.5px !important;
      line-height: 1.58 !important;
      margin: 4px 0 !important;
    }
  }
}

/* 步骤列表 */
.steps-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.step-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 4px 0;
  font-size: 12px;
  color: var(--bs-text-secondary);
  line-height: 1.5;
  position: relative;

  &:not(:last-child)::before {
    content: '';
    position: absolute;
    left: 4px;
    top: 16px;
    bottom: -4px;
    width: 1px;
    background: var(--bs-border-component);
  }
}

.step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--bs-text-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
  transition: all 0.2s;
}

.step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 6px rgba(230, 162, 60, 0.5);
  animation: dot-pulse 1.2s infinite;
}

.step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 6px rgba(64, 158, 255, 0.5);
}

.step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 6px rgba(103, 194, 58, 0.5);
}

@keyframes dot-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.step-text {
  flex: 1;
  min-width: 0;
  word-break: break-word;
}

/* 主内容区（买方投研报告高密度、护眼排版） */
.msg-main-content {
  background: var(--bs-bg-container, #171b22);
  border: 1px solid var(--bs-border-component, rgba(255, 255, 255, 0.08));
  border-radius: 8px;
  padding: 12px 16px;
  position: relative;
  min-height: 48px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.15);
  transition: all 0.2s ease;

  /* 彻底纠正原 TDesign 强制 16px 与 2.0 超大行高导致的字体过大与松散膨胀 */
  :deep(.t-chat__text) {
    font: inherit !important;
    font-size: 13.5px !important;
    line-height: 1.62 !important;
    padding: 0 !important;
    background: transparent !important;
    word-break: break-word !important;
  }

  :deep(.t-chat__text__assistant) {
    color: var(--bs-text-primary, rgba(230, 235, 242, 0.88)) !important;
    line-height: 1.62 !important;
  }

  :deep(.t-chat__text__content) {
    font-size: 13.5px !important;
    line-height: 1.62 !important;
    letter-spacing: 0.15px;

    /* 正文段落 */
    p {
      font-size: 13.5px !important;
      line-height: 1.62 !important;
      margin: 6px 0 !important;
      color: inherit;
    }

    /* 标题阶梯约束：彻底杜绝 32px/24px 巨幅突兀与刺目眩光 */
    h1 {
      font-size: 16px !important;
      font-weight: 650 !important;
      line-height: 1.4 !important;
      margin: 14px 0 8px 0 !important;
      padding-left: 8px !important;
      border-left: 3px solid var(--bs-brand, #38ada9) !important;
      color: var(--bs-text-primary, rgba(240, 245, 250, 0.95)) !important;
    }

    h2 {
      font-size: 15px !important;
      font-weight: 600 !important;
      line-height: 1.4 !important;
      margin: 12px 0 6px 0 !important;
      padding-left: 6px !important;
      border-left: 2.5px solid #409eff !important;
      color: var(--bs-text-primary, rgba(240, 245, 250, 0.92)) !important;
    }

    h3 {
      font-size: 14px !important;
      font-weight: 600 !important;
      line-height: 1.4 !important;
      margin: 10px 0 4px 0 !important;
      color: var(--bs-text-primary, rgba(240, 245, 250, 0.9)) !important;
    }

    h4, h5, h6 {
      font-size: 13px !important;
      font-weight: 600 !important;
      line-height: 1.4 !important;
      margin: 8px 0 4px 0 !important;
    }

    /* 列表紧凑化 */
    ul, ol {
      margin: 4px 0 6px 0 !important;
      padding-left: 20px !important;
      line-height: 1.6 !important;
    }

    li {
      font-size: 13.5px !important;
      margin: 3px 0 !important;
      line-height: 1.6 !important;
    }

    /* 加粗文本：柔和护眼权重与高光（杜绝生硬 700+ 纯白反光） */
    strong, b {
      font-weight: 600 !important;
      color: #79bbff !important;
    }

    /* 彻底消除原 TDesign 刺目的红色报警色行内代码块 (rgb(198, 71, 81)) */
    code:not(pre code) {
      color: #58a6ff !important;
      background: rgba(64, 158, 255, 0.12) !important;
      border: 1px solid rgba(64, 158, 255, 0.22) !important;
      border-radius: 4px !important;
      padding: 1px 6px !important;
      font-size: 12px !important;
      font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace !important;
      margin: 0 3px !important;
      word-break: break-word !important;
    }

    /* 结构化表格优化：紧凑型买方金融数据报表 */
    table {
      width: 100% !important;
      border-collapse: collapse !important;
      margin: 10px 0 !important;
      font-size: 12.5px !important;
      line-height: 1.5 !important;
      border: 1px solid var(--bs-border-component, rgba(255, 255, 255, 0.08)) !important;
      border-radius: 6px !important;
      overflow: hidden !important;
    }

    th {
      background: var(--bs-bg-secondary, rgba(255, 255, 255, 0.04)) !important;
      color: var(--bs-text-secondary, rgba(255, 255, 255, 0.72)) !important;
      font-weight: 600 !important;
      padding: 6px 10px !important;
      text-align: left !important;
      border: 1px solid var(--bs-border-component, rgba(255, 255, 255, 0.08)) !important;
    }

    td {
      padding: 5px 10px !important;
      border: 1px solid var(--bs-border-component, rgba(255, 255, 255, 0.08)) !important;
      font-variant-numeric: tabular-nums !important;
    }

    tbody tr:nth-child(even) {
      background: rgba(255, 255, 255, 0.02) !important;
    }

    tbody tr:hover {
      background: rgba(64, 158, 255, 0.06) !important;
    }

    /* 引用块优化：温和微卡片 */
    blockquote {
      border-left: 3px solid var(--bs-brand, #38ada9) !important;
      background: rgba(56, 173, 169, 0.06) !important;
      color: var(--bs-text-secondary, rgba(255, 255, 255, 0.72)) !important;
      padding: 6px 12px !important;
      margin: 8px 0 !important;
      border-radius: 0 6px 6px 0 !important;
      font-size: 13px !important;
      line-height: 1.55 !important;
    }

    /* 代码块优化 */
    pre, .hljs {
      background: #141820 !important;
      border: 1px solid rgba(255, 255, 255, 0.08) !important;
      border-radius: 6px !important;
      font-size: 12px !important;
      line-height: 1.5 !important;
      padding: 8px 12px !important;
      margin: 8px 0 !important;
    }

    /* 分割线 */
    hr {
      border: none !important;
      border-top: 1px solid var(--bs-border-component, rgba(255, 255, 255, 0.08)) !important;
      margin: 12px 0 !important;
    }
  }

  /* 消除 TDesign chat 内部组件的红色代码侵入 */
  :deep(.t-chat__text__assistant :where(code):not(:where(pre *))) {
    color: #58a6ff !important;
    background: rgba(64, 158, 255, 0.12) !important;
    border: 1px solid rgba(64, 158, 255, 0.22) !important;
  }
}

.streaming-placeholder {
  display: flex;
  align-items: center;
  min-height: 28px;
  padding: 4px 0;
}

.compact-typing-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--bs-text-secondary, #4e5969);

  .typing-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--bs-brand, #0052d9);
    animation: typing-bounce 1.4s infinite ease-in-out both;

    &:nth-child(1) { animation-delay: -0.32s; }
    &:nth-child(2) { animation-delay: -0.16s; }
  }

  .typing-text {
    font-size: 12px;
    opacity: 0.85;
    letter-spacing: 0.3px;
  }
}

@keyframes typing-bounce {
  0%, 80%, 100% {
    transform: scale(0.6);
    opacity: 0.4;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}

.empty-state {
  color: var(--bs-text-disabled);
  font-style: italic;
  text-align: center;
  padding: 20px;
}

/* 可行动的空回复提示：左对齐、非斜体（这是排查指引，不是一句占位文案），
   用警示色边框把它与普通正文区分开，避免被误读成模型正文。 */
.empty-state-actionable {
  text-align: left;
  font-style: normal;
  color: var(--bs-text-secondary);
  line-height: 1.85;
  padding: 14px 16px;
  border-left: 3px solid #e6a23c;
  border-radius: 6px;
  background: var(--bs-bg-secondary);
}

.empty-state-actionable .empty-title {
  font-weight: 600;
  color: #e6a23c;
  margin-bottom: 6px;
}

.empty-state-actionable .empty-hint-title {
  margin-top: 8px;
  font-weight: 600;
  color: var(--bs-text-primary);
}

.empty-state-actionable ol {
  margin: 4px 0 0;
  padding-left: 22px;
}

.empty-state-actionable li {
  margin-bottom: 3px;
}

/* 指标胶囊 */
.metrics-capsule {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  padding: 4px 10px;
  background: var(--bs-bg-secondary, rgba(140, 140, 140, 0.06));
  border: 1px solid var(--bs-border-1, rgba(140, 140, 140, 0.12));
  border-radius: 12px;
  font-size: 11px;
  color: var(--bs-text-secondary);
  user-select: none;
}

.metric-tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-weight: 500;
}

.metric-warning {
  color: #f59e0b;
  font-weight: 600;
}

/* 操作栏 */
.msg-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px 10px;
  margin-top: 6px;
  padding-left: 4px;
  border-top: 1px solid var(--bs-border-component);
  padding-top: 8px;
}

/* 智能体建议联动动作卡片栏 */
.action-chips-container {
  margin-top: 10px;
  padding: 8px 12px;
  background: var(--bs-bg-secondary, rgba(24, 160, 88, 0.06));
  border: 1px dashed var(--bs-brand, #18a058);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.action-chips-header {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  color: var(--bs-text-primary, #333);
}

.action-chips-icon {
  font-size: 14px;
}

.action-chips-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.action-chip-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: var(--bs-bg-container, #fff);
  border: 1px solid var(--bs-brand, #18a058);
  border-radius: 6px;
  color: var(--bs-brand, #18a058);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.action-chip-btn:hover {
  background: var(--bs-brand, #18a058);
  color: #fff;
  transform: translateY(-1px);
  box-shadow: 0 4px 10px rgba(24, 160, 88, 0.25);
}

.action-btn-arrow {
  font-size: 11px;
  opacity: 0.8;
  transition: transform 0.2s;
}

.action-chip-btn:hover .action-btn-arrow {
  transform: translateX(2px);
}

@media (max-width: 768px) {
  .msg-actions {
    gap: 4px 6px;
  }
  .msg-actions :deep(.t-button) {
    padding: 0 4px;
    font-size: 12px;
  }
}
</style>