// K 线指标层：指标开关、指标序列缓存与主/副图同步、信号汇总。
// 由 StockLightweightKlineChart.vue 抽出（R-22a），逻辑逐字搬运，仅做两处机械改写：
//  1) 49 个指标开关 ref 收敛为 reactive 的 flags 记录（key 见 kline/indicators/registry.ts）；
//  2) 闭包读取的 chart / candleSeries / volSeries / mergedRawRows 改为 ctx 传入的 getter，
//     以便组件仍持有图表与数据所有权；hoverRawRow / mergedRawRowsVersion 以 ref 原样传入，
//     保持 indicatorSignalSummary 的 computed 响应性。
import { computed, reactive } from 'vue'
import { HistogramSeries, LineSeries, LineStyle } from 'lightweight-charts'
import { smaValues, emaFinite, emaLeadingNull, bollingerBands, obvValues, macdBundle, kdjBundle, rsiBundle, atrValues, vwapValues, mfiValues, kamaValues, keltnerChannelValues, supertrendValues, ichimokuValues, cciValues, ttmSqueezeValues, sarValues, donchianChannelValues, adxValues, williamsRValues, stochRsiValues, cmfValues, aroonValues, cmoValues, forceIndexValues, pivotPointsValues, demaValues, zigzagValues, satsValues, alligatorValues, aoValues, hullMaValues, adValues, trixValues, rocValues, fractalValues, chopValues, elderRayValues, chaikinOscValues, vwapBandsValues, massIndexValues, ulcerIndexValues, coppockValues, temaValues, smiValues, smcValues, trixSlopeValues } from '../components/kline/calc'
import { KLINE_INDICATOR_KEYS } from '../components/kline/indicators/registry'

export function useKlineIndicators(ctx) {
  const getChart = ctx.getChart
  const getCandleSeries = ctx.getCandleSeries
  const getMergedRawRows = ctx.getMergedRawRows
  const mergedRawRowsVersion = ctx.mergedRawRowsVersion
  const hoverRawRow = ctx.hoverRawRow
  // 49 个指标开关，初始全关；key 与工具栏注册表一致
  const flags = reactive(Object.fromEntries(KLINE_INDICATOR_KEYS.map((k) => [k, false])))

  function toggleIndicator(key) {
    flags[key] = !flags[key]
    syncIndicators()
  }

  function resetInd() {
  ind.ma5 = ind.ma10 = ind.ma20 = ind.ma60 = null
  ind.bollU = ind.bollM = ind.bollL = null
  ind.obv = null
  ind.macdHist = ind.macdDif = ind.macdDea = null
  ind.kdjK = ind.kdjD = ind.kdjJ = null
  ind.rsi = null
  ind.atr = null
  ind.vwap = null
  ind.mfi = null
  ind.kama = null
  ind.keltnerU = ind.keltnerM = ind.keltnerL = null
  ind.supertrend = null
  ind.ema12 = ind.ema21 = null
  ind.ichTenkan = ind.ichKijun = ind.ichSpanA = ind.ichSpanB = ind.ichChikou = null
  ind.cci = null
  ind.ttmHist = ind.ttmDots = null
  ind.sar = null
  ind.donchianU = ind.donchianM = ind.donchianL = null
  ind.adx = ind.adxDiP = ind.adxDiM = null
  ind.williamsR = null
  ind.stochRsi = null
  ind.stochRsiD = null
  ind.cmf = null
  ind.aroonUp = ind.aroonDown = null
  ind.cmo = null
  ind.forceIndex = null
  ind.avgAmp5 = null
  ind.avgAmp10 = null
  ind.avgAmp20 = null
  ind.pivotPP = ind.pivotS1 = ind.pivotS2 = ind.pivotR1 = ind.pivotR2 = null
  ind.dema = null
  ind.zigzag = null
  ind.satsLine = null
  ind.satsUpper = null
  ind.satsLower = null
  ind.smcSwingHigh = null
  ind.smcSwingLow = null
  ind.smcIntHigh = null
  ind.smcIntLow = null
  ind.smcBos = null
  ind.smcChoch = null
  ind.smcSwBos = null
  ind.smcSwChoch = null
  ind.smcFvgTop = null
  ind.smcFvgBot = null
  ind.smcObTop = null
  ind.smcObBot = null
  }

const ind = {
  ma5: null,
  ma10: null,
  ma20: null,
  ma60: null,
  bollU: null,
  bollM: null,
  bollL: null,
  obv: null,
  macdHist: null,
  macdDif: null,
  macdDea: null,
  kdjK: null,
  kdjD: null,
  kdjJ: null,
  rsi: null,
  atr: null,
  vwap: null,
  mfi: null,
  kama: null,
  keltnerU: null,
  keltnerM: null,
  keltnerL: null,
  supertrend: null,
  ema12: null,
  ema21: null,
  ichTenkan: null,
  ichKijun: null,
  ichSpanA: null,
  ichSpanB: null,
  ichChikou: null,
  cci: null,
  ttmHist: null,
  ttmDots: null,
  sar: null,
  donchianU: null,
  donchianM: null,
  donchianL: null,
  adx: null,
  adxDiP: null,
  adxDiM: null,
  williamsR: null,
  stochRsi: null,
  stochRsiD: null,
  cmf: null,
  aroonUp: null,
  aroonDown: null,
  cmo: null,
  forceIndex: null,
  pivotPP: null,
  pivotS1: null,
  pivotS2: null,
  pivotR1: null,
  pivotR2: null,
  dema: null,
  zigzag: null,
  satsLine: null,
  satsUpper: null,
  satsLower: null,
  avgAmp5: null,
  avgAmp10: null,
  avgAmp20: null,
  alligatorJaw: null,
  alligatorTeeth: null,
  alligatorLips: null,
  aoLine: null,
  aoHist: null,
  hullMA: null,
  adLine: null,
  trixLine: null,
  trixSignal: null,
  trixSlopeHist: null,
  rocLine: null,
  fractalHigh: null,
  fractalLow: null,
  chopLine: null,
  elderBull: null,
  elderBear: null,
  chaikinOscLine: null,
  vwapBandsU: null,
  vwapBandsM: null,
  vwapBandsL: null,
  massIndexLine: null,
  ulcerLine: null,
  coppockLine: null,
  temaLine: null,
  smiLine: null,
  smiSignal: null,
  smcSwingHigh: null,
  smcSwingLow: null,
  smcIntHigh: null,
  smcIntLow: null,
  smcBos: null,
  smcChoch: null,
  smcSwBos: null,
  smcSwChoch: null,
  smcFvgTop: null,
  smcFvgBot: null,
  smcObTop: null,
  smcObBot: null,
}


function removeSeriesSafe(api) {
  if (!api || !getChart()) return null
  try {
    getChart().removeSeries(api)
  } catch {
    /* ignore */
  }
  return null
}



function extractOHLCV(rows) {
  const sorted = [...(rows || [])].sort((a, b) => sortKey(a.day) - sortKey(b.day))
  const times = []
  const opens = []
  const closes = []
  const highs = []
  const lows = []
  const vols = []
  const amplitudes = []
  for (const r of sorted) {
    const t = toChartTime(r.day)
    if (t === null) continue
    const o = Number(r.open)
    const h = Number(r.high)
    const l = Number(r.low)
    const c = Number(r.close)
    const v = Number(r.volume)
    if (![o, h, l, c].every(Number.isFinite)) continue
    times.push(t)
    opens.push(o)
    closes.push(c)
    highs.push(h)
    lows.push(l)
    vols.push(Number.isFinite(v) ? v : 0)
    const rawAmp = parseNumStr(r.amplitude)
    amplitudes.push(Number.isFinite(rawAmp) ? rawAmp : (o > 0 ? (h - l) / o * 100 : NaN))
  }
  return { times, opens, closes, highs, lows, vols, amplitudes }
}

function toLineData(times, values) {
  const arr = []
  for (let i = 0; i < times.length; i++) {
    const v = values[i]
    if (v != null && Number.isFinite(v)) arr.push({ time: times[i], value: v })
  }
  return arr
}

/** 单根 K 的近似「成本中枢」：优先日 VWAP（成交额/量），否则典型价，夹在 [L,H] */

function tearDownAllSubPanes() {
  if (!getChart()) return
  ind.obv = removeSeriesSafe(ind.obv)
  ind.macdHist = removeSeriesSafe(ind.macdHist)
  ind.macdDif = removeSeriesSafe(ind.macdDif)
  ind.macdDea = removeSeriesSafe(ind.macdDea)
  ind.kdjK = removeSeriesSafe(ind.kdjK)
  ind.kdjD = removeSeriesSafe(ind.kdjD)
  ind.kdjJ = removeSeriesSafe(ind.kdjJ)
  ind.rsi = removeSeriesSafe(ind.rsi)
  ind.atr = removeSeriesSafe(ind.atr)
  ind.mfi = removeSeriesSafe(ind.mfi)
  ind.cci = removeSeriesSafe(ind.cci)
  ind.ttmHist = removeSeriesSafe(ind.ttmHist)
  ind.ttmDots = removeSeriesSafe(ind.ttmDots)
  ind.adx = removeSeriesSafe(ind.adx)
  ind.adxDiP = removeSeriesSafe(ind.adxDiP)
  ind.adxDiM = removeSeriesSafe(ind.adxDiM)
  ind.williamsR = removeSeriesSafe(ind.williamsR)
  ind.stochRsi = removeSeriesSafe(ind.stochRsi)
  ind.stochRsiD = removeSeriesSafe(ind.stochRsiD)
  ind.cmf = removeSeriesSafe(ind.cmf)
  ind.aroonUp = removeSeriesSafe(ind.aroonUp)
  ind.aroonDown = removeSeriesSafe(ind.aroonDown)
  ind.cmo = removeSeriesSafe(ind.cmo)
  ind.forceIndex = removeSeriesSafe(ind.forceIndex)
  ind.avgAmp5 = removeSeriesSafe(ind.avgAmp5)
  ind.avgAmp10 = removeSeriesSafe(ind.avgAmp10)
  ind.avgAmp20 = removeSeriesSafe(ind.avgAmp20)
  ind.aoHist = removeSeriesSafe(ind.aoHist)
  ind.aoLine = removeSeriesSafe(ind.aoLine)
  ind.adLine = removeSeriesSafe(ind.adLine)
  ind.trixLine = removeSeriesSafe(ind.trixLine)
  ind.trixSignal = removeSeriesSafe(ind.trixSignal)
  ind.trixSlopeHist = removeSeriesSafe(ind.trixSlopeHist)
  ind.rocLine = removeSeriesSafe(ind.rocLine)
  ind.chopLine = removeSeriesSafe(ind.chopLine)
  ind.elderBull = removeSeriesSafe(ind.elderBull)
  ind.elderBear = removeSeriesSafe(ind.elderBear)
  ind.chaikinOscLine = removeSeriesSafe(ind.chaikinOscLine)
  ind.massIndexLine = removeSeriesSafe(ind.massIndexLine)
  ind.ulcerLine = removeSeriesSafe(ind.ulcerLine)
  ind.coppockLine = removeSeriesSafe(ind.coppockLine)
  ind.smiLine = removeSeriesSafe(ind.smiLine)
  ind.smiSignal = removeSeriesSafe(ind.smiSignal)
  ind.signalRatioBullish = removeSeriesSafe(ind.signalRatioBullish)
  ind.signalRatioBearish = removeSeriesSafe(ind.signalRatioBearish)
  ind.signalRatioNet = removeSeriesSafe(ind.signalRatioNet)
  if (getChart() && typeof getChart().panes === 'function') {
    while (getChart().panes().length > 1) {
      getChart().removePane(getChart().panes().length - 1)
    }
    getChart().panes()[0]?.setStretchFactor(1)
  }
}

const subLineOpts = {
  lineWidth: 1,
  lastValueVisible: false,
  priceLineVisible: false,
}

function syncSubPaneIndicators(times, closes, highs, lows, vols) {
  if (!getChart()) return
  tearDownAllSubPanes()

  const subs = []
  if (flags.obv) subs.push('obv')
  if (flags.macd) subs.push('macd')
  if (flags.kdj) subs.push('kdj')
  if (flags.rsi) subs.push('rsi')
  if (flags.atr) subs.push('atr')
  if (flags.mfi) subs.push('mfi')
  if (flags.cci) subs.push('cci')
  if (flags.ttmSqueeze) subs.push('ttmSqueeze')
  if (flags.adx) subs.push('adx')
  if (flags.williamsR) subs.push('williamsR')
  if (flags.stochRsi) subs.push('stochRsi')
  if (flags.cmf) subs.push('cmf')
  if (flags.aroon) subs.push('aroon')
  if (flags.cmo) subs.push('cmo')
  if (flags.forceIndex) subs.push('forceIndex')
  if (flags.avgAmp) subs.push('avgAmp')
  if (flags.ao) subs.push('ao')
  if (flags.ad) subs.push('ad')
  if (flags.trix) subs.push('trix')
  if (flags.trixSlope) subs.push('trixSlope')
  if (flags.roc) subs.push('roc')
  if (flags.chop) subs.push('chop')
  if (flags.elderRay) subs.push('elderRay')
  if (flags.chaikinOsc) subs.push('chaikinOsc')
  if (flags.massIndex) subs.push('massIndex')
  if (flags.ulcerIndex) subs.push('ulcerIndex')
  if (flags.coppock) subs.push('coppock')
  if (flags.smi) subs.push('smi')
  if (flags.signalRatio) subs.push('signalRatio')
  if (subs.length === 0) return

  if (getChart() && typeof getChart().panes === 'function') {
    getChart().panes()[0]?.setStretchFactor(3)
  }

  let paneIdx = 1
  for (const key of subs) {
    if (key === 'obv') {
      const obv = obvValues(closes, vols)
      ind.obv = getChart().addSeries(
        LineSeries,
        {
          color: '#22c55e',
          lineWidth: 1,
          title: 'OBV',
          lastValueVisible: true,
          priceLineVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.obv.setData(toLineData(times, obv))
    } else if (key === 'macd') {
      const { dif, dea, hist } = macdBundle(closes)
      ind.macdHist = getChart().addSeries(
        HistogramSeries,
        {
          priceFormat: { type: 'price', precision: 4, minMove: 0.0001 },
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      ind.macdDif = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'DIF',
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      ind.macdDea = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#6366f1',
          title: 'DEA',
          priceScaleId: 'macd',
        },
        paneIdx,
      )
      const histData = []
      for (let i = 0; i < times.length; i++) {
        const hv = hist[i]
        if (hv == null || !Number.isFinite(hv)) continue
        histData.push({
          time: times[i],
          value: hv,
          color:
            hv >= 0
              ? 'rgba(239, 83, 80, 0.55)'
              : 'rgba(38, 166, 154, 0.55)',
        })
      }
      ind.macdHist.setData(histData)
      ind.macdDif.setData(toLineData(times, dif))
      ind.macdDea.setData(toLineData(times, dea))
    } else if (key === 'kdj') {
      const { K, D, J } = kdjBundle(highs, lows, closes, 9)
      ind.kdjK = getChart().addSeries(
        LineSeries,
        { ...subLineOpts, color: '#f59e0b', title: 'K' },
        paneIdx,
      )
      ind.kdjD = getChart().addSeries(
        LineSeries,
        { ...subLineOpts, color: '#3b82f6', title: 'D' },
        paneIdx,
      )
      ind.kdjJ = getChart().addSeries(
        LineSeries,
        { ...subLineOpts, color: '#a855f7', title: 'J' },
        paneIdx,
      )
      ind.kdjK.setData(toLineData(times, K))
      ind.kdjD.setData(toLineData(times, D))
      ind.kdjJ.setData(toLineData(times, J))
    } else if (key === 'rsi') {
      const rsi = rsiBundle(closes, 14)
      ind.rsi = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#d946ef',
          title: 'RSI14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.rsi.setData(toLineData(times, rsi))
    } else if (key === 'atr') {
      const atr = atrValues(highs, lows, closes, 14)
      ind.atr = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#06b6d4',
          title: 'ATR14',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.atr.setData(toLineData(times, atr))
    } else if (key === 'mfi') {
      const mfi = mfiValues(highs, lows, closes, vols, 14)
      ind.mfi = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'MFI14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.mfi.setData(toLineData(times, mfi))
    } else if (key === 'cci') {
      const cci = cciValues(highs, lows, closes, 20)
      ind.cci = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#eab308',
          title: 'CCI20',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.cci.setData(toLineData(times, cci))
    } else if (key === 'ttmSqueeze') {
      const { squeeze, momentum } = ttmSqueezeValues(highs, lows, closes)
      ind.ttmHist = getChart().addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.ttmDots = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#6366f1',
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 2,
          title: 'SQZ',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const histData = []
      const dotData = []
      for (let i = 0; i < times.length; i++) {
        const mv = momentum[i]
        if (mv != null && Number.isFinite(mv)) {
          histData.push({
            time: times[i],
            value: mv,
            color: mv >= 0
              ? (squeeze[i] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.4)')
              : (squeeze[i] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.4)'),
          })
          dotData.push({
            time: times[i],
            value: 0,
            color: squeeze[i] ? '#eab308' : '#22c55e',
          })
        }
      }
      ind.ttmHist.setData(histData)
      ind.ttmDots.setData(dotData)
    } else if (key === 'adx') {
      const { adx, diP, diM } = adxValues(highs, lows, closes, 14)
      ind.adxDiP = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#22c55e',
          lineWidth: 1,
          title: '+DI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adxDiM = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          lineWidth: 1,
          title: '-DI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adx = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'ADX',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.adxDiP.setData(toLineData(times, diP))
      ind.adxDiM.setData(toLineData(times, diM))
      ind.adx.setData(toLineData(times, adx))
    } else if (key === 'williamsR') {
      const wr = williamsRValues(highs, lows, closes, 14)
      ind.williamsR = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'W%R14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.williamsR.setData(toLineData(times, wr))
    } else if (key === 'stochRsi') {
      const { k, d } = stochRsiValues(closes, 14, 14, 3, 3)
      ind.stochRsi = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#06b6d4',
          title: 'StochRSI K',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.stochRsiD = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'StochRSI D',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.stochRsi.setData(toLineData(times, k))
      ind.stochRsiD.setData(toLineData(times, d))
    } else if (key === 'cmf') {
      const cmf = cmfValues(highs, lows, closes, vols, 20)
      ind.cmf = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#14b8a6',
          title: 'CMF20',
          priceFormat: { type: 'price', precision: 3, minMove: 0.001 },
        },
        paneIdx,
      )
      ind.cmf.setData(toLineData(times, cmf))
    } else if (key === 'aroon') {
      const { up, down } = aroonValues(highs, lows, 25)
      ind.aroonUp = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#22c55e',
          title: 'Aroon Up',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.aroonDown = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Aroon Down',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.aroonUp.setData(toLineData(times, up))
      ind.aroonDown.setData(toLineData(times, down))
    } else if (key === 'cmo') {
      const cmo = cmoValues(closes, 14)
      ind.cmo = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'CMO14',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.cmo.setData(toLineData(times, cmo))
    } else if (key === 'forceIndex') {
      const fi = forceIndexValues(closes, vols, 13)
      ind.forceIndex = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'FI13',
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.forceIndex.setData(toLineData(times, fi))
    } else if (key === 'avgAmp') {
      const { amplitudes } = extractOHLCV(getMergedRawRows())
      const aa5 = smaValues(amplitudes, 5)
      const aa10 = smaValues(amplitudes, 10)
      const aa20 = smaValues(amplitudes, 20)
      ind.avgAmp5 = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: '均幅5',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.avgAmp10 = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          title: '均幅10',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.avgAmp20 = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#a855f7',
          title: '均幅20',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.avgAmp5.setData(toLineData(times, aa5))
      ind.avgAmp10.setData(toLineData(times, aa10))
      ind.avgAmp20.setData(toLineData(times, aa20))
    } else if (key === 'ao') {
      const ao = aoValues(highs, lows)
      ind.aoHist = getChart().addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.aoLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          title: 'AO',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const aoHistData = []
      for (let i = 0; i < times.length; i++) {
        const v = ao[i]
        if (v != null && Number.isFinite(v)) {
          aoHistData.push({
            time: times[i],
            value: v,
            color: v >= 0
              ? (i > 0 && ao[i - 1] != null && v > ao[i - 1] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.35)')
              : (i > 0 && ao[i - 1] != null && v < ao[i - 1] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.35)'),
          })
        }
      }
      ind.aoHist.setData(aoHistData)
      ind.aoLine.setData(toLineData(times, ao))
    } else if (key === 'ad') {
      const ad = adValues(highs, lows, closes, vols)
      ind.adLine = getChart().addSeries(
        LineSeries,
        {
          color: '#22c55e',
          lineWidth: 1,
          title: 'A/D',
          lastValueVisible: true,
          priceLineVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      ind.adLine.setData(toLineData(times, ad))
    } else if (key === 'trix') {
      const trix = trixValues(closes, 15)
      const signal = emaLeadingNull(trix, 9)
      ind.trixLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'TRIX',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.trixSignal = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Signal',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.trixLine.setData(toLineData(times, trix))
      ind.trixSignal.setData(toLineData(times, signal))
    } else if (key === 'trixSlope') {
      const slope = trixSlopeValues(closes, 15)
      ind.trixSlopeHist = getChart().addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const slopeData = []
      for (let i = 0; i < times.length; i++) {
        const sv = slope[i]
        if (sv != null && Number.isFinite(sv)) {
          slopeData.push({
            time: times[i],
            value: sv,
            color: sv >= 0
              ? (i > 0 && slope[i - 1] != null && sv > slope[i - 1] ? 'rgba(239, 83, 80, 0.7)' : 'rgba(239, 83, 80, 0.35)')
              : (i > 0 && slope[i - 1] != null && sv < slope[i - 1] ? 'rgba(38, 166, 154, 0.7)' : 'rgba(38, 166, 154, 0.35)'),
          })
        }
      }
      ind.trixSlopeHist.setData(slopeData)
    } else if (key === 'roc') {
      const roc = rocValues(closes, 12)
      ind.rocLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#d946ef',
          title: 'ROC12',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.rocLine.setData(toLineData(times, roc))
    } else if (key === 'chop') {
      const chop = chopValues(highs, lows, closes, 14)
      ind.chopLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f97316',
          title: 'CHOP',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.chopLine.setData(toLineData(times, chop))
    } else if (key === 'elderRay') {
      const { bullPower, bearPower } = elderRayValues(highs, lows, closes, 13)
      ind.elderBull = getChart().addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.elderBear = getChart().addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      const bullData = []
      const bearData = []
      for (let i = 0; i < times.length; i++) {
        const bv = bullPower[i]
        const brv = bearPower[i]
        if (bv != null && Number.isFinite(bv)) {
          bullData.push({
            time: times[i],
            value: bv,
            color: bv >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(239, 68, 68, 0.35)',
          })
        }
        if (brv != null && Number.isFinite(brv)) {
          bearData.push({
            time: times[i],
            value: brv,
            color: brv >= 0 ? 'rgba(34, 197, 94, 0.7)' : 'rgba(34, 197, 94, 0.35)',
          })
        }
      }
      ind.elderBull.setData(bullData)
      ind.elderBear.setData(bearData)
    } else if (key === 'chaikinOsc') {
      const co = chaikinOscValues(highs, lows, closes, vols, 3, 10)
      ind.chaikinOscLine = getChart().addSeries(
        HistogramSeries,
        {
          priceLineVisible: false,
          lastValueVisible: false,
          priceFormat: { type: 'price', precision: 0, minMove: 1 },
        },
        paneIdx,
      )
      const coData = []
      for (let i = 0; i < times.length; i++) {
        const v = co[i]
        if (v != null && Number.isFinite(v)) {
          coData.push({
            time: times[i],
            value: v,
            color: v >= 0 ? 'rgba(239, 68, 68, 0.7)' : 'rgba(34, 197, 94, 0.7)',
          })
        }
      }
      ind.chaikinOscLine.setData(coData)
    } else if (key === 'massIndex') {
      const mi = massIndexValues(highs, lows)
      ind.massIndexLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          title: 'Mass',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.massIndexLine.setData(toLineData(times, mi))
    } else if (key === 'ulcerIndex') {
      const ui = ulcerIndexValues(closes)
      ind.ulcerLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Ulcer',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.ulcerLine.setData(toLineData(times, ui))
    } else if (key === 'coppock') {
      const cp = coppockValues(closes)
      ind.coppockLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#8b5cf6',
          title: 'Coppock',
          priceFormat: { type: 'price', precision: 2, minMove: 0.01 },
        },
        paneIdx,
      )
      ind.coppockLine.setData(toLineData(times, cp))
    } else if (key === 'smi') {
      const { smi: smiData, signal: smiSig } = smiValues(highs, lows, closes)
      ind.smiLine = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#3b82f6',
          lineWidth: 2,
          title: 'SMI',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.smiSignal = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#ef4444',
          title: 'Signal',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.smiLine.setData(toLineData(times, smiData))
      ind.smiSignal.setData(toLineData(times, smiSig))
    } else if (key === 'signalRatio') {
      // 逐根 K 线评估所有指标信号，计算看多/看空/中性/震荡比例
      const bullishArr = new Array(times.length).fill(null)
      const bearishArr = new Array(times.length).fill(null)
      const netArr = new Array(times.length).fill(null)
      // 预计算所有指标数组（只算一次）
      const ma5 = smaValues(closes, 5)
      const ma10 = smaValues(closes, 10)
      const ma20 = smaValues(closes, 20)
      const ma60 = smaValues(closes, 60)
      const ema12 = emaFinite(closes, 12)
      const ema21 = emaFinite(closes, 21)
      const { upper: bollU, mid: bollM, lower: bollL } = bollingerBands(closes, 20, 2)
      const vw = vwapValues(highs, lows, closes, vols, 20)
      const dema21 = demaValues(closes, 21)
      const tema21 = temaValues(closes, 21)
      const kama10 = kamaValues(closes, 10, 2, 30)
      const hull9 = hullMaValues(closes, 9)
      const { upper: kU, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
      const { direction: stDir } = supertrendValues(highs, lows, closes, 10, 3)
      const { tenkan: ichTen, kijun: ichKij, spanA: ichSA, senkouB: ichSB } = ichimokuValues(highs, lows, closes)
      const { direction: sarDir } = sarValues(highs, lows, closes, 0.02, 0.2)
      const { upper: dcU, lower: dcL } = donchianChannelValues(highs, lows, 20)
      const { jaw: agJ, teeth: agT, lips: agL } = alligatorValues(highs, lows, closes)
      const { directions: zzDir } = zigzagValues(highs, lows, closes, 5)
      const { direction: satsDir } = satsValues(highs, lows, closes, vols)
      const { pp: pivPP, s1: pivS1, r1: pivR1 } = pivotPointsValues(highs, lows, closes)
      const { upper: vbU, lower: vbL } = vwapBandsValues(highs, lows, closes, vols)
      const { dif: macdDif, dea: macdDea, hist: macdHist } = macdBundle(closes)
      const rsi14 = rsiBundle(closes, 14)
      const { K: kdjK, D: kdjD, J: kdjJ } = kdjBundle(highs, lows, closes, 9)
      const cci20 = cciValues(highs, lows, closes, 20)
      const wr14 = williamsRValues(highs, lows, closes, 14)
      const { k: stochK, d: stochD } = stochRsiValues(closes, 14, 14, 3, 3)
      const { adx: adxVal, diP: adxP, diM: adxM } = adxValues(highs, lows, closes, 14)
      const { up: arUp, down: arDown } = aroonValues(highs, lows, 25)
      const cmo14 = cmoValues(closes, 14)
      const trix15 = trixValues(closes, 15)
      const trixSig = emaLeadingNull(trix15, 9)
      const roc12 = rocValues(closes, 12)
      const coppock = coppockValues(closes)
      const { smi: smiD, signal: smiS } = smiValues(highs, lows, closes)
      const ao534 = aoValues(highs, lows)
      const obv = obvValues(closes, vols)
      const mfi14 = mfiValues(highs, lows, closes, vols, 14)
      const cmf20 = cmfValues(highs, lows, closes, vols, 20)
      const adLine = adValues(highs, lows, closes, vols)
      const fi13 = forceIndexValues(closes, vols, 13)
      const co310 = chaikinOscValues(highs, lows, closes, vols, 3, 10)
      const atr14 = atrValues(highs, lows, closes, 14)
      const chop14 = chopValues(highs, lows, closes, 14)
      const miVal = massIndexValues(highs, lows)
      const uiVal = ulcerIndexValues(closes)
      const { squeeze: ttmSq, momentum: ttmMo } = ttmSqueezeValues(highs, lows, closes)
      const { bullPower: erBull, bearPower: erBear } = elderRayValues(highs, lows, closes, 13)
      // 辅助：读取数组在 i 位置的值
      const v = (arr, i) => (i >= 0 && i < arr.length && arr[i] != null && Number.isFinite(arr[i])) ? arr[i] : null
      const vPrev = (arr, i) => v(arr, i - 1)
      // 构建ZigZag方向缓存（找最近非零方向）
      const zzLastDir = new Array(times.length).fill(0)
      let lastNonZero = 0
      for (let i = 0; i < zzDir.length; i++) {
        if (zzDir[i] === 1 || zzDir[i] === -1) lastNonZero = zzDir[i]
        zzLastDir[i] = lastNonZero
      }
      // 逐根K线评估
      for (let i = 0; i < times.length; i++) {
        const c = closes[i]
        if (c == null || !Number.isFinite(c)) continue
        let bull = 0, bear = 0, neut = 0, osci = 0, cnt = 0
        // MA
        { const a5=v(ma5,i),a10=v(ma10,i),a20=v(ma20,i),a60=v(ma60,i)
          if(a5!=null&&a10!=null&&a20!=null&&a60!=null){cnt++;if(a5>a10&&a10>a20&&a20>a60)bull++;else if(a5<a10&&a10<a20&&a20<a60)bear++;else if((a5>a20&&a10<a60)||(a5<a20&&a10>a60))osci++;else neut++;} }
        // EMA
        { const e12=v(ema12,i),e21=v(ema21,i)
          if(e12!=null&&e21!=null){cnt++;if(e12>e21)bull++;else if(e12<e21)bear++;else neut++;} }
        // BOLL
        { const bu=v(bollU,i),bm=v(bollM,i),bl=v(bollL,i)
          if(bu!=null&&bm!=null&&bl!=null){cnt++;if(c>bu)bull++;else if(c<bl)bear++;else if(c>bm)osci++;else neut++;} }
        // VWAP
        { const vwv=v(vw,i);if(vwv!=null){cnt++;if(c>vwv)bull++;else if(c<vwv)bear++;else neut++;} }
        // DEMA
        { const dv=v(dema21,i);if(dv!=null){cnt++;if(c>dv)bull++;else if(c<dv)bear++;else neut++;} }
        // TEMA
        { const tv=v(tema21,i);if(tv!=null){cnt++;if(c>tv)bull++;else if(c<tv)bear++;else neut++;} }
        // KAMA
        { const kv=v(kama10,i),kp=vPrev(kama10,i)
          if(kv!=null&&kp!=null){cnt++;if(c>kv&&kv>kp)bull++;else if(c<kv&&kv<kp)bear++;else neut++;} }
        // HullMA
        { const hv=v(hull9,i),hp=vPrev(hull9,i)
          if(hv!=null&&hp!=null){cnt++;if(hv>hp)bull++;else if(hv<hp)bear++;else neut++;} }
        // Keltner
        { const ku=v(kU,i),kl=v(kL,i)
          if(ku!=null&&kl!=null){cnt++;if(c>ku)bull++;else if(c<kl)bear++;else osci++;} }
        // SuperTrend
        { const sd=v(stDir,i);if(sd!=null){cnt++;if(sd===1)bull++;else if(sd===-1)bear++;else neut++;} }
        // Ichimoku
        { const it=v(ichTen,i),ik=v(ichKij,i),isa=v(ichSA,i),isb=v(ichSB,i)
          if(it!=null&&ik!=null&&isa!=null&&isb!=null){const ct=Math.max(isa,isb),cb=Math.min(isa,isb);cnt++;if(c>ct&&it>ik)bull++;else if(c<cb&&it<ik)bear++;else if(c>=cb&&c<=ct)osci++;else neut++;} }
        // SAR
        { const sd=v(sarDir,i);if(sd!=null){cnt++;if(sd===1)bull++;else if(sd===-1)bear++;else neut++;} }
        // Donchian
        { const du=v(dcU,i),dl=v(dcL,i)
          if(du!=null&&dl!=null){cnt++;if(c>=du)bull++;else if(c<=dl)bear++;else osci++;} }
        // Alligator
        { const aj=v(agJ,i),at=v(agT,i),al=v(agL,i)
          if(aj!=null&&at!=null&&al!=null){cnt++;if(al>at&&at>aj)bull++;else if(al<at&&at<aj)bear++;else osci++;} }
        // ZigZag
        { const zd=zzLastDir[i];if(zd!==0){cnt++;if(zd===-1)bull++;else if(zd===1)bear++;else neut++;} }
        // SATS
        { const sd=v(satsDir,i);if(sd!=null){cnt++;if(sd===1)bull++;else if(sd===-1)bear++;else neut++;} }
        // Pivot
        { const pp=v(pivPP,i),s1=v(pivS1,i),r1=v(pivR1,i)
          if(pp!=null&&r1!=null&&s1!=null){cnt++;if(c>r1)bull++;else if(c<s1)bear++;else if(c>pp)osci++;else neut++;} }
        // VWAPBands
        { const vu=v(vbU,i),vl=v(vbL,i)
          if(vu!=null&&vl!=null){cnt++;if(c>vu)bull++;else if(c<vl)bear++;else osci++;} }
        // MACD
        { const md=v(macdDif,i),me=v(macdDea,i),mh=v(macdHist,i)
          if(md!=null&&me!=null&&mh!=null){cnt++;if(md>me&&mh>0)bull++;else if(md<me&&mh<0)bear++;else if((md>0&&mh<0)||(md<0&&mh>0))osci++;else neut++;} }
        // RSI
        { const rv=v(rsi14,i);if(rv!=null){cnt++;if(rv>70)osci++;else if(rv<30)osci++;else if(rv>50)bull++;else bear++;} }
        // KDJ
        { const kk=v(kdjK,i),kd=v(kdjD,i),kj=v(kdjJ,i)
          if(kk!=null&&kd!=null&&kj!=null){cnt++;if(kj>kk&&kk>kd&&kk<80)bull++;else if(kj<kk&&kk<kd&&kk>20)bear++;else if(kk>80)bear++;else if(kk<20)bull++;else osci++;} }
        // CCI
        { const cv=v(cci20,i);if(cv!=null){cnt++;if(cv>100)bull++;else if(cv<-100)bear++;else osci++;} }
        // W%R
        { const wv=v(wr14,i);if(wv!=null){cnt++;if(wv<-80)bull++;else if(wv>-20)bear++;else osci++;} }
        // StochRSI
        { const sk=v(stochK,i),sd2=v(stochD,i)
          if(sk!=null&&sd2!=null){cnt++;if(sk<20&&sd2<20&&sk>sd2)bull++;else if(sk>80&&sd2>80&&sk<sd2)bear++;else osci++;} }
        // ADX
        { const av=v(adxVal,i),ap=v(adxP,i),am=v(adxM,i)
          if(av!=null&&ap!=null&&am!=null){cnt++;if(av>25&&ap>am)bull++;else if(av>25&&ap<am)bear++;else osci++;} }
        // Aroon
        { const au=v(arUp,i),ad2=v(arDown,i)
          if(au!=null&&ad2!=null){cnt++;if(au>70&&ad2<30)bull++;else if(ad2>70&&au<30)bear++;else osci++;} }
        // CMO
        { const cv=cmo14[i];if(cv!=null&&Number.isFinite(cv)){cnt++;if(cv>50)bull++;else if(cv<-50)bear++;else osci++;} }
        // TRIX
        { const tv=v(trix15,i),ts=v(trixSig,i)
          if(tv!=null&&ts!=null){cnt++;if(tv>ts)bull++;else if(tv<ts)bear++;else neut++;} }
        // ROC
        { const rv=v(roc12,i);if(rv!=null){cnt++;if(rv>0)bull++;else if(rv<0)bear++;else neut++;} }
        // Coppock
        { const cv=v(coppock,i),cp=vPrev(coppock,i)
          if(cv!=null&&cp!=null){cnt++;if(cv>0&&cp<=0)bull++;else if(cv<0)bear++;else neut++;} }
        // SMI
        { const sv=v(smiD,i),ss=v(smiS,i)
          if(sv!=null&&ss!=null){cnt++;if(sv>ss&&sv>0)bull++;else if(sv<ss&&sv<0)bear++;else osci++;} }
        // AO
        { const av2=v(ao534,i),ap2=vPrev(ao534,i)
          if(av2!=null&&ap2!=null){cnt++;if(av2>0&&av2>ap2)bull++;else if(av2<0&&av2<ap2)bear++;else if(av2>0&&av2<ap2)osci++;else neut++;} }
        // OBV
        { const ov=v(obv,i),op=vPrev(obv,i)
          if(ov!=null&&op!=null){cnt++;if(ov>op)bull++;else if(ov<op)bear++;else neut++;} }
        // MFI
        { const mv=v(mfi14,i);if(mv!=null){cnt++;if(mv>80)osci++;else if(mv<20)osci++;else if(mv>50)bull++;else bear++;} }
        // CMF
        { const cv=v(cmf20,i);if(cv!=null){cnt++;if(cv>0.05)bull++;else if(cv<-0.05)bear++;else osci++;} }
        // A/D
        { const av3=v(adLine,i),ap3=vPrev(adLine,i)
          if(av3!=null&&ap3!=null){cnt++;if(av3>ap3)bull++;else if(av3<ap3)bear++;else neut++;} }
        // FI
        { const fv=v(fi13,i);if(fv!=null){cnt++;if(fv>0)bull++;else if(fv<0)bear++;else neut++;} }
        // ChaikinOsc
        { const cv=v(co310,i),cp=vPrev(co310,i)
          if(cv!=null&&cp!=null){cnt++;if(cv>0&&cv>cp)bull++;else if(cv<0&&cv<cp)bear++;else osci++;} }
        // ATR
        { const av4=v(atr14,i),ap4=vPrev(atr14,i)
          if(av4!=null&&ap4!=null){cnt++;if(av4>ap4)osci++;else neut++;} }
        // CHOP
        { const cv2=v(chop14,i);if(cv2!=null){cnt++;if(cv2>61.8)osci++;else neut++;} }
        // MassIndex
        { const mv2=v(miVal,i),mp=vPrev(miVal,i)
          if(mv2!=null&&mp!=null){cnt++;if(mp>27&&mv2<27)bull++;else neut++;} }
        // UlcerIndex
        { const uv=v(uiVal,i);if(uv!=null){cnt++;if(uv<5)bull++;else if(uv>15)bear++;else neut++;} }
        // TTM
        { const sq=v(ttmSq,i),mo=v(ttmMo,i)
          if(sq!=null&&mo!=null){cnt++;if(!sq&&mo>0)bull++;else if(!sq&&mo<0)bear++;else if(sq)osci++;else neut++;} }
        // ElderRay
        { const bp=v(erBull,i),brp=v(erBear,i)
          if(bp!=null&&brp!=null){cnt++;if(bp>0&&bp>brp)bull++;else if(brp<0&&brp<bp)bear++;else osci++;} }
        // 汇总
        if (cnt > 0) {
          bullishArr[i] = (bull / cnt) * 100
          bearishArr[i] = (bear / cnt) * 100
          netArr[i] = ((bull - bear) / cnt) * 100
          // 中立(neut) 与震荡(osci) 计票当前不参与信号比渲染。
          // 它们是完整四分类的组成部分（分类分支本身是可读性刚需），
          // 故显式消费一次以避免 no-useless-assignment 把它们误判为死赋值。
          void neut; void osci
        }
      }
      ind.signalRatioBullish = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: 'rgba(239, 68, 68, 0.7)',
          lineWidth: 1,
          title: '看多%',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.signalRatioBearish = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: 'rgba(34, 197, 94, 0.7)',
          lineWidth: 1,
          title: '看空%',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.signalRatioNet = getChart().addSeries(
        LineSeries,
        {
          ...subLineOpts,
          color: '#f59e0b',
          lineWidth: 2,
          title: '净信号',
          priceFormat: { type: 'price', precision: 1, minMove: 0.1 },
        },
        paneIdx,
      )
      ind.signalRatioBullish.setData(toLineData(times, bullishArr))
      ind.signalRatioBearish.setData(toLineData(times, bearishArr))
      ind.signalRatioNet.setData(toLineData(times, netArr))
    }
    paneIdx++
  }

  if (getChart() && typeof getChart().panes === 'function') {
    for (let i = 1; i < getChart().panes().length; i++) {
      getChart().panes()[i]?.setStretchFactor(1)
    }
  }
}

function syncIndicators() {
  if (!getChart() || !getCandleSeries()) return

  const { times, opens, closes, highs, lows, vols } = extractOHLCV(getMergedRawRows())
  if (!times.length) {
    ind.ma5 = removeSeriesSafe(ind.ma5)
    ind.ma10 = removeSeriesSafe(ind.ma10)
    ind.ma20 = removeSeriesSafe(ind.ma20)
    ind.ma60 = removeSeriesSafe(ind.ma60)
    ind.bollU = removeSeriesSafe(ind.bollU)
    ind.bollM = removeSeriesSafe(ind.bollM)
    ind.bollL = removeSeriesSafe(ind.bollL)
    ind.vwap = removeSeriesSafe(ind.vwap)
    ind.kama = removeSeriesSafe(ind.kama)
    ind.keltnerU = removeSeriesSafe(ind.keltnerU)
    ind.keltnerM = removeSeriesSafe(ind.keltnerM)
    ind.keltnerL = removeSeriesSafe(ind.keltnerL)
    ind.supertrend = removeSeriesSafe(ind.supertrend)
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
    ind.ichTenkan = removeSeriesSafe(ind.ichTenkan)
    ind.ichKijun = removeSeriesSafe(ind.ichKijun)
    ind.ichSpanA = removeSeriesSafe(ind.ichSpanA)
    ind.ichSpanB = removeSeriesSafe(ind.ichSpanB)
    ind.ichChikou = removeSeriesSafe(ind.ichChikou)
    ind.supertrend = removeSeriesSafe(ind.supertrend)
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
    ind.sar = removeSeriesSafe(ind.sar)
    ind.donchianU = removeSeriesSafe(ind.donchianU)
    ind.donchianM = removeSeriesSafe(ind.donchianM)
    ind.donchianL = removeSeriesSafe(ind.donchianL)
    ind.pivotPP = removeSeriesSafe(ind.pivotPP)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2)
    ind.dema = removeSeriesSafe(ind.dema)
    ind.zigzag = removeSeriesSafe(ind.zigzag)
    ind.satsLine = removeSeriesSafe(ind.satsLine)
    ind.satsUpper = removeSeriesSafe(ind.satsUpper)
    ind.satsLower = removeSeriesSafe(ind.satsLower)
    ind.alligatorJaw = removeSeriesSafe(ind.alligatorJaw)
    ind.alligatorTeeth = removeSeriesSafe(ind.alligatorTeeth)
    ind.alligatorLips = removeSeriesSafe(ind.alligatorLips)
    ind.hullMA = removeSeriesSafe(ind.hullMA)
    ind.fractalHigh = removeSeriesSafe(ind.fractalHigh)
    ind.fractalLow = removeSeriesSafe(ind.fractalLow)
    ind.vwapBandsU = removeSeriesSafe(ind.vwapBandsU)
    ind.vwapBandsM = removeSeriesSafe(ind.vwapBandsM)
    ind.vwapBandsL = removeSeriesSafe(ind.vwapBandsL)
    ind.temaLine = removeSeriesSafe(ind.temaLine)
    ind.smcSwingHigh = removeSeriesSafe(ind.smcSwingHigh)
    ind.smcSwingLow = removeSeriesSafe(ind.smcSwingLow)
    ind.smcIntHigh = removeSeriesSafe(ind.smcIntHigh)
    ind.smcIntLow = removeSeriesSafe(ind.smcIntLow)
    ind.smcBos = removeSeriesSafe(ind.smcBos)
    ind.smcChoch = removeSeriesSafe(ind.smcChoch)
    ind.smcSwBos = removeSeriesSafe(ind.smcSwBos)
    ind.smcSwChoch = removeSeriesSafe(ind.smcSwChoch)
    ind.smcFvgTop = removeSeriesSafe(ind.smcFvgTop)
    ind.smcFvgBot = removeSeriesSafe(ind.smcFvgBot)
    ind.smcObTop = removeSeriesSafe(ind.smcObTop)
    ind.smcObBot = removeSeriesSafe(ind.smcObBot)
    tearDownAllSubPanes()
    return
  }

  const lineCommon = {
    lineWidth: 1,
    lastValueVisible: false,
    priceLineVisible: false,
  }

  if (flags.ma) {
    const m5 = smaValues(closes, 5)
    const m10 = smaValues(closes, 10)
    const m20 = smaValues(closes, 20)
    const m60 = smaValues(closes, 60)
    if (!ind.ma5) {
      ind.ma5 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', title: 'MA5' },
        0,
      )
    }
    if (!ind.ma10) {
      ind.ma10 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: 'MA10' },
        0,
      )
    }
    if (!ind.ma20) {
      ind.ma20 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#a855f7', title: 'MA20' },
        0,
      )
    }
    if (!ind.ma60) {
      ind.ma60 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#64748b', title: 'MA60' },
        0,
      )
    }
    ind.ma5.setData(toLineData(times, m5))
    ind.ma10.setData(toLineData(times, m10))
    ind.ma20.setData(toLineData(times, m20))
    ind.ma60.setData(toLineData(times, m60))
  } else {
    ind.ma5 = removeSeriesSafe(ind.ma5)
    ind.ma10 = removeSeriesSafe(ind.ma10)
    ind.ma20 = removeSeriesSafe(ind.ma20)
    ind.ma60 = removeSeriesSafe(ind.ma60)
  }

  if (flags.boll) {
    const { upper, mid, lower } = bollingerBands(closes, 20, 2)
    if (!ind.bollU) {
      ind.bollU = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#94a3b8',
          lineStyle: LineStyle.Dashed,
          title: 'BOLL上',
        },
        0,
      )
    }
    if (!ind.bollM) {
      ind.bollM = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#0ea5e9', title: 'BOLL中' },
        0,
      )
    }
    if (!ind.bollL) {
      ind.bollL = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#94a3b8',
          lineStyle: LineStyle.Dashed,
          title: 'BOLL下',
        },
        0,
      )
    }
    ind.bollU.setData(toLineData(times, upper))
    ind.bollM.setData(toLineData(times, mid))
    ind.bollL.setData(toLineData(times, lower))
  } else {
    ind.bollU = removeSeriesSafe(ind.bollU)
    ind.bollM = removeSeriesSafe(ind.bollM)
    ind.bollL = removeSeriesSafe(ind.bollL)
  }

  if (flags.vwap) {
    const vwap = vwapValues(highs, lows, closes, vols, 20)
    if (!ind.vwap) {
      ind.vwap = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#ec4899', title: 'VWAP20' },
        0,
      )
    }
    ind.vwap.setData(toLineData(times, vwap))
  } else {
    ind.vwap = removeSeriesSafe(ind.vwap)
  }

  if (flags.kama) {
    const kama = kamaValues(closes, 10, 2, 30)
    if (!ind.kama) {
      ind.kama = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#14b8a6', title: 'KAMA10' },
        0,
      )
    }
    ind.kama.setData(toLineData(times, kama))
  } else {
    ind.kama = removeSeriesSafe(ind.kama)
  }

  if (flags.keltner) {
    const { upper: kU, mid: kM, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
    if (!ind.keltnerU) {
      ind.keltnerU = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'Kelt上',
        },
        0,
      )
    }
    if (!ind.keltnerM) {
      ind.keltnerM = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#8b5cf6', title: 'Kelt中' },
        0,
      )
    }
    if (!ind.keltnerL) {
      ind.keltnerL = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'Kelt下',
        },
        0,
      )
    }
    ind.keltnerU.setData(toLineData(times, kU))
    ind.keltnerM.setData(toLineData(times, kM))
    ind.keltnerL.setData(toLineData(times, kL))
  } else {
    ind.keltnerU = removeSeriesSafe(ind.keltnerU)
    ind.keltnerM = removeSeriesSafe(ind.keltnerM)
    ind.keltnerL = removeSeriesSafe(ind.keltnerL)
  }

  if (flags.supertrend) {
    const { supertrend: stVal, direction } = supertrendValues(highs, lows, closes, 10, 3)
    if (!ind.supertrend) {
      ind.supertrend = getChart().addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 2, title: 'ST(10,3)' },
        0,
      )
    }
    const stData = []
    for (let i = 0; i < times.length; i++) {
      if (stVal[i] != null) {
        stData.push({
          time: times[i],
          value: stVal[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.supertrend.setData(stData)
  } else {
    ind.supertrend = removeSeriesSafe(ind.supertrend)
  }

  if (flags.ema) {
    const e12 = emaFinite(closes, 12)
    const e21 = emaFinite(closes, 21)
    if (!ind.ema12) {
      ind.ema12 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', title: 'EMA12' },
        0,
      )
    }
    if (!ind.ema21) {
      ind.ema21 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: 'EMA21' },
        0,
      )
    }
    ind.ema12.setData(toLineData(times, e12))
    ind.ema21.setData(toLineData(times, e21))
  } else {
    ind.ema12 = removeSeriesSafe(ind.ema12)
    ind.ema21 = removeSeriesSafe(ind.ema21)
  }

  if (flags.ichimoku) {
    const { tenkan, kijun, spanA, senkouB, chikou } = ichimokuValues(highs, lows, closes)
    if (!ind.ichTenkan) {
      ind.ichTenkan = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', title: '转换' },
        0,
      )
    }
    if (!ind.ichKijun) {
      ind.ichKijun = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', title: '基准' },
        0,
      )
    }
    if (!ind.ichSpanA) {
      ind.ichSpanA = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineStyle: LineStyle.Dashed, title: '先行A' },
        0,
      )
    }
    if (!ind.ichSpanB) {
      ind.ichSpanB = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineStyle: LineStyle.Dashed, title: '先行B' },
        0,
      )
    }
    if (!ind.ichChikou) {
      ind.ichChikou = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#a855f7', lineWidth: 1, lineStyle: LineStyle.Dotted, title: '迟行' },
        0,
      )
    }
    ind.ichTenkan.setData(toLineData(times, tenkan))
    ind.ichKijun.setData(toLineData(times, kijun))
    ind.ichSpanA.setData(toLineData(times, spanA))
    ind.ichSpanB.setData(toLineData(times, senkouB))
    ind.ichChikou.setData(toLineData(times, chikou))
  } else {
    ind.ichTenkan = removeSeriesSafe(ind.ichTenkan)
    ind.ichKijun = removeSeriesSafe(ind.ichKijun)
    ind.ichSpanA = removeSeriesSafe(ind.ichSpanA)
    ind.ichSpanB = removeSeriesSafe(ind.ichSpanB)
    ind.ichChikou = removeSeriesSafe(ind.ichChikou)
  }

  if (flags.sar) {
    const { sar, direction } = sarValues(highs, lows, closes, 0.02, 0.2)
    if (!ind.sar) {
      ind.sar = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 3,
          title: 'SAR',
        },
        0,
      )
    }
    const sarData = []
    for (let i = 0; i < times.length; i++) {
      if (sar[i] != null) {
        sarData.push({
          time: times[i],
          value: sar[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.sar.setData(sarData)
  } else {
    ind.sar = removeSeriesSafe(ind.sar)
  }

  if (flags.donchian) {
    const { upper, mid, lower } = donchianChannelValues(highs, lows, 20)
    if (!ind.donchianU) {
      ind.donchianU = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#f97316', lineStyle: LineStyle.Dashed, title: 'DC上' },
        0,
      )
    }
    if (!ind.donchianM) {
      ind.donchianM = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#fb923c', lineStyle: LineStyle.Dotted, title: 'DC中' },
        0,
      )
    }
    if (!ind.donchianL) {
      ind.donchianL = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#f97316', lineStyle: LineStyle.Dashed, title: 'DC下' },
        0,
      )
    }
    ind.donchianU.setData(toLineData(times, upper))
    ind.donchianM.setData(toLineData(times, mid))
    ind.donchianL.setData(toLineData(times, lower))
  } else {
    ind.donchianU = removeSeriesSafe(ind.donchianU)
    ind.donchianM = removeSeriesSafe(ind.donchianM)
    ind.donchianL = removeSeriesSafe(ind.donchianL)
  }

  if (flags.pivot) {
    const { pp, s1, s2, r1, r2 } = pivotPointsValues(highs, lows, closes)
    if (!ind.pivotPP) {
      ind.pivotPP = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#a3a3a3', lineStyle: LineStyle.Dotted, title: 'PP' },
        0,
      )
    }
    if (!ind.pivotS1) {
      ind.pivotS1 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineStyle: LineStyle.Dashed, title: 'S1' },
        0,
      )
    }
    if (!ind.pivotS2) {
      ind.pivotS2 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#16a34a', lineStyle: LineStyle.Dashed, title: 'S2' },
        0,
      )
    }
    if (!ind.pivotR1) {
      ind.pivotR1 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineStyle: LineStyle.Dashed, title: 'R1' },
        0,
      )
    }
    if (!ind.pivotR2) {
      ind.pivotR2 = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#dc2626', lineStyle: LineStyle.Dashed, title: 'R2' },
        0,
      )
    }
    ind.pivotPP.setData(toLineData(times, pp))
    ind.pivotS1.setData(toLineData(times, s1))
    ind.pivotS2.setData(toLineData(times, s2))
    ind.pivotR1.setData(toLineData(times, r1))
    ind.pivotR2.setData(toLineData(times, r2))
  } else {
    ind.pivotPP = removeSeriesSafe(ind.pivotPP)
    ind.pivotS1 = removeSeriesSafe(ind.pivotS1)
    ind.pivotS2 = removeSeriesSafe(ind.pivotS2)
    ind.pivotR1 = removeSeriesSafe(ind.pivotR1)
    ind.pivotR2 = removeSeriesSafe(ind.pivotR2)
  }

  if (flags.dema) {
    const d = demaValues(closes, 21)
    if (!ind.dema) {
      ind.dema = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#ec4899', title: 'DEMA21' },
        0,
      )
    }
    ind.dema.setData(toLineData(times, d))
  } else {
    ind.dema = removeSeriesSafe(ind.dema)
  }

  if (flags.zigzag) {
    const { zigzag, directions } = zigzagValues(highs, lows, closes, 5)
    if (!ind.zigzag) {
      ind.zigzag = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 2,
          lineStyle: LineStyle.Dashed,
          color: '#f59e0b',
          pointMarkersVisible: true,
          pointMarkersRadius: 4,
          title: 'ZigZag',
        },
        0,
      )
    }
    const zzData = []
    for (let i = 0; i < times.length; i++) {
      if (zigzag[i] != null) {
        zzData.push({
          time: times[i],
          value: zigzag[i],
          color: directions[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.zigzag.setData(zzData)
  } else {
    ind.zigzag = removeSeriesSafe(ind.zigzag)
  }

  if (flags.sats) {
    const { stLine, upper, lower, direction } = satsValues(highs, lows, closes, vols)
    if (!ind.satsLine) {
      ind.satsLine = getChart().addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 2, title: 'SATS' },
        0,
      )
    }
    const satsData = []
    for (let i = 0; i < times.length; i++) {
      if (stLine[i] != null) {
        satsData.push({
          time: times[i],
          value: stLine[i],
          color: direction[i] === 1 ? '#ef4444' : '#22c55e',
        })
      }
    }
    ind.satsLine.setData(satsData)
    if (!ind.satsUpper) {
      ind.satsUpper = getChart().addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 1, lineStyle: LineStyle.Dashed, color: 'rgba(148,163,184,0.35)', title: 'SATS上' },
        0,
      )
    }
    if (!ind.satsLower) {
      ind.satsLower = getChart().addSeries(
        LineSeries,
        { ...lineCommon, lineWidth: 1, lineStyle: LineStyle.Dashed, color: 'rgba(148,163,184,0.35)', title: 'SATS下' },
        0,
      )
    }
    ind.satsUpper.setData(toLineData(times, upper))
    ind.satsLower.setData(toLineData(times, lower))
  } else {
    ind.satsLine = removeSeriesSafe(ind.satsLine)
    ind.satsUpper = removeSeriesSafe(ind.satsUpper)
    ind.satsLower = removeSeriesSafe(ind.satsLower)
  }

  if (flags.alligator) {
    const { jaw, teeth, lips } = alligatorValues(highs, lows, closes)
    if (!ind.alligatorJaw) {
      ind.alligatorJaw = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#22c55e', lineWidth: 1, lineStyle: LineStyle.Dashed, title: '颚(13)' },
        0,
      )
    }
    if (!ind.alligatorTeeth) {
      ind.alligatorTeeth = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#ef4444', lineWidth: 1, lineStyle: LineStyle.Dashed, title: '齿(8)' },
        0,
      )
    }
    if (!ind.alligatorLips) {
      ind.alligatorLips = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#3b82f6', lineWidth: 1, title: '唇(5)' },
        0,
      )
    }
    ind.alligatorJaw.setData(toLineData(times, jaw))
    ind.alligatorTeeth.setData(toLineData(times, teeth))
    ind.alligatorLips.setData(toLineData(times, lips))
  } else {
    ind.alligatorJaw = removeSeriesSafe(ind.alligatorJaw)
    ind.alligatorTeeth = removeSeriesSafe(ind.alligatorTeeth)
    ind.alligatorLips = removeSeriesSafe(ind.alligatorLips)
  }

  if (flags.hullMA) {
    const hull = hullMaValues(closes, 9)
    if (!ind.hullMA) {
      ind.hullMA = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#f59e0b', lineWidth: 2, title: 'Hull(9)' },
        0,
      )
    }
    ind.hullMA.setData(toLineData(times, hull))
  } else {
    ind.hullMA = removeSeriesSafe(ind.hullMA)
  }

  if (flags.fractal) {
    const { fractalHigh, fractalLow } = fractalValues(highs, lows)
    if (!ind.fractalHigh) {
      ind.fractalHigh = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 5,
          title: '▲Fractal',
          color: '#ef4444',
        },
        0,
      )
    }
    if (!ind.fractalLow) {
      ind.fractalLow = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          lineWidth: 0,
          pointMarkersVisible: true,
          pointMarkersRadius: 5,
          title: '▼Fractal',
          color: '#22c55e',
        },
        0,
      )
    }
    ind.fractalHigh.setData(toLineData(times, fractalHigh))
    ind.fractalLow.setData(toLineData(times, fractalLow))
  } else {
    ind.fractalHigh = removeSeriesSafe(ind.fractalHigh)
    ind.fractalLow = removeSeriesSafe(ind.fractalLow)
  }

  if (flags.vwapBands) {
    const { vwap: vbM, upper: vbU, lower: vbL } = vwapBandsValues(highs, lows, closes, vols)
    if (!ind.vwapBandsU) {
      ind.vwapBandsU = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'VB上',
        },
        0,
      )
    }
    if (!ind.vwapBandsM) {
      ind.vwapBandsM = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#8b5cf6', title: 'VWAP' },
        0,
      )
    }
    if (!ind.vwapBandsL) {
      ind.vwapBandsL = getChart().addSeries(
        LineSeries,
        {
          ...lineCommon,
          color: '#a78bfa',
          lineStyle: LineStyle.Dashed,
          title: 'VB下',
        },
        0,
      )
    }
    ind.vwapBandsU.setData(toLineData(times, vbU))
    ind.vwapBandsM.setData(toLineData(times, vbM))
    ind.vwapBandsL.setData(toLineData(times, vbL))
  } else {
    ind.vwapBandsU = removeSeriesSafe(ind.vwapBandsU)
    ind.vwapBandsM = removeSeriesSafe(ind.vwapBandsM)
    ind.vwapBandsL = removeSeriesSafe(ind.vwapBandsL)
  }

  if (flags.tema) {
    const tema = temaValues(closes, 21)
    if (!ind.temaLine) {
      ind.temaLine = getChart().addSeries(
        LineSeries,
        { ...lineCommon, color: '#06b6d4', lineWidth: 2, title: 'TEMA(21)' },
        0,
      )
    }
    ind.temaLine.setData(toLineData(times, tema))
  } else {
    ind.temaLine = removeSeriesSafe(ind.temaLine)
  }

  if (flags.smc) {
    const smc = smcValues(highs, lows, closes, opens)
    if (!ind.smcSwingHigh) {
      ind.smcSwingHigh = getChart().addSeries(LineSeries, { ...lineCommon, color: '#ef4444', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 6, title: 'SwH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcSwingLow = getChart().addSeries(LineSeries, { ...lineCommon, color: '#22c55e', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 6, title: 'SwL', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcIntHigh = getChart().addSeries(LineSeries, { ...lineCommon, color: '#f87171', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'iH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcIntLow = getChart().addSeries(LineSeries, { ...lineCommon, color: '#4ade80', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'iL', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcBos = getChart().addSeries(LineSeries, { ...lineCommon, color: '#3b82f6', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 4, title: 'BOS', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcChoch = getChart().addSeries(LineSeries, { ...lineCommon, color: '#f59e0b', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 4, title: 'CHoCH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcSwBos = getChart().addSeries(LineSeries, { ...lineCommon, color: '#6366f1', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 5, title: 'SwBOS', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcSwChoch = getChart().addSeries(LineSeries, { ...lineCommon, color: '#eab308', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 5, title: 'SwCHoCH', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcFvgTop = getChart().addSeries(LineSeries, { ...lineCommon, color: '#ef4444', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 2, title: 'FVG↑', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcFvgBot = getChart().addSeries(LineSeries, { ...lineCommon, color: '#22c55e', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 2, title: 'FVG↓', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcObTop = getChart().addSeries(LineSeries, { ...lineCommon, color: '#ef4444', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'OB↑', lastValueVisible: false, priceLineVisible: false }, 0)
      ind.smcObBot = getChart().addSeries(LineSeries, { ...lineCommon, color: '#22c55e', lineWidth: 0, pointMarkersVisible: true, pointMarkersRadius: 3, title: 'OB↓', lastValueVisible: false, priceLineVisible: false }, 0)
    }
    ind.smcSwingHigh.setData(smc.swingHighPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcSwingLow.setData(smc.swingLowPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcIntHigh.setData(smc.intHighPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcIntLow.setData(smc.intLowPoints.map(p => ({ time: times[p.idx], value: p.price })).sort((a, b) => a.time - b.time))
    ind.smcBos.setData(smc.bosLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    ind.smcChoch.setData(smc.chochLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    ind.smcSwBos.setData(smc.swingBosLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    ind.smcSwChoch.setData(smc.swingChochLines.map(b => ({ time: times[b.toIdx], value: b.toPrice })).sort((a, b) => a.time - b.time))
    const fvgTopData = smc.fvgZones.filter(z => !z.mitigated).map(z => ({ time: times[z.startIdx], value: z.top })).sort((a, b) => a.time - b.time)
    const fvgBotData = smc.fvgZones.filter(z => !z.mitigated).map(z => ({ time: times[z.startIdx], value: z.bot })).sort((a, b) => a.time - b.time)
    ind.smcFvgTop.setData(fvgTopData)
    ind.smcFvgBot.setData(fvgBotData)
    const obTopData = smc.orderBlocks.filter(o => !o.mitigated).map(o => ({ time: times[o.idx], value: o.top })).sort((a, b) => a.time - b.time)
    const obBotData = smc.orderBlocks.filter(o => !o.mitigated).map(o => ({ time: times[o.idx], value: o.bot })).sort((a, b) => a.time - b.time)
    ind.smcObTop.setData(obTopData)
    ind.smcObBot.setData(obBotData)
  } else {
    ind.smcSwingHigh = removeSeriesSafe(ind.smcSwingHigh)
    ind.smcSwingLow = removeSeriesSafe(ind.smcSwingLow)
    ind.smcIntHigh = removeSeriesSafe(ind.smcIntHigh)
    ind.smcIntLow = removeSeriesSafe(ind.smcIntLow)
    ind.smcBos = removeSeriesSafe(ind.smcBos)
    ind.smcChoch = removeSeriesSafe(ind.smcChoch)
    ind.smcSwBos = removeSeriesSafe(ind.smcSwBos)
    ind.smcSwChoch = removeSeriesSafe(ind.smcSwChoch)
    ind.smcFvgTop = removeSeriesSafe(ind.smcFvgTop)
    ind.smcFvgBot = removeSeriesSafe(ind.smcFvgBot)
    ind.smcObTop = removeSeriesSafe(ind.smcObTop)
    ind.smcObBot = removeSeriesSafe(ind.smcObBot)
  }

  syncSubPaneIndicators(times, closes, highs, lows, vols)
}

function evaluateIndicatorSignals(endIdx) {
  const rows = getMergedRawRows()
  if (!rows || rows.length < 2) return []

  // 截取到 endIdx（含），使指标计算基于该 K 线位置的数据
  const sliced = endIdx != null && endIdx >= 0 && endIdx < rows.length - 1
    ? rows.slice(0, endIdx + 1)
    : rows
  const { times, closes, highs, lows, vols } = extractOHLCV(sliced)
  const n = times.length
  if (n < 2) return []

  const last = (arr) => {
    for (let i = arr.length - 1; i >= 0; i--) {
      if (arr[i] != null && Number.isFinite(arr[i])) return arr[i]
    }
    return null
  }
  const prev = (arr) => {
    let cnt = 0
    for (let i = arr.length - 1; i >= 0; i--) {
      if (arr[i] != null && Number.isFinite(arr[i])) {
        cnt++
        if (cnt === 2) return arr[i]
      }
    }
    return null
  }
  const signals = []

  // ── Trend indicators ──

  // MA
  {
    const m5 = smaValues(closes, 5)
    const m10 = smaValues(closes, 10)
    const m20 = smaValues(closes, 20)
    const m60 = smaValues(closes, 60)
    const v5 = last(m5), v10 = last(m10), v20 = last(m20), v60 = last(m60)
    if (v5 != null && v10 != null && v20 != null && v60 != null) {
      if (v5 > v10 && v10 > v20 && v20 > v60) signals.push({ name: 'MA', signal: 'bullish' })
      else if (v5 < v10 && v10 < v20 && v20 < v60) signals.push({ name: 'MA', signal: 'bearish' })
      else if ((v5 > v20 && v10 < v60) || (v5 < v20 && v10 > v60)) signals.push({ name: 'MA', signal: 'oscillating' })
      else signals.push({ name: 'MA', signal: 'neutral' })
    }
  }

  // EMA
  {
    const e12 = emaFinite(closes, 12)
    const e21 = emaFinite(closes, 21)
    const v12 = last(e12), v21 = last(e21)
    if (v12 != null && v21 != null) {
      if (v12 > v21) signals.push({ name: 'EMA', signal: 'bullish' })
      else if (v12 < v21) signals.push({ name: 'EMA', signal: 'bearish' })
      else signals.push({ name: 'EMA', signal: 'neutral' })
    }
  }

  // BOLL
  {
    const { upper, mid, lower } = bollingerBands(closes, 20, 2)
    const vU = last(upper), vM = last(mid), vL = last(lower), c = closes[n - 1]
    if (vU != null && vM != null && vL != null) {
      if (c > vU) signals.push({ name: 'BOLL', signal: 'bullish' })
      else if (c < vL) signals.push({ name: 'BOLL', signal: 'bearish' })
      else if (c > vM) signals.push({ name: 'BOLL', signal: 'oscillating' })
      else signals.push({ name: 'BOLL', signal: 'neutral' })
    }
  }

  // VWAP
  {
    const vw = vwapValues(highs, lows, closes, vols, 20)
    const v = last(vw), c = closes[n - 1]
    if (v != null) {
      if (c > v) signals.push({ name: 'VWAP', signal: 'bullish' })
      else if (c < v) signals.push({ name: 'VWAP', signal: 'bearish' })
      else signals.push({ name: 'VWAP', signal: 'neutral' })
    }
  }

  // DEMA
  {
    const d = demaValues(closes, 21)
    const v = last(d), c = closes[n - 1]
    if (v != null) {
      if (c > v) signals.push({ name: 'DEMA', signal: 'bullish' })
      else if (c < v) signals.push({ name: 'DEMA', signal: 'bearish' })
      else signals.push({ name: 'DEMA', signal: 'neutral' })
    }
  }

  // TEMA
  {
    const t = temaValues(closes, 21)
    const v = last(t), c = closes[n - 1]
    if (v != null) {
      if (c > v) signals.push({ name: 'TEMA', signal: 'bullish' })
      else if (c < v) signals.push({ name: 'TEMA', signal: 'bearish' })
      else signals.push({ name: 'TEMA', signal: 'neutral' })
    }
  }

  // KAMA
  {
    const k = kamaValues(closes, 10, 2, 30)
    const v = last(k), c = closes[n - 1], pv = prev(k)
    if (v != null && pv != null) {
      if (c > v && v > pv) signals.push({ name: 'KAMA', signal: 'bullish' })
      else if (c < v && v < pv) signals.push({ name: 'KAMA', signal: 'bearish' })
      else signals.push({ name: 'KAMA', signal: 'neutral' })
    }
  }

  // HullMA
  {
    const h = hullMaValues(closes, 9)
    const v = last(h), pv = prev(h)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'HullMA', signal: 'bullish' })
      else if (v < pv) signals.push({ name: 'HullMA', signal: 'bearish' })
      else signals.push({ name: 'HullMA', signal: 'neutral' })
    }
  }

  // Keltner
  {
    const { upper: kU, lower: kL } = keltnerChannelValues(highs, lows, closes, 20, 10, 1.5)
    const vU = last(kU), vL = last(kL), c = closes[n - 1]
    if (vU != null && vL != null) {
      if (c > vU) signals.push({ name: 'Keltner', signal: 'bullish' })
      else if (c < vL) signals.push({ name: 'Keltner', signal: 'bearish' })
      else signals.push({ name: 'Keltner', signal: 'oscillating' })
    }
  }

  // SuperTrend
  {
    const { direction } = supertrendValues(highs, lows, closes, 10, 3)
    const d = last(direction)
    if (d != null) {
      if (d === 1) signals.push({ name: 'SuperTrend', signal: 'bullish' })
      else if (d === -1) signals.push({ name: 'SuperTrend', signal: 'bearish' })
      else signals.push({ name: 'SuperTrend', signal: 'neutral' })
    }
  }

  // Ichimoku
  {
    const { tenkan, kijun, spanA, senkouB } = ichimokuValues(highs, lows, closes)
    const vT = last(tenkan), vK = last(kijun), vSA = last(spanA), vSB = last(senkouB), c = closes[n - 1]
    if (vT != null && vK != null && vSA != null && vSB != null) {
      const cloudTop = Math.max(vSA, vSB)
      const cloudBot = Math.min(vSA, vSB)
      if (c > cloudTop && vT > vK) signals.push({ name: 'Ichimoku', signal: 'bullish' })
      else if (c < cloudBot && vT < vK) signals.push({ name: 'Ichimoku', signal: 'bearish' })
      else if (c >= cloudBot && c <= cloudTop) signals.push({ name: 'Ichimoku', signal: 'oscillating' })
      else signals.push({ name: 'Ichimoku', signal: 'neutral' })
    }
  }

  // SAR
  {
    const { direction } = sarValues(highs, lows, closes, 0.02, 0.2)
    const d = last(direction)
    if (d != null) {
      if (d === 1) signals.push({ name: 'SAR', signal: 'bullish' })
      else if (d === -1) signals.push({ name: 'SAR', signal: 'bearish' })
      else signals.push({ name: 'SAR', signal: 'neutral' })
    }
  }

  // Donchian
  {
    const { upper, lower } = donchianChannelValues(highs, lows, 20)
    const vU = last(upper), vL = last(lower), c = closes[n - 1]
    if (vU != null && vL != null) {
      if (c >= vU) signals.push({ name: 'Donchian', signal: 'bullish' })
      else if (c <= vL) signals.push({ name: 'Donchian', signal: 'bearish' })
      else signals.push({ name: 'Donchian', signal: 'oscillating' })
    }
  }

  // Alligator
  {
    const { jaw, teeth, lips } = alligatorValues(highs, lows, closes)
    const vJ = last(jaw), vT = last(teeth), vL = last(lips)
    if (vJ != null && vT != null && vL != null) {
      if (vL > vT && vT > vJ) signals.push({ name: 'Alligator', signal: 'bullish' })
      else if (vL < vT && vT < vJ) signals.push({ name: 'Alligator', signal: 'bearish' })
      else signals.push({ name: 'Alligator', signal: 'oscillating' })
    }
  }

  // ZigZag
  {
    const { directions } = zigzagValues(highs, lows, closes, 5)
    let zzDir = 0
    for (let i = directions.length - 1; i >= 0; i--) {
      if (directions[i] === 1 || directions[i] === -1) { zzDir = directions[i]; break }
    }
    if (zzDir === -1) signals.push({ name: 'ZigZag', signal: 'bullish' })
    else if (zzDir === 1) signals.push({ name: 'ZigZag', signal: 'bearish' })
    else signals.push({ name: 'ZigZag', signal: 'neutral' })
  }

  // SATS
  {
    const { direction } = satsValues(highs, lows, closes, vols)
    const d = last(direction)
    if (d != null) {
      if (d === 1) signals.push({ name: 'SATS', signal: 'bullish' })
      else if (d === -1) signals.push({ name: 'SATS', signal: 'bearish' })
      else signals.push({ name: 'SATS', signal: 'neutral' })
    }
  }

  // Pivot
  {
    const { pp, s1, r1 } = pivotPointsValues(highs, lows, closes)
    const vPP = last(pp), vS1 = last(s1), vR1 = last(r1), c = closes[n - 1]
    if (vPP != null && vR1 != null && vS1 != null) {
      if (c > vR1) signals.push({ name: 'Pivot', signal: 'bullish' })
      else if (c < vS1) signals.push({ name: 'Pivot', signal: 'bearish' })
      else if (c > vPP) signals.push({ name: 'Pivot', signal: 'oscillating' })
      else signals.push({ name: 'Pivot', signal: 'neutral' })
    }
  }

  // VWAPBands
  {
    const { upper: vbU, lower: vbL } = vwapBandsValues(highs, lows, closes, vols)
    const vU = last(vbU), vL = last(vbL), c = closes[n - 1]
    if (vU != null && vL != null) {
      if (c > vU) signals.push({ name: 'VWAPBands', signal: 'bullish' })
      else if (c < vL) signals.push({ name: 'VWAPBands', signal: 'bearish' })
      else signals.push({ name: 'VWAPBands', signal: 'oscillating' })
    }
  }

  // ── Momentum / oscillator indicators ──

  // MACD
  {
    const { dif, dea, hist } = macdBundle(closes)
    const vDif = last(dif), vDea = last(dea), vHist = last(hist)
    if (vDif != null && vDea != null && vHist != null) {
      if (vDif > vDea && vHist > 0) signals.push({ name: 'MACD', signal: 'bullish' })
      else if (vDif < vDea && vHist < 0) signals.push({ name: 'MACD', signal: 'bearish' })
      else if (vDif > 0 && vHist < 0) signals.push({ name: 'MACD', signal: 'oscillating' })
      else if (vDif < 0 && vHist > 0) signals.push({ name: 'MACD', signal: 'oscillating' })
      else signals.push({ name: 'MACD', signal: 'neutral' })
    }
  }

  // RSI
  {
    const rsi = rsiBundle(closes, 14)
    const v = last(rsi)
    if (v != null) {
      if (v > 70) signals.push({ name: 'RSI', signal: 'oscillating' })
      else if (v < 30) signals.push({ name: 'RSI', signal: 'oscillating' })
      else if (v > 50) signals.push({ name: 'RSI', signal: 'bullish' })
      else signals.push({ name: 'RSI', signal: 'bearish' })
    }
  }

  // KDJ
  {
    const { K, D, J } = kdjBundle(highs, lows, closes, 9)
    const vK = last(K), vD = last(D), vJ = last(J)
    if (vK != null && vD != null && vJ != null) {
      if (vJ > vK && vK > vD && vK < 80) signals.push({ name: 'KDJ', signal: 'bullish' })
      else if (vJ < vK && vK < vD && vK > 20) signals.push({ name: 'KDJ', signal: 'bearish' })
      else if (vK > 80) signals.push({ name: 'KDJ', signal: 'bearish' })
      else if (vK < 20) signals.push({ name: 'KDJ', signal: 'bullish' })
      else signals.push({ name: 'KDJ', signal: 'oscillating' })
    }
  }

  // CCI
  {
    const cci = cciValues(highs, lows, closes, 20)
    const v = last(cci)
    if (v != null) {
      if (v > 100) signals.push({ name: 'CCI', signal: 'bullish' })
      else if (v < -100) signals.push({ name: 'CCI', signal: 'bearish' })
      else signals.push({ name: 'CCI', signal: 'oscillating' })
    }
  }

  // Williams %R
  {
    const wr = williamsRValues(highs, lows, closes, 14)
    const v = last(wr)
    if (v != null) {
      if (v < -80) signals.push({ name: 'W%R', signal: 'bullish' })
      else if (v > -20) signals.push({ name: 'W%R', signal: 'bearish' })
      else signals.push({ name: 'W%R', signal: 'oscillating' })
    }
  }

  // StochRSI
  {
    const { k, d } = stochRsiValues(closes, 14, 14, 3, 3)
    const vK = last(k), vD = last(d)
    if (vK != null && vD != null) {
      if (vK < 20 && vD < 20 && vK > vD) signals.push({ name: 'StochRSI', signal: 'bullish' })
      else if (vK > 80 && vD > 80 && vK < vD) signals.push({ name: 'StochRSI', signal: 'bearish' })
      else signals.push({ name: 'StochRSI', signal: 'oscillating' })
    }
  }

  // ADX
  {
    const { adx, diP, diM } = adxValues(highs, lows, closes, 14)
    const vAdx = last(adx), vP = last(diP), vM = last(diM)
    if (vAdx != null && vP != null && vM != null) {
      if (vAdx > 25 && vP > vM) signals.push({ name: 'ADX', signal: 'bullish' })
      else if (vAdx > 25 && vP < vM) signals.push({ name: 'ADX', signal: 'bearish' })
      else signals.push({ name: 'ADX', signal: 'oscillating' })
    }
  }

  // Aroon
  {
    const { up, down } = aroonValues(highs, lows, 25)
    const vU = last(up), vD = last(down)
    if (vU != null && vD != null) {
      if (vU > 70 && vD < 30) signals.push({ name: 'Aroon', signal: 'bullish' })
      else if (vD > 70 && vU < 30) signals.push({ name: 'Aroon', signal: 'bearish' })
      else signals.push({ name: 'Aroon', signal: 'oscillating' })
    }
  }

  // CMO
  {
    const cmo = cmoValues(closes, 14)
    const v = last(cmo)
    if (v != null) {
      if (v > 50) signals.push({ name: 'CMO', signal: 'bullish' })
      else if (v < -50) signals.push({ name: 'CMO', signal: 'bearish' })
      else signals.push({ name: 'CMO', signal: 'oscillating' })
    }
  }

  // TRIX
  {
    const trix = trixValues(closes, 15)
    const signal = emaLeadingNull(trix, 9)
    const vT = last(trix), vS = last(signal)
    if (vT != null && vS != null) {
      if (vT > vS) signals.push({ name: 'TRIX', signal: 'bullish' })
      else if (vT < vS) signals.push({ name: 'TRIX', signal: 'bearish' })
      else signals.push({ name: 'TRIX', signal: 'neutral' })
    }
  }

  // ROC
  {
    const roc = rocValues(closes, 12)
    const v = last(roc)
    if (v != null) {
      if (v > 0) signals.push({ name: 'ROC', signal: 'bullish' })
      else if (v < 0) signals.push({ name: 'ROC', signal: 'bearish' })
      else signals.push({ name: 'ROC', signal: 'neutral' })
    }
  }

  // Coppock
  {
    const cp = coppockValues(closes)
    const v = last(cp), pv = prev(cp)
    if (v != null && pv != null) {
      if (v > 0 && pv <= 0) signals.push({ name: 'Coppock', signal: 'bullish' })
      else if (v < 0) signals.push({ name: 'Coppock', signal: 'bearish' })
      else signals.push({ name: 'Coppock', signal: 'neutral' })
    }
  }

  // SMI
  {
    const { smi: smiData, signal: smiSig } = smiValues(highs, lows, closes)
    const vSmi = last(smiData), vSig = last(smiSig)
    if (vSmi != null && vSig != null) {
      if (vSmi > vSig && vSmi > 0) signals.push({ name: 'SMI', signal: 'bullish' })
      else if (vSmi < vSig && vSmi < 0) signals.push({ name: 'SMI', signal: 'bearish' })
      else signals.push({ name: 'SMI', signal: 'oscillating' })
    }
  }

  // AO
  {
    const ao = aoValues(highs, lows)
    const v = last(ao), pv = prev(ao)
    if (v != null && pv != null) {
      if (v > 0 && v > pv) signals.push({ name: 'AO', signal: 'bullish' })
      else if (v < 0 && v < pv) signals.push({ name: 'AO', signal: 'bearish' })
      else if (v > 0 && v < pv) signals.push({ name: 'AO', signal: 'oscillating' })
      else signals.push({ name: 'AO', signal: 'neutral' })
    }
  }

  // ── Volume indicators ──

  // OBV
  {
    const obv = obvValues(closes, vols)
    const v = last(obv), pv = prev(obv)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'OBV', signal: 'bullish' })
      else if (v < pv) signals.push({ name: 'OBV', signal: 'bearish' })
      else signals.push({ name: 'OBV', signal: 'neutral' })
    }
  }

  // MFI
  {
    const mfi = mfiValues(highs, lows, closes, vols, 14)
    const v = last(mfi)
    if (v != null) {
      if (v > 80) signals.push({ name: 'MFI', signal: 'oscillating' })
      else if (v < 20) signals.push({ name: 'MFI', signal: 'oscillating' })
      else if (v > 50) signals.push({ name: 'MFI', signal: 'bullish' })
      else signals.push({ name: 'MFI', signal: 'bearish' })
    }
  }

  // CMF
  {
    const cmf = cmfValues(highs, lows, closes, vols, 20)
    const v = last(cmf)
    if (v != null) {
      if (v > 0.05) signals.push({ name: 'CMF', signal: 'bullish' })
      else if (v < -0.05) signals.push({ name: 'CMF', signal: 'bearish' })
      else signals.push({ name: 'CMF', signal: 'oscillating' })
    }
  }

  // A/D
  {
    const ad = adValues(highs, lows, closes, vols)
    const v = last(ad), pv = prev(ad)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'A/D', signal: 'bullish' })
      else if (v < pv) signals.push({ name: 'A/D', signal: 'bearish' })
      else signals.push({ name: 'A/D', signal: 'neutral' })
    }
  }

  // ForceIndex
  {
    const fi = forceIndexValues(closes, vols, 13)
    const v = last(fi)
    if (v != null) {
      if (v > 0) signals.push({ name: 'FI', signal: 'bullish' })
      else if (v < 0) signals.push({ name: 'FI', signal: 'bearish' })
      else signals.push({ name: 'FI', signal: 'neutral' })
    }
  }

  // ChaikinOsc
  {
    const co = chaikinOscValues(highs, lows, closes, vols, 3, 10)
    const v = last(co), pv = prev(co)
    if (v != null && pv != null) {
      if (v > 0 && v > pv) signals.push({ name: 'ChaikinOsc', signal: 'bullish' })
      else if (v < 0 && v < pv) signals.push({ name: 'ChaikinOsc', signal: 'bearish' })
      else signals.push({ name: 'ChaikinOsc', signal: 'oscillating' })
    }
  }

  // ── Volatility indicators ──

  // ATR
  {
    const atr = atrValues(highs, lows, closes, 14)
    const v = last(atr), pv = prev(atr)
    if (v != null && pv != null) {
      if (v > pv) signals.push({ name: 'ATR', signal: 'oscillating' })
      else signals.push({ name: 'ATR', signal: 'neutral' })
    }
  }

  // CHOP
  {
    const chop = chopValues(highs, lows, closes, 14)
    const v = last(chop)
    if (v != null) {
      if (v > 61.8) signals.push({ name: 'CHOP', signal: 'oscillating' })
      else signals.push({ name: 'CHOP', signal: 'neutral' })
    }
  }

  // MassIndex
  {
    const mi = massIndexValues(highs, lows)
    const v = last(mi), pv = prev(mi)
    if (v != null && pv != null) {
      if (pv > 27 && v < 27) signals.push({ name: 'MassIndex', signal: 'bullish' })
      else signals.push({ name: 'MassIndex', signal: 'neutral' })
    }
  }

  // UlcerIndex
  {
    const ui = ulcerIndexValues(closes)
    const v = last(ui)
    if (v != null) {
      if (v < 5) signals.push({ name: 'UlcerIndex', signal: 'bullish' })
      else if (v > 15) signals.push({ name: 'UlcerIndex', signal: 'bearish' })
      else signals.push({ name: 'UlcerIndex', signal: 'neutral' })
    }
  }

  // TTMSqueeze
  {
    const { squeeze, momentum } = ttmSqueezeValues(highs, lows, closes)
    const sq = last(squeeze), mo = last(momentum)
    if (sq != null && mo != null) {
      if (!sq && mo > 0) signals.push({ name: 'TTM', signal: 'bullish' })
      else if (!sq && mo < 0) signals.push({ name: 'TTM', signal: 'bearish' })
      else if (sq) signals.push({ name: 'TTM', signal: 'oscillating' })
      else signals.push({ name: 'TTM', signal: 'neutral' })
    }
  }

  // ElderRay
  {
    const { bullPower, bearPower } = elderRayValues(highs, lows, closes, 13)
    const bP = last(bullPower), brP = last(bearPower)
    if (bP != null && brP != null) {
      if (bP > 0 && bP > brP) signals.push({ name: 'ElderRay', signal: 'bullish' })
      else if (brP < 0 && brP < bP) signals.push({ name: 'ElderRay', signal: 'bearish' })
      else signals.push({ name: 'ElderRay', signal: 'oscillating' })
    }
  }

  return signals
}

const indicatorSignalSummary = computed(() => {
  // 依赖 mergedRawRowsVersion 以感知 mergedRawRows 变更
  void mergedRawRowsVersion.value
  // 根据 hoverRawRow 计算当前光标对应的 K 线索引
  const hoveredRow = hoverRawRow.value
  let endIdx = null
  if (hoveredRow) {
    const curDay = String(hoveredRow.day || '').replace(/\//g, '-')
    const idx = getMergedRawRows().findIndex(x => String(x.day || '').replace(/\//g, '-') === curDay)
    if (idx >= 0) endIdx = idx
  }
  const sigs = evaluateIndicatorSignals(endIdx)
  if (!sigs.length) return null
  const counts = { bullish: 0, bearish: 0, neutral: 0, oscillating: 0 }
  for (const s of sigs) {
    if (counts[s.signal] !== undefined) counts[s.signal]++
  }
  const total = sigs.length
  return {
    total,
    bullish: counts.bullish,
    bearish: counts.bearish,
    neutral: counts.neutral,
    oscillating: counts.oscillating,
    bullishPct: total > 0 ? Math.round((counts.bullish / total) * 100) : 0,
    bearishPct: total > 0 ? Math.round((counts.bearish / total) * 100) : 0,
    neutralPct: total > 0 ? Math.round((counts.neutral / total) * 100) : 0,
    oscillatingPct: total > 0 ? Math.round((counts.oscillating / total) * 100) : 0,
    signals: sigs,
  }
})


  return { flags, ind, syncIndicators, indicatorSignalSummary, toggleIndicator, resetInd }
}
