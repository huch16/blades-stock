// 搜索框问题 2 回归测试：原文解析优先于联想建议。
// 问题 A 回归测试：拼音/名称输入且联想唯一时，回车优先用建议（gzmt → 600519.SH）。
// 运行：npm test（或 node --test tests/）。Node 24 类型剥离直接导入
// ../src/utils/stockSearch.ts，无需 vitest。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  toEastMoneyCode,
  stripNameCodeFormat,
  filterStockOptions,
  isCodeText,
  resolveSearchTarget,
} from '../src/utils/stockSearch.ts'

const LIST = [
  { name: '平安银行', ts_code: '000001.SZ', symbol: '000001', cnspell: 'payh' },
  { name: '贵州茅台', ts_code: '600519.SH', symbol: '600519', cnspell: 'gzmt' },
  { name: '中国平安', ts_code: '601318.SH', symbol: '601318', cnspell: 'zgpa' },
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

test('isCodeText: 纯数字/后缀/前缀+数字是代码原文，纯字母与中文名不是', () => {
  assert.equal(isCodeText('000001'), true)
  assert.equal(isCodeText('600519'), true)
  assert.equal(isCodeText('600519.SH'), true)
  assert.equal(isCodeText('600519.sh'), true)
  assert.equal(isCodeText('sz000001'), true)
  assert.equal(isCodeText('sh600519'), true)
  assert.equal(isCodeText('平安银行 (000001.SZ)'), true)
  assert.equal(isCodeText('安徽凤凰 - 920000.BJ'), true)
  assert.equal(isCodeText('gzmt'), false)
  assert.equal(isCodeText('AAPL'), false)
  assert.equal(isCodeText('shyh'), false)
  assert.equal(isCodeText('贵州茅台'), false)
  assert.equal(isCodeText(''), false)
})

test('filterStockOptions: 拼音 gzmt / zgpa 各自唯一命中', () => {
  assert.deepEqual(filterStockOptions(LIST, 'gzmt').map((s) => s.ts_code), ['600519.SH'])
  assert.deepEqual(filterStockOptions(LIST, 'zgpa').map((s) => s.ts_code), ['601318.SH'])
})

test('resolveSearchTarget: 拼音输入且唯一建议时用建议（问题 A 回归）', () => {
  assert.equal(resolveSearchTarget('gzmt', '', false, ['600519.SH']), '600519.SH')
  assert.equal(resolveSearchTarget('zgpa', '', false, ['601318.SH']), '601318.SH')
  assert.equal(resolveSearchTarget('贵州茅台', '', false, ['600519.SH']), '600519.SH')
})

test('resolveSearchTarget: 多条/无建议时仍按原文解析', () => {
  assert.equal(resolveSearchTarget('600', '', false, ['600519.SH', '600000.SH']), '600')
  assert.equal(resolveSearchTarget('AAPL', '', false, []), 'AAPL')
  assert.equal(resolveSearchTarget('gzmt', '', false, []), 'gzmt')
})

test('resolveSearchTarget: 合法代码原文不受建议影响（问题 2 回归）', () => {
  assert.equal(resolveSearchTarget('000001', '', false, ['000001.SZ']), '000001')
  assert.equal(resolveSearchTarget('600519', '', false, ['600519.SH']), '600519')
  assert.equal(resolveSearchTarget('600519.SH', '', false, ['600519.SH']), '600519.SH')
})

test('联动：gzmt 唯一建议时解析为 600519.SH；无建议时 AAPL 仍为 AAPL.US', () => {
  assert.equal(toEastMoneyCode(resolveSearchTarget('gzmt', '', false, ['600519.SH'])), '600519.SH')
  assert.equal(toEastMoneyCode(resolveSearchTarget('zgpa', '', false, ['601318.SH'])), '601318.SH')
  assert.equal(toEastMoneyCode(resolveSearchTarget('AAPL', '', false, [])), 'AAPL.US')
  assert.equal(toEastMoneyCode(resolveSearchTarget('000001', '', false, ['000001.SZ'])), '000001.SZ')
  assert.equal(toEastMoneyCode(resolveSearchTarget('贵州茅台', '', false, ['600519.SH'])), '600519.SH')
})
