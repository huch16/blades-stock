<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted, onUnmounted, ref, reactive} from 'vue'
import {GetStockChanges, GetConfig, GetStockChangeHistory, SaveStockChangesToHistory, GetAllStockChangesWithPaging} from "../../wailsjs/go/main/App";
import {NButton, NTag, NText, useMessage, useNotification} from "naive-ui";

const notify = useNotification()
const message = useMessage()
const loadingRef = ref(true)
const dataRef = ref([])
const autoRefresh = ref(true)
const refreshInterval = ref(null)
const refreshSeconds = ref(10)
const countdown = ref(10)

const viewMode = ref('realtime')
const isTrading = ref(false)
const marketStatus = ref('')

const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 50,
  itemCount: 0,
  keyword: "",
  range: null,
  startTime: null,
  endTime: null,
  minVolume: null,
  minAmount: null,
  minChangeRate: null,
  maxChangeRate: null,
  industry: "",
  concept: "",
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const volumeOptions = [
  { label: '不限', value: null },
  { label: '>100手', value: 10000 },
  { label: '>200手', value: 20000 },
  { label: '>500手', value: 50000 },
  { label: '>1000手', value: 100000 },
  { label: '>2000手', value: 200000 },
  { label: '>5000手', value: 500000 },
  { label: '>10000手', value: 1000000 },
  { label: '>20000手', value: 2000000 },
  { label: '>50000手', value: 5000000 },
]

const amountOptions = [
  { label: '不限', value: null },
  { label: '>100万', value: 1000000 },
  { label: '>500万', value: 5000000 },
  { label: '>1000万', value: 10000000 },
  { label: '>2000万', value: 20000000 },
  { label: '>5000万', value: 50000000 },
  { label: '>1亿', value: 100000000 },
  { label: '>2亿', value: 200000000 },
  { label: '>5亿', value: 500000000 },
]

const changeRateOptions = [
  { label: '不限', value: null },
  { label: '>3%', value: 3 },
  { label: '>5%', value: 5 },
  { label: '>7%', value: 7 },
  { label: '>9%', value: 9 },
  { label: '>涨停', value: 9.9 },
]

const bullishTypes = [
  {label: '火箭发射', value: '8201'},
  {label: '快速反弹', value: '8202'},
  {label: '大笔买入', value: '8193'},
  {label: '封涨停板', value: '4'},
  {label: '打开跌停板', value: '32'},
  {label: '有大买盘', value: '64'},
  {label: '竞价上涨', value: '8207'},
  {label: '高开5日线', value: '8209'},
  {label: '向上缺口', value: '8211'},
  {label: '60日新高', value: '8213'},
  {label: '60日大幅上涨', value: '8215'},
  {label: '打开涨停板', value: '16'},
]

const bearishTypes = [
  {label: '加速下跌', value: '8204'},
  {label: '高台跳水', value: '8203'},
  {label: '大笔卖出', value: '8194'},
  {label: '封跌停板', value: '8'},
  {label: '有大卖盘', value: '128'},
  {label: '竞价下跌', value: '8208'},
  {label: '低开5日线', value: '8210'},
  {label: '向下缺口', value: '8212'},
  {label: '60日新低', value: '8214'},
  {label: '60日大幅下跌', value: '8216'},
]

const allTypeValues = [...bullishTypes, ...bearishTypes].map(t => t.value)
const selectedTypes = ref(allTypeValues)

const columnsRef = ref([
  {
    title: '日期',
    key: 'changeDate',
    width: 120,
    render(row) {
      const date = row.changeDate || row.ChangeDate
      const time = row.changeTime || row.ChangeTime || row.time
      if (date) {
        return h(NText, {type: 'info'}, {default: () => date + ' ' + time})
      }
      return h(NText, {type: 'info'}, {default: () => time})
    }
  },
  {
    title: '代码',
    key: 'code',
    width: 80,
    render(row) {
      const code = row.stockCode || row.StockCode || row.code
      return h(NText, {type: 'info', style: 'cursor: pointer', onClick: () => copyCode(code)}, {default: () => code})
    }
  },
  {
    title: '名称',
    key: 'name',
    width: 100,
    render(row) {
      const name = row.stockName || row.StockName || row.name || '-'
      return h(NText, {
        strong: true,
        style: 'cursor: pointer; color: #2080f0; text-decoration: underline;',
        onClick: () => openKlineModal(row)
      }, () => name)
    }
  },
  {
    title: '异动类型',
    key: 'typeName',
    width: 90,
    render(row) {
      const typeName = row.typeName || row.TypeName
      const bullishSet = new Set(['火箭发射', '快速反弹', '大笔买入', '封涨停板', '打开跌停板', '有大买盘', '竞价上涨', '高开5日线', '向上缺口', '60日新高', '60日大幅上涨', '打开涨停板'])
      const bearishSet = new Set(['加速下跌', '高台跳水', '大笔卖出', '封跌停板', '有大卖盘', '竞价下跌', '低开5日线', '向下缺口', '60日新低', '60日大幅下跌'])
      
      let tagType = 'default'
      if (bullishSet.has(typeName)) {
        tagType = 'error'
      } else if (bearishSet.has(typeName)) {
        tagType = 'success'
      }
      return h(NTag, {type: tagType, size: 'small'}, {default: () => typeName})
    }
  },
  {
    title: '价格',
    key: 'price',
    width: 70,
    render(row) {
      const price = row.price || row.Price
      if (price > 0) {
        return price.toFixed(2)
      }
      return '-'
    }
  },
  {
    title: '涨跌幅(%)',
    key: 'changeRate',
    width: 85,
    render(row) {
      const changeRate = row.changeRate || row.ChangeRate
      if (changeRate !== 0) {
        const color = changeRate > 0 ? '#dc2626' : '#16a34a'
        const prefix = changeRate > 0 ? '+' : ''
        return h('span', {style: {color: color, fontWeight: '500'}}, prefix + changeRate.toFixed(2) + '%')
      }
      return '-'
    }
  },
  {
    title: '成交量',
    key: 'volume',
    width: 80,
    render(row) {
      const volume = row.volume || row.Volume
      if (volume > 0) {
        return formatVolume(volume)
      }
      return '-'
    }
  },
  {
    title: '金额',
    key: 'amount',
    width: 100,
    render(row) {
      const amount = row.amount || row.Amount
      if (amount > 0) {
        return formatAmount(amount)
      }
      return '-'
    }
  },
  {
    title: '行业',
    key: 'industry',
    width: 80,
    ellipsis: {
      tooltip: true
    },
    render(row) {
      return row.industry || row.Industry || '-'
    }
  },
  {
    title: '概念',
    key: 'concept',
    minWidth: 200,
    render(row) {
      const text = row.concept || row.Concept || '-'
      return h('div', {style: 'white-space: normal; line-height: 1.4;'}, text)
    }
  },
  {
    title: '操作',
    key: 'actions',
    width: 200,
    fixed: 'right',
    render(row) {
      return h(NSpace, { size: 6, justify: 'start' }, () => [
        h(NButton, {
          size: 'tiny',
          type: 'primary',
          secondary: true,
          onClick: () => triggerAiAnomalyAnalysis(row)
        }, () => '⚡ 排雷'),
        h(NButton, {
          size: 'tiny',
          secondary: true,
          onClick: () => openKlineModal(row)
        }, () => '📈 K线'),
        h(NButton, {
          size: 'tiny',
          type: 'info',
          secondary: true,
          onClick: () => openDiagnosisDrawer(row)
        }, () => '📊 诊断')
      ])
    }
  },
])

function formatVolume(vol) {
  const lots = vol / 100
  if (lots >= 100000000) {
    return (lots / 100000000).toFixed(2) + '亿手'
  } else if (lots >= 10000) {
    return (lots / 10000).toFixed(2) + '万手'
  } else if (lots >= 1) {
    return lots.toFixed(0) + '手'
  }
  return vol + '股'
}

function formatAmount(amount) {
  if (amount >= 100000000) {
    return (amount / 100000000).toFixed(2) + '亿'
  } else if (amount >= 10000) {
    return (amount / 10000).toFixed(2) + '万'
  }
  return amount.toFixed(2)
}

function copyCode(code) {
  navigator.clipboard.writeText(code).then(() => {
    message.success('已复制: ' + code)
  })
}

function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
  if (c.startsWith('6') || c.startsWith('9')) return c + '.SH'
  if (c.startsWith('0') || c.startsWith('3')) return c + '.SZ'
  if (c.startsWith('4') || c.startsWith('8')) return c + '.BJ'
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return c
}

function openKlineModal(row) {
  const code = row.stockCode || row.StockCode || row.code || ''
  const name = row.stockName || row.StockName || row.name || ''
  const em = toEastMoneyCode(code)
  if (!em) {
    message.warning('无效的股票代码')
    return
  }
  klineCode.value = em
  klineName.value = name
  showKlineModal.value = true
}

function openDiagnosisDrawer(row) {
  const code = row.stockCode || row.StockCode || row.code || ''
  const name = row.stockName || row.StockName || row.name || ''
  diagnosisStockCode.value = code
  diagnosisStockName.value = name
  showDiagnosisDrawer.value = true
}

function triggerAiAnomalyAnalysis(row) {
  const code = row.stockCode || row.StockCode || row.code || ''
  const name = row.stockName || row.StockName || row.name || ''
  const typeName = row.typeName || row.TypeName || '异动'
  const date = row.changeDate || row.ChangeDate || ''
  const time = row.changeTime || row.ChangeTime || row.time || ''
  const timeStr = date ? `${date} ${time}` : time
  const price = row.price || row.Price || 0
  const changeRate = row.changeRate || row.ChangeRate || 0
  const amount = row.amount || row.Amount || 0
  const amountStr = amount > 0 ? formatAmount(amount) : '-'
  const industry = row.industry || row.Industry || '未知行业'
  const concept = row.concept || row.Concept || '无'

  const prompt = `【股票盘中异动归因与排雷排查】
标的：${name} (${code})
异动时间：${timeStr}
异动信号：${typeName}
盘面数据：现价 ${price > 0 ? price.toFixed(2) + '元' : '-'}，日内涨幅 ${changeRate >= 0 ? '+' : ''}${changeRate.toFixed(2)}%，异动金额 ${amountStr}
所属行业：${industry}
核心概念：${concept}

请以 A 股短线实战游资与量化风控专家视角，用 3 点通俗干练的分析（150字以内）给出排雷指导：
1. 【异动归因】：本次异动大概率属于主线合力真突破、板块脉冲跟风，还是主力拉高诱多/恐慌砸盘？
2. 【排雷风险排查】：近期是否存在解禁减持、业绩预亏、高位筹码松动或重大黑天鹅风险？
3. 【实战防套锦囊】：个人散户此刻应当追高、观望等待回调，还是果断止损防飞刀？给出明确防套点位与纪律。`

  EventsEmit('openFloatingAgentWithQuestion', { question: prompt, agentMode: 'direct' })
  message.success(`已发起 [${name}] 异动 AI 归因排雷推演！`)
}

async function triggerAiMarketAnomalySummary() {
  let list = dataRef.value || []
  let modeDesc = viewMode.value === 'realtime' ? '盘中实时' : '历史回溯'
  let totalCount = paginationReactive.itemCount

  // 如果当前数据为空（如非交易时段、或实时接口尚未返回），自动拉取最新异动兜底数据
  if (!list.length) {
    loadingRef.value = true
    const loadingMsg = message.loading('当前暂无实时异动，正在提取最新交易日异动数据...', { duration: 0 })
    try {
      // 1. 先尝试获取当天全部异动数据
      const allRes = await GetAllStockChangesWithPaging(100)
      if (allRes && allRes.data && allRes.data.length > 0) {
        list = allRes.data
        totalCount = allRes.totalCount || list.length
        dataRef.value = list
        paginationReactive.itemCount = totalCount
        modeDesc = '当日异动归档'
      } else {
        // 2. 若当日接口无数据（如周末休市），从本地历史异动库获取最新记录
        const histRes = await GetStockChangeHistory({ page: 1, pageSize: 50 })
        if (histRes && histRes.list && histRes.list.length > 0) {
          list = histRes.list
          totalCount = histRes.total || list.length
          dataRef.value = list
          paginationReactive.itemCount = totalCount
          const sampleDate = list[0]?.changeDate || list[0]?.ChangeDate || '最新'
          modeDesc = `历史交易日 (${sampleDate})`
        }
      }
    } catch (e) {
      console.warn('获取兜底异动数据失败:', e)
    } finally {
      loadingMsg.destroy()
      loadingRef.value = false
    }
  }

  if (!list.length) {
    message.warning('未能获取到异动样本数据，请稍后重试或切换至历史数据模式查询。')
    return
  }

  const bullishList = []
  const bearishList = []
  const industryCount = {}

  for (const item of list.slice(0, 60)) {
    const name = item.stockName || item.StockName || item.name || ''
    const code = item.stockCode || item.StockCode || item.code || ''
    const typeName = item.typeName || item.TypeName || ''
    const ind = item.industry || item.Industry || ''
    if (ind && ind !== '-') {
      industryCount[ind] = (industryCount[ind] || 0) + 1
    }
    const isBull = ['火箭发射', '快速反弹', '大笔买入', '封涨停板', '60日新高', '有大买盘', '竞价上涨', '高开5日线'].includes(typeName)
    const label = `${name}${code ? '(' + code + ')' : ''} [${typeName}]`
    if (isBull && bullishList.length < 6) {
      bullishList.push(label)
    } else if (!isBull && bearishList.length < 6) {
      bearishList.push(label)
    }
  }

  const hotIndustries = Object.entries(industryCount)
    .sort((a, b) => b[1] - a[1])
    .slice(0, 4)
    .map(([k, v]) => `${k}(${v}次)`)
    .join('、') || '暂无高度集中行业'

  const prompt = `【日内异动大盘资金全景透视】
数据状态：${modeDesc}（异动样本总数：${totalCount} 条）
异动聚焦行业：${hotIndustries}
典型异动进攻代表：${bullishList.join('、') || '无明显进攻信号'}
典型异动承压代表：${bearishList.join('、') || '无明显跳水抛压'}

请结合以上盘面集中异动特征，从短线主力与游资博弈视角，输出结构化全景透视（包含以下 3 个维度）：
1. 【主力资金与行业主线】：主力大单主要在攻坚哪个赛道？属于真金白银建仓还是拉高诱多？
2. 【虚实辨析与持续性】：进攻型异动标的是否具备板块共振？哪些容易是脉冲式一日游/飞刀？
3. 【散户避坑与实操风控】：针对当前异动结构，给出明确的防追高点位策略与防套纪律。`

  EventsEmit('openFloatingAgentWithQuestion', { question: prompt, agentMode: 'direct' })
  message.success('已提炼盘面异动，正在右侧 AI 助手展开全景透视...', { duration: 3500 })
}

function checkTradingTime() {
  const now = new Date()
  const day = now.getDay()
  const hour = now.getHours()
  const minute = now.getMinutes()
  const currentTime = hour * 100 + minute

  if (day === 0 || day === 6) {
    isTrading.value = false
    marketStatus.value = '休市（周末）'
    return
  }

  const morningStart = 915
  const morningEnd = 1130
  const afternoonStart = 1257
  const afternoonEnd = 1500

  if (currentTime >= morningStart && currentTime <= morningEnd) {
    isTrading.value = true
    if (currentTime < 930) {
      marketStatus.value = '集合竞价'
    } else {
      marketStatus.value = '上午交易'
    }
  } else if (currentTime >= afternoonStart && currentTime <= afternoonEnd) {
    isTrading.value = true
    if (currentTime < 1300) {
      marketStatus.value = '午间集合竞价'
    } else {
      marketStatus.value = '下午交易'
    }
  } else if (currentTime > morningEnd && currentTime < afternoonStart) {
    isTrading.value = false
    marketStatus.value = '午间休市'
  } else if (currentTime > afternoonEnd) {
    isTrading.value = false
    marketStatus.value = '已收盘'
  } else {
    isTrading.value = false
    marketStatus.value = '未开盘'
  }
}

async function fetchRealtimeData() {
  loadingRef.value = true
  try {
    const types = selectedTypes.value.map(t => parseInt(t))
    const result = await GetStockChanges(types, 0, paginationReactive.pageSize)
    if (result && result.data && result.data.length > 0) {
      dataRef.value = result.data || []
      paginationReactive.itemCount = result.totalCount || 0
    } else if (!isTrading.value) {
      // 非交易时间实时推送为空时，自动加载历史库最新记录填充表格，避免用户打开页面空空如也
      const histResult = await GetStockChangeHistory({ page: 1, pageSize: paginationReactive.pageSize })
      if (histResult && histResult.list && histResult.list.length > 0) {
        dataRef.value = histResult.list
        paginationReactive.itemCount = histResult.total || 0
      }
    }
  } catch (e) {
    console.error('获取异动数据失败:', e)
  } finally {
    loadingRef.value = false
  }
}

async function fetchHistoryData() {
  loadingRef.value = true
  try {
    const query = {
      page: paginationReactive.page,
      pageSize: paginationReactive.pageSize,
    }
    if (paginationReactive.range && paginationReactive.range.length === 2) {
      query.startDate = formatDate(paginationReactive.range[0])
      query.endDate = formatDate(paginationReactive.range[1])
    }
    if (paginationReactive.startTime) {
      query.startTime = formatTime(paginationReactive.startTime)
    }
    if (paginationReactive.endTime) {
      query.endTime = formatTime(paginationReactive.endTime)
    }
    if (paginationReactive.minVolume) {
      query.minVolume = paginationReactive.minVolume
    }
    if (paginationReactive.minAmount) {
      query.minAmount = paginationReactive.minAmount
    }
    if (paginationReactive.minChangeRate) {
      query.minChangeRate = paginationReactive.minChangeRate
    }
    if (paginationReactive.maxChangeRate) {
      query.maxChangeRate = paginationReactive.maxChangeRate
    }
    if (paginationReactive.industry.trim()) {
      query.industry = paginationReactive.industry.trim()
    }
    if (paginationReactive.concept.trim()) {
      query.concept = paginationReactive.concept.trim()
    }
    if (paginationReactive.keyword.trim()) {
      const keyword = paginationReactive.keyword.trim()
      if (/^\d+$/.test(keyword)) {
        query.stockCode = keyword
      } else {
        query.stockName = keyword
      }
    }
    if (selectedTypes.value.length > 0) {
      query.changeTypes = selectedTypes.value.map(t => parseInt(t))
    }
    const result = await GetStockChangeHistory(query)
    if (result) {
      dataRef.value = result.list || []
      paginationReactive.itemCount = result.total || 0
      paginationReactive.pageCount = result.totalPages || 1
    }
  } catch (e) {
    console.error('获取历史数据失败:', e)
  } finally {
    loadingRef.value = false
  }
}

function formatDate(dateValue) {
  if (!dateValue) return ''
  const date = new Date(dateValue)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

function formatTime(timeValue) {
  if (!timeValue) return ''
  const date = new Date(timeValue)
  const hours = String(date.getHours()).padStart(2, '0')
  const minutes = String(date.getMinutes()).padStart(2, '0')
  const seconds = String(date.getSeconds()).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}`
}

async function fetchData() {
  if (viewMode.value === 'realtime') {
    await fetchRealtimeData()
  } else {
    await fetchHistoryData()
  }
}

async function saveCurrentData() {
  const types = selectedTypes.value.map(t => parseInt(t))
  const result = await SaveStockChangesToHistory(types)
  message.info(result)
}

function startAutoRefresh() {
  stopAutoRefresh()
  countdown.value = refreshSeconds.value
  refreshInterval.value = setInterval(() => {
    if (typeof document !== 'undefined' && document.hidden) return
    checkTradingTime()
    countdown.value--
    if (countdown.value <= 0) {
      if (viewMode.value === 'realtime' && !isTrading.value) {
        countdown.value = refreshSeconds.value
        return
      }
      fetchData()
      countdown.value = refreshSeconds.value
    }
  }, 1000)
}

function stopAutoRefresh() {
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
}

function toggleAutoRefresh() {
  if (autoRefresh.value) {
    startAutoRefresh()
  } else {
    stopAutoRefresh()
  }
}

function selectAllBullish() {
  const bullishValues = bullishTypes.map(t => t.value)
  const currentBearish = selectedTypes.value.filter(t => bearishTypes.some(b => b.value === t))
  selectedTypes.value = [...bullishValues, ...currentBearish]
  fetchData()
}

function selectAllBearish() {
  const bearishValues = bearishTypes.map(t => t.value)
  const currentBullish = selectedTypes.value.filter(t => bullishTypes.some(b => b.value === t))
  selectedTypes.value = [...currentBullish, ...bearishValues]
  fetchData()
}

function selectAllTypes() {
  selectedTypes.value = allTypeValues
  fetchData()
}

function clearAllTypes() {
  selectedTypes.value = []
  fetchData()
}

function handleViewModeChange() {
  paginationReactive.page = 1
  if (viewMode.value === 'realtime') {
    checkTradingTime()
    if (isTrading.value) {
      autoRefresh.value = true
      startAutoRefresh()
    } else {
      autoRefresh.value = false
    }
  } else {
    autoRefresh.value = false
    stopAutoRefresh()
  }
  fetchData()
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    paginationReactive.page = currentPage
    fetchHistoryData()
  }
}

function handleSearch() {
  paginationReactive.page = 1
  fetchHistoryData()
}

function handleSearchKeyup(e) {
  if (e.key === 'Enter') {
    handleSearch()
  }
}

async function fetchAllCurrentData() {
  loadingRef.value = true
  try {
    const result = await GetAllStockChangesWithPaging(500)
    if (result) {
      dataRef.value = result.data || []
      paginationReactive.itemCount = result.totalCount || 0
      message.success(`获取到 ${result.data?.length || 0} 条当日异动数据`)
    }
  } catch (e) {
    console.error('获取全部异动数据失败:', e)
    message.error('获取全部异动数据失败')
  } finally {
    loadingRef.value = false
  }
}

onBeforeMount(() => {
  GetConfig().then(result => {
    if (result && result.darkTheme) {
      darkTheme.value = true
      document.documentElement.classList.add('dark')
    }
  })
})

onMounted(() => {
  checkTradingTime()
  fetchData()
  if (viewMode.value === 'realtime' && isTrading.value && autoRefresh.value) {
    startAutoRefresh()
  }
})

onBeforeUnmount(() => {
  stopAutoRefresh()
})
</script>

<template>
  <n-card>
    <template #header>
      <n-space vertical>
        <n-space
          justify="space-between"
          align="center"
        >
          <n-space align="center">
            <n-text strong>
              股票异动监控
            </n-text>
            <n-tag
              v-if="viewMode === 'realtime'"
              :type="isTrading ? 'success' : 'warning'"
              size="small"
            >
              {{ marketStatus }}
            </n-tag>
            <n-tag
              :type="viewMode === 'realtime' ? 'error' : 'info'"
              size="small"
            >
              {{ viewMode === 'realtime' ? '实时数据' : '历史数据' }}
            </n-tag>
            <n-text
              depth="3"
              style="font-size: 12px"
            >
              共 {{ paginationReactive.itemCount }} 条记录
            </n-text>
          </n-space>
          <n-space align="center">
            <n-radio-group
              v-model:value="viewMode"
              @update:value="handleViewModeChange"
            >
              <n-radio-button value="realtime">
                实时数据
              </n-radio-button>
              <n-radio-button value="history">
                历史数据
              </n-radio-button>
            </n-radio-group>
            
            <template v-if="viewMode === 'realtime'">
              <n-text
                v-if="autoRefresh && isTrading"
                depth="3"
                style="font-size: 12px"
              >
                {{ countdown }}秒后刷新
              </n-text>
              <n-text
                v-else-if="!isTrading"
                depth="3"
                style="font-size: 12px"
              >
                非交易时间暂停刷新
              </n-text>
              <n-switch
                v-model:value="autoRefresh"
                :disabled="!isTrading"
                @update:value="toggleAutoRefresh"
              >
                <template #checked>
                  自动刷新
                </template>
                <template #unchecked>
                  手动刷新
                </template>
              </n-switch>
            </template>
            
            <n-button
              :loading="loadingRef"
              type="primary"
              size="small"
              @click="fetchData"
            >
              刷新
            </n-button>

            <n-button
              type="warning"
              secondary
              size="small"
              :loading="loadingRef"
              @click="triggerAiMarketAnomalySummary"
            >
              ⚡ AI 异动大盘全景透视
            </n-button>
            
            <n-button
              v-if="viewMode === 'realtime'"
              size="small"
              @click="saveCurrentData"
            >
              保存到历史
            </n-button>
          </n-space>
        </n-space>

        <n-alert
          v-if="viewMode === 'realtime' && !isTrading"
          type="info"
          size="small"
        >
          当前非A股交易时间（周一至周五 9:30-11:30, 13:00-15:00），自动刷新已暂停。您可以查看历史数据或手动刷新。
        </n-alert>

        <n-space
          v-if="viewMode === 'history'"
          align="center"
        >
          <n-input 
            v-model:value="paginationReactive.keyword" 
            placeholder="输入股票代码或名称" 
            clearable 
            style="width: 200px"
            @keyup="handleSearchKeyup"
          />
          <n-date-picker
            v-model:value="paginationReactive.range"
            type="daterange"
            clearable
          />
          <n-time-picker 
            v-model:value="paginationReactive.startTime" 
            placeholder="开始时间" 
            clearable 
            format="HH:mm:ss"
            style="width: 120px"
          />
          <n-time-picker 
            v-model:value="paginationReactive.endTime" 
            placeholder="结束时间" 
            clearable 
            format="HH:mm:ss"
            style="width: 120px"
          />
          <n-button
            type="primary"
            :loading="loadingRef"
            @click="handleSearch"
          >
            查询
          </n-button>
          <n-button
            :loading="loadingRef"
            @click="fetchAllCurrentData"
          >
            获取今日全部数据
          </n-button>
        </n-space>
        <n-space
          v-if="viewMode === 'history'"
          align="center"
          style="margin-top: 8px"
        >
          <n-select
            v-model:value="paginationReactive.minVolume"
            :options="volumeOptions"
            placeholder="成交量筛选"
            style="width: 120px"
            clearable
          />
          <n-select
            v-model:value="paginationReactive.minAmount"
            :options="amountOptions"
            placeholder="金额筛选"
            style="width: 120px"
            clearable
          />
          <n-select
            v-model:value="paginationReactive.minChangeRate"
            :options="changeRateOptions"
            placeholder="涨跌幅筛选"
            style="width: 120px"
            clearable
          />
          <n-input 
            v-model:value="paginationReactive.industry" 
            placeholder="行业关键词" 
            clearable 
            style="width: 120px"
          />
          <n-input 
            v-model:value="paginationReactive.concept" 
            placeholder="概念关键词" 
            clearable 
            style="width: 120px"
          />
        </n-space>
        
        <n-space
          align="center"
          style="margin-top: 8px"
        >
          <n-text depth="3">
            异动类型筛选：
          </n-text>
          <n-button
            size="tiny"
            type="primary"
            @click="selectAllTypes"
          >
            全选
          </n-button>
          <n-button
            size="tiny"
            @click="clearAllTypes"
          >
            清空
          </n-button>
          <n-text
            depth="3"
            style="font-size: 12px"
          >
            已选 {{ selectedTypes.length }}/{{ allTypeValues.length }} 种
          </n-text>
        </n-space>

        <n-space vertical>
          <n-space align="center">
            <n-text style="color: #dc2626; font-weight: 500;">
              利好异动
            </n-text>
            <n-button
              size="tiny"
              @click="selectAllBullish"
            >
              全选利好
            </n-button>
          </n-space>
          <n-checkbox-group
            v-model:value="selectedTypes"
            @update:value="fetchData"
          >
            <n-space>
              <n-checkbox
                v-for="item in bullishTypes"
                :key="item.value"
                :value="item.value"
                :label="item.label"
              >
                <template #default>
                  <n-text :style="{color: selectedTypes.includes(item.value) ? '#dc2626' : undefined}">
                    {{ item.label }}
                  </n-text>
                </template>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </n-space>

        <n-space vertical>
          <n-space align="center">
            <n-text style="color: #16a34a; font-weight: 500;">
              利空异动
            </n-text>
            <n-button
              size="tiny"
              @click="selectAllBearish"
            >
              全选利空
            </n-button>
          </n-space>
          <n-checkbox-group
            v-model:value="selectedTypes"
            @update:value="fetchData"
          >
            <n-space>
              <n-checkbox
                v-for="item in bearishTypes"
                :key="item.value"
                :value="item.value"
                :label="item.label"
              >
                <template #default>
                  <n-text :style="{color: selectedTypes.includes(item.value) ? '#16a34a' : undefined}">
                    {{ item.label }}
                  </n-text>
                </template>
              </n-checkbox>
            </n-space>
          </n-checkbox-group>
        </n-space>
      </n-space>
    </template>

    <n-data-table
      remote
      :columns="columnsRef"
      :data="dataRef"
      :loading="loadingRef"
      :pagination="viewMode === 'history' ? paginationReactive : false"
      :bordered="false"
      :max-height="500"
      :scroll-x="1460"
      striped
      size="small"
      @update:page="handlePageChange"
    />
  </n-card>

  <n-modal
    v-model:show="showKlineModal"
    preset="card"
    :title="(klineName || '') + ' (' + klineCode + ') — 多周期K线'"
    style="width: 95vw; max-width: 1200px;"
    :bordered="true"
  >
    <stock-lightweight-kline-chart
      v-if="showKlineModal"
      :key="'kline-' + klineCode"
      :dark-theme="darkTheme"
      :code="klineCode"
      :stock-name="klineName"
      :chart-height="500"
    />
  </n-modal>

  <stock-diagnosis-drawer
    v-model:show="showDiagnosisDrawer"
    :stock-code="diagnosisStockCode"
    :stock-name="diagnosisStockName"
  />
</template>

<style scoped>
</style>
