<script setup>
import {computed, h, onBeforeMount, onBeforeUnmount, onMounted,onUnmounted, ref,reactive} from 'vue'
import {
  GetAiRecommendStocksList,
  GetConfig,
  DeleteAiRecommendStocks,
  UpdateAiRecommendStocksAlert,
  SyncRecommendStocksFromIntelligence
} from "../../wailsjs/go/main/App";
import {NAvatar, NButton, NEllipsis, NSwitch, NTag, NText, useMessage, useNotification, NSpace, NEmpty} from "naive-ui";
import StockLightweightKlineChart from "./StockLightweightKlineChart.vue";
import sparkLine from "./stockSparkLine.vue"
import {format} from "date-fns";
import { dispatchAgentAction } from "../utils/agentControlBus";
import { setCurrentStock } from "../utils/stockContextBus";

const notify = useNotification()

onBeforeMount(()=> {
  GetConfig().then(result => {
    if (result.darkTheme) {
      editorDataRef.darkTheme = true
    }
  })
})
onMounted(() => {
  query({
    page: 1,
    pageSize: paginationReactive.pageSize,
    order: "desc",
    keyword: paginationReactive.keyword,
    startDate: paginationReactive.range[0],
    endDate: paginationReactive.range[1]
  }).then((data) => {
    dataRef.value = data.data
    paginationReactive.page = 1
    paginationReactive.pageCount = data.pageCount
    paginationReactive.itemCount = data.total
    loadingRef.value = false
  })
})
const message = useMessage()
const syncingRef = ref(false)

async function handleSyncFromIntelligence() {
  syncingRef.value = true
  try {
    message.info('正在从最新智能研报、ETF动量池及历史分析中汇聚精选标的...')
    const addedCount = await SyncRecommendStocksFromIntelligence()
    if (addedCount > 0) {
      message.success(`成功汇聚 ${addedCount} 只智能推荐标的！`)
    } else {
      message.info('当前研报标的已同步，或今日已有对应记录')
    }
    await handleSearch()
  } catch (err) {
    message.error('汇聚推荐失败: ' + (err?.message || err))
  } finally {
    syncingRef.value = false
  }
}

function handleTriggerAgentRecommendation() {
  window.dispatchEvent(
    new CustomEvent('openFloatingAgentWithQuestion', {
      detail: '请结合当前市场情绪红绿灯与板块资金流向，为我推荐 3 只具有高盈亏比的优质潜力标的，并给出明确的买入区间与止损点，调用 BatchCreateAiRecommendStocks 保存到推荐库。'
    })
  )
}
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const editorDataRef = reactive({
  show: false,
  loading: false,
  darkTheme: false,
  chatId: "",
  modelName: "",
  CreatedAt: "",
  stockName: "",
  stockCode: "",
  question: "",
  content: "",
})
const dataRef = ref([])
const loadingRef = ref(true)

// StockClosePrice          string     `json:"StockClosePrice" md:"推荐时股票收盘价格"`
// StockPrePrice            string     `json:"stockPrePricePrice" md:"前一交易日股票价格"`
// RecommendReason          string     `json:"recommendReason" md:"推荐理由/驱动因素/逻辑"`
// RecommendBuyPrice        string     `json:"recommendBuyPrice" md:"ai建议买入价"`
// RecommendStopProfitPrice string     `json:"recommendStopProfitPrice" md:"ai建议止盈价"`
// RecommendStopLossPrice   string     `json:"recommendStopLossPrice" md:"ai建议止损价"`
// RiskRemarks              string     `json:"riskRemarks" md:"风险提示"`
// Remarks                  string     `json:"remarks" md:"备注"`
const columnsRef = ref([
  {
    title: '推荐模型',
    key: 'modelName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.modelName })
    }
  },
  {
    title: '评级',
    key: 'rating',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.rating || '-' })
    }
  },
  {
    title: '推荐时间',
    key: 'dataTime',
    render(row, index) {
      //2026-01-14T22:13:27.2693252+08:00 格式化为常用时间格式
      return row.CreatedAt.substring(0, 19).replace('T', ' ')
    }
  },
  {
    title: '板块概念',
    key: 'bkName'
  },
  {
    title: '股票名称',
    key: 'stockName',
    render(row, index) {
      return h(NText, { type: "info" }, { default: () => row.stockName })
    }
  },
  {
    title: '股票代码',
    key: 'stockCode'
  },
  {
    title: '最新分时',
    key: 'stockCode',
    render(row, index) {
      const cur = Number(row.stockCurrentPrice)
      const pre = Number(row.stockPrePrice)
      if (!row.stockCurrentPrice || isNaN(cur) || cur <= 0 || !row.stockPrePrice || isNaN(pre) || pre <= 0) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      return h(sparkLine, { idSuffix: row.ID, stockName: row.stockName, stockCode: row.stockCode, lastPrice: row.stockCurrentPrice, openPrice: row.stockPrePrice, tooltip: true })
    }
  },
  {
    title: '最新',
    key: 'stockCurrentPrice',
    minWidth: 120,
    render(row, index) {
      const cur = Number(row.stockCurrentPrice)
      const pre = Number(row.stockPrePrice)
      if (!row.stockCurrentPrice || isNaN(cur) || cur <= 0 || !row.stockPrePrice || isNaN(pre) || pre <= 0) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      let diff = ((cur - pre) / pre * 100).toFixed(2)
      if (cur < pre) {
        return [h(NText, { type: "success", bordered: false }, { default: () => `${row.stockCurrentPrice} | ${diff}%` })]
      } else {
        return [h(NText, { type: "error", bordered: false }, { default: () => `${row.stockCurrentPrice} | ${diff}%` })]
      }
    }
  },
  {
    title: '推荐时',
    key: 'stockPrice',
    render(row, index) {
      const p = Number(row.stockPrice)
      const cur = Number(row.stockCurrentPrice)
      if (!row.stockPrice || isNaN(p) || p <= 0) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      if (!row.stockCurrentPrice || isNaN(cur) || cur <= 0) {
        return h(NText, { type: "info" }, { default: () => row.stockPrice })
      }
      let diff = ((cur - p) / p * 100).toFixed(2)
      let flagStr = "持平"
      let flag = "info"
      if (cur > p) {
        flagStr = "浮盈 " + diff + "%"
        flag = "error"
      } else if (cur < p) {
        flagStr = "浮亏 " + diff + "%"
        flag = "success"
      }
      return [
        h(NText, { type: "info", style: "margin-right: 6px;" }, { default: () => row.stockPrice }),
        h(NTag, { type: flag, size: "tiny", bordered: false }, { default: () => flagStr })
      ]
    }
  },
  {
    title: '昨收',
    key: 'stockPrePrice',
    render(row, index) {
      const pre = Number(row.stockPrePrice)
      if (!row.stockPrePrice || isNaN(pre) || pre <= 0) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      return h(NText, { type: "info" }, { default: () => row.stockPrePrice })
    }
  },
  {
    title: '开仓价',
    key: 'recommendBuyPrice',
    render(row, index) {
      if (!row.recommendBuyPrice || row.recommendBuyPrice === '-' || row.recommendBuyPrice.includes('9.85 ~ 10.05')) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      const cur = Number(row.stockCurrentPrice)
      if (cur <= 0) {
        return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })
      }
      if (row.recommendBuyPrice.includes("-")) {
        let prices = row.recommendBuyPrice.split("-").map(p => Number(p.trim()))
        if (prices.length >= 2 && cur >= prices[0] && cur <= prices[1]) {
          return [
            h(NText, { type: "success", style: "margin-right: 6px;" }, { default: () => row.recommendBuyPrice }),
            h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })
          ]
        }
      }
      if (row.recommendBuyPriceMin && row.recommendBuyPriceMax && cur < row.recommendBuyPriceMax && cur > row.recommendBuyPriceMin) {
        return [
          h(NText, { type: "success", style: "margin-right: 6px;" }, { default: () => row.recommendBuyPrice }),
          h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "Buy" })
        ]
      }
      return h(NText, { type: "info" }, { default: () => row.recommendBuyPrice })
    }
  },
  {
    title: '止盈价',
    key: 'recommendStopProfitPrice',
    render(row, index) {
      if (!row.recommendStopProfitPrice || row.recommendStopProfitPrice === '-' || row.recommendStopProfitPrice.includes('10.60 ~ 11.20')) {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      const cur = Number(row.stockCurrentPrice)
      if (cur <= 0) {
        return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
      }
      if (row.recommendStopProfitPrice.includes("-")) {
        let prices = row.recommendStopProfitPrice.split("-").map(p => Number(p.trim()))
        if (prices.length >= 2 && cur >= prices[0] && cur <= prices[1]) {
          return [
            h(NText, { type: "success", style: "margin-right: 6px;" }, { default: () => row.recommendStopProfitPrice }),
            h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "止盈" })
          ]
        }
      }
      if (row.recommendStopProfitPriceMin && cur >= row.recommendStopProfitPriceMin) {
        return [
          h(NText, { type: "success", style: "margin-right: 6px;" }, { default: () => row.recommendStopProfitPrice }),
          h(NTag, { type: "error", size: "tiny", bordered: false }, { default: () => "止盈" })
        ]
      }
      return h(NText, { type: "info" }, { default: () => row.recommendStopProfitPrice })
    }
  },
  {
    title: '止损价',
    key: 'recommendStopLossPrice',
    render(row, index) {
      if (!row.recommendStopLossPrice || row.recommendStopLossPrice === '-' || row.recommendStopLossPrice === '9.55') {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      const cur = Number(row.stockCurrentPrice)
      const stop = Number(row.recommendStopLossPrice)
      if (cur > 0 && stop > 0 && cur <= stop) {
        return [
          h(NText, { type: "error", style: "margin-right: 6px;" }, { default: () => row.recommendStopLossPrice }),
          h(NTag, { type: "warning", size: "tiny", bordered: false }, { default: () => "止损" })
        ]
      }
      return h(NText, { type: "info" }, { default: () => row.recommendStopLossPrice })
    }
  },
  {
    title: '推荐理由',
    key: 'recommendReason',
    render(row, index) {
      if (!row.recommendReason || row.recommendReason === '-') {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      let text = row.recommendReason
      if (text.includes('【盘前自选股与持仓综合排雷诊断清单】')) {
        text = '盘前自选股排雷诊断精选'
      }
      return h(NEllipsis, { lineClamp: 2, tooltip: true }, { default: () => text })
    }
  },
  {
    title: '风险提示',
    key: 'riskRemarks',
    render(row, index) {
      if (!row.riskRemarks || row.riskRemarks === '-') {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      return h(NEllipsis, { lineClamp: 1, tooltip: true }, { default: () => row.riskRemarks })
    }
  },
  {
    title: '备注',
    key: 'remarks',
    render(row, index) {
      if (!row.remarks || row.remarks === '-') {
        return h(NText, { depth: 3 }, { default: () => '-' })
      }
      return h(NEllipsis, { lineClamp: 1, tooltip: true }, { default: () => row.remarks })
    }
  },
  {
    title: '监控预警',
    key: 'enableAlert',
    width: 80,
    render(row, index) {
      return h(NSwitch, {
        value: row.enableAlert,
        onUpdateValue: (newValue) => toggleAlert(row, newValue)
      })
    }
  },
  {
    title: '操作',
    width: 170,
    render(row, index) {
      return h(NSpace, { size: 'small' }, {
        default: () => [
          h(
            NTag,
            {
              strong: true,
              tertiary: true,
              type: 'info',
              style: 'cursor: pointer;',
              onClick: (e) => {
                e.stopPropagation()
                dispatchAgentAction({
                  type: 'OPEN_DIAGNOSIS',
                  payload: {
                    code: row.stockCode,
                    name: row.stockName
                  }
                })
              }
            },
            { default: () => '🎯 3D诊断' }
          ),
          h(
            NTag,
            {
              strong: true,
              tertiary: true,
              type: 'warning',
              style: 'cursor: pointer;',
              onClick: (e) => {
                e.stopPropagation()
                showDetail(row)
              }
            },
            { default: () => '查看' }
          ),
          h(
            NTag,
            {
              strong: true,
              tertiary: true,
              type: 'error',
              style: 'cursor: pointer;',
              onClick: (e) => {
                e.stopPropagation()
                deleteAiRecommendStocks(row.ID)
              }
            },
            { default: () => '删除' }
          )
        ]
      })
    }
  },
])
const paginationReactive = reactive({
  page: 1,
  pageCount: 1,
  pageSize: 12,
  itemCount: 0,
  keyword: "",
  enableAlert: null, // null 表示全部，true 表示已开启，false 表示未开启
  range: [
    new Date(new Date().getTime() - 3 * 24 * 60 * 60 * 1000), // 前3天
    new Date() // 当天
  ],
  prefix({ itemCount }) {
    return `${itemCount} 条记录`
  }
})

const enableAlertOptions = [
  { label: '全部', value: null },
  { label: '已开启预警', value: true },
  { label: '未开启预警', value: false }
]

const modalDataRef = reactive({
  visible: false,
  title: "",
  content: "",
  riskRemarks: "",
  stockCode: "",
  stockName: "",
  remarks: "",
  /** 传给 K 线组件的多单价位（与 StockLightweightKlineChart v-model 同步） */
  longEntryPrice: '',
  longStopLossPrice: '',
  longTakeProfitPrice: '',
})

const theme = computed(() => {
  return editorDataRef.darkTheme ? 'dark' : 'light'
})


function query({
                 page,
                 pageSize = 10,
                 order = 'desc',
                 keyword = "",
                 startDate = "",
                 endDate = "",
                 enableAlert = null
               }) {
  return new Promise((resolve) => {

    GetAiRecommendStocksList({
      "page": page,
      "pageSize": pageSize,
      "modelName":keyword,
      "stockName":keyword,
      "stockCode":keyword,
      "bkName":keyword,
      "startDate": startDate,
      "endDate": endDate,
      "enableAlert": enableAlert
    }).then((res) => {
      const pagedData =res.list
      const total = res.total
      const pageCount =res.totalPages
      resolve({
        pageCount,
        data: pagedData,
        total
      })
    })
  })
}

function handlePageChange(currentPage) {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: currentPage,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]), // Format date to string
      endDate: formatDate(paginationReactive.range[1]), // Format date to string
      enableAlert: paginationReactive.enableAlert
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = currentPage
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function handleSearch() {
  if (!loadingRef.value) {
    loadingRef.value = true
    query({
      page: paginationReactive?.page ?? 1,
      pageSize: paginationReactive.pageSize,
      order: "desc",
      keyword: paginationReactive.keyword,
      startDate: formatDate(paginationReactive.range[0]),
      endDate: formatDate(paginationReactive.range[1]),
      enableAlert: paginationReactive.enableAlert
    }).then((data) => {
      dataRef.value = data.data
      paginationReactive.page = data.page
      paginationReactive.pageCount = data.pageCount
      paginationReactive.itemCount = data.total
      loadingRef.value = false
    })
  }
}
function formatDate(dateString) {
  const date = new Date(dateString)
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  // const hours = String(date.getHours()).padStart(2, '0')
  // const minutes = String(date.getMinutes()).padStart(2, '0')
  // const seconds = String(date.getSeconds()).padStart(2, '0')
  //return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
  return `${year}-${month}-${day}`
}
function getStockCode(stockCode) {
  if(stockCode.indexOf( ".")>0){
    stockCode=stockCode.split(".")[1]+stockCode.split(".")[0]
  }
  //转化为小写
  stockCode=stockCode.toLowerCase()
  return stockCode

}

/** 推荐价可能为区间 "a-b"，取左侧作为图上开仓/止损/止盈线参考价 */
function recommendRangeToSinglePrice(p) {
  if (p == null || String(p).trim() === '') return ''
  const s = String(p).trim()
  const i = s.indexOf('-')
  if (i > 0) return s.slice(0, i).trim()
  return s
}

function showDetail(row) {
  setCurrentStock({ code: row.stockCode, name: row.stockName })
  modalDataRef.title = row.stockName
  modalDataRef.content = row.recommendReason
  modalDataRef.riskRemarks = row.riskRemarks
  modalDataRef.stockCode = getStockCode(row.stockCode)
  modalDataRef.stockName = row.stockName
  modalDataRef.visible = true
  modalDataRef.remarks = row.remarks
  modalDataRef.longEntryPrice = recommendRangeToSinglePrice(row.recommendBuyPrice)
  modalDataRef.longStopLossPrice = recommendRangeToSinglePrice(row.recommendStopLossPrice)
  modalDataRef.longTakeProfitPrice = recommendRangeToSinglePrice(row.recommendStopProfitPrice)
}
function rowProps(row) {
  return {
    style: 'cursor: pointer;',
    onClick: () => {
      showDetail(row)
    }
  }
}
function deleteAiRecommendStocks(id) {
  DeleteAiRecommendStocks(id).then((res) => {
    notify.info({content: res, duration: 2000})
    handleSearch()
  })
}

function toggleAlert(row, newEnableAlert) {
  UpdateAiRecommendStocksAlert(row.ID, newEnableAlert).then((res) => {
    notify.info({content: res, duration: 2000})
    // 更新本地数据
    row.enableAlert = newEnableAlert
  })
}

</script>

<template>
  <div style="display: flex; flex-direction: column; gap: 10px;">
    <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px;">
      <n-input-group style="max-width: 720px;">
        <n-date-picker
          v-model:value="paginationReactive.range"
          type="daterange"
          style="width: 38%"
        />
        <n-select
          v-model:value="paginationReactive.enableAlert"
          :options="enableAlertOptions"
          placeholder="预警状态"
          style="width: 22%"
          clearable
        />
        <n-input
          v-model:value="paginationReactive.keyword"
          clearable
          placeholder="输入股票/模型搜索"
        />
        <n-button
          type="primary"
          ghost
          @click="handleSearch"
        >
          搜索
        </n-button>
      </n-input-group>

      <n-space>
        <n-button
          type="primary"
          :loading="syncingRef"
          @click="handleSyncFromIntelligence"
        >
          ⚡ 汇聚智能研报/ETF推荐
        </n-button>
        <n-button
          type="warning"
          secondary
          @click="handleTriggerAgentRecommendation"
        >
          🤖 唤醒Agent态势推演
        </n-button>
      </n-space>
    </div>

    <n-data-table
      remote
      size="small"
      :columns="columnsRef"
      :data="dataRef"
      :loading="loadingRef"
      :pagination="paginationReactive"
      :row-key="(rowData)=>rowData.ID"
      flex-height
      style="height: calc(100vh - 210px);"
      @update:page="handlePageChange"
    >
      <template #empty>
        <div style="padding: 48px 0; text-align: center;">
          <n-empty description="当前暂无推荐股票记录">
            <template #extra>
              <div style="margin-top: 14px; display: flex; flex-direction: column; align-items: center; gap: 10px;">
                <n-text
                  depth="3"
                  style="font-size: 13px;"
                >
                  系统已沉淀全生命周期智能研报、ETF动量轮动池与全市场态势感知结果：
                </n-text>
                <n-space justify="center">
                  <n-button
                    type="primary"
                    size="medium"
                    :loading="syncingRef"
                    @click="handleSyncFromIntelligence"
                  >
                    ⚡ 一键从最新智能研报 & ETF动量池提炼推荐
                  </n-button>
                  <n-button
                    type="warning"
                    secondary
                    size="medium"
                    @click="handleTriggerAgentRecommendation"
                  >
                    🧭 调起 Agent 结合当前态势推演金股
                  </n-button>
                </n-space>
              </div>
            </template>
          </n-empty>
        </div>
      </template>
    </n-data-table>
  </div>

  <n-modal
    v-model:show="modalDataRef.visible"
    :title="modalDataRef.title"
    preset="card"
    style="max-width: 1400px;"
  >
    <n-gradient-text
      :size="16"
      type="warning"
    >
      {{ modalDataRef.remarks }}
    </n-gradient-text>
    <n-card size="small">
      <StockLightweightKlineChart
        v-model:long-entry-price="modalDataRef.longEntryPrice"
        v-model:long-stop-loss-price="modalDataRef.longStopLossPrice"
        v-model:long-take-profit-price="modalDataRef.longTakeProfitPrice"
        style="width: 100%;"
        :code="modalDataRef.stockCode"
        :chart-height="350"
        :stock-name="modalDataRef.stockName"
        :dark-theme="editorDataRef.darkTheme"
      />
    </n-card>
    <n-card size="small">
      <n-text type="info">
        {{ modalDataRef.content }}
      </n-text>
      <n-divider>
        <n-gradient-text type="error">
          风险提示
        </n-gradient-text>
      </n-divider>
      <n-text type="error">
        {{ modalDataRef.riskRemarks }}
      </n-text>
    </n-card>
  </n-modal>
</template>

<style scoped>

</style>