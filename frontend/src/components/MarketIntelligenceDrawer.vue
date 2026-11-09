<script setup>
import { computed, defineAsyncComponent, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NModal,
  NSpin,
  NTag,
  NButton,
  NSpace,
  NEmpty,
  NInput,
  NSelect,
  useMessage,
  useDialog,
  useThemeVars
} from 'naive-ui'
import {
  GeneratePreMarketBriefing,
  GeneratePostMarketReview,
  GenerateETFRotationReport,
  GenerateSerenityResearchReport,
  GetLatestMarketIntelligenceReport,
  GetLatestMarketIntelligenceReportByType,
  GetMarketLifecycleStatus,
  GetAiConfigs,
  GetConfig,
  PurgeCorruptedMarketReports,
  ClearMarketIntelligenceReport,
  SaveStockThesis
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime'

// 异步按需加载 Markdown 预览渲染引擎
const MdPreview = defineAsyncComponent(() => import('md-editor-v3').then(m => {
  import('md-editor-v3/lib/preview.css')
  return m.MdPreview
}))

import { sanitizeMarkdownHtml } from '../utils/sanitize'
import { dispatchAgentAction, consultAgent } from '../utils/agentControlBus'

const props = defineProps({
  show: { type: Boolean, default: false },
  reportType: { type: String, default: 'pre_market_briefing' },
  initialSerenityTarget: { type: String, default: '' },
  embedded: { type: Boolean, default: false }
})

const emit = defineEmits(['update:show', 'changeTab'])

const router = useRouter()
const message = useMessage()
const dialog = useDialog()
const themeVars = useThemeVars()
const loading = ref(false)
const currentTab = ref(props.reportType || 'pre_market_briefing')
const lifecycle = ref(null)
const aiConfigs = ref([])
const selectedAiConfigId = ref(0)
const aiConfigReady = ref(false)
const darkConfig = ref(false)

// 响应式判断暗黑模式（结合 Naive UI themeVars 与 系统配置）
const isDark = computed(() => {
  if (darkConfig.value) return true
  const bg = themeVars.value?.cardColor || themeVars.value?.bodyColor || ''
  if (bg && (bg.startsWith('#1') || bg.startsWith('#0') || bg.startsWith('#2'))) {
    return true
  }
  if (typeof document !== 'undefined') {
    return (
      document.documentElement.getAttribute('theme-mode') === 'dark' ||
      document.documentElement.classList.contains('dark')
    )
  }
  return false
})

async function loadTheme() {
  try {
    const config = await GetConfig()
    darkConfig.value = !!config?.darkTheme
  } catch (_e) {
    darkConfig.value = false
  }
}

// 针对不同标签页缓存已生成的报告
const reports = ref({
  pre_market_briefing: null,
  post_market_review: null,
  etf_rotation_decision: null,
  serenity_industry_analysis: null
})

// Serenity 产业链分析输入参数
const serenityTarget = ref(props.initialSerenityTarget || '半导体')
const serenityCode = ref('')
const serenityPresets = [
  '半导体与集成电路', '人工智能算力与CPO', '创新药与生物医药',
  '高端制造与工业母机', '低空经济与商业航天', '新能源与先进储能',
  '大消费与白酒食品', '银行与高股息红利', '核心资源品与贵金属'
]

const navTabs = [
  { key: 'pre_market_briefing', icon: '🌅', title: '盘前前瞻', desc: '隔夜外盘 · 竞价风向' },
  { key: 'post_market_review', icon: '🌆', title: '盘后复盘', desc: '全景量价 · 次日推演' },
  { key: 'etf_rotation_decision', icon: '📈', title: 'ETF轮动决策', desc: '21日动量 · 矩阵评分' },
  { key: 'serenity_industry_analysis', icon: '🔍', title: '产业深度拆解', desc: 'Serenity · 瓶颈挖掘' }
]

const currentReport = computed(() => {
  return reports.value[currentTab.value] || null
})

const reportActions = computed(() => {
  return currentReport.value?.actions || []
})

function getActionDesc(act) {
  if (!act) return '系统联动'
  switch (act.type) {
    case 'OPEN_DIAGNOSIS':
      return `🎯 3D量化诊断: ${act.payload?.name || act.payload?.code}`
    case 'SHOW_KLINE':
      return `📈 查看K线: ${act.payload?.name || act.payload?.code}`
    case 'NAVIGATE_VIEW':
      return `🧭 页面跳转: ${act.payload?.path}`
    case 'OPEN_MARKET_INTELLIGENCE':
      return '📑 切换研报'
    case 'DISCIPLINE_ALERT':
      return `⚠️ 风控警示: ${act.payload?.title || ''}`
    default:
      return act.type
  }
}

function handleExecuteAction(act) {
  dispatchAgentAction(act)
  message.success('已触发系统联动！')
}

// 严密判断研报是否异常损坏或包含未完成中断信令
function isCorruptedReport(rep) {
  if (!rep || !rep.content) return false
  const raw = String(rep.content).trim()
  if (raw.length < 100) return true
  if (
    raw.includes('❌') ||
    raw.includes('Agent 调用失败') ||
    raw.includes('context canceled') ||
    raw.includes('context deadline exceeded') ||
    raw.includes('出现错误：502') ||
    raw.includes('502 Bad Gateway') ||
    raw.includes('bad gateway') ||
    raw.includes('unexpected EOF') ||
    raw.includes('unexpected end of JSON input') ||
    // 网关限流 / 账号池空 / 容灾降级：这些是**链路遥测**，不是研报正文。
    // 用户截图实证：报告位置只剩一行“容灾切换：主模型 … 429 Token pool is empty”，
    // 却因为长度够而被当成了有效研报展示。
    raw.includes('Token pool is empty') ||
    raw.includes('账号池') ||
    raw.includes('额度耗尽') ||
    raw.includes('网关限流') ||
    raw.includes('容灾切换')
  ) {
    return true
  }
  // 遥测行（[STEP]/[STATUS]/[DONE]）不构成正文：剔除后所剩无几即为残卷
  const withoutTelemetry = raw
    .split('\n')
    .filter(l => !/^\s*\[(STEP|STATUS|DONE)\]/.test(l))
    .join('\n')
    .trim()
  if (withoutTelemetry.length < 100) return true
  return false
}

const isReportCorrupted = computed(() => {
  return isCorruptedReport(currentReport.value)
})

// 清理旧缓存或流式残留中的 [STATUS] 杂质
const displayContent = computed(() => {
  if (isReportCorrupted.value) return ''
  const raw = currentReport.value?.content || ''
  if (!raw) return ''
  return raw
    .split('\n')
    .filter(line => {
      const trimmed = line.trim()
      return !trimmed.startsWith('[STATUS]') && !trimmed.includes('[STATUS]🤔') && !trimmed.includes('[STATUS]✍️')
    })
    .join('\n')
    .trim()
})

const currentTitle = computed(() => {
  const found = navTabs.find(t => t.key === currentTab.value)
  return found ? `${found.icon} ${found.title}` : '智能市场研报'
})

// 针对 Serenity 研报自动解析结构化买方认知假说 (Thesis)，实现从研报到认知账本的直通沉淀
const extractedThesis = computed(() => {
  if (currentTab.value !== 'serenity_industry_analysis') return null
  const content = displayContent.value
  if (!content) return null
  const match = content.match(/```thesis\s*\{?([\s\S]*?)\}?\s*```/)
  if (!match || !match[1]) return null
  try {
    const raw = '{' + match[1].trim() + '}'
    const parsed = JSON.parse(raw)
    if (!parsed || (!parsed.thesis && !parsed.stockCode)) return null
    return {
      stockCode: parsed.stockCode || serenityCode.value || '',
      stockName: parsed.stockName || serenityTarget.value || '',
      thesis: parsed.thesis || '',
      verdict: parsed.verdict || 'OBSERVE',
      evidenceChain: parsed.evidenceChain || '',
      researchGaps: parsed.researchGaps || '',
      counterarguments: parsed.counterarguments || '',
      trackingChecklist: parsed.trackingChecklist || ''
    }
  } catch (_e) {
    return null
  }
})

const isSavingThesis = ref(false)
async function handleSaveThesisToLedger() {
  const t = extractedThesis.value
  if (!t || !t.stockCode) {
    message.warning('请先输入有效的股票代码或在研报中指定标的')
    return
  }
  isSavingThesis.value = true
  try {
    const activeCfg = aiConfigs.value?.find(c => (c.ID || c.id) === selectedAiConfigId.value)
    const modelName = activeCfg?.modelName || activeCfg?.ModelName || 'Serenity-Skill4'
    let verdictCode = 'NEW_THESIS'
    let verdictText = '观望/新建档'
    const vUpper = (t.verdict || '').toUpperCase()
    if (vUpper.includes('BUY') || vUpper.includes('买入') || vUpper.includes('多')) {
      verdictCode = 'THESIS_STRENGTHENED'
      verdictText = '买入/逻辑强化'
    } else if (vUpper.includes('AVOID') || vUpper.includes('规避') || vUpper.includes('离场')) {
      verdictCode = 'THESIS_BROKEN'
      verdictText = '规避/逻辑证伪'
    }

    const keyCat = `【证据天梯】${t.evidenceChain}\n【跟踪清单】${t.trackingChecklist}`
    const riskFac = `【研究缺口】${t.researchGaps}\n【证伪条件】${t.counterarguments}`

    await SaveStockThesis(
      t.stockCode,
      t.stockName || t.stockCode,
      verdictCode,
      verdictText,
      t.thesis,
      keyCat,
      riskFac,
      modelName
    )
    message.success(`🎉 成功将【${t.stockName || t.stockCode}】买方认知假说沉淀入库！`)
    EventsEmit('stockThesisUpdated', { stockCode: t.stockCode })
  } catch (err) {
    console.error('SaveStockThesis failed:', err)
    message.error(`沉淀买方认知失败: ${err?.message || err}`)
  } finally {
    isSavingThesis.value = false
  }
}

const phaseTagType = computed(() => {
  const phase = lifecycle.value?.phase
  if (!phase) return 'default'
  if (phase === 'pre_market') return 'info'
  if (phase === 'morning_trading' || phase === 'afternoon_trading') return 'success'
  if (phase === 'post_market') return 'warning'
  if (phase === 'closed') return 'default'
  return 'default'
})

const aiConfigOptions = computed(() => {
  const list = Array.isArray(aiConfigs.value) ? aiConfigs.value : []
  return list.map((c, index) => {
    const id = c.ID != null ? Number(c.ID) : (c.id != null ? Number(c.id) : index + 1)
    const name = c.name || c.Name || `配置 #${id}`
    const modelName = c.modelName || c.ModelName || ''
    return {
      label: modelName ? `${name} [${modelName}]` : name,
      value: id
    }
  })
})

watch(
  () => props.reportType,
  (val) => {
    if (val) {
      currentTab.value = val
    }
  }
)

watch(
  () => props.initialSerenityTarget,
  (val) => {
    if (val) {
      serenityTarget.value = val
    }
  }
)

watch(
  () => props.show,
  async (visible) => {
    if (visible) {
      if (props.reportType) {
        currentTab.value = props.reportType
      }
      if (props.initialSerenityTarget) {
        serenityTarget.value = props.initialSerenityTarget
      }
      await loadTheme()
      await loadAiConfigs()
      await refreshAll()
    }
  }
)

async function loadAiConfigs(preferredId) {
  try {
    const configs = await GetAiConfigs()
    aiConfigs.value = Array.isArray(configs) ? configs : []

    const validIds = aiConfigs.value.map(c => (c.ID != null ? Number(c.ID) : Number(c.id)))
    if (preferredId != null && validIds.includes(Number(preferredId))) {
      selectedAiConfigId.value = Number(preferredId)
    } else {
      const saved = localStorage.getItem('market_intelligence_ai_config_id')
      const savedId = saved ? parseInt(saved, 10) : 0
      if (savedId && validIds.includes(savedId)) {
        selectedAiConfigId.value = savedId
      } else if (validIds.length > 0) {
        selectedAiConfigId.value = validIds[0]
      }
    }
    aiConfigReady.value = true
  } catch (e) {
    console.error('GetAiConfigs error:', e)
  }
}

watch(selectedAiConfigId, (newId) => {
  if (newId) {
    localStorage.setItem('market_intelligence_ai_config_id', String(newId))
  }
})

function goToSettings() {
  emit('update:show', false)
  if (router) {
    router.push({ name: 'settings' })
  }
}

async function refreshAll() {
  await Promise.all([loadLatestForTab(currentTab.value), loadLifecycle()])
}

async function loadLatestForTab(tabKey) {
  try {
    let report = null
    if (typeof GetLatestMarketIntelligenceReportByType === 'function') {
      report = await GetLatestMarketIntelligenceReportByType(tabKey)
    }
    if (!report) {
      const latest = await GetLatestMarketIntelligenceReport()
      if (latest && latest.type === tabKey) {
        report = latest
      }
    }
    if (report && isCorruptedReport(report)) {
      report = null
    }
    reports.value[tabKey] = report
  } catch (e) {
    console.warn('loadLatestForTab error:', e)
  }
}

/**
 * 清除市场智能研报（内存缓存 + 归档记录）。
 *
 * 之前**完全没有**这个能力：研报只活在服务端进程内存里并通过 SSE 广播，
 * 而抽屉挂载/切页会主动拉取“最新研报”，所以一份早该作废的旧报告会被反复回显，
 * 用户只能重启容器 —— 这就是「删除旧的报告都没有办法」。
 */
async function handleClearReport(scope = 'current') {
  const tab = currentTab.value
  const label = scope === 'all' ? '全部市场智能研报' : `【${currentTitle.value}】研报`
  dialog.warning({
    title: '确认清除',
    content: `确定要清除${label}吗？将同时删除内存缓存与归档表中的历史记录，此操作不可撤销。`,
    positiveText: '确认清除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        const res = await ClearMarketIntelligenceReport(scope === 'all' ? '' : tab)
        if (scope === 'all') {
          reports.value = {}
        } else {
          reports.value[tab] = null
        }
        message.success(String(res || '已清除'))
        await refreshAll()
      } catch (e) {
        message.error('清除失败: ' + (e?.message || e))
      }
    }
  })
}

async function handlePurgeCorrupted() {
  try {
    let count = 0
    if (typeof PurgeCorruptedMarketReports === 'function') {
      count = await PurgeCorruptedMarketReports()
    }
    reports.value[currentTab.value] = null
    message.success(`已深度扫描并彻底清理 ${count} 条残损历史记录`)
    await refreshAll()
  } catch (e) {
    message.error('清理失败: ' + (e?.message || e))
  }
}

async function loadLifecycle() {
  try {
    lifecycle.value = await GetMarketLifecycleStatus()
  } catch (e) {
    console.error('GetMarketLifecycleStatus error:', e)
  }
}

function handleTabChange(tabKey) {
  currentTab.value = tabKey
  emit('changeTab', tabKey)
  if (!reports.value[tabKey]) {
    loadLatestForTab(tabKey)
  }
}

function selectSerenityPreset(preset) {
  serenityTarget.value = preset
}

let pollTimer = null

function startPollingForCompletion(tabKey, startTime) {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  let attempts = 0
  const maxAttempts = 36 // 最长轮询 180 秒 (36 * 5s)

  pollTimer = setInterval(async () => {
    attempts++
    try {
      let report = null
      if (typeof GetLatestMarketIntelligenceReportByType === 'function') {
        report = await GetLatestMarketIntelligenceReportByType(tabKey)
      }
      if (report && !isCorruptedReport(report)) {
        const reportTime = new Date(report.generatedAt).getTime()
        // 判定生成的研报时间是否在点击推演之后（容差 5 秒）
        if (reportTime >= startTime - 5000) {
          clearInterval(pollTimer)
          pollTimer = null
          reports.value[tabKey] = report
          loading.value = false
          message.success(`【${currentTitle.value}】后台大模型深度研报生成完成，已自动呈现在此！`)
          return
        }
      }
    } catch (e) {
      console.warn('轮询研报生成状态中...', e)
    }

    if (attempts >= maxAttempts) {
      clearInterval(pollTimer)
      pollTimer = null
      loading.value = false
      message.warning('后台生成等待超时，请稍后刷新抽屉或检查大模型运行日志。')
    }
  }, 5000)
}

async function generate() {
  if (!selectedAiConfigId.value) {
    message.warning('请先选择 AI 配置')
    return
  }

  loading.value = true
  const startTime = Date.now()
  message.info(`正在生成【${currentTitle.value}】，多Agent量化模型深度推演中...`)

  try {
    let res = null
    if (currentTab.value === 'pre_market_briefing') {
      res = await GeneratePreMarketBriefing(selectedAiConfigId.value)
    } else if (currentTab.value === 'post_market_review') {
      res = await GeneratePostMarketReview(selectedAiConfigId.value)
    } else if (currentTab.value === 'etf_rotation_decision') {
      res = await GenerateETFRotationReport(selectedAiConfigId.value)
    } else if (currentTab.value === 'serenity_industry_analysis') {
      res = await GenerateSerenityResearchReport(
        serenityTarget.value || '核心赛道',
        serenityCode.value || '',
        selectedAiConfigId.value
      )
    }

    if (res && !isCorruptedReport(res)) {
      reports.value[currentTab.value] = res
      message.success(`【${currentTitle.value}】生成成功！`)
      loading.value = false
    } else {
      message.warning('生成完成，暂无有效内容返回，请检查所选大模型的 API Key 及配额是否有效。')
      loading.value = false
    }
  } catch (err) {
    console.error('generate error:', err)
    const errMsg = String(err?.message || err)
    // 弹性容错机制：大模型深度推演因长文本容易触发反向代理（如 Nginx）的 60s 超时 (502)
    // 但后端已通过解耦独立的 10 分钟 context 在后台继续推演，此时切入后台轮询等待
    // 注意：必须同时匹配**中文**的「超时 / 熔断」。
    // bridge 抛出的超时文案原本是纯中文（“请求超时：接口 … 超过 30 秒，已执行保护性熔断”），
    // 而这里只匹配英文 'timeout' → 永远命中不了 → 直接报「研报生成失败」，
    // 后台其实还在正常生成。这是「盘前推演等失败」的直接原因。
    if (
      errMsg.includes('502') ||
      errMsg.includes('503') ||
      errMsg.includes('504') ||
      errMsg.includes('timeout') ||
      errMsg.includes('超时') ||
      errMsg.includes('熔断') ||
      errMsg.includes('Failed to fetch') ||
      errMsg.includes('NetworkError') ||
      errMsg.includes('context canceled')
    ) {
      message.loading('大模型深度推演耗时较长（通常需 40~90 秒），上游代理连接已断开，系统已自动转入后台持续生成中...', { duration: 8000 })
      startPollingForCompletion(currentTab.value, startTime)
    } else {
      message.error(`研报生成失败: ${errMsg}。请检查 AI 配置是否有效，或切换其他大模型重试。`)
      loading.value = false
    }
  }
}

function copyMarkdown() {
  if (!displayContent.value) return
  navigator.clipboard.writeText(displayContent.value).then(() => {
    message.success('研报 Markdown 内容已复制到剪贴板')
  })
}

function handleClose() {
  emit('update:show', false)
}

function askAgentAssistant() {
  if (!currentReport.value) return
  const title = currentReport.value.title || currentTitle.value
  const summary = currentReport.value.summary || ''
  const content = currentReport.value.content || ''
  
  // 提取研报核心精华前置注入为一手量化与买方参考数据
  const reportExcerpt = content.length > 1200 ? content.slice(0, 1200) + '...' : content
  const context = `【宏观研报类型】: ${currentTitle.value}\n【研报标题】: ${title}\n【核心摘要】: ${summary}\n【研报精要】:\n${reportExcerpt}`
  
  const question = `请基于这份【${title}】的核心观点与宏观研判，进行买方深度推演：\n1. 提炼核心逻辑与关键盘面推论；\n2. 结合当前市场风险偏好，梳理最具弹性的主线赛道与潜伏标的；\n3. 给出明确的风控止损红线与仓位管理建议。`

  if (props.show) {
    handleClose()
  }

  consultAgent({
    question,
    context,
    mode: 'plan_execute',
    autoSend: true,
  })
}

EventsOn('marketIntelligenceReport', (incoming) => {
  if (!incoming || isCorruptedReport(incoming)) return
  if (reports.value[incoming.type] !== undefined) {
    reports.value[incoming.type] = incoming
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    loading.value = false
    message.success(`【${incoming.title || '智能研报'}】已由智能体实时同步！`)
  }
})

EventsOn('show-market-intelligence', (type) => {
  if (type) currentTab.value = type
  emit('update:show', true)
})

EventsOn('show-market-intelligence-serenity', (data) => {
  currentTab.value = 'serenity_industry_analysis'
  if (data) {
    serenityTarget.value = typeof data === 'string' ? data : (data?.name || '')
    if (data?.code) serenityCode.value = data.code
  }
  emit('update:show', true)
})

let onWinShowIntel = null
let onWinShowSerenity = null

if (typeof window !== 'undefined') {
  onWinShowIntel = (e) => {
    if (e.detail) currentTab.value = e.detail
    emit('update:show', true)
  }
  onWinShowSerenity = (e) => {
    currentTab.value = 'serenity_industry_analysis'
    if (e.detail) {
      serenityTarget.value = typeof e.detail === 'string' ? e.detail : (e.detail?.name || '')
      if (e.detail?.code) serenityCode.value = e.detail.code
    }
    emit('update:show', true)
  }
  window.addEventListener('show-market-intelligence', onWinShowIntel)
  window.addEventListener('show-market-intelligence-serenity', onWinShowSerenity)
}

// 呼叫全屏 Agent 结合当前宏观情报进行持仓防守推演（跨组件上下文穿透）
function callAgentForPortfolioDeduction() {
  const title = currentTitle.value || '市场情报'
  const phase = lifecycle.value?.phaseName || '休市整固'
  const desc = lifecycle.value?.marketStateDesc || ''
  
  let summarySnippet = ''
  if (currentReport.value?.content) {
    // 截取前 400 字核心逻辑
    summarySnippet = currentReport.value.content.replace(/[#*`]/g, '').trim().slice(0, 400) + '...'
  }

  let contextText = `【大盘宏观情报态势】：\n报告主题: ${title}\n市场节律相位: ${phase}\n状态描述: ${desc}\n`
  if (summarySnippet) {
    contextText += `情报摘要: ${summarySnippet}\n`
  }

  const question = `请结合当前【${title}】大盘宏观情报与【${phase}】市场节律状态，对全市场风险敞口与实盘自选股持仓防守位进行深度推演。重点评估：1. 当前仓位控制上限与主线板块博弈胜率；2. 针对自选股持仓有哪些黑天鹅风险需提前设防；3. 是否存在买方强反转或防御切换机会？`

  handleClose()
  consultAgent({
    question,
    context: contextText,
    mode: 'plan_execute',
    autoSend: true,
  })
}

function onDrawerSettingsUpdated(cfg) {
  if (cfg) {
    const prefId = cfg.aiConfigId ?? cfg.defaultAiConfigId
    loadAiConfigs(prefId)
  }
}

function onDrawerAiConfigsUpdated(payload) {
  const prefId = payload?.defaultAiConfigId
  loadAiConfigs(prefId)
}

// 响应式移动端视口检测
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)
function handleDrawerResize() {
  isMobile.value = typeof window !== 'undefined' ? window.innerWidth <= 768 : false
}

// 沉浸式全屏与工作站窗口尺寸计算（彻底告别狭窄局促的右侧侧边抽屉）
const isFullscreen = ref(false)
function toggleFullscreen() {
  isFullscreen.value = !isFullscreen.value
}
const modalStyle = computed(() => {
  if (isFullscreen.value) {
    return {
      width: '100vw',
      height: '100vh',
      maxWidth: '100vw',
      maxHeight: '100vh',
      margin: '0',
      borderRadius: '0',
      display: 'flex',
      flexDirection: 'column'
    }
  }
  return {
    width: isMobile.value ? '98vw' : 'min(1360px, 94vw)',
    height: isMobile.value ? '96vh' : '90vh',
    maxHeight: '92vh',
    display: 'flex',
    flexDirection: 'column',
    borderRadius: '14px'
  }
})

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('resize', handleDrawerResize)
    handleDrawerResize()
  }
  loadTheme()
  loadAiConfigs()
  loadLifecycle()
  if (props.embedded) {
    loadLatestForTab(currentTab.value)
  }
  EventsOn('updateSettings', onDrawerSettingsUpdated)
  EventsOn('aiConfigsUpdated', onDrawerAiConfigsUpdated)
})

onBeforeUnmount(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('resize', handleDrawerResize)
  }
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
  EventsOff('marketIntelligenceReport')
  EventsOff('show-market-intelligence')
  EventsOff('show-market-intelligence-serenity')
  EventsOff('updateSettings', onDrawerSettingsUpdated)
  EventsOff('aiConfigsUpdated', onDrawerAiConfigsUpdated)
  if (typeof window !== 'undefined') {
    if (onWinShowIntel) window.removeEventListener('show-market-intelligence', onWinShowIntel)
    if (onWinShowSerenity) window.removeEventListener('show-market-intelligence-serenity', onWinShowSerenity)
  }
})
</script>

<template>
  <!-- 场景 A：主界面 Tab 内嵌工作站渲染 (Embedded Workstation Mode) -->
  <div
    v-if="embedded"
    class="intel-workstation-embedded"
    :class="{ 'is-dark': isDark }"
  >
    <div class="embedded-header-bar">
      <div class="header-title-wrap">
        <span class="header-icon">🧠</span>
        <span class="header-title">AI 市场智能中枢 · {{ currentTitle }}</span>
        <NTag :type="phaseTagType" round size="small" class="phase-tag">
          ● {{ lifecycle?.phaseName || '休市整固' }}
        </NTag>
      </div>
      <div class="header-actions">
        <span v-if="lifecycle?.currentTime" class="status-clock">
          {{ lifecycle?.currentTime }}
        </span>
        <div class="model-picker-wrap">
          <span class="model-picker-label">🤖 模型:</span>
          <NSelect
            v-if="aiConfigOptions.length > 0"
            v-model:value="selectedAiConfigId"
            :options="aiConfigOptions"
            size="tiny"
            style="min-width: 170px; max-width: 250px;"
            placeholder="选择决策模型"
          />
          <NButton
            v-else
            size="tiny"
            type="warning"
            ghost
            @click="goToSettings"
          >
            ⚙️ 前往配置 AI
          </NButton>
          <NButton
            size="tiny"
            type="primary"
            secondary
            style="font-weight: 600; margin-left: 6px;"
            title="将当前宏观情报带入全屏智能体中枢进行持仓风控推演"
            @click="callAgentForPortfolioDeduction"
          >
            🤖 持仓防守推演
          </NButton>
        </div>
      </div>
    </div>

    <!-- 核心工作区内容 -->
    <div class="intel-workstation-body">
      <!-- 1. 顶部 4 大核心维度导航卡片栏 -->
      <div class="nav-tabs-container">
        <div
          v-for="item in navTabs"
          :key="item.key"
          class="nav-tab-item"
          :class="{ active: currentTab === item.key }"
          @click="handleTabChange(item.key)"
        >
          <div class="tab-icon-box">{{ item.icon }}</div>
          <div class="tab-text-box">
            <div class="tab-title">{{ item.title }}</div>
            <div class="tab-subtitle">{{ item.desc }}</div>
          </div>
          <div v-if="currentTab === item.key" class="tab-active-indicator" />
        </div>
      </div>

      <!-- 2. 状态信息栏 -->
      <div class="market-status-bar">
        <div class="status-left">
          <span class="status-desc">{{ lifecycle?.marketStateDesc }}</span>
        </div>
        <div class="status-right">
          <span v-if="lifecycle?.nextEvent" class="next-event-chip">
            ⏭ 下一节点: <strong>{{ lifecycle.nextEvent }} ({{ lifecycle.nextEventTime }})</strong>
          </span>
        </div>
      </div>

      <!-- 3. Serenity 专属参数输入栏 -->
      <div v-if="currentTab === 'serenity_industry_analysis'" class="serenity-ctrl-card">
        <div class="serenity-input-row">
          <NInput
            v-model:value="serenityTarget"
            size="small"
            placeholder="输入目标行业/产业链/龙头公司（如：半导体设备、中际旭创）"
            style="flex: 1;"
            @keydown.enter="generate"
          />
          <NInput
            v-model:value="serenityCode"
            size="small"
            placeholder="可选代码(如: 300308)"
            style="width: 130px;"
          />
          <NButton type="primary" size="small" :loading="loading" @click="generate">
            🔬 发起深度拆解
          </NButton>
        </div>
        <div class="serenity-presets-row">
          <span class="presets-label">⚡ 热门方向：</span>
          <NTag
            v-for="p in serenityPresets"
            :key="p"
            size="tiny"
            round
            checkable
            :checked="serenityTarget === p"
            style="cursor: pointer;"
            @click="selectSerenityPreset(p)"
          >
            {{ p }}
          </NTag>
        </div>
      </div>

      <!-- 4. 报告元数据与快捷操作条 -->
      <div class="report-meta-bar">
        <div class="meta-left">
          <span v-if="currentReport && !isReportCorrupted" class="meta-item">
            📅 生成时间：<strong>{{ new Date(currentReport.generatedAt).toLocaleString() }}</strong>
          </span>
          <span v-if="currentReport?.source && !isReportCorrupted" class="meta-item meta-source">
            来源：{{ currentReport.source }}
          </span>
          <span v-if="!currentReport || isReportCorrupted" class="meta-item meta-empty">
            当前尚未生成该维度研报，点击右侧按钮立即推演
          </span>
        </div>
        <div class="meta-right">
          <NButton v-if="displayContent" size="tiny" secondary @click="copyMarkdown">
            📋 复制 Markdown
          </NButton>
          <NButton size="tiny" type="primary" :loading="loading" @click="generate">
            {{ currentReport && !isReportCorrupted ? '🔄 重新推演' : '🚀 立即生成' }}
          </NButton>
          <NButton v-if="currentReport" size="tiny" secondary type="error" @click="handleClearReport('current')">
            🗑️ 清除此报告
          </NButton>
        </div>
      </div>

      <!-- 发现残损研报时的引导与清理卡片 -->
      <div v-if="isReportCorrupted" class="corrupted-alert-card">
        <div class="corrupted-alert-left">
          <span class="corrupted-icon">⚠️</span>
          <div class="corrupted-text">
            <div class="corrupted-title">检测到历史研报存在网络中断或网关超时截断 (502 / context canceled)</div>
            <div class="corrupted-desc">系统已为您主动隔离拦截报错残卷，请点击右侧按钮重新启动大模型推演。</div>
          </div>
        </div>
        <NSpace>
          <NButton size="small" type="primary" :loading="loading" @click="generate">
            🚀 立即重新推演
          </NButton>
          <NButton size="small" secondary @click="handlePurgeCorrupted">
            🧹 物理清除残损记录
          </NButton>
        </NSpace>
      </div>

      <!-- 5. 核心决策摘要卡片 -->
      <div v-if="currentReport?.summary && !isReportCorrupted" class="summary-card">
        <div class="summary-header">
          <span class="summary-badge">💡 核心决策摘要</span>
        </div>
        <div class="summary-content">
          {{ currentReport.summary }}
        </div>
      </div>

      <!-- 研报伴随系统联动动作项 (Agent-Action) -->
      <div v-if="reportActions && reportActions.length > 0 && !isReportCorrupted" class="report-actions-container">
        <div class="report-actions-title">⚡ 研报建议系统联动：</div>
        <div class="report-actions-list">
          <NButton
            v-for="(act, idx) in reportActions"
            :key="idx"
            size="small"
            type="primary"
            secondary
            @click="handleExecuteAction(act)"
          >
            {{ getActionDesc(act) }}
          </NButton>
        </div>
      </div>

      <!-- Serenity 认知账本结构化沉淀卡片 (Embedded) -->
      <div v-if="extractedThesis && !isReportCorrupted" class="thesis-card">
        <div class="thesis-header">
          <div class="thesis-title">
            <span class="thesis-badge">🧠 Serenity 买方认知沉淀 (认知账本直通)</span>
            <NTag v-if="extractedThesis.stockCode" type="info" size="small" round>
              {{ extractedThesis.stockCode }} {{ extractedThesis.stockName }}
            </NTag>
            <NTag :type="extractedThesis.verdict?.toUpperCase().includes('BUY') ? 'success' : (extractedThesis.verdict?.toUpperCase().includes('AVOID') ? 'error' : 'warning')" size="small" round>
              {{ extractedThesis.verdict }}
            </NTag>
          </div>
          <NButton
            type="primary"
            size="small"
            :loading="isSavingThesis"
            @click="handleSaveThesisToLedger"
          >
            📥 确认沉淀至认知账本
          </NButton>
        </div>
        <div class="thesis-body">
          <div class="thesis-row">
            <span class="thesis-label">🎯 核心投资假说：</span>
            <span class="thesis-val">{{ extractedThesis.thesis }}</span>
          </div>
          <div v-if="extractedThesis.evidenceChain" class="thesis-row">
            <span class="thesis-label">📜 客观证据天梯：</span>
            <span class="thesis-val">{{ extractedThesis.evidenceChain }}</span>
          </div>
          <div v-if="extractedThesis.researchGaps" class="thesis-row gap-row">
            <span class="thesis-label">⚠️ 关键研究缺口 (Research Gaps)：</span>
            <span class="thesis-val">{{ extractedThesis.researchGaps }}</span>
          </div>
          <div v-if="extractedThesis.counterarguments" class="thesis-row risk-row">
            <span class="thesis-label">🛑 证伪失效条件 (Counterarguments)：</span>
            <span class="thesis-val">{{ extractedThesis.counterarguments }}</span>
          </div>
        </div>
      </div>

      <!-- 6. 研报正文展示容器 -->
      <NSpin :show="loading">
        <div v-if="!currentReport || !displayContent" class="empty-report-view">
          <NEmpty :description="'尚未生成 ' + currentTitle" style="padding: 48px 0;">
            <template #extra>
              <NButton type="primary" size="medium" :loading="loading" @click="generate">
                🚀 立即启动 {{ currentTitle }} 多Agent分析
              </NButton>
            </template>
          </NEmpty>
        </div>
        <div v-else class="markdown-preview-container">
          <MdPreview
            :theme="isDark ? 'dark' : 'light'"
            :model-value="displayContent"
            preview-theme="github"
            :show-code-row-number="false"
            :sanitize="sanitizeMarkdownHtml"
            style="background: transparent;"
          />
        </div>
      </NSpin>

      <!-- 底部辅助工具条 -->
      <div class="workstation-bottom-bar" style="flex-direction: column; align-items: stretch; gap: 8px;">
        <div style="display: flex; align-items: center; justify-content: space-between; width: 100%;">
          <span class="footer-hint">
            {{ currentReport && !isReportCorrupted ? '🧠 由 blades-stock 纯Go量化引擎与投委会多Agent协同生成' : '提示：选择上方标签即可切换对应研报' }}
          </span>
          <NSpace>
            <NButton
              v-if="currentReport && !isReportCorrupted"
              secondary
              type="info"
              title="带上本研报核心要点与上下文，直通 AI 智能体推演工作台进行互动追问与标的挖掘"
              @click="askAgentAssistant"
            >
              💬 针对本报告提问 AI 助手 ➔
            </NButton>
            <NButton
              v-if="currentReport && !isReportCorrupted"
              tertiary
              @click="loadLatestForTab(currentTab)"
            >
              刷新缓存
            </NButton>
            <NButton
              type="primary"
              :loading="loading"
              @click="generate"
            >
              {{ currentReport && !isReportCorrupted ? '重新推演' : '立即生成' }}
            </NButton>
            <NButton v-if="currentReport" secondary type="error" @click="handleClearReport('current')">
              🗑️ 清除此报告
            </NButton>
          </NSpace>
        </div>
      </div>
    </div>
  </div>

  <!-- 场景 B：作为全局模态工作台弹出 (Modal Mode) —— 彻底取代狭窄侧边抽屉 -->
  <NModal
    v-else
    :show="show"
    preset="card"
    class="market-intelligence-modal"
    :class="{ 'is-fullscreen': isFullscreen, 'is-dark': isDark }"
    :style="modalStyle"
    :mask-closable="true"
    @update:show="handleClose"
  >
    <template #header>
      <div class="modal-header-bar">
        <div class="header-title-wrap">
          <span class="header-icon">🧠</span>
          <span class="header-title">AI 市场智能中枢 · {{ currentTitle }}</span>
          <NTag :type="phaseTagType" round size="small" class="phase-tag">
            ● {{ lifecycle?.phaseName || '休市整固' }}
          </NTag>
        </div>
        <div class="header-actions">
          <span v-if="lifecycle?.currentTime" class="status-clock">
            {{ lifecycle?.currentTime }}
          </span>
          <div class="model-picker-wrap">
            <span class="model-picker-label">🤖 模型:</span>
            <NSelect
              v-if="aiConfigOptions.length > 0"
              v-model:value="selectedAiConfigId"
              :options="aiConfigOptions"
              size="tiny"
              style="min-width: 150px; max-width: 220px;"
              placeholder="选择决策模型"
            />
            <NButton
              v-else
              size="tiny"
              type="warning"
              ghost
              @click="goToSettings"
            >
              ⚙️ 配置AI
            </NButton>
            <NButton
              size="tiny"
              type="primary"
              secondary
              style="font-weight: 600; margin-left: 6px;"
              title="将当前宏观情报带入全屏智能体中枢进行持仓风控推演"
              @click="callAgentForPortfolioDeduction"
            >
              🤖 持仓防守推演
            </NButton>
          </div>
          <NButton
            size="tiny"
            quaternary
            circle
            class="fs-toggle-btn"
            :title="isFullscreen ? '还原窗口' : '全屏展示'"
            @click="toggleFullscreen"
          >
            {{ isFullscreen ? '🗗' : '⛶' }}
          </NButton>
        </div>
      </div>
    </template>

    <div class="intel-modal-scrollable">
      <!-- 1. 顶部 4 大核心维度导航卡片栏 -->
      <div class="nav-tabs-container">
        <div
          v-for="item in navTabs"
          :key="item.key"
          class="nav-tab-item"
          :class="{ active: currentTab === item.key }"
          @click="handleTabChange(item.key)"
        >
          <div class="tab-icon-box">{{ item.icon }}</div>
          <div class="tab-text-box">
            <div class="tab-title">{{ item.title }}</div>
            <div class="tab-subtitle">{{ item.desc }}</div>
          </div>
          <div v-if="currentTab === item.key" class="tab-active-indicator" />
        </div>
      </div>

      <!-- 2. 状态信息栏 -->
      <div class="market-status-bar">
        <div class="status-left">
          <span class="status-desc">{{ lifecycle?.marketStateDesc }}</span>
        </div>
        <div class="status-right">
          <span v-if="lifecycle?.nextEvent" class="next-event-chip">
            ⏭ 下一节点: <strong>{{ lifecycle.nextEvent }} ({{ lifecycle.nextEventTime }})</strong>
          </span>
        </div>
      </div>

      <!-- 3. Serenity 专属参数输入栏 -->
      <div v-if="currentTab === 'serenity_industry_analysis'" class="serenity-ctrl-card">
        <div class="serenity-input-row">
          <NInput
            v-model:value="serenityTarget"
            size="small"
            placeholder="输入目标行业/产业链/龙头公司（如：半导体设备、中际旭创）"
            style="flex: 1;"
            @keydown.enter="generate"
          />
          <NInput
            v-model:value="serenityCode"
            size="small"
            placeholder="可选代码(如: 300308)"
            style="width: 130px;"
          />
          <NButton type="primary" size="small" :loading="loading" @click="generate">
            🔬 发起深度拆解
          </NButton>
        </div>
        <div class="serenity-presets-row">
          <span class="presets-label">⚡ 热门方向：</span>
          <NTag
            v-for="p in serenityPresets"
            :key="p"
            size="tiny"
            round
            checkable
            :checked="serenityTarget === p"
            style="cursor: pointer;"
            @click="selectSerenityPreset(p)"
          >
            {{ p }}
          </NTag>
        </div>
      </div>

      <!-- 4. 报告元数据与快捷操作条 -->
      <div class="report-meta-bar">
        <div class="meta-left">
          <span v-if="currentReport && !isReportCorrupted" class="meta-item">
            📅 生成时间：<strong>{{ new Date(currentReport.generatedAt).toLocaleString() }}</strong>
          </span>
          <span v-if="currentReport?.source && !isReportCorrupted" class="meta-item meta-source">
            来源：{{ currentReport.source }}
          </span>
          <span v-if="!currentReport || isReportCorrupted" class="meta-item meta-empty">
            当前尚未生成该维度研报，点击右侧按钮立即推演
          </span>
        </div>
        <div class="meta-right">
          <NButton v-if="displayContent" size="tiny" secondary @click="copyMarkdown">
            📋 复制 Markdown
          </NButton>
          <NButton size="tiny" type="primary" :loading="loading" @click="generate">
            {{ currentReport && !isReportCorrupted ? '🔄 重新推演' : '🚀 立即生成' }}
          </NButton>
          <NButton v-if="currentReport" size="tiny" secondary type="error" @click="handleClearReport('current')">
            🗑️ 清除此报告
          </NButton>
        </div>
      </div>

      <!-- 发现残损研报时的引导与清理卡片 -->
      <div v-if="isReportCorrupted" class="corrupted-alert-card">
        <div class="corrupted-alert-left">
          <span class="corrupted-icon">⚠️</span>
          <div class="corrupted-text">
            <div class="corrupted-title">检测到历史研报存在网络中断或网关超时截断 (502 / context canceled)</div>
            <div class="corrupted-desc">系统已为您主动隔离拦截报错残卷，请点击右侧按钮重新启动大模型推演。</div>
          </div>
        </div>
        <NSpace>
          <NButton size="small" type="primary" :loading="loading" @click="generate">
            🚀 立即重新推演
          </NButton>
          <NButton size="small" secondary @click="handlePurgeCorrupted">
            🧹 物理清除残损记录
          </NButton>
        </NSpace>
      </div>

      <!-- 5. 核心决策摘要卡片 -->
      <div v-if="currentReport?.summary && !isReportCorrupted" class="summary-card">
        <div class="summary-header">
          <span class="summary-badge">💡 核心决策摘要</span>
        </div>
        <div class="summary-content">
          {{ currentReport.summary }}
        </div>
      </div>

      <!-- 研报伴随系统联动动作项 (Agent-Action) -->
      <div v-if="reportActions && reportActions.length > 0 && !isReportCorrupted" class="report-actions-container">
        <div class="report-actions-title">⚡ 研报建议系统联动：</div>
        <div class="report-actions-list">
          <NButton
            v-for="(act, idx) in reportActions"
            :key="idx"
            size="small"
            type="primary"
            secondary
            @click="handleExecuteAction(act)"
          >
            {{ getActionDesc(act) }}
          </NButton>
        </div>
      </div>

      <!-- Serenity 认知账本结构化沉淀卡片 (Modal) -->
      <div v-if="extractedThesis && !isReportCorrupted" class="thesis-card">
        <div class="thesis-header">
          <div class="thesis-title">
            <span class="thesis-badge">🧠 Serenity 买方认知沉淀 (认知账本直通)</span>
            <NTag v-if="extractedThesis.stockCode" type="info" size="small" round>
              {{ extractedThesis.stockCode }} {{ extractedThesis.stockName }}
            </NTag>
            <NTag :type="extractedThesis.verdict?.toUpperCase().includes('BUY') ? 'success' : (extractedThesis.verdict?.toUpperCase().includes('AVOID') ? 'error' : 'warning')" size="small" round>
              {{ extractedThesis.verdict }}
            </NTag>
          </div>
          <NButton
            type="primary"
            size="small"
            :loading="isSavingThesis"
            @click="handleSaveThesisToLedger"
          >
            📥 确认沉淀至认知账本
          </NButton>
        </div>
        <div class="thesis-body">
          <div class="thesis-row">
            <span class="thesis-label">🎯 核心投资假说：</span>
            <span class="thesis-val">{{ extractedThesis.thesis }}</span>
          </div>
          <div v-if="extractedThesis.evidenceChain" class="thesis-row">
            <span class="thesis-label">📜 客观证据天梯：</span>
            <span class="thesis-val">{{ extractedThesis.evidenceChain }}</span>
          </div>
          <div v-if="extractedThesis.researchGaps" class="thesis-row gap-row">
            <span class="thesis-label">⚠️ 关键研究缺口 (Research Gaps)：</span>
            <span class="thesis-val">{{ extractedThesis.researchGaps }}</span>
          </div>
          <div v-if="extractedThesis.counterarguments" class="thesis-row risk-row">
            <span class="thesis-label">🛑 证伪失效条件 (Counterarguments)：</span>
            <span class="thesis-val">{{ extractedThesis.counterarguments }}</span>
          </div>
        </div>
      </div>

      <!-- 6. 研报正文展示容器 -->
      <NSpin :show="loading">
        <div v-if="!currentReport || !displayContent" class="empty-report-view">
          <NEmpty :description="'尚未生成 ' + currentTitle" style="padding: 48px 0;">
            <template #extra>
              <NButton type="primary" size="medium" :loading="loading" @click="generate">
                🚀 立即启动 {{ currentTitle }} 多Agent分析
              </NButton>
            </template>
          </NEmpty>
        </div>
        <div v-else class="markdown-preview-container">
          <MdPreview
            :theme="isDark ? 'dark' : 'light'"
            :model-value="displayContent"
            preview-theme="github"
            :show-code-row-number="false"
            :sanitize="sanitizeMarkdownHtml"
            style="background: transparent;"
          />
        </div>
      </NSpin>
    </div>

    <template #footer>
      <div class="modal-footer-row" style="flex-direction: column; align-items: stretch; gap: 8px;">
        <div style="display: flex; align-items: center; justify-content: space-between; width: 100%;">
          <span class="footer-hint">
            {{ currentReport && !isReportCorrupted ? '🧠 由 blades-stock 纯Go量化引擎与投委会多Agent协同生成' : '提示：选择上方标签即可切换对应研报' }}
          </span>
          <NSpace>
            <NButton
              v-if="currentReport && !isReportCorrupted"
              secondary
              type="info"
              title="带上本研报核心要点与上下文，直通 AI 智能体推演工作台进行互动追问与标的挖掘"
              @click="askAgentAssistant"
            >
              💬 针对本报告提问 AI 助手 ➔
            </NButton>
            <NButton
              v-if="currentReport && !isReportCorrupted"
              tertiary
              @click="loadLatestForTab(currentTab)"
            >
              刷新缓存
            </NButton>
            <NButton
              type="primary"
              :loading="loading"
              @click="generate"
            >
              {{ currentReport && !isReportCorrupted ? '重新推演' : '立即生成' }}
            </NButton>
            <NButton v-if="currentReport" secondary type="error" @click="handleClearReport('current')">
              🗑️ 清除此报告
            </NButton>
          </NSpace>
        </div>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.intel-drawer-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 残损拦截警告条 */
.corrupted-alert-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.28);
  border-radius: 8px;
  margin-top: 4px;
}

.corrupted-alert-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.corrupted-icon {
  font-size: 22px;
}

.corrupted-title {
  font-weight: 600;
  font-size: 13px;
  color: #ef4444;
}

.corrupted-desc {
  font-size: 12px;
  opacity: 0.8;
  margin-top: 2px;
}

/* 顶部 4 维导航卡片 */
.nav-tabs-container {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 8px;
  margin-bottom: 2px;
}

.nav-tab-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: var(--n-card-color, rgba(128, 128, 128, 0.06));
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.16));
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  user-select: none;
}

.nav-tab-item:hover {
  background: rgba(128, 128, 128, 0.12);
  border-color: rgba(128, 128, 128, 0.3);
  transform: translateY(-1px);
}

.nav-tab-item.active {
  background: rgba(24, 160, 88, 0.1);
  border-color: #18a058;
  box-shadow: 0 2px 8px rgba(24, 160, 88, 0.15);
}

.tab-icon-box {
  font-size: 22px;
  line-height: 1;
}

.tab-text-box {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.tab-title {
  font-size: 13px;
  font-weight: 600;
  line-height: 1.3;
  color: var(--n-text-color, #2d3748);
  white-space: nowrap;
}

.nav-tab-item.active .tab-title {
  color: #18a058;
  font-weight: 700;
}

.tab-subtitle {
  font-size: 11px;
  color: #718096;
  line-height: 1.2;
  margin-top: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tab-active-indicator {
  position: absolute;
  bottom: 0;
  left: 15%;
  right: 15%;
  height: 2px;
  background: #18a058;
  border-radius: 2px;
}

/* 市场状态栏 */
.market-status-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 14px;
  background: var(--n-card-color, rgba(128, 128, 128, 0.05));
  border: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.12));
  border-radius: 8px;
  font-size: 12px;
}

.status-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.phase-tag {
  font-weight: 600;
}

.status-clock {
  color: #718096;
  font-family: monospace;
}

.status-desc {
  color: #4a5568;
}

.status-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.next-event-chip {
  color: #718096;
}

.next-event-chip strong {
  color: #2d3748;
}

.model-picker-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.model-picker-label {
  color: #718096;
  font-size: 11px;
}

/* Serenity 输入卡片 */
.serenity-ctrl-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(128, 128, 128, 0.04);
  border: 1px dashed var(--n-border-color, rgba(128, 128, 128, 0.2));
  border-radius: 8px;
}

.serenity-input-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.serenity-presets-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.presets-label {
  font-size: 12px;
  color: #718096;
}

/* 报告元数据栏 */
.report-meta-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 2px;
  font-size: 12px;
  color: #718096;
}

.meta-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.meta-item strong {
  color: #2d3748;
}

.meta-source {
  padding: 1px 6px;
  background: rgba(128, 128, 128, 0.1);
  border-radius: 4px;
  color: #4a5568;
}

.meta-empty {
  color: #e53e3e;
}

.meta-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 核心摘要卡片 - 柔和舒适护眼 */
.summary-card {
  padding: 10px 14px;
  background: rgba(14, 165, 233, 0.05);
  border-left: 3px solid #0ea5e9;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.summary-header {
  display: flex;
  align-items: center;
}

.summary-badge {
  font-size: 11.5px;
  font-weight: 600;
  color: #0284c7;
}

.summary-content {
  font-size: 12.5px;
  line-height: 1.6;
  font-weight: 450;
  color: #334155;
}

/* 伴随联动操作 */
.report-actions-container {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 8px 12px;
  background: rgba(128, 128, 128, 0.04);
  border: 1px dashed var(--n-border-color, rgba(128, 128, 128, 0.18));
  border-radius: 6px;
}

.report-actions-title {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--n-text-color, #475569);
}

.report-actions-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

/* Serenity 买方认知沉淀卡片 - 结构化账本直通 */
.thesis-card {
  padding: 12px 16px;
  background: rgba(16, 185, 129, 0.05);
  border: 1px solid rgba(16, 185, 129, 0.25);
  border-left: 4px solid #10b981;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 6px;
}

.thesis-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}

.thesis-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.thesis-badge {
  font-size: 12px;
  font-weight: 700;
  color: #059669;
}

.thesis-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12.5px;
  line-height: 1.6;
}

.thesis-row {
  display: flex;
  align-items: flex-start;
  gap: 6px;
}

.thesis-label {
  font-weight: 600;
  color: var(--n-text-color, #475569);
  flex-shrink: 0;
}

.thesis-val {
  color: var(--n-text-color, #1e293b);
  word-break: break-word;
}

.gap-row .thesis-label {
  color: #d97706;
}

.gap-row .thesis-val {
  color: #b45309;
}

.risk-row .thesis-label {
  color: #dc2626;
}

.risk-row .thesis-val {
  color: #b91c1c;
}

.is-dark .thesis-card {
  background: rgba(16, 185, 129, 0.08);
  border-color: rgba(16, 185, 129, 0.3);
}

.is-dark .thesis-badge {
  color: #34d399;
}

.is-dark .thesis-label {
  color: #cbd5e1;
}

.is-dark .thesis-val {
  color: #f1f5f9;
}

.is-dark .gap-row .thesis-label,
.is-dark .gap-row .thesis-val {
  color: #fbbf24;
}

.is-dark .risk-row .thesis-label,
.is-dark .risk-row .thesis-val {
  color: #f87171;
}

/* Markdown 预览区 - 精美舒适投研排版，杜绝字体过大刺眼 */
.markdown-preview-container {
  margin-top: 2px;
  padding: 2px;
}

/* 全局限制 Markdown 默认过大的标题与正文 */
:deep(.md-editor-preview) {
  font-size: 13px !important;
  line-height: 1.65 !important;
  color: #334155 !important;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "PingFang SC", "Microsoft YaHei", sans-serif !important;
}

:deep(.md-editor-preview h1) {
  font-size: 18px !important;
  font-weight: 700 !important;
  line-height: 1.4 !important;
  margin-top: 14px !important;
  margin-bottom: 10px !important;
  padding-bottom: 6px !important;
  border-bottom: 1px solid rgba(128, 128, 128, 0.16) !important;
  color: #0f172a !important;
}

:deep(.md-editor-preview h2) {
  font-size: 15px !important;
  font-weight: 600 !important;
  line-height: 1.4 !important;
  margin-top: 12px !important;
  margin-bottom: 8px !important;
  padding-bottom: 4px !important;
  border-bottom: 1px dashed rgba(128, 128, 128, 0.14) !important;
  color: #1e293b !important;
}

:deep(.md-editor-preview h3) {
  font-size: 13.5px !important;
  font-weight: 600 !important;
  margin-top: 10px !important;
  margin-bottom: 6px !important;
  color: #334155 !important;
}

:deep(.md-editor-preview h4),
:deep(.md-editor-preview h5),
:deep(.md-editor-preview h6) {
  font-size: 12.5px !important;
  font-weight: 600 !important;
  margin-top: 8px !important;
  margin-bottom: 4px !important;
  color: #475569 !important;
}

:deep(.md-editor-preview p) {
  font-size: 13px !important;
  line-height: 1.65 !important;
  margin-bottom: 8px !important;
  color: #334155 !important;
}

:deep(.md-editor-preview ul),
:deep(.md-editor-preview ol) {
  padding-left: 18px !important;
  margin-bottom: 8px !important;
}

:deep(.md-editor-preview li) {
  font-size: 12.5px !important;
  line-height: 1.6 !important;
  margin-bottom: 3px !important;
  color: #334155 !important;
}

:deep(.md-editor-preview strong) {
  font-weight: 600 !important;
  color: #0f172a !important;
}

:deep(.md-editor-preview table) {
  font-size: 12px !important;
  border-collapse: collapse !important;
  margin: 10px 0 !important;
  width: 100% !important;
}

:deep(.md-editor-preview table th) {
  background: rgba(128, 128, 128, 0.06) !important;
  font-weight: 600 !important;
  padding: 6px 10px !important;
  border: 1px solid rgba(128, 128, 128, 0.16) !important;
  color: #1e293b !important;
}

:deep(.md-editor-preview table td) {
  padding: 5px 10px !important;
  border: 1px solid rgba(128, 128, 128, 0.14) !important;
  color: #334155 !important;
}

:deep(.md-editor-preview blockquote) {
  font-size: 12.5px !important;
  padding: 6px 12px !important;
  margin: 8px 0 !important;
  border-left: 3px solid #0ea5e9 !important;
  background: rgba(14, 165, 233, 0.04) !important;
  color: #475569 !important;
  border-radius: 0 4px 4px 0 !important;
}

:deep(.md-editor-preview code) {
  font-size: 11.5px !important;
  padding: 2px 5px !important;
  background: rgba(128, 128, 128, 0.08) !important;
  border-radius: 4px !important;
  font-family: monospace !important;
}

.empty-report-view {
  background: rgba(128, 128, 128, 0.03);
  border-radius: 8px;
  border: 1px dashed var(--n-border-color, rgba(128, 128, 128, 0.15));
}

/* 底部操作行 */
.drawer-footer-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.footer-hint {
  font-size: 11px;
  color: #718096;
}

/* ========================================================
   暗黑模式适配样式 (Dark Mode Adaptation)
   ======================================================== */
.is-dark .nav-tab-item {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.12);
}

.is-dark .nav-tab-item:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: rgba(255, 255, 255, 0.22);
}

.is-dark .nav-tab-item.active {
  background: rgba(99, 226, 183, 0.14);
  border-color: #63e2b7;
  box-shadow: 0 2px 10px rgba(99, 226, 183, 0.18);
}

.is-dark .tab-title {
  color: #e2e8f0;
}

.is-dark .nav-tab-item.active .tab-title {
  color: #63e2b7;
}

.is-dark .tab-subtitle {
  color: #94a3b8;
}

.is-dark .tab-active-indicator {
  background: #63e2b7;
}

.is-dark .market-status-bar {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.1);
}

.is-dark .status-clock {
  color: #94a3b8;
}

.is-dark .status-desc {
  color: #cbd5e1;
}

.is-dark .next-event-chip {
  color: #94a3b8;
}

.is-dark .next-event-chip strong {
  color: #f1f5f9;
}

.is-dark .model-picker-label {
  color: #94a3b8;
}

.is-dark .serenity-ctrl-card {
  background: rgba(255, 255, 255, 0.03);
  border-color: rgba(255, 255, 255, 0.16);
}

.is-dark .presets-label {
  color: #94a3b8;
}

.is-dark .report-meta-bar {
  color: #94a3b8;
}

.is-dark .meta-item strong {
  color: #f1f5f9;
}

.is-dark .meta-source {
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
}

.is-dark .meta-empty {
  color: #fc8181;
}

.is-dark .summary-card {
  background: rgba(56, 189, 248, 0.08);
  border-left-color: #38bdf8;
}

.is-dark .summary-badge {
  color: #38bdf8;
}

.is-dark .summary-content {
  color: #cbd5e1;
}

.is-dark .empty-report-view {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.1);
}

.is-dark .footer-hint {
  color: #94a3b8;
}

/* 深色模式下 Markdown 排版：柔和降反差、护眼防眩目 */
.is-dark :deep(.md-editor-preview) {
  color: #cbd5e1 !important;
}

.is-dark :deep(.md-editor-preview p),
.is-dark :deep(.md-editor-preview li),
.is-dark :deep(.md-editor-preview span),
.is-dark :deep(.md-editor-preview td) {
  color: #cbd5e1 !important;
}

.is-dark :deep(.md-editor-preview th) {
  color: #e2e8f0 !important;
  background: rgba(255, 255, 255, 0.05) !important;
}

.is-dark :deep(.md-editor-preview h1),
.is-dark :deep(.md-editor-preview h2),
.is-dark :deep(.md-editor-preview h3),
.is-dark :deep(.md-editor-preview h4) {
  color: #e2e8f0 !important;
  border-bottom-color: rgba(255, 255, 255, 0.1) !important;
}

.is-dark :deep(.md-editor-preview strong) {
  color: #f1f5f9 !important;
}

.is-dark :deep(.md-editor-preview table) {
  border-color: rgba(255, 255, 255, 0.12) !important;
}

.is-dark :deep(.md-editor-preview table tr) {
  background-color: transparent !important;
  border-top-color: rgba(255, 255, 255, 0.1) !important;
}

.is-dark :deep(.md-editor-preview table tr:nth-child(2n)) {
  background-color: rgba(255, 255, 255, 0.02) !important;
}

.is-dark :deep(.md-editor-preview table th),
.is-dark :deep(.md-editor-preview table td) {
  border-color: rgba(255, 255, 255, 0.1) !important;
}

.is-dark :deep(.md-editor-preview blockquote) {
  border-left-color: #38bdf8 !important;
  color: #94a3b8 !important;
  background: rgba(56, 189, 248, 0.05) !important;
}

.is-dark :deep(.md-editor-preview code) {
  background: rgba(255, 255, 255, 0.08) !important;
  color: #7dd3fc !important;
}

.is-dark :deep(details) {
  background: rgba(255, 255, 255, 0.03) !important;
  border-left-color: #38bdf8 !important;
  color: #cbd5e1 !important;
}

.is-dark :deep(details summary) {
  color: #e2e8f0 !important;
}

/* 移动端 (Screen <= 768px) 布局重排与触控优化 */
@media screen and (max-width: 768px) {
  .nav-tabs-container {
    grid-template-columns: repeat(2, 1fr) !important;
    gap: 6px !important;
  }
  .nav-tab-item {
    padding: 6px 8px !important;
  }
  .tab-title {
    font-size: 13px !important;
  }
  .tab-subtitle {
    font-size: 10px !important;
  }
  .market-status-bar {
    flex-direction: column !important;
    align-items: stretch !important;
    gap: 8px !important;
  }
  .status-left, .status-right {
    width: 100% !important;
    justify-content: space-between !important;
    flex-wrap: wrap !important;
    gap: 6px !important;
  }
  .model-picker-wrap {
    width: 100% !important;
  }
  .model-picker-wrap :deep(.n-select) {
    max-width: 100% !important;
    width: 100% !important;
  }
  .serenity-input-row {
    flex-direction: column !important;
    align-items: stretch !important;
    gap: 6px !important;
  }
  .serenity-input-row :deep(.n-input) {
    width: 100% !important;
  }
}

/* ========== 全屏/模态沉浸式智能中枢工作台 (彻底取代狭窄侧边抽屉) ========== */
:deep(.market-intelligence-modal) {
  overflow: hidden !important;
  display: flex !important;
  flex-direction: column !important;
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  box-shadow: 0 24px 64px -12px rgba(0, 0, 0, 0.5), 0 0 0 1px rgba(255, 255, 255, 0.08);
}

:deep(.market-intelligence-modal .n-card-header) {
  padding: 14px 20px !important;
  border-bottom: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.16));
  background: var(--n-card-color, rgba(255, 255, 255, 0.85));
  backdrop-filter: blur(12px);
}

:deep(.market-intelligence-modal .n-card__content) {
  padding: 16px 20px !important;
  flex: 1 !important;
  overflow: hidden !important;
  display: flex !important;
  flex-direction: column !important;
}

:deep(.market-intelligence-modal .n-card__footer) {
  padding: 12px 20px !important;
  border-top: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.16));
  background: var(--n-card-color, rgba(255, 255, 255, 0.85));
}

.modal-header-bar,
.embedded-header-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.header-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-icon {
  font-size: 22px;
}

.header-title {
  font-size: 16px;
  font-weight: 700;
  color: var(--n-text-color, #1a202c);
  letter-spacing: 0.3px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.fs-toggle-btn {
  font-size: 15px;
  color: var(--n-text-color-2);
}

.fs-toggle-btn:hover {
  color: #18a058;
}

.intel-modal-scrollable {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-right: 6px;
}

.modal-footer-row,
.workstation-bottom-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

/* 内嵌工作站容器 */
.intel-workstation-embedded {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 6px 4px 16px;
}

.embedded-header-bar {
  padding-bottom: 12px;
  border-bottom: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.16));
}

.workstation-bottom-bar {
  margin-top: 16px;
  padding-top: 14px;
  border-top: 1px solid var(--n-border-color, rgba(128, 128, 128, 0.16));
}

.intel-workstation-body {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
</style>
