// 多单标注：开仓/止损/止盈/成本四条价位线、图上点击取价与拖动改价、输入框双向同步。
// 由 StockLightweightKlineChart.vue 抽出（R-22a），逻辑逐字搬运，仅把闭包读取的
// chart / candleSeries 改为 ctx 传入的 getter；props / emit / chartContainerRef /
// defaultLatestRawRow 以原样传入。watchers 随功能一起搬入本文件。
import { computed, nextTick, ref, watch } from 'vue'
import { LineStyle } from 'lightweight-charts'
import { parseNumStr } from '../components/kline/format'

export function useKlineLongPosition(ctx) {
  const getChart = ctx.getChart
  const getCandleSeries = ctx.getCandleSeries
  const chartContainerRef = ctx.chartContainerRef
  const defaultLatestRawRow = ctx.defaultLatestRawRow
  const props = ctx.props
  const emit = ctx.emit

  function resetLongDragState() {
    longFocusedPriceField.value = null
    longPositionDragActive = false
    longDragKind = null
    longSuppressChartClick = false
  }

  // 图表点击处理器读取：拖动价位线后抑制一次图上点击设价
  function isChartClickSuppressed() {
    return longSuppressChartClick
  }

const showLongPosition = ref(false)
const longEntryStr = ref('')
const longStopStr = ref('')
const longTakeProfitStr = ref('')
const longCostStr = ref('')
/** 在 K 线主区点击，按顺序写入开仓 → 止损 → 止盈（再点回到开仓） */
const longClickPickEnabled = ref(false)
const longClickNextField = ref('entry')
/** 点过开仓/止损/止盈输入框后，下一次主图点击写入对应价位（blur 延迟清除以兼容「先失焦后 click」） */
const longFocusedPriceField = ref(null)
/** 由 props 写入价位时抑制 emit，避免与 v-model 循环 */
const suppressLongPriceEmit = ref(false)
let longPositionPriceLines = []
/** 各价位线句柄，用于命中与拖动时 applyOptions */
let longLineByKind = { entry: null, stop: null, takeProfit: null }
/** 正在拖动某条多单价位线时禁止 watch 里整表重建线条 */
let longPositionDragActive = false
let longDragKind = null
/** 命中线后抑制一次「图上点击设价」，避免拖/点冲突 */
let longSuppressChartClick = false
let longPaneDragEl = null
/** window 上拖动用监听器是否已挂（避免重复解绑 / 重复 up） */
let longDragWindowListenersOn = false
/** 拖动过程中最近一次指针 Y，用于松手后恢复「可拖」光标 */
let longLastPointerClientY = null
/** 垂直方向命中容差（px） */
const LONG_PRICE_LINE_HIT_PX = 12
/** 输入框 blur 后延迟清除「待图表取价」状态（ms），需大于 click 相对 blur 的间隔 */
const LONG_FOCUS_BLUR_CLEAR_MS = 600
let longFocusBlurTimer = null

/** 指标线系列（与主图生命周期同步） */
function clearLongPositionPriceLines() {
  longLineByKind = { entry: null, stop: null, takeProfit: null }
  if (!getCandleSeries()) {
    longPositionPriceLines = []
    return
  }
  for (const pl of longPositionPriceLines) {
    try {
      getCandleSeries().removePriceLine(pl)
    } catch {
      /* ignore */
    }
  }
  longPositionPriceLines = []
}

function syncLongPositionPriceLines() {
  clearLongPositionPriceLines()
  if (!showLongPosition.value || !getCandleSeries()) {
    return
  }
  const entry = parseNumStr(longEntryStr.value)
  const stop = parseNumStr(longStopStr.value)
  const tp = parseNumStr(longTakeProfitStr.value)
  const cost = parseNumStr(longCostStr.value)

  if (!Number.isFinite(entry) && !Number.isFinite(cost)) {
    return
  }

  const pushLine = (price, kind, partial) => {
    const pl = getCandleSeries().createPriceLine({
      price,
      lineWidth: 2,
      axisLabelVisible: true,
      ...partial,
    })
    longPositionPriceLines.push(pl)
    longLineByKind[kind] = pl
  }
  if (Number.isFinite(entry)) {
    pushLine(entry, 'entry', {
      color: '#3b82f6',
      lineStyle: LineStyle.Solid,
      title: '开仓',
    })
  }
  if (Number.isFinite(cost)) {
    pushLine(cost, 'cost', {
      color: '#f59e0b',
      lineStyle: LineStyle.Dashed,
      title: '成本',
    })
  }
  if (Number.isFinite(stop)) {
    pushLine(stop, 'stop', {
      color: CLR_FALL,
      lineStyle: LineStyle.Dashed,
      title: '止损',
    })
  }
  if (Number.isFinite(tp)) {
    pushLine(tp, 'takeProfit', {
      color: CLR_RISE,
      lineStyle: LineStyle.Dashed,
      title: '止盈',
    })
  }
}

function getLongDragPaneElement() {
  if (!getChart()) return chartContainerRef.value
  try {
    return getChart().panes?.()?.[0]?.getHTMLElement() ?? chartContainerRef.value
  } catch {
    return chartContainerRef.value
  }
}

function longPaneLocalYFromClient(clientY) {
  const el = getLongDragPaneElement()
  if (!el) return null
  const r = el.getBoundingClientRect()
  const y = clientY - r.top
  return Number.isFinite(y) ? y : null
}

function refreshLongPriceLineCursorFromCrosshair(param) {
  const paneEl = getLongDragPaneElement()
  if (!paneEl) return
  if (longPositionDragActive) {
    paneEl.style.cursor = 'grabbing'
    return
  }
  if (
    !showLongPosition.value ||
    !longLineByKind.entry ||
    param.point === undefined ||
    (param.paneIndex != null && param.paneIndex !== 0)
  ) {
    paneEl.style.cursor = ''
    return
  }
  const kind = hitTestLongPriceLineKind(param.point.y)
  paneEl.style.cursor = kind ? 'grab' : ''
}

function clearLongPriceLinePaneCursor() {
  if (longPositionDragActive) return
  const paneEl = getLongDragPaneElement()
  if (paneEl) paneEl.style.cursor = ''
}

/** @returns {'entry'|'stop'|'takeProfit'|null} */
function hitTestLongPriceLineKind(localY) {
  if (!getCandleSeries() || !showLongPosition.value || localY == null) return null
  let best = null
  let bestDist = LONG_PRICE_LINE_HIT_PX + 1
  for (const kind of ['entry', 'stop', 'takeProfit']) {
    const line = longLineByKind[kind]
    if (!line) continue
    const p = Number(line.options().price)
    if (!Number.isFinite(p)) continue
    const ly = getCandleSeries().priceToCoordinate(p)
    if (ly == null) continue
    const cy = Number(ly)
    if (!Number.isFinite(cy)) continue
    const d = Math.abs(cy - localY)
    if (d <= LONG_PRICE_LINE_HIT_PX && d < bestDist) {
      bestDist = d
      best = kind
    }
  }
  return best
}

function detachLongDragWindowListeners() {
  if (!longDragWindowListenersOn) return
  longDragWindowListenersOn = false
  window.removeEventListener('pointermove', onLongDragWindowMove, true)
  window.removeEventListener('pointerup', onLongDragWindowUp, true)
  window.removeEventListener('pointercancel', onLongDragWindowUp, true)
}

function onLongDragWindowMove(e) {
  if (!longPositionDragActive || !longDragKind || !getCandleSeries()) return
  longLastPointerClientY = e.clientY
  const y = longPaneLocalYFromClient(e.clientY)
  if (y == null) return
  const raw = getCandleSeries().coordinateToPrice(y)
  const price = raw == null ? NaN : Number(raw)
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  const line = longLineByKind[longDragKind]
  if (line) {
    try {
      line.applyOptions({ price })
    } catch {
      /* ignore */
    }
  }
  if (longDragKind === 'entry') longEntryStr.value = s
  else if (longDragKind === 'stop') longStopStr.value = s
  else longTakeProfitStr.value = s
}

function onLongDragWindowUp() {
  if (!longDragWindowListenersOn) return
  const was = longPositionDragActive
  detachLongDragWindowListeners()
  longPositionDragActive = false
  longDragKind = null
  if (was) syncLongPositionPriceLines()
  const paneEl = getLongDragPaneElement()
  if (paneEl) {
    if (
      longLastPointerClientY != null &&
      showLongPosition.value &&
      longLineByKind.entry
    ) {
      const ly = longPaneLocalYFromClient(longLastPointerClientY)
      const kind = ly != null ? hitTestLongPriceLineKind(ly) : null
      paneEl.style.cursor = kind ? 'grab' : ''
    } else {
      paneEl.style.cursor = ''
    }
  }
  longLastPointerClientY = null
  setTimeout(() => {
    longSuppressChartClick = false
  }, 0)
}

function onLongPriceLinePointerDownCapture(e) {
  if (!showLongPosition.value || !getCandleSeries()) return
  if (e.pointerType === 'mouse' && e.button !== 0) return
  const y = longPaneLocalYFromClient(e.clientY)
  if (y == null) return
  const kind = hitTestLongPriceLineKind(y)
  if (!kind) return
  longLastPointerClientY = e.clientY
  longSuppressChartClick = true
  longPositionDragActive = true
  longDragKind = kind
  const paneElGrab = getLongDragPaneElement()
  if (paneElGrab) paneElGrab.style.cursor = 'grabbing'
  try {
    e.preventDefault()
  } catch {
    /* ignore */
  }
  e.stopPropagation()
  detachLongDragWindowListeners()
  longDragWindowListenersOn = true
  window.addEventListener('pointermove', onLongDragWindowMove, true)
  window.addEventListener('pointerup', onLongDragWindowUp, true)
  window.addEventListener('pointercancel', onLongDragWindowUp, true)
}

function attachLongPriceLineDragListeners() {
  detachLongPriceLinePaneListener()
  longPaneDragEl = getLongDragPaneElement()
  if (longPaneDragEl) {
    longPaneDragEl.addEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
  }
}

function detachLongPriceLinePaneListener() {
  if (longPaneDragEl) {
    longPaneDragEl.removeEventListener('pointerdown', onLongPriceLinePointerDownCapture, true)
    longPaneDragEl = null
  }
}

const longPositionStats = computed(() => {
  const entry = parseNumStr(longEntryStr.value)
  const stop = parseNumStr(longStopStr.value)
  const tp = parseNumStr(longTakeProfitStr.value)
  if (!Number.isFinite(entry)) return null
  const risk = Number.isFinite(stop) ? entry - stop : NaN
  const reward = Number.isFinite(tp) ? tp - entry : NaN
  const rr =
    Number.isFinite(risk) && risk > 0 && Number.isFinite(reward)
      ? reward / risk
      : NaN
  return {
    riskPts: risk,
    rewardPts: reward,
    riskRr: rr,
    riskPct: Number.isFinite(risk) && entry !== 0 ? (risk / entry) * 100 : NaN,
    rewardPct: Number.isFinite(reward) && entry !== 0 ? (reward / entry) * 100 : NaN,
  }
})

const longPositionHint = computed(() => {
  if (!showLongPosition.value) return ''
  if (!Number.isFinite(parseNumStr(longEntryStr.value))) {
    return '输入开仓价后显示线；止损低于开仓、止盈高于开仓为典型多单'
  }
  const s = longPositionStats.value
  if (!s) return ''
  const parts = []
  if (Number.isFinite(s.riskPts)) parts.push(`风险幅度 ${s.riskPts.toFixed(2)}`)
  if (Number.isFinite(s.rewardPts)) parts.push(`目标幅度 ${s.rewardPts.toFixed(2)}`)
  if (Number.isFinite(s.riskRr) && s.riskRr > 0) parts.push(`盈亏比 1 : ${s.riskRr.toFixed(2)}`)
  parts.push('单击线条可拖动改价')
  return parts.join(' · ')
})

function toggleLongPosition() {
  showLongPosition.value = !showLongPosition.value
  if (!showLongPosition.value) {
    longClickPickEnabled.value = false
    clearLongFocusedPriceField()
    clearLongPriceLinePaneCursor()
  }
  syncLongPositionPriceLines()
}

function fillLongEntryFromLatestClose() {
  const r = defaultLatestRawRow.value
  if (!r) return
  const c = parseNumStr(r.close)
  if (!Number.isFinite(c)) return
  longEntryStr.value = c.toFixed(2)
  showLongPosition.value = true
  syncLongPositionPriceLines()
}

const longClickNextLabel = computed(() => {
  const m = { entry: '开仓价', stop: '止损价', takeProfit: '止盈价' }
  return m[longClickNextField.value] || '开仓价'
})

const longFocusChartHint = computed(() => {
  const k = longFocusedPriceField.value
  if (!k) return ''
  const m = { entry: '开仓', stop: '止损', takeProfit: '止盈' }
  return `已选「${m[k] || ''}」：请在 K 线主图（非成交量）点击纵轴位置写入价格`
})

function cancelLongFocusBlurTimer() {
  if (longFocusBlurTimer != null) {
    clearTimeout(longFocusBlurTimer)
    longFocusBlurTimer = null
  }
}

function onLongPriceInputFocus(kind) {
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = kind
}

function onLongPriceInputBlur() {
  cancelLongFocusBlurTimer()
  longFocusBlurTimer = setTimeout(() => {
    longFocusBlurTimer = null
    longFocusedPriceField.value = null
  }, LONG_FOCUS_BLUR_CLEAR_MS)
}

function clearLongFocusedPriceField() {
  cancelLongFocusBlurTimer()
  longFocusedPriceField.value = null
}

function applyLongPriceFromChartByField(kind, price) {
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  showLongPosition.value = true
  if (kind === 'entry') longEntryStr.value = s
  else if (kind === 'stop') longStopStr.value = s
  else if (kind === 'takeProfit') longTakeProfitStr.value = s
  clearLongFocusedPriceField()
  syncLongPositionPriceLines()
}

function toggleLongClickPick() {
  longClickPickEnabled.value = !longClickPickEnabled.value
  if (longClickPickEnabled.value) {
    showLongPosition.value = true
    longClickNextField.value = 'entry'
  }
  syncLongPositionPriceLines()
}

function resetLongClickSequence() {
  longClickNextField.value = 'entry'
}

function applyLongClickPrice(price) {
  if (!Number.isFinite(price)) return
  const s = price.toFixed(2)
  const step = longClickNextField.value
  if (step === 'entry') {
    longEntryStr.value = s
    longClickNextField.value = 'stop'
  } else if (step === 'stop') {
    longStopStr.value = s
    longClickNextField.value = 'takeProfit'
  } else {
    longTakeProfitStr.value = s
    longClickNextField.value = 'entry'
  }
  syncLongPositionPriceLines()
}

function pricePropToInputStr(v) {
  if (v == null) return ''
  return String(v).trim()
}

watch(
  () => [props.longEntryPrice, props.longStopLossPrice, props.longTakeProfitPrice, props.costPrice],
  ([e, s, t, c]) => {
    let needReleaseSuppress = false
    if (e !== undefined) {
      const se = pricePropToInputStr(e)
      if (se !== longEntryStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longEntryStr.value = se
      }
    }
    if (s !== undefined) {
      const ss = pricePropToInputStr(s)
      if (ss !== longStopStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longStopStr.value = ss
      }
    }
    if (t !== undefined) {
      const st = pricePropToInputStr(t)
      if (st !== longTakeProfitStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longTakeProfitStr.value = st
      }
    }
    if (c !== undefined) {
      const sc = pricePropToInputStr(c)
      if (sc !== longCostStr.value) {
        suppressLongPriceEmit.value = true
        needReleaseSuppress = true
        longCostStr.value = sc
      }
    }
    if (needReleaseSuppress) {
      nextTick(() => {
        suppressLongPriceEmit.value = false
      })
    }
    const hasPrice = Number.isFinite(parseNumStr(longEntryStr.value)) || Number.isFinite(parseNumStr(longStopStr.value)) || Number.isFinite(parseNumStr(longTakeProfitStr.value)) || Number.isFinite(parseNumStr(longCostStr.value))
    if (hasPrice) {
      showLongPosition.value = true
      nextTick(() => {
        syncLongPositionPriceLines()
      })
    }
  },
  { immediate: true },
)

watch(longEntryStr, (v) => {
  if (suppressLongPriceEmit.value) return
  emit('update:longEntryPrice', v)
})
watch(longStopStr, (v) => {
  if (suppressLongPriceEmit.value) return
  emit('update:longStopLossPrice', v)
})
watch(longTakeProfitStr, (v) => {
  if (suppressLongPriceEmit.value) return
  emit('update:longTakeProfitPrice', v)
})
watch(longCostStr, (v) => {
  if (suppressLongPriceEmit.value) return
  emit('update:costPrice', v)
})

watch(
  [showLongPosition, longEntryStr, longStopStr, longTakeProfitStr],
  () => {
    if (longPositionDragActive) return
    syncLongPositionPriceLines()
  },
)

watch(showLongPosition, (newVal) => {
  if (newVal && getCandleSeries()) {
    nextTick(() => syncLongPositionPriceLines())
  }
})


  return { showLongPosition, longEntryStr, longStopStr, longTakeProfitStr, longCostStr, longClickPickEnabled, longClickNextField, longFocusedPriceField, suppressLongPriceEmit, longPositionStats, longPositionHint, longClickNextLabel, longFocusChartHint, toggleLongPosition, fillLongEntryFromLatestClose, toggleLongClickPick, resetLongClickSequence, applyLongClickPrice, applyLongPriceFromChartByField, onLongPriceInputFocus, onLongPriceInputBlur, clearLongFocusedPriceField, syncLongPositionPriceLines, clearLongPositionPriceLines, attachLongPriceLineDragListeners, detachLongPriceLinePaneListener, detachLongDragWindowListeners, cancelLongFocusBlurTimer, refreshLongPriceLineCursorFromCrosshair, clearLongPriceLinePaneCursor, resetLongDragState, isChartClickSuppressed }
}
