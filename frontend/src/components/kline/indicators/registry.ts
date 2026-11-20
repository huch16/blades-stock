// K 线指标工具栏注册表：由 StockLightweightKlineChart.vue 的侧边栏逐字解析生成
// （R-22a 数据驱动改造；label / tip / 分组 / 标题样式均与原模板逐字一致）

export interface KlineIndicatorItem {
  key: string
  label: string
  tip: string
}

export interface KlineIndicatorSection {
  key: string
  title: string
  style: string
  items: KlineIndicatorItem[]
}

export const KLINE_INDICATOR_SECTIONS: KlineIndicatorSection[] = [
  {
    key: 'trend',
    title: '📈趋势',
    style: 'font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(239,68,68,0.08); border-radius: 4px; border-left: 3px solid #ef4444; color: #ef4444',
    items: [
      { key: 'ma', label: 'MA', tip: 'ma' },
      { key: 'ema', label: 'EMA', tip: 'ema' },
      { key: 'kama', label: 'KAMA', tip: 'kama' },
      { key: 'supertrend', label: 'STrend', tip: 'supertrend' },
      { key: 'sar', label: 'SAR', tip: 'sar' },
      { key: 'ichimoku', label: 'Ichi', tip: 'ichimoku' },
      { key: 'aroon', label: 'Aroon', tip: 'aroon' },
      { key: 'dema', label: 'DEMA', tip: 'dema' },
      { key: 'sats', label: 'SATS', tip: 'sats' },
      { key: 'alligator', label: 'Gator', tip: 'alligator' },
      { key: 'hullMA', label: 'Hull', tip: 'hullMA' },
      { key: 'tema', label: 'TEMA', tip: 'tema' },
    ],
  },
  {
    key: 'volatility',
    title: '🎢波动',
    style: 'font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(245,158,11,0.08); border-radius: 4px; border-left: 3px solid #f59e0b; color: #d97706',
    items: [
      { key: 'boll', label: 'BOLL', tip: 'boll' },
      { key: 'keltner', label: 'Kelt', tip: 'keltner' },
      { key: 'donchian', label: 'Donch', tip: 'donchian' },
      { key: 'atr', label: 'ATR', tip: 'atr' },
      { key: 'avgAmp', label: '均幅', tip: 'avgAmp' },
      { key: 'ttmSqueeze', label: 'TTM', tip: 'ttmSqueeze' },
      { key: 'zigzag', label: 'ZigZag', tip: 'zigzag' },
      { key: 'fractal', label: 'Fractal', tip: 'fractal' },
      { key: 'massIndex', label: 'Mass', tip: 'massIndex' },
      { key: 'smc', label: 'SMC', tip: 'smc' },
    ],
  },
  {
    key: 'momentum',
    title: '💫动量',
    style: 'font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(59,130,246,0.08); border-radius: 4px; border-left: 3px solid #3b82f6; color: #2563eb',
    items: [
      { key: 'macd', label: 'MACD', tip: 'macd' },
      { key: 'kdj', label: 'KDJ', tip: 'kdj' },
      { key: 'rsi', label: 'RSI', tip: 'rsi' },
      { key: 'cci', label: 'CCI', tip: 'cci' },
      { key: 'williamsR', label: 'W%R', tip: 'williamsR' },
      { key: 'stochRsi', label: 'SRSI', tip: 'stochRsi' },
      { key: 'cmo', label: 'CMO', tip: 'cmo' },
      { key: 'ao', label: 'AO', tip: 'ao' },
      { key: 'trix', label: 'TRIX', tip: 'trix' },
      { key: 'trixSlope', label: 'TRIX斜率', tip: 'trixSlope' },
      { key: 'roc', label: 'ROC', tip: 'roc' },
      { key: 'smi', label: 'SMI', tip: 'smi' },
      { key: 'coppock', label: 'Coppck', tip: 'coppock' },
    ],
  },
  {
    key: 'volume',
    title: '📊量价',
    style: 'font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(16,185,129,0.08); border-radius: 4px; border-left: 3px solid #10b981; color: #059669',
    items: [
      { key: 'obv', label: 'OBV', tip: 'obv' },
      { key: 'vwap', label: 'VWAP', tip: 'vwap' },
      { key: 'mfi', label: 'MFI', tip: 'mfi' },
      { key: 'cmf', label: 'CMF', tip: 'cmf' },
      { key: 'forceIndex', label: 'FI', tip: 'forceIndex' },
      { key: 'ad', label: 'A/D', tip: 'ad' },
      { key: 'chaikinOsc', label: 'ChkOsc', tip: 'chaikinOsc' },
      { key: 'vwapBands', label: 'VWBnd', tip: 'vwapBands' },
    ],
  },
  {
    key: 'strength',
    title: '📏强度',
    style: 'font-size: 13px; font-weight: 700; display: block; margin-bottom: 4px; padding: 2px 6px; background: rgba(139,92,246,0.08); border-radius: 4px; border-left: 3px solid #8b5cf6; color: #7c3aed',
    items: [
      { key: 'adx', label: 'ADX', tip: 'adx' },
      { key: 'pivot', label: 'Pivot', tip: 'pivot' },
      { key: 'chop', label: 'CHOP', tip: 'chop' },
      { key: 'elderRay', label: 'Elder', tip: 'elderRay' },
      { key: 'ulcerIndex', label: 'Ulcer', tip: 'ulcerIndex' },
      { key: 'signalRatio', label: '信号比', tip: 'signalRatio' },
    ],
  },
]

// 全部 49 个指标键（与 KLINE_INDICATOR_SECTIONS 逐项对应，供 useKlineIndicators 初始化 flags）
export const KLINE_INDICATOR_KEYS: string[] = KLINE_INDICATOR_SECTIONS.flatMap((s) => s.items.map((i) => i.key))
