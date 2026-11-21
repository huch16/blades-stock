// 搜索框问题 2 回归测试：原文解析优先于联想建议。
// 运行：npm test（或 node --test tests/）。Node 24 类型剥离直接导入
// ../src/utils/stockSearch.ts，无需 vitest。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  toEastMoneyCode,
  stripNameCodeFormat,
  filterStockOptions,
  resolveSearchTarget,
} from '../src/utils/stockSearch.ts'

const LIST = [
  { name: '平安银行', ts_code: '000001.SZ', symbol: '000001', cnspell: 'payh' },
  { name: '贵州茅台', ts_code: '600519.SH', symbol: '600519', cnspell: 'gzmt' },
  { name: '安徽凤凰', ts_code: '920000.BJ', symbol: '920000', cnspell: 'ahfh' },
  { name: '浦发银行', ts_code: '600000.SH', symbol: '600000', cnspell: 'pfyh' },
]

test('toEastMoneyCode: 纯数字按首位定市场', () => {
  assert.equal(toEastMoneyCode('000001'), '000001.SZ')
  assert.equal(toEastMoneyCode('600519'), '600519.SH')
  assert.equal(toEastMoneyCode('920000'), '920000.BJ')
  assert.equal(toEastMoneyCode('300059'), '300059.SZ')
  assert.equal(toEastMoneyCode('159915'), '159915.SZ')
})

test('toEastMoneyCode: 标准后缀/前缀直接规范化', () => {
  assert.equal(toEastMoneyCode('000001.SZ'), '000001.SZ')
  assert.equal(toEastMoneyCode('600519.sh'), '600519.SH')
  assert.equal(toEastMoneyCode('sz000001'), '000001.SZ')
  assert.equal(toEastMoneyCode('AAPL'), 'AAPL.US')
  assert.equal(toEastMoneyCode(''), '')
})

test('toEastMoneyCode: “名字 - 代码”取后半部分', () => {
  assert.equal(toEastMoneyCode('安徽凤凰 - 920000.BJ'), '920000.BJ')
  assert.equal(toEastMoneyCode('平安银行 000001'), '000001.SZ')
})

test('stripNameCodeFormat: 剥掉“名字 (代码)”', () => {
  assert.equal(stripNameCodeFormat('平安银行 (000001.SZ)'), '000001.SZ')
  assert.equal(stripNameCodeFormat('贵州茅台 (600519.SH)'), '600519.SH')
  assert.equal(stripNameCodeFormat('000001'), '000001')
  assert.equal(stripNameCodeFormat('安徽凤凰 - 920000.BJ'), '安徽凤凰 - 920000.BJ')
})

test('filterStockOptions: 输入 000001 只命中平安银行，不含安徽凤凰 920000.BJ', () => {
  const got = filterStockOptions(LIST, '000001')
  assert.deepEqual(got.map((s) => s.ts_code), ['000001.SZ'])
})

test('filterStockOptions: 名称/拼音/代码片段匹配与上限', () => {
  assert.deepEqual(filterStockOptions(LIST, '凤凰').map((s) => s.ts_code), ['920000.BJ'])
  assert.deepEqual(filterStockOptions(LIST, 'ahfh').map((s) => s.ts_code), ['920000.BJ'])
  assert.deepEqual(filterStockOptions(LIST, '600').map((s) => s.ts_code), ['600519.SH', '600000.SH'])
  assert.equal(filterStockOptions(LIST, '600', 1).length, 1)
  assert.deepEqual(filterStockOptions(LIST, '  '), [])
  assert.deepEqual(filterStockOptions(LIST, '不存在'), [])
})

test('resolveSearchTarget: 未方向键选中时原文胜出（问题 2 回归）', () => {
  assert.equal(resolveSearchTarget('000001', '920000.BJ', false), '000001')
  assert.equal(resolveSearchTarget('000001', '', false), '000001')
})

test('resolveSearchTarget: 方向键选中后回车按建议走', () => {
  assert.equal(resolveSearchTarget('000001', '920000.BJ', true), '920000.BJ')
  assert.equal(resolveSearchTarget('000001', '', true), '000001')
})

test('联动：原文优先时 000001 解析为 000001.SZ 而非建议的 920000.BJ', () => {
  assert.equal(toEastMoneyCode(resolveSearchTarget('000001', '920000.BJ', false)), '000001.SZ')
  assert.equal(toEastMoneyCode(resolveSearchTarget('000001', '920000.BJ', true)), '920000.BJ')
  assert.equal(
    toEastMoneyCode(stripNameCodeFormat('平安银行 (000001.SZ)')),
    '000001.SZ',
  )
})
