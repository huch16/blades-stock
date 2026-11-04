<script setup>
import {
  EventsEmit,
  EventsOff,
  EventsOn,
  WindowFullscreen,
  WindowUnfullscreen,
  WindowSetTitle
} from '../wailsjs/runtime'
import {h, computed, onBeforeMount, onBeforeUnmount, onMounted, ref, onErrorCaptured} from "vue";
import {RouterLink, useRouter} from 'vue-router'
import {createDiscreteApi,darkTheme,lightTheme , NIcon, NText,NButton,dateZhCN,zhCN} from 'naive-ui'
import {
  AlarmOutline,
  AnalyticsOutline,
  BarChartSharp, Bonfire, BonfireOutline, DiamondOutline, EaselSharp,
  ExpandOutline, Flag,
  Flame, FlameSharp, FlaskOutline, InformationOutline,
  NewspaperOutline,
  NewspaperSharp, Notifications,
  Pulse,
  ReorderTwoOutline,
  SettingsOutline, ServerOutline, Skull, SkullOutline, SkullSharp,
  SparklesOutline, FlashOutline, Star,
  StarOutline,
  StatsChartOutline,
  Wallet, WarningOutline, TimeOutline, SearchOutline,
} from '@vicons/ionicons5'
import {AnalyzeSentiment, GetConfig, GetGroupList, GetVersionInfo, IsTradingTime, IsHKTradingTime, IsUSTradingTime, SaveStockThesis} from "../wailsjs/go/main/App";
import WebAuthModal from "./components/WebAuthModal.vue";
import { isWails, getAuthToken, webEventBus } from "./api/bridge";
import StockDiagnosisDrawer from "./components/StockDiagnosisDrawer.vue";
import { initAgentControlBus, registerAgentActionHandler, setPendingConsult, consultAgent } from "./utils/agentControlBus";
import {Dragon, Fire, FirefoxBrowser, Gripfire, Robot} from "@vicons/fa";
import {ReportAnalytics, ReportMoney, ReportSearch, TrendingUp} from "@vicons/tabler";
import {LocalFireDepartmentRound} from "@vicons/material";
import {AppsList20Regular, BoxSearch20Regular, CommentNote20Filled} from "@vicons/fluent";
import {FireFilled, MoneyCollectOutlined, NotificationFilled, StockOutlined} from "@vicons/antd";




const router = useRouter()
const isAuthenticated = ref(isWails() || Boolean(getAuthToken()))
const loading = ref(false)
const loadingMsg = ref("加载完成...")
const enableNews = ref(false)
const contentStyle = ref("")
const enableFund = ref(false)
const enableAgent = ref(false)
const enableDarkTheme = ref(null)
const content = ref('未经授权,禁止商业目的!\n\n数据来源于网络,仅供参考;投资有风险,入市需谨慎')
const isFullscreen = ref(false)
const activeKey = ref('stock')
const containerRef = ref({})
const realtimeProfit = ref(0)
const telegraph = ref([])
const groupList = ref([])
const officialStatement= ref("")
const marketStatus = ref('')
let marketStatusTimer = null

// Web 浏览器原生桌面推送通知辅助
function sendBrowserNotification(title, body) {
  if (typeof window === 'undefined' || isWails() || !('Notification' in window)) return
  if (Notification.permission === 'granted') {
    try {
      new Notification(title, {
        body: body || '',
        icon: '/favicon.ico',
      })
    } catch (e) {}
  } else if (Notification.permission === 'default') {
    Notification.requestPermission().then(permission => {
      if (permission === 'granted') {
        try {
          new Notification(title, {
            body: body || '',
            icon: '/favicon.ico',
          })
        } catch (e) {}
      }
    }).catch(() => {})
  }
}

// Agent 全局控制总线状态
const { dialog: globalDialog, message: globalMessage } = createDiscreteApi(['dialog', 'message'])

// 全局组件错误边界 (Error Boundary): 捕获子组件渲染与生命周期异常，防止整个页面白屏 (B-009)
onErrorCaptured((err, instance, info) => {
  console.error('[ErrorBoundary] 捕获到子组件未处理渲染异常:', err, 'Info:', info)
  try {
    globalMessage.warning('检测到局部组件渲染波动，系统已自动隔离，核心投研功能正常运行', { duration: 4000 })
  } catch (e) {}
  // 返回 false 阻止异常向顶层扩散导致整个 DOM 树白屏崩溃
  return false
})

const showAgentDiagnosisDrawer = ref(false)
const agentDiagnosisStockCode = ref('')
const agentDiagnosisStockName = ref('')

initAgentControlBus()
registerAgentActionHandler((action) => {
  if (!action || !action.type) return
  console.log('[AgentControlBus] Received action:', action)
  switch (action.type) {
    case 'NAVIGATE_VIEW':
      if (action.payload?.path) {
        router.push(action.payload.path)
        const key = action.payload.path.replace('/', '') || 'stock'
        activeKey.value = key
        globalMessage.info(`🤖 Agent 已为您导航至页面：${action.payload.path}`)
      }
      break
    case 'OPEN_DIAGNOSIS':
      if (action.payload?.code) {
        agentDiagnosisStockCode.value = action.payload.code
        agentDiagnosisStockName.value = action.payload.name || ''
        showAgentDiagnosisDrawer.value = true
        globalMessage.info(`🤖 Agent 已为您调起 3D 量化诊断抽屉：${agentDiagnosisStockName.value || agentDiagnosisStockCode.value}`)
      }
      break
    case 'OPEN_MARKET_INTELLIGENCE':
      onOpenMarketIntelligence(action.payload?.tab || 'pre_market_briefing', action.payload?.target)
      globalMessage.info('🤖 Agent 已为您打开市场情报中心')
      break
    case 'SHOW_KLINE':
      if (action.payload?.code) {
        const targetCode = action.payload.code
        const targetName = action.payload.name || ''
        router.push({
          name: 'klineAnalysis',
          query: {
            code: targetCode,
            name: targetName,
          },
        })
        activeKey.value = 'klineAnalysis'
        globalMessage.info(`🤖 已为您平滑切换至【${targetName || targetCode}】K线分析视图`)
      }
      break
    case 'DISCIPLINE_ALERT':
      globalDialog.warning({
        title: action.payload?.title || '⚠️ 操盘纪律强风控预警',
        content: action.payload?.message || '当前大盘处于高风险期，请严格遵守既定止损纪律，拒绝冲动操作！',
        positiveText: '严格遵守风控纪律',
      })
      break
    case 'CONSULT_AGENT':
      // 跨组件一键呼叫统一全屏 AI 智能体推演工作台
      showAgentDiagnosisDrawer.value = false
      setPendingConsult(action.payload)
      router.push({ name: 'agent' })
      activeKey.value = 'agent'
      setTimeout(() => {
        EventsEmit('agent:consult', action.payload)
        if (typeof window !== 'undefined') {
          window.dispatchEvent(new CustomEvent('agent:consult', { detail: action.payload }))
        }
      }, 100)
      break
    case 'FILTER_WATCHLIST':
      router.push({
        name: 'stock',
        query: {
          filter: action.payload?.keyword || action.payload?.tag || '',
        },
      })
      activeKey.value = 'stock'
      globalMessage.info('🤖 已为您定位至自选股列表')
      break
    case 'SAVE_THESIS':
      if (action.payload?.stockCode) {
        const p = action.payload
        SaveStockThesis(
          p.stockCode,
          p.stockName || '',
          p.verdict || 'HOLD',
          p.verdictText || '观察跟踪',
          p.coreThesis || '',
          p.keyCatalysts || '',
          p.riskFactors || '',
          p.modelName || 'Agent',
        ).then(() => {
          globalMessage.success(`🤖 已将【${p.stockName || p.stockCode}】买方立论成功沉淀至认知账本！`)
        }).catch((err) => {
          console.error('[AgentControlBus] SaveStockThesis error:', err)
        })
      }
      break
  }
})

function onOpenMarketIntelligence(type, target) {
  const tab = type || 'pre_market_briefing'
  const query = { tab }
  if (target) {
    query.target = typeof target === 'string' ? target : (target?.name || '')
  }
  activeKey.value = 'intelligence'
  router.push({ name: 'intelligence', query })
}

EventsOn('show-market-intelligence', (type) => {
  onOpenMarketIntelligence(type)
})

EventsOn('show-market-intelligence-serenity', (data) => {
  onOpenMarketIntelligence('serenity_industry_analysis', data)
})

if (typeof window !== 'undefined') {
  window.addEventListener('show-market-intelligence', (e) => {
    onOpenMarketIntelligence(e.detail)
  })
  window.addEventListener('show-market-intelligence-serenity', (e) => {
    onOpenMarketIntelligence('serenity_industry_analysis', e.detail)
  })
}

// 全局兼容兜底：承接所有组件发起的 openFloatingAgentWithQuestion，统一平滑收敛至 /agent 智能体推演工作台
function handleLegacyFloatingAgent(payload) {
  let question = ''
  let agentMode = 'plan_execute'
  if (typeof payload === 'string') {
    question = payload
  } else if (payload && typeof payload === 'object') {
    question = payload.question || payload.prompt || ''
    if (payload.agentMode) agentMode = payload.agentMode
  }
  if (question) {
    consultAgent({
      question,
      mode: agentMode,
      autoSend: true
    })
  }
}

EventsOn('openFloatingAgentWithQuestion', handleLegacyFloatingAgent)

if (typeof window !== 'undefined') {
  window.addEventListener('openFloatingAgentWithQuestion', (e) => {
    handleLegacyFloatingAgent(e.detail)
  })
}

const investmentMottos = [
  "投资有风险，入市需谨慎",
  "别人贪婪我恐惧，别人恐惧我贪婪",
  "股市有风险，投资需谨慎",
  "不要把所有鸡蛋放在一个篮子里",
  "时间是优秀企业的朋友",
  "买股票就是买公司",
  "市场短期是投票机，长期是称重机",
  "保住本金是投资的第一要务",
  "在别人恐慌时贪婪，在别人贪婪时恐慌",
  "风险来自于你不知道自己在做什么",
  "价格是你付出的，价值是你得到的",
  "投资最重要的品质是耐心",
  "机会总是留给有准备的人",
  "知行合一，方能致远",
  "顺势而为，逆势而思",
  "投资是一场马拉松，不是百米冲刺",
  "独立思考是投资成功的关键",
  "市场永远在波动，但价值终将回归",
  "控制风险比追求收益更重要",
  "学习是最好的投资",
]
const currentMotto = ref(investmentMottos[Math.floor(Math.random() * investmentMottos.length)])

function refreshMotto() {
  currentMotto.value = investmentMottos[Math.floor(Math.random() * investmentMottos.length)]
}

function updateMarketStatus() {
  Promise.all([
    IsTradingTime().catch(() => false),
    IsHKTradingTime().catch(() => false),
    IsUSTradingTime().catch(() => false)
  ]).then(([cn, hk, us]) => {
    const parts = []
    parts.push(cn ? 'A股交易中' : 'A股休市')
    parts.push(hk ? '港股交易中' : '港股休市')
    parts.push(us ? '美股交易中' : '美股休市')
    marketStatus.value = parts.join(' | ')
    if (isWails()) {
      WindowSetTitle("Blades Stock " + marketStatus.value + " " + officialStatement.value + "  「" + currentMotto.value + "」  [数据来源于网络，仅供参考；投资有风险，入市需谨慎]")
    } else {
      WindowSetTitle("Blades Stock · " + marketStatus.value)
    }
  })
}
const menuOptions = ref([
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'stock',
                query: {
                  groupName: '全部',
                  groupId: 0,
                },
                params: {},
              },
              onClick: () => {
                activeKey.value = 'stock'
              },
            },
            {default: () => '股票自选',}
        ),
    key: 'stock',
    icon: renderIcon(StarOutline),
    children: [
      {
        label: () =>
            h(
                'a',
                {
                  href: '#',
                  type: 'info',
                  onClick: () => {
                    activeKey.value = 'stock'
                    //console.log("push",item)
                    router.push({
                      name: 'stock',
                      query: {
                        groupName: '全部',
                        groupId: 0,
                      },
                    })
                    EventsEmit("changeTab", {ID: 0, name: '全部'})
                  },
                  to: {
                    name: 'stock',
                    query: {
                      groupName: '全部',
                      groupId: 0,
                    },
                  }
                },
                {default: () => '全部',}
            ),
        key: 0,
      }
    ],
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              href: '#',
              to: {
                name: 'market',
                params: {}
              },
              onClick: () => {
                activeKey.value = 'market'
                EventsEmit("changeMarketTab", {ID: 0, name: '市场快讯'})
              },
            },
            {default: () => '市场行情'}
        ),
    key: 'market',
    icon: renderIcon(NewspaperOutline),
    children: [
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "市场快讯",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '市场快讯'})
                  },
                },
                {default: () => '市场快讯',}
            ),
        key: 'market1',
        icon: renderIcon(NewspaperSharp),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "全球股指",
                    },
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '全球股指'})
                  },
                },
                {default: () => '全球股指',}
            ),
        key: 'market2',
        icon: renderIcon(BarChartSharp),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "重大指数",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '重大指数'})
                  },
                },
                {default: () => '重大指数',}
            ),
        key: 'market3',
        icon: renderIcon(AnalyticsOutline),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "行业排名",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '行业排名'})
                  },
                },
                {default: () => '行业排名',}
            ),
        key: 'market4',
        icon: renderIcon(Flag),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "个股资金流向",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '个股资金流向'})
                  },
                },
                {default: () => '个股资金流向',}
            ),
        key: 'market5',
        icon: renderIcon(Pulse),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "板块资金流向",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '板块资金流向'})
                  },
                },
                {default: () => '板块资金流向',}
            ),
        key: 'market5_1',
        icon: renderIcon(ReportMoney),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "概念资金流向",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '概念资金流向'})
                  },
                },
                {default: () => '概念资金流向',}
            ),
        key: 'market5_2',
        icon: renderIcon(TrendingUp),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "龙虎榜",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '龙虎榜'})
                  },
                },
                {default: () => '龙虎榜',}
            ),
        key: 'market6',
        icon: renderIcon(Dragon),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "个股研报",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '个股研报'})
                  },
                },
                {default: () => '个股研报',}
            ),
        key: 'market7',
        icon: renderIcon(StockOutlined),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "公司公告",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '公司公告'})
                  },
                },
                {default: () => '公司公告',}
            ),
        key: 'market8',
        icon: renderIcon(NotificationFilled),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "行业研究",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '行业研究'})
                  },
                },
                {default: () => '行业研究',}
            ),
        key: 'market9',
        icon: renderIcon(ReportSearch),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "当前热门",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '当前热门'})
                  },
                },
                {default: () => '当前热门',}
            ),
        key: 'market10',
        icon: renderIcon(Gripfire),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  href: '#',
                  to: {
                    name: 'market',
                    query: {
                      name: "名站优选",
                    }
                  },
                  onClick: () => {
                    activeKey.value = 'market'
                    EventsEmit("changeMarketTab", {ID: 0, name: '名站优选'})
                  },
                },
                {default: () => '名站优选',}
            ),
        key: 'market11',
        icon: renderIcon(FirefoxBrowser),
      },
    ]
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'intelligence',
              },
              onClick: () => {
                activeKey.value = 'intelligence'
              },
            },
            {default: () => '宏观智能'}
        ),
    key: 'intelligence',
    icon: renderIcon(AnalyticsOutline),
    children: [
      {
        label: () =>
            h(
                RouterLink,
                {
                  to: {
                    name: 'intelligence',
                    query: { tab: 'pre_market_briefing' },
                  },
                  onClick: () => {
                    activeKey.value = 'intelligence'
                  },
                },
                {default: () => '🌅 盘前前瞻'}
            ),
        key: 'intel_pre',
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  to: {
                    name: 'intelligence',
                    query: { tab: 'etf_rotation_decision' },
                  },
                  onClick: () => {
                    activeKey.value = 'intelligence'
                  },
                },
                {default: () => '📈 ETF轮动'}
            ),
        key: 'intel_etf',
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  to: {
                    name: 'intelligence',
                    query: { tab: 'post_market_review' },
                  },
                  onClick: () => {
                    activeKey.value = 'intelligence'
                  },
                },
                {default: () => '🌆 盘后复盘'}
            ),
        key: 'intel_post',
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  to: {
                    name: 'intelligence',
                    query: { tab: 'serenity_industry_analysis' },
                  },
                  onClick: () => {
                    activeKey.value = 'intelligence'
                  },
                },
                {default: () => '🔍 产业拆解'}
            ),
        key: 'intel_serenity',
      },
    ]
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'klineAnalysis',
              },
              onClick: () => {
                activeKey.value = 'klineAnalysis'
              },
            },
            {default: () => 'K线分析'}
        ),
    key: 'klineAnalysis',
    icon: renderIcon(StatsChartOutline),
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'fund',
                query: {
                  name: '基金自选',
                },
              },
              onClick: () => {
                activeKey.value = 'fund'
              },
            },
            {default: () => '基金自选',}
        ),
    show: enableFund.value,
    key: 'fund',
    icon: renderIcon(SparklesOutline),
    children: [
      {
        label: () =>
            h(
                RouterLink,
                {
                  to: {name: 'fund', query: {name: '基金自选'}},
                  onClick: () => {
                    activeKey.value = 'fund'
                    EventsEmit("changeFundTab", {name: '基金自选'})
                  },
                },
                {default: () => '基金自选'}
            ),
        key: 'fundFollow',
        icon: renderIcon(StarOutline),
      },
      {
        label: () =>
            h(
                RouterLink,
                {
                  to: {name: 'fund', query: {name: '基金排行'}},
                  onClick: () => {
                    activeKey.value = 'fund'
                    EventsEmit("changeFundTab", {name: '基金排行'})
                  },
                },
                {default: () => '基金排行'}
            ),
        key: 'fundRanking',
        icon: renderIcon(TrendingUp),
      },
    ]
  },
  {
    label: () =>
        h(
            RouterLink,
            {
              to: {
                name: 'agent',
                query: {
                  name:"AI Agent 助手",
                },
              },
              onClick: () => {
                activeKey.value = 'agent'
              },
            },
            {default: () => 'AI Agent 助手'}
        ),
    key: 'agent',
    show: true,
    icon: renderIcon(Robot),
  },
    {
      label: () =>
          h(
              RouterLink,
              {
                to: {
                  name: 'research',
                  query: {
                    name:"研究中心",
                  },
                },
                onClick: () => {
                  activeKey.value = 'research'
                  setTimeout(() => {
                    EventsEmit("changeResearchTab", {ID: 0, name: 'AI分析报告'})
                  }, 100)
                },
              },
              {default: () => '研究中心'}
          ),
      key: 'research',
      icon: renderIcon(FlaskOutline),
      children:[
          {
            label: () =>
                h(
                    RouterLink,
                    {
                      to: {
                        name: 'research',
                        query: {
                          name:"AI研报与认知账本",
                        },
                      },
                      onClick: () => {
                        activeKey.value = 'research'
                        setTimeout(() => {
                          EventsEmit("changeResearchTab", {ID: 0, name: 'AI研报与认知账本'})
                        }, 100)
                      },
                    },
                    {default: () => 'AI研报与认知账本'}
                ),
            key: 'research1',
            icon: renderIcon(ReportAnalytics),
          },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"盘面大单异动",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 2, name: '盘面大单异动'})
                      }, 100)
                    },
                  },
                  {default: () => '盘面大单异动'}
              ),
          key: 'stockChanges',
          icon: renderIcon(TrendingUp),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"涨停连板梯队",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 9, name: '涨停连板梯队'})
                      }, 100)
                    },
                  },
                  {default: () => '涨停连板梯队'}
              ),
          key: 'uplimitLadder',
          icon: renderIcon(LocalFireDepartmentRound),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"量化选股策略库",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 0, name: '量化选股策略库'})
                      }, 100)
                    },
                  },
                  {default: () => '量化选股策略库'}
              ),
          key: 'research_select_stock',
          icon: renderIcon(BoxSearch20Regular),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"实盘交易台账",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 6, name: '实盘交易台账'})
                      }, 100)
                    },
                  },
                  {default: () => '实盘交易台账'}
              ),
          key: 'research6',
          icon: renderIcon(MoneyCollectOutlined),
        },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'research',
                      query: {
                        name:"定时自动化任务",
                      },
                    },
                    onClick: () => {
                      activeKey.value = 'research'
                      setTimeout(() => {
                        EventsEmit("changeResearchTab", {ID: 5, name: '定时自动化任务'})
                      }, 100)
                    },
                  },
                  {default: () => '定时自动化任务'}
              ),
          key: 'research5',
          icon: renderIcon(TimeOutline),
        },
      ],
    },
        {
          label: () =>
              h(
                  RouterLink,
                  {
                    to: {
                      name: 'settings',
                      onClick: () => {
                        activeKey.value = 'settings'
                      },
                    }
                  },
                  {default: () => '设置'}
              ),
          key: 'settings',
          icon: renderIcon(SettingsOutline),
        },
  {
    show:false,
    label: () => h("a", {
      href: '#',
      onClick: toggleFullscreen,
      title: '全屏 Ctrl+F 退出全屏 Esc',
    }, {default: () => isFullscreen.value ? '取消全屏' : '全屏'}),
    key: 'full',
    icon: renderIcon(ExpandOutline),
  },
  // {
  //   label: ()=> h("a", {
  //     href: 'javascript:void(0)',
  //     style: 'cursor: move;',
  //     onClick: toggleStartMoveWindow,
  //   }, { default: () => '移动' }),
  //   key: 'move',
  //   icon: renderIcon(MoveOutline),
  // },
])

// 重建"股票自选"菜单的分组子项（保留"全部"，用最新分组列表替换其余子项）
function refreshStockGroupMenu() {
  GetGroupList().then(result => {
    groupList.value = result
    menuOptions.value.forEach((item) => {
      if (item.key === 'stock') {
        const allItem = item.children.find(c => c.key === 0)
        item.children = allItem ? [allItem] : []
        item.children.push(...groupList.value.map(g => {
          return {
            label: () =>
                h(
                    'a',
                    {
                      href: '#',
                      type: 'info',
                      onClick: () => {
                        router.push({
                          name: 'stock',
                          query: {
                            groupName: g.name,
                            groupId: g.ID,
                          },
                        })
                        setTimeout(() => {
                          EventsEmit("changeTab", g)
                        }, 100)
                      },
                      to: {
                        name: 'stock',
                        query: {
                          groupName: g.name,
                          groupId: g.ID,
                        },
                      }
                    },
                    {default: () => g.name,}
                ),
            key: g.ID,
          }
        }))
      }
    })
  }).catch(err => {
    console.error("refreshStockGroupMenu error:", err)
  })
}

function renderIcon(icon) {
  return () => h(NIcon, null, {default: () => h(icon)})
}

function toggleFullscreen(e) {
  activeKey.value = 'full'
  //console.log(e)
  if (isFullscreen.value) {
    WindowUnfullscreen()
    //e.target.innerHTML = '全屏'
  } else {
    WindowFullscreen()
    // e.target.innerHTML = '取消全屏'
  }
  isFullscreen.value = !isFullscreen.value
}

// const drag = ref(false)
// const lastPos= ref({x:0,y:0})
// function toggleStartMoveWindow(e) {
//   drag.value=!drag.value
//   lastPos.value={x:e.clientX,y:e.clientY}
// }
// function dragstart(e) {
//   if (drag.value) {
//     let x=e.clientX-lastPos.value.x
//     let y=e.clientY-lastPos.value.y
//     WindowGetPosition().then((pos) => {
//       WindowSetPosition(pos.x+x,pos.y+y)
//     })
//   }
// }
// window.addEventListener('mousemove', dragstart)

EventsOn("realtime_profit", (data) => {
  realtimeProfit.value = data
})
EventsOn("telegraph", (data) => {
  telegraph.value = data
})

EventsOn("loadingMsg", (data) => {
  if(data==="done"){
    loadingMsg.value = "加载完成..."
    EventsEmit("loadingDone", "app")
    loading.value  = false
  }else{
    loading.value  = true
    loadingMsg.value = data
  }
})

setTimeout(() => {
  if (loading.value) {
    loading.value = false
    loadingMsg.value = "加载完成..."
    EventsEmit("loadingDone", "app")
  }
}, 50)

onBeforeUnmount(() => {
  if (marketStatusTimer) {
    clearInterval(marketStatusTimer)
    marketStatusTimer = null
  }
  window.removeEventListener('resize', updateResponsiveLayout)
  EventsOff("realtime_profit")
  EventsOff("loadingMsg")
  EventsOff("telegraph")
  EventsOff("newsPush")
  EventsOff("groupListChanged")
  EventsOff("stockPriceAlertTriggered")
  EventsOff("marketIntelligenceReport")
  EventsOff("show-market-intelligence")
  EventsOff("show-market-intelligence-serenity")
  EventsOff("openFloatingAgentWithQuestion")
  EventsOff("updateSettings")
})

window.onerror = function (msg, source, lineno, colno, error) {
  if (typeof msg === 'string' && msg.includes('ResizeObserver')) return true
  EventsEmit("frontendError", {
    page: "App.vue",
    message: msg,
    source: source,
    lineno: lineno,
    colno: colno,
    error: error ? error.stack : null,
  });
  return true;
};

onBeforeMount(() => {
  GetVersionInfo().then(result => {
    if(result.officialStatement){
      content.value = result.officialStatement+"\n\n"+content.value
    }
    officialStatement.value = result.officialStatement || ""
    updateMarketStatus()
  }).catch(err => {
    console.error("GetVersionInfo error:", err)
  })

  refreshStockGroupMenu()
  // 监听分组变化（新增/改名/删除），实时刷新菜单栏
  EventsOn("groupListChanged", () => {
    refreshStockGroupMenu()
  })

  GetConfig().then((res) => {
    enableFund.value = res.enableFund
    enableAgent.value = true

    menuOptions.value.filter((item) => {
      if (item.key === 'fund') {
        item.show = res.enableFund
      }
      if (item.key === 'agent') {
        item.show = true
      }
    })

    applyThemeMode(!!res.darkTheme)
  }).catch(err => {
    console.error("GetConfig error:", err)
  })

  EventsOn("updateSettings", (cfg) => {
    if (cfg && typeof cfg.darkTheme !== 'undefined') {
      applyThemeMode(!!cfg.darkTheme)
    }
  })
})

function applyThemeMode(isDark) {
  if (isDark) {
    enableDarkTheme.value = darkTheme
    document.documentElement.setAttribute('theme-mode', 'dark')
    document.documentElement.classList.add('dark')
    document.body.classList.add('is-dark')
  } else {
    enableDarkTheme.value = null
    document.documentElement.setAttribute('theme-mode', 'light')
    document.documentElement.classList.remove('dark')
    document.body.classList.remove('is-dark')
  }
}

let appInitialized = false

// 移动端响应式感知与布局控制
const isMobile = ref(typeof window !== 'undefined' ? window.innerWidth <= 768 : false)

function updateResponsiveLayout() {
  isMobile.value = typeof window !== 'undefined' ? window.innerWidth <= 768 : false
  if (!isWails()) {
    if (isMobile.value) {
      contentStyle.value = "height: calc(100dvh - 58px - var(--sab, 0px)); max-height: calc(100dvh - 58px - var(--sab, 0px)); overflow-y: auto; -webkit-overflow-scrolling: touch; padding-bottom: calc(64px + var(--sab, 0px));"
    } else {
      contentStyle.value = "height: calc(100dvh - 64px); max-height: calc(100dvh - 64px); overflow-y: auto; padding-bottom: 72px;"
    }
  } else {
    contentStyle.value = "max-height: calc(92vh); overflow: hidden; padding-bottom: 72px;"
  }
}

const isAgentRoute = computed(() => {
  const routeName = router.currentRoute.value?.name
  const path = router.currentRoute.value?.path
  return routeName === 'agent' || path === '/agent'
})

const agentHostStyle = computed(() => {
  if (isWails()) {
    return "height: calc(100vh - 40px); max-height: calc(100vh - 40px); overflow: hidden; padding: 0; margin: 0;"
  }
  if (isMobile.value) {
    return "height: calc(100dvh - 58px - 56px - var(--sab, 0px)); max-height: calc(100dvh - 58px - 56px - var(--sab, 0px)); overflow: hidden; padding: 0; margin: 0;"
  }
  return "height: calc(100dvh - 64px); max-height: calc(100dvh - 64px); overflow: hidden; padding: 0; margin: 0;"
})

const currentMobileTab = computed(() => {
  const routeName = router.currentRoute.value.name
  if (!routeName || routeName === 'stock') return 'stock'
  if (routeName === 'market') return 'market'
  if (routeName === 'intelligence') return 'intelligence'
  if (routeName === 'agent') return 'agent'
  if (routeName === 'research') return 'research'
  if (routeName === 'settings') return 'settings'
  return 'stock'
})

function switchMobileTab(key) {
  activeKey.value = key
  if (key === 'stock') {
    router.push({ name: 'stock' })
  } else if (key === 'market') {
    router.push({ name: 'market' })
  } else if (key === 'intelligence') {
    router.push({ name: 'intelligence' })
  } else if (key === 'agent') {
    router.push({ name: 'agent' })
  } else if (key === 'research') {
    router.push({ name: 'research' })
  } else if (key === 'settings') {
    router.push({ name: 'settings' })
  }
}

function initApp() {
  if (appInitialized) return
  appInitialized = true

  updateMarketStatus()
  if (!marketStatusTimer) {
    marketStatusTimer = setInterval(() => {
      refreshMotto()
      updateMarketStatus()
    }, 60000)
  }
  updateResponsiveLayout()

  // 确保 SSE 连接就绪
  webEventBus.reconnectSSE()

  GetConfig().then((res) => {
    if (res.enableNews) {
      enableNews.value = true
    }
    enableFund.value = res.enableFund
    enableAgent.value = res.enableAgent
    const {notification } =createDiscreteApi(["notification"], {
      configProviderProps: {
        theme: enableDarkTheme.value ? darkTheme : lightTheme ,
        max: 3,
      },
    })
    EventsOn("newsPush", (data) => {
      //console.log(data)
      if(data.isRed){
        sendBrowserNotification(`🔥 突发快讯 [${data.time}]`, data.content)
        notification.create({
          //type:"error",
         // avatar: () => h(NIcon,{component:Notifications,color:"red"}),
          title: data.time,
          content: () => h('div',{type:"error",style:{
              "text-align":"left",
              "font-size":"14px",
              "color":"#f67979"
            }}, { default: () => data.content }),
          meta: () => h(NText,{type:"warning"}, { default: () => data.source}),
          duration:1000*40,
        })
      }else{
         notification.create({
          //type:"info",
          //avatar: () => h(NIcon,{component:Notifications}),
          title: data.time,
          content: () => h('div',{type:"info",style:{
            "text-align":"left",
              "font-size":"14px",
              "color": data.source==="blades-stock"?"#F98C24":"#549EC8"
            }}, { default: () => data.content }),
          meta: () => h(NText,{type:"warning"}, { default: () => data.source}),
          duration:1000*30 ,
        })
      }
    })

    // 监听价格预警触发及 Agent 智能归因推送
    EventsOn("stockPriceAlertTriggered", (data) => {
      sendBrowserNotification(`🚨 价格预警: ${data.name} (${data.code})`, `${data.reason}\n${data.aiAnalysis || ''}`)
      notification.warning({
        title: `🚨 价格预警: ${data.name} (${data.code})`,
        content: () => h('div', { style: { 'text-align': 'left', 'font-size': '13px' } }, [
          h('div', { style: { 'font-weight': 'bold', 'margin-bottom': '6px', 'color': '#ff7d00' } }, data.reason),
          h('div', { style: { 'white-space': 'pre-wrap', 'line-height': '1.5', 'background': 'rgba(0,0,0,0.05)', 'padding': '8px', 'border-radius': '4px', 'font-size': '12px' } }, 
            data.aiAnalysis || 'Agent 正在进行归因分析...'
          )
        ]),
        meta: () => h(NText, { depth: 3 }, { default: () => `${data.time} · AI 自动归因` }),
        duration: 1000 * 60 * 3, // 保持 3 分钟
      })
    })

    // 监听市场全流程智能研报推送（盘前/盘后）
    EventsOn("marketIntelligenceReport", (data) => {
      if (!data) return
      const isBrief = data.type === 'pre_market_briefing'
      sendBrowserNotification(data.title || (isBrief ? '🌅 盘前智能早报' : '🌆 收盘全景复盘'), data.summary || '')
      notification.success({
        title: isBrief ? `🌅 ${data.title}` : `🌆 ${data.title}`,
        content: () => h('div', { style: { 'text-align': 'left', 'font-size': '13px' } }, [
          h('div', { style: { 'font-weight': 'bold', 'margin-bottom': '6px', 'color': '#18a058' } }, data.summary),
          h('div', { style: { 'white-space': 'pre-wrap', 'line-height': '1.5', 'background': 'rgba(0,0,0,0.05)', 'padding': '8px', 'border-radius': '4px', 'font-size': '12px', 'max-height': '260px', 'overflow-y': 'auto' } },
            (data.content || '').slice(0, 500) + ((data.content || '').length > 500 ? '...' : '')
          )
        ]),
        action: () => h(NButton, {
          size: 'small',
          type: 'primary',
          secondary: true,
          onClick: () => onOpenMarketIntelligence(data.type)
        }, { default: () => '📖 查看宏观研报 ➔' }),
        meta: () => h(NText, { depth: 3 }, { default: () => `${new Date(data.generatedAt).toLocaleString()} · ${data.source || 'Agent'}` }),
        duration: 1000 * 60 * 5,
      })
    })
  }).catch(err => {
    console.error("GetConfig(onMounted) error:", err)
  })
}

onMounted(() => {
  window.addEventListener('resize', updateResponsiveLayout)
  updateResponsiveLayout()

  window.addEventListener('blades:authenticated', () => {
    isAuthenticated.value = true
    initApp()
  })
  window.addEventListener('blades:unauthorized', () => {
    if (!isWails()) {
      isAuthenticated.value = false
    }
  })

  if (isAuthenticated.value) {
    initApp()
  }
})
</script>
<template>
  <n-config-provider
    ref="containerRef"
    :theme="enableDarkTheme"
    :locale="zhCN"
    :date-locale="dateZhCN"
  >
    <n-message-provider>
      <n-notification-provider>
        <n-modal-provider>
          <n-dialog-provider>
            <WebAuthModal />
            <template v-if="isAuthenticated">
              <!-- 全局 Agent 控制抽屉 -->
              <StockDiagnosisDrawer
                v-model:show="showAgentDiagnosisDrawer"
                :stock-code="agentDiagnosisStockCode"
                :stock-name="agentDiagnosisStockName"
              />
              <n-flex>
                <n-grid
                  x-gap="12"
                  :cols="1"
                >
                  <n-gi>
                    <n-spin :show="loading">
                      <template #description>
                        {{ loadingMsg }}
                      </template>
                      <n-marquee
                        v-if="(telegraph.length>0)&&(enableNews)"
                        :speed="100"
                        style="position: relative;top:0;z-index: 19;width: 100%"
                      >
                        <n-tag
                          v-for="(item, idx) in telegraph"
                          :key="idx"
                          type="warning"
                          style="margin-right: 10px"
                        >
                          {{ item }}
                        </n-tag>
                      </n-marquee>
                      <!-- Agent 对话中枢具有完全独立的自包含视口与输入底栏，必须彻底脱离外层 n-scrollbar 与 72px 底部内边距，杜绝双滚动条互搏引起的剧烈上下跳动 -->
                      <div
                        v-if="isAgentRoute"
                        class="agent-standalone-host"
                        :style="agentHostStyle"
                      >
                        <RouterView />
                      </div>
                      <n-scrollbar
                        v-else
                        :style="contentStyle"
                      >
                        <n-skeleton
                          v-if="loading"
                          height="calc(100vh)"
                        />
                        <RouterView />
                        <!-- 全局合规免责声明底栏 -->
                        <div class="global-compliance-bar">
                          <span>【合规风险提示与免责声明】：本系统为量化投研算法与辅助分析软件，所有量化策略指标、历史回测数据、AI 研判推演及风控测算均基于公开客观历史数据与既定算法模型生成，不构成任何形式的投资建议、买卖要约或收益承诺。证券市场有风险，投资决策需审慎独立。</span>
                        </div>
                      </n-scrollbar>
                    </n-spin>
                  </n-gi>
                  <n-gi style="position: fixed;bottom:0;z-index: 99;width: 100%;">
                    <!-- 移动端原生 App 风格 TabBar (Screen <= 768px) -->
                    <div v-if="isMobile" class="mobile-tabbar-container">
                      <div class="mobile-tabbar">
                        <div
                          class="mobile-tab-item"
                          :class="{ active: currentMobileTab === 'stock' }"
                          @click="switchMobileTab('stock')"
                        >
                          <div class="mobile-tab-icon">
                            <n-icon size="20" :component="StarOutline" />
                          </div>
                          <span class="mobile-tab-label">自选</span>
                        </div>

                        <div
                          class="mobile-tab-item"
                          :class="{ active: currentMobileTab === 'market' }"
                          @click="switchMobileTab('market')"
                        >
                          <div class="mobile-tab-icon">
                            <n-icon size="20" :component="Pulse" />
                          </div>
                          <span class="mobile-tab-label">行情</span>
                        </div>

                        <div
                          class="mobile-tab-item"
                          :class="{ active: currentMobileTab === 'intelligence' }"
                          @click="switchMobileTab('intelligence')"
                        >
                          <div class="mobile-tab-icon">
                            <n-icon size="20" :component="AnalyticsOutline" />
                          </div>
                          <span class="mobile-tab-label">宏观智能</span>
                        </div>

                        <div
                          class="mobile-tab-item mobile-tab-agent"
                          :class="{ active: currentMobileTab === 'agent' }"
                          @click="switchMobileTab('agent')"
                        >
                          <div class="mobile-tab-icon agent-icon-glow">
                            <n-icon size="22" :component="Robot" />
                          </div>
                          <span class="mobile-tab-label">AI Agent</span>
                        </div>

                        <div
                          class="mobile-tab-item"
                          :class="{ active: currentMobileTab === 'research' }"
                          @click="switchMobileTab('research')"
                        >
                          <div class="mobile-tab-icon">
                            <n-icon size="20" :component="FlaskOutline" />
                          </div>
                          <span class="mobile-tab-label">研究</span>
                        </div>

                        <div
                          class="mobile-tab-item"
                          :class="{ active: currentMobileTab === 'settings' }"
                          @click="switchMobileTab('settings')"
                        >
                          <div class="mobile-tab-icon">
                            <n-icon size="20" :component="SettingsOutline" />
                          </div>
                          <span class="mobile-tab-label">设置</span>
                        </div>
                      </div>
                    </div>

                    <!-- 桌面端横向菜单 (Screen > 768px) -->
                    <n-card
                      v-else
                      size="small"
                      style="--wails-draggable:no-drag"
                    >
                      <n-menu
                        v-model:value="activeKey"
                        style="font-size: 18px;"
                        mode="horizontal"
                        :options="menuOptions"
                        responsive
                      />
                    </n-card>
                  </n-gi>
                </n-grid>
              </n-flex>
            </template>
          </n-dialog-provider>
        </n-modal-provider>
      </n-notification-provider>
    </n-message-provider>
  </n-config-provider>
</template>
<style>
/* Web 浏览器模式优雅降级与响应式适配 */
body.is-web-mode {
  user-select: auto !important;
  -webkit-user-select: auto !important;
  overflow-x: hidden;
  touch-action: pan-y;
  -webkit-tap-highlight-color: transparent;
}

body.is-web-mode [style*="--wails-draggable:drag"] {
  --wails-draggable: no-drag !important;
  -webkit-app-region: no-drag !important;
}

/* 消除小屏与 iPad 移动端横向溢出及浮动悬浮球避让 */
@media screen and (max-width: 768px) {
  body.is-web-mode .n-layout-sider {
    min-width: 0 !important;
  }
  body.is-web-mode .edge-trigger {
    bottom: 74px !important;
    right: 14px !important;
  }
  body.is-web-mode .drawer-panel {
    width: 100vw !important;
    max-width: 100vw !important;
  }
  body.is-web-mode .market-intelligence-drawer {
    width: 100vw !important;
    max-width: 100vw !important;
  }
}

/* -------------------------------------------------------------------------
   移动端原生级底部导航栏 (Mobile TabBar)
   ------------------------------------------------------------------------- */
.mobile-tabbar-container {
  width: 100%;
  background: rgba(255, 255, 255, 0.94);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border-top: 1px solid rgba(0, 0, 0, 0.07);
  box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.04);
  padding-bottom: max(6px, var(--sab, 0px));
}

html.dark .mobile-tabbar-container,
html[theme-mode='dark'] .mobile-tabbar-container,
body.is-dark .mobile-tabbar-container {
  background: rgba(20, 20, 24, 0.94);
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  box-shadow: 0 -4px 20px rgba(0, 0, 0, 0.25);
}

.mobile-tabbar {
  display: flex;
  align-items: center;
  justify-content: space-around;
  height: 52px;
  max-width: 600px;
  margin: 0 auto;
  user-select: none;
  -webkit-user-select: none;
}

.mobile-tab-item {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 2px;
  height: 100%;
  cursor: pointer;
  color: #8c8c94;
  transition: all 0.2s cubic-bezier(0.25, 1, 0.5, 1);
  -webkit-tap-highlight-color: transparent;
}

.mobile-tab-item:active {
  transform: scale(0.92);
}

.mobile-tab-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.2s ease;
}

.mobile-tab-label {
  font-size: 11px;
  font-weight: 500;
  line-height: 1.1;
  letter-spacing: 0.2px;
}

.mobile-tab-item.active {
  color: #18a058;
}

html.dark .mobile-tab-item.active,
html[theme-mode='dark'] .mobile-tab-item.active,
body.is-dark .mobile-tab-item.active {
  color: #63e2b7;
}

.mobile-tab-item.active .mobile-tab-icon {
  transform: translateY(-2px);
}

/* AI智能体特色微光动效 */
.mobile-tab-agent.active .agent-icon-glow {
  filter: drop-shadow(0 0 6px rgba(24, 160, 88, 0.45));
}

html.dark .mobile-tab-agent.active .agent-icon-glow,
html[theme-mode='dark'] .mobile-tab-agent.active .agent-icon-glow,
body.is-dark .mobile-tab-agent.active .agent-icon-glow {
  filter: drop-shadow(0 0 8px rgba(99, 226, 183, 0.6));
}

/* 全局合规免责声明条 */
.global-compliance-bar {
  text-align: center;
  padding: 12px 20px 20px 20px;
  font-size: 11.5px;
  color: #8b949e;
  line-height: 1.6;
  border-top: 1px dashed rgba(255, 255, 255, 0.08);
  margin-top: 24px;
}
html.light .global-compliance-bar,
html[theme-mode='light'] .global-compliance-bar {
  color: #666;
  border-top-color: rgba(0, 0, 0, 0.08);
}
</style>
