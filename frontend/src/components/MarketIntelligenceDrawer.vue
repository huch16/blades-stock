<script setup>
import { computed, h, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NDrawer,
  NDrawerContent,
  NSpin,
  NCard,
  NFlex,
  NTag,
  NDivider,
  NButton,
  NSpace,
  NEmpty,
  NInput,
  NSelect,
  useMessage,
  useThemeVars
} from 'naive-ui'
import {
  GeneratePreMarketBriefing,
  GeneratePostMarketReview,
  GenerateETFRotationReport,
  GenerateSerenityResearchReport,
  GetLatestMarketIntelligenceReport,
  GetMarketLifecycleStatus,
  GetAiConfigs,
  GetConfig
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import { MdPreview } from 'md-editor-v3'
import 'md-editor-v3/lib/preview.css'

const props = defineProps({
  show: { type: Boolean, default: false },
  reportType: { type: String, default: 'pre_market_briefing' },
  initialSerenityTarget: { type: String, default: '' }
})

const emit = defineEmits(['update:show', 'openFloatingAgentWithQuestion'])

const router = useRouter()
const message = useMessage()
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
  } catch (e) {
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
const serenityPresets = ['半导体设备', '算力CPO', '人形机器人', '低空经济', '创新药', '商业航天', '固态电池']

const navTabs = [
  { key: 'pre_market_briefing', icon: '🌅', title: '盘前前瞻', desc: '隔夜外盘 · 竞价风向' },
  { key: 'post_market_review', icon: '🌆', title: '盘后复盘', desc: '全景量价 · 次日推演' },
  { key: 'etf_rotation_decision', icon: '📈', title: 'ETF轮动决策', desc: '21日动量 · 矩阵评分' },
  { key: 'serenity_industry_analysis', icon: '🔍', title: '产业深度拆解', desc: 'Serenity · 瓶颈挖掘' }
]

const currentReport = computed(() => {
  return reports.value[currentTab.value] || null
})

// 清理旧缓存或流式残留中的 [STATUS] 杂质
const displayContent = computed(() => {
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

async function loadAiConfigs() {
  try {
    const configs = await GetAiConfigs()
    aiConfigs.value = Array.isArray(configs) ? configs : []

    const saved = localStorage.getItem('market_intelligence_ai_config_id')
    const savedId = saved ? parseInt(saved, 10) : 0

    const validIds = aiConfigs.value.map(c => (c.ID != null ? Number(c.ID) : Number(c.id)))
    if (savedId && validIds.includes(savedId)) {
      selectedAiConfigId.value = savedId
    } else if (validIds.length > 0) {
      selectedAiConfigId.value = validIds[0]
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
    const latest = await GetLatestMarketIntelligenceReport()
    if (latest && latest.type === tabKey) {
      reports.value[tabKey] = latest
    }
  } catch (e) {
    console.error('GetLatestMarketIntelligenceReport error:', e)
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
  if (!reports.value[tabKey]) {
    loadLatestForTab(tabKey)
  }
}

function selectSerenityPreset(preset) {
  serenityTarget.value = preset
}

async function generate() {
  if (!selectedAiConfigId.value) {
    message.warning('请先选择 AI 配置')
    return
  }

  loading.value = true
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

    if (res) {
      reports.value[currentTab.value] = res
      message.success(`【${currentTitle.value}】生成成功！`)
    } else {
      message.warning('生成完成，暂无内容返回')
    }
  } catch (err) {
    console.error('generate error:', err)
    message.error(`生成失败: ${err?.message || err}`)
  } finally {
    loading.value = false
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
  const title = currentReport.value?.title || currentTitle.value
  emit(
    'openFloatingAgentWithQuestion',
    `请就这份【${title}】做精炼要点复述，并结合当前盘面给出重点关注标的和风控对策。`
  )
  handleClose()
}

EventsOn('marketIntelligenceReport', (incoming) => {
  if (!incoming) return
  if (reports.value[incoming.type] !== undefined) {
    reports.value[incoming.type] = incoming
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

if (typeof window !== 'undefined') {
  const onWinShowIntel = (e) => {
    if (e.detail) currentTab.value = e.detail
    emit('update:show', true)
  }
  const onWinShowSerenity = (e) => {
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

onMounted(() => {
  loadTheme()
  loadAiConfigs()
  loadLifecycle()
})

onBeforeUnmount(() => {
  EventsOff('marketIntelligenceReport')
  EventsOff('show-market-intelligence')
  EventsOff('show-market-intelligence-serenity')
})
</script>

<template>
  <NDrawer
    :show="show"
    @update:show="handleClose"
    :width="860"
    placement="right"
  >
    <NDrawerContent
      :title="'🧠 AI 市场智能中枢 · ' + currentTitle"
      :native-scrollbar="false"
      closable
    >
      <div class="intel-drawer-container" :class="{ 'is-dark': isDark }">
        <!-- 1. 顶部 4 大核心维度导航卡片栏（高辨识度，完美适配暗黑） -->
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

        <!-- 2. 市场状态与模型控制栏 -->
        <div class="market-status-bar">
          <div class="status-left">
            <NTag :type="phaseTagType" round size="small" class="phase-tag">
              ● {{ lifecycle?.phaseName || '休市整固' }}
            </NTag>
            <span class="status-clock">
              {{ lifecycle?.currentTime || '' }}
            </span>
            <span class="status-desc">
              {{ lifecycle?.marketStateDesc }}
            </span>
          </div>
          <div class="status-right">
            <span v-if="lifecycle?.nextEvent" class="next-event-chip">
              ⏭ 下一节点: <strong>{{ lifecycle.nextEvent }} ({{ lifecycle.nextEventTime }})</strong>
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
            </div>
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
              @click="selectSerenityPreset(p)"
              style="cursor: pointer;"
            >
              {{ p }}
            </NTag>
          </div>
        </div>

        <!-- 4. 报告元数据与快捷操作条 -->
        <div class="report-meta-bar">
          <div class="meta-left">
            <span v-if="currentReport" class="meta-item">
              📅 生成时间：<strong>{{ new Date(currentReport.generatedAt).toLocaleString() }}</strong>
            </span>
            <span v-if="currentReport?.source" class="meta-item meta-source">
              来源：{{ currentReport.source }}
            </span>
            <span v-if="!currentReport" class="meta-item meta-empty">
              当前尚未生成该维度研报，点击右侧按钮立即推演
            </span>
          </div>
          <div class="meta-right">
            <NButton
              v-if="displayContent"
              size="tiny"
              secondary
              @click="copyMarkdown"
            >
              📋 复制 Markdown
            </NButton>
            <NButton
              size="tiny"
              type="primary"
              :loading="loading"
              @click="generate"
            >
              {{ currentReport ? '🔄 重新推演' : '🚀 立即生成' }}
            </NButton>
          </div>
        </div>

        <!-- 5. 核心决策摘要卡片 (清晰醒目，适配暗黑) -->
        <div v-if="currentReport?.summary" class="summary-card">
          <div class="summary-header">
            <span class="summary-badge">💡 核心决策摘要</span>
          </div>
          <div class="summary-content">
            {{ currentReport.summary }}
          </div>
        </div>

        <!-- 6. 研报正文展示容器 (支持主题自适应) -->
        <NSpin :show="loading">
          <div v-if="!currentReport || !displayContent" class="empty-report-view">
            <NEmpty
              :description="`尚未生成 ${currentTitle}`"
              style="padding: 48px 0;"
            >
              <template #extra>
                <NButton type="primary" size="medium" @click="generate" :loading="loading">
                  🚀 立即启动 {{ currentTitle }} 多Agent分析
                </NButton>
              </template>
            </NEmpty>
          </div>

          <div v-else class="markdown-preview-container">
            <MdPreview
              :theme="isDark ? 'dark' : 'light'"
              :modelValue="displayContent"
              preview-theme="github"
              :show-code-row-number="false"
              style="background: transparent;"
            />
          </div>
        </NSpin>
      </div>

      <template #footer>
        <div class="drawer-footer-row">
          <span class="footer-hint">
            {{ currentReport ? '🧠 由 blades-stock 量化矩阵引擎与投委会多Agent协同生成' : '提示：选择上方标签即可切换对应研报' }}
          </span>
          <NSpace>
            <NButton @click="askAgentAssistant" v-if="currentReport" secondary type="info">
              💬 针对本报告提问AI助手
            </NButton>
            <NButton @click="loadLatestForTab(currentTab)" tertiary v-if="currentReport">
              刷新缓存
            </NButton>
            <NButton @click="generate" type="primary" :loading="loading">
              {{ currentReport ? '重新推演' : '立即生成' }}
            </NButton>
          </NSpace>
        </div>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>

<style scoped>
.intel-drawer-container {
  display: flex;
  flex-direction: column;
  gap: 14px;
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

/* 核心摘要卡片 */
.summary-card {
  padding: 12px 16px;
  background: rgba(24, 160, 88, 0.08);
  border-left: 4px solid #18a058;
  border-radius: 6px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.summary-header {
  display: flex;
  align-items: center;
}

.summary-badge {
  font-size: 12px;
  font-weight: 700;
  color: #18a058;
}

.summary-content {
  font-size: 13px;
  line-height: 1.6;
  font-weight: 500;
  color: var(--n-text-color, #2d3748);
}

/* Markdown 预览区 */
.markdown-preview-container {
  margin-top: 4px;
  padding: 4px 2px;
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
  background: rgba(99, 226, 183, 0.12);
  border-left-color: #63e2b7;
}

.is-dark .summary-badge {
  color: #63e2b7;
}

.is-dark .summary-content {
  color: #f1f5f9;
}

.is-dark .empty-report-view {
  background: rgba(255, 255, 255, 0.02);
  border-color: rgba(255, 255, 255, 0.1);
}

.is-dark .footer-hint {
  color: #94a3b8;
}

/* 保证 Markdown 在深色模式下所有文字和表格的明亮度 */
.is-dark :deep(.md-editor-preview) {
  color: #e2e8f0;
}

.is-dark :deep(.md-editor-preview p),
.is-dark :deep(.md-editor-preview li),
.is-dark :deep(.md-editor-preview span),
.is-dark :deep(.md-editor-preview td),
.is-dark :deep(.md-editor-preview th) {
  color: #e2e8f0;
}

.is-dark :deep(.md-editor-preview h1),
.is-dark :deep(.md-editor-preview h2),
.is-dark :deep(.md-editor-preview h3),
.is-dark :deep(.md-editor-preview h4) {
  color: #f8fafc;
  border-bottom-color: rgba(255, 255, 255, 0.15);
}

.is-dark :deep(.md-editor-preview table) {
  border-color: rgba(255, 255, 255, 0.15);
}

.is-dark :deep(.md-editor-preview table tr) {
  background-color: transparent;
  border-top-color: rgba(255, 255, 255, 0.15);
}

.is-dark :deep(.md-editor-preview table tr:nth-child(2n)) {
  background-color: rgba(255, 255, 255, 0.03);
}

.is-dark :deep(.md-editor-preview table th),
.is-dark :deep(.md-editor-preview table td) {
  border-color: rgba(255, 255, 255, 0.15);
}

.is-dark :deep(.md-editor-preview blockquote) {
  border-left-color: #63e2b7;
  color: #cbd5e1;
  background: rgba(255, 255, 255, 0.03);
}

.is-dark :deep(details) {
  background: rgba(255, 255, 255, 0.05) !important;
  border-left-color: #63e2b7 !important;
  color: #e2e8f0 !important;
}

.is-dark :deep(details summary) {
  color: #f1f5f9 !important;
}
</style>
