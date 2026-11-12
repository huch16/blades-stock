<script setup lang="ts">
import { GetStockList, GetConfig } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime'
import StockLightweightKlineChart from './StockLightweightKlineChart.vue'
import StockDiagnosisDrawer from './StockDiagnosisDrawer.vue'
import {
  NAutoComplete, NButton, NFlex, NText, NInputGroup, NTag,
} from 'naive-ui'
import { useMessage } from 'naive-ui'
import { onBeforeMount, onMounted, onBeforeUnmount, ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { setCurrentStock } from '../utils/stockContextBus'

const route = useRoute()
const message = useMessage()
const searchQuery = ref('')
const selectedCode = ref('')
const selectedName = ref('')
const stockList = ref([])
const options = ref([])
const darkTheme = ref(false)
const chartHeight = ref(Math.max(450, window.innerHeight - 150))
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

async function findStockList(val) {
  if (!val || !val.trim()) {
    options.value = []
    return
  }
  const q = val.trim().toLowerCase()
  // 1. 先在内存列表 stockList 中匹配 (支持名称、代码、纯数字symbol、拼音缩写cnspell)
  let filtered = stockList.value.filter(item =>
    (item.name && item.name.toLowerCase().includes(q)) ||
    (item.ts_code && item.ts_code.toLowerCase().includes(q)) ||
    (item.symbol && item.symbol.toLowerCase().includes(q)) ||
    (item.cnspell && item.cnspell.toLowerCase().includes(q))
  ).slice(0, 30)

  // 2. 若内存无结果或全量列表尚未加载完毕，动态向后端 API 请求模糊搜索
  if (filtered.length === 0) {
    try {
      const remote = await GetStockList(val.trim())
      if (Array.isArray(remote) && remote.length > 0) {
        filtered = remote.slice(0, 30)
      }
    } catch (e) {
      console.warn('GetStockList search error:', e)
    }
  }

  options.value = filtered.map(item => ({
    label: (item.name || '') + ' - ' + (item.ts_code || item.symbol || ''),
    value: item.ts_code || item.symbol,
  }))
}

async function handleSearch(value) {
  if (!value) return
  const raw = String(value).trim()
  if (!raw) return

  // 说明（no-useless-assignment）：targetCode 仅作为「名称/拼音 → 代码」解析的中间量，
  // 解析后的最终值统一落在 directEmCode 上。此前它在函数作用域声明并带初值 ''，
  // 但初值从未被读取、else 分支的赋值也从未被使用，现下沉为块级局部变量。
  let targetName = ''

  // 1. 先尝试直接转东财代码（如果是纯数字代码如 600519，或标准格式如 600519.SH、AAPL.US）
  let directEmCode = toEastMoneyCode(raw)

  // 2. 如果输入的是中文股票名（如"贵州茅台"、"茅台"）或拼音缩写（如"gzmt"）
  if (!directEmCode) {
    const q = raw.toLowerCase()
    // 优先从内存列表匹配
    let matched = stockList.value.find(item =>
      (item.name && item.name.toLowerCase() === q) ||
      (item.cnspell && item.cnspell.toLowerCase() === q)
    )
    if (!matched) {
      matched = stockList.value.find(item =>
        (item.name && item.name.toLowerCase().includes(q))
      )
    }

    // 若内存未命中，向后端搜索
    if (!matched) {
      try {
        const remote = await GetStockList(raw)
        if (Array.isArray(remote) && remote.length > 0) {
          matched = remote[0]
        }
      } catch (e) {
        console.warn('handleSearch remote lookup error:', e)
      }
    }

    if (matched) {
      const matchedCode = matched.ts_code || matched.symbol
      targetName = matched.name || ''
      directEmCode = toEastMoneyCode(matchedCode)
    }
  } else {
    const found = stockList.value.find(item => item.ts_code === raw || toEastMoneyCode(item.ts_code) === directEmCode)
    if (found) {
      targetName = found.name
    }
  }

  if (!directEmCode) {
    unsupportedCode.value = true
    return
  }

  unsupportedCode.value = false
  selectedCode.value = directEmCode
  selectedName.value = targetName || selectedName.value || ''
  addToRecent(directEmCode, selectedName.value)
  setCurrentStock({ code: directEmCode, name: selectedName.value })
  searchQuery.value = selectedName.value ? `${selectedName.value} (${directEmCode})` : directEmCode
}

function addToRecent(code, name) {
  if (!code) return
  const list = recentStocks.value.filter(s => s.code !== code)
  list.unshift({ code, name: name || code })
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
  selectedName.value = name || ''
  addToRecent(emCode, name)
  setCurrentStock({ code: emCode, name: selectedName.value })
}

function updateChartHeight() {
  chartHeight.value = Math.max(450, window.innerHeight - 150)
}

function applyStockFromRoute() {
  const queryCode = route.query.code || route.query.ts_code
  const queryName = route.query.name || route.query.stockName
  if (queryCode) {
    if (queryName) selectedName.value = String(queryName)
    handleSearch(String(queryCode))
    return true
  }
  return false
}

// 监听路由参数变化（例如从其它页面跨组件点击跳转）
watch(() => route.query.code, (newCode) => {
  if (newCode) {
    applyStockFromRoute()
  }
})

onBeforeMount(() => {
  GetStockList('').then(result => {
    stockList.value = result || []
    // 股票列表加载完成后再次确认当前选中股票的名称
    if (selectedCode.value && !selectedName.value) {
      const found = stockList.value.find(item => toEastMoneyCode(item.ts_code) === selectedCode.value)
      if (found) selectedName.value = found.name
    }
  }).catch(err => { console.error('GetStockList error:', err) })
  GetConfig().then(result => {
    darkTheme.value = !!result.darkTheme
  }).catch(err => { console.error('GetConfig error:', err) })
})

onMounted(async () => {
  loadRecentStocks()
  updateChartHeight()
  window.addEventListener('resize', updateChartHeight)

  // 1. 优先尝试从 URL 路由参数加载
  const fromRoute = applyStockFromRoute()

  // 2. 若未从路由指定，自动加载最近浏览的第一只股票，避免进入页面时空白
  if (!fromRoute && recentStocks.value.length > 0) {
    const firstRecent = recentStocks.value[0]
    selectRecent(firstRecent.code, firstRecent.name)
  } else if (!fromRoute && !selectedCode.value) {
    // 3. 全新启动无浏览历史时，默认加载贵州茅台 (600519.SH) 作为初始标的，保障开箱立即可用
    selectRecent('600519.SH', '贵州茅台')
  }

  // 3. 监听全局选股事件
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
      addToRecent(emCode, data.name || '')
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
  <div
    class="kline-analysis-page"
    :class="{ 'kline-analysis-page--dark': darkTheme }"
  >
    <!-- 顶部综合控制台：股票名称/代码 + 最近浏览 + 搜索框 + 三维诊断 -->
    <div class="kline-title-bar">
      <NFlex
        justify="space-between"
        align="center"
        :wrap="false"
      >
        <!-- 左侧：选中国股票与快速历史切换标签 -->
        <NFlex
          align="center"
          :size="12"
        >
          <div
            v-if="selectedCode"
            class="kline-stock-badge"
          >
            <NText
              :depth="darkTheme ? 1 : 3"
              class="kline-stock-name"
            >
              {{ selectedName || selectedCode }}&nbsp;
            </NText>
            <NTag
              size="small"
              :bordered="false"
              type="info"
            >
              {{ selectedCode }}
            </NTag>
          </div>
          <div
            v-else
            class="kline-stock-badge"
          >
            <NText
              depth="3"
              style="font-size: 15px; font-weight: 600"
            >
              未选择个股
            </NText>
          </div>

          <!-- 最近浏览快捷标签 -->
          <div
            v-if="recentStocks.length"
            class="recent-stocks"
          >
            <NText
              depth="3"
              style="font-size: 11px; white-space: nowrap"
            >
              历史:
            </NText>
            <NButton
              v-for="s in recentStocks.slice(0, 6)"
              :key="s.code"
              size="tiny"
              quaternary
              :type="s.code === selectedCode ? 'primary' : 'default'"
              @click="selectRecent(s.code, s.name)"
            >
              {{ s.name || s.code }}
            </NButton>
          </div>
        </NFlex>

        <!-- 右侧：搜索栏与 AI 三维量化诊断 -->
        <NFlex
          align="center"
          :size="8"
        >
          <n-input-group style="width: 250px;">
            <n-auto-complete
              v-model:value="searchQuery"
              :options="options"
              placeholder="搜索股票/代码/拼音..."
              clearable
              size="small"
              :on-select="handleSearch"
              @update:value="findStockList"
            />
            <n-button
              size="small"
              type="primary"
              secondary
              @click="handleSearch(searchQuery)"
            >
              🔍
            </n-button>
          </n-input-group>

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

      <NFlex
        v-if="unsupportedCode"
        align="center"
        :size="6"
        style="margin-top: 4px"
      >
        <NText
          type="warning"
          style="font-size: 12px"
        >
          ⚠️ 该股票代码格式暂不支持绘制 K 线图，请尝试规范的 6 位 A 股代码
        </NText>
      </NFlex>
    </div>

    <!-- 主 K 线图渲染区 -->
    <StockLightweightKlineChart
      v-if="selectedCode"
      :key="selectedCode"
      :code="selectedCode"
      :stock-name="selectedName"
      :dark-theme="darkTheme"
      :chart-height="chartHeight"
      :realtime-interval-ms="60000"
    />
    <div
      v-else
      class="kline-empty-placeholder"
    >
      <NText
        depth="3"
        style="font-size: 14px"
      >
        🔍 请在上方搜索框输入股票名称或代码以加载 K 线图与三维诊断
      </NText>
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
  padding: 4px 6px 8px 6px;
  border-bottom: 1px solid var(--n-border-color, rgba(0, 0, 0, 0.06));
  margin-bottom: 6px;
}
.kline-stock-badge {
  display: flex;
  align-items: center;
  gap: 4px;
}
.kline-stock-name {
  font-size: 16px;
  font-weight: 700;
}
.recent-stocks {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}
.kline-empty-placeholder {
  width: 100%;
  height: 380px;
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
