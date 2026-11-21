// K 线时间纯函数测试（frontend/src/components/kline/time.ts）。
// 此前 time.ts 因无扩展名 import './constants' 无法被 node:test 导入而零测试；
// import 改为 './constants.ts' 后可测。本文件覆盖时间解析与 K 线行合并——
// mergeKlineRows 是乱码代码/重复行进入图表前的另一道去重防线。
// 运行：npm test（node --test tests/*.test.mjs）。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  eastMoneyDayToUnixSeconds,
  eastMoneyKlineFieldToUnixSeconds,
  sortKey,
  toChartTime,
  mergeKlineRows,
  mergeRefreshWithLatest,
  extractYmdDatePart,
} from '../src/components/kline/time.ts'

const noonSec = (ymd) => Math.floor(Date.parse(`${ymd}T12:00:00+08:00`) / 1000)

test('eastMoneyDayToUnixSeconds: 纯日期与空串返回 null，带时间按 +08:00 解析', () => {
  assert.equal(eastMoneyDayToUnixSeconds(''), null)
  assert.equal(eastMoneyDayToUnixSeconds('2026-11-20'), null)
  assert.equal(
    eastMoneyDayToUnixSeconds('2026-11-20 09:30:00'),
    Math.floor(Date.parse('2026-11-20T09:30:00+08:00') / 1000),
  )
  assert.equal(
    eastMoneyDayToUnixSeconds('2026-11-20T09:30:00Z'),
    Math.floor(Date.parse('2026-11-20T09:30:00Z') / 1000),
  )
  assert.equal(eastMoneyDayToUnixSeconds('garbage'), null)
})

test('eastMoneyKlineFieldToUnixSeconds: 纯日期取正午，14/12 位数字串解析为当时时间', () => {
  assert.equal(eastMoneyKlineFieldToUnixSeconds('2026-11-20'), noonSec('2026-11-20'))
  assert.equal(
    eastMoneyKlineFieldToUnixSeconds('20261120153000'),
    Math.floor(Date.parse('2026-11-20T15:30:00+08:00') / 1000),
  )
  assert.equal(
    eastMoneyKlineFieldToUnixSeconds('202611201530'),
    Math.floor(Date.parse('2026-11-20T15:30:00+08:00') / 1000),
  )
  assert.equal(eastMoneyKlineFieldToUnixSeconds(''), null)
  assert.equal(eastMoneyKlineFieldToUnixSeconds('garbage'), null)
})

test('sortKey: 日期取正午 +08:00 的毫秒（经 eastMoneyKlineFieldToUnixSeconds），14 位串取当时毫秒，非法取 0', () => {
  assert.equal(sortKey('2026-11-20'), noonSec('2026-11-20') * 1000)
  assert.equal(
    sortKey('20261120153000'),
    Math.floor(Date.parse('2026-11-20T15:30:00+08:00') / 1000) * 1000,
  )
  assert.equal(sortKey('garbage'), 0)
})

test('toChartTime: 纯日期取正午秒，空串 null', () => {
  assert.equal(toChartTime('2026-11-20'), noonSec('2026-11-20'))
  assert.equal(toChartTime('2026-11-20', true), noonSec('2026-11-20'))
  assert.equal(toChartTime(''), null)
})

test('mergeKlineRows: 按 day 去重（旧行优先），按时间升序，无 day 的行丢弃', () => {
  const existing = [
    { day: '2026-11-20', close: '1' },
    { day: '2026-11-19', close: '0' },
  ]
  const incoming = [
    { day: '2026-11-20', close: 'CHANGED' },
    { day: '2026-11-21', close: '2' },
    { close: 'no-day' },
  ]
  const merged = mergeKlineRows(existing, incoming)
  assert.deepEqual(
    merged.map((r) => r.day),
    ['2026-11-19', '2026-11-20', '2026-11-21'],
  )
  assert.equal(merged.find((r) => r.day === '2026-11-20').close, '1')
})

test('mergeRefreshWithLatest: 最新块替换重叠尾部，空块原样返回', () => {
  const existing = [
    { day: '2026-11-18', close: 'a' },
    { day: '2026-11-19', close: 'b' },
    { day: '2026-11-20', close: 'c' },
  ]
  const latest = [
    { day: '2026-11-20', close: 'C2' },
    { day: '2026-11-21', close: 'd' },
  ]
  const merged = mergeRefreshWithLatest(existing, latest)
  assert.deepEqual(
    merged.map((r) => r.day),
    ['2026-11-18', '2026-11-19', '2026-11-20', '2026-11-21'],
  )
  assert.equal(merged.find((r) => r.day === '2026-11-20').close, 'C2')
  assert.deepEqual(mergeRefreshWithLatest(existing, []), existing)
})

test('extractYmdDatePart: 取日期部分，8 位数字串转横线格式', () => {
  assert.equal(extractYmdDatePart('2026-11-20 09:30:00'), '2026-11-20')
  assert.equal(extractYmdDatePart('20261120'), '2026-11-20')
  assert.equal(extractYmdDatePart('garbage'), '')
})
