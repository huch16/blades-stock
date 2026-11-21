// 股票搜索纯函数：K 线分析页搜索框的解析与联想逻辑。
// 从 kline-analysis.vue 抽出，便于单元测试（node --test tests/stockSearch.test.mjs）。
// 注意：本文件只用可擦除的 TypeScript 语法（无枚举/命名空间/参数属性），
// 以便 Node 24 的类型剥离直接运行。

export interface StockListItem {
  name?: string
  ts_code?: string
  symbol?: string
  cnspell?: string
}

// 纯数字代码按首位定市场；标准后缀/前缀直接规范化；纯字母视为美股。
// 兼容 "天目药业 - 600671.SH" 或 "天目药业 600671"（取最后一段）。
export function toEastMoneyCode(code: string): string {
  if (!code) return ''
  let c = String(code).trim()
  // 兼容 "天目药业 - 600671.SH" 或 "天目药业 600671"
  for (const sep of [' - ', '-', ' ']) {
    if (c.includes(sep)) {
      const parts = c.split(sep)
      const last = parts[parts.length - 1]
      if (last !== undefined) c = last.trim()
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

// 剥掉 “名字 (代码)” 格式，取括号内的东财代码；无括号时原样返回。
// handleSearch 会把框写成 “平安银行 (000001.SZ)”，不剥掉就解析失败。
export function stripNameCodeFormat(input: string): string {
  const m = String(input).trim().match(/\(\s*([A-Za-z0-9]{2,}\.(?:SH|SZ|BJ|HK|US|SS))\s*\)\s*$/i)
  if (m && m[1]) return m[1]
  return String(input).trim()
}

// 规范东财代码格式校验：只认“定长数字 + 标准后缀”。
// A 股 6 位数字 + .SH/.SZ/.BJ/.SS；港股 5 位数字 + .HK；美股 1–5 位字母 + .US。
// 注意不要用“像不像代码原文”的宽松判断代替本函数：任意位数字 + 后缀
// （如 600500000119.SH）按宽松判断也会通过；本函数才是恢复 / 入库前的
// 守门员——toEastMoneyCode 对 "600500000119" 也会照常补出 "600500000119.SH"，
// 必须在这里拦下。
export function isValidEastMoneyCode(code: string): boolean {
  if (!code) return false
  const c = String(code).trim().toUpperCase()
  return /^(\d{6}\.(SH|SZ|BJ|SS)|\d{5}\.HK|[A-Z]{1,5}\.US)$/.test(c)
}

// 内存列表联想匹配（支持名称、代码、纯数字 symbol、拼音缩写 cnspell），按 includes 取前 limit 条。
// 注意：920000.BJ（安徽凤凰）的 name/ts_code/symbol/cnspell 均不含 "000001"，
// 所以输入 000001 时它不可能从这里进入联想列表（问题 2 的回归点）。
export function filterStockOptions(list: StockListItem[], query: string, limit = 30): StockListItem[] {
  const q = query.trim().toLowerCase()
  if (!q) return []
  return list.filter(item =>
    (item.name ?? '').toLowerCase().includes(q) ||
    (item.ts_code ?? '').toLowerCase().includes(q) ||
    (item.symbol ?? '').toLowerCase().includes(q) ||
    (item.cnspell ?? '').toLowerCase().includes(q)
  ).slice(0, limit)
}

// 输入本身是不是合法的东财代码原文：纯数字（000001）、带标准后缀
// （600519.SH）、或带市场前缀 + 数字（sh600519、sz000001、gb_aapl）。
// 纯字母（如 gzmt / AAPL）不算——它可能是拼音缩写，需让位于唯一联想建议；
// 无建议时仍由 toEastMoneyCode 按美股补 .US。
// 注意前缀后必须跟数字：拼音 "shyh"（上海银行）不能误判为 sh 前缀代码。
export function isCodeText(input: string): boolean {
  const c = stripNameCodeFormat(input)
  if (!c) return false
  if (/\.(SH|SZ|BJ|HK|US|SS)$/i.test(c)) return true
  if (/^\d+$/.test(c)) return true
  if (/^(sh|sz|bj|hk|us)\d+$/i.test(c)) return true
  if (/^gb_[a-z]+$/i.test(c)) return true
  return false
}

// 搜索目标选择：回车/放大镜按输入原文解析，精确代码优先于联想建议；
// 只有用户用方向键明确选中过建议（keyboardSelected）时，才按建议值走。
// 解决 naive-ui auto-complete 自动高亮首项导致回车误跳的问题（问题 2）。
// 例外（问题 A）：输入本身不是合法代码原文（如拼音 gzmt、中文名），且当前
// 联想列表恰好只有 1 条（suggestions）时，优先用该建议——否则 gzmt 会被
// toEastMoneyCode 按纯字母补成 GZMT.US。多条/无建议时仍按原文解析。
// suggestions 取组件回车时刻实际持有的联想列表的代码值（options.value 的
// value 字段）；本地过滤结果优先，远程结果由 findStockList 的 generation
// 守卫保证过期回包写不进来。
export function resolveSearchTarget(
  rawText: string,
  suggestionValue: string,
  keyboardSelected: boolean,
  suggestions: string[] = [],
): string {
  if (keyboardSelected && suggestionValue) return suggestionValue
  if (isCodeText(rawText)) return rawText
  if (suggestions.length === 1 && suggestions[0]) return suggestions[0]
  return rawText
}
