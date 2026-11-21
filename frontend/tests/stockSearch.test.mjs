// 搜索框问题 2 回归测试：原文解析优先于联想建议。
// 问题 A 回归测试：拼音/名称输入且联想唯一时，回车优先用建议（gzmt → 600519.SH）。
// 尾巴回归测试：代次守卫（超时横幅竞态）、列表晚到等 gzmt、方向键快照防悬停误选。
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
  isValidEastMoneyCode,
  createGenerationGuard,
  shouldWaitForStockList,
  parsePendingOptionCode,
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

test('isValidEastMoneyCode: 只认规范定长代码', () => {
  for (const c of ['600519.SH', '000001.SZ', '920000.BJ', '159915.SZ', '00700.HK', 'AAPL.US', '600519.sh']) {
    assert.equal(isValidEastMoneyCode(c), true, c)
  }
})

test('isValidEastMoneyCode: 拒绝乱码与短码（600500000119.SH 回归）', () => {
  for (const c of ['', '600500000119.SH', '6005000001.SH', '6005.SH', '600519', '600519.XX', '600519.SH6005', 'ABCDEF.US', '1234567.SH']) {
    assert.equal(isValidEastMoneyCode(c), false, c)
  }
})

test('联动：toEastMoneyCode 会放过乱码，isValidEastMoneyCode 拦下', () => {
  // 框内 "(600519.SH)" 中间插入 "000001" 得到的拼接串，解析层照常放行——
  // 这正是本次乱码能一路写进历史、刷新后又被恢复的缺口。
  const garbled = toEastMoneyCode(stripNameCodeFormat('贵州茅台 (600500000119.SH)'))
  assert.equal(garbled, '600500000119.SH')
  assert.equal(isValidEastMoneyCode(garbled), false)
})

// ---- 尾巴 1：代次守卫（超时横幅竞态）----

function deferred() {
  let resolve, reject
  const promise = new Promise((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

test('createGenerationGuard: 只有最新代次可写，invalidate 后全失效', () => {
  const guard = createGenerationGuard()
  const g1 = guard.next()
  assert.equal(guard.isCurrent(g1), true)
  assert.equal(guard.current, g1)
  const g2 = guard.next()
  assert.equal(guard.isCurrent(g1), false)
  assert.equal(guard.isCurrent(g2), true)
  guard.invalidate()
  assert.equal(guard.isCurrent(g2), false)
})

test('代次守卫模拟 loadData 交错：旧请求晚失败，不覆盖新成功的状态', async () => {
  // 按组件里的写法模拟：旧 loadData 先发起，新 loadData 后发起并成功，
  // 旧请求此时才失败——其 catch/finally 不得写 errorText、不碰 loading。
  const guard = createGenerationGuard()
  let errorText = ''
  let loading = false
  const oldReq = deferred()
  const oldGen = guard.next()
  loading = true
  errorText = ''
  const oldTask = (async () => {
    try {
      await oldReq.promise
    } catch (e) {
      if (!guard.isCurrent(oldGen)) return
      errorText = String(e?.message || e)
    } finally {
      if (guard.isCurrent(oldGen)) loading = false
    }
  })()
  const newGen = guard.next()
  loading = true
  errorText = ''
  const newTask = (async () => {
    try {
      await Promise.resolve('ok')
      errorText = ''
    } finally {
      if (guard.isCurrent(newGen)) loading = false
    }
  })()
  await newTask
  assert.equal(errorText, '')
  assert.equal(loading, false)
  oldReq.reject(new Error('timeout'))
  await oldTask
  assert.equal(errorText, '')
  assert.equal(loading, false)
})

test('代次守卫模拟 loadData 交错：旧失败发生在新请求发起后，不得写横幅', async () => {
  const guard = createGenerationGuard()
  let errorText = ''
  const oldReq = deferred()
  const oldGen = guard.next()
  const oldTask = (async () => {
    try {
      await oldReq.promise
    } catch (e) {
      if (!guard.isCurrent(oldGen)) return
      errorText = String(e?.message || e)
    }
  })()
  guard.next() // 新 loadData 发起，开头已清 errorText
  errorText = ''
  oldReq.reject(new Error('timeout'))
  await oldTask
  assert.equal(errorText, '')
})

test('代次守卫模拟轮询：loadData 发起后，旧轮询回包丢弃', async () => {
  // 轮询只读 loadGuard.current 快照，不推进它；新 loadData 发起即失效。
  const loadGuard = createGenerationGuard()
  const pollGuard = createGenerationGuard()
  let wrote = false
  const pollReq = deferred()
  const loadGenAtStart = loadGuard.current
  const myPollGen = pollGuard.next()
  const pollTask = (async () => {
    await pollReq.promise
    if (!pollGuard.isCurrent(myPollGen)) return
    if (!loadGuard.isCurrent(loadGenAtStart)) return
    wrote = true
  })()
  loadGuard.next() // 新 loadData 发起
  pollReq.resolve('rows')
  await pollTask
  assert.equal(wrote, false)
})

test('代次守卫模拟轮询：无新 loadData 时轮询回包正常写入', async () => {
  const loadGuard = createGenerationGuard()
  const pollGuard = createGenerationGuard()
  let wrote = false
  const pollReq = deferred()
  const loadGenAtStart = loadGuard.current
  const myPollGen = pollGuard.next()
  const pollTask = (async () => {
    await pollReq.promise
    if (!pollGuard.isCurrent(myPollGen)) return
    if (!loadGuard.isCurrent(loadGenAtStart)) return
    wrote = true
  })()
  pollReq.resolve('rows')
  await pollTask
  assert.equal(wrote, true)
})

test('代次守卫模拟历史分页：loadData 的 invalidate 让在途历史分页失效', async () => {
  const historyGuard = createGenerationGuard()
  let merged = false
  const histReq = deferred()
  const myGen = historyGuard.next()
  const histTask = (async () => {
    await histReq.promise
    if (!historyGuard.isCurrent(myGen)) return
    merged = true
  })()
  historyGuard.invalidate() // loadData 发起（清空了 mergedRawRows）
  histReq.resolve('rows')
  await histTask
  assert.equal(merged, false)
})

// ---- 尾巴 2：列表晚到时等 gzmt ----

test('shouldWaitForStockList: 只在列表未到、非代码原文、联想为空时等', () => {
  assert.equal(shouldWaitForStockList('gzmt', [], false), true)
  assert.equal(shouldWaitForStockList('贵州茅台', [], false), true)
  assert.equal(shouldWaitForStockList('AAPL', [], false), true)
  assert.equal(shouldWaitForStockList('gzmt', [], true), false)
  assert.equal(shouldWaitForStockList('000001', [], false), false)
  assert.equal(shouldWaitForStockList('600519.SH', [], false), false)
  assert.equal(shouldWaitForStockList('gzmt', ['600519.SH'], false), false)
})

test('列表晚到：gzmt 在全量列表里唯一命中，回车用建议而非 GZMT.US', () => {
  // 模拟 handleSearch 等列表加载完后重算联想：本地过滤唯一命中 → resolve 用建议。
  const fresh = filterStockOptions(LIST, 'gzmt', 30).map((s) => s.ts_code)
  assert.deepEqual(fresh, ['600519.SH'])
  assert.equal(toEastMoneyCode(resolveSearchTarget('gzmt', '', false, fresh)), '600519.SH')
  // 对照：联想为空时现状仍是 GZMT.US（列表超时/加载失败的回退行为）。
  assert.equal(toEastMoneyCode(resolveSearchTarget('gzmt', '', false, [])), 'GZMT.US')
})

// ---- 尾巴 3：方向键快照防悬停误选 ----

test('parsePendingOptionCode: 从 pending label 解析代码值，且须在联想列表里', () => {
  const values = ['600519.SH', '000001.SZ']
  assert.equal(parsePendingOptionCode('贵州茅台 - 600519.SH', values), '600519.SH')
  assert.equal(parsePendingOptionCode('平安银行 - 000001.SZ', values), '000001.SZ')
  assert.equal(parsePendingOptionCode('贵州茅台 - 600519.SH', ['000001.SZ']), '')
  assert.equal(parsePendingOptionCode('', values), '')
  assert.equal(parsePendingOptionCode('600519.SH', values), '600519.SH')
  assert.equal(parsePendingOptionCode(' - 600519.SH', values), '600519.SH')
  assert.equal(parsePendingOptionCode('贵州茅台 - 600519.SH ', values), '600519.SH')
})

test('方向键快照：解析出的值在联想列表里时，回车按它走；读不到时回退', () => {
  const code = parsePendingOptionCode('贵州茅台 - 600519.SH', ['600519.SH', '000001.SZ'])
  assert.equal(code, '600519.SH')
  assert.equal(resolveSearchTarget('gzmt', code, true), '600519.SH')
  // 快照读不到（''）且无方向键选中时，回车按原文/唯一建议走（旧行为）。
  assert.equal(resolveSearchTarget('gzmt', '', false, ['600519.SH']), '600519.SH')
  assert.equal(resolveSearchTarget('gzmt', '', false, []), 'gzmt')
})
