// BUIS 五阶投研流水线：贯通标的、步骤执行器、极速工具、标的填装弹窗、股票字典。
// 由 agent-chat.vue 抽出（R-22c），逻辑逐字搬运；共享状态与服务由组件以 ctx 传入。
import { computed, onMounted, reactive, ref } from 'vue'
import { GetStockList } from '../../wailsjs/go/main/App'

export function useAgentChatPipeline(ctx) {
  const applyScenario = ctx.applyScenario
  const fillInput = ctx.fillInput
  const inputValue = ctx.inputValue
  const messages = ctx.messages
  const perceptionSnapshot = ctx.perceptionSnapshot
  const getTrafficText = ctx.getTrafficText

// ========== BUIS 五阶流水线与交互控制中枢状态 ==========
const welcomeViewMode = ref<'pipeline' | 'direct'>('pipeline')
const step4StockInput = ref('600519')
const directDiagCode = ref('600519')
const directRiskCode = ref('600519')
const pkCodeA = ref('300750')
const pkCodeB = ref('002594')
const hotStocks = [
  { code: '600519', name: '贵州茅台' },
  { code: '300750', name: '宁德时代' },
  { code: '002594', name: '比亚迪' },
  { code: '688981', name: '中芯国际' },
  { code: '300059', name: '东方财富' },
  { code: '601318', name: '中国平安' },
  { code: '600900', name: '长江电力' },
]
// 全局股票代码字典（从后端加载，包含A股/ETF/指数等）
// 格式: { '600519': '贵州茅台', '300750': '宁德时代', ... }
const stockDict = ref<Record<string, string>>({})
// 初始化股票字典（从后端加载全量股票列表）
async function initStockDict() {
  try {
    const list = await GetStockList('')
    if (list && list.length > 0) {
      for (const item of list) {
        const code = item.stockCode || item.code || item.ts_code
        const name = item.name || item.stockName
        if (code && name) {
          const cleanCode = code.replace(/\.(SH|SZ|BJ)$/i, '').toUpperCase()
          if (!stockDict.value[cleanCode]) {
            stockDict.value[cleanCode] = item.name
          }
        }
      }
      // 合并 hotStocks 作为兜底
      for (const stk of hotStocks) {
        if (!stockDict.value[stk.code]) {
          stockDict.value[stk.code] = stk.name
        }
      }
      // 挂载到全局，供 extractStockFromAssistantText 使用
      ;(window as any).__STOCK_DICT__ = stockDict.value
    }
  } catch (e) {
    console.warn('[agent-chat] 股票字典初始化失败:', e)
    // 兜底：使用 hotStocks
    for (const stk of hotStocks) {
      stockDict.value[stk.code] = stk.name
    }
    ;(window as any).__STOCK_DICT__ = stockDict.value
  }
}
// 组件挂载时初始化股票字典
onMounted(() => {
  initStockDict()
})
// 交互式标的填装弹窗状态（彻底消灭占位符）
const showStockActionModal = ref(false)
const targetStockAction = ref<'diag' | 'risk' | 'finance' | 'pk'>('diag')
const modalStockInput = ref('')
const modalStockInputB = ref('')

const stockActionModalTitle = computed(() => {
  if (targetStockAction.value === 'diag') return '🔬 标的全维深度体检'
  if (targetStockAction.value === 'risk') return '🛡️ 股权质押与风险极限排雷'
  if (targetStockAction.value === 'finance') return '📑 经营现金流与真实ROE体检'
  if (targetStockAction.value === 'pk') return '⚖️ 赛道双龙头深度对比 PK'
  return '🎯 标的快速填装'
})

const isModalStockActionValid = computed(() => {
  if (targetStockAction.value === 'pk') {
    return Boolean(modalStockInput.value.trim() && modalStockInputB.value.trim())
  }
  return Boolean(modalStockInput.value.trim())
})

function setModalPK(a: string, b: string) {
  modalStockInput.value = a
  modalStockInputB.value = b
}

function openStockActionModal(action: 'diag' | 'risk' | 'finance' | 'pk') {
  targetStockAction.value = action
  // 若输入框已包含 6 位股票代码，直接复用
  const matched = inputValue.value.match(/\b\d{6}\b/)
  if (matched && action !== 'pk') {
    modalStockInput.value = matched[0]
  } else if (!modalStockInput.value) {
    modalStockInput.value = '600519'
  }
  if (action === 'pk' && !modalStockInputB.value) {
    modalStockInput.value = '300750'
    modalStockInputB.value = '002594'
  }
  showStockActionModal.value = true
}

function submitModalStockAction() {
  if (!isModalStockActionValid.value) return
  const codeA = modalStockInput.value.trim()
  const codeB = modalStockInputB.value.trim()
  showStockActionModal.value = false

  if (targetStockAction.value === 'diag') {
    applyScenario(`【L4 个股全景四维体检】请对股票 ${codeA} 进行全维度量化深度诊断：
1. 优先调用系统内置 DiagnoseStock(${codeA}) 获取 VectorBT 向量化回测置信胜率、量化综合评分与关键支撑阻力位；
2. 调用 GetEastMoneyKLineWithMA(${codeA}, "day", 60) 查看日K线均线系统与量价趋势；
3. 调用 GetStockFinancialInfo(${codeA}) 与 GetStockLatestFinance(${codeA}) 穿透营收利润与真实ROE；
4. 调用 GetStockHolderTrend(${codeA}) 穿透近几个季度股东户数增减与筹码集中度。
【执行纪律】：直接输出结构化量化研报，严禁在正文输出"下一步需要调用..."等机械规划文字。`, false, 'pipeline')
  } else if (targetStockAction.value === 'risk') {
    applyScenario(`【L4 筹码与风险极限排雷】请对股票 ${codeA} 进行深度排雷诊断：
1. 调用 GetStockHolderTrend(${codeA}) 穿透股东户数变动趋势，判断主力吸筹或散户接盘；
2. 调用 GetStockMarginTrading(${codeA}) 分析两融杠杆资金多空动向；
3. 核查控股股东股权质押比例、未来一年限售解禁日程、商誉减值与资产负债率风险。
【执行纪律】：直接输出专业排雷研报与风控评级。`, false, 'pipeline')
  } else if (targetStockAction.value === 'finance') {
    applyScenario(`【L4 财报体检与真实ROE验证】请调用系统内置 GetStockFinancialInfo(${codeA}) 与 GetStockLatestFinance(${codeA}) 深度体检财务质量：
1. 对比近3年营业收入与扣非净利润增长趋势；
2. 穿透销售毛利率、真实ROE与净利率水平；
3. 核验经营活动现金流净额与净利润比值，验证其盈利真实性，排查应收账款与坏账风险。
【执行纪律】：直接输出财务健康度体检结论。`, false, 'pipeline')
  } else if (targetStockAction.value === 'pk') {
    applyScenario(`【L4 赛道双龙头深度PK】请调用系统内置量化与行情工具，对比分析股票 ${codeA} 与 ${codeB}：
1. 分别调用 GetEastMoneyKLineWithMA(${codeA}, "day", 60) 与 GetEastMoneyKLineWithMA(${codeB}, "day", 60) 对比量价强弱与均线排列；
2. 分别调用 GetStockLatestFinance 对比两者的市盈率估值、真实ROE与营收成长质量；
3. 分别调用 GetStockHolderTrend 对比两者的筹码集中度变动。
【执行纪律】：直接输出双龙头横向对比报告与配置优选建议。`, false, 'pipeline')
  }
}
// ========== 五阶时序流水线贯通上下文（确保 L1 -> L2 -> L3 -> L4 -> L5 实体数据无缝流转） ==========
const pipelineContext = reactive({
  currentStep: 1,
  stockCode: '600519',
  stockName: '贵州茅台',
  macroStance: '',
  positionCap: '',
  mainTrack: '',
  lastVerdict: '',
})

function setPipelineStock(code: string, name?: string) {
  const trimmed = (code || '').trim()
  if (!trimmed) return
  pipelineContext.stockCode = trimmed
  step4StockInput.value = trimmed
  directDiagCode.value = trimmed
  directRiskCode.value = trimmed
  if (name) {
    pipelineContext.stockName = name.trim()
  } else {
    const found = hotStocks.find(s => s.code === trimmed)
    if (found) pipelineContext.stockName = found.name
    else pipelineContext.stockName = ''
  }
}

function onPipelineStockCodeChange(val: string) {
  const trimmed = (val || '').trim()
  pipelineContext.stockCode = trimmed
  step4StockInput.value = trimmed
  directDiagCode.value = trimmed
  directRiskCode.value = trimmed
  const found = hotStocks.find(s => s.code === trimmed)
  if (found) {
    pipelineContext.stockName = found.name
  } else if (!val) {
    pipelineContext.stockName = ''
  }
}

function onPipelineStockCodeEnter() {
  const trimmed = (pipelineContext.stockCode || '').trim()
  if (trimmed) {
    setPipelineStock(trimmed)
  }
}

// 从助手回答中智能抽取选股/推荐标的代码与名称，自动贯通到下游体检与立论
function extractStockFromAssistantText(text: string) {
  if (!text) return
  // 全局股票代码字典（从后端加载，包含A股/ETF/指数等）
  const stockDict: Record<string, string> = (window as any).__STOCK_DICT__ || {}

  // 辅助函数：验证股票名称是否在字典中
  function validateStockName(code: string, name: string): boolean {
    if (!name || name.length < 2 || name.length > 20) return false
    // 过滤明显的非股票名称片段
    const invalidPatterns = [/^的.*/, /^全天$/, /^最新$/, /^最新$/, /^第.*$/, /^共.*$/, /^前.*$/, /^后.*$/, /^上.*$/, /^下.*$/, /^左.*$/, /^右.*$/, /^中.*$/, /^全.*$/]
    for (const pattern of invalidPatterns) {
      if (pattern.test(name)) return false
    }
    // 如果字典中有该代码，校验名称是否匹配
    if (stockDict[code]) {
      return stockDict[code] === name
    }
    // 允许未收录的代码，但要求名称看起来像股票名称（2-10个汉字，无明显非法片段）
    return /^[\u4e00-\u9fa5]{2,10}$/.test(name)
  }

  // 辅助函数：设置管线股票（带校验）
  function validatedSetPipelineStock(code: string, name: string) {
    if (validateStockName(code, name)) {
      setPipelineStock(code, name)
    } else {
      // 仅设置代码，不设置名称（由 setPipelineStock 回退查找 hotStocks）
      setPipelineStock(code, '')
    }
  }

  // 1. 优先匹配 Markdown 表格数据行：| 1 | 002371 | 北方华创 |
  // 注意：字符类 [^|\r\n] 内的 | 无需转义；但字符类之外的 \| 是「字面竖线」，
  // 绝不能写成 |，否则会退化为正则的「或」运算符而彻底改变语义。
  const tableMatch = text.match(/\|\s*\d+\s*\|\s*(\d{6})\s*\|\s*([^|\r\n]+)\s*\|/)
  if (tableMatch && tableMatch[1]) {
    const code = tableMatch[1].trim()
    const name = tableMatch[2].trim().replace(/\s+/g, '')
    validatedSetPipelineStock(code, name)
    return
  }
  // 2. 匹配 "北方华创 (002371)"
  const parenMatch = text.match(/([\u4e00-\u9fa5A-Za-z0-9]{2,12})\s*[(（](\d{6})[)）]/)
  if (parenMatch && parenMatch[2]) {
    validatedSetPipelineStock(parenMatch[2].trim(), parenMatch[1].trim())
    return
  }
  // 3. 匹配 "002371 北方华创"
  const codeNameMatch = text.match(/(?:\b|\D)(\d{6})(?:\b|\D)\s*([\u4e00-\u9fa5]{2,10})/)
  if (codeNameMatch && codeNameMatch[1]) {
    validatedSetPipelineStock(codeNameMatch[1].trim(), codeNameMatch[2].trim())
    return
  }
  // 4. 匹配 "代码: 002371 名称: 北方华创" 等格式
  const labelMatch = text.match(/(?:代码|股票代码|代码)[:：\s]*(\d{6})[^\u4e00-\u9fa5]*([\u4e00-\u9fa5]{2,10})/)
  if (labelMatch && labelMatch[1]) {
    validatedSetPipelineStock(labelMatch[1].trim(), labelMatch[2].trim())
    return
  }
}

// ========== 五阶时序流水线快速执行方法（全链路绑定 Blades 内置工具与权威数据） ==========
function executeStep1() {
  pipelineContext.currentStep = 1
  const snap = perceptionSnapshot.value
  let realTimeFacts = ''
  if (snap) {
    const lightText = getTrafficText(snap.TrafficLight || snap.trafficLight)
    const phase = snap.Phase || snap.phaseDesc || snap.clockPhase || '正常交易时钟'
    const up = snap.UpCount || snap.upCount || 0
    const down = snap.DownCount || snap.downCount || 0
    const limitUp = snap.LimitUpCount || snap.limitUpCount || 0
    const limitDown = snap.LimitDownCount || snap.limitDownCount || 0
    pipelineContext.macroStance = lightText
    realTimeFacts = `
【系统实时态势感知一手事实（由 blades 引擎刚刚捕获）】：
- 盘面交易节律：${phase}
- 多空红绿灯状态：${lightText} (${snap.TrafficLight || snap.trafficLight})
- 全市场涨跌家数比：上涨 ${up} 家 vs 下跌 ${down} 家
- 涨跌停极端对比：涨停 ${limitUp} 家 vs 跌停 ${limitDown} 家`
  }

  applyScenario(`【L1 宏观大势与仓位择时】${realTimeFacts}
【执行纪律与工具调用要求】：
1. 优先调用系统内置 GetMarketData 获取核心宽基指数（上证指数 000001、沪深300 000300、创业板指 399006）的最新行情截面。
2. 严禁在回答中输出任何"下一步需要调用..."、"我将查询..."等机械思考或规划句子。
3. 直接输出结构化宏观研报：
   - 一、大盘核心指数行情与量价均线态势（调用 GetMarketData）
   - 二、全市场微观多空博弈与红绿灯节律诊断（结合上述系统实盘数据）
   - 三、宏观择时评级与总仓位上限指导（必须给出明确的最高持仓上限百分比，例如 ≤30% 或 ≤70%）。`, false, 'pipeline')
}

function executeStep2(type: 'etf' | 'seats') {
  pipelineContext.currentStep = 2
  if (type === 'etf') {
    applyScenario(`【L2 赛道主线轮动】请调用系统内置 GetETFMomentumRanking 工具获取场内核心 ETF 21日加权动量排名：

2. 完整列出动量评分 >0 的正动量进攻候选池，并根据动量衰减度指出当前资金流入最强的主线板块；
3. 对动量 <=0 的防守/规避板块给出调仓规避建议。
4. 直接输出研报正文，严禁输出任何机械调用步骤或规划文字。`, false, 'pipeline')
  } else {
    applyScenario(`【L2 主力风向追踪】请调用系统内置 GetLongTigerList / GetStockBillboard 工具获取最新龙虎榜明细：

2. 重点筛选机构专用席位大额净买入的股票及所属板块，分析机构资金的真实建仓方向；
3. 排查游资席位是否存在"一日游"打板出逃迹象。
4. 直接输出研报正文，严禁输出机械调用步骤。`, false, 'pipeline')
  }
}

function executeStep3(type: 'breakout' | 'trend' | 'macd' | 'dividend') {
  pipelineContext.currentStep = 3
  if (type === 'breakout') {
    applyScenario(`【L3 量化多因子初筛池 · 放量突破】请调用系统内置 ScreenByStrategy(strategyName="放量突破", limit=10) 工具：
1. 筛选全市场呈现成交量放量突破近期整理平台的强势标的；

3. 输出入选股票的具体代码、名称、技术面评分、现价、涨跌幅、量比与突破确认点位；
4. 综合评选出最优的前3只标的，系统将自动继承首选标的供后续 Step 4 四维深度体检。直接输出选股清单。`, false, 'pipeline')
  } else if (type === 'trend') {
    applyScenario(`【L3 量化多因子初筛池 · 均线多头】请调用系统内置 ScreenByStrategy(strategyName="趋势跟随", limit=10) 工具：
1. 筛选均线多头排列（MA5>MA10>MA20）且趋势持续向上的强势标的；

3. 输出入选股票的具体代码、名称、技术面评分与均线支撑位，直接输出选股清单。`, false, 'pipeline')
  } else if (type === 'macd') {
    applyScenario(`【L3 量化多因子初筛池 · MACD金叉】请调用系统内置 ScreenByStrategy(strategyName="macd_golden_cross", limit=10) 工具：
1. 筛选近期在零轴上方形成二次金叉放量共振的个股；

3. 输出入选股票的具体代码、名称与金叉确认理由，直接输出选股清单。`, false, 'pipeline')
  } else {
    applyScenario(`【L3 价值防守选股池】请利用系统内置量化筛选工具（FilterStocks / ScreenByStrategy）：
1. 筛选市盈率PE<15、近三年平均股息率>3.5%、ROE>12%的高股息稳健型股票；

3. 评估入选标的的安全边际与抗跌属性，给出防御性底仓配置建议，直接输出选股清单。`, false, 'pipeline')
  }
}

function executeStep4(action: 'diag' | 'risk' | 'finance') {
  pipelineContext.currentStep = 4
  const code = (step4StockInput.value || pipelineContext.stockCode || '600519').trim()
  const name = pipelineContext.stockName || ''
  const targetDesc = name ? `${name} (${code})` : `股票 ${code}`

  if (action === 'diag') {
    applyScenario(`【L4 个股全景深度体检】请对 ${targetDesc} 进行全维度深度量化诊断：
1. 优先调用系统内置 DiagnoseStock(${code}) 获取 VectorBT 向量化回测置信胜率、量化综合评分与第一目标价/止损位；
2. 调用 GetEastMoneyKLineWithMA(${code}, "day", 60) 查看日K线均线系统与关键支撑阻力位；
3. 调用 GetStockFinancialInfo(${code}) 与 GetStockLatestFinance(${code}) 穿透营收利润与真实ROE；
4. 调用 GetStockHolderTrend(${code}) 穿透股东户数集中度与主力资金动向。
【执行纪律】：直接输出专业研报，给出买方 5-Verdict 确定性评级与明确的下行证伪止损线，严禁输出机械思考文字。`, false, 'pipeline')
  } else if (action === 'risk') {
    applyScenario(`【L4 筹码与风险极限排雷】请对 ${targetDesc} 进行深度排雷：
1. 调用 GetStockHolderTrend(${code}) 穿透股东户数集中度趋势；
2. 调用 GetStockMarginTrading(${code}) 分析两融杠杆资金多空动向；
3. 核查控股股东股权质押比例、未来一年限售解禁日程、商誉减值与资产负债率风险。
【执行纪律】：直接输出风控评级与排雷研报。`, false, 'pipeline')
  } else {
    applyScenario(`【L4 财报体检与真实ROE验证】请调用系统内置 GetStockFinancialInfo(${code}) 与 GetStockLatestFinance(${code}) 深度体检 ${targetDesc} 的财务质量：
1. 对比近3年营业收入与扣非净利润增长趋势；
2. 穿透销售毛利率、真实ROE与销售净利率；
3. 核验经营活动现金流净额与净利润比值，验证其盈利真实性，排查应收账款异常。
【执行纪律】：直接输出财务健康度结论。`, false, 'pipeline')
  }
}

function executeStep5(customCode?: string) {
  pipelineContext.currentStep = 5
  const code = (customCode || step4StockInput.value || pipelineContext.stockCode || '600519').trim()
  const name = pipelineContext.stockName || ''
  const targetDesc = name ? `${name} (${code})` : `股票 ${code}`

  applyScenario(`【L5 买方立论与 5-Verdict 裁决】请根据前期宏观大势、赛道动量、量化体检与风控排雷结论，对 ${targetDesc} 作出最终买方投资决断：
1. 明确给出严肃的 5-Verdict 买方评级（【强力买入】、【逢低布局】、【观望等待】、【逢高减持】、【规避离场】）；
2. 提炼核心买入立论假说（Core Thesis，1-2句话高度概括其商业壁垒与成长逻辑）；
3. 明确列出下一阶段重点跟踪的验证催化事件（Key Catalysts）；
4. 设定严格的下行证伪止损底线价位（Risk Factors / Invalidation Price）；
5. 【核心纪律与工具调用】：必须直接调用系统内置 SaveStockThesis 工具，将上述结构化投资假说正式持久化存入系统认知账本数据库！并在回答末尾输出认知账本保存凭据。`, false, 'pipeline')
}

function viewVerdictStandards() {
  applyScenario(`请说明 Blades 买方认知账本与 5-Verdict 评级规范：包括【强力买入、逢低布局、观望等待、逢高减持、规避离场】的评定边界、核心买入立论提取、催化剂假设提取以及下行证伪止损底线的设定方法。`, false, 'pipeline')
}

function executeDirectDiag() {
  const code = (directDiagCode.value || '600519').trim()
  setPipelineStock(code)
  applyScenario(`【L4 极速全维体检】请对股票 ${code} 进行全维度深度诊断：优先调用系统内置 DiagnoseStock(${code}) 获取 VectorBT 量化评分与置信胜率，调用 GetEastMoneyKLineWithMA(${code}, "day", 60) 查看均线量价，调用 GetStockLatestFinance 查看财务质量。直接给出买方 5-Verdict 裁决与止损位。`, false, 'pipeline')
}

function executeDirectRisk() {
  const code = (directRiskCode.value || '600519').trim()
  setPipelineStock(code)
  applyScenario(`【L4 专项极限排雷】请对股票 ${code} 进行深度排雷：调用 GetStockHolderTrend(${code}) 查看股东户数集中度，调用 GetStockMarginTrading(${code}) 查看两融杠杆，核查股权质押与限售解禁日程。直接输出综合排雷评级。`, false, 'pipeline')
}

function executeDirectPK() {
  const a = (pkCodeA.value || '300750').trim()
  const b = (pkCodeB.value || '002594').trim()
  applyScenario(`【L4 双龙头横向对比】请调用系统内置量化工具对比 ${a} 与 ${b}：分别对比两者的日K量价走势（GetEastMoneyKLineWithMA）、最新财务ROE与估值（GetStockLatestFinance）、以及筹码集中度（GetStockHolderTrend）。直接输出双龙对比结论。`, false, 'pipeline')
}

function applyPresetPK(a: string, b: string) {
  pkCodeA.value = a
  pkCodeB.value = b
}

function handleQuickChipClick(chip: any) {
  if (typeof chip.action === 'function') {
    chip.action()
  } else if (chip.text) {
    fillInput(chip.text)
  }
}

// 下游时序推荐计算：根据最近一条助手的输出推演内容，智能推荐下一步动作（自动继承贯通标的）
const pipelineNextStep = computed(() => {
  if (messages.value.length === 0) return null
  const lastMsg = messages.value[messages.value.length - 1]
  if (lastMsg.role !== 'assistant' || !lastMsg.content) return null
  const text = lastMsg.content

  const currentCode = pipelineContext.stockCode || '600519'
  const currentName = pipelineContext.stockName || ''
  const displayTarget = currentName ? `${currentName} (${currentCode})` : currentCode

  // 1. 如果是宏观大势或 ETF 动量分析 -> 推荐进入 Step 3 进行多因子选股
  if (text.includes('动量') || text.includes('ETF') || text.includes('大势') || text.includes('择时') || text.includes('宏观')) {
    return {
      label: '👉 进入 Step 3 · 运行多因子放量突破量化选股',
      action: () => executeStep3('breakout'),
    }
  }
  // 2. 如果是选股初筛池 -> 推荐进入 Step 4 对筛选出的标的进行全维体检
  if (text.includes('选股') || text.includes('突破') || text.includes('初筛') || text.includes('高股息') || text.includes('多头排列')) {
    return {
      label: `👉 进入 Step 4 · 对标的「${displayTarget}」启动四维体检与极限排雷`,
      action: () => executeStep4('diag'),
    }
  }
  // 3. 如果是个股体检、排雷或财报分析 -> 推荐进入 Step 5 做出终局买方立论并写入账本
  if (text.includes('诊断') || text.includes('体检') || text.includes('排雷') || text.includes('5-Verdict') || text.includes('VectorBT')) {
    return {
      label: `👉 进入 Step 5 · 对「${displayTarget}」做出买方 5-Verdict 裁决并存入认知账本`,
      action: () => executeStep5(),
    }
  }
  return null
})

  return { welcomeViewMode, step4StockInput, directDiagCode, directRiskCode, pkCodeA, pkCodeB, hotStocks, showStockActionModal, targetStockAction, modalStockInput, modalStockInputB, stockActionModalTitle, isModalStockActionValid, setModalPK, openStockActionModal, submitModalStockAction, pipelineContext, setPipelineStock, onPipelineStockCodeChange, onPipelineStockCodeEnter, extractStockFromAssistantText, executeStep1, executeStep2, executeStep3, executeStep4, executeStep5, viewVerdictStandards, executeDirectDiag, executeDirectRisk, executeDirectPK, applyPresetPK, handleQuickChipClick, pipelineNextStep }
}
