// K 线技术指标纯函数测试（frontend/src/components/kline/calc.ts）。
// calc.ts 此前零测试，却是 K 线图全部指标的唯一计算源：SMA/EMA/BOLL/
// RSI/MACD/KDJ/OBV 一旦算错，全图跟着错，且没有任何后端能兜底。
// 只测数值语义明确的函数；时间类函数（time.ts）因无扩展名 import
// 在 node --test 下导不进来，见报告，不在此硬测。
// 运行：npm test（node --test tests/*.test.mjs）。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  smaValues,
  emaFinite,
  emaLeadingNull,
  weightedMaValues,
  bollingerBands,
  obvValues,
  rsiBundle,
  macdBundle,
  kdjBundle,
} from '../src/components/kline/calc.ts'

const approx = (a, b, eps = 1e-9) => Math.abs(a - b) <= eps

test('smaValues: 周期前补 null，窗口滑动求均值', () => {
  assert.deepEqual(smaValues([1, 2, 3, 4, 5], 3), [null, null, 2, 3, 4])
  assert.deepEqual(smaValues([10, 10, 10], 1), [10, 10, 10])
  assert.deepEqual(smaValues([1, 2], 5), [null, null])
  assert.deepEqual(smaValues([], 5), [])
})

test('emaFinite: 前 period-1 个为 null，种子为首个窗口 SMA，之后按 k=2/(n+1) 递推', () => {
  assert.deepEqual(emaFinite([1, 2, 3, 4, 5], 3), [null, null, 2, 3, 4])
  // 含 NaN 的窗口播不了种：含 NaN 的窗口及之后都保持 null（不让脏数据污染 EMA）。
  assert.deepEqual(emaFinite([1, 2, Number.NaN, 4, 5], 3), [null, null, null, null, null])
})

test('emaLeadingNull: 攒够 period 个非空值才出第一个数', () => {
  assert.deepEqual(emaLeadingNull([null, 1, 2, 3, 4], 3), [null, null, null, 2, 3])
  assert.deepEqual(emaLeadingNull([null, null], 3), [null, null])
})

test('weightedMaValues: 近端权重更大（线性加权）', () => {
  const got = weightedMaValues([1, 2, 3, 4], 3)
  assert.equal(got[0], null)
  assert.equal(got[1], null)
  assert.ok(approx(got[2], 14 / 6)) // (1*1+2*2+3*3)/6
  assert.ok(approx(got[3], 20 / 6)) // (2*1+3*2+4*3)/6
})

test('bollingerBands: 平坦序列上下轨重合于中轨；用样本标准差（÷ n-1）', () => {
  const flat = bollingerBands([10, 10, 10, 10, 10], 5, 2)
  assert.deepEqual(flat.mid, [null, null, null, null, 10])
  assert.equal(flat.upper[4], 10)
  assert.equal(flat.lower[4], 10)
  const b = bollingerBands([1, 2, 3, 4, 5], 5, 2)
  const std = Math.sqrt(10 / 4) // 样本标准差
  assert.ok(approx(b.mid[4], 3))
  assert.ok(approx(b.upper[4], 3 + 2 * std))
  assert.ok(approx(b.lower[4], 3 - 2 * std))
  assert.equal(b.upper[0], null)
})

test('obvValues: 涨加量、跌减量、平不变', () => {
  assert.deepEqual(obvValues([10, 11, 10, 10], [100, 200, 300, 400]), [100, 300, 0, 0])
  assert.deepEqual(obvValues([], []), [])
})

test('rsiBundle: 平坦序列返回 50（中性）而非 100；全涨返回 100', () => {
  const flat = rsiBundle(new Array(16).fill(10), 14)
  assert.equal(flat[14], 50)
  assert.equal(flat[15], 50)
  assert.equal(flat[0], null)
  const up = rsiBundle(Array.from({ length: 16 }, (_, i) => i + 1), 14)
  assert.equal(up[14], 100)
  // 数据不足一个周期时全 null，不硬算。
  assert.deepEqual(rsiBundle([1, 2, 3], 14), [null, null, null])
})

test('macdBundle: DIF 在 EMA26 播种前全 null；DEA 攒够 9 个 DIF 后才出数', () => {
  const closes = Array.from({ length: 40 }, (_, i) => i + 1)
  const { dif, dea, hist } = macdBundle(closes)
  assert.equal(dif.length, 40)
  assert.ok(dif.slice(0, 25).every((v) => v === null))
  assert.ok(Number.isFinite(dif[25]))
  assert.equal(dea[32], null)
  assert.ok(Number.isFinite(dea[33]))
  assert.ok(Number.isFinite(hist[33]))
  assert.equal(hist[0], null)
})

test('kdjBundle: 高低相等（停牌式平线）时 RSV 取 50，K/D/J 恒为 50', () => {
  const flat = Array(12).fill(10)
  const { K, D, J } = kdjBundle(flat, flat, flat, 9)
  assert.ok(K.slice(0, 8).every((v) => v === null))
  assert.equal(K[11], 50)
  assert.equal(D[11], 50)
  assert.equal(J[11], 50)
})
