// 筹码分布纯函数测试（frontend/src/components/kline/chip.ts）。
// chip.ts 此前零测试；chipBarCostCenter 决定每根 K 线的筹码落点，
// calcChipDistribution 决定整段筹码的平均成本与获利比例——
// 算错直接误导“支撑/压力位”判断。
// 运行：npm test（node --test tests/*.test.mjs）。
import test from 'node:test'
import assert from 'node:assert/strict'
import {
  chipBarCostCenter,
  addChipVolumeKernel,
  calcChipDistribution,
} from '../src/components/kline/chip.ts'

test('chipBarCostCenter: 优先用成交均价（额/量），再退化到典型价', () => {
  // 额/量 = 9.5，在 [9, 10] 内，直接取用。
  assert.equal(
    chipBarCostCenter({ high: 10, low: 9, close: 9.5, open: 9.2, volume: 1000, amount: 9500 }),
    9.5,
  )
  // 无有效量额时退化到 (高+低+收)/3。
  assert.equal(
    chipBarCostCenter({ high: 10, low: 8, close: 9, open: 9, volume: 0, amount: 0 }),
    9,
  )
  // 成交均价超出区间时钳制到区间内。
  assert.equal(
    chipBarCostCenter({ high: 10, low: 9, close: 9.5, open: 9.2, volume: 100, amount: 2000 }),
    10,
  )
  // 高 < 低的脏数据返回 null，不参与分布。
  assert.equal(
    chipBarCostCenter({ high: 8, low: 10, close: 9, open: 9, volume: 100, amount: 900 }),
    null,
  )
  assert.equal(
    chipBarCostCenter({ high: 0, low: 0, close: 0, open: 0, volume: 100, amount: 900 }),
    null,
  )
})

test('addChipVolumeKernel: 非正量 / 非正价不写分布；总量守恒', () => {
  const dist = new Float64Array(10)
  addChipVolumeKernel(dist, 10, 9, 0.2, 10, 10, 100, 10) // 零跨度：整量落单 bin
  const sum = [...dist].reduce((a, b) => a + b, 0)
  assert.equal(sum, 100)
  // 非正量不写。
  const d2 = new Float64Array(10)
  addChipVolumeKernel(d2, 10, 9, 0.2, 9, 11, 0, 10)
  assert.equal([...d2].reduce((a, b) => a + b, 0), 0)
  // 宽区间核分布：总量仍守恒，且中心 bin 高于边缘 bin。
  const d3 = new Float64Array(10)
  addChipVolumeKernel(d3, 10, 9, 0.2, 9, 11, 100, 10)
  const s3 = [...d3].reduce((a, b) => a + b, 0)
  assert.ok(Math.abs(s3 - 100) < 1e-9)
  assert.ok(d3[5] >= d3[0] && d3[5] >= d3[9])
})

test('calcChipDistribution: 空输入与脏输入返回空分布；单价行总量守恒', () => {
  assert.deepEqual(calcChipDistribution([], 10), { items: [], avgCost: 0, profitRatio: 0, current: 0 })
  assert.deepEqual(calcChipDistribution(null, 10), { items: [], avgCost: 0, profitRatio: 0, current: 0 })
  const single = calcChipDistribution(
    [{ high: 10, low: 10, close: 10, open: 10, volume: 100, amount: 1000, turnoverRate: '0' }],
    10,
  )
  assert.equal(single.items.length, 10)
  const volSum = single.items.reduce((a, it) => a + it.vol, 0)
  assert.ok(Math.abs(volSum - 100) < 0.01)
  assert.ok(Math.abs(single.avgCost - 10) < 0.05)
  assert.ok(single.current === 10)
  // 获利比例在 [0,1] 内。
  assert.ok(single.profitRatio >= 0 && single.profitRatio <= 1)
})

test('calcChipDistribution: 换手率衰减——老筹码随换手逐步稀释', () => {
  const rows = [
    { high: 10, low: 9, close: 9.5, open: 9.2, volume: 100, amount: 950, turnoverRate: '0' },
    { high: 10, low: 9, close: 9.5, open: 9.2, volume: 100, amount: 950, turnoverRate: '50' }, // 50% 换手
  ]
  const dist = calcChipDistribution(rows, 10)
  const volSum = dist.items.reduce((a, it) => a + it.vol, 0)
  // 第二根先把旧分布 ×(1-0.5) 再加 100：总量 ≈ 150。
  assert.ok(Math.abs(volSum - 150) < 0.01)
})
