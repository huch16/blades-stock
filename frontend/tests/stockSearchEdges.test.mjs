// stockSearch.ts 边界用例补充（与 stockSearch.test.mjs 的回归用例互补）。
// 重点：isValidEastMoneyCode 定长校验的边界（乱码代码 600500000119.SH
// 必须拦下）、toEastMoneyCode 的前缀/ETF/港股分支、isCodeText 的
// gb_/hk 前缀、filter 的大小写、resolveSearchTarget 的空输入观察。
// 运行：npm test（node --test tests/*.test.mjs）。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  toEastMoneyCode,
  stripNameCodeFormat,
  filterStockOptions,
  isCodeText,
  resolveSearchTarget,
  isValidEastMoneyCode,
} from '../src/utils/stockSearch.ts'

const LIST = [
  { name: '平安银行', ts_code: '000001.SZ', symbol: '000001', cnspell: 'payh' },
  { name: '贵州茅台', ts_code: '600519.SH', symbol: '600519', cnspell: 'gzmt' },
  { name: '中国平安', ts_code: '601318.SH', symbol: '601318', cnspell: 'zgpa' },
]

test('isValidEastMoneyCode: 定长校验边界', () => {
  // 合法：A 股 6 位、港股 5 位、美股 1–5 位字母，小写后缀也认。
  assert.equal(isValidEastMoneyCode('000001.SZ'), true)
  assert.equal(isValidEastMoneyCode('600519.SH'), true)
  assert.equal(isValidEastMoneyCode('600519.sh'), true)
  assert.equal(isValidEastMoneyCode('920000.BJ'), true)
  assert.equal(isValidEastMoneyCode('600519.SS'), true)
  assert.equal(isValidEastMoneyCode('00700.HK'), true)
  assert.equal(isValidEastMoneyCode('AAPL.US'), true)
  assert.equal(isValidEastMoneyCode(' 600519.SH '), true)
  // 非法：位数不对、缺后缀、字母超长、空。
  assert.equal(isValidEastMoneyCode('600500000119.SH'), false) // 2026-11-21 线上乱码代码
  assert.equal(isValidEastMoneyCode('0070.HK'), false) // 港股 4 位
  assert.equal(isValidEastMoneyCode('007000.HK'), false) // 港股 6 位
  assert.equal(isValidEastMoneyCode('60051.SH'), false) // A 股 5 位
  assert.equal(isValidEastMoneyCode('600519'), false) // 缺后缀
  assert.equal(isValidEastMoneyCode('ABCDEF.US'), false) // 美股 6 字母
  assert.equal(isValidEastMoneyCode(''), false)
  assert.equal(isValidEastMoneyCode('贵州茅台'), false)
})

test('isValidEastMoneyCode 与 toEastMoneyCode 的分工：后者会给乱码补后缀，前者才拦得住', () => {
  // toEastMoneyCode 对任意位数字都照常补后缀——这正是不能用它代替校验的原因。
  assert.equal(toEastMoneyCode('600500000119'), '600500000119.SH')
  assert.equal(isValidEastMoneyCode(toEastMoneyCode('600500000119')), false)
  assert.equal(isValidEastMoneyCode(toEastMoneyCode('600519')), true)
})

test('toEastMoneyCode: 前缀 / ETF / 港股 / 美股分支', () => {
  assert.equal(toEastMoneyCode('hk00700'), '00700.HK')
  assert.equal(toEastMoneyCode('HK00700'), '00700.HK')
  assert.equal(toEastMoneyCode('usAAPL'), 'AAPL.US')
  assert.equal(toEastMoneyCode('gb_aapl'), 'AAPL.US')
  assert.equal(toEastMoneyCode('510300'), '510300.SH') // 沪市 ETF
  assert.equal(toEastMoneyCode('159915'), '159915.SZ') // 深市 ETF
  assert.equal(toEastMoneyCode('00700'), '00700.SZ') // 纯数字 00700 按首位定深市（港股须带 hk 前缀或 .HK 后缀）
  assert.equal(toEastMoneyCode(' 600519 '), '600519.SH')
  assert.equal(toEastMoneyCode('贵州茅台'), '') // 中文名不是代码，解析为空
  assert.equal(toEastMoneyCode('600519.SH '), '600519.SH')
})

test('stripNameCodeFormat: 括号格式与非括号原样返回', () => {
  assert.equal(stripNameCodeFormat('平安银行 (000001.SZ)'), '000001.SZ')
  assert.equal(stripNameCodeFormat('贵州茅台(600519.SH)'), '600519.SH')
  assert.equal(stripNameCodeFormat('贵州茅台 (600519.sh)'), '600519.sh')
  assert.equal(stripNameCodeFormat('600519'), '600519')
  assert.equal(stripNameCodeFormat(''), '')
})

test('isCodeText: gb_/hk 前缀与纯字母的区分', () => {
  assert.equal(isCodeText('gb_aapl'), true)
  assert.equal(isCodeText('hk00700'), true)
  assert.equal(isCodeText('us123'), true) // us 前缀后须跟数字
  assert.equal(isCodeText('usAAPL'), false) // us 后跟字母不是代码原文（与 AAPL 同理）
  assert.equal(isCodeText('00700'), true)
  assert.equal(isCodeText('00700.HK'), true)
  assert.equal(isCodeText('aapl'), false) // 纯字母可能是拼音或美股代码，不算“合法代码原文”
  assert.equal(isCodeText('BABA'), false)
})

test('filterStockOptions: 大小写不敏感、空查询返回空', () => {
  assert.deepEqual(filterStockOptions(LIST, 'GZMT').map((s) => s.ts_code), ['600519.SH'])
  assert.deepEqual(filterStockOptions(LIST, 'PAYH').map((s) => s.ts_code), ['000001.SZ'])
  assert.deepEqual(filterStockOptions(LIST, '平安').map((s) => s.ts_code), ['000001.SZ', '601318.SH'])
  assert.deepEqual(filterStockOptions(LIST, ''), [])
  assert.deepEqual(filterStockOptions([], 'gzmt'), [])
})

test('resolveSearchTarget: 方向键选中优先于一切；合法代码原文不受建议影响', () => {
  assert.equal(resolveSearchTarget('gzmt', '600519.SH', true, []), '600519.SH')
  assert.equal(resolveSearchTarget('600519.SH', '000001.SZ', false, ['000001.SZ']), '600519.SH')
  assert.equal(resolveSearchTarget('sh600519', '', false, ['000001.SZ']), 'sh600519')
})

// 空输入 + 恰好一条建议时会返回建议（当前行为钉住；
// 若未来认为空输入不应跳转，改这里的同时要同步改 kline-analysis.vue 的回车处理）。
test('resolveSearchTarget: 空输入配唯一建议时的当前行为', () => {
  assert.equal(resolveSearchTarget('', '', false, ['600519.SH']), '600519.SH')
})
