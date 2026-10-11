<template>
  <Transition name="fade">
    <div
      v-if="showButton"
      :class="['edge-trigger', { 'edge-trigger-busy': hasBackgroundTask }]"
      :title="hasBackgroundTask ? 'Blades Stock AI Agent 助手正在后台分析...' : 'Blades Stock AI Agent 助手'"
      @click="togglePanel"
    >
      <div class="edge-trigger-inner">
        <NIcon
          :component="SparklesOutline"
          size="18"
        />
        <span class="edge-trigger-text">AI助手</span>
        <div
          v-if="hasBackgroundTask"
          class="edge-trigger-badge"
        />
      </div>
    </div>
  </Transition>

  <Transition name="drawer-slide">
    <div
      v-if="panelVisible"
      class="drawer-wrap"
    >
      <div
        class="drawer-mask"
        @click="closePanel"
      />
      <div
        class="drawer-panel"
        @click.stop
      >
        <NCard
          size="small"
          class="panel-card"
          :bordered="false"
          content-style="padding: 0; display: flex; flex-direction: column; min-height: 0; overflow: hidden;"
        >
          <template #header>
            <div class="panel-header">
              <span class="panel-title">Blades Stock AI Agent 助手</span>
              <div class="panel-actions">
                <NButton
                  size="small"
                  quaternary
                  title="开始新对话"
                  @click="startNewChat"
                >
                  新对话
                </NButton>
                <NButton
                  quaternary
                  circle
                  size="small"
                  title="分享到社区"
                  :loading="shareLoading"
                  @click="shareAiToCommunity"
                >
                  <template #icon>
                    <NIcon :component="ShareSocialOutline" />
                  </template>
                </NButton>
                <NButton
                  quaternary
                  circle
                  size="small"
                  title="关闭"
                  @click="closePanel"
                >
                  <template #icon>
                    <NIcon :component="CloseOutline" />
                  </template>
                </NButton>
              </div>
            </div>
          </template>

          <div class="chat-body">
            <!-- 全局态势感知仪表条 -->
            <div
              v-if="perceptionSnapshot"
              class="perception-banner"
              title="点击呼叫 Agent 深度解读当前宏观节律与持仓策略"
              @click="askMacroPerception"
            >
              <div class="perception-left">
                <span class="phase-tag">{{ perceptionSnapshot.Phase }}</span>
                <span
                  class="traffic-dot"
                  :style="{ backgroundColor: getTrafficColor(perceptionSnapshot.TrafficLight) }"
                />
                <span
                  class="traffic-text"
                  :style="{ color: getTrafficColor(perceptionSnapshot.TrafficLight) }"
                >
                  {{ getTrafficText(perceptionSnapshot.TrafficLight) }}
                </span>
                <span
                  v-if="perceptionSnapshot.MarketBreadth"
                  class="breadth-text"
                >
                  <span style="color:#d03050">涨{{ perceptionSnapshot.MarketBreadth.UpCount }}</span> / <span style="color:#18a058">跌{{ perceptionSnapshot.MarketBreadth.DownCount }}</span>
                </span>
              </div>
              <div class="perception-right">
                <NTag
                  v-if="perceptionSnapshot.PortfolioRisk && perceptionSnapshot.PortfolioRisk.RiskLevel !== 'SAFE'"
                  size="tiny"
                  type="error"
                  round
                >
                  ⚠️ {{ perceptionSnapshot.PortfolioRisk.RiskLevel === 'DANGER' ? '高危持仓' : '持仓警示' }}
                </NTag>
                <span class="ask-hint">态势推演 ➔</span>
              </div>
            </div>

            <!-- 当前聚焦个股智能感知与一键直达条 -->
            <div
              v-if="currentStock && currentStock.code"
              class="context-stock-banner"
            >
              <div class="context-stock-left">
                <span class="context-stock-pin">📌 当前关注:</span>
                <span class="context-stock-name">{{ currentStock.name || currentStock.code }}</span>
                <span class="context-stock-code">({{ currentStock.code }})</span>
              </div>
              <div class="context-stock-actions">
                <NButton
                  size="tiny"
                  secondary
                  type="primary"
                  round
                  :disabled="diagnosing"
                  @click="triggerDiagnosis(currentStock.code, currentStock.name, 'full')"
                >
                  🎯 三维体检
                </NButton>
                <NButton
                  size="tiny"
                  secondary
                  type="info"
                  round
                  :disabled="diagnosing"
                  @click="triggerDiagnosis(currentStock.code, currentStock.name, 'money')"
                >
                  💰 资金筹码
                </NButton>
                <NButton
                  size="tiny"
                  secondary
                  type="warning"
                  round
                  :disabled="diagnosing"
                  @click="triggerDiagnosis(currentStock.code, currentStock.name, 'vectorbt')"
                >
                  🧪 策略回测
                </NButton>
              </div>
            </div>
            <Transition name="hint-fade">
              <div
                v-if="hintVisible"
                class="hint-bar"
              >
                {{ hintText }}
              </div>
            </Transition>
            <div
              v-if="shareTipVisible"
              class="share-tip"
            >
              <div class="share-tip-text">
                {{ shareTipText }}
              </div>
              <NButton
                size="tiny"
                quaternary
                class="share-tip-close"
                @click="shareTipVisible = false"
              >
                关闭
              </NButton>
            </div>
            <NScrollbar
              ref="scrollbarRef"
              class="chat-scroll"
            >
              <div class="message-list">
                <div
                  v-for="(group, groupIndex) in messageGroups"
                  :key="group.id"
                  class="message-group"
                >
                  <div
                    class="message-group-header"
                    @click="toggleGroup(groupIndex)"
                  >
                    <div class="message-group-summary">
                      <NIcon
                        :component="isGroupExpanded(groupIndex) ? ChevronDownOutline : ChevronForwardOutline"
                        size="16"
                      />
                      <span class="message-group-title">{{ group.userMsg.content.slice(0, 50) }}{{ group.userMsg.content.length > 50 ? '...' : '' }}</span>
                      <span class="message-group-time">{{ group.userMsg.time }}</span>
                    </div>
                  </div>
                  <div
                    v-show="isGroupExpanded(groupIndex)"
                    class="message-group-content"
                  >
                    <div
                      :class="['message-item', group.userMsg.role]"
                    >
                      <div class="msg-avatar user-avatar">
                        <NIcon
                          :component="PersonCircleOutline"
                          size="18"
                        />
                      </div>
                      <div class="msg-bubble">
                        <div class="msg-content">
                          <div
                            v-if="group.userMsg.time"
                            class="msg-meta msg-meta-user-inner"
                          >
                            <span class="msg-time">{{ group.userMsg.time }}</span>
                          </div>
                          <MdPreview
                            v-if="group.userMsg.content"
                            :theme="theme"
                            :style="{ textAlign: 'right' }"
                            :model-value="group.userMsg.content"
                            :editor-id="'agent-msg-' + group.userIndex"
                            class="msg-markdown"
                          />
                        </div>
                      </div>
                    </div>
                    <div
                      v-if="group.assistantMsg"
                      :class="['message-item', 'assistant']"
                    >
                      <div class="msg-avatar assistant-avatar">
                        <NIcon
                          :component="SparklesOutline"
                          size="20"
                        />
                      </div>
                      <div class="msg-bubble">
                        <div class="msg-content">
                          <div
                            v-if="group.assistantMsg.statusText && isStreamLoad"
                            class="msg-status-text"
                          >
                            <NSpin size="small" /> <span>{{ group.assistantMsg.statusText }}</span>
                          </div>
                          <div
                            v-if="group.assistantMsg.steps && group.assistantMsg.steps.length > 0"
                            class="msg-steps-wrapper"
                          >
                            <div
                              class="msg-steps-header"
                              :class="{ 'steps-expanded': isStreamLoad }"
                              @click="toggleReasoning(group.assistantIndex)"
                            >
                              <NIcon
                                :component="reasoningExpandedMap[group.assistantIndex] || isStreamLoad ? ChevronDownOutline : ChevronForwardOutline"
                                size="14"
                              />
                              <span class="msg-steps-title">📋 执行步骤</span>
                              <span class="msg-steps-count">{{ group.assistantMsg.steps.length }}</span>
                            </div>
                            <div
                              v-show="reasoningExpandedMap[group.assistantIndex] || isStreamLoad"
                              class="msg-steps-content"
                            >
                              <div
                                v-for="(step, si) in group.assistantMsg.steps"
                                :key="si"
                                class="msg-step-item"
                              >
                                <div
                                  class="msg-step-dot"
                                  :class="getStepDotClass(step)"
                                />
                                <span class="msg-step-text">{{ step }}</span>
                              </div>
                            </div>
                          </div>
                          <div
                            v-if="group.assistantMsg.reasoning || (isStreamLoad && group.assistantMsg.rawReasoning)"
                            class="msg-reasoning-wrapper"
                          >
                            <div
                              class="msg-reasoning-header"
                              :class="{ 'reasoning-expanded': isStreamLoad }"
                              @click="toggleReasoning('r-' + group.assistantIndex)"
                            >
                              <NIcon
                                :component="reasoningExpandedMap['r-' + group.assistantIndex] || isStreamLoad ? ChevronDownOutline : ChevronForwardOutline"
                                size="14"
                              />
                              <span class="msg-reasoning-title">💭 思考过程</span>
                            </div>
                            <div
                              v-show="reasoningExpandedMap['r-' + group.assistantIndex] || isStreamLoad"
                              class="msg-reasoning-content"
                            >
                              <MdPreview
                                :theme="theme"
                                :style="{ textAlign: 'left' }"
                                :model-value="group.assistantMsg.reasoning || ''"
                                :editor-id="'agent-reasoning-' + group.assistantIndex"
                                class="msg-markdown"
                              />
                            </div>
                          </div>
                          <div
                            v-if="group.assistantMsg.jsonMarkdown"
                            class="msg-json-md-wrapper"
                          >
                            <div
                              class="msg-json-md-header"
                              @click="toggleReasoning('j-' + group.assistantIndex)"
                            >
                              <NIcon
                                :component="reasoningExpandedMap['j-' + group.assistantIndex] ? ChevronDownOutline : ChevronForwardOutline"
                                size="14"
                              />
                              <span class="msg-json-md-title">📊 分析报告</span>
                            </div>
                            <div
                              v-show="reasoningExpandedMap['j-' + group.assistantIndex]"
                              class="msg-json-md-content"
                            >
                              <MdPreview
                                :theme="theme"
                                :style="{ textAlign: 'left' }"
                                :model-value="group.assistantMsg.jsonMarkdown"
                                :editor-id="'agent-json-md-' + group.assistantIndex"
                                class="msg-markdown"
                                @on-html-changed="onMdHtmlChanged"
                              />
                            </div>
                          </div>
                          <MdPreview
                            :theme="theme"
                            :style="{ textAlign: 'left' }"
                            :model-value="group.assistantMsg.content || '...'"
                            :editor-id="'agent-msg-' + group.assistantIndex"
                            class="msg-markdown"
                            @on-html-changed="onMdHtmlChanged"
                          />
                          <!-- 伴随全局联动控制动作卡片 -->
                          <div
                            v-if="group.assistantMsg.actions && group.assistantMsg.actions.length > 0"
                            class="agent-action-cards"
                          >
                            <div
                              v-for="(act, actIdx) in group.assistantMsg.actions"
                              :key="actIdx"
                              class="agent-action-card"
                            >
                              <div class="action-card-info">
                                <span class="action-icon">⚡</span>
                                <span class="action-title">{{ getActionDesc(act) }}</span>
                              </div>
                              <NButton
                                size="tiny"
                                type="primary"
                                secondary
                                @click="executeAction(act)"
                              >
                                立即联动
                              </NButton>
                            </div>
                          </div>
                          <!-- 性能与可观测性监控胶囊 -->
                          <div
                            v-if="group.assistantMsg.metrics"
                            class="agent-metrics-capsule"
                          >
                            <span
                              class="metrics-tag"
                              title="端到端总耗时"
                            >⏱️ {{ formatDuration(group.assistantMsg.metrics.total_duration) }}</span>
                            <span
                              v-if="group.assistantMsg.metrics.first_chunk_latency"
                              class="metrics-tag"
                              title="模型首包响应延迟"
                            >⚡ 首包: {{ formatDuration(group.assistantMsg.metrics.first_chunk_latency) }}</span>
                            <span
                              v-if="group.assistantMsg.metrics.tool_call_count > 0"
                              class="metrics-tag"
                              title="执行工具调用次数"
                            >🔧 {{ group.assistantMsg.metrics.tool_call_count }}次工具</span>
                            <span
                              v-if="group.assistantMsg.metrics.retry_count > 0"
                              class="metrics-tag metrics-warning"
                              title="遭遇瞬时网络/限流故障后自动恢复"
                            >🔁 重试{{ group.assistantMsg.metrics.retry_count }}次成功</span>
                          </div>
                          <div
                            v-if="isStreamLoad && groupIndex === messageGroups.length - 1 && !group.assistantMsg.content"
                            class="msg-loading"
                          >
                            <NSpin size="small" />
                            <span>思考中...</span>
                          </div>
                          <div class="msg-bubble-actions">
                            <div
                              v-if="group.assistantMsg.modelName || group.assistantMsg.time"
                              class="msg-meta-row-assistant"
                            >
                              <span
                                v-if="group.assistantMsg.modelName"
                                class="msg-model-name"
                                :title="group.assistantMsg.modelName"
                              >{{ group.assistantMsg.modelName }}</span>
                              <span
                                v-if="group.assistantMsg.time"
                                class="msg-time"
                              >{{ group.assistantMsg.time }}</span>
                            </div>
                            <NButton
                              quaternary
                              size="tiny"
                              class="msg-toggle-btn"
                              @click="toggleGroup(groupIndex)"
                            >
                              <template #icon>
                                <NIcon :component="isGroupExpanded(groupIndex) ? ChevronUpOutline : ChevronDownOutline" />
                              </template>
                              {{ isGroupExpanded(groupIndex) ? '收起' : '展开' }}
                            </NButton>
                            <NButton
                              quaternary
                              size="tiny"
                              class="msg-copy-btn"
                              @click="copyAiContent(group.assistantMsg)"
                            >
                              <template #icon>
                                <NIcon :component="CopyOutline" />
                              </template>
                              复制
                            </NButton>
                            <NButton
                              quaternary
                              size="tiny"
                              class="msg-export-img-btn"
                              :loading="exportImageKey === String(group.assistantIndex)"
                              title="导出为图片"
                              @click="exportAiReplyImage(group.assistantIndex, $event)"
                            >
                              <template #icon>
                                <NIcon :component="ImageOutline" />
                              </template>
                              导出图
                            </NButton>
                            <NButton
                              quaternary
                              size="tiny"
                              class="msg-share-btn"
                              :loading="shareLoading"
                              @click="shareAiContent(group.assistantMsg)"
                            >
                              <template #icon>
                                <NIcon :component="ShareSocialOutline" />
                              </template>
                              分享
                            </NButton>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </NScrollbar>
          </div>

          <div class="chat-footer">
            <div class="chat-footer-row">
              <NSelect
                v-model:value="aiConfigId"
                :options="aiConfigOptions"
                size="small"
                filterable
                to="body"
                placement="top-start"
                placeholder="选择模型"
                :consistent-menu-width="false"
                :menu-props="{ style: { zIndex: 10002 } }"
                class="chat-footer-select"
              />
              <div class="chat-footer-thinking">
                <span class="chat-footer-thinking-label">思考模式</span>
                <NSwitch
                  v-model:value="thinkingMode"
                  size="small"
                />
              </div>
              <div class="chat-footer-memory">
                <span class="chat-footer-thinking-label">记忆模式</span>
                <NSwitch
                  v-model:value="memoryMode"
                  size="small"
                />
                <NSelect
                  v-if="memoryMode"
                  v-model:value="memoryCount"
                  :options="memoryCountOptions"
                  size="small"
                  :consistent-menu-width="false"
                  to="body"
                  placement="top-start"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-memory-count"
                />
              </div>
              <div class="chat-footer-agent-mode">
                <NSelect
                  v-model:value="agentMode"
                  :options="agentModeOptions"
                  size="small"
                  to="body"
                  placement="top-start"
                  placeholder="Agent模式"
                  :consistent-menu-width="false"
                  :menu-props="{ style: { zIndex: 10002 } }"
                  class="chat-footer-agent-mode-select"
                />
              </div>
            </div>
            <div class="chat-quick-chips">
              <NTag
                size="small"
                round
                :bordered="false"
                class="quick-chip"
                @click="handleQuickChip('三维量化诊断')"
              >
                🎯 量化诊断
              </NTag>
              <NTag
                size="small"
                round
                :bordered="false"
                class="quick-chip"
                @click="handleQuickChip('VectorBT 策略回测: ')"
              >
                🧪 策略回测
              </NTag>
              <NTag
                size="small"
                round
                :bordered="false"
                class="quick-chip"
                @click="handleQuickChip('主力资金博弈与筹码集中度分析: ')"
              >
                💰 资金筹码
              </NTag>
              <NTag
                size="small"
                round
                :bordered="false"
                type="info"
                class="quick-chip"
                @click="triggerMarketIntelligence('pre_market_briefing')"
              >
                🌅 盘前前瞻
              </NTag>
              <NTag
                size="small"
                round
                :bordered="false"
                type="warning"
                class="quick-chip"
                @click="triggerMarketIntelligence('post_market_review')"
              >
                🌆 盘后复盘
              </NTag>
              <NTag
                size="small"
                round
                :bordered="false"
                type="success"
                class="quick-chip"
                @click="triggerMarketIntelligence('etf_rotation_decision')"
              >
                📈 ETF轮动
              </NTag>
              <NTag
                size="small"
                round
                :bordered="false"
                type="error"
                class="quick-chip"
                @click="triggerMarketIntelligence('serenity_industry_analysis')"
              >
                🔍 产业链拆解
              </NTag>
            </div>
            <div class="chat-footer-input">
              <NInput
                v-model:value="inputValue"
                type="textarea"
                placeholder="输入消息，回车发送..."
                :autosize="{ minRows: 2, maxRows: 4 }"
                :disabled="isStreamLoad"
                @keydown.enter.exact.prevent="sendMessage"
              />
              <NButton
                v-if="isStreamLoad"
                type="warning"
                quaternary
                class="chat-footer-abort"
                @click="abortStream(true)"
              >
                中断
              </NButton>
              <NButton
                type="primary"
                :loading="isStreamLoad"
                :disabled="isStreamLoad || !canSend"
                @click="sendMessage"
              >
                发送
              </NButton>
            </div>
          </div>
        </NCard>
      </div>
    </div>
  </Transition>
</template>

<script setup>
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount, onBeforeMount } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NIcon, NInput, NScrollbar, NSelect, NSpin, NSwitch, NTag, useMessage } from 'naive-ui'
import {
  CloseOutline,
  SparklesOutline,
  PersonCircleOutline,
  CopyOutline,
  ShareSocialOutline,
  ImageOutline,
  ChevronDownOutline,
  ChevronForwardOutline,
  ChevronUpOutline
} from '@vicons/ionicons5'
import {
  ChatWithAgent,
  GetAiConfigs,
  GetConfig,
  SaveAiAssistantSession,
  GetAiAssistantSession,
  ShareText,
  AbortChatWithAgent,
  SaveAIResponseResult,
  SaveImage
} from '../../wailsjs/go/main/App'
import { EventsOff, EventsOn, EventsEmit } from '../../wailsjs/runtime'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'
import html2canvas from 'html2canvas'

const emit = defineEmits(['open-market-intelligence'])

function triggerMarketIntelligence(type) {
  emit('open-market-intelligence', type)
  try {
    EventsEmit('show-market-intelligence', type)
  } catch (e) {}
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('show-market-intelligence', { detail: type }))
  }
}
import { cleanFinalContent } from '../utils/cleanText'
import { dispatchAgentAction } from '../utils/agentControlBus'
import { currentStock } from '../utils/stockContextBus'

const STORAGE_KEY_MODEL_ID = 'blades-stock-agent-last-model-id'

const route = useRoute()
const message = useMessage()

const showButton = computed(() => route.name !== 'agent')

const panelVisible = ref(false)
const inputValue = ref('')
const isStreamLoad = ref(false)
const sentFromFloating = ref(false)
const messages = ref([])
let formatTimer = null
const sessionId = ref('')
const aiConfigOptions = ref([])
const aiConfigId = ref(null)
const perceptionSnapshot = ref(null)

async function refreshPerceptionSnapshot() {
  try {
    const api = window.go?.main?.App?.GetSystemStateSnapshot
    if (api) {
      const snap = await api()
      if (snap) {
        perceptionSnapshot.value = snap
      }
    }
  } catch (e) {
    console.warn('refreshPerceptionSnapshot failed:', e)
  }
}

function askMacroPerception() {
  if (!perceptionSnapshot.value) return
  const phase = perceptionSnapshot.value.Phase || ''
  const traffic = perceptionSnapshot.value.TrafficLight || ''
  inputValue.value = `请结合当前市场【${phase}】节律与情绪【${traffic}】状态，对全天大局观、主线机会和实盘持仓风控进行综合推演。`
  sendMessage()
}

function getTrafficColor(light) {
  if (light === 'GREEN') return '#18a058'
  if (light === 'YELLOW') return '#f0a020'
  if (light === 'RED') return '#d03050'
  return '#909399'
}

function getTrafficText(light) {
  if (light === 'GREEN') return '主升进攻'
  if (light === 'YELLOW') return '震荡混沌'
  if (light === 'RED') return '退潮防守'
  return '中性'
}

function getActionDesc(act) {
  if (!act) return '系统联动'
  switch (act.type) {
    case 'OPEN_DIAGNOSIS':
      return `调起 3D 量化诊断：${act.payload?.name || act.payload?.code}`
    case 'SHOW_KLINE':
      return `查看 K 线走势：${act.payload?.name || act.payload?.code}`
    case 'NAVIGATE_VIEW':
      return `页面跳转：${act.payload?.path}`
    case 'OPEN_MARKET_INTELLIGENCE':
      return '打开市场情报中心'
    case 'DISCIPLINE_ALERT':
      return `风控警示：${act.payload?.title || ''}`
    default:
      return act.type
  }
}

function executeAction(act) {
  dispatchAgentAction(act)
  message.success('已触发系统联动！')
}

function modelLabelForConfig(configId) {
  const opts = aiConfigOptions.value
  if (!opts?.length) return ''
  const id = configId != null ? Number(configId) : Number(opts[0].value)
  const found = opts.find(o => Number(o.value) === id)
  return found?.label != null ? String(found.label) : ''
}

const thinkingMode = ref(true)
const memoryMode = ref(false)
const memoryCount = ref(1)
const memoryCountOptions = [
  { label: '1 条', value: 1 },
  { label: '2 条', value: 2 },
  { label: '3 条', value: 3 },
  { label: '4 条', value: 4 },
  { label: '5 条', value: 5 },
  { label: '10 条', value: 10 },
]
const agentMode = ref('auto')
const agentModeOptions = [
  { label: '🤖 自动选择', value: 'auto' },
  { label: '⚡ 快速模式', value: 'react' },
  { label: '🧠 规划模式', value: 'plan_execute' },
]

watch(agentMode, (val) => {
  if (val === 'react') showHint('⚡ 快速模式推荐使用DeepSeek最新版')
  else if (val === 'plan_execute') showHint('🧠 规划模式推荐使用GLM最新版')
})

watch(aiConfigId, (val) => {
  const label = modelLabelForConfig(val).toLowerCase()
  const labelCompact = label.replace(/[\s_-]/g, '')
  if (label.includes('deepseek-chat')) {
    agentMode.value = 'plan_execute'
    thinkingMode.value = false
    showHint('deepseek-chat 已使用规划模式并关闭思考模式')
  } else if (label.includes('deepseek')) {
    showHint('⚡ DeepSeek模型推荐使用快速模式')
  } else if (labelCompact.includes('glm5.1')) {
    agentMode.value = 'plan_execute'
    thinkingMode.value = true
    showHint('GLM 5.1 已使用规划模式并开启思考模式')
  } else if (label.includes('glm')) {
    showHint('🧠 GLM模型推荐使用规划模式')
  }
})


const canSend = computed(() => !!inputValue.value.trim())
const scrollbarRef = ref(null)
const darkTheme = ref(false)
const shareLoading = ref(false)
const exportImageKey = ref('')
const shareTipVisible = ref(false)
const shareTipText = ref('')
const hintVisible = ref(false)
const hintText = ref('')
let hintTimer = null

function showHint(text) {
  hintText.value = text
  hintVisible.value = true
  if (hintTimer) clearTimeout(hintTimer)
  hintTimer = setTimeout(() => { hintVisible.value = false }, 3000)
}
const isAborted = ref(false)
const expandedGroups = ref(new Set())
const reasoningExpandedMap = ref({})

const hasBackgroundTask = computed(() => isStreamLoad.value && sentFromFloating.value && !panelVisible.value)
const AGENT_EVENT = 'agent-message'

const messageGroups = computed(() => {
  const groups = []
  let currentGroup = null
  
  for (let i = 0; i < messages.value.length; i++) {
    const msg = messages.value[i]
    if (msg.role === 'user') {
      if (currentGroup) {
        groups.push(currentGroup)
      }
      currentGroup = {
        id: i,
        userMsg: msg,
        userIndex: i,
        assistantMsg: null,
        assistantIndex: -1
      }
    } else if (msg.role === 'assistant' && currentGroup) {
      currentGroup.assistantMsg = msg
      currentGroup.assistantIndex = i
    }
  }
  if (currentGroup) {
    groups.push(currentGroup)
  }
  return groups
})

function isGroupExpanded(groupIndex) {
  return expandedGroups.value.has(groupIndex)
}

function toggleGroup(groupIndex) {
  const newSet = new Set(expandedGroups.value)
  if (newSet.has(groupIndex)) {
    newSet.delete(groupIndex)
  } else {
    newSet.add(groupIndex)
  }
  expandedGroups.value = newSet
}

function initDefaultExpanded() {
  if (messageGroups.value.length > 0 && expandedGroups.value.size === 0) {
    expandedGroups.value = new Set([messageGroups.value.length - 1])
  }
}

function ensureLatestGroupExpanded() {
  if (messageGroups.value.length > 0) {
    const lastIndex = messageGroups.value.length - 1
    const newSet = new Set(expandedGroups.value)
    newSet.add(lastIndex)
    expandedGroups.value = newSet
  }
}

function toggleReasoning(index) {
  reasoningExpandedMap.value = {
    ...reasoningExpandedMap.value,
    [index]: !reasoningExpandedMap.value[index]
  }
}

function getStepDotClass(step) {
  if (step.includes('✅')) return 'step-done'
  if (step.includes('🔧')) return 'step-tool'
  if (step.includes('⚡') || step.includes('🧠') || step.includes('📋') || step.includes('🔄')) return 'step-active'
  return ''
}

function onMdHtmlChanged() {
  nextTick(() => {
    document.querySelectorAll('.msg-markdown .md-editor-code-block').forEach(block => {
      if (block.querySelector('.code-collapse-btn')) return
      const codeEl = block.querySelector('code')
      if (!codeEl) return
      const lang = (codeEl.className || '').toLowerCase()
      const isJson = lang.includes('json') || lang.includes('language-json')
      const text = codeEl.textContent || ''
      const lineCount = text.split('\n').length
      if (!isJson && lineCount <= 8) return

      block.classList.add('code-collapsed')
      const btn = document.createElement('span')
      btn.className = 'code-collapse-btn'
      btn.textContent = '展开'
      btn.addEventListener('click', (e) => {
        e.stopPropagation()
        const collapsed = block.classList.toggle('code-collapsed')
        btn.textContent = collapsed ? '展开' : '收起'
      })
      block.appendChild(btn)
    })
  })
}

async function copyAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    message.warning('暂无可复制的 AI 正文内容')
    return
  }
  try {
    if (navigator && navigator.clipboard && navigator.clipboard.writeText) {
      await navigator.clipboard.writeText(text)
      message.success('已复制 AI 回答内容')
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      message.success('已复制 AI 回答内容')
    }
  } catch (e) {
    message.error('复制失败，请手动选择文本')
  }
}

function shareTextToCommunity(text, title) {
  if (shareLoading.value) return
  shareLoading.value = true
  shareTipText.value = '正在分享到社区...'
  shareTipVisible.value = true
  ShareText(text, title)
    .then((msg) => {
      shareTipText.value = msg
      shareTipVisible.value = true
    })
    .catch((err) => {
      shareTipText.value = '分享失败: ' + (err?.message ?? err)
      shareTipVisible.value = true
    })
    .finally(() => {
      shareLoading.value = false
    })
}

function shareAiContent(msg) {
  const text = (msg?.content ?? '').trim()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 正文内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'Blades Stock AI Agent助手')
}

function getLastAssistantContent() {
  for (let i = messages.value.length - 1; i >= 0; i--) {
    const m = messages.value[i]
    if (m?.role === 'assistant') {
      const text = (m?.content ?? '').trim()
      if (text) return text
    }
  }
  return ''
}

function shareAiToCommunity() {
  const text = getLastAssistantContent()
  if (!text) {
    shareTipText.value = '暂无可分享的 AI 回复内容'
    shareTipVisible.value = true
    return
  }
  shareTextToCommunity(text, 'Blades Stock AI Agent助手')
}

async function exportAiReplyImage(assistantIndex, evt) {
  const msg = messages.value[assistantIndex]
  if (msg?.role !== 'assistant') return
  if (!(msg.content ?? '').trim()) {
    shareTipText.value = '暂无可导出的 AI 回答内容'
    shareTipVisible.value = true
    return
  }
  const editorId = 'agent-msg-' + assistantIndex
  const bubble = evt?.currentTarget?.closest?.('.msg-bubble')
  const key = String(assistantIndex)
  if (exportImageKey.value) return
  exportImageKey.value = key
  await nextTick()
  try {
    const target = document.getElementById(`${editorId}-preview-wrapper`) ||
      document.getElementById(`${editorId}-preview`) ||
      bubble?.querySelector('.md-editor-preview') ||
      null
    if (!target) {
      shareTipText.value = '未找到预览区域，请展开回答后重试'
      shareTipVisible.value = true
      return
    }
    const savedStyles = []
    const overflowParents = []
    let el = target.parentElement
    while (el && el !== document.body) {
      const style = getComputedStyle(el)
      if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
        savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
        overflowParents.push(el)
        el.style.overflow = 'visible'
        el.style.overflowY = 'visible'
        el.style.height = 'auto'
        el.style.maxHeight = 'none'
      }
      el = el.parentElement
    }
    const savedTargetStyle = { height: target.style.height, maxHeight: target.style.maxHeight, overflow: target.style.overflow, overflowY: target.style.overflowY }
    target.style.height = 'auto'
    target.style.maxHeight = 'none'
    target.style.overflow = 'visible'
    target.style.overflowY = 'visible'
    await nextTick()
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: darkTheme.value ? '#1e1e1e' : '#ffffff'
    })
    target.style.height = savedTargetStyle.height
    target.style.maxHeight = savedTargetStyle.maxHeight
    target.style.overflow = savedTargetStyle.overflow
    target.style.overflowY = savedTargetStyle.overflowY
    savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
      el.style.overflow = overflow
      el.style.overflowY = overflowY
      el.style.height = height
      el.style.maxHeight = maxHeight
    })
    const dataUrl = canvas.toDataURL('image/png')
    const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')
    const result = await SaveImage(`blades-stock-agent-${safeTime}`, base64)
    if (result && !result.includes('异常') && !result.includes('无法')) {
      shareTipText.value = '已导出为 PNG 图片：' + result
    } else {
      shareTipText.value = result || '导出取消'
    }
    shareTipVisible.value = true
  } catch (e) {
    shareTipText.value = '导出图片失败: ' + (e?.message ?? e)
    shareTipVisible.value = true
  } finally {
    exportImageKey.value = ''
  }
}

function abortStream(showTip = true) {
  if (!isStreamLoad.value) return
  isAborted.value = true
  isStreamLoad.value = false
  stopFormatTimer()
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      if (last.rawContent) {
        last.rawContent = cleanFinalContent(last.rawContent)
        const fmt = formatMarkdown(last.rawContent)
        last.content = fmt.content
        if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
      }
    if (last.rawReasoning) {
      const fmt = formatMarkdown(last.rawReasoning)
      last.reasoning = fmt.content
    }
  }
  if (showTip) {
    shareTipText.value = '已中断本次 AI 回答'
    shareTipVisible.value = true
  }
  AbortChatWithAgent()
}

const theme = computed(() => (darkTheme.value ? 'dark' : 'light'))

async function loadHistory() {
  try {
    const resp = await GetAiAssistantSession('')
    if (resp?.sessionId) {
      sessionId.value = resp.sessionId
    }
    const list = resp?.messages
    if (Array.isArray(list) && list.length > 0) {
      messages.value = list.map(m => ({
        role: m.role ?? '',
        content: m.content ?? '',
        rawContent: m.rawContent ?? '',
        time: m.time ?? '',
        modelName: m.modelName ?? '',
        reasoning: m.reasoning ?? '',
        rawReasoning: m.rawReasoning ?? '',
        steps: m.steps ?? [],
        statusText: '',
        jsonMarkdown: m.jsonMarkdown ?? ''
      }))
      nextTick(() => {
        initDefaultExpanded()
      })
    }
  } catch (_) {
  }
}

function saveHistory() {
  if (messages.value.length === 0) return
  const list = messages.value.map(m => ({
    role: m.role,
    content: m.content,
    time: m.time ?? '',
    modelName: m.modelName ?? '',
    reasoning: m.reasoning ?? '',
    steps: m.steps ?? [],
    jsonMarkdown: m.jsonMarkdown ?? ''
  }))
  SaveAiAssistantSession(sessionId.value, list).catch(() => {})
}

function openPanel() {
  panelVisible.value = true
  refreshPerceptionSnapshot()
  if (!sessionId.value) {
    sessionId.value = Date.now().toString()
  }
  if (messages.value.length === 0) {
    messages.value = [
      {
        role: 'assistant',
        content: '我是 Blades Stock AI Agent 助手，可以帮您分析股票、查询市场数据、获取研究报告等。请问有什么可以帮您的？',
        time: new Date().toLocaleString(),
        modelName: '',
        reasoning: ''
      }
    ]
  }
  nextTick(() => {
    initDefaultExpanded()
    scrollToBottom()
  })
}

function closePanel() {
  panelVisible.value = false
}

async function togglePanel() {
  if (!panelVisible.value) {
    openPanel()
  } else {
    closePanel()
  }
}

function scrollToBottom() {
  nextTick(() => {
    scrollbarRef.value?.scrollTo({ top: 99999, behavior: 'smooth' })
  })
}

function fillQuickPrompt(prefix) {
  if (inputValue.value && !inputValue.value.startsWith(prefix)) {
    inputValue.value = prefix + inputValue.value
  } else if (!inputValue.value) {
    inputValue.value = prefix
  }
}

function sendDirectPrompt(promptText) {
  if (isStreamLoad.value) {
    message.warning('Agent 正在思考分析中，请稍候...')
    return
  }
  inputValue.value = promptText
  sendMessage()
}

function handleQuickChip(prefix) {
  if (currentStock.value?.code) {
    const stockName = currentStock.value.name ? `${currentStock.value.name} (${currentStock.value.code})` : currentStock.value.code
    sendDirectPrompt(`${prefix} ${stockName}`)
  } else {
    fillQuickPrompt(prefix)
  }
}

function sendMessage() {
  if (isStreamLoad.value) {
    abortStream(false)
  }
  const text = inputValue.value.trim()
  if (!text) {
    message.warning('请输入你的问题')
    return
  }

  messages.value.push({
    role: 'user',
    content: text,
    time: new Date().toLocaleString(),
    modelName: '',
    reasoning: '',
    steps: []
  })
  const configId = aiConfigId.value ?? aiConfigOptions.value[0]?.value ?? 0
  const modelName = modelLabelForConfig(configId)
  messages.value.push({
    role: 'assistant',
    content: '',
    rawContent: '',
    time: new Date().toLocaleString(),
    modelName,
    reasoning: '',
    rawReasoning: '',
    steps: [],
    statusText: '',
    jsonMarkdown: ''
  })
  inputValue.value = ''
  isStreamLoad.value = true
  isAborted.value = false
  sentFromFloating.value = true
  startFormatTimer()
  saveHistory()
  nextTick(() => {
    ensureLatestGroupExpanded()
    const lastGroup = messageGroups.value[messageGroups.value.length - 1]
    if (lastGroup) {
      reasoningExpandedMap.value = {
        ...reasoningExpandedMap.value,
        [lastGroup.assistantIndex]: true,
        ['j-' + lastGroup.assistantIndex]: true
      }
    }
    scrollToBottom()
  })
  ChatWithAgent(text, configId, null, memoryMode.value, memoryCount.value, thinkingMode.value, agentMode.value === 'auto' ? '' : agentMode.value, sessionId.value || "")
}

function startNewChat() {
  if (isStreamLoad.value) {
    message.warning('当前有回答正在生成，请先中断或等待完成')
    return
  }
  messages.value = []
  sessionId.value = Date.now().toString()
}

function startFormatTimer() {
  stopFormatTimer()
  formatTimer = setInterval(() => {
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      if (last.rawContent) {
        const fmt = formatMarkdown(last.rawContent)
        last.content = fmt.content
        if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
        if (fmt.actions && fmt.actions.length > 0) {
          last.actions = fmt.actions
        }
      }
      if (last.rawReasoning) {
        const fmt = formatMarkdown(last.rawReasoning)
        last.reasoning = fmt.content
      }
    }
  }, 1500)
}

function stopFormatTimer() {
  if (formatTimer) {
    clearInterval(formatTimer)
    formatTimer = null
  }
}

function extractAgentActions(content) {
  if (!content) return { content: '', actions: [] }
  const actions = []
  const regex = /```agent-action\s*([\s\S]*?)```/g
  const cleaned = content.replace(regex, (_, jsonStr) => {
    try {
      const obj = JSON.parse(jsonStr.trim())
      if (obj && obj.type) {
        actions.push(obj)
      }
    } catch (e) {}
    return ''
  })
  return { content: cleaned.trim(), actions }
}

function formatMarkdown(content) {
  if (!content) return { content: '', jsonMarkdown: '', actions: [] }

  const { content: actionCleaned, actions } = extractAgentActions(content)
  const { content: cleaned, jsonMarkdown } = extractJsonMarkdown(actionCleaned)

  let inCodeBlock = false
  const lines = cleaned.split('\n')
  const result = []

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
    jsonMarkdown,
    actions
  }
}

function hasMarkdownContent(str) {
  if (!str || typeof str !== 'string') return false
  return /(^|\n)\s*#{1,6}\s/.test(str) ||
    /(^|\n)\s*\|/.test(str) ||
    /(^|\n)\s*---/.test(str) ||
    /(^|\n)\s*[-*+]\s/.test(str) ||
    /(^|\n)\s*>\s/.test(str) ||
    /(^|\n)\s*```/.test(str)
}

function extractMarkdownFromJson(obj) {
  if (typeof obj === 'string') return obj
  if (Array.isArray(obj)) {
    const items = obj.map(item => typeof item === 'string' ? item : JSON.stringify(item, null, 2))
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

function extractJsonMarkdown(content) {
  if (!content) return { content: '', jsonMarkdown: '' }
  const cleaned = []
  const jsonParts = []
  let i = 0
  const len = content.length
  let inCodeBlock = false

  while (i < len) {
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

  return {
    content: cleaned.join(''),
    jsonMarkdown: jsonParts.join('\n\n---\n\n')
  }
}

function findJsonEnd(content, start) {
  let depth = 0
  let bracketDepth = 0
  let inStr = false
  let escape = false
  for (let i = start; i < content.length; i++) {
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

function splitInlineHeading(line) {
  const match = line.match(/(#{1,6}\s+\S)/)
  if (!match) return line
  const idx = match.index
  if (idx === 0) return line
  const prefix = line.substring(0, idx)
  if (prefix.trim() === '') return line
  return prefix + '\n\n' + line.substring(idx)
}

function isBlockElement(line) {
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

function parseStepText(text) {
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

function onAgentMessage(msg) {
  if (isAborted.value) return

  if (msg.content === 'agent-DONE' || (msg?.response_meta?.finish_reason === 'stop')) {
    isStreamLoad.value = false
    sentFromFloating.value = false
    isAborted.value = false
    stopFormatTimer()
    const last = messages.value[messages.value.length - 1]
    if (last && last.role === 'assistant') {
      if (msg.metrics) {
        last.metrics = msg.metrics
      }
      if (last.rawContent) {
        const fmt = formatMarkdown(last.rawContent)
        last.content = fmt.content
        if (fmt.jsonMarkdown) last.jsonMarkdown = fmt.jsonMarkdown
        if (fmt.actions && fmt.actions.length > 0) {
          last.actions = fmt.actions
        }
      }
      if (last.rawReasoning) {
        const fmt = formatMarkdown(last.rawReasoning)
        last.reasoning = fmt.content
      }
    }
    saveHistory()
    nextTick(scrollToBottom)
    if (msg.content === 'agent-DONE' && last && last.role === 'assistant' && last.content) {
      const user = messages.value[messages.value.length - 2]
      let stockCode = "agent"
      let stockName = "市场分析"
      if (user && user.content) {
        const m = user.content.match(/(?:sh|sz|bj)?([0-9]{6})/i)
        if (m) {
          stockCode = m[1]
          stockName = ""
        }
      }
      SaveAIResponseResult(stockCode, stockName, last.content, sessionId.value, user?.content || "", aiConfigId.value)
    }
    return
  }

  const roleLower = String(msg?.role || '').toLowerCase()
  if (roleLower !== 'assistant') {
    return
  }

  const last = messages.value[messages.value.length - 1]
  if (last && last.role === 'assistant') {
    if (msg?.reasoning_content) {
      const rc = msg.reasoning_content
      if (rc.startsWith('[STATUS]')) {
        last.statusText = rc.replace(/^\[STATUS\]/, '').trim()
      } else if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!last.steps) last.steps = []
          const parsed = parseStepText(stepText)
          last.steps.push(...parsed)
        }
      } else {
        last.rawReasoning = (last.rawReasoning || '') + rc
        last.reasoning = last.rawReasoning
      }
    }
    if (msg?.content) {
      last.rawContent = (last.rawContent || '') + msg.content
      last.content = last.rawContent
    }
    nextTick(scrollToBottom)
  }
}

watch(panelVisible, (v) => {
  if (v) {
    nextTick(scrollToBottom)
  }
})

onBeforeMount(() => {
  GetConfig().then(result => {
    darkTheme.value = result.darkTheme
  })
})

function formatDuration(ns) {
  if (!ns && ns !== 0) return ''
  const ms = ns / 1000000
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${(ms / 1000).toFixed(1)}s`
}

onMounted(() => {
  EventsOn(AGENT_EVENT, onAgentMessage)
  EventsOn('openFloatingAgentWithQuestion', (payload) => {
    panelVisible.value = true
    const text = typeof payload === 'string' ? payload : (payload?.question || '')
    if (text) {
      inputValue.value = text
      setTimeout(() => {
        if (!isStreamLoad.value && inputValue.value.trim()) {
          sendMessage()
        }
      }, 150)
    }
  })
  loadHistory()
  GetAiConfigs().then(res => {
    const list = Array.isArray(res) ? res : []
    aiConfigOptions.value = list.map((c, index) => {
      const id = c.ID != null ? Number(c.ID) : (c.id != null ? Number(c.id) : index)
      const name = c.name ?? c.Name ?? ''
      const modelName = c.modelName ?? c.ModelName ?? ''
      return {
        label: name + (modelName ? ' [' + modelName + ']' : ''),
        value: id
      }
    })
    if (aiConfigOptions.value.length) {
      const lastModelId = localStorage.getItem(STORAGE_KEY_MODEL_ID)
      if (lastModelId) {
        const foundId = Number(lastModelId)
        const isValid = aiConfigOptions.value.some(opt => opt.value === foundId)
        aiConfigId.value = isValid ? foundId : aiConfigOptions.value[0].value
      } else {
        aiConfigId.value = aiConfigOptions.value[0].value
      }
    }
  })
})

watch(aiConfigId, (newId) => {
  if (newId != null) {
    localStorage.setItem(STORAGE_KEY_MODEL_ID, String(newId))
  }
})

onBeforeUnmount(() => {
  EventsOff(AGENT_EVENT)
  EventsOff('openFloatingAgentWithQuestion')
})
</script>

<style scoped>
.edge-trigger {
  position: fixed;
  top: 50%;
  right: 0;
  z-index: 9998;
  transform: translateY(-50%);
  width: 32px;
  height: 120px;
  border-radius: 12px 0 0 12px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: -2px 0 12px rgba(102, 126, 234, 0.4);
  transition: width 0.2s ease, box-shadow 0.2s ease;
}
.edge-trigger-busy {
  box-shadow: -4px 0 18px rgba(248, 113, 113, 0.8);
}
.edge-trigger:hover {
  width: 40px;
  box-shadow: -4px 0 16px rgba(102, 126, 234, 0.5);
}
.edge-trigger-inner {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
}
.edge-trigger-text {
  font-size: 14px;
  writing-mode: vertical-rl;
  letter-spacing: 2px;
  line-height: 1;
  white-space: nowrap;
}
.edge-trigger-badge {
  position: absolute;
  top: 6px;
  left: 6px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f97316;
  box-shadow: 0 0 6px rgba(248, 113, 113, 0.9);
  animation: pulse 1.5s infinite;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.drawer-wrap {
  position: fixed;
  inset: 0;
  z-index: 9999;
  pointer-events: none;
}
.drawer-wrap > * {
  pointer-events: auto;
}
.drawer-mask {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.35);
  cursor: pointer;
}
.drawer-panel {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 60vw;
  min-width: 320px;
  max-width: calc(100vw - 48px);
  background: var(--n-color-modal);
  box-shadow: -8px 0 24px rgba(0, 0, 0, 0.15);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.panel-card {
  height: 100%;
  border-radius: 0;
  box-shadow: none;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.panel-card :deep(.n-card-header) {
  padding: 12px 16px;
  flex-shrink: 0;
}
.panel-card :deep(.n-card__content) {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}
.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.panel-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}
.panel-title {
  font-weight: 600;
  font-size: 16px;
}

.chat-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
  position: relative;
}
.hint-bar {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 8px 14px;
  border-radius: 8px;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(118, 75, 162, 0.12) 100%);
  border: 1px solid rgba(102, 126, 234, 0.25);
  font-size: 13px;
  color: var(--n-text-color-2);
  text-align: center;
  line-height: 1.5;
}
.hint-fade-enter-active,
.hint-fade-leave-active {
  transition: opacity 0.3s, transform 0.3s;
}
.hint-fade-enter-from,
.hint-fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
.share-tip {
  flex-shrink: 0;
  margin: 10px 16px 0;
  padding: 10px 12px;
  border-radius: 10px;
  background: rgba(0, 0, 0, 0.04);
  border: 1px solid var(--n-border-color);
  display: flex;
  gap: 10px;
  align-items: flex-start;
}
.share-tip-text {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
  text-align: left;
}
.share-tip-close {
  flex-shrink: 0;
}
.chat-scroll {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}
.chat-scroll :deep(.n-scrollbar-content) {
  min-height: 0;
}
.message-list {
  padding: 12px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.message-group {
  border: 1px solid var(--n-border-color);
  border-radius: 12px;
  overflow: hidden;
  background: var(--n-color-modal);
}
.message-group-header {
  padding: 10px 14px;
  cursor: pointer;
  background: rgba(0, 0, 0, 0.02);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.message-group-header:hover {
  background: rgba(0, 0, 0, 0.04);
}
.message-group-summary {
  display: flex;
  align-items: center;
  gap: 8px;
}
.message-group-title {
  flex: 1;
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.message-group-time {
  font-size: 11px;
  color: var(--n-text-color-3);
  flex-shrink: 0;
}
.message-group-content {
  padding: 12px 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.message-group-content .message-item {
  padding: 0 14px;
}
.message-item {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: flex-start;
}
.message-item.user {
  align-items: flex-end;
}
.msg-avatar {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}
.assistant-avatar {
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}
.user-avatar {
  background: linear-gradient(135deg, #34d399 0%, #22c55e 35%, #06b6d4 100%);
  color: #fff;
  box-shadow: 0 6px 14px rgba(34, 197, 94, 0.22);
  border: 1px solid rgba(255, 255, 255, 0.45);
}
.msg-bubble {
  max-width: 100%;
  width: 100%;
  box-sizing: border-box;
  padding: 8px 10px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  word-break: break-word;
  display: flex;
  flex-direction: column;
}
.message-item.assistant .msg-bubble {
  background: var(--n-color-modal);
  border: 1px solid var(--n-border-color);
}
.message-item.user .msg-bubble {
  background: var(--n-color-primary);
  color: #fff;
  text-align: right;
}
.message-item.user .msg-content,
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  text-align: right;
}
.msg-content {
  white-space: normal;
  width: 100%;
  min-width: 0;
  flex: 1;
}
.msg-reasoning-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-steps-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-steps-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.08) 0%, rgba(46, 139, 87, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-steps-header:hover {
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.14) 0%, rgba(46, 139, 87, 0.14) 100%);
}
.msg-steps-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-steps-count {
  font-size: 11px;
  background: var(--n-primary-color);
  color: #fff;
  border-radius: 10px;
  padding: 0 6px;
  line-height: 18px;
  min-width: 18px;
  text-align: center;
}
.msg-steps-content {
  padding: 10px 12px 10px 16px;
  max-height: 300px;
  overflow-y: auto;
}
.msg-step-item {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 4px 0;
  position: relative;
  font-size: 12px;
  color: var(--n-text-color-2);
  line-height: 1.5;
}
.msg-step-item:not(:last-child)::before {
  content: '';
  position: absolute;
  left: 4px;
  top: 18px;
  bottom: -4px;
  width: 1px;
  background: var(--n-border-color);
}
.msg-step-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--n-text-color-disabled);
  flex-shrink: 0;
  margin-top: 4px;
  position: relative;
  z-index: 1;
}
.msg-step-dot.step-active {
  background: #e6a23c;
  box-shadow: 0 0 4px rgba(230, 162, 60, 0.4);
}
.msg-step-dot.step-tool {
  background: #409eff;
  box-shadow: 0 0 4px rgba(64, 158, 255, 0.4);
}
.msg-step-dot.step-done {
  background: #67c23a;
  box-shadow: 0 0 4px rgba(103, 194, 58, 0.4);
}
.msg-step-text {
  flex: 1;
  min-width: 0;
  word-break: break-all;
  text-align: left;
}
.msg-reasoning-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.08) 0%, rgba(118, 75, 162, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-reasoning-header:hover {
  background: linear-gradient(135deg, rgba(102, 126, 234, 0.12) 0%, rgba(118, 75, 162, 0.12) 100%);
}
.msg-reasoning-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-reasoning-content {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  padding: 12px;
  line-height: 1.6;
  max-height: 300px;
  overflow-y: auto;
  text-align: left;
}
.msg-json-md-wrapper {
  margin-bottom: 12px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  overflow: hidden;
  background: var(--n-color-hover);
}
.msg-json-md-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  cursor: pointer;
  user-select: none;
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.08) 0%, rgba(5, 150, 105, 0.08) 100%);
  border-bottom: 1px solid var(--n-border-color);
  transition: background 0.2s;
}
.msg-json-md-header:hover {
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.14) 0%, rgba(5, 150, 105, 0.14) 100%);
}
.msg-json-md-title {
  font-size: 13px;
  font-weight: 500;
  color: var(--n-text-color-2);
}
.msg-json-md-content {
  padding: 12px;
  max-height: 300px;
  overflow-y: auto;
  text-align: left;
}
.msg-reasoning {
  font-size: 12px;
  color: var(--n-text-color-3);
  white-space: pre-wrap;
  background: var(--n-color-hover);
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 8px;
  border-left: 3px solid var(--n-primary-color);
}
.msg-bubble-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: flex-end;
  align-items: center;
  margin-top: 8px;
}
.msg-meta-row-assistant {
  flex: 1 1 100%;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
  font-size: 11px;
  color: var(--n-text-color-3);
}
.msg-meta-row-assistant .msg-time {
  flex-shrink: 0;
}
.msg-model-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}
.msg-share-btn,
.msg-copy-btn,
.msg-export-img-btn,
.msg-toggle-btn {
  padding: 2px 10px;
  font-size: 12px;
  border-radius: 12px;
  color: var(--n-primary-color);
  background-color: var(--n-primary-color-suppl);
  border: 1px solid var(--n-primary-color);
  transition: color 0.2s, border-color 0.2s, background-color 0.2s;
}
.msg-share-btn:hover,
.msg-copy-btn:hover,
.msg-export-img-btn:hover,
.msg-toggle-btn:hover {
  border-color: var(--n-primary-color);
  background-color: var(--n-primary-color);
  color: #fff;
}
.message-item.user .msg-bubble .msg-share-btn,
.message-item.user .msg-bubble .msg-copy-btn,
.message-item.user .msg-bubble .msg-export-img-btn,
.message-item.user .msg-bubble .msg-toggle-btn {
  color: rgba(255, 255, 255, 0.92);
  background-color: rgba(255, 255, 255, 0.22);
  border-color: rgba(255, 255, 255, 0.65);
}
.message-item.user .msg-bubble .msg-share-btn:hover,
.message-item.user .msg-bubble .msg-copy-btn:hover,
.message-item.user .msg-bubble .msg-export-img-btn:hover,
.message-item.user .msg-bubble .msg-toggle-btn:hover {
  color: #fff;
  border-color: rgba(255, 255, 255, 0.95);
  background-color: rgba(255, 255, 255, 0.32);
}
.msg-content .msg-markdown {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}
.msg-content .msg-markdown :deep(.md-editor-preview-wrapper) {
  width: 100%;
}
.msg-content .msg-markdown :deep(.md-editor-preview) {
  font-size: 13px;
  line-height: 1.6;
  padding: 0 8px;
  width: 100%;
  box-sizing: border-box;
}
.message-item.user .msg-content :deep(.md-editor-preview),
.message-item.user .msg-content :deep(.md-editor-preview-wrapper) {
  color: inherit;
}
.msg-loading {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--n-text-color-3);
}

.msg-meta {
  margin-top: 4px;
  font-size: 11px;
  color: var(--n-text-color-3);
  display: flex;
}
.msg-meta-user-inner {
  justify-content: flex-end;
  margin-top: 6px;
  margin-bottom: 0;
}
.message-item.user .msg-meta-user-inner {
  color: rgba(255, 255, 255, 0.78);
}

.chat-footer {
  flex-shrink: 0;
  padding: 12px 16px 16px;
  border-top: 1px solid var(--n-border-color);
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: var(--n-color-modal);
}
.chat-footer-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.chat-footer-select {
  flex: 1;
  min-width: 0;
}
.chat-footer-select .n-select {
  width: 100%;
}
.chat-footer-prompt {
  flex: 0 0 120px;
  min-width: 0;
}
.chat-footer-prompt .n-select {
  width: 100%;
}
.chat-footer-thinking {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-thinking-label {
  font-size: 12px;
  color: var(--n-text-color-2);
}
.chat-footer-memory {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.chat-footer-memory-count {
  width: 70px;
}
.chat-footer-agent-mode-select {
  width: 120px;
}
.chat-footer-memory-count .n-select {
  width: 100%;
}
.chat-quick-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 2px 4px 6px 4px;
}
.quick-chip {
  cursor: pointer;
  transition: all 0.15s ease;
  font-size: 11px;
  user-select: none;
}
.quick-chip:hover {
  opacity: 0.85;
  transform: translateY(-1px);
}
.chat-footer-input {
  display: flex;
  gap: 8px;
  align-items: flex-end;
}
.chat-footer-input .n-input {
  flex: 1;
  min-width: 0;
}
.chat-footer-input .n-input :deep(textarea) {
  text-align: left;
}
.chat-footer-input .n-button {
  flex-shrink: 0;
}
.chat-footer-abort {
  color: #f97316;
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.drawer-slide-enter-active .drawer-mask,
.drawer-slide-leave-active .drawer-mask {
  transition: opacity 0.25s ease;
}
.drawer-slide-enter-active .drawer-panel,
.drawer-slide-leave-active .drawer-panel {
  transition: transform 0.25s ease;
}
.drawer-slide-enter-from .drawer-mask,
.drawer-slide-leave-to .drawer-mask {
  opacity: 0;
}
.drawer-slide-enter-from .drawer-panel,
.drawer-slide-leave-to .drawer-panel {
  transform: translateX(100%);
}
.drawer-slide-enter-to .drawer-mask,
.drawer-slide-leave-from .drawer-mask {
  opacity: 1;
}
.drawer-slide-enter-to .drawer-panel,
.drawer-slide-leave-from .drawer-panel {
  transform: translateX(0);
}

/* 全局态势感知仪表条 */
.perception-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  margin: 6px 12px;
  background: var(--n-color-embedded, rgba(128, 128, 128, 0.08));
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.2));
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 12px;
}
.perception-banner:hover {
  background: var(--n-color-hover, rgba(128, 128, 128, 0.15));
  border-color: #18a058;
}
.perception-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.phase-tag {
  font-weight: 600;
  color: var(--n-text-color, #333);
}
.traffic-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  display: inline-block;
  box-shadow: 0 0 6px currentColor;
}
.traffic-text {
  font-weight: 600;
}
.breadth-text {
  font-size: 11px;
  margin-left: 4px;
}
.perception-right {
  display: flex;
  align-items: center;
  gap: 6px;
}
.ask-hint {
  font-size: 11px;
  color: #18a058;
  font-weight: 500;
}

/* 当前聚焦个股智能感知条 */
.context-stock-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  margin: 0 12px 6px 12px;
  background: linear-gradient(135deg, rgba(64, 152, 252, 0.08) 0%, rgba(24, 160, 88, 0.08) 100%);
  border: 1px solid rgba(64, 152, 252, 0.25);
  border-radius: 8px;
  font-size: 12px;
  gap: 8px;
  flex-wrap: wrap;
}
.context-stock-left {
  display: flex;
  align-items: center;
  gap: 4px;
}
.context-stock-pin {
  color: #888;
  font-size: 11px;
}
.context-stock-name {
  font-weight: 700;
  color: var(--n-text-color, #333);
}
.context-stock-code {
  font-size: 11px;
  color: #4098fc;
}
.context-stock-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* 伴随控制动作卡片 */
.agent-action-cards {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 10px;
  padding-top: 8px;
  border-top: 1px dashed var(--n-border-color, rgba(128, 128, 128, 0.2));
}
.agent-action-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 10px;
  background: rgba(24, 160, 88, 0.08);
  border: 1px solid rgba(24, 160, 88, 0.25);
  border-radius: 6px;
  font-size: 12px;
}
.action-card-info {
  display: flex;
  align-items: center;
  gap: 6px;
}
.action-icon {
  font-size: 13px;
}
.action-title {
  color: var(--n-text-color, #333);
  font-weight: 500;
}
</style>

<style>
body > div:has(.n-select-menu) {
  z-index: 10002 !important;
}

.msg-markdown .md-editor-code-block {
  position: relative;
}
.msg-markdown .md-editor-code-block pre {
  margin: 0;
}
.msg-markdown .md-editor-code-block .code-collapse-btn {
  position: absolute;
  top: 0;
  right: 0;
  z-index: 2;
  padding: 2px 8px;
  font-size: 11px;
  color: var(--n-text-color-3);
  background: var(--n-color-hover);
  border: 1px solid var(--n-border-color);
  border-radius: 0 4px 0 4px;
  cursor: pointer;
  user-select: none;
  opacity: 0;
  transition: opacity 0.2s;
}
.msg-markdown .md-editor-code-block:hover .code-collapse-btn {
  opacity: 1;
}
.msg-markdown .md-editor-code-block.code-collapsed pre {
  max-height: 80px;
  overflow: hidden;
}
.msg-markdown .md-editor-code-block.code-collapsed::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 40px;
  background: linear-gradient(transparent, var(--n-color));
  pointer-events: none;
}
.msg-status-text {
  display: flex;
  align-items: center;
  gap: 6px;
  background: var(--n-color-info);
  color: var(--n-color-info-text);
  border-radius: 6px;
  padding: 6px 10px;
  margin-bottom: 6px;
  font-size: 13px;
}

.agent-metrics-capsule {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  margin: 6px 0 4px 0;
  padding: 3px 8px;
  background: rgba(140, 140, 140, 0.08);
  border: 1px solid rgba(140, 140, 140, 0.15);
  border-radius: 12px;
  font-size: 11px;
  color: var(--n-text-color-3);
  user-select: none;
}

.agent-metrics-capsule .metrics-tag {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  font-weight: 500;
}

.agent-metrics-capsule .metrics-warning {
  color: #f59e0b;
  font-weight: 600;
}
.steps-expanded,
.reasoning-expanded {
  background: var(--n-color-info);
}
</style>
