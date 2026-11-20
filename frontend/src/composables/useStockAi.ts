// AI 分析与报告导出：盘前风控扫描、个股 AI 分析/重分析、认知档案读取、
// 报告存图 / 存 Markdown / 存 Word。
// 由 stock.vue 抽出（R-22b），逻辑逐字搬运；共享状态与服务由组件以 ctx 传入。
import { consultAgent } from '../utils/agentControlBus'
import {
  GetAIResponseResult,
  NewChatStream,
  SaveAsMarkdown,
  SaveImage,
  SaveWordFile,
  GetStockThesis,
} from '../../wailsjs/go/main/App'
import { nextTick } from 'vue'

export function useStockAi(ctx) {
  const activeThesis = ctx.activeThesis
  const aiAnalysisTimeout = ctx.aiAnalysisTimeout
  const aiConfigStore = ctx.aiConfigStore
  const currentGroupId = ctx.currentGroupId
  const data = ctx.data
  const enableTools = ctx.enableTools
  const groupList = ctx.groupList
  const groupResults = ctx.groupResults
  const mdEditorRef = ctx.mdEditorRef
  const mdPreviewRef = ctx.mdPreviewRef
  const message = ctx.message
  const modalShow4 = ctx.modalShow4
  const sortedResults = ctx.sortedResults
  const thinkingMode = ctx.thinkingMode
  const tipsRef = ctx.tipsRef

async function fetchActiveThesis(code: string) {
  if (!code) {
    activeThesis.value = null
    return
  }
  try {
    const res = await GetStockThesis(code)
    activeThesis.value = res || null
  } catch {
    activeThesis.value = null
  }
}
function triggerAiWatchlistScreening() {
  const currentGroupObj = groupList.value.find(g => g.ID === currentGroupId.value)
  const groupTitle = currentGroupId.value === 0 ? '全部自选股' : (currentGroupObj ? currentGroupObj.name : '当前分组')
  const targetResults = currentGroupId.value === 0 ? sortedResults.value : groupResults.value
  const targetList = Object.values(targetResults || {})

  if (!targetList.length) {
    message.warning('当前自选分组无标的，请先添加自选股')
    return
  }

  const holdings = []
  const watchlist = []

  for (const item of targetList) {
    const code = item['股票代码'] || ''
    const name = item['股票名称'] || ''
    const price = item['当前价格'] || item['上次当前价格'] || '-'
    const change = item.changePercent !== undefined ? (item.changePercent >= 0 ? `+${item.changePercent}%` : `${item.changePercent}%`) : '-'
    const cost = Number(item.costPrice || 0)
    const volume = Number(item.costVolume || 0)
    const profitRate = item.profitRate !== undefined ? Number(item.profitRate) : 0

    if (volume > 0 && cost > 0) {
      const profitStr = profitRate >= 0 ? `+${profitRate.toFixed(2)}%` : `${profitRate.toFixed(2)}%`
      holdings.push(`${name}(${code}): 现价${price}元, 涨幅${change}, 持仓成本${cost.toFixed(2)}元(${profitStr})`)
    } else {
      watchlist.push(`${name}(${code}): 现价${price}元, 涨幅${change}`)
    }
  }

  const summary = []
  if (holdings.length > 0) {
    summary.push(`【真实持仓 (${holdings.length}只)】:\n` + holdings.slice(0, 50).join(';\n'))
  }
  if (watchlist.length > 0) {
    summary.push(`【自选观察池 (${watchlist.length}只)】:\n` + watchlist.slice(0, 50).join(';\n'))
  }

  const prompt = `【盘前持仓风控扫描与防守边界审计】
自选分组：${groupTitle}（共 ${targetList.length} 只标的）
标的与持仓详情：
${summary.join('\n\n')}

请以买方投资总监与风控合规视角，对上述自选与持仓标的进行盘前多维度持仓风控扫描并给出今日实操防守策略（250字以内）：
1. 【持仓与自选风险分级清单】：
   - 🟢【多头企稳/进攻跟踪】：具备多头均线排列、主线共振、或持仓浮盈具备防守垫的标的；
   - 🟡【均线压制/防守观望】：运行在MA20均线下方、逼近上方阻力位、或缩量滞涨的标的；
   - 🔴【破位预警/果断止损】：有效跌破关键趋势支撑、浮亏扩大或存在解禁商誉风险的标的；
2. 【今日盘中盯盘主线与红线】：今日操作重点聚焦哪只标的？绝对禁止逆势冲动追高哪只？`

  consultAgent({
    question: prompt,
    mode: 'plan_execute',
    autoSend: true,
  })
  message.success(`已发起 [${groupTitle}] 盘前持仓风控扫描！`)
}
function aiReCheckStock(stock, stockCode) {
  if (data.loading) {
    message.warning("AI分析正在进行中，请稍候")
    return
  }
  if (!aiConfigStore.activeConfigId) {
    message.error("请先选择AI模型配置")
    return
  }
  // 清除之前的超时定时器
  if (aiAnalysisTimeout.value) {
    clearTimeout(aiAnalysisTimeout.value)
    aiAnalysisTimeout.value = null
  }
  data.modelName = ""
  data.airesult = ""
  data.time = ""
  data.name = stock
  data.code = stockCode
  data.loading = true
  modalShow4.value = true
  fetchActiveThesis(stockCode)
  data.analysisStatus = "正在连接AI服务..."
  data.analysisAborted = false
  message.loading("ai检测中...", {
    duration: 0,
  })
  //

  //message.info("sysPromptId:"+data.sysPromptId)
  NewChatStream(stock, stockCode, data.question, aiConfigStore.activeConfigId, data.sysPromptId, enableTools.value,thinkingMode.value)
    .catch(err => {
      if (data.analysisAborted) return
      data.loading = false
      data.analysisStatus = ""
      message.destroyAll()
      const errMsg = err?.message || err || "未知错误"
      message.error("AI分析请求失败: " + errMsg)
      data.airesult = "❌ AI分析请求失败: " + errMsg
    })

  // 设置超时兜底（5分钟）
  aiAnalysisTimeout.value = setTimeout(() => {
    if (data.loading) {
      data.loading = false
      data.analysisStatus = ""
      data.analysisAborted = true
      message.destroyAll()
      message.error("AI分析超时，请检查网络连接或AI服务配置")
      if (!data.airesult) {
        data.airesult = "❌ AI分析超时，请检查网络连接或AI服务配置是否正确。"
      }
    }
    aiAnalysisTimeout.value = null
  }, 5 * 60 * 1000)
}
function aiCheckStock(stock, stockCode) {
  fetchActiveThesis(stockCode)
  GetAIResponseResult(stockCode).then(result => {
    if (result.content) {
      data.modelName = result.modelName
      data.chatId = result.chatId
      data.question = result.question
      data.name = stock
      data.code = stockCode
      data.loading = false
      modalShow4.value = true
      data.airesult = result.content
      const date = new Date(result.CreatedAt);
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      data.time = `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`
    } else {
      data.modelName = ""
      data.question = ""
      data.airesult = ""
      data.time = ""
      data.name = stock
      data.code = stockCode
      data.loading = false
      modalShow4.value = true
      // message.loading("ai检测中...", {
      //   duration: 0,
      // })
      // NewChatStream(stock, stockCode, "", data.sysPromptId)
    }
  })
}
function saveAsImage(name, code) {
  const previewEl = mdPreviewRef.value?.$el || mdEditorRef.value?.$el
  const element = previewEl?.querySelector('.md-editor-preview-wrapper') ||
                  previewEl?.querySelector('.md-editor-preview') ||
                  document.querySelector('.md-editor-preview')
  if (!element) {
    message.error('无法找到分析结果元素')
    return
  }
  const savedStyles = []
  let el = element.parentElement
  while (el && el !== document.body) {
    const style = getComputedStyle(el)
    if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
      savedStyles.push({ el, overflow: el.style.overflow, overflowY: el.style.overflowY, height: el.style.height, maxHeight: el.style.maxHeight })
      el.style.overflow = 'visible'
      el.style.overflowY = 'visible'
      el.style.height = 'auto'
      el.style.maxHeight = 'none'
    }
    el = el.parentElement
  }
  const savedTargetStyle = { height: element.style.height, maxHeight: element.style.maxHeight, overflow: element.style.overflow, overflowY: element.style.overflowY }
  element.style.height = 'auto'
  element.style.maxHeight = 'none'
  element.style.overflow = 'visible'
  element.style.overflowY = 'visible'
  nextTick(async () => {
    const isDark = document.documentElement.getAttribute('theme-mode') === 'dark'
    try {
      const html2canvasModule = await import('html2canvas')
      const html2canvas = html2canvasModule.default || html2canvasModule
      const canvas = await html2canvas(element, {
        useCORS: true,
        scale: 2,
        allowTaint: true,
        logging: false,
        backgroundColor: isDark ? '#1e1e1e' : '#ffffff'
      })
      element.style.height = savedTargetStyle.height
      element.style.maxHeight = savedTargetStyle.maxHeight
      element.style.overflow = savedTargetStyle.overflow
      element.style.overflowY = savedTargetStyle.overflowY
      savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
        el.style.overflow = overflow
        el.style.overflowY = overflowY
        el.style.height = height
        el.style.maxHeight = maxHeight
      })
      const dataUrl = canvas.toDataURL('image/png')
      const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
      const result = await SaveImage(name + '[' + code + ']AI分析', base64)
      if (result && !result.includes('异常') && !result.includes('无法')) {
        message.success('已导出为 PNG 图片：' + result)
      } else {
        message.info(result || '导出取消')
      }
    } catch (e) {
      element.style.height = savedTargetStyle.height
      element.style.maxHeight = savedTargetStyle.maxHeight
      element.style.overflow = savedTargetStyle.overflow
      element.style.overflowY = savedTargetStyle.overflowY
      savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
        el.style.overflow = overflow
        el.style.overflowY = overflowY
        el.style.height = height
        el.style.maxHeight = maxHeight
      })
      message.error('导出图片失败: ' + (e?.message ?? e))
    }
  })
}
function saveAsMarkdown() {
  SaveAsMarkdown(data.code, data.name).then(result => {
    if (!result || result === '取消保存') {
      message.info(result || '已取消保存')
      return
    }
    if (result.startsWith('保存失败') || result.startsWith('分析结果异常')) {
      message.error(result)
      return
    }
    if (result.startsWith('已保存至：')) {
      message.success(result)
      return
    }
    // Web 模式下后端返回 Markdown 正文，触发浏览器下载
    const blob = new Blob([result], { type: 'text/markdown;charset=utf-8' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `${data.name || '个股'}[${data.code || ''}]-AI分析结果.md`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(link.href)
    message.success('已导出为 Markdown 文件并开始下载')
  }).catch(err => {
    message.error('导出 Markdown 失败: ' + (err?.message || err))
  })
}
function getHtml(ref) {
  if (ref.value) {
    // 获取 MdPreview 组件的根元素
    const rootElement = ref.value.$el;
    // 获取 HTML 内容
    return rootElement.innerHTML;
  } else {
    console.error('mdPreviewRef is not yet available');
    return "";
  }
}
async function saveAsWord() {
  // 将富文本内容拼接为一个完整的html
  const html = getHtml(mdPreviewRef)
  const tipsHtml = getHtml(tipsRef)
  const value = `
         ${html}
         <hr>
         <div style="font-size: 12px;color: red">
         ${tipsHtml}
          </div>
<br>
本报告由Blades Stock项目生成：
<p>
<a href="https://github.com/huch16/blades-stock">
AI赋能股票分析：自选股行情获取，成本盈亏展示，涨跌报警推送，市场整体/个股情绪分析，K线技术指标分析等。数据全部保留在本地。支持DeepSeek，OpenAI， Ollama，LMStudio，AnythingLLM，硅基流动，火山方舟，阿里云百炼等平台或模型。
</a></p>
`
  // landscape就是横着的，portrait是竖着的，默认是竖屏portrait。
  const { asBlob } = await import('html-docx-js-typescript')
  const blob = await asBlob(value, {orientation: 'portrait'})
  const {platform} = await Environment()
  // 修复（no-case-declarations）：case 分支内使用 const/let 必须包块，
  // 否则词法声明会提升到整个 switch 作用域，既易与相邻 case 重名冲突，
  // 也会在严格模式下抛出 ReferenceError（TDZ）。
  switch (platform) {
    case 'windows': {
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = `${data.name}[${data.code}]-ai-analysis-result.docx`;
      a.click()
      // 下载后将标签移除
      URL.revokeObjectURL(a.href);
      a.remove()
      break
    }
    default: {
      const arrayBuffer = await blob.arrayBuffer()
      const uint8Array = new Uint8Array(arrayBuffer)
      const binary = uint8Array.reduce((data, byte) => data + String.fromCharCode(byte), '')
      const base64 = btoa(binary)
      await SaveWordFile(`${data.name}[${data.code}]-ai-analysis-result.docx`, base64).then(result => {
        message.success(result)
      })
    }
  }
}

  return { triggerAiWatchlistScreening, aiCheckStock, aiReCheckStock, saveAsImage, saveAsMarkdown, saveAsWord }
}
