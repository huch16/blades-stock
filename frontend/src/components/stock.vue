<script setup lang="ts">
import { computed, defineAsyncComponent, h, nextTick, onBeforeMount, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import {
  AddGroup,
  AddStockGroup,
  Follow,
  GetConfig,
  GetFollowList,
  GetGroupList,
  GetPromptTemplates,
  GetStockList,
  GetVersionInfo,
  Greet,
  InitializeGroupSort,
  RemoveGroup,
  RemoveStockGroup,
  SaveAIResponseResult,
  UnFollow,
  UpdateGroup,
  BatchGreet,
  GetStockThesisHistory,
  GetAllStockTheses,
  // 修复：openCenteredWindow() 在非 windows 平台分支调用 OpenURL()，但此前从未导入，
  // 一旦走到 default 分支即抛 `ReferenceError: OpenURL is not defined`（与 NSpace 同类缺陷）。
  OpenURL,
} from '../../wailsjs/go/main/App'
import FollowedStockGrid from './FollowedStockGrid.vue'
import StockDiagnosisDrawer from './StockDiagnosisDrawer.vue'
import { useAiConfigStore } from '../stores/aiConfig'
import { cleanFinalContent } from '../utils/cleanText'
import { consultAgent } from '../utils/agentControlBus'
import {
  NAvatar,
  NButton,
  NCard,
  NEmpty,
  NFlex,
  NForm,
  NFormItem,
  NInputNumber,
  NTag,
  NText,
  NTimeline,
  NTimelineItem,
  useDialog,
  useMessage,
  useNotification,
} from 'naive-ui'
import {
  Environment,
  EventsEmit,
  EventsOff,
  EventsOn,
  WindowReload,
} from '../../wailsjs/runtime'
import { isWails } from '../api/bridge'
import {Add, CreateOutline} from '@vicons/ionicons5'
import { sanitizeMarkdownHtml } from '../utils/sanitize';
import { useStockTabs } from '../composables/useStockTabs'
import { useStockEcharts } from '../composables/useStockEcharts'
import { useStockAi } from '../composables/useStockAi'
import { useStockAlerts } from '../composables/useStockAlerts'

// 异步按需懒加载重型 Markdown、CodeMirror、PDF 及 Word 导出库，避免首页冷启动全量加载 ~1MB JS
const MdEditor = defineAsyncComponent(() => import('md-editor-v3').then(m => {
  import('md-editor-v3/lib/style.css');
  return m.MdEditor;
}))
const MdPreview = defineAsyncComponent(() => import('md-editor-v3').then(m => {
  import('md-editor-v3/lib/style.css');
  return m.MdPreview;
}))
const ExportPDF = defineAsyncComponent(() => import('@vavt/v3-extension').then(m => {
  import('@vavt/v3-extension/lib/asset/ExportPDF.css');
  return m.ExportPDF;
}))

// 轻量 K 线（含 lightweight-charts + 全量技术指标计算 + 筹码分布）体积约 134 KB，
// 此前由 unplugin-vue-components 自动注册而**静态打进首屏入口 chunk**。
// 它只在用户点开「轻量K线」弹层 (modalShow6) 时才需要，改为显式异步组件后，
// 与 market.vue / FundRanking.vue 的全站既有写法保持一致，
// 首屏入口 chunk 直接减约 134 KB (br ≈ 38 KB)。
const StockLightweightKlineChart = defineAsyncComponent(() => import('./StockLightweightKlineChart.vue'))

import {useRoute, useRouter} from 'vue-router'
import MoneyTrend from "./moneyTrend.vue";

const route = useRoute()
const router = useRouter()

// Pinia Store - AI 配置全局管理
const aiConfigStore = useAiConfigStore()

const dialog = useDialog()
const toolbars = [0];

const kLineChartRef = ref(null);
const kLineChartRef2 = ref(null);


const handleProgress = (_progress) => {
  //console.log(`Export progress: ${progress.ratio * 100}%`);
};
const enableEditor = ref(false)
const mdPreviewRef = ref(null)
const mdEditorRef = ref(null)
const aiResultScrollRef = ref(null)
const tipsRef = ref(null)
const message = useMessage()
const notify = useNotification()
const stocks = ref([])
const results = ref({})
const stockList = ref([])
const followList = ref([])
const groupList = ref([])
const options = ref([])
const modalShow = ref(false)
const modalShow2 = ref(false)
const modalShow3 = ref(false)
const modalShow4 = ref(false)
const modalShow5 = ref(false)
const modalShow6 = ref(false)
const lwKlineCode = ref('')
const lwKlineName = ref('')
const currentStockTradingPrice = ref({
  stockCode: '',
  costPrice: 0,
  entryPrice: 0,
  takeProfitPrice: 0,
  stopLossPrice: 0,
})
const klineAutoCloseTimer = ref(null)
const addBTN = ref(true)
const enableTools = ref(true)
const thinkingMode = ref(true)
const formModel = ref({
  name: "",
  code: "",
  costPrice: 0.000,
  volume: 0,
  alarm: 0,
  alarmPrice: 0,
  sort: 999,
  cron: "",
  entryPrice: 0,
  takeProfitPrice: 0,
  stopLossPrice: 0,
})

const promptTemplates = ref([])
const sysPromptOptions = ref([])
const userPromptOptions = ref([])
const data = reactive({
  modelName: "",
  chatId: "",
  question: "",
  sysPromptId: null,
  aiConfigId: null,
  name: "",
  code: "",
  fenshiURL: "",
  kURL: "",
  resultText: "Please enter your name below 👇",
  fullscreen: false,
  airesult: "",
  openAiEnable: false,
  loading: true,
  analysisStatus: "",
  analysisAborted: false,
  darkTheme: false,
  changePercent: 0
})
const feishiInterval = ref(null)
const aiAnalysisTimeout = ref(null)


const currentGroupId = ref(0)

// 买方认知档案与判定账本
const thesesMap = ref<Record<string, any>>({})
const activeThesis = ref<any>(null)
const thesisHistoryList = ref<any[]>([])
const showThesisHistoryModal = ref(false)

function getVerdictType(verdict?: string) {
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

function loadAllTheses() {
  GetAllStockTheses().then((res: any) => {
    thesesMap.value = res || {}
  }).catch(() => {})
}


async function viewThesisHistory(code: string) {
  if (!code) return
  try {
    const list = await GetStockThesisHistory(code, 30)
    thesisHistoryList.value = list || []
    showThesisHistoryModal.value = true
  } catch (err: any) {
    message.error('获取认知历史失败: ' + (err?.message || err))
  }
}


const theme = computed(() => {
  return data.darkTheme ? 'dark' : 'light'
})

const icon = ref('');

const sortedResults = computed(() => {
  // 原生 Object.keys 等价于 lodash keys，改用原生以移除 lodash（vendor-utils 减重）
  const sortedKeys = Object.keys(results.value).sort();
  const sortedObject = {};
  sortedKeys.forEach(key => {
    sortedObject[key] = results.value[key];
  });
  return sortedObject
});

const groupResults = computed(() => {
  if (currentGroupId.value === 0) {
    return sortedResults.value
  }
  // 用 Set 替换 Array.includes，避免在自选股数量多时退化为 O(n^2) 查找
  const codeSet = new Set(stocks.value)
  const group = {}
  for (const key in sortedResults.value) {
    const item = sortedResults.value[key]
    if (item && codeSet.has(item['股票代码'])) {
      group[key] = item
    }
  }
  return group
})

// 逻辑抽至 composables（R-22b）：拖拽标签 / ECharts / AI 分析 / 成本报警
const { initDraggableTabs, cleanupDraggableTabs } = useStockTabs({ groupList, message })
const { handleKLine, handleFeishi, clearFeishi, disposeKlineCharts } = useStockEcharts({ data, feishiInterval, kLineChartRef, kLineChartRef2 })
const { triggerAiWatchlistScreening, aiCheckStock, aiReCheckStock, saveAsImage, saveAsMarkdown, saveAsWord } = useStockAi({ activeThesis, aiAnalysisTimeout, aiConfigStore, currentGroupId, data, enableTools, groupList, groupResults, mdEditorRef, mdPreviewRef, message, modalShow4, sortedResults, thinkingMode, tipsRef })
const { checkPriceLineAlerts, updateCostPriceAndVolumeNew, saveTradingPriceToBackend, setStock } = useStockAlerts({ currentGroupId, currentStockTradingPrice, followList, formModel, fromEastMoneyCode, message, modalShow, monitor, stocks })
const showPopover = ref(false)
// 拖拽相关变量

// 拖拽处理函数







onBeforeMount(() => {
  GetGroupList().then(result => {
    groupList.value = result
    const sorts = result.map(item => item.sort);
    const uniqueSorts = new Set(sorts);
    if (sorts.length !== uniqueSorts.size) {
      fetchGroupList();
    } else {
      if (route.query.groupId) {
        message.success("切换分组:" + route.query.groupName)
        currentGroupId.value = Number(route.query.groupId)
      }
    }
  }).catch(err => { console.error("GetGroupList error:", err) })
  GetStockList("").then(result => {
    stockList.value = result
    options.value = result.map(item => {
      return {
        label: item.name + " - " + item.ts_code,
        value: item.ts_code
      }
    })
  }).catch(err => { console.error("GetStockList error:", err) })
  GetConfig().then(result => {
    if (result.openAiEnable) {
      data.openAiEnable = true
    }
    if (result.darkTheme) {
      data.darkTheme = true
    }
  }).catch(err => { console.error("GetConfig error:", err) })
  GetPromptTemplates("", "").then(res => {
    promptTemplates.value = res

    sysPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型系统Prompt')
    userPromptOptions.value = promptTemplates.value.filter(item => item.type === '模型用户Prompt')

  }).catch(err => { console.error("GetPromptTemplates error:", err) })

  // 从 Store 加载 AI 配置（Pinia 统一管理）
  aiConfigStore.loadConfigs()

  EventsOn("loadingDone", (_data) => {
    message.loading("刷新股票基础数据...")
    GetStockList("").then(result => {
      stockList.value = result
      options.value = result.map(item => {
        return {
          label: item.name + " - " + item.ts_code,
          value: item.ts_code
        }
      })
    })
  })

  EventsOn("refresh", (data) => {
    message.success(data)
  })

  EventsOn("showSearch", (data) => {
    addBTN.value = data === 1;
  })

  EventsOn("stock_price", (data) => {
    updateData(data)
  })

  EventsOn("refreshFollowList", (_data) => {

    WindowReload()
  })

  EventsOn("newChatStream", async (msg) => {
    if (data.analysisAborted) return
    if (msg === "DONE") {
      // 清除超时定时器
      if (aiAnalysisTimeout.value) {
        clearTimeout(aiAnalysisTimeout.value)
        aiAnalysisTimeout.value = null
      }
      data.loading = false
      if (data.airesult && !data.airesult.startsWith("❌")) {
        data.airesult = cleanFinalContent(data.airesult)
        SaveAIResponseResult(data.code, data.name, data.airesult, data.chatId, data.question, aiConfigStore.activeConfigId)
        data.analysisStatus = "分析完成"
        notify.success({
          title: 'AI分析完成',
          content: `[${data.name}] 分析已完成`,
          duration: 3000,
        })
        setTimeout(() => {
          data.analysisStatus = ""
        }, 3000)
      } else {
        data.analysisStatus = "分析失败"
        message.error(data.airesult || "AI分析失败")
      }
      message.destroyAll()
    } else {
      if (msg.chatId) {
        data.chatId = msg.chatId
      }
      if (msg.question) {
        data.question = msg.question
      }
      // [STATUS] → 更新状态栏；[STEP] → 忽略（工具调用日志）
      let hasContent = false
      if (msg.content && !msg.content.startsWith("[STATUS]") && !msg.content.startsWith("[STEP]") && msg.content !== "agent-DONE") {
        data.airesult += msg.content
        hasContent = true
      } else if (msg.content && msg.content.startsWith("[STATUS]")) {
        data.analysisStatus = msg.content.replace("[STATUS]", "").trim()
      }
      // reasoning_content 仅用于状态/进度展示，绝不混入答案正文，
      // 避免思考模型的完整草稿与 [STATUS]/[STEP] 状态行污染结果。
      if (msg.reasoning_content) {
        if (msg.reasoning_content.startsWith("[STATUS]")) {
          data.analysisStatus = msg.reasoning_content.replace("[STATUS]", "").trim()
        } else if (msg.reasoning_content.startsWith("[STEP]")) {
          const stepText = msg.reasoning_content.replace("[STEP]", "").trim()
          if (stepText) {
            data.analysisStatus = stepText.length > 50 ? stepText.slice(0, 48) + "..." : stepText
          }
        }
      }
      if (msg.extraContent) {
        data.airesult += msg.extraContent
        hasContent = true
      }
      if (hasContent && !data.airesult) {
        data.analysisStatus = "AI正在分析中..."
      }
      data.loading = !hasContent
      scrollToAiResultBottom()
    }
  })

  EventsOn("changeTab", async (msg) => {
    currentGroupId.value = Number(msg.ID)
    nextTick(() => {
      updateTab(currentGroupId.value);
    });
  })


  EventsOn("warnMsg", async (msg) => {
    notify.error({
      avatar: () =>
          h(NAvatar, {
            size: 'small',
            round: false,
            src: icon.value
          }),
      title: '警告',
      duration: 5000,
      content: () => {
        return h('div', {
          style: {
            'text-align': 'left',
            'font-size': '14px',
          }
        }, {default: () => msg})
      },
    })
  })
})
// 监听分组列表变化，重新初始化拖拽
const unwatch = watch(groupList, () => {
  nextTick(() => {
    initDraggableTabs();
  });
});

// 在组件卸载时清理监听器
onBeforeUnmount(() => {
  unwatch();
  EventsOff('stockThesisUpdated', loadAllTheses);
});
onMounted(() => {
  nextTick(() => {
    initDraggableTabs();
  });
  loadAllTheses();
  EventsOn('stockThesisUpdated', loadAllTheses);

  message.loading("Loading...")
  GetFollowList(currentGroupId.value).then(result => {

    followList.value = result
    const codes = []
    for (const followedStock of result) {
      let code = followedStock.StockCode
      if (code.startsWith("us")) {
        code = "gb_" + code.replace("us", "").toLowerCase()
      }
      if (!stocks.value.includes(code)) {
        stocks.value.push(code)
      }
      codes.push(code)
    }
    BatchGreet(codes).then(results => {
      for (const r of results) {
        updateData(r)
      }
    })
    message.destroyAll()
  }).catch(err => {
    message.destroyAll()
    console.warn('stock.vue GetFollowList/BatchGreet failed:', err)
  })

  GetVersionInfo().then((res) => {
    icon.value = res.icon
  }).catch(err => {
    console.warn('stock.vue GetVersionInfo failed:', err)
  })
})
// 清理拖拽事件监听器
// 清理拖拽事件监听器

// 初始化可拖拽选项卡

onBeforeUnmount(() => {
  // //console.log(`the component is now unmounted.`)
  //clearInterval(ticker.value)
  message.destroyAll()
  notify.destroyAll()
  clearInterval(feishiInterval.value)
  // 清理 AI 分析超时定时器
  if (aiAnalysisTimeout.value) {
    clearTimeout(aiAnalysisTimeout.value)
    aiAnalysisTimeout.value = null
  }
  // 清理多周期 K 线自动关闭定时器
  if (klineAutoCloseTimer.value) {
    clearTimeout(klineAutoCloseTimer.value)
    klineAutoCloseTimer.value = null
  }

  EventsOff("refresh")
  EventsOff("showSearch")
  EventsOff("stock_price")
  EventsOff("refreshFollowList")
  EventsOff("newChatStream")
  EventsOff("changeTab")
  EventsOff("updateVersion")
  EventsOff("updateNeedAdmin")
  EventsOff("warnMsg")
  EventsOff("loadingDone")

  cleanupDraggableTabs()

  // K 线/分时图表实例只在 ECharts 已加载时才存在；从未打开过弹窗时为空操作，不触发拉取。
  disposeKlineCharts()

})

// 添加一个获取分组列表的函数，用于处理初始化逻辑
function fetchGroupList() {
  InitializeGroupSort().then(initResult => {
    if (initResult) {
      GetGroupList().then(result => {
        groupList.value = result
        if (route.query.groupId) {
          message.success("切换分组:" + route.query.groupName)
          currentGroupId.value = Number(route.query.groupId)
        }
      })
    } else {
      message.error("初始化分组序号失败")
    }
  })
}

// 关注时的分组选择下拉选项（参考形态选股的分组选择写法）
const followGroupOptions = computed(() => {
  const opts = [{label: '默认（不分组）', key: 0}]
  groupList.value.forEach(g => opts.push({label: g.name, key: g.ID}))
  opts.push({type: 'divider', key: 'divider'})
  opts.push({label: '新建分组', key: 'new'})
  return opts
})

// 新建分组后待关注的股票（null 表示非关注流程打开的分组弹窗）
const pendingFollow = ref(null)

function groupNameById(id) {
  const g = groupList.value.find(item => item.ID === id)
  return g ? g.name : ''
}

function handleFollowSelect(key) {
  if (key === 'new') {
    if (!data?.code) {
      message.error("请输入有效股票代码")
      showPopover.value = true
      return
    }
    pendingFollow.value = {code: data.code, name: data.name}
    addTabModel.value = {name: '', sort: 1}
    addTabPane.value = true
    return
  }
  doFollowStock(Number(key))
}

function resolveStockCode() {
  if (data?.code) return data.code
  if (!data?.name) return ''
  const trimmed = data.name.trim()
  const match = trimmed.match(/\b\d{6}\b/)
  if (match) {
    const d = match[0][0]
    const prefix = (d === '6' || d === '5') ? 'sh' : 'sz'
    data.code = prefix + match[0]
    return data.code
  }
  if (options.value.length > 0) {
    const opt = options.value[0].value
    if (opt.indexOf(".") > 0) {
      data.code = opt.split(".")[1].toLowerCase() + opt.split(".")[0]
    } else {
      data.code = opt
    }
    return data.code
  }
  return ''
}

// 关注并加入分组（groupId=0 表示不分组），参考形态选股 doFollow
function doFollowStock(groupId) {
  if (!data?.code) {
    resolveStockCode()
  }
  if (!data?.code) {
    message.error("请输入有效股票代码")
    showPopover.value = true
    return
  }
  if (stocks.value.includes(data.code)) {
    message.error("已经关注了")
    return
  }
  Follow(data.code).then(result => {
    if (result === "关注成功") {
      // 后端 Follow 把 us 前缀归一化为 gb_，前端 stocks 数组需同步
      if (data.code.startsWith("us")) {
        data.code = "gb_" + data.code.replace("us", "").toLowerCase()
      }
      stocks.value.push(data.code)
      message.success(groupId > 0 ? `已关注，并加入分组「${groupNameById(groupId)}」` : '关注成功')
      // 加入分组（code 用 gb_ 格式，与后端 followed_stock.stock_code 一致）
      if (groupId > 0) {
        AddStockGroup(groupId, data.code).then(() => {
          GetGroupList().then(gList => { groupList.value = gList })
          if (currentGroupId.value === groupId) {
            updateTab(currentGroupId.value)
          }
        }).catch(err => message.error('加入分组失败: ' + (err?.message || err)))
      }
      GetFollowList(currentGroupId.value).then(result => { followList.value = result })
      monitor()
    } else {
      message.error(result)
    }
  }).catch(err => message.error('关注失败: ' + (err?.message || err)))
}


function removeMonitor(code, name, key) {
  //console.log("removeMonitor",name,code,key)
  stocks.value.splice(stocks.value.indexOf(code), 1)
  //console.log("removeMonitor-key",key)
  //console.log("removeMonitor-v",results.value[key])

  delete results.value[key]
  //console.log("removeMonitor-v",results.value[key])

  UnFollow(code).then(result => {
    message.success(result)
    monitor()
  })
}




function getStockList(value) {
  if (!value || !value.trim()) {
    options.value = []
    return
  }
  const q = value.trim().toLowerCase()
  const result = stockList.value.filter(item =>
    (item.name && item.name.toLowerCase().includes(q)) ||
    (item.ts_code && item.ts_code.toLowerCase().includes(q)) ||
    (item.symbol && item.symbol.toLowerCase().includes(q)) ||
    (item.cnspell && item.cnspell.toLowerCase().includes(q))
  ).slice(0, 30)

  if (result.length === 0) {
    GetStockList(value.trim()).then(remote => {
      if (Array.isArray(remote) && remote.length > 0) {
        options.value = remote.slice(0, 30).map(item => ({
          label: item.name + " - " + item.ts_code,
          value: item.ts_code
        }))
      }
    }).catch(() => {})
  }

  options.value = result.map(item => {
    return {
      label: item.name + " - " + item.ts_code,
      value: item.ts_code
    }
  })
  if (value && value.indexOf("-") <= 0) {
    data.code = value
  }

  if (data.code) {
    let findId = data.code
    if (findId.startsWith("us")) {
      findId = "gb_" + findId.replace("us", "").toLowerCase()
    }
    blinkBorder(findId)
  }
}

function blinkBorder(findId) {
  // 获取要滚动到的元素
  const element = document.getElementById(findId);
  //console.log("blinkBorder",findId,element)
  if (element) {
    // 滚动到该元素
    element.scrollIntoView({behavior: 'smooth'});
    const pelement = document.getElementById(findId + '_gi');
    if (pelement) {
      // 添加闪烁效果
      pelement.classList.add('blink-border');
      // 3秒后移除闪烁效果
      setTimeout(() => {
        pelement.classList.remove('blink-border');
      }, 1000 * 5);
    } else {
      console.error(`Element with ID ${findId}_gi not found`);
    }
  }
}

async function updateData(result) {
  ////console.log("stock_price",result['日期'],result['时间'],result['股票代码'],result['股票名称'],result['当前价格'],result['盘前盘后'])

  if (result["当前价格"] <= 0) {
    result["当前价格"] = result["卖一报价"]
  }

  if (result.changePercent > 0) {
    result.type = "error"
    result.color = "#E88080"
  } else if (result.changePercent < 0) {
    result.type = "success"
    result.color = "#63E2B7"
  } else {
    result.type = "default"
    result.color = "#FFFFFF"
  }

  if (result.profitAmount > 0) {
    result.profitType = "error"
  } else if (result.profitAmount < 0) {
    result.profitType = "success"
  }
  if (result["当前价格"]) {
    // if (result.alarmChangePercent > 0 && Math.abs(result.changePercent) >= result.alarmChangePercent) {
    //   SendMessage(result, 1)
    // }

    // if (result.alarmPrice > 0 && result["当前价格"] >= result.alarmPrice) {
    //   SendMessage(result, 2)
    // }

    // if (result.costPrice > 0 && result["当前价格"] >= result.costPrice) {
    //   SendMessage(result, 3)
    // }

    checkPriceLineAlerts(result)
  }

  // 行情系高频推送，避免整体重建 results 触发全卡片重渲染：
  // 只移除同一股票（sort 变化导致 key 变化时）的旧条目，再写入新条目。
  const _stockCode = result["股票代码"]
  result.key = GetSortKey(result.sort, _stockCode)
  let _prev = null
  for (const oldKey in results.value) {
    const old = results.value[oldKey]
    if (old && old["股票代码"] === _stockCode) {
      _prev = old
      if (oldKey !== result.key) delete results.value[oldKey]
      break
    }
  }
  // 缓存上一次推送的数值，供 n-number-animation 平滑过渡（替代 from=0 的高频重启动画）
  result.lastChangePercent = _prev ? _prev.changePercent : 0
  result.lastProfitAmountToday = _prev ? _prev.profitAmountToday : 0
  results.value[result.key] = result
  if (!stocks.value.includes(_stockCode)) {
    delete results.value[result.key]
  }
}


async function monitor() {
  if (stocks.value && stocks.value.length === 0) {
    showPopover.value = true
  }
  BatchGreet(stocks.value).then(results => {
    for (const r of results) {
      updateData(r)
    }
  })
}


function GetSortKey(sort, code) {
  // 原生 String.prototype.padStart（ES2017）等价于 lodash padStart，改用原生以移除 lodash
  return String(sort).padStart(8, '0') + "_" + code
}

function onSelect(item) {
  ////console.log("onSelect",item)

  if (item.indexOf("-") > 0) {
    item = item.split("-")[1].toLowerCase()
  }
  if (item.indexOf(".") > 0) {
    data.code = item.split(".")[1].toLowerCase() + item.split(".")[0]
  }

}

function openCenteredWindow(url, width, height) {
  const left = (window.screen.width - width) / 2;
  const top = (window.screen.height - height) / 2;
  Environment().then(env => {
    switch (env.platform) {
      case 'windows':
        window.open(
            url,
            'centeredWindow',
            `width=${width},height=${height},left=${left},top=${top},location=no,menubar=no,toolbar=no,display=standalone`
        )
        break
      default :
        OpenURL(url)
        break
    }
  })


  //
  // return window.open(
  //     url,
  //     'centeredWindow',
  //     `width=${width},height=${height},left=${left},top=${top}`
  // );
}

function search(code, name) {
  const url = "https://www.iwencai.com/unifiedwap/result?w=" + encodeURIComponent(name)
  // Web 模式：必须在点击手势内同步 window.open，否则 setTimeout + 异步 Environment
  // 会脱离手势调用栈而被浏览器拦截；且服务端 OpenURL 是在服务器进程里打开浏览器，
  // 用户侧没有任何反应，故 Web 下一律直接 window.open（桌面端保留居中弹窗）。
  if (!isWails()) {
    const win = window.open(url, '_blank', 'noopener')
    if (!win) message.warning('新窗口被浏览器拦截，请允许本站弹出窗口后重试')
    return
  }
  openCenteredWindow(url, 1000, 800)
}

function handleLongEntryPriceUpdate(newPrice) {
  currentStockTradingPrice.value.entryPrice = newPrice
  saveTradingPriceToBackend()
}

function handleLongStopLossPriceUpdate(newPrice) {
  currentStockTradingPrice.value.stopLossPrice = newPrice
  saveTradingPriceToBackend()
}

function handleLongTakeProfitPriceUpdate(newPrice) {
  currentStockTradingPrice.value.takeProfitPrice = newPrice
  saveTradingPriceToBackend()
}

function handleCostPriceUpdate(newPrice) {
  currentStockTradingPrice.value.costPrice = newPrice
  saveTradingPriceToBackend()
}




function showFenshi(code, name, changePercent) {
  data.code = code
  data.name = name
  data.changePercent = changePercent
  // 必须走 https：生产 CSP 为 `img-src 'self' data: blob: https:`，
  // 任何 http:// 图片都会被浏览器直接阻断，导致分时图整张空白。
  // 上游 image.sinajs.cn 同时支持 http/https（实测同码同体积），故无兼容风险。
  data.fenshiURL = 'https://image.sinajs.cn/newchart/min/n/' + data.code + '.gif' + "?t=" + Date.now()

  if (code.startsWith('hk')) {
    data.fenshiURL = 'https://image.sinajs.cn/newchart/hk_stock/min/' + data.code.replace("hk", "") + '.gif' + "?t=" + Date.now()
  }
  if (code.startsWith('gb_')) {
    data.fenshiURL = 'https://image.sinajs.cn/newchart/usstock/min/' + data.code.replace("gb_", "") + '.gif' + "?t=" + Date.now()
  }

  modalShow2.value = true
}



function showMoney(code, name) {
  data.code = code
  data.name = name
  modalShow5.value = true
}

/** 新浪/应用内代码转为东方财富接口常用格式（如 600519.SH） */
function toEastMoneyCode(code) {
  if (!code) return ''
  const c = String(code).trim()
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
    if (d === '6' || d === '5') return c + '.SH' // 6: 沪市A股/科创; 5: 沪市ETF/基金
    if (d === '0' || d === '3' || d === '1') return c + '.SZ' // 0: 深市主板; 3: 创业板; 1: 深市ETF/LOF
    if (d === '4' || d === '8' || d === '9') return c + '.BJ' // 北交所
    return c + '.SZ'
  }
  // 纯字母代码视为美股（如 AAPL → AAPL.US）
  if (/^[a-zA-Z]+$/.test(c)) return c.toUpperCase() + '.US'
  return ''
}

/** 东方财富格式转回应用内部代码格式（如 000001.SZ → sh000001） */
function fromEastMoneyCode(emCode) {
  if (!emCode) return ''
  const c = String(emCode).trim().toUpperCase()
  if (c.endsWith('.SH')) return 'sh' + c.slice(0, -3)
  if (c.endsWith('.SZ')) return 'sz' + c.slice(0, -3)
  if (c.endsWith('.BJ')) return 'bj' + c.slice(0, -3)
  if (c.endsWith('.HK')) return 'hk' + c.slice(0, -3).toLowerCase()
  if (c.endsWith('.US')) return 'us' + c.slice(0, -3).toLowerCase()
  return c.toLowerCase()
}

// AI 三维结构化量化诊断
const showDiagnosis = ref(false)
const diagnosisStockCode = ref('')
const diagnosisStockName = ref('')

function handleDiagnoseStock(code, name) {
  diagnosisStockCode.value = toEastMoneyCode(code) || code
  diagnosisStockName.value = name || ''
  showDiagnosis.value = true
}


async function showLightweightKline(code, name) {
  const em = toEastMoneyCode(code)
  if (!em) {
    message.warning('当前代码暂不支持K线图')
    return
  }
  lwKlineCode.value = em
  lwKlineName.value = name || ''

  // 刷新自选列表，确保获取最新的交易价格数据
  try {
    const list = await GetFollowList(currentGroupId.value)
    followList.value = list || []
  } catch (e) {
    console.error('[showLightweightKline] 刷新自选列表失败:', e)
  }

  // 从自选列表中获取交易价格
  // lwKlineCode 格式为 000001.SZ，followList 中的 StockCode 格式为 sh000001
  // 需要进行格式转换来匹配
  let followListCode = code
  if (code.startsWith('sh') || code.startsWith('sz') || code.startsWith('bj') || code.startsWith('hk')) {
    // 如果是 sh000001 格式，转换为东方财富格式
    const market = code.slice(0, 2).toUpperCase()
    const stockNum = code.slice(2)
    followListCode = stockNum + '.' + market
  }

  const stockInfo = followList.value.find(item => item.StockCode === code || item.StockCode === followListCode)
  if (stockInfo) {
    currentStockTradingPrice.value.stockCode = lwKlineCode.value  // 使用东方财富格式
    currentStockTradingPrice.value.costPrice = stockInfo.CostPrice || 0
    currentStockTradingPrice.value.entryPrice = stockInfo.EntryPrice || 0
    currentStockTradingPrice.value.takeProfitPrice = stockInfo.TakeProfitPrice || 0
    currentStockTradingPrice.value.stopLossPrice = stockInfo.StopLossPrice || 0
  } else {
    currentStockTradingPrice.value.stockCode = lwKlineCode.value
    currentStockTradingPrice.value.costPrice = 0
    currentStockTradingPrice.value.entryPrice = 0
    currentStockTradingPrice.value.takeProfitPrice = 0
    currentStockTradingPrice.value.stopLossPrice = 0
  }

  modalShow6.value = true
}

function showK(code, name) {
  data.code = code
  data.name = name
  // 同 showFenshi：CSP `img-src ... https:` 会阻断 http:// 图片，必须使用 https
  data.kURL = 'https://image.sinajs.cn/newchart/daily/n/' + data.code + '.gif' + "?t=" + Date.now()
  if (code.startsWith('hk')) {
    data.kURL = 'https://image.sinajs.cn/newchart/hk_stock/daily/' + data.code.replace("hk", "") + '.gif' + "?t=" + Date.now()
  }
  if (code.startsWith('gb_')) {
    data.kURL = 'https://image.sinajs.cn/newchart/usstock/daily/' + data.code.replace("gb_", "") + '.gif' + "?t=" + Date.now()
  }
  modalShow3.value = true
  //https://image.sinajs.cn/newchart/usstock/daily/dji.gif
  //https://image.sinajs.cn/newchart/hk_stock/daily/06030.gif?1740729404273
}





// 呼叫全屏 Agent 带着当前个股与买方认知档案进行深度量化推演
function callAgentForDeepStockAnalysis(stockName: string, stockCode: string) {
  modalShow4.value = false
  const activeT = activeThesis.value
  let contextText = `【标的信息】：${stockName} (${stockCode})\n`
  if (activeT && activeT.verdictText) {
    contextText += `前期买方认知档案判定：【${activeT.verdictText}】\n`
    if (activeT.coreThesis) contextText += `核心假说: ${activeT.coreThesis}\n`
    if (activeT.keyCatalysts) contextText += `跟踪催化: ${activeT.keyCatalysts}\n`
    if (activeT.riskFactors) contextText += `证伪条件/防守位: ${activeT.riskFactors}\n`
  }
  const question = `请针对【${stockName} (${stockCode})】结合最新量化事实、盘口资金与技术筹码形态，进行买方深度推演。重点评估：1. 当前投资逻辑是维持、强化还是证伪；2. 关键阻力与防守支撑位；3. 预期催化剂与赔率胜率。`
  consultAgent({
    question,
    stockCode,
    stockName,
    context: contextText,
    mode: 'plan_execute',
    autoSend: true,
  })
}



window.onerror = function (msg, source, lineno, colno, error) {
  if (typeof msg === 'string' && msg.includes('ResizeObserver')) return true
  EventsEmit("frontendError", {
    page: "stock.vue",
    message: msg,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null,
    data: data,
    results: results,
    followList: followList,
    stockList: stockList,
    stocks: stocks,
    formModel: formModel,
  });
  return true;
};


async function copyToClipboard() {
  try {
    await navigator.clipboard.writeText(data.airesult);
    message.success('分析结果已复制到剪切板');
  } catch (err) {
    message.error('复制失败: ' + err);
  }
}

function scrollToAiResultBottom() {
  nextTick(() => {
    requestAnimationFrame(() => {
      const el = aiResultScrollRef.value
      if (el) {
        el.scrollTop = el.scrollHeight
      }
    })
  })
}



// 导出文档

const addTabModel = ref({
  name: '',
  sort: 1,
})
const addTabPane = ref(false)

function addTab() {
  addTabPane.value = true
}

function saveTabPane() {
  AddGroup(addTabModel.value).then(result => {
    message.info(result)
    addTabPane.value = false
    GetGroupList().then(gList => {
      groupList.value = gList
      // 通知 App.vue 菜单栏立即刷新分组子项
      EventsEmit("groupListChanged")
      // 若来自关注流程的新建分组，创建成功后执行关注+加分组
      if (pendingFollow.value) {
        const created = gList.find(g => g.name === addTabModel.value.name)
        const pf = pendingFollow.value
        pendingFollow.value = null
        if (created) {
          data.code = pf.code
          data.name = pf.name
          doFollowStock(created.ID)
        }
      }
    })
  })
}

// 修改分组名称
const renameTabPane = ref(false)
const renameModel = reactive({id: 0, name: ''})

function openRenameGroup() {
  const g = groupList.value.find(item => item.ID === currentGroupId.value)
  if (!g) {
    message.warning('请先选择一个分组')
    return
  }
  renameModel.id = g.ID
  renameModel.name = g.name
  renameTabPane.value = true
}

function saveRenameGroup() {
  const newName = renameModel.name.trim()
  if (!newName) {
    message.warning('请输入分组名称')
    return
  }
  UpdateGroup(renameModel.id, newName).then(result => {
    message.info(result)
    renameTabPane.value = false
    GetGroupList().then(gList => {
      groupList.value = gList
      // 通知 App.vue 菜单栏立即刷新分组子项
      EventsEmit("groupListChanged")
    })
  }).catch(err => message.error('修改失败: ' + (err?.message || err)))
}

function AddStockGroupInfo(groupId, code, _name) {
  // 注意：不要把 gb_ 前缀转成 us。后端 Follow 已把美股存为 gb_aapl（us→gb_ + ToLower），
  // AddStockGroup 原样写入 group_stock_info.stock_code，GetFollowList(groupId) 再用该字段
  // IN 匹配 followed_stock.stock_code。若转成 usaapl 会导致美股分组关联失败（卡片上看不到）。
  AddStockGroup(groupId, code).then(result => {
    message.info(result)
    GetGroupList().then(gList => {
      groupList.value = gList
    })
    // 当前正处于目标分组时，刷新该分组，让新成员立即可见
    if (currentGroupId.value === groupId) {
      updateTab(currentGroupId.value)
    }
  }).catch(err => {
    message.error('设置分组失败: ' + (err?.message || err))
  })
}

function updateTab(name) {
  stocks.value = []
  const tabId= Number(name)
  currentGroupId.value = tabId;
  GetFollowList(tabId).then(result => {
    followList.value = result

    for (const followedStock of result) {
      if (followedStock.StockCode.startsWith("us")) {
        followedStock.StockCode = "gb_" + followedStock.StockCode.replace("us", "").toLowerCase()
      }
      stocks.value.push(followedStock.StockCode)
      Greet(followedStock.StockCode).then(result => {
        updateData(result)
      })
    }
    //monitor()
    message.destroyAll()
  })
}

function delTab(groupId) {
  const infos = groupList.value = groupList.value.filter(item => item.ID === Number(groupId))
  dialog.create({
    title: '删除分组',
    type: 'warning',
    content: '确定要删除[' + infos[0].name + ']分组吗？分组数据将不能恢复哟！',
    positiveText: '确定',
    negativeText: '取消',
    onPositiveClick: () => {
      RemoveGroup(Number(groupId)).then(result => {
        message.info(result)
        GetGroupList().then(result => {
          groupList.value = result
          // 通知 App.vue 菜单栏立即刷新分组子项
          EventsEmit("groupListChanged")
        })
      })
    }
  })
}

function delStockGroup(code, name, groupId) {
  RemoveStockGroup(code, name, groupId).then(result => {
    updateTab(groupId)
    message.info(result)
  })
}

function searchNotice(stockCode) {
  router.push({
    name: 'market',
    query: {
      name: '公司公告',
      stockCode: stockCode,
    },
  })
}

function searchStockReport(stockCode) {
  router.push({
    name: 'market',
    query: {
      name: '个股研报',
      stockCode: stockCode,
    },
  })
}

// 监听多周期 K 线模态框关闭，清除定时器
watch(modalShow6, (newVal) => {
  if (!newVal && klineAutoCloseTimer.value) {
    clearTimeout(klineAutoCloseTimer.value)
    klineAutoCloseTimer.value = null
  }
})
</script>

<template>
  <n-tabs type="card" style="--wails-draggable:no-drag" animated addable :data-currentGroupId="currentGroupId"
          :value="String(currentGroupId)" @add="addTab" @update:value="updateTab" placement="top" @close="(key)=>{delTab(key)}">

    <template #suffix>
      <n-button size="small" type="warning" secondary @click="triggerAiWatchlistScreening" style="margin-left:4px;">
        🛡️ <span class="hidden-xs">盘前持仓风控扫描</span><span class="visible-xs">风控扫描</span>
      </n-button>
      <n-button v-if="currentGroupId>0" size="small" tertiary type="primary" @click="openRenameGroup" style="margin-left:4px;">
        <n-icon :component="CreateOutline"/>&nbsp;<span class="hidden-xs">重命名</span>
      </n-button>
    </template>

    <n-tab-pane closable name="0" :tab="'全部'">
    <FollowedStockGrid :results="sortedResults" :open-ai-enable="data.openAiEnable" :group-list="groupList" :theses-map="thesesMap"
      @remove-monitor="removeMonitor"
      @ai-check-stock="aiCheckStock"
      @set-cost="setStock"
      @show-fenshi="showFenshi"
      @show-k="showK"
      @show-money="showMoney"
      @search-detail="search"
      @search-notice="searchNotice"
      @search-stock-report="searchStockReport"
      @add-stock-group="AddStockGroupInfo"
      @del-stock-group="delStockGroup"
      @show-lightweight-kline="showLightweightKline"
      @diagnose-stock="handleDiagnoseStock" />
    />
    </n-tab-pane>
    <n-tab-pane closable v-for="group in groupList" :key="'group-' + group.ID" :group-id="group.ID" :name="String(group.ID)" :tab="group.name">
    <FollowedStockGrid :results="groupResults" :show-spark-line="true" :group-id="group.ID" :open-ai-enable="data.openAiEnable" :group-list="groupList" :theses-map="thesesMap"
      @remove-monitor="removeMonitor"
      @ai-check-stock="aiCheckStock"
      @set-cost="setStock"
      @show-fenshi="showFenshi"
      @show-k="showK"
      @show-money="showMoney"
      @search-detail="search"
      @search-notice="searchNotice"
      @search-stock-report="searchStockReport"
      @add-stock-group="AddStockGroupInfo"
      @del-stock-group="delStockGroup"
      @show-lightweight-kline="showLightweightKline"
      @diagnose-stock="handleDiagnoseStock" />
    />
    </n-tab-pane>
  </n-tabs>

  <div class="watchlist-search-dock">
    <n-input-group style="box-shadow: 0 4px 18px rgba(0,0,0,0.22); border-radius: 8px; overflow: hidden; background: var(--n-color, #fff);">
      <n-auto-complete v-model:value="data.name" v-if="addBTN"
                       :input-props="{
                                autocomplete: 'disabled',
                              }"
                       :options="options"
                       placeholder="输入股票代码/名称添加自选"
                       clearable
                       @update:value="getStockList"
                       @input="getStockList(data.name)"
                       :on-select="onSelect"/>

      <n-popover trigger="manual" :show="showPopover">
        <template #trigger>
          <n-dropdown trigger="click" :options="followGroupOptions" @select="handleFollowSelect" placement="top">
            <n-button type="primary" v-if="addBTN">
              <n-icon :component="Add"/> &nbsp;关注
            </n-button>
          </n-dropdown>
        </template>
        <span>输入股票名称/代码关键词开始添加自选</span>
      </n-popover>
    </n-input-group>
  </div>
  <n-modal transform-origin="center" size="small" v-model:show="modalShow" :title="formModel.name" style="width: 800px;max-width: calc(100vw - 32px);"
           :preset="'card'">
    <n-form :model="formModel" :rules="{
              costPrice: { required: true, message: '请输入成本'},
              volume: { required: true, message: '请输入数量'},
              alarm:{required: true, message: '涨跌报警值'} ,
              alarmPrice: { required: true, message: '请输入报警价格'},
              sort: { required: true, message: '请输入排序值'},
            }" label-placement="left" label-width="100px">
      <n-grid :cols="2" :x-gap="12">
        <n-gi>
          <n-form-item label="股票成本" path="costPrice">
            <n-input-number v-model:value="formModel.costPrice" min="0" placeholder="请输入股票成本" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股票数量" path="volume">
            <n-input-number v-model:value="formModel.volume" min="0" step="100" placeholder="请输入股票数量" style="width: 100%">
              <template #suffix>
                股
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="涨跌提醒" path="alarm">
            <n-input-number v-model:value="formModel.alarm" min="0" placeholder="涨跌报警值(%)" style="width: 100%">
              <template #suffix>
                %
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股价提醒" path="alarmPrice">
            <n-input-number v-model:value="formModel.alarmPrice" min="0" placeholder="股价报警值" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="开仓价" path="entryPrice">
            <n-input-number v-model:value="formModel.entryPrice" min="0" step="0.01" placeholder="请输入开仓价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="股票排序" path="sort">
            <n-input-number v-model:value="formModel.sort" min="0" placeholder="排序值" style="width: 100%">
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="止盈价" path="takeProfitPrice">
            <n-input-number v-model:value="formModel.takeProfitPrice" min="0" step="0.01" placeholder="请输入止盈价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
        <n-gi>
          <n-form-item label="止损价" path="stopLossPrice">
            <n-input-number v-model:value="formModel.stopLossPrice" min="0" step="0.01" placeholder="请输入止损价" style="width: 100%">
              <template #suffix>
                {{ formModel.code.indexOf("hk") >= 0 ? "HK$" : "¥" }}
              </template>
            </n-input-number>
          </n-form-item>
        </n-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-button type="primary"
                @click="updateCostPriceAndVolumeNew(formModel.code,formModel.costPrice,formModel.volume,formModel.alarm,formModel)">
        保存
      </n-button>
    </template>
  </n-modal>

  <n-modal v-model:show="addTabPane" title="添加分组" style="width: 400px;text-align: left" :preset="'card'">
    <n-form
        :model="addTabModel"
        size="medium"
        label-placement="left"
    >
      <n-grid :cols="2">
        <n-form-item-gi label="分组名称:" path="name" :span="5">
          <n-input v-model:value="addTabModel.name" style="width: 100%" placeholder="请输入分组名称"/>
        </n-form-item-gi>
        <n-form-item-gi label="分组排序:" path="sort" :span="5">
          <n-input-number v-model:value="addTabModel.sort" style="width: 100%" min="0"
                          placeholder="请输入分组排序值"></n-input-number>
        </n-form-item-gi>
      </n-grid>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="saveTabPane">
          保存
        </n-button>
        <n-button type="warning" @click="addTabPane=false">
          取消
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="renameTabPane" title="修改分组名称" style="width: 400px;text-align: left" :preset="'card'">
    <n-form :model="renameModel" size="medium" label-placement="left">
      <n-form-item-gi label="分组名称:" path="name" :span="5">
        <n-input v-model:value="renameModel.name" style="width: 100%" placeholder="请输入新的分组名称"
                 @keyup.enter="saveRenameGroup"/>
      </n-form-item-gi>
    </n-form>
    <template #footer>
      <n-flex justify="end">
        <n-button type="primary" @click="saveRenameGroup">
          保存
        </n-button>
        <n-button type="warning" @click="renameTabPane=false">
          取消
        </n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="modalShow2" :title="data.name+' '+ data.changePercent+'%'" style="width: 1000px;max-width: calc(100vw - 32px);"
           :preset="'card'" @after-enter="handleFeishi" @after-leave="clearFeishi">
    <!-- 历史遗留：新浪 GIF 静态图方案已停用，分时图改由下方 ECharts（showFsChart）绘制。
         data.fenshiURL 保留为可回退方案，且已统一改为 https —— 生产 CSP 为
         `img-src 'self' data: blob: https:`，若将来取消注释启用此绑定，http:// 会被静默阻断。 -->
    <!--    <n-image :src="data.fenshiURL" />-->
    <div ref="kLineChartRef2" style="width: 100%; height: 500px;"></div>
  </n-modal>
  <n-modal v-model:show="modalShow3" :title="data.name" style="width: 1000px;max-width: calc(100vw - 32px);" :preset="'card'"
           @after-enter="handleKLine">
    <!-- 同上：日K 静态图已停用，改由 handleKLine -> ECharts 绘制；data.kURL 为 https 回退方案 -->
    <!--    <n-image :src="data.kURL" />-->
    <div ref="kLineChartRef" style="width: 100%; height: 500px;"></div>
  </n-modal>

  <n-modal transform-origin="center" v-model:show="modalShow4" preset="card" style="width: 800px;max-width: calc(100vw - 32px);"
           :title="'['+data.name+']AI分析'">
    <!-- 🎯 买方认知档案沉淀卡片 -->
    <n-card
      v-if="activeThesis && activeThesis.verdictText"
      size="small"
      :bordered="true"
      style="margin-bottom: 12px; background: rgba(24, 160, 88, 0.05); border-left: 4px solid #18a058; text-align: left;"
    >
      <n-flex justify="space-between" align="center" style="margin-bottom: 6px;">
        <n-flex align="center">
          <span style="font-weight: bold; font-size: 13px;">🎯 买方认知档案</span>
          <n-tag :type="getVerdictType(activeThesis.verdict)" size="small" round :bordered="false">
            {{ activeThesis.verdictText }}
          </n-tag>
          <n-text depth="3" style="font-size: 11px;">
            更新于 {{ activeThesis.updatedAt ? new Date(activeThesis.updatedAt).toLocaleDateString() : '' }}
          </n-text>
        </n-flex>
        <n-button size="tiny" type="primary" secondary @click="viewThesisHistory(data.code)">
          📈 认知演进历史
        </n-button>
      </n-flex>
      <div v-if="activeThesis.coreThesis" style="font-size: 12px; margin-bottom: 4px; line-height: 1.5;">
        <strong style="color: #18a058;">核心假说：</strong>{{ activeThesis.coreThesis }}
      </div>
      <n-flex size="small" style="font-size: 11px;">
        <span v-if="activeThesis.keyCatalysts" style="color: #f0a020;">
          <strong>跟踪催化：</strong>{{ activeThesis.keyCatalysts }}
        </span>
        <span v-if="activeThesis.riskFactors" style="color: #d03050; margin-left: 8px;">
          <strong>证伪条件：</strong>{{ activeThesis.riskFactors }}
        </span>
      </n-flex>
    </n-card>

    <n-spin size="small" :show="data.loading && !data.airesult">
      <MdEditor v-if="enableEditor" :toolbars="toolbars" ref="mdEditorRef" style="height: 440px;max-height: 60vh;text-align: left"
                :modelValue="data.airesult" :theme="theme">
        <template #defToolbars>
          <ExportPDF :file-name="data.name+'['+data.code+']AI分析报告'" style="text-align: left"
                     :modelValue="data.airesult" @onProgress="handleProgress"/>
        </template>
      </MdEditor>
      <div v-if="!enableEditor" ref="aiResultScrollRef" style="height: 440px;max-height: 60vh;text-align: left;overflow-y: auto;">
        <MdPreview ref="mdPreviewRef" :modelValue="data.airesult" :theme="theme" :sanitize="sanitizeMarkdownHtml"/>
      </div>
    </n-spin>
    <template #footer>
      <n-flex justify="space-between" ref="tipsRef">
        <n-text type="info" v-if="data.time">
          <n-tag v-if="data.modelName" type="warning" round :title="data.chatId" :bordered="false">
            {{ data.modelName }}
          </n-tag>
          {{ data.time }}
        </n-text>
        <n-text type="success" v-if="data.analysisStatus">{{ data.analysisStatus }}</n-text>
        <n-text type="error">*AI分析结果仅供参考，请以实际行情为准。投资需谨慎，风险自担。</n-text>
      </n-flex>
    </template>
    <template #action>
      <n-flex justify="left" style="margin-bottom: 10px">
        <n-switch v-model:value="enableTools" :round="false">
          <template #checked>
            工具调用
          </template>
          <template #unchecked>
            非工具调用
          </template>
        </n-switch>
        <n-switch v-model:value="thinkingMode" :round="false">
          <template #checked>
            思考模式
          </template>
          <template #unchecked>
            非思考模式
          </template>
        </n-switch>
        <n-gradient-text type="error" style="margin-left: 10px">
          *AI函数工具调用可以增强AI获取数据的能力,但会消耗更多tokens。
        </n-gradient-text>
      </n-flex>
      <n-flex justify="space-between" style="margin-bottom: 10px">
        <n-select style="width: 31%" v-model:value="aiConfigStore.activeConfigId" label-field="name" value-field="ID"
                  :options="aiConfigStore.configs" placeholder="请选择AI模型服务配置"/>
        <n-select style="width: 31%" v-model:value="data.sysPromptId" label-field="name" value-field="ID"
                  :options="sysPromptOptions" placeholder="请选择系统提示词"/>
        <n-select style="width: 31%" v-model:value="data.question" label-field="name" value-field="content"
                  :options="userPromptOptions" placeholder="请选择用户提示词"/>
      </n-flex>
      <n-flex justify="right">
        <n-input v-model:value="data.question" style="text-align: left" clearable
                 type="textarea"
                 :show-count="true"
                 placeholder="请输入您的问题:例如{{stockName}}[{{stockCode}}]分析和总结"
                 :autosize="{
              minRows: 2,
              maxRows: 5
            }"
        />
        <!--        <n-button size="tiny" type="error" @click="enableEditor=!enableEditor">编辑/预览</n-button>-->
        <n-button size="tiny" type="primary" secondary style="font-weight: 600;" @click="callAgentForDeepStockAnalysis(data.name, data.code)">
          🤖 呼叫 Agent 深度推演
        </n-button>
        <n-button size="tiny" type="warning" @click="aiReCheckStock(data.name,data.code)">开始AI分析</n-button>
        <n-button size="tiny" type="info" @click="saveAsImage(data.name,data.code)">保存为图片</n-button>
        <n-button size="tiny" type="success" @click="copyToClipboard">复制到剪切板</n-button>
        <n-button size="tiny" type="primary" @click="saveAsMarkdown">保存为Markdown文件</n-button>
        <n-button size="tiny" type="primary" @click="saveAsWord">保存为Word文件</n-button>
      </n-flex>
    </template>
  </n-modal>
  <n-modal v-model:show="modalShow5" :title="data.name+'资金趋势'" style="width: 1000px;max-width: calc(100vw - 32px);" :preset="'card'">
    <money-trend :code="data.code" :name="data.name" :days="360" :dark-theme="data.darkTheme"
                 :chart-height="500"></money-trend>
  </n-modal>
  <n-modal
    v-model:show="modalShow6"
    :title="(lwKlineName || '') + ' — 多周期K线'"
    preset="card"
    style="width: min(1100px, 96vw); max-width: 96vw; box-sizing: border-box"
    :content-style="{
      maxHeight: 'min(85vh, 820px)',
      overflowY: 'auto',
      overflowX: 'hidden',
      minWidth: 0,
      boxSizing: 'border-box',
    }"
  >
    <stock-lightweight-kline-chart
      v-if="modalShow6"
      :key="'lightweight-' + lwKlineCode"
      :code="lwKlineCode"
      :stock-name="lwKlineName"
      :dark-theme="data.darkTheme"
      :chart-height="500"
      :long-entry-price="currentStockTradingPrice.entryPrice"
      :long-stop-loss-price="currentStockTradingPrice.stopLossPrice"
      :long-take-profit-price="currentStockTradingPrice.takeProfitPrice"
      :cost-price="currentStockTradingPrice.costPrice"
      @update:longEntryPrice="handleLongEntryPriceUpdate"
      @update:longStopLossPrice="handleLongStopLossPriceUpdate"
      @update:longTakeProfitPrice="handleLongTakeProfitPriceUpdate"
      @update:costPrice="handleCostPriceUpdate"
    />
  </n-modal>

  <!-- AI 结构化三维量化深度诊断抽屉 -->
  <StockDiagnosisDrawer
    v-model:show="showDiagnosis"
    :stock-code="diagnosisStockCode"
    :stock-name="diagnosisStockName"
  />

  <!-- 买方认知历史演进时间线弹窗 -->
  <n-modal
    v-model:show="showThesisHistoryModal"
    preset="card"
    style="width: 650px; max-width: calc(100vw - 32px); text-align: left;"
    title="📈 买方认知演进时间线"
  >
    <div style="max-height: 60vh; overflow-y: auto; padding-right: 8px;">
      <n-empty v-if="!thesisHistoryList || thesisHistoryList.length === 0" description="暂无历史认知演化记录" />
      <n-timeline v-else>
        <n-timeline-item
          v-for="item in thesisHistoryList"
          :key="item.id"
          :type="getVerdictType(item.verdict)"
          :title="item.verdictText || item.verdict"
          :time="new Date(item.createdAt).toLocaleString()"
        >
          <div style="font-size: 12px; margin-top: 4px; line-height: 1.5;">
            <div v-if="item.coreThesis" style="margin-bottom: 2px;">
              <strong>核心逻辑：</strong>{{ item.coreThesis }}
            </div>
            <div v-if="item.keyCatalysts" style="color: #f0a020; margin-bottom: 2px;">
              <strong>验证催化：</strong>{{ item.keyCatalysts }}
            </div>
            <div v-if="item.riskFactors" style="color: #d03050; margin-bottom: 2px;">
              <strong>证伪条件：</strong>{{ item.riskFactors }}
            </div>
            <n-text depth="3" style="font-size: 11px;">
              模型: {{ item.modelName || '默认' }} · 触发: {{ item.triggerEvent || '系统' }}
            </n-text>
          </div>
        </n-timeline-item>
      </n-timeline>
    </div>
  </n-modal>
</template>

<style scoped>
.md-editor-preview h3 {
  text-align: center !important;
}

.md-editor-preview p {
  text-align: left !important;
}

/* 添加闪烁效果的CSS类 */
.blink-border {
  animation: blink-border 1s linear infinite;
  border: 4px solid transparent;
}

@keyframes blink-border {
  0% {
    border-color: red;
  }
  50% {
    border-color: transparent;
  }
  100% {
    border-color: red;
  }
}

/* 所有标签的通用样式 */
:deep(.n-tabs-nav .n-tabs-tab) {
  position: relative;
  cursor: pointer;
}

/* 可拖拽标签的样式 */
:deep(.n-tabs-nav .n-tabs-tab[draggable="true"]) {
  user-select: none;
  cursor: move;
}

.tab-drag-over {
  background-color: #e6f7ff !important;
  border: 2px dashed #1890ff !important;
  transform: scale(1.02);
  transition: all 0.2s ease;
  z-index: 10;
}

.tab-drag-over::after {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: -1;
}

.tab-dragging {
  opacity: 0.5;
}

/* 底部自选搜索输入框响应式停靠 */
.watchlist-search-dock {
  position: fixed;
  bottom: 64px;
  right: 20px;
  z-index: 100;
  width: 420px;
  max-width: calc(100vw - 40px);
  transition: all 0.25s ease;
}

.visible-xs {
  display: none !important;
}

@media screen and (max-width: 768px) {
  .watchlist-search-dock {
    left: 12px;
    right: 12px;
    width: auto;
    max-width: none;
    bottom: calc(64px + var(--sab, 0px));
  }
  .hidden-xs {
    display: none !important;
  }
  .visible-xs {
    display: inline !important;
  }
}
</style>
