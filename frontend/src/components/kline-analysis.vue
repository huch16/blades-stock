<script setup>
import { GetStockList, GetConfig } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff, EventsEmit } from '../../wailsjs/runtime'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import StockDiagnosisDrawer from './StockDiagnosisDrawer.vue'
import {
  NAutoComplete, NButton, NFlex, NText, NInputGroup,
} from 'naive-ui'
import { useMessage } from 'naive-ui'
import { onBeforeMount, onMounted, onBeforeUnmount, ref, computed } from 'vue'

const message = useMessage()
const searchQuery = ref('')
const selectedCode = ref('')
const selectedName = ref('')
const stockList = ref([])
const options = ref([])
const darkTheme = ref(false)
const chartHeight = ref(window.innerHeight - 230)
const recentStocks = ref([])
const unsupportedCode = ref(false)
let stockChangeHandler = null

// AI 深度量化诊断状态
const diagnosisDrawerVisible = ref(false)
const diagnosisLoading = ref(false)

// 只有在已选择具体股票时才允许三维诊断
const canDiagnose = computed(() => !!selectedCode.value)

function handleDiagnose() {
  if (!canDiagnose.value) {
    message.warning('请先在上方搜索或选择一只具体股票后再发起三维诊断')
    return
  }
  diagnosisDrawerVisible.value = true
}


function toEastMoneyCode(code) {
  if (!code) return ''
  let c = String(code).trim()
  // 兼容 "天目药业 - 600671.SH" 或 "天目药业 600671"
  for (const sep of [' - ', '-', ' ']) {
    if (c.includes(sep)) {
      const parts = c.split(sep)
      c = parts[parts.length - 1].trim()
    }
  }
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return c.toUpperCase()
  const lower = c.toLowerCase()
  if (lower.startsWith('sh')) return lower.slice(2) + '.SH'
  if (lower.startsWith('sz')) return lower.slice(2) + '.SZ'
  if (lower.startsWith('bj')) return lower.slice(2) + '.BJ'
  if (lower.startsWith('hk')) return lower.slice(2).toUpperCase() + '.HK'
  if (lower.startsWith('us')) return lower.slice(2).toUpperCase() + '.US'
  if (lower.startsWith('gb_')) return lower.slice(3).toUpperCase() + '.US'
  if (/^\d+$/.test(c)) {
    const d = c[0]
    if (d === '6' || d === '5') return c + '.SH' // 6: 沪市主板/科创板; 5: 沪市ETF/封闭式基金 (如510300, 588000)
    if (d === '0' || d === '3' || d === '1') return c + '.SZ' // 0: 深市主板; 3: 创业板; 1: 深市ETF/LOF (如159915)
    if (d === '4' || d === '8' || d === '9') return c + '.BJ' // 北交所 / 新三板
    return c + '.SZ'
  }
  // 纯字母代码视为美股（如 AAPL → AAPL.US）
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

function findStockList(val) {
  if (!val || !val.trim()) {
    options.value = []
    return
  }
  const q = val.trim().toLowerCase()
  const filtered = stockList.value.filter(item =>
    item.name.toLowerCase().includes(q) ||
    item.ts_code.toLowerCase().includes(q)
  ).slice(0, 30)
  options.value = filtered.map(item => ({
    label: item.name + ' - ' + item.ts_code,
    value: item.ts_code,
  }))
}

function handleSearch(value) {
  const emCode = toEastMoneyCode(value)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  const found = stockList.value.find(item => item.ts_code === value)
  selectedName.value = found ? found.name : ''
  addToRecent(value, selectedName.value)
}

function addToRecent(code, name) {
  const list = recentStocks.value.filter(s => s.code !== code)
  list.unshift({ code, name })
  if (list.length > 10) list.length = 10
  recentStocks.value = list
  try {
    localStorage.setItem('kline-recent-stocks', JSON.stringify(list))
  } catch {}
}

function loadRecentStocks() {
  try {
    const raw = localStorage.getItem('kline-recent-stocks')
    if (raw) recentStocks.value = JSON.parse(raw)
  } catch {}
}

function selectRecent(code, name) {
  const emCode = toEastMoneyCode(code)
  if (!emCode) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  selectedName.value = name
  addToRecent(code, name)
}

function updateChartHeight() {
  chartHeight.value = Math.max(400, window.innerHeight - 230)
}

onBeforeMount(() => {
  GetStockList('').then(result => {
    stockList.value = result || []
  }).catch(err => { console.error('GetStockList error:', err) })
  GetConfig().then(result => {
    darkTheme.value = !!result.darkTheme
  }).catch(err => { console.error('GetConfig error:', err) })
})

onMounted(async () => {
  loadRecentStocks()
  updateChartHeight()
  window.addEventListener('resize', updateChartHeight)

  stockChangeHandler = (data) => {
    if (data && data.ts_code) {
      const emCode = toEastMoneyCode(data.ts_code)
      if (!emCode) {
        unsupportedCode.value = true
        return
      }
      unsupportedCode.value = false
      selectedCode.value = emCode
      selectedName.value = data.name || ''
      addToRecent(data.ts_code, data.name || '')
    }
  }
  EventsOn('klineSelectStock', stockChangeHandler)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateChartHeight)
  if (stockChangeHandler) {
    EventsOff('klineSelectStock', stockChangeHandler)
  }
})
</script>

<template>
  <div class="kline-analysis-page" :class="{ 'kline-analysis-page--dark': darkTheme }">
    <div class="kline-title-bar">
      <NFlex justify="space-between" align="center">
        <div>
          <NText :depth="darkTheme ? 1 : 3" style="font-size: 16px; font-weight: 700">{{ selectedName }}&nbsp;</NText>
          <NText depth="3" style="font-size: 13px">{{ selectedCode }}</NText>
        </div>
        <NFlex align="center" :size="8">
          <NButton
            type="primary"
            secondary
            size="small"
            :loading="diagnosisLoading"
            :disabled="!canDiagnose"
            @click="handleDiagnose"
          >
            <template #icon>
              <span>✨</span>
            </template>
            AI 量化三维诊断
          </NButton>
        </NFlex>
      </NFlex>
    </div>
    <StockLightweightKlineChart
      v-if="selectedCode"
      :key="selectedCode"
      :code="selectedCode"
      :stockName="selectedName"
      :darkTheme="darkTheme"
      :chartHeight="chartHeight"
      :realtimeIntervalMs="60000"
    />
    <div v-else class="kline-empty-placeholder">
      <NText depth="3" style="font-size: 14px">
        🔍 请在上方搜索框输入股票名称或代码以加载 K 线与三维诊断
      </NText>
    </div>

    <div class="kline-search-bar">
      <n-input-group>
        <n-auto-complete
          v-model:value="searchQuery"
          :options="options"
          placeholder="股票名称/代码搜索..."
          clearable
          :on-select="handleSearch"
          @update:value="findStockList"
        />
        <n-button type="primary">
          🔍
        </n-button>
      </n-input-group>
      <NFlex v-if="unsupportedCode" align="center" :size="6" style="margin-top: 4px">
        <NText type="warning" style="font-size: 12px">该股票暂不支持K线图</NText>
      </NFlex>
      <div v-if="recentStocks.length && !selectedCode" class="recent-stocks">
        <NText depth="3" style="font-size: 11px; white-space: nowrap">最近:</NText>
        <n-button
          v-for="s in recentStocks.slice(0, 6)"
          :key="s.code"
          size="tiny"
          secondary
          @click="selectRecent(s.code, s.name)"
        >
          {{ s.name }}
        </n-button>
      </div>
    </div>

    <!-- AI 结构化三维量化深度诊断抽屉 -->
    <StockDiagnosisDrawer
      v-model:show="diagnosisDrawerVisible"
      :stock-code="selectedCode"
      :stock-name="selectedName"
    />

  </div>
</template>

<style scoped>
.kline-analysis-page {
  width: 100%;
  padding: 4px 8px;
  box-sizing: border-box;
  --wails-draggable: no-drag;
  position: relative;
}
.kline-analysis-page--dark {
  background: #0a0a0a;
  color: #e2e8f0;
}
.kline-title-bar {
  padding: 2px 4px 6px 4px;
}
.kline-search-bar {
  position: fixed;
  bottom: 18px;
  right: 12px;
  z-index: 10;
  width: 320px;
}
.recent-stocks {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  flex-wrap: wrap;
}
.diagnosis-container {
  padding-bottom: 20px;
}
.score-card {
  border-radius: 8px;
}
.score-label {
  font-size: 12px;
  color: #888;
}
.score-value {
  font-size: 32px;
  font-weight: 800;
  line-height: 1.1;
}
.score-max {
  font-size: 14px;
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
.kline-analysis-page--dark .dim-evaluation {
  background: rgba(255, 255, 255, 0.05);
  color: #bbb;
}
.kline-empty-placeholder {
  width: 100%;
  height: 200px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px dashed var(--n-border-color, #ccc);
  border-radius: 6px;
  margin: 12px 0;
  background: rgba(0, 0, 0, 0.02);
}
.kline-analysis-page--dark .kline-empty-placeholder {
  background: rgba(255, 255, 255, 0.05);
}
</style>
