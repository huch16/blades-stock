<template>
  <div class="chat-container">
    <!-- 顶部 HUD 控制中枢 -->
    <header class="chat-header">
      <div class="header-left">
        <n-button
          quaternary
          size="small"
          class="session-drawer-btn"
          @click="showSessionDrawer = true"
          title="会话历史管理"
        >
          <template #icon>
            <ChatIcon />
          </template>
          <span class="session-title-text">{{ currentSessionTitle }}</span>
          <span class="session-badge">{{ messages.length }}</span>
        </n-button>
        <n-button
          quaternary
          circle
          size="tiny"
          @click="createNewSession"
          title="新建对话"
        >
          <AddIcon />
        </n-button>
      </div>

      <div class="header-center">
        <!-- 模式快捷胶囊 -->
        <div class="mode-capsule-group">
          <button
            v-for="opt in agentModeOptions"
            :key="opt.value"
            class="mode-capsule-btn"
            :class="{ active: agentMode === opt.value }"
            @click="agentMode = opt.value"
            :title="opt.description"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <div class="header-right">
        <!-- 模型选择 -->
        <n-select
          v-model:value="activeConfigId"
          :options="configOptions"
          :placeholder="configOptions.length ? '选择 AI 模型配置' : '请先在设置中添加 AI'"
          size="small"
          class="model-selector"
          clearable
          @update:value="onConfigChange"
        />

        <n-tooltip placement="bottom" content="投研场景库">
          <n-button
            quaternary
            circle
            size="small"
            @click="showScenarioModal = true"
          >
            <ViewListIcon />
          </n-button>
        </n-tooltip>

        <n-tooltip placement="bottom" content="清空当前对话">
          <n-button
            quaternary
            circle
            size="small"
            @click="clearCurrentHistory"
          >
            <DeleteIcon />
          </n-button>
        </n-tooltip>

        <n-tooltip placement="bottom" :content="isCompact ? '标准视图' : '紧凑视图'">
          <n-button
            quaternary
            circle
            size="small"
            @click="isCompact = !isCompact"
          >
            <FullscreenExitIcon v-if="!isCompact" />
            <FullscreenIcon v-else />
          </n-button>
        </n-tooltip>
      </div>
    </header>

    <!-- 全局宏观态势感知 HUD 仪表条 (桌面端与移动端统一呈现) -->
    <section
      v-if="perceptionSnapshot"
      class="macro-perception-hud"
      title="点击呼叫 Agent 深度解读当前宏观节律与持仓策略"
      @click="askMacroPerception"
    >
      <div class="hud-left">
        <span class="phase-pill">{{ perceptionSnapshot.Phase || '宏观节律感知' }}</span>
        <span
          class="traffic-beacon"
          :style="{ backgroundColor: getTrafficColor(perceptionSnapshot.TrafficLight) }"
        />
        <span
          class="traffic-label"
          :style="{ color: getTrafficColor(perceptionSnapshot.TrafficLight) }"
        >
          {{ getTrafficText(perceptionSnapshot.TrafficLight) }}
        </span>
        <span
          v-if="perceptionSnapshot.MarketBreadth"
          class="breadth-ratio"
        >
          <span class="up-count">涨 {{ perceptionSnapshot.MarketBreadth.UpCount }}</span>
          <span class="separator">/</span>
          <span class="down-count">跌 {{ perceptionSnapshot.MarketBreadth.DownCount }}</span>
        </span>
      </div>
      <div class="hud-right">
        <n-tag
          v-if="perceptionSnapshot.PortfolioRisk && perceptionSnapshot.PortfolioRisk.RiskLevel !== 'SAFE'"
          size="small"
          type="error"
          round
          class="risk-tag"
        >
          ⚠️ {{ perceptionSnapshot.PortfolioRisk.RiskLevel === 'DANGER' ? '高危持仓预警' : '持仓警示' }}
        </n-tag>
        <span class="action-hint">
          <span>宏观态势推演</span>
          <span class="arrow">➔</span>
        </span>
      </div>
    </section>

    <!-- 主体聊天消息滚动区域 (自然时间线：旧消息在上，新消息在下) -->
    <main
      class="chat-messages-viewport"
      ref="messagesContainer"
      @scroll="handleScroll"
    >
      <!-- 消息锚点导航（折叠式微型右侧指示器） -->
      <MessageAnchorNav
        :items="anchorNavItems"
        :show="messages.length > 2"
        @scroll="scrollToMessage"
      />

      <!-- 空白欢迎态：专业投研引导工作台 -->
      <section v-if="messages.length === 0" class="welcome-workspace">
        <div class="welcome-hero">
          <div class="hero-icon-glow">💬</div>
          <h2 class="hero-title">Blades AI 智能投研中枢</h2>
          <p class="hero-desc">
            融合多源实时行情、本地量化指标引擎、ETF动量轮动与网络搜索的多智能体协作系统
          </p>
        </div>

        <div class="scenario-grid">
          <div
            v-for="(card, idx) in scenarioCards"
            :key="idx"
            class="scenario-card"
            @click="applyScenario(card.prompt)"
          >
            <div class="card-header">
              <span class="card-icon">{{ card.icon }}</span>
              <span class="card-badge">{{ card.badge }}</span>
            </div>
            <h4 class="card-title">{{ card.title }}</h4>
            <p class="card-desc">{{ card.desc }}</p>
            <div class="card-prompt-hint">
              <span>试一试：{{ card.example }}</span>
              <span class="arrow-hint">→</span>
            </div>
          </div>
        </div>

        <div class="quick-keywords-bar">
          <span class="keywords-label">快速联想：</span>
          <div class="keywords-list">
            <span
              v-for="kw in quickKeywords"
              :key="kw"
              class="keyword-chip"
              @click="applyScenario(kw)"
            >
              {{ kw }}
            </span>
          </div>
        </div>
      </section>

      <!-- 消息列表渲染 (严格单次渲染，杜绝重复占位) -->
      <div v-else class="messages-flow">
        <ChatMessageComp
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
          @save-thesis="handleSaveThesisFromMsg"
        />
      </div>

      <!-- 滚动回底部悬浮球 -->
      <Transition name="fade">
        <button
          v-if="showScrollBottom"
          class="fab-scroll-bottom"
          @click="scrollToBottom(true)"
          title="回到底部最新消息"
        >
          <ArrowDownIcon />
          <span class="fab-text">最新</span>
        </button>
      </Transition>
    </main>

    <!-- 底部输入与操作中枢 -->
    <footer class="chat-input-cockpit" :class="{ compact: isCompact }">
      <!-- 快捷指令横条 (芯片式工具条) -->
      <div class="input-quick-bar" v-if="!isCompact">
        <div class="quick-chips">
          <button
            v-for="chip in quickChips"
            :key="chip.label"
            class="quick-chip-btn"
            @click="fillInput(chip.text)"
            :title="chip.tip"
          >
            <span class="chip-icon">{{ chip.icon }}</span>
            <span class="chip-label">{{ chip.label }}</span>
          </button>
        </div>

        <div class="status-summary" v-if="isStreaming">
          <span class="live-dot"></span>
          <span class="live-text">{{ currentStreamStatus || '正在智能推理分析中...' }}</span>
        </div>
      </div>

      <!-- 输入框主体 -->
      <div class="input-sender-box">
        <t-chat-sender
          ref="chatSenderRef"
          v-model="inputValue"
          :textarea-props="{
            placeholder: configOptions.length ? '输入个股代码/名称、ETF轮动需求或选股策略... (Enter发送，Shift+Enter换行)' : '⚠️ 请先选择或在设置中添加 AI 模型配置',
            rows: 1,
            autosize: { minRows: 1, maxRows: 6 },
          }"
          :loading="isStreaming"
          :stop-disabled="false"
          @send="handleSend"
          @stop="handleStop"
        >
          <template #suffix>
            <div class="sender-actions">
              <t-button
                v-if="isStreaming"
                theme="danger"
                variant="base"
                size="medium"
                class="stop-btn"
                @click="handleStop"
              >
                ⏹️ 停止生成
              </t-button>
              <t-button
                v-else
                theme="primary"
                variant="base"
                size="medium"
                :disabled="!inputValue.trim() || !activeConfigId"
                @click="handleSend"
              >
                发送
              </t-button>
            </div>
          </template>
        </t-chat-sender>
      </div>

      <!-- 缺少配置引导 -->
      <div v-if="!configOptions.length" class="config-alert-banner">
        ⚠️ 未检测到有效 AI 模型配置，请前往 <router-link to="/settings">设置中心</router-link> 选配 Ollama / DeepSeek / 硅基流动 等模型。
      </div>
    </footer>

    <!-- 左侧会话历史抽屉 -->
    <n-drawer
      v-model:show="showSessionDrawer"
      :width="drawerWidth"
      placement="left"
    >
      <n-drawer-content title="💬 投研会话记录" closable>
        <div class="session-drawer-content">
          <n-button
            type="primary"
            dashed
            block
            class="btn-new-chat"
            @click="createNewSession"
          >
            ➕ 新建投研对话
          </n-button>

          <div class="session-list">
            <div
              v-for="s in sessions"
              :key="s.id"
              class="session-item"
              :class="{ active: s.id === currentSessionId }"
              @click="switchSession(s.id)"
            >
              <div class="session-info">
                <span class="session-name" :title="s.title">{{ s.title }}</span>
                <span class="session-meta">
                  {{ formatSessionTime(s.updatedAt) }} · {{ s.messages.length }}条
                </span>
              </div>
              <div class="session-actions" @click.stop>
                <n-button
                  quaternary
                  circle
                  size="tiny"
                  @click="deleteSession(s.id)"
                  title="删除此会话"
                >
                  <DeleteIcon />
                </n-button>
              </div>
            </div>
          </div>
        </div>
      </n-drawer-content>
    </n-drawer>

    <!-- 投研场景库弹窗 -->
    <n-modal
      v-model:show="showScenarioModal"
      preset="card"
      title="💡 专业投研场景库"
      style="width: 720px; max-width: 90vw;"
    >
      <div class="modal-scenarios-list">
        <div
          v-for="(item, idx) in allScenarios"
          :key="idx"
          class="scenario-modal-item"
          @click="selectModalScenario(item.prompt)"
        >
          <div class="item-head">
            <span class="item-tag">{{ item.category }}</span>
            <span class="item-title">{{ item.title }}</span>
          </div>
          <p class="item-desc">{{ item.prompt }}</p>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeMount, onBeforeUnmount, nextTick, provide } from 'vue'
import {
  NSelect,
  NButton,
  NTooltip,
  NDrawer,
  NDrawerContent,
  NModal,
  NTag
} from 'naive-ui'
import {
  ArrowDownIcon,
  DeleteIcon,
  FullscreenIcon,
  FullscreenExitIcon,
  ChatIcon,
  AddIcon,
  ViewListIcon
} from 'tdesign-icons-vue-next'
import { ChatWithAgent, GetConfig, GetVersionInfo, GetSystemStateSnapshot, SaveStockThesis } from '../../wailsjs/go/main/App'
import { useAiConfigStore } from '@/stores/aiConfig'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime'
import { cleanFinalContent } from '@/utils/cleanText'
import {
  useMessageAnchor,
  type ChatMessage,
  type StreamChunk,
  genMsgId
} from './chat'
import ChatMessageComp from './chat/ChatMessage.vue'
import MessageAnchorNav from './chat/MessageAnchorNav.vue'

// ========== 数据接口与持久化类型 ==========
interface ChatSession {
  id: string
  title: string
  updatedAt: number
  messages: ChatMessage[]
}

const STORAGE_KEY = 'blades_stock_chat_sessions_v2'
const ACTIVE_SESSION_KEY = 'blades_stock_active_session_id_v2'

// ========== 核心状态 ==========
const aiConfigStore = useAiConfigStore()
const sessions = ref<ChatSession[]>([])
const currentSessionId = ref<string>('')
const inputValue = ref('')
const isStreaming = ref(false)
const streamingMsgId = ref<string | null>(null)
const currentStreamStatus = ref<string>('')
const showScrollBottom = ref(false)
const isCompact = ref(false)
const showSessionDrawer = ref(false)
const showScenarioModal = ref(false)
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)
const drawerWidth = computed(() => (isMobile.value ? '85vw' : 320))

function onWindowResize() {
  isMobile.value = typeof window !== 'undefined' ? window.innerWidth <= 768 : false
}
const agentMode = ref<'auto' | 'react' | 'plan_execute'>('auto')
const userAvatar = ref('https://tdesign.gtimg.com/site/avatar.jpg')
const userName = ref('用户')
const messagesContainer = ref<HTMLElement | null>(null)

// 响应式消息列表：直接绑定当前激活会话的 messages
const messages = computed<ChatMessage[]>({
  get() {
    const s = sessions.value.find(item => item.id === currentSessionId.value)
    return s ? s.messages : []
  },
  set(val: ChatMessage[]) {
    const s = sessions.value.find(item => item.id === currentSessionId.value)
    if (s) {
      s.messages = val
      s.updatedAt = Date.now()
      saveSessionsToStorage()
    }
  }
})

const currentSessionTitle = computed(() => {
  const s = sessions.value.find(item => item.id === currentSessionId.value)
  return s ? s.title : '默认对话'
})

// ========== AI 配置计算 ==========
const activeConfigId = computed({
  get: () => aiConfigStore.activeConfigId,
  set: (val) => { aiConfigStore.setActiveConfigId(val) }
})

const configOptions = computed(() => aiConfigStore.configs.map(c => ({
  label: `${c.name || '配置' + c.ID} (${c.modelName || '默认模型'})`,
  value: c.ID,
})))

// ========== 全局宏观态势感知 ==========
const perceptionSnapshot = ref<any>(null)
let perceptionTimer: any = null

async function refreshPerceptionSnapshot() {
  try {
    const snap = await GetSystemStateSnapshot()
    if (snap) {
      perceptionSnapshot.value = snap
    }
  } catch (e) {
    console.warn('[agent-chat] refreshPerceptionSnapshot failed:', e)
  }
}

function getTrafficColor(light?: string) {
  if (light === 'SAFE') return '#18a058'
  if (light === 'CAUTION') return '#f0a020'
  if (light === 'DANGER') return '#d03050'
  return '#888'
}

function getTrafficText(light?: string) {
  if (light === 'SAFE') return '积极做多'
  if (light === 'CAUTION') return '谨慎防守'
  if (light === 'DANGER') return '强风控空仓'
  return '中性观望'
}

function askMacroPerception() {
  if (!perceptionSnapshot.value) return
  const phase = perceptionSnapshot.value.Phase || ''
  const traffic = perceptionSnapshot.value.TrafficLight || ''
  inputValue.value = `请结合当前市场【${phase}】节律与情绪【${traffic}】状态，对全天大局观、主线机会和实盘持仓风控进行综合推演。`
  agentMode.value = 'plan_execute'
  nextTick(() => {
    handleSend()
  })
}

const agentModeOptions = [
  { label: '🤖 智能自适应', value: 'auto' as const, description: '根据问题复杂度自动选择单步或多步执行策略' },
  { label: '⚡ 极速诊断', value: 'react' as const, description: 'ReAct 即时单步调用，秒级响应，适合快速行情问答' },
  { label: '🧠 深度规划', value: 'plan_execute' as const, description: 'Plan-and-Execute 深度多步拆解，适合全维度复杂分析' },
  { label: '📑 买方直出', value: 'direct' as const, description: '原生直连生成机构深度研报，零外部工具开销' },
]

// ========== 投研场景库数据 ==========
const scenarioCards = [
  {
    icon: '🎯',
    badge: '个股体检',
    title: '全维度深度诊断',
    desc: '综合基本面、技术面均线、大资金动向与近期研报给出综合评级。',
    example: '深度分析 600519 贵州茅台',
    prompt: '请对 600519 贵州茅台 进行全维度深度诊断，涵盖：1. 估值与基本面指标；2. 近期日K线量价与支撑阻力；3. 主力资金动向；4. 核心催化与潜在风险。',
  },
  {
    icon: '🚀',
    badge: '主线战法',
    title: 'ETF 加权动量轮动',
    desc: '计算全市场核心宽基与行业 ETF 21日加权动量评分，捕捉进攻主线。',
    example: '计算当前 ETF 动量排名',
    prompt: '请调用 GetETFMomentumRanking 工具获取当前场内核心 ETF 21日加权动量排名，并根据动量数值分析当前最强进攻板块与防御配置建议。',
  },
  {
    icon: '📈',
    badge: '技术选股',
    title: '量化指标战法选股',
    desc: '基于均线多头排列、MACD金叉或放量突破，执行本地策略筛选。',
    example: '筛选均线多头放量突破个股',
    prompt: '请帮我筛选当前市场上呈现均线多头排列且成交量放量突破的个股，并给出具体技术面评分和选股理由。',
  },
  {
    icon: '⚖️',
    badge: '横向对比',
    title: '龙头双股深度PK',
    desc: '对比同行业两只核心龙头的商业模式、估值弹性与技术面强弱。',
    example: '对比 宁德时代 vs 比亚迪',
    prompt: '请对比分析 300750 宁德时代 与 002594 比亚迪：比较两者的估值水平、盈利质量、近期资金流入以及当前K线走势强弱。',
  },
]

const quickKeywords = [
  '分析 000001 平安银行',
  '计算 场内ETF动量排名',
  '深度量化排雷 600519',
  '筛选 低估值高股息股票',
  '查询最新龙虎榜机构席位',
  '半导体产业链最新热点',
  '贵州茅台 vs 五粮液',
  '回测 双均线策略',
]

const quickChips = [
  { icon: '📊', label: '个股诊断', text: '请全面分析股票 ', tip: '输入代码或名称' },
  { icon: '🛡️', label: '深度排雷', text: '请对 [输入代码] 进行深度排雷：核查控股股东质押比例、未来一年解禁日程、近3季度股东户数及两融资金。', tip: '防爆雷与筹码穿透' },
  { icon: '📑', label: '财报体检', text: '请深度体检 [输入代码] 的财务质量：分析近3年营业收入与扣非净利润、真实ROE与销售毛利率、以及经营活动现金流净额。', tip: '基本面财务深度穿透' },
  { icon: '🚀', label: 'ETF动量', text: '请计算当前场内主流ETF的21日加权动量排名并解读', tip: '动量轮动分析' },
  { icon: '🔍', label: '多头选股', text: '请筛选近期均线多头排列且量比大于1.5的强势股', tip: '技术指标选股' },
  { icon: '📰', label: '热点资讯', text: '检索今天A股市场最核心的概念热点与资金流向', tip: '市场情绪与风口' },
  { icon: '⚖️', label: '双股PK', text: '对比分析 与 的估值与走势差异', tip: '同赛道对比' },
]

const allScenarios = [
  { category: '个股研究', title: '全维度深度诊断', prompt: '请对 [股票代码/名称] 进行全方位诊断，包括基本面财务、K线趋势技术形态、资金进出动向及未来投资建议。' },
  { category: '个股研究', title: '支撑位与阻力位测算', prompt: '请分析 [股票代码] 的近期日K线走势，测算关键支撑位、压力位与止损参考点位。' },
  { category: '买方量化投研', title: '深度排雷与筹码穿透', prompt: '请对 [股票代码] 进行深度排雷：查询控股股东质押比例、未来一年解禁日程、近3季度股东户数趋势及主力资金净流入，给出综合风控评级。' },
  { category: '买方量化投研', title: '财报体检与真实ROE验证', prompt: '请深入体检 [股票代码] 的财务报表：对比近3年营业收入与扣非净利润、ROE与销售净利率、分析经营现金流净额与净利润比值，验证其盈利真实性。' },
  { category: '买方量化投研', title: '龙虎榜机构席位追踪', prompt: '请查询最近一个交易日的龙虎榜机构席位明细与上榜汇总，重点筛选机构专用席位大额净买入的股票，分析机构真实建仓意图。' },
  { category: '买方量化投研', title: '业绩预告超预期挖掘', prompt: '请检索近期披露业绩预告中净利润同比大幅预增的公司，结合估值百分位给出性价比标的分析。' },
  { category: '买方量化投研', title: '宽基指数估值与权重穿透', prompt: '请分析当前沪深300与中证A500的市盈率PE、历史分位数及前五大成分股与板块权重分布。' },
  { category: '量化策略', title: 'ETF 加权动量轮动', prompt: '请获取主流场内ETF的21日加权动量排名表，按动量评分排序并给出当前轮动调仓建议。' },
  { category: '量化选股', title: '价值防守型选股', prompt: '请筛选市盈率PE<15、近三年股息率>3.5%、ROE>12%的高股息稳健型股票。' },
  { category: '量化选股', title: '强势突破型选股', prompt: '请利用量化策略筛选突破近期整理平台且放量上涨的标的。' },
  { category: '行业宏观', title: '产业链景气度研判', prompt: '请深入分析当前半导体及算力产业链的行业景气度、产业政策催化与核心受益龙头。' },
]

// ========== 锚点导航 ==========
const {
  anchorNavItems,
  registerElement,
  unregisterElement,
  scrollToMessage,
} = useMessageAnchor(messages)

provide('registerMessageElement', registerElement)
provide('unregisterMessageElement', unregisterElement)

// ========== 本地会话持久化逻辑 ==========
function loadSessionsFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      sessions.value = JSON.parse(raw)
    }
  } catch (e) {
    console.error('Failed to parse sessions from localStorage', e)
  }

  if (!sessions.value || sessions.value.length === 0) {
    const defaultSession: ChatSession = {
      id: genMsgId('session'),
      title: '新对话',
      updatedAt: Date.now(),
      messages: [],
    }
    sessions.value = [defaultSession]
    currentSessionId.value = defaultSession.id
  } else {
    const savedActive = localStorage.getItem(ACTIVE_SESSION_KEY)
    const exists = sessions.value.find(s => s.id === savedActive)
    currentSessionId.value = exists ? exists.id : sessions.value[0].id
  }
}

function saveSessionsToStorage() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions.value))
    localStorage.setItem(ACTIVE_SESSION_KEY, currentSessionId.value)
  } catch (e) {
    console.error('Failed to save sessions to localStorage', e)
  }
}

function createNewSession() {
  const newSession: ChatSession = {
    id: genMsgId('session'),
    title: '新对话',
    updatedAt: Date.now(),
    messages: [],
  }
  sessions.value.unshift(newSession)
  currentSessionId.value = newSession.id
  saveSessionsToStorage()
  showSessionDrawer.value = false
}

function switchSession(id: string) {
  currentSessionId.value = id
  saveSessionsToStorage()
  showSessionDrawer.value = false
  nextTick(() => {
    scrollToBottom(false)
  })
}

function deleteSession(id: string) {
  if (sessions.value.length <= 1) {
    const s = sessions.value[0]
    s.title = '新对话'
    s.messages = []
    s.updatedAt = Date.now()
    saveSessionsToStorage()
    return
  }
  sessions.value = sessions.value.filter(s => s.id !== id)
  if (currentSessionId.value === id) {
    currentSessionId.value = sessions.value[0].id
  }
  saveSessionsToStorage()
}

function formatSessionTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

// ========== Agent 流式通信监听 ==========
function handleAgentMessage(data: StreamChunk) {
  if (!isStreaming.value) return

  const curList = messages.value
  const targetMsg = curList.find(m => m.id === streamingMsgId.value)
  if (!targetMsg) return

  if (data.role === 'assistant') {
    if ((data as any).model_name || (data as any).modelName) {
      targetMsg.modelName = ((data as any).model_name || (data as any).modelName) + ' (容灾降级)'
    }
    // 状态更新
    if (data.reasoning_content) {
      const rc = data.reasoning_content
      if (rc.startsWith('[STATUS]')) {
        const st = rc.replace(/^\[STATUS\]/, '').trim()
        targetMsg.statusText = st
        currentStreamStatus.value = st
      } else if (rc.startsWith('[STEP]')) {
        const stepText = rc.replace(/^\[STEP\]/, '').trim()
        if (stepText) {
          if (!targetMsg.steps) targetMsg.steps = []
          targetMsg.steps.push(stepText)
        }
      } else {
        targetMsg.rawReasoning = (targetMsg.rawReasoning || '') + rc
        targetMsg.reasoning = targetMsg.rawReasoning
      }
    }

    // 正文流式追加
    if (data.content) {
      if (data.content === 'agent-DONE') {
        if (data.metrics) targetMsg.metrics = data.metrics
      } else {
        targetMsg.rawContent = (targetMsg.rawContent || '') + data.content
        targetMsg.content = targetMsg.rawContent
      }
    }

    // 工具调用追踪
    if (data.tool_calls && data.tool_calls.length > 0) {
      if (!targetMsg.toolCalls) targetMsg.toolCalls = []
      targetMsg.toolCalls.push(...data.tool_calls)
    }

    if (data.metrics) {
      targetMsg.metrics = data.metrics
    }

    // 智能滚动吸底
    autoScrollIfNearBottom()

    // 结束处理
    if (data.response_meta?.finish_reason === 'stop') {
      finalizeStream(targetMsg)
    }
  }
}

function finalizeStream(targetMsg?: ChatMessage) {
  isStreaming.value = false
  streamingMsgId.value = null
  currentStreamStatus.value = ''

  if (targetMsg) {
    if (targetMsg.rawContent) {
      targetMsg.content = cleanFinalContent(targetMsg.rawContent)
    }
  }
  saveSessionsToStorage()
}

// ========== 滚动控制 ==========
let userIsScrolledUp = false

function handleScroll() {
  const el = messagesContainer.value
  if (!el) return
  const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  userIsScrolledUp = distanceFromBottom > 150
  showScrollBottom.value = userIsScrolledUp
}

function autoScrollIfNearBottom() {
  if (!userIsScrolledUp) {
    scrollToBottom(false)
  }
}

function scrollToBottom(smooth = true) {
  nextTick(() => {
    const el = messagesContainer.value
    if (el) {
      el.scrollTo({
        top: el.scrollHeight,
        behavior: smooth ? 'smooth' : 'auto',
      })
    }
  })
}

// ========== 发送与停止 ==========
function handleSend() {
  if (isStreaming.value) {
    handleStop()
    return
  }

  const text = inputValue.value.trim()
  if (!text) return

  const configId = activeConfigId.value
  if (!configId || configId <= 0) {
    import('tdesign-vue-next').then(({ MessagePlugin }) => {
      MessagePlugin.warning('请先选择有效的 AI 模型配置')
    })
    return
  }

  // 1. 用户消息放入列表末尾 (正序)
  const userMsg: ChatMessage = {
    id: genMsgId('user'),
    avatar: userAvatar.value,
    name: userName.value,
    datetime: new Date().toISOString(),
    content: text,
    role: 'user',
  }

  // 2. AI 占位消息放入列表末尾
  const assistantId = genMsgId('assistant')
  const assistantMsg: ChatMessage = {
    id: assistantId,
    avatar: '',
    name: 'Blades 投研 Agent',
    datetime: new Date().toISOString(),
    content: '',
    rawContent: '',
    reasoning: '',
    rawReasoning: '',
    role: 'assistant',
    steps: [],
    statusText: '思考中...',
    toolCalls: [],
  }

  const s = sessions.value.find(item => item.id === currentSessionId.value)
  if (s) {
    s.messages.push(userMsg, assistantMsg)
    // 自动重命名默认会话标题
    if (s.title === '新对话' && text.length > 0) {
      s.title = text.slice(0, 18) + (text.length > 18 ? '…' : '')
    }
    s.updatedAt = Date.now()
    saveSessionsToStorage()
  }

  streamingMsgId.value = assistantId
  isStreaming.value = true
  currentStreamStatus.value = '正在思考并解析意图...'
  userIsScrolledUp = false

  inputValue.value = ''
  scrollToBottom(true)

  ChatWithAgent(
    text,
    configId,
    null as any,
    false,
    0,
    false,
    agentMode.value === 'auto' ? '' : agentMode.value,
    currentSessionId.value || ''
  ).catch((err: any) => {
    console.error('ChatWithAgent error', err)
    assistantMsg.content = `❌ 服务调用失败：${err?.message || err}`
    finalizeStream(assistantMsg)
  })
}

function handleStop() {
  isStreaming.value = false
  streamingMsgId.value = null
  currentStreamStatus.value = ''
  saveSessionsToStorage()
  import('tdesign-vue-next').then(({ MessagePlugin }) => {
    MessagePlugin.info('已停止响应')
  })
}

// ========== 场景交互辅助 ==========
function applyScenario(promptText: string) {
  inputValue.value = promptText
  nextTick(() => {
    handleSend()
  })
}

function fillInput(text: string) {
  inputValue.value = text
  const textarea = document.querySelector('.chat-input-cockpit textarea') as HTMLTextAreaElement
  if (textarea) textarea.focus()
}

function selectModalScenario(prompt: string) {
  inputValue.value = prompt
  showScenarioModal.value = false
  const textarea = document.querySelector('.chat-input-cockpit textarea') as HTMLTextAreaElement
  if (textarea) textarea.focus()
}

function handleCopy(content: string) {
  navigator.clipboard.writeText(content).then(() => {
    import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.success('内容已复制到剪贴板'))
  }).catch(() => {
    import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.error('复制失败'))
  })
}

function handleRetry(msgId: string) {
  const curList = messages.value
  const idx = curList.findIndex(m => m.id === msgId)
  if (idx === -1) return
  const msg = curList[idx]
  if (msg.role === 'user') {
    inputValue.value = msg.content
    messages.value = curList.slice(0, idx)
    handleSend()
  }
}

function handleReference(msgId: string) {
  const msg = messages.value.find(m => m.id === msgId)
  if (msg) {
    const quote = msg.content.slice(0, 150)
    inputValue.value += `\n> 引用：${quote}...\n`
    import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.success('已引用到输入框'))
  }
}

function handleEdit(msg: ChatMessage) {
  if (msg.role === 'user') {
    inputValue.value = msg.content
    messages.value = messages.value.filter(m => m.id !== msg.id)
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

function clearCurrentHistory() {
  import('tdesign-vue-next').then(({ DialogPlugin }) => {
    DialogPlugin.confirm({
      header: '清空当前对话',
      body: '确定要清空当前会话的全部消息历史吗？此操作不可恢复。',
      confirmBtn: '确认清空',
      cancelBtn: '取消',
      onConfirm: () => {
        messages.value = []
        saveSessionsToStorage()
        import('tdesign-vue-next').then(({ MessagePlugin }) => MessagePlugin.success('会话已清空'))
      }
    })
  })
}

// ========== 跨组件咨询接入与买方立论记忆入库 ==========
function handleAgentConsult(payload: any) {
  if (!payload) return
  const { question, stockCode, stockName, context, mode, autoSend } = payload
  if (!question && !context) return

  // 1. 设置执行模式
  if (mode && ['auto', 'react', 'plan_execute', 'direct'].includes(mode)) {
    agentMode.value = mode as any
  }

  // 2. 组装输入框文本 (一手量化指标前置注入 + 深度分析推演任务)
  let finalInput = ''
  if (context) {
    finalInput = `【一手量化与买方参考数据】：\n${context}\n\n【分析推演任务】：\n${question || '请结合上述一手量化参考数据，给出全维度买方深度研报与买方立论。'}`
  } else {
    finalInput = question || ''
  }

  inputValue.value = finalInput

  // 3. 自动触发分析
  if (autoSend !== false) {
    nextTick(() => {
      handleSend()
    })
  }
}

function onWindowAgentConsult(e: any) {
  if (e?.detail) {
    handleAgentConsult(e.detail)
  }
}

// 智能提炼买方 5-Verdict 认知结构 (立论、假说、催化、证伪风险)
function extractThesisFromText(text: string, defaultCode?: string, defaultName?: string) {
  if (!text) return null

  // 1. 识别股票代码 (标准 6 位 A 股/ETF 代码)
  let code = defaultCode || ''
  if (!code) {
    const codeMatch = text.match(/\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b/)
    if (codeMatch) code = codeMatch[1]
  }
  if (!code) return null

  // 2. 识别股票名称
  let name = defaultName || ''
  if (!name) {
    const nameMatch = text.match(/[【(（]([A-Za-z\u4e00-\u9fa5\d]+)[)）】]\s*(?:[(（]?\b\d{6}\b[)）]?)/) ||
                      text.match(/【([A-Za-z\u4e00-\u9fa5\d]+)\s*[0-9]{6}】/)
    if (nameMatch) name = nameMatch[1]
  }

  // 3. 识别买方五维定性判定
  let verdict = 'NO_CHANGE'
  let verdictText = '维持现状'
  if (text.includes('逻辑强化') || text.includes('THESIS_STRENGTHENED') || text.includes('强烈买入') || text.includes('积极做多')) {
    verdict = 'THESIS_STRENGTHENED'
    verdictText = '逻辑强化'
  } else if (text.includes('逻辑证伪') || text.includes('THESIS_BROKEN') || text.includes('清仓止损') || text.includes('强风控空仓')) {
    verdict = 'THESIS_BROKEN'
    verdictText = '逻辑证伪'
  } else if (text.includes('逻辑削弱') || text.includes('THESIS_WEAKENED') || text.includes('谨慎防守') || text.includes('逢高减仓')) {
    verdict = 'THESIS_WEAKENED'
    verdictText = '逻辑削弱'
  } else if (text.includes('首次建档') || text.includes('新论点') || text.includes('首次立论') || text.includes('NEW_THESIS')) {
    verdict = 'NEW_THESIS'
    verdictText = '首次立论'
  }

  // 4. 提取核心投资假说
  let coreThesis = ''
  const thesisMatch = text.match(/(?:核心投资逻辑|核心假说|核心立论|投资逻辑|核心论点)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (thesisMatch) {
    coreThesis = thesisMatch[1].replace(/[#*`]/g, '').trim()
  } else {
    coreThesis = text.slice(0, 160).replace(/[#*`]/g, '').trim()
  }

  // 5. 提取重点跟踪催化剂
  let keyCatalysts = ''
  const catMatch = text.match(/(?:催化剂|跟踪催化剂|核心催化|催化事件|关键催化)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (catMatch) {
    keyCatalysts = catMatch[1].replace(/[#*`]/g, '').trim()
  }

  // 6. 提取证伪条件与防守止损
  let riskFactors = ''
  const riskMatch = text.match(/(?:证伪条件|风险因素|反转条件|止损条件|防守位|黑天鹅风险)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (riskMatch) {
    riskFactors = riskMatch[1].replace(/[#*`]/g, '').trim()
  }

  return {
    code,
    name: name || code,
    verdict,
    verdictText,
    coreThesis,
    keyCatalysts,
    riskFactors,
  }
}

// 消息卡片点击「存入买方认知账本」
async function handleSaveThesisFromMsg(msg: ChatMessage) {
  const content = msg.content || msg.rawContent || ''
  const thesis = extractThesisFromText(content)
  if (!thesis || !thesis.code) {
    import('tdesign-vue-next').then(({ MessagePlugin }) => {
      MessagePlugin.warning('未在分析结果中识别到明确的股票代码或买方论点')
    })
    return
  }

  try {
    const activeCfg = aiConfigStore.activeConfig
    const modelName = activeCfg?.modelName || 'Blades-Agent'
    await SaveStockThesis(
      thesis.code,
      thesis.name || thesis.code,
      thesis.verdict,
      thesis.verdictText,
      thesis.coreThesis,
      thesis.keyCatalysts,
      thesis.riskFactors,
      modelName
    )
    import('tdesign-vue-next').then(({ MessagePlugin }) => {
      MessagePlugin.success(`🎉 已将【${thesis.name || thesis.code}】买方认知(${thesis.verdictText})归档入库！`)
    })
    // 广播标的认知更新事件，驱动自选股卡片与诊断抽屉实时刷新
    EventsEmit('stockThesisUpdated', { stockCode: thesis.code })
  } catch (err: any) {
    console.error('SaveStockThesis error:', err)
    import('tdesign-vue-next').then(({ MessagePlugin }) => {
      MessagePlugin.error(`存入买方认知账本失败: ${err?.message || err}`)
    })
  }
}

// ========== 生命周期 ==========
onBeforeMount(() => {
  EventsOff('agent-message', handleAgentMessage)
  EventsOn('agent-message', handleAgentMessage)
})

async function onChatSettingsUpdated() {
  await aiConfigStore.loadConfigs()
}

onMounted(async () => {
  window.addEventListener('resize', onWindowResize)
  loadSessionsFromStorage()
  await aiConfigStore.loadConfigs()
  EventsOn('updateSettings', onChatSettingsUpdated)
  EventsOn('aiConfigsUpdated', onChatSettingsUpdated)

  refreshPerceptionSnapshot()
  perceptionTimer = setInterval(refreshPerceptionSnapshot, 60000)

  EventsOn('agent:consult', handleAgentConsult)
  if (typeof window !== 'undefined') {
    window.addEventListener('agent:consult', onWindowAgentConsult)
  }

  GetConfig().then((res) => {
    if (res.darkTheme) {
      document.documentElement.setAttribute('theme-mode', 'dark')
    } else {
      document.documentElement.removeAttribute('theme-mode')
    }
  }).catch(() => {})

  GetVersionInfo().then((res) => {
    if (res.icon) {
      messages.value.forEach(m => {
        if (m.role === 'assistant') m.avatar = res.icon
      })
    }
  }).catch(() => {})

  nextTick(() => {
    scrollToBottom(false)
  })
})

onBeforeUnmount(() => {
  if (perceptionTimer) {
    clearInterval(perceptionTimer)
    perceptionTimer = null
  }
  EventsOff('agent:consult', handleAgentConsult)
  if (typeof window !== 'undefined') {
    window.removeEventListener('agent:consult', onWindowAgentConsult)
  }
  window.removeEventListener('resize', onWindowResize)
  EventsOff('agent-message', handleAgentMessage)
  EventsOff('updateSettings', onChatSettingsUpdated)
  EventsOff('aiConfigsUpdated', onChatSettingsUpdated)
})
</script>

<style scoped lang="less">
.chat-container {
  display: flex;
  flex-direction: column;
  height: 100%;
  width: 100%;
  background: var(--td-bg-color-page, #f8f9fa);
  color: var(--td-text-color-primary, #1d2129);
  position: relative;
  overflow: hidden;
}

/* 顶部 HUD 控制面板 */
.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--td-bg-color-container, #ffffff);
  border-bottom: 1px solid var(--td-border-level-1-color, #e5e6eb);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
  z-index: 10;
  gap: 12px;

  .header-left {
    display: flex;
    align-items: center;
    gap: 8px;

    .session-drawer-btn {
      font-weight: 600;
      font-size: 14px;
      display: flex;
      align-items: center;
      gap: 6px;
      max-width: 200px;

      .session-title-text {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }

      .session-badge {
        font-size: 11px;
        background: var(--td-bg-color-secondarycontainer, #f2f3f5);
        color: var(--td-text-color-secondary, #4e5969);
        padding: 1px 6px;
        border-radius: 10px;
      }
    }
  }

  .header-center {
    display: flex;
    align-items: center;
    justify-content: center;

    .mode-capsule-group {
      display: inline-flex;
      background: var(--td-bg-color-secondarycontainer, #f2f3f5);
      padding: 3px;
      border-radius: 20px;
      gap: 2px;

      .mode-capsule-btn {
        border: none;
        background: transparent;
        padding: 4px 12px;
        font-size: 12px;
        border-radius: 16px;
        cursor: pointer;
        color: var(--td-text-color-secondary, #4e5969);
        transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
        font-weight: 500;

        &:hover {
          color: var(--td-text-color-primary, #1d2129);
        }

        &.active {
          background: var(--td-bg-color-container, #ffffff);
          color: var(--td-brand-color, #0052d9);
          box-shadow: 0 2px 4px rgba(0, 0, 0, 0.06);
          font-weight: 600;
        }
      }
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 8px;

    .model-selector {
      width: 240px;
    }
  }
}

/* 宏观态势感知 HUD 仪表条 (双端统一呈现) */
.macro-perception-hud {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: var(--td-bg-color-secondarycontainer, rgba(24, 160, 88, 0.06));
  border-bottom: 1px solid var(--td-border-level-1-color, rgba(255, 255, 255, 0.08));
  cursor: pointer;
  transition: all 0.2s ease;
  user-select: none;
  font-size: 12px;
  flex-shrink: 0;

  &:hover {
    background: var(--td-bg-color-component-hover, rgba(24, 160, 88, 0.12));

    .action-hint .arrow {
      transform: translateX(3px);
    }
  }

  .hud-left {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;

    .phase-pill {
      font-weight: 600;
      padding: 2px 8px;
      border-radius: 4px;
      background: rgba(255, 255, 255, 0.08);
      color: var(--td-text-color-primary, #e5e5e5);
    }

    .traffic-beacon {
      width: 8px;
      height: 8px;
      border-radius: 50%;
      display: inline-block;
      box-shadow: 0 0 6px currentColor;
    }

    .traffic-label {
      font-weight: 600;
      letter-spacing: 0.5px;
    }

    .breadth-ratio {
      font-size: 12px;
      margin-left: 4px;

      .up-count {
        color: #f67979;
        font-weight: 600;
      }
      .separator {
        margin: 0 4px;
        opacity: 0.4;
      }
      .down-count {
        color: #18a058;
        font-weight: 600;
      }
    }
  }

  .hud-right {
    display: flex;
    align-items: center;
    gap: 10px;

    .risk-tag {
      font-size: 11px;
    }

    .action-hint {
      display: flex;
      align-items: center;
      gap: 4px;
      color: #18a058;
      font-weight: 500;

      .arrow {
        transition: transform 0.2s ease;
      }
    }
  }
}

/* 主体消息滚动视图 (正序) */
.chat-messages-viewport {
  flex: 1;
  overflow-y: auto;
  padding: 20px 24px 30px;
  position: relative;
  scroll-behavior: smooth;

  &::-webkit-scrollbar {
    width: 6px;
  }
  &::-webkit-scrollbar-thumb {
    background: var(--td-border-level-2-color, #c9cdd4);
    border-radius: 3px;
  }
}

/* 欢迎页卡片矩阵 */
.welcome-workspace {
  max-width: 900px;
  margin: 20px auto 40px;
  display: flex;
  flex-direction: column;
  align-items: center;

  .welcome-hero {
    text-align: center;
    margin-bottom: 28px;

    .hero-icon-glow {
      font-size: 40px;
      margin-bottom: 8px;
      animation: float-glow 3s ease-in-out infinite;
    }

    .hero-title {
      font-size: 22px;
      font-weight: 700;
      margin: 0 0 8px;
      background: linear-gradient(135deg, var(--td-brand-color, #0052d9), #2ba471);
      -webkit-background-clip: text;
      -webkit-text-fill-color: transparent;
    }

    .hero-desc {
      font-size: 13px;
      color: var(--td-text-color-secondary, #86909c);
      max-width: 600px;
      line-height: 1.6;
      margin: 0 auto;
    }
  }

  .scenario-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 16px;
    width: 100%;
    margin-bottom: 24px;

    @media (max-width: 768px) {
      grid-template-columns: 1fr;
    }
  }

  .scenario-card {
    background: var(--td-bg-color-container, #ffffff);
    border: 1px solid var(--td-border-level-1-color, #e5e6eb);
    border-radius: 12px;
    padding: 16px 20px;
    cursor: pointer;
    transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.02);

    &:hover {
      border-color: var(--td-brand-color, #0052d9);
      transform: translateY(-2px);
      box-shadow: 0 8px 16px rgba(0, 82, 217, 0.08);
    }

    .card-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;

      .card-icon {
        font-size: 20px;
      }

      .card-badge {
        font-size: 11px;
        padding: 2px 8px;
        border-radius: 12px;
        background: var(--td-brand-color-light, #ecf2fe);
        color: var(--td-brand-color, #0052d9);
        font-weight: 600;
      }
    }

    .card-title {
      font-size: 15px;
      font-weight: 600;
      margin: 0 0 6px;
      color: var(--td-text-color-primary, #1d2129);
    }

    .card-desc {
      font-size: 12px;
      color: var(--td-text-color-secondary, #4e5969);
      margin: 0 0 12px;
      line-height: 1.5;
    }

    .card-prompt-hint {
      display: flex;
      justify-content: space-between;
      align-items: center;
      font-size: 12px;
      color: var(--td-text-color-placeholder, #86909c);
      padding-top: 8px;
      border-top: 1px dashed var(--td-border-level-1-color, #f2f3f5);

      .arrow-hint {
        color: var(--td-brand-color, #0052d9);
        font-weight: bold;
      }
    }
  }

  .quick-keywords-bar {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-wrap: wrap;
    font-size: 12px;
    width: 100%;
    padding: 0 8px;

    .keywords-label {
      color: var(--td-text-color-secondary, #86909c);
    }

    .keywords-list {
      display: flex;
      flex-wrap: wrap;
      gap: 8px;
    }

    .keyword-chip {
      background: var(--td-bg-color-container, #ffffff);
      border: 1px solid var(--td-border-level-2-color, #e5e6eb);
      padding: 3px 10px;
      border-radius: 14px;
      cursor: pointer;
      color: var(--td-text-color-secondary, #4e5969);
      transition: all 0.2s;

      &:hover {
        background: var(--td-brand-color-light, #ecf2fe);
        color: var(--td-brand-color, #0052d9);
        border-color: var(--td-brand-color, #0052d9);
      }
    }
  }
}

/* 消息流布局 */
.messages-flow {
  display: flex;
  flex-direction: column;
  gap: 20px;
  max-width: 980px;
  margin: 0 auto;
}

/* 回到底部悬浮球 */
.fab-scroll-bottom {
  position: fixed;
  right: 28px;
  bottom: 120px;
  background: var(--td-bg-color-container, #ffffff);
  border: 1px solid var(--td-border-level-1-color, #e5e6eb);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.12);
  border-radius: 20px;
  padding: 6px 14px;
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  color: var(--td-brand-color, #0052d9);
  font-size: 12px;
  font-weight: 600;
  z-index: 50;
  transition: all 0.2s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 6px 18px rgba(0, 82, 217, 0.2);
  }
}

/* 底部输入中枢 */
.chat-input-cockpit {
  padding: 10px 20px 16px;
  background: var(--td-bg-color-container, #ffffff);
  border-top: 1px solid var(--td-border-level-1-color, #e5e6eb);
  flex-shrink: 0;
  z-index: 20;

  &.compact {
    padding: 8px 16px 10px;
  }

  .input-quick-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
    font-size: 12px;

    .quick-chips {
      display: flex;
      gap: 6px;
      overflow-x: auto;
      padding-bottom: 2px;

      &::-webkit-scrollbar {
        height: 0;
      }
    }

    .quick-chip-btn {
      display: inline-flex;
      align-items: center;
      gap: 4px;
      background: var(--td-bg-color-secondarycontainer, #f2f3f5);
      border: 1px solid transparent;
      padding: 3px 10px;
      border-radius: 12px;
      cursor: pointer;
      font-size: 12px;
      color: var(--td-text-color-secondary, #4e5969);
      white-space: nowrap;
      transition: all 0.2s;

      &:hover {
        background: var(--td-brand-color-light, #ecf2fe);
        color: var(--td-brand-color, #0052d9);
      }
    }

    .status-summary {
      display: flex;
      align-items: center;
      gap: 6px;
      color: var(--td-brand-color, #0052d9);
      font-size: 12px;
      white-space: nowrap;

      .live-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--td-brand-color, #0052d9);
        animation: pulse-dot 1.5s infinite;
      }
    }
  }

  .input-sender-box {
    max-width: 980px;
    margin: 0 auto;

    :deep(.t-chat-sender) {
      .t-chat-sender__textarea {
        border-radius: 10px !important;
        border: 1px solid var(--td-border-level-2-color, #dcdcdc) !important;
        background: var(--td-bg-color-container, #ffffff) !important;
        font-size: 14px !important;
        line-height: 1.6 !important;
        padding: 8px 12px !important;
        transition: border-color 0.2s;

        &:focus {
          border-color: var(--td-brand-color, #0052d9) !important;
          box-shadow: 0 0 0 2px var(--td-brand-color-focus, rgba(0, 82, 217, 0.15)) !important;
        }
      }
    }

    .sender-actions {
      display: flex;
      align-items: center;
      gap: 8px;

      .stop-btn {
        font-weight: 600;
        animation: pulse-warning 2s infinite;
      }
    }
  }

  .config-alert-banner {
    max-width: 980px;
    margin: 8px auto 0;
    padding: 6px 12px;
    background: #fffbe6;
    border: 1px solid #ffe58f;
    border-radius: 6px;
    font-size: 12px;
    color: #ad6800;

    a {
      color: var(--td-brand-color, #0052d9);
      font-weight: bold;
    }
  }
}

/* 抽屉样式 */
.session-drawer-content {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 12px;

  .btn-new-chat {
    border-radius: 8px;
    font-weight: 600;
  }

  .session-list {
    flex: 1;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 6px;

    .session-item {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 10px 12px;
      border-radius: 8px;
      cursor: pointer;
      border: 1px solid transparent;
      background: var(--td-bg-color-secondarycontainer, #f2f3f5);
      transition: all 0.2s;

      &:hover {
        background: var(--td-bg-color-container, #ffffff);
        border-color: var(--td-border-level-2-color, #dcdcdc);
      }

      &.active {
        background: var(--td-brand-color-light, #ecf2fe);
        border-color: var(--td-brand-color, #0052d9);

        .session-name {
          color: var(--td-brand-color, #0052d9);
          font-weight: 600;
        }
      }

      .session-info {
        display: flex;
        flex-direction: column;
        gap: 2px;
        overflow: hidden;

        .session-name {
          font-size: 13px;
          white-space: nowrap;
          overflow: hidden;
          text-overflow: ellipsis;
        }

        .session-meta {
          font-size: 11px;
          color: var(--td-text-color-placeholder, #86909c);
        }
      }
    }
  }
}

/* 弹窗场景列表 */
.modal-scenarios-list {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  max-height: 480px;
  overflow-y: auto;

  .scenario-modal-item {
    padding: 12px 14px;
    background: var(--td-bg-color-secondarycontainer, #f8f9fa);
    border: 1px solid var(--td-border-level-1-color, #e5e6eb);
    border-radius: 8px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      border-color: var(--td-brand-color, #0052d9);
      background: var(--td-brand-color-light, #ecf2fe);
    }

    .item-head {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 6px;

      .item-tag {
        font-size: 11px;
        background: var(--td-brand-color, #0052d9);
        color: #ffffff;
        padding: 1px 6px;
        border-radius: 4px;
      }

      .item-title {
        font-size: 14px;
        font-weight: 600;
      }
    }

    .item-desc {
      font-size: 12px;
      color: var(--td-text-color-secondary, #4e5969);
      margin: 0;
      line-height: 1.5;
    }
  }
}

/* 动画定义 */
@keyframes pulse-dot {
  0% { transform: scale(0.95); opacity: 0.6; }
  50% { transform: scale(1.3); opacity: 1; }
  100% { transform: scale(0.95); opacity: 0.6; }
}

@keyframes float-glow {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-4px); }
}

@keyframes pulse-warning {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.75; }
}

/* -------------------------------------------------------------------------
   移动端 (Screen <= 768px) 深度触控与全景响应式适配
   ------------------------------------------------------------------------- */
@media screen and (max-width: 768px) {
  .chat-header {
    flex-wrap: wrap !important;
    gap: 6px !important;
    padding: 6px 10px !important;

    .header-left {
      order: 1;
      .session-drawer-btn {
        max-width: 130px !important;
        font-size: 12px !important;
      }
    }

    .header-right {
      order: 2;
      margin-left: auto;
      gap: 4px !important;
      .model-selector {
        width: 140px !important;
        max-width: 160px !important;
      }
    }

    .header-center {
      order: 3;
      width: 100% !important;
      overflow-x: auto !important;
      justify-content: flex-start !important;
      padding-top: 2px;
      -webkit-overflow-scrolling: touch;

      &::-webkit-scrollbar {
        height: 0;
      }

      .mode-capsule-group {
        width: 100%;
        justify-content: space-between;
        .mode-capsule-btn {
          flex: 1;
          text-align: center;
          padding: 3px 6px !important;
          font-size: 11px !important;
        }
      }
    }
  }

  .chat-messages-viewport {
    padding: 10px 8px 20px !important;
  }

  .welcome-workspace {
    margin: 10px auto 20px !important;
    .welcome-hero {
      margin-bottom: 16px !important;
      .hero-title {
        font-size: 18px !important;
      }
    }
  }

  .fab-scroll-bottom {
    right: 14px !important;
    bottom: 90px !important;
    padding: 4px 10px !important;
    font-size: 11px !important;
  }

  .chat-input-cockpit {
    padding: 6px 10px max(10px, var(--sab, 0px)) !important;

    .input-quick-bar {
      margin-bottom: 6px !important;
      .quick-chips {
        gap: 4px !important;
      }
      .quick-chip-btn {
        padding: 2px 8px !important;
        font-size: 11px !important;
      }
    }
  }
}
</style>
