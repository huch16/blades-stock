<script setup>
import { ref, watch, computed, onMounted, onBeforeUnmount } from 'vue'
import {
  NDrawer,
  NDrawerContent,
  NSpin,
  NCard,
  NFlex,
  NTag,
  NDivider,
  NEmpty,
  NGrid,
  NGridItem,
  NModal,
  NStatistic,
  NProgress,
  NAlert,
  NButton,
  NTimeline,
  NTimelineItem,
  useMessage
} from 'naive-ui'
import { DiagnoseStock, GetStockThesis, GetStockThesisHistory } from '../../wailsjs/go/main/App'
import { EventsEmit, EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { setCurrentStock } from '../utils/stockContextBus'
import { consultAgent } from '../utils/agentControlBus'

const props = defineProps({
  show: { type: Boolean, default: false },
  stockCode: { type: String, default: '' },
  stockName: { type: String, default: '' }
})

const emit = defineEmits(['update:show'])

const message = useMessage()
const loading = ref(false)
const diagnosisData = ref(null)
const activeCode = ref(props.stockCode)
const activeName = ref(props.stockName)
const recentStocks = ref([])

// 买方认知档案
const thesisData = ref(null)
const thesisHistoryList = ref([])
const showThesisHistory = ref(false)

function getVerdictType(verdict) {
  switch (verdict) {
    case 'THESIS_STRENGTHENED':
      return 'success'
    case 'NEW_THESIS':
      return 'info'
    case 'THESIS_WEAKENED':
      return 'warning'
    case 'THESIS_BROKEN':
      return 'error'
    default:
      return 'default'
  }
}

async function openThesisHistory(code) {
  const target = code || activeCode.value || props.stockCode
  if (!target) return
  try {
    const list = await GetStockThesisHistory(target, 30)
    thesisHistoryList.value = list || []
    showThesisHistory.value = true
  } catch (err) {
    message.error('获取认知历史失败: ' + (err?.message || err))
  }
}

// 呼叫全屏 Agent 深度推演（跨组件上下文穿透）
function callAgentForDeepAnalysis() {
  const code = activeCode.value || props.stockCode
  const name = diagnosisData.value?.stockName || activeName.value || props.stockName || code
  if (!code) return

  let contextText = `标的代码: ${code}, 标的名称: ${name}\n`
  if (diagnosisData.value) {
    const d = diagnosisData.value
    contextText += `量化综合评分: ${d.totalScore || '未知'}分\n`
    if (d.advice) contextText += `量化操作建议: ${d.advice}\n`
    if (d.supportPrice || d.resistancePrice) {
      contextText += `技术关键位: 支撑位 ${d.supportPrice || '--'} / 阻力位 ${d.resistancePrice || '--'}\n`
    }
  }
  if (thesisData.value) {
    const t = thesisData.value
    contextText += `当前买方认知判定: 【${t.verdictText || t.verdict || '无'}】\n`
    if (t.coreThesis) contextText += `原核心假说: ${t.coreThesis}\n`
    if (t.keyCatalysts) contextText += `原核心催化: ${t.keyCatalysts}\n`
    if (t.riskFactors) contextText += `原核心证伪风险: ${t.riskFactors}\n`
  }

  const question = `请基于最新的盘口资金、均线量价形态与基本面驱动因素，对【${name} (${code})】进行全维度买方深度推演。重点评估：1. 当前买方逻辑是维持、强化还是证伪；2. 关键催化剂兑现窗口与潜在黑天鹅风险；3. 给出严格的止损防守位与胜率赔率测算。`

  handleClose()
  consultAgent({
    question,
    stockCode: code,
    stockName: name,
    context: contextText,
    mode: 'plan_execute',
    autoSend: true,
  })
}

// 幂等防抖：同一股票 30 秒内不重复请求
const diagnosing = ref(false)
const lastDiagnosisTime = new Map()

function loadRecentStocks() {
  try {
    const raw = localStorage.getItem('kline-recent-stocks')
    if (raw) {
      recentStocks.value = JSON.parse(raw).slice(0, 5)
    }
  } catch {}
}

watch(
  () => props.stockCode,
  (newCode) => {
    if (newCode) {
      activeCode.value = newCode
      activeName.value = props.stockName
    }
  }
)

watch(
  () => props.show,
  async (visible) => {
    if (visible) {
      loadRecentStocks()
      if (props.stockCode) {
        activeCode.value = props.stockCode
        activeName.value = props.stockName
      }
      if (activeCode.value) {
        setCurrentStock({ code: activeCode.value, name: activeName.value })
        await fetchDiagnosis(activeCode.value)
      }
    }
  }
)

async function switchStock(code, name) {
  if (!code || code === activeCode.value) return
  activeCode.value = code
  activeName.value = name || ''
  setCurrentStock({ code, name })
  await fetchDiagnosis(code)
}

async function fetchDiagnosis(targetCode) {
  const code = targetCode || activeCode.value || props.stockCode
  if (!code) return

  // 前端幂等：30 秒内同一股票不重复请求
  const now = Date.now()
  const last = lastDiagnosisTime.get(code) || 0
  if (now - last < 30000) {
    console.log('[Diagnosis] 跳过重复请求:', code, '距上次', (now - last) / 1000, '秒')
    return
  }
  if (diagnosing.value) {
    console.log('[Diagnosis] 正在诊断中，跳过:', code)
    return
  }

  lastDiagnosisTime.set(code, now)
  diagnosing.value = true
  loading.value = true

  // 并发拉取买方认知假说档案
  GetStockThesis(code).then(res => {
    thesisData.value = res || null
  }).catch(() => {
    thesisData.value = null
  })

  try {
    const res = await DiagnoseStock(code)
    if (res) {
      diagnosisData.value = res
      if (res.stockName) activeName.value = res.stockName
    } else {
      message.error('量化诊断返回空数据')
    }
  } catch (err) {
    console.error('DiagnoseStock error:', err)
    message.error('量化诊断执行失败: ' + (err?.message || err))
  } finally {
    loading.value = false
    diagnosing.value = false
  }
}

function handleClose() {
  emit('update:show', false)
}

function askAgentAssistant() {
  handleClose()
  const targetName = diagnosisData.value?.stockName || activeName.value || props.stockName || ''
  const targetCode = diagnosisData.value?.stockCode || activeCode.value || props.stockCode || ''
  const score = diagnosisData.value?.overallScore || ''
  const suggestion = diagnosisData.value?.suggestion || ''
  const q = `请针对 ${targetName} (${targetCode}) 结合当前三维量化诊断（综合评分 ${score}分，建议 ${suggestion}），详细分析近期走势、买卖关键点位以及操作建议。`
  EventsEmit('openFloatingAgentWithQuestion', q)
}

// 移动端响应式感知与抽屉尺寸适配
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)
function handleDrawerResize() {
  isMobile.value = typeof window !== 'undefined' ? window.innerWidth <= 768 : false
}
const drawerWidth = computed(() => {
  return isMobile.value ? '100%' : 540
})

function onThesisUpdated(payload) {
  const code = activeCode.value || props.stockCode
  if (!code) return
  if (!payload || !payload.stockCode || payload.stockCode === code) {
    GetStockThesis(code).then(res => {
      thesisData.value = res || null
    }).catch(() => {})
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('resize', handleDrawerResize)
    handleDrawerResize()
  }
  EventsOn('stockThesisUpdated', onThesisUpdated)
})

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('resize', handleDrawerResize)
  }
  EventsOff('stockThesisUpdated', onThesisUpdated)
})
</script>

<template>
  <NDrawer
    :show="show"
    :width="drawerWidth"
    placement="right"
    :trap-focus="false"
    @update:show="handleClose"
  >
    <NDrawerContent
      :title="`✨ ${diagnosisData?.stockName || activeName || stockName} (${diagnosisData?.stockCode || activeCode || stockCode}) 量化三维诊断`"
      closable
    >
      <!-- 快速历史对比切换条与深度推演操作 -->
      <div
        style="margin-bottom: 12px; display: flex; align-items: center; justify-content: space-between; gap: 8px; flex-wrap: wrap;"
      >
        <div
          v-if="recentStocks.length > 1"
          style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap;"
        >
          <span style="font-size: 11px; color: #888;">快速切换标的:</span>
          <NButton
            v-for="s in recentStocks"
            :key="s.code"
            size="tiny"
            quaternary
            :type="s.code === activeCode ? 'primary' : 'default'"
            @click="switchStock(s.code, s.name)"
          >
            {{ s.name || s.code }}
          </NButton>
        </div>
        <div style="margin-left: auto;">
          <NButton
            type="primary"
            size="small"
            secondary
            style="font-weight: 600;"
            @click="callAgentForDeepAnalysis"
          >
            🤖 呼叫 Agent 深度推演
          </NButton>
        </div>
      </div>

      <div
        v-if="loading"
        style="display: flex; justify-content: center; align-items: center; height: 300px;"
      >
        <NSpin
          size="large"
          description="正在调取 VectorBT 回测引擎、资金筹码及技术形态..."
        />
      </div>

      <div
        v-else-if="diagnosisData"
        class="diagnosis-container"
      >
        <!-- 🎯 买方认知假说与演化档案 -->
        <NCard
          v-if="thesisData && thesisData.verdictText"
          size="small"
          embedded
          class="thesis-card"
          style="margin-bottom: 10px; border-left: 4px solid #18a058; background: rgba(24, 160, 88, 0.05);"
        >
          <template #header>
            <NFlex
              justify="space-between"
              align="center"
            >
              <NFlex
                align="center"
                :size="8"
              >
                <span style="font-weight: 700; font-size: 13px;">🎯 买方投资假说 (Thesis Ledger)</span>
                <NTag
                  size="small"
                  round
                  :bordered="false"
                  :type="getVerdictType(thesisData.verdict)"
                >
                  {{ thesisData.verdictText }}
                </NTag>
              </NFlex>
              <NButton
                size="tiny"
                secondary
                type="primary"
                @click="openThesisHistory(activeCode)"
              >
                📈 认知演化
              </NButton>
            </NFlex>
          </template>
          <div style="font-size: 12px; line-height: 1.6; margin-bottom: 6px;">
            <strong style="color: #18a058;">核心假说：</strong>{{ thesisData.coreThesis }}
          </div>
          <NFlex
            v-if="thesisData.keyCatalysts || thesisData.riskFactors"
            vertical
            :size="4"
            style="font-size: 11px;"
          >
            <div
              v-if="thesisData.keyCatalysts"
              style="color: #f0a020;"
            >
              <strong>跟踪催化：</strong>{{ thesisData.keyCatalysts }}
            </div>
            <div
              v-if="thesisData.riskFactors"
              style="color: #d03050;"
            >
              <strong>证伪条件：</strong>{{ thesisData.riskFactors }}
            </div>
          </NFlex>
        </NCard>

        <!-- 核心评分卡片 -->
        <NCard
          size="small"
          embedded
          class="score-card"
        >
          <NFlex
            justify="space-between"
            align="center"
          >
            <div>
              <div class="score-label">
                综合量化评分
              </div>
              <div
                class="score-value"
                :class="diagnosisData.overallScore >= 70 ? 'bull' : (diagnosisData.overallScore <= 40 ? 'bear' : 'neutral')"
              >
                {{ diagnosisData.overallScore }} <span class="score-max">/ 100</span>
              </div>
            </div>
            <div style="text-align: right">
              <NTag
                :type="diagnosisData.overallScore >= 70 ? 'success' : (diagnosisData.overallScore <= 40 ? 'error' : 'warning')"
                size="large"
                round
                style="font-weight: 700; font-size: 14px;"
              >
                {{ diagnosisData.suggestion }}
              </NTag>
              <div style="margin-top: 6px; font-size: 12px; color: #888;">
                历史回测置信度: <strong style="color: #4098fc">{{ diagnosisData.confidence }}%</strong>
              </div>
            </div>
          </NFlex>

          <NDivider style="margin: 10px 0 8px 0;" />

          <!-- 关键点位建议 -->
          <NGrid
            :cols="4"
            :x-gap="8"
          >
            <NGridItem>
              <NStatistic
                label="当前价格"
                :value="diagnosisData.currentPrice.toFixed(2)"
              />
            </NGridItem>
            <NGridItem>
              <NStatistic
                label="第一目标价"
                :value="diagnosisData.targetPrice.toFixed(2)"
              >
                <template #suffix>
                  <span style="font-size: 10px; color: #18a058;">
                    (+{{ (((diagnosisData.targetPrice - diagnosisData.currentPrice) / diagnosisData.currentPrice) * 100).toFixed(1) }}%)
                  </span>
                </template>
              </NStatistic>
            </NGridItem>
            <NGridItem>
              <NStatistic
                label="建议止损价"
                :value="diagnosisData.stopLossPrice.toFixed(2)"
              >
                <template #suffix>
                  <span style="font-size: 10px; color: #d03050;">
                    ({{ (((diagnosisData.stopLossPrice - diagnosisData.currentPrice) / diagnosisData.currentPrice) * 100).toFixed(1) }}%)
                  </span>
                </template>
              </NStatistic>
            </NGridItem>
            <NGridItem>
              <NStatistic
                label="盈亏比"
                :value="diagnosisData.riskRewardRatio.toFixed(2)"
              />
            </NGridItem>
          </NGrid>
        </NCard>

        <!-- 维度1: 量化回测维度 -->
        <NCard
          size="small"
          class="dim-card"
          style="margin-top: 10px"
        >
          <template #header>
            <NFlex
              justify="space-between"
              align="center"
            >
              <span>📊 量化回测验证 (VectorBT)</span>
              <span style="font-weight: 700; color: #2080f0;">{{ diagnosisData.quantDim.score }} 分</span>
            </NFlex>
          </template>
          <NProgress
            type="line"
            :percentage="diagnosisData.quantDim.score"
            :color="diagnosisData.quantDim.score >= 60 ? '#18a058' : '#e06c75'"
            :show-indicator="false"
            style="margin-bottom: 8px;"
          />
          <div style="font-size: 12px; color: #666; margin-bottom: 6px;">
            策略: {{ diagnosisData.quantDim.strategyName }} (交易样本: {{ diagnosisData.quantDim.totalTrades }} 次)
          </div>
          <NGrid
            :cols="3"
            :x-gap="6"
            style="margin-bottom: 8px"
          >
            <NGridItem>
              <div class="mini-stat-label">
                历史胜率
              </div>
              <div class="mini-stat-value">
                {{ diagnosisData.quantDim.winRatePct }}%
              </div>
            </NGridItem>
            <NGridItem>
              <div class="mini-stat-label">
                夏普比率
              </div>
              <div class="mini-stat-value">
                {{ diagnosisData.quantDim.sharpeRatio }}
              </div>
            </NGridItem>
            <NGridItem>
              <div class="mini-stat-label">
                最大回撤
              </div>
              <div
                class="mini-stat-value"
                style="color: #d03050"
              >
                {{ diagnosisData.quantDim.maxDrawdownPct }}%
              </div>
            </NGridItem>
          </NGrid>
          <div class="dim-evaluation">
            {{ diagnosisData.quantDim.evaluation }}
          </div>
        </NCard>

        <!-- 维度2: 资金筹码维度 -->
        <NCard
          size="small"
          class="dim-card"
          style="margin-top: 10px"
        >
          <template #header>
            <NFlex
              justify="space-between"
              align="center"
            >
              <span>💰 资金筹码博弈</span>
              <span style="font-weight: 700; color: #2080f0;">{{ diagnosisData.moneyDim.score }} 分</span>
            </NFlex>
          </template>
          <NProgress
            type="line"
            :percentage="diagnosisData.moneyDim.score"
            :color="diagnosisData.moneyDim.score >= 60 ? '#18a058' : '#e06c75'"
            :show-indicator="false"
            style="margin-bottom: 8px;"
          />
          <NGrid
            :cols="2"
            :x-gap="6"
            style="margin-bottom: 8px"
          >
            <NGridItem>
              <div class="mini-stat-label">
                近5日主力净买入
              </div>
              <div
                class="mini-stat-value"
                :class="diagnosisData.moneyDim.mainNetInflow5D >= 0 ? 'bull' : 'bear'"
              >
                {{ diagnosisData.moneyDim.mainNetInflow5D }} 万元
              </div>
            </NGridItem>
            <NGridItem>
              <div class="mini-stat-label">
                超大单净占比
              </div>
              <div class="mini-stat-value">
                {{ diagnosisData.moneyDim.superLargeNetPct }}%
              </div>
            </NGridItem>
          </NGrid>
          <div style="font-size: 12px; margin-bottom: 4px;">
            <NTag
              size="tiny"
              type="info"
            >
              {{ diagnosisData.moneyDim.recentInflowTrend }}
            </NTag>
            <NTag
              size="tiny"
              type="default"
              style="margin-left: 6px;"
            >
              {{ diagnosisData.moneyDim.chipConcentration }}
            </NTag>
          </div>
          <div class="dim-evaluation">
            {{ diagnosisData.moneyDim.evaluation }}
          </div>
        </NCard>

        <!-- 维度3: 技术形态维度 -->
        <NCard
          size="small"
          class="dim-card"
          style="margin-top: 10px"
        >
          <template #header>
            <NFlex
              justify="space-between"
              align="center"
            >
              <span>📈 技术与波动形态</span>
              <span style="font-weight: 700; color: #2080f0;">{{ diagnosisData.technicalDim.score }} 分</span>
            </NFlex>
          </template>
          <NProgress
            type="line"
            :percentage="diagnosisData.technicalDim.score"
            :color="diagnosisData.technicalDim.score >= 60 ? '#18a058' : '#e06c75'"
            :show-indicator="false"
            style="margin-bottom: 8px;"
          />
          <div style="font-size: 12px; color: #666; margin-bottom: 6px;">
            均线形态: <strong>{{ diagnosisData.technicalDim.trend }}</strong> | ATR(14): {{ diagnosisData.technicalDim.atr }}
          </div>
          <div class="dim-evaluation">
            {{ diagnosisData.technicalDim.evaluation }}
          </div>
        </NCard>

        <!-- 历史战绩与反思记录 -->
        <NCard
          size="small"
          class="dim-card"
          style="margin-top: 10px; background: rgba(32, 128, 240, 0.04);"
        >
          <template #header>
            <span style="font-size: 13px;">🧠 AI 历史预测复盘闭环</span>
          </template>
          <div style="font-size: 12px; line-height: 1.6;">
            {{ diagnosisData.historyTrackRecord }}
          </div>
        </NCard>

        <!-- 风险提示 -->
        <div
          v-if="diagnosisData.risks && diagnosisData.risks.length > 0"
          style="margin-top: 10px"
        >
          <NAlert
            title="关键风险排查"
            type="warning"
            :show-icon="true"
          >
            <ul style="margin: 0; padding-left: 16px; font-size: 12px;">
              <li
                v-for="(r, idx) in diagnosisData.risks"
                :key="idx"
              >
                {{ r }}
              </li>
            </ul>
          </NAlert>
        </div>

        <!-- 底部操作按钮 -->
        <div style="margin-top: 16px; text-align: center;">
          <NButton
            type="primary"
            block
            size="medium"
            @click="askAgentAssistant"
          >
            💬 与 AI 投研助手深度探讨
          </NButton>
        </div>
      </div>
    </NDrawerContent>
  </NDrawer>

  <!-- 认知演化历史时间线弹窗 -->
  <NModal
    v-model:show="showThesisHistory"
    preset="card"
    style="width: 580px; max-width: calc(100vw - 32px); text-align: left;"
    title="📈 买方认知演进时间线"
  >
    <div style="max-height: 55vh; overflow-y: auto; padding-right: 8px;">
      <NEmpty
        v-if="!thesisHistoryList || thesisHistoryList.length === 0"
        description="暂无历史认知演化记录"
      />
      <NTimeline v-else>
        <NTimelineItem
          v-for="item in thesisHistoryList"
          :key="item.id"
          :type="getVerdictType(item.verdict)"
          :title="item.verdictText || item.verdict"
          :time="new Date(item.createdAt).toLocaleString()"
        >
          <div style="font-size: 12px; margin-top: 4px; line-height: 1.5;">
            <div
              v-if="item.coreThesis"
              style="margin-bottom: 2px;"
            >
              <strong>核心逻辑：</strong>{{ item.coreThesis }}
            </div>
            <div
              v-if="item.keyCatalysts"
              style="color: #f0a020; margin-bottom: 2px;"
            >
              <strong>验证催化：</strong>{{ item.keyCatalysts }}
            </div>
            <div
              v-if="item.riskFactors"
              style="color: #d03050; margin-bottom: 2px;"
            >
              <strong>证伪条件：</strong>{{ item.riskFactors }}
            </div>
            <div style="font-size: 11px; color: #888; margin-top: 2px;">
              模型: {{ item.modelName || '默认' }} · 触发: {{ item.triggerEvent || '系统' }}
            </div>
          </div>
        </NTimelineItem>
      </NTimeline>
    </div>
  </NModal>
</template>

<style scoped>
.diagnosis-container {
  padding: 4px;
}
.score-card {
  border-radius: 8px;
}
.score-label {
  font-size: 13px;
  color: #888;
}
.score-value {
  font-size: 28px;
  font-weight: 800;
  line-height: 1.2;
}
.score-max {
  font-size: 13px;
  font-weight: 400;
  color: #999;
}
.bull {
  color: #18a058;
}
.bear {
  color: #d03050;
}
.neutral {
  color: #f0a020;
}
.mini-stat-label {
  font-size: 11px;
  color: #888;
}
.mini-stat-value {
  font-size: 14px;
  font-weight: 600;
}
.dim-card {
  border-radius: 8px;
}
.dim-evaluation {
  font-size: 12px;
  color: #555;
  background: rgba(0, 0, 0, 0.03);
  padding: 6px 8px;
  border-radius: 4px;
  line-height: 1.5;
}
</style>
