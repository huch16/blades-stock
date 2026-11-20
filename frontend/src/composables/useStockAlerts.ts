// 成本 / 报警：价位线报警检查、成本数量修改、交易价回写后端、成本弹窗打开。
// 由 stock.vue 抽出（R-22b），逻辑逐字搬运；共享状态与服务由组件以 ctx 传入。
import {
  GetFollowList,
  SendDingDingMessageByType,
  SetAlarmChangePercent,
  SetCostPriceAndVolume,
  SetStockAICron,
  SetStockSort,
  SetTradingPrice,
} from '../../wailsjs/go/main/App'

export function useStockAlerts(ctx) {
  const currentGroupId = ctx.currentGroupId
  const currentStockTradingPrice = ctx.currentStockTradingPrice
  const followList = ctx.followList
  const formModel = ctx.formModel
  const fromEastMoneyCode = ctx.fromEastMoneyCode
  const message = ctx.message
  const modalShow = ctx.modalShow
  const monitor = ctx.monitor
  const stocks = ctx.stocks

const priceLineAlertCache = new Map()
function checkPriceLineAlerts(result) {
  const code = result["股票代码"]
  const price = result["当前价格"]
  if (!price || price <= 0) return

  const followedStock = followList.value.find(s => {
    const sCode = s.StockCode || ''
    return sCode === code || sCode === 'sh' + code || sCode === 'sz' + code ||
           sCode === code.replace('sh', '').replace('sz', '') ||
           (sCode.length > 2 && code.length > 2 && sCode.includes(code.slice(2)))
  })

  if (!followedStock) return

  const alerts = []
  let triggeredType = 0
  if (followedStock.EntryPrice > 0) {
    const diff = ((price - followedStock.EntryPrice) / followedStock.EntryPrice * 100).toFixed(2)
    alerts.push(`开仓价: ${followedStock.EntryPrice} (${diff >= 0 ? '+' : ''}${diff}%)`)
  }
  if (followedStock.TakeProfitPrice > 0) {
    if (price >= followedStock.TakeProfitPrice) {
      alerts.push(`止盈价: ${followedStock.TakeProfitPrice} ⚠️ 已触及`)
      triggeredType = 4
    } else {
      const diff = ((followedStock.TakeProfitPrice - price) / followedStock.TakeProfitPrice * 100).toFixed(2)
      alerts.push(`止盈价: ${followedStock.TakeProfitPrice} (距离 ${diff}%)`)
    }
  }
  if (followedStock.StopLossPrice > 0) {
    if (price <= followedStock.StopLossPrice) {
      alerts.push(`止损价: ${followedStock.StopLossPrice} ⚠️ 已触及`)
      triggeredType = 5
    } else {
      const diff = ((price - followedStock.StopLossPrice) / followedStock.StopLossPrice * 100).toFixed(2)
      alerts.push(`止损价: ${followedStock.StopLossPrice} (+${diff}%)`)
    }
  }

  if (alerts.length === 0) return

  const cacheKey = `${code}_${price}`
  if (priceLineAlertCache.get(cacheKey)) return

  const notifyKey = `${code}_notify`
  const lastNotify = priceLineAlertCache.get(notifyKey) || 0
  const now = Date.now()
  if (now - lastNotify < 60000) return

  priceLineAlertCache.set(cacheKey, true)
  priceLineAlertCache.set(notifyKey, now)

  const stockName = followedStock.Name || followedStock.StockName || result["股票名称"] || code
  const stockCodeDisplay = code.length > 6 ? code : code.toUpperCase()

  // 注：此处曾用 notify.info 弹通知（头像 + 价位线预警列表），已停用，现改为钉钉消息。
  // 原注释代码已删除：构建前检查会扫描注释里的渲染调用并误判为未导入标识符而拦截构建。

  if (triggeredType > 0) {
    const msg = `### 📈 价位线预警\n\n### ${stockName} (${stockCodeDisplay})\n\n- 当前价格: ${price}\n- 预警类型: ${triggeredType === 4 ? '止盈触及' : '止损触及'}\n- 开仓价: ${followedStock.EntryPrice || '-'}\n- 止盈价: ${followedStock.TakeProfitPrice || '-'}\n- 止损价: ${followedStock.StopLossPrice || '-'}`;
    SendDingDingMessageByType(msg, code, triggeredType)
  }
}
function updateCostPriceAndVolumeNew(code, price, volume, alarm, formModel) {
  if (formModel.sort) {
    SetStockSort(formModel.sort, code).then(_result => {
      //message.success(result)
    })
  }
  if (formModel.cron) {
    SetStockAICron(formModel.cron, code).then(_result => {
      //message.success(result)
    })
  }

  if (alarm || formModel.alarmPrice) {
    SetAlarmChangePercent(alarm, formModel.alarmPrice, code).then(_result => {
      //message.success(result)
    })
  }
  
  // 保存交易价格（开仓价、止盈价、止损价、成本价）
  if (formModel.entryPrice || formModel.takeProfitPrice || formModel.stopLossPrice || formModel.costPrice) {
    SetTradingPrice(code, formModel.entryPrice || 0, formModel.takeProfitPrice || 0, formModel.stopLossPrice || 0, formModel.costPrice || 0).then(_result => {
      //message.success(result)
    })
  }
  
  SetCostPriceAndVolume(code, price, volume).then(result => {
    modalShow.value = false
    message.success(result)
    GetFollowList(currentGroupId.value).then(result => {
      followList.value = result
      stocks.value = []
      for (const followedStock of result) {
        if (!stocks.value.includes(followedStock.StockCode)) {
          stocks.value.push(followedStock.StockCode)
        }
      }
      monitor()
      message.destroyAll()
    })
  })
}
function saveTradingPriceToBackend() {
  if (!currentStockTradingPrice.value.stockCode) {
    return
  }
  const emCode = currentStockTradingPrice.value.stockCode
  const code = fromEastMoneyCode(emCode)
  if (!code) {
    console.warn('无法转换股票代码:', emCode)
    return
  }
  const entryPrice = Number(currentStockTradingPrice.value.entryPrice) || 0
  const takeProfitPrice = Number(currentStockTradingPrice.value.takeProfitPrice) || 0
  const stopLossPrice = Number(currentStockTradingPrice.value.stopLossPrice) || 0
  const costPrice = Number(currentStockTradingPrice.value.costPrice) || 0
  SetTradingPrice(
    code,
    entryPrice,
    takeProfitPrice,
    stopLossPrice,
    costPrice
  ).then(result => {
    if (result === '设置成功') {
      const emCode = currentStockTradingPrice.value.stockCode
      const internalCode = code
      const followItem = followList.value.find(item => item.StockCode === internalCode || item.StockCode === emCode)
      if (followItem) {
        followItem.EntryPrice = entryPrice
        followItem.TakeProfitPrice = takeProfitPrice
        followItem.StopLossPrice = stopLossPrice
      }
    }
  }).catch(err => {
    console.error('SetTradingPrice error:', err)
  })
}
function setStock(code, name) {
  const res = followList.value.filter(item => item.StockCode === code)
  ////console.log("res:",res)
  formModel.value.name = name
  formModel.value.code = code
  formModel.value.volume = res[0].Volume ? res[0].Volume : 0
  formModel.value.costPrice = res[0].CostPrice
  formModel.value.alarm = res[0].AlarmChangePercent
  formModel.value.alarmPrice = res[0].AlarmPrice
  formModel.value.sort = res[0].Sort
  formModel.value.cron = res[0].Cron
  formModel.value.entryPrice = res[0].EntryPrice || 0
  formModel.value.takeProfitPrice = res[0].TakeProfitPrice || 0
  formModel.value.stopLossPrice = res[0].StopLossPrice || 0
  modalShow.value = true
}

  return { checkPriceLineAlerts, updateCostPriceAndVolumeNew, saveTradingPriceToBackend, setStock }
}
