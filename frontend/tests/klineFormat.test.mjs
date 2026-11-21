// K 线数字格式化纯函数测试（frontend/src/components/kline/format.ts）。
// format.ts 此前零测试；这些函数直接决定 K 线页价格/成交量/涨跌幅的
// 显示值，错一位就是误导性行情数字。
// 运行：npm test（node --test tests/*.test.mjs）。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  parseNumStr,
  formatPrice2,
  formatVolumeCn,
  formatAmountCn,
  formatPctField,
  formatSigned2,
} from '../src/components/kline/format.ts'

test('parseNumStr: 去逗号/百分号/空白；非法返回 NaN', () => {
  assert.equal(parseNumStr('1,234.56'), 1234.56)
  assert.equal(parseNumStr('2.30%'), 2.3)
  assert.equal(parseNumStr(' 12.22 '), 12.22)
  assert.equal(parseNumStr(''), 0) // Number('') === 0
  assert.ok(Number.isNaN(parseNumStr('-')))
  assert.ok(Number.isNaN(parseNumStr('abc')))
  // null/undefined 经 ?? '' 后同样落到 Number('') === 0（当前行为钉住）。
  assert.equal(parseNumStr(null), 0)
  assert.equal(parseNumStr(undefined), 0)
})

test('formatPrice2: 保留两位小数；非法显示 --', () => {
  assert.equal(formatPrice2('12.22'), '12.22')
  assert.equal(formatPrice2('12.2'), '12.20')
  assert.equal(formatPrice2('1,485.75'), '1485.75')
  assert.equal(formatPrice2('-'), '--')
  assert.equal(formatPrice2(''), '0.00') // 与 parseNumStr('') === 0 一致
})

test('formatVolumeCn: 亿 / 万分界与取整', () => {
  assert.equal(formatVolumeCn('123456789'), '1.23亿')
  assert.equal(formatVolumeCn('100000000'), '1.00亿')
  assert.equal(formatVolumeCn('123456'), '12.35万')
  assert.equal(formatVolumeCn('10000'), '1.00万')
  assert.equal(formatVolumeCn('9999'), '9999')
  assert.equal(formatVolumeCn('0'), '0')
  assert.equal(formatVolumeCn('-'), '--')
})

test('formatAmountCn: 一律按亿显示', () => {
  assert.equal(formatAmountCn('150000000'), '1.50亿')
  assert.equal(formatAmountCn('123456'), '0.00亿')
  assert.equal(formatAmountCn('-'), '--')
})

test('formatPctField / formatSigned2: 百分号与正负号', () => {
  assert.equal(formatPctField('2.3'), '2.30%')
  assert.equal(formatPctField('-1.5'), '-1.50%')
  assert.equal(formatPctField('-'), '--')
  assert.equal(formatSigned2('2.3'), '+2.30')
  assert.equal(formatSigned2('-1.5'), '-1.50')
  assert.equal(formatSigned2('0'), '0.00')
  assert.equal(formatSigned2('-'), '--')
})
