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

    <n-grid v-if="strategyType === 'BOLL'" :cols="2" :x-gap="12" :y-gap="12" style="margin-top: 12px">
      <n-gi>
        <div class="input-group">
          <label>计算周期 (Window)</label>
          <n-input-number v-model:value="bollWindow" :min="5" :max="100" />
        </div>
      </n-gi>
      <n-gi>
        <div class="input-group">
          <label>标准差倍数 (Std)</label>
          <n-input-number v-model:value="bollNumStd" :min="1.0" :max="4.0" :step="0.1" />
        </div>
      </n-gi>
    </n-grid>

    <n-grid v-if="strategyType === 'DualThrust'" :cols="2" :x-gap="12" :y-gap="12" style="margin-top: 12px">
      <n-gi>
        <div class="input-group">
          <label>K 突破系数</label>
          <n-input-number v-model:value="dualThrustK" :min="0.1" :max="2.0" :step="0.05" />
        </div>
      </n-gi>
      <n-gi>
        <div class="input-group">
          <label>计算周期 (Period)</label>
          <n-input-number v-model:value="dualThrustPeriod" :min="2" :max="60" />
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

      <!-- 样本内外分离验证 (杜绝过拟合炼金术) -->
      <div v-if="gridResult.in_sample_return !== undefined && gridResult.in_sample_return !== 0" class="oos-validation-card" style="margin-top: 14px; padding: 12px; background: rgba(24, 160, 88, 0.08); border-radius: 8px; border-left: 4px solid #18a058;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px;">
          <span style="font-weight: 600;">🔬 样本内外分离验证 (Train-Test Split 7:3)</span>
          <n-tag :type="gridResult.out_sample_return < 0 ? 'error' : (gridResult.degradation_ratio || 0) > 50 ? 'warning' : 'success'">
            {{ gridResult.overfitting_risk || '样本检验完成' }}
          </n-tag>
        </div>
        <div style="font-size: 12px; color: #888; margin-bottom: 8px;">{{ gridResult.train_test_split_desc }}</div>
        <n-grid :cols="3" :x-gap="8">
          <n-gi>
            <div class="stat-card" style="background: rgba(255,255,255,0.05); padding: 8px; border-radius: 6px;">
              <div class="stat-label" style="font-size: 12px;">训练集(70%)收益</div>
              <div class="stat-value" :class="getReturnClass(gridResult.in_sample_return)">
                {{ formatPct(gridResult.in_sample_return) }}
              </div>
            </div>
          </n-gi>
          <n-gi>
            <div class="stat-card" style="background: rgba(255,255,255,0.05); padding: 8px; border-radius: 6px;">
              <div class="stat-label" style="font-size: 12px;">测试集(30%)盲测收益</div>
              <div class="stat-value" :class="getReturnClass(gridResult.out_sample_return)">
                {{ formatPct(gridResult.out_sample_return) }}
              </div>
            </div>
          </n-gi>
          <n-gi>
            <div class="stat-card" style="background: rgba(255,255,255,0.05); padding: 8px; border-radius: 6px;">
              <div class="stat-label" style="font-size: 12px;">收益衰减率</div>
              <div class="stat-value" :class="(gridResult.degradation_ratio || 0) > 50 ? 'danger' : ''">
                {{ (gridResult.degradation_ratio || 0).toFixed(1) }}%
              </div>
            </div>
          </n-gi>
        </n-grid>
      </div>
    </div>

    <!-- 🏛️ 量化回测参数披露与合规风险提示 -->
    <div class="vectorbt-disclaimer-box" style="margin-top: 16px; padding: 10px 14px; background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255, 255, 255, 0.08); border-radius: 6px; font-size: 11.5px; color: #888; line-height: 1.55;">
      <div style="font-weight: 600; color: #bbb; margin-bottom: 4px;">📊 向量化回测模型参数透明度披露：</div>
      <div>• <strong>样本窗口</strong>：近 250~500 根前复权 K 线 · 包含双向滑点 (0.1%) 与交易手续费 (万2.5)</div>
      <div>• <strong>样本内外验证</strong>：网格寻优提供 70% 训练集 vs 30% 盲测集对比，严防历史过拟合与未来函数</div>
      <div style="margin-top: 6px; border-top: 1px dashed rgba(255, 255, 255, 0.08); padding-top: 6px; color: #777;">
        ⚖️ <strong>【合规风险提示】</strong>：历史回测统计数据仅供量化策略研发参考，过往收益不代表未来表现。实盘交易受流动性、冲击成本与突发黑天鹅影响，请严格控制头寸与防守止损。
      </div>
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
const bollWindow = ref(20)
const bollNumStd = ref(2.0)
const dualThrustK = ref(0.5)
const dualThrustPeriod = ref(5)
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

// 选择股票：规范化代码格式，杜绝前后缀混乱导致的巴别塔现象
function onSelectStock(item: string) {
  if (!item) return
  const clean = item.trim().toLowerCase().replace(/[-_]/g, '')
  if (clean.includes('.')) {
    const parts = clean.split('.')
    stockCode.value = (parts[1] || '') + (parts[0] || '')
  } else if (/^\d{6}$/.test(clean)) {
    const p = clean.startsWith('6') || clean.startsWith('5') ? 'sh' : 'sz'
    stockCode.value = p + clean
  } else {
    stockCode.value = clean
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
  in_sample_return?: number
  out_sample_return?: number
  degradation_ratio?: number
  overfitting_risk?: string
  train_test_split_desc?: string
}

const result = ref<BacktestData | null>(null)
const stats = ref<BacktestStats>({} as BacktestStats)
const gridResult = ref<GridSearchResult | null>(null)

// 策略选项
const strategyOptions = [
  { label: 'MA 均线交叉', value: 'MA' },
  { label: 'MACD 经典趋势', value: 'MACD' },
  { label: 'RSI 超买超卖', value: 'RSI' },
  { label: 'BOLL 布林带均值回归', value: 'BOLL' },
  { label: 'Dual Thrust 区间突破', value: 'DualThrust' },
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
  } else if (strategyType.value === 'BOLL') {
    params.window = bollWindow.value
    params.num_std = bollNumStd.value
  } else if (strategyType.value === 'DualThrust') {
    params.k = dualThrustK.value
    params.period = dualThrustPeriod.value
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
