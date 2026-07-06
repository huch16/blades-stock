<template>
  <div class="vectorbt-panel" :class="{ 'dark-theme': darkTheme }">
    <!-- 标题 -->
    <div class="panel-header">
      <span class="panel-title">📊 VectorBT 向量化回测</span>
      <span class="panel-subtitle">基于矩阵运算的超速回测引擎</span>
    </div>

    <!-- 输入区域 -->
    <n-grid :cols="4" :x-gap="12" :y-gap="12">
      <!-- 股票代码 -->
      <n-gi>
        <div class="input-group">
          <label>股票代码</label>
          <n-auto-complete
            v-model:value="stockCode"
            :input-props="{ autocomplete: 'disabled' }"
            :options="stockOptions"
            placeholder="股票名称/代码（如 000001）"
            clearable
            @update-value="searchStock"
            @select="onSelectStock"
            @keyup.enter="runBacktest"
          />
        </div>
      </n-gi>

      <!-- 策略选择 -->
      <n-gi>
        <div class="input-group">
          <label>策略类型</label>
          <n-select
            v-model:value="strategyType"
            :options="strategyOptions"
          />
        </div>
      </n-gi>

      <!-- 初始资金 -->
      <n-gi>
        <div class="input-group">
          <label>初始资金</label>
          <n-input-number
            v-model:value="initCash"
            :min="1000"
            :step="10000"
          />
        </div>
      </n-gi>
    </n-grid>

    <!-- 策略参数 -->
    <n-grid v-if="strategyType === 'MA'" :cols="2" :x-gap="12" :y-gap="12" style="margin-top: 12px">
      <n-gi>
        <div class="input-group">
          <label>快线周期</label>
          <n-input-number v-model:value="fastPeriod" :min="1" :max="100" />
        </div>
      </n-gi>
      <n-gi>
        <div class="input-group">
          <label>慢线周期</label>
          <n-input-number v-model:value="slowPeriod" :min="1" :max="500" />
        </div>
      </n-gi>
    </n-grid>

    <n-grid v-if="strategyType === 'MACD'" :cols="3" :x-gap="12" :y-gap="12" style="margin-top: 12px">
      <n-gi>
        <div class="input-group">
          <label>快线周期</label>
          <n-input-number v-model:value="macdFast" :min="1" :max="100" />
        </div>
      </n-gi>
      <n-gi>
        <div class="input-group">
          <label>慢线周期</label>
          <n-input-number v-model:value="macdSlow" :min="1" :max="200" />
        </div>
      </n-gi>
      <n-gi>
        <div class="input-group">
          <label>信号周期</label>
          <n-input-number v-model:value="macdSignal" :min="1" :max="100" />
        </div>
      </n-gi>
    </n-grid>

    <n-grid v-if="strategyType === 'RSI'" :cols="1" :x-gap="12" style="margin-top: 12px">
      <n-gi>
        <div class="input-group">
          <label>窗口周期</label>
          <n-input-number v-model:value="rsiWindow" :min="1" :max="100" style="width: 200px" />
        </div>
      </n-gi>
    </n-grid>

    <!-- 操作按钮 -->
    <div class="action-buttons">
      <n-button type="primary" :loading="loading" @click="runBacktest">
        ▶ 运行回测
      </n-button>
      <n-button :loading="gridLoading" @click="runGridSearch">
        🔍 参数扫描
      </n-button>
      <n-button @click="resetForm">
        重置
      </n-button>
    </div>

    <!-- 结果展示 -->
    <div v-if="result" class="result-section">
      <!-- 策略信息 -->
      <div class="result-header">
        <n-tag type="info">{{ result.strategy }}</n-tag>
        <n-tag type="default">参数: {{ formatParams(result.params) }}</n-tag>
      </div>

      <!-- 核心指标卡片 -->
      <n-grid :cols="4" :x-gap="12" :y-gap="12" class="stats-grid">
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">总收益率</div>
            <div class="stat-value" :class="getReturnClass(stats.TotalReturn)">
              {{ formatPct(stats.TotalReturn) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">年化收益率</div>
            <div class="stat-value" :class="getReturnClass(stats.AnnualReturn)">
              {{ formatPct(stats.AnnualReturn) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">夏普比率</div>
            <div class="stat-value" :class="getSharpeClass(stats.SharpeRatio)">
              {{ formatNum(stats.SharpeRatio) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">最大回撤</div>
            <div class="stat-value danger">
              {{ formatPct(stats.MaxDrawdownPct) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">胜率</div>
            <div class="stat-value">
              {{ formatPct(stats.WinRate) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">盈亏比</div>
            <div class="stat-value">
              {{ formatNum(stats.ProfitFactor) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">交易次数</div>
            <div class="stat-value">
              {{ stats.TotalTrades }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">索提诺比率</div>
            <div class="stat-value" :class="getSharpeClass(stats.SortinoRatio)">
              {{ formatNum(stats.SortinoRatio) }}
            </div>
          </div>
        </n-gi>
      </n-grid>

      <!-- 详细报告 -->
      <div class="report-section">
        <div class="section-title">📋 详细报告</div>
        <pre class="report-content">{{ result.report }}</pre>
      </div>
    </div>

    <!-- 网格扫描结果 -->
    <div v-if="gridResult" class="grid-result-section">
      <div class="section-title">🔍 参数扫描结果</div>
      <div class="grid-message">{{ gridResult.message }}</div>

      <!-- 最优参数 -->
      <div v-if="gridResult.best_strategy" class="best-params">
        <n-tag type="success">最优策略: {{ gridResult.best_strategy }}</n-tag>
        <n-tag type="default">参数: {{ formatParams(gridResult.best_params) }}</n-tag>
      </div>

      <!-- 最优表现 -->
      <n-grid v-if="gridResult.best_stats" :cols="4" :x-gap="12" :y-gap="12" class="stats-grid">
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">总收益率</div>
            <div class="stat-value highlight">
              {{ formatPct(gridResult.best_stats.TotalReturn) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">夏普比率</div>
            <div class="stat-value">
              {{ formatNum(gridResult.best_stats.SharpeRatio) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">最大回撤</div>
            <div class="stat-value danger">
              {{ formatPct(gridResult.best_stats.MaxDrawdownPct) }}
            </div>
          </div>
        </n-gi>
        <n-gi>
          <div class="stat-card">
            <div class="stat-label">交易次数</div>
            <div class="stat-value">
              {{ gridResult.best_stats.TotalTrades }}
            </div>
          </div>
        </n-gi>
      </n-grid>
    </div>

    <!-- 加载状态 -->
    <n-spin v-if="loading || gridLoading" stroke="primary" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { RunVectorBacktest, RunVectorGridSearch, GetConfig, GetStockList } from '../../wailsjs/go/main/App'
import {
  NButton,
  NAutoComplete,
  NInputNumber,
  NSelect,
  NGrid,
  NGi,
  NTag,
  NSpin,
  useMessage
} from 'naive-ui'

const message = useMessage()

// 状态
const loading = ref(false)
const gridLoading = ref(false)
const darkTheme = ref(false)

// 表单数据
const stockCode = ref('')
const strategyType = ref('MA')
const fastPeriod = ref(5)
const slowPeriod = ref(20)
const macdFast = ref(12)
const macdSlow = ref(26)
const macdSignal = ref(9)
const rsiWindow = ref(14)
const initCash = ref(100000)

interface StockItem {
  name?: string
  ts_code?: string
  [key: string]: unknown
}

// 股票自动补全
const stockList = ref<StockItem[]>([])
const stockOptions = ref<Array<{ label: string; value: string }>>([])

// 加载股票列表
onMounted(async () => {
  try {
    const config = await GetConfig()
    darkTheme.value = config.darkTheme ?? false
  } catch (e) {
    console.error('获取配置失败:', e)
  }
  try {
    const list = await GetStockList("")
    stockList.value = list
  } catch (e) {
    console.error('加载股票列表失败:', e)
  }
})

// 搜索股票
function searchStock(value: string) {
  if (!value) {
    stockOptions.value = []
    return
  }
  stockOptions.value = stockList.value
    .filter((item: any) => item.name?.includes(value) || item.ts_code?.includes(value))
    .map((item: any) => ({
      label: `${item.name} - ${item.ts_code}`,
      value: item.ts_code,
    }))
  if (value.indexOf("-") <= 0) {
    stockCode.value = value
  }
}

// 选择股票：转换为内部格式（sz000001 / sh600000）
function onSelectStock(item: string) {
  if (item.indexOf("-") > 0) {
    item = item.split("-")[1].toLowerCase()
  }
  if (item.indexOf(".") > 0) {
    const parts = item.split(".")
    stockCode.value = parts[1].toLowerCase() + parts[0]
  } else {
    stockCode.value = item
  }
}

interface BacktestStats {
  TotalReturn: number
  AnnualReturn: number
  SharpeRatio: number
  MaxDrawdownPct: number
  WinRate: number
  ProfitFactor: number
  TotalTrades: number
  SortinoRatio: number
}

interface BacktestData {
  report: string
  stats: BacktestStats | null
  strategy: string
  params: Record<string, number>
}

interface GridSearchResult {
  code: number
  message: string
  best_strategy: string
  best_params: Record<string, unknown>
  best_stats: BacktestStats | null
  total_combinations: number
  duration: string
}

const result = ref<BacktestData | null>(null)
const stats = ref<BacktestStats>({} as BacktestStats)
const gridResult = ref<GridSearchResult | null>(null)

// 策略选项
const strategyOptions = [
  { label: 'MA 均线交叉', value: 'MA' },
  { label: 'MACD', value: 'MACD' },
  { label: 'RSI 超买超卖', value: 'RSI' },
]

// 获取参数
function getParams(): Record<string, number> {
  const params: Record<string, number> = {}
  if (strategyType.value === 'MA') {
    params.fast = fastPeriod.value
    params.slow = slowPeriod.value
  } else if (strategyType.value === 'MACD') {
    params.fast = macdFast.value
    params.slow = macdSlow.value
    params.signal = macdSignal.value
  } else if (strategyType.value === 'RSI') {
    params.window = rsiWindow.value
  }
  return params
}

// 运行回测
async function runBacktest() {
  if (!stockCode.value.trim()) {
    message.warning('请输入股票代码')
    return
  }

  loading.value = true
  result.value = null

  try {
    const res = await RunVectorBacktest(
      stockCode.value.trim(),
      strategyType.value,
      getParams()
    )

    if (res.code === 0) {
      message.error(res.message || '回测失败')
      return
    }

    result.value = res.data
    stats.value = res.data.stats || {}
    message.success('回测完成')
  } catch (e) {
    message.error('回测出错: ' + String(e))
  } finally {
    loading.value = false
  }
}

// 运行网格扫描
async function runGridSearch() {
  if (!stockCode.value.trim()) {
    message.warning('请输入股票代码')
    return
  }

  gridLoading.value = true
  gridResult.value = null

  try {
    const res = await RunVectorGridSearch(
      stockCode.value.trim(),
      strategyType.value
    )

    if (res.code === 0) {
      message.error(res.message || '扫描失败')
      return
    }

    gridResult.value = res
    message.success('参数扫描完成')
  } catch (e) {
    message.error('扫描出错: ' + String(e))
  } finally {
    gridLoading.value = false
  }
}

// 重置表单
function resetForm() {
  stockCode.value = ''
  strategyType.value = 'MA'
  fastPeriod.value = 5
  slowPeriod.value = 20
  macdFast.value = 12
  macdSlow.value = 26
  macdSignal.value = 9
  rsiWindow.value = 14
  result.value = null
  gridResult.value = null
}

// 格式化
function formatPct(v: number): string {
  if (v === undefined || v === null) return '—'
  return (v * 100).toFixed(2) + '%'
}

function formatNum(v: number): string {
  if (v === undefined || v === null) return '—'
  return v.toFixed(4)
}

function formatParams(params: any): string {
  if (!params) return ''
  return Object.entries(params)
    .map(([k, v]) => `${k}=${v}`)
    .join(', ')
}

// 样式类
function getReturnClass(v: number): string {
  if (v === undefined || v === null) return ''
  return v >= 0 ? 'positive' : 'negative'
}

function getSharpeClass(v: number): string {
  if (v === undefined || v === null) return ''
  return v >= 1 ? 'positive' : v >= 0 ? '' : 'negative'
}
</script>

<style scoped>
.vectorbt-panel {
  padding: 16px;
  background: #f5f5f5;
  border-radius: 8px;
  position: relative;
}

.dark-theme {
  background: #1a1a2e;
  color: #e0e0e0;
}

.panel-header {
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid #e0e0e0;
}

.dark-theme .panel-header {
  border-bottom-color: #333;
}

.panel-title {
  font-size: 18px;
  font-weight: 600;
  margin-right: 8px;
}

.panel-subtitle {
  font-size: 12px;
  color: #888;
}

.input-section {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}

.input-group {
  margin-bottom: 0;
}

.input-group label {
  display: block;
  font-size: 12px;
  color: #666;
  margin-bottom: 4px;
}

.dark-theme .input-group label {
  color: #aaa;
}

.action-buttons {
  display: flex;
  gap: 8px;
  margin-top: 16px;
  margin-bottom: 16px;
}

.result-section,
.grid-result-section {
  margin-top: 16px;
  padding: 16px;
  background: white;
  border-radius: 8px;
}

.dark-theme .result-section,
.dark-theme .grid-result-section {
  background: #252540;
}

.result-header {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}

.stats-grid {
  margin-bottom: 16px;
}

.stat-card {
  padding: 12px;
  background: #fafafa;
  border-radius: 6px;
  text-align: center;
}

.dark-theme .stat-card {
  background: #1e1e35;
}

.stat-label {
  font-size: 12px;
  color: #888;
  margin-bottom: 4px;
}

.dark-theme .stat-label {
  color: #888;
}

.stat-value {
  font-size: 18px;
  font-weight: 600;
  color: #333;
}

.dark-theme .stat-value {
  color: #e0e0e0;
}

.stat-value.positive {
  color: #e74c3c;
}

.stat-value.negative {
  color: #27ae60;
}

.stat-value.danger {
  color: #e74c3c;
}

.stat-value.highlight {
  color: #f39c12;
  font-size: 22px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  margin-bottom: 8px;
}

.report-section {
  margin-top: 12px;
}

.report-content {
  padding: 12px;
  background: #f8f8f8;
  border-radius: 6px;
  font-size: 12px;
  overflow-x: auto;
  white-space: pre-wrap;
  max-height: 400px;
  overflow-y: auto;
}

.dark-theme .report-content {
  background: #1a1a2e;
}

.grid-message {
  padding: 8px;
  margin-bottom: 12px;
  background: #e8f5e9;
  border-radius: 4px;
  font-size: 13px;
}

.dark-theme .grid-message {
  background: #1b3a2d;
}

.best-params {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
}
</style>
