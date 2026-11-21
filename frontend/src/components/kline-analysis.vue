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
import { toEastMoneyCode, stripNameCodeFormat, filterStockOptions, resolveSearchTarget, isValidEastMoneyCode, shouldWaitForStockList, parsePendingOptionCode } from '../utils/stockSearch'

const route = useRoute()
const message = useMessage()
const searchQuery = ref('')
const selectedCode = ref('')
const stockList = ref([])
const stockListLoaded = ref(false)
const options = ref([])

// 搜索竞速守卫：handleSearch / findStockList 都是异步的，旧调用的慢响应
// （如列表未加载时 findStockList 的远程回包、handleSearch 的远程查名）
// 不得覆盖新调用的结果；只有最新一次调用能提交。
let searchGen = 0
let findStockGen = 0
// 全量列表加载中的 promise：回车时若列表未到，handleSearch 等它（不发新请求）。
let stockListLoadPromise: Promise<void> | null = null
// 等全量列表的上限：超时则按现状走（拼音无建议时仍补 .US），不让回车无感地挂起。
const STOCK_LIST_WAIT_MS = 3000
// 方向键是否明确选中过联想建议：naive-ui 的 n-auto-complete 会把下拉首项
// 自动设为 pending（auto-pending），回车会选中它。只在用户按过方向键后，
// 回车才按建议走；否则回车/放大镜一律按输入原文解析（问题 2）。
let arrowNavSelected = false
// 方向键选中后回车：naive-ui 的 keydown 选中建议后，keyup.enter 还会再触发一次，
// 用此标志跳过那次重复解析（此时输入框已被改写为建议的 label）。
let suppressEnterKeyup = false
// 方向键选中的建议值：每次方向键 keyup 后从 naive-ui 的 pending 项快照
// （handleSearchKeyup），回车以它为准，不受随后鼠标悬停另一建议的干扰。
// naive-ui 不暴露 pending 值，快照读不到时为空，调用方回退到旧行为。
let keyboardSelectedValue = ''
// 方向键仍被按住（keyup 未到）时快照可能过期：回车不走快照，放行给 naive-ui。
let arrowKeyDownPending = false

// 联想列表一变，方向键选中即失效（旧建议已不在列表里），回车回到原文解析。
watch(options, () => { arrowNavSelected = false; keyboardSelectedValue = ''; arrowKeyDownPending = false })
const darkTheme = ref(false)
const chartHeight = ref(Math.max(450, window.innerHeight - 150))
const recentStocks = ref([])
const unsupportedCode = ref(false)
let stockChangeHandler = null

// 名称由代码派生（单一事实来源）：优先用已加载的股票列表按代码查名，
// 其次用历史记录里该代码对应的名字；两者都没有时为空，界面回退显示代码。
// 列表还没加载完时，不拿历史记录的名字来凑——旧版本留下的错配条目
// （如 code=000001.SZ 却存着“东方财富”）会趁虚而入；此时显示代码，列表一到就自动纠正。
const selectedName = computed(() => {
  if (!selectedCode.value) return ''
  const found = stockList.value.find(item => toEastMoneyCode(item.ts_code) === selectedCode.value)
  if (found && found.name) return found.name
  if (!stockListLoaded.value) return ''
  const recent = recentStocks.value.find(s => s.code === selectedCode.value)
  return recent?.name || ''
})

// 按代码查权威名称（股票列表优先），查不到时用传入的后备名。
function resolveStockName(code, fallback = '') {
  if (!code) return fallback
  const found = stockList.value.find(item => toEastMoneyCode(item.ts_code) === code)
  if (found && found.name) return found.name
  return fallback
}

// 按代码查权威名称（异步）：本地列表优先；列表还没加载完或查不到时，
// 向后端查一次（便宜的读接口），这样搜索/切换时不依赖全量列表是否已加载完。
async function lookupStockName(code) {
  if (!code) return ''
  const local = resolveStockName(code, '')
  if (local) return local
  try {
    const remote = await GetStockList(code)
    if (Array.isArray(remote) && remote.length > 0) {
      const m = remote.find(item => toEastMoneyCode(item.ts_code) === code) || remote[0]
      if (m && m.name) return m.name
    }
  } catch (e) {
    console.warn('lookupStockName remote error:', e)
  }
  return ''
}

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

async function findStockList(val) {
  // 每次调用都推进代次，使之前未完成的远程请求回包后失效。
  const myGen = ++findStockGen
  if (!val || !val.trim()) {
    options.value = []
    return
  }
  const q = val.trim()
  // 1. 先在内存列表 stockList 中匹配 (支持名称、代码、纯数字symbol、拼音缩写cnspell)
  let filtered = filterStockOptions(stockList.value, q, 30)

  // 2. 若内存无结果或全量列表尚未加载完毕，动态向后端 API 请求模糊搜索
  if (filtered.length === 0) {
    try {
      const remote = await GetStockList(q)
      // 旧输入的慢回包不得覆盖新输入的联想列表：列表未加载时每次按键都会发
      // 远程请求，先发的 “0”（命中上千条、最慢）可能比后发的 “000001” 晚到，
      // 此前它会把 options 写回成 “0” 的结果，首项被回车选中即误跳（问题 2）。
      if (myGen !== findStockGen) return
      if (searchQuery.value.trim() !== q) return
      if (Array.isArray(remote) && remote.length > 0) {
        filtered = remote.slice(0, 30)
      }
    } catch (e) {
      console.warn('GetStockList search error:', e)
    }
  }

  if (myGen !== findStockGen) return
  options.value = filtered.map(item => ({
    label: (item.name || '') + ' - ' + (item.ts_code || item.symbol || ''),
    value: item.ts_code || item.symbol,
  }))
}

async function handleSearch(value, nameHint = '') {
  // 每次调用都推进代次：回车与放大镜可能连续触发两次 handleSearch，
  // 旧调用的慢响应（远程查名）不得覆盖新调用的结果（问题 2：放大镜点击
  // 后回车的误选曾把点击的结果覆盖掉）。
  const myGen = ++searchGen
  if (!value) return
  // 剥掉 handleSearch 自己写出的 “名字 (代码)” 格式（如 “贵州茅台 (000001.SZ)”），
  // 否则 toEastMoneyCode 按空格切分会得到 “(000001.SZ)” 而解析失败，导致切不动股票。
  const raw = stripNameCodeFormat(value)
  if (!raw) return

  // 回车/放大镜：输入本身不是合法代码原文（如拼音 gzmt）且当前联想列表
  // 恰好只有 1 条时，优先用该建议（问题 A：gzmt → 贵州茅台 600519.SH）；
  // 否则按原文解析（000001 → 000001.SZ、AAPL → AAPL.US 均不变）。
  // options 是回车时刻实际持有的联想列表：本地过滤在 findStockList 里同步
  // 写回，远程回包受 generation 守卫保护，过期结果写不进来。
  // 全量列表未加载完且联想为空时，拼音/名称输入无法判断：后端 GetStockList
  // 不按 cnspell 匹配，远程查不到拼音；等一次已在路上的列表加载（不发新
  // 请求），最多等 STOCK_LIST_WAIT_MS，超时或加载失败则按现状走。
  // 列表到了就用本地数据重算联想，再按同一规则取目标。
  let suggestionCodes = options.value.map((o) => o.value)
  if (stockListLoadPromise && shouldWaitForStockList(raw, suggestionCodes, stockListLoaded.value)) {
    await Promise.race([
      stockListLoadPromise,
      new Promise((resolve) => setTimeout(resolve, STOCK_LIST_WAIT_MS)),
    ])
    if (myGen !== searchGen) return
    if (stockListLoaded.value) {
      suggestionCodes = filterStockOptions(stockList.value, raw, 30).map((item) => item.ts_code || item.symbol)
    }
  }
  // handleSelectOption 传进来的已是合法代码（600519.SH），此规则对它无影响。
  const target = resolveSearchTarget(raw, '', false, suggestionCodes)

  // 说明（no-useless-assignment）：targetCode 仅作为「名称/拼音 → 代码」解析的中间量，
  // 解析后的最终值统一落在 directEmCode 上。此前它在函数作用域声明并带初值 ''，
  // 但初值从未被读取、else 分支的赋值也从未被使用，现下沉为块级局部变量。
  let targetName = ''

  // 1. 先尝试直接转东财代码（如果是纯数字代码如 600519，或标准格式如 600519.SH、AAPL.US）
  let directEmCode = toEastMoneyCode(target)

  // 2. 如果输入的是中文股票名（如"贵州茅台"、"茅台"）或拼音缩写（如"gzmt"）
  if (!directEmCode) {
    const q = target.toLowerCase()
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
        const remote = await GetStockList(target)
        if (myGen !== searchGen) return
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
    const found = stockList.value.find(item => item.ts_code === target || toEastMoneyCode(item.ts_code) === directEmCode)
    if (found) {
      targetName = found.name
    } else {
      // 本地列表还没加载完或查不到时，向后端查一次拿权威名称，
      // 否则名称只能等列表加载完才纠正，中间会显示旧名或代码。
      targetName = await lookupStockName(directEmCode)
      if (myGen !== searchGen) return
    }
  }

  // 格式守门：toEastMoneyCode 对任意位数字都会补后缀（如 600500000119 →
  // 600500000119.SH），非法代码到这里必须拦下，不进 selectedCode、不写历史。
  if (!isValidEastMoneyCode(directEmCode)) {
    if (myGen !== searchGen) return
    unsupportedCode.value = true
    return
  }

  if (myGen !== searchGen) return
  unsupportedCode.value = false
  selectedCode.value = directEmCode
  // 名称不再单独存一份可漂移的状态：先把解析到的名字写进历史记录，
  // 再读派生出来的 selectedName（列表优先，其次历史记录），保证名、码、图三者一致。
  // 查名失败时 targetName 为空，addToRecent 会保留该代码已存的好名字或回退到代码本身，
  // 不会再把上一只股票的名字配到新代码上。
  if (!targetName && nameHint) targetName = nameHint
  addToRecent(directEmCode, targetName)
  setCurrentStock({ code: directEmCode, name: selectedName.value })
  searchQuery.value = selectedName.value ? `${selectedName.value} (${directEmCode})` : directEmCode
}

// 搜索框 keydown 拦截（经 :input-props 挂在原生 input 上，在事件冒泡到
// naive-ui 的 auto-complete 根节点之前触发；见 naive-ui 2.44.1
// AutoComplete.handleKeyDown：回车时若下拉有 pending 项就会选中它）。
// naive-ui 的 NInternalSelectMenu 以 auto-pending: true 渲染，联想列表一变
// 就把首项设为 pending（getNode(输入文本) 查不到 → getFirstAvailableNode()），
// 且鼠标悬停也会改 pending——两者都不是用户的明确选中。
// - 输入法组合中（isComposing）时不动，让输入法与 naive-ui 自行处理；
// - 方向键：只记录“用户明确用方向键选中过建议”，放行给 naive-ui 移动 pending；
//   keyup 时再把移动后的 pending 快照到 keyboardSelectedValue（见 handleSearchKeyup）；
// - 回车：方向键选中过 → 以快照值直接选中，不再放行给 naive-ui，以免随后
//   的鼠标悬停改写 pending 导致回车选错；快照缺失或方向键仍按住时回退到
//   旧行为（放行给 naive-ui 选 pending），并抑制随后 keyup 的重复解析；
//   否则 → 拦截掉 naive-ui 的自动选中，随后 keyup 按输入原文解析。
function handleSearchKeydown(e) {
  if (e.isComposing) return
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    if (options.value.length > 0) {
      arrowNavSelected = true
      arrowKeyDownPending = true
    }
    return
  }
  if (e.key === 'Escape') {
    // 菜单已关，方向键选中随之作废：否则随后回车会选一个已被关掉的建议。
    arrowNavSelected = false
    keyboardSelectedValue = ''
    arrowKeyDownPending = false
    return
  }
  if (e.key === 'Enter') {
    if (arrowNavSelected) {
      if (!arrowKeyDownPending && keyboardSelectedValue) {
        e.stopPropagation()
        suppressEnterKeyup = true
        handleSelectOption(keyboardSelectedValue)
        return
      }
      suppressEnterKeyup = true
      return
    }
    e.stopPropagation()
  }
}

// 读 naive-ui 当前 pending 建议的代码值（naive-ui 2.44.1：pending 项带
// n-base-select-option--pending class，label 为 “名字 - 代码”；
// 本页无其他 auto-complete/select 菜单，不会读到别处）。
// 读不到、解析不出、或不在当前联想列表里时返回 ''（调用方回退到旧行为）。
// 注意：本应用未自定义 naive-ui 的 cls-prefix，默认前缀即 n；
// 将来升级 naive-ui 改了内部 class 时本函数返回 ''，只是回退到旧行为，
// 不会选错股票。
function readKeyboardPendingValue() {
  if (typeof document === 'undefined') return ''
  const el = document.querySelector('.n-base-select-option--pending')
  if (!el) return ''
  return parsePendingOptionCode(el.textContent || '', options.value.map((o: { value: string }) => o.value))
}

// 方向键 keyup：naive-ui 已在 keydown 里移动 pending，此时快照即方向键
// 选中的项；随后鼠标悬停再改 pending 也不会改写快照。方向键仍按住时
// keyup 未到，快照保持旧值，回车时由 arrowKeyDownPending 挡下，
// 不用可能过期的快照。
function handleSearchKeyup(e: KeyboardEvent) {
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    arrowKeyDownPending = false
    if (arrowNavSelected) {
      keyboardSelectedValue = readKeyboardPendingValue()
    }
  }
}

// keyup.enter：精确代码优先于联想建议；拼音/名称输入且当前联想恰好唯一时
// 用建议（问题 A：gzmt → 600519.SH，见 handleSearch 里的 resolveSearchTarget）。
// 方向键选中后回车走 handleSelectOption（快照值直接调，或放行给 naive-ui
// 选 pending），本函数跳过，避免对已被改写为建议 label 的
// 输入框再解析一次。
function handleSearchEnter() {
  if (suppressEnterKeyup) {
    suppressEnterKeyup = false
    return
  }
  handleSearch(searchQuery.value)
}

// 联想建议被明确选中时才触发：点击下拉项，或方向键选中后回车。
// （直接回车时 handleSearchKeydown 已拦截 naive-ui 的自动选中，
// 所以这里不会收到“首项自动高亮”的误触发。）
function handleSelectOption(value) {
  arrowNavSelected = false
  keyboardSelectedValue = ''
  arrowKeyDownPending = false
  handleSearch(value)
}

// 输入框失焦：菜单关闭，方向键选中随之失效；回车回到原文解析。
function onSearchBlur() {
  arrowNavSelected = false
  suppressEnterKeyup = false
  keyboardSelectedValue = ''
  arrowKeyDownPending = false
}

function addToRecent(code, name) {
  if (!code) return
  if (!isValidEastMoneyCode(code)) return // 非法代码不写历史（最后一道防线）
  // 存之前先按代码查权威名称：列表里有就以列表为准（能纠正旧版本留下的错配条目）。
  // 注意：不沿用该代码旧存的名字——它可能是旧 bug 留下的错配（如 000001.SZ 存着“东方财富”），
  // 解析不到就存空，界面显示代码，等列表加载后 validateRecentNames 再纠正。
  const resolvedName = resolveStockName(code, name)
  const list = recentStocks.value.filter(s => s.code !== code)
  list.unshift({ code, name: resolvedName })
  if (list.length > 10) list.length = 10
  recentStocks.value = list
  try {
    localStorage.setItem('kline-recent-stocks', JSON.stringify(list))
  } catch {}
}

// 股票列表加载完成后，按代码校正历史记录里的名字，
// 清掉旧版本 bug 留下的名码错配条目（如 code=000001.SZ 却存着“东方财富”）。
function validateRecentNames() {
  // 格式非法条目（如已存入的 600500000119.SH）直接剔除，不等列表加载；
  // 否则它会一直留在历史里，下次刷新又被当作 selectedCode 恢复。
  const valid = recentStocks.value.filter(s => s && isValidEastMoneyCode(s.code))
  if (valid.length !== recentStocks.value.length) {
    recentStocks.value = valid
    try {
      localStorage.setItem('kline-recent-stocks', JSON.stringify(recentStocks.value))
    } catch {}
  }
  if (!recentStocks.value.length || !stockList.value.length) return
  let changed = false
  recentStocks.value = recentStocks.value.map(s => {
    const found = stockList.value.find(item => toEastMoneyCode(item.ts_code) === s.code)
    if (found && found.name && found.name !== s.name) {
      changed = true
      return { code: s.code, name: found.name }
    }
    return s
  })
  if (changed) {
    try {
      localStorage.setItem('kline-recent-stocks', JSON.stringify(recentStocks.value))
    } catch {}
  }
}

function loadRecentStocks() {
  try {
    const raw = localStorage.getItem('kline-recent-stocks')
    if (raw) {
      const parsed = JSON.parse(raw)
      // 非数组或含格式非法代码（如 600500000119.SH）的条目直接丢弃，
      // 不让它成为恢复 selectedCode 的候选。
      recentStocks.value = Array.isArray(parsed)
        ? parsed.filter(s => s && typeof s.code === 'string' && isValidEastMoneyCode(s.code))
        : []
    }
  } catch {}
  validateRecentNames()
}

async function selectRecent(code, name) {
  const emCode = toEastMoneyCode(code)
  if (!isValidEastMoneyCode(emCode)) {
    unsupportedCode.value = true
    return
  }
  unsupportedCode.value = false
  selectedCode.value = emCode
  // 名字以权威查询为准（本地列表优先，否则向后端查一次），不直接信任传入的旧名字——
  // 历史条目可能是旧 bug 留下的错配。查完后若用户已切到别的股票，就不再回写。
  const resolved = await lookupStockName(emCode)
  if (selectedCode.value !== emCode) return
  const finalName = resolved || name || ''
  addToRecent(emCode, finalName)
  setCurrentStock({ code: emCode, name: selectedName.value })
  searchQuery.value = selectedName.value ? `${selectedName.value} (${emCode})` : emCode
}

function updateChartHeight() {
  chartHeight.value = Math.max(450, window.innerHeight - 150)
}

function applyStockFromRoute() {
  const queryCode = route.query.code || route.query.ts_code
  const queryName = route.query.name || route.query.stockName
  if (queryCode) {
    handleSearch(String(queryCode), queryName ? String(queryName) : '')
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
  // 末尾接 catch 使该 promise 永不 reject：handleSearch 等它时无需再包 try/catch。
  stockListLoadPromise = GetStockList('').then(result => {
    stockList.value = result || []
    stockListLoaded.value = true
    // 列表加载完成后校正一次历史记录里的名字（清掉旧版本留下的名码错配条目），
    // 界面上的名称由 selectedCode 派生，列表一到就自动纠正，无需再单独赋值。
    validateRecentNames()
  }).catch(err => { console.error('GetStockList error:', err); stockListLoaded.value = true })
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
      if (!isValidEastMoneyCode(emCode)) {
        unsupportedCode.value = true
        return
      }
      unsupportedCode.value = false
      selectedCode.value = emCode
      addToRecent(emCode, data.name || '')
      searchQuery.value = selectedName.value ? `${selectedName.value} (${emCode})` : emCode
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
              :input-props="{ onKeydown: handleSearchKeydown, onKeyup: handleSearchKeyup }"
              placeholder="搜索股票/代码/拼音..."
              clearable
              size="small"
              :on-select="handleSelectOption"
              @update:value="findStockList"
              @keyup.enter="handleSearchEnter"
              @blur="onSearchBlur"
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
