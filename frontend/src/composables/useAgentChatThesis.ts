// 买方认知账本：从助手回答提炼 5-Verdict 认知结构、存入账本、导出投研长图。
// 由 agent-chat.vue 抽出（R-22c），逻辑逐字搬运；共享状态与服务由组件以 ctx 传入。
import { h, ref } from 'vue'
import { useDialog } from 'naive-ui'
import html2canvas from 'html2canvas'
import { useAiConfigStore } from '@/stores/aiConfig'
import { SaveImage, SaveStockThesis } from '../../wailsjs/go/main/App'
import { EventsEmit } from '../../wailsjs/runtime'
import type { ChatMessage } from '../components/chat'

export function useAgentChatThesis(ctx) {
  const messages = ctx.messages
  const notifyWarning = ctx.notifyWarning
  const notifySuccess = ctx.notifySuccess
  const notifyError = ctx.notifyError
  const dialog = useDialog()
  const aiConfigStore = useAiConfigStore()

const lastConsultStock = ref<{ code: string; name: string } | null>(null)
// 智能提炼买方 5-Verdict 认知结构 (立论、假说、催化、证伪风险)
function extractThesisFromText(text: string, defaultCode?: string, defaultName?: string) {
  if (!text) return null

  // 1. 识别股票代码 (标准 6 位 A 股/ETF 代码，支持兼容 sh/sz 前缀、.SH/.SZ 后缀以及 Markdown 标记)
  let code = defaultCode || ''
  if (!code) {
    // 优先匹配标准 6 位代码
    const codeMatch = text.match(/(?:sh|sz|bj)?\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b(?:\.(?:SH|SZ|BJ))?/i)
    if (codeMatch) {
      code = codeMatch[1]
    } else if (lastConsultStock.value?.code) {
      code = lastConsultStock.value.code
    } else {
      // 从会话历史反向追溯最近提及的标准股票代码
      for (let i = messages.value.length - 1; i >= 0; i--) {
        const histText = messages.value[i].content || messages.value[i].rawContent || ''
        const histMatch = histText.match(/(?:sh|sz|bj)?\b(00\d{4}|30\d{4}|60\d{4}|68\d{4}|51\d{4}|58\d{4}|15\d{4})\b(?:\.(?:SH|SZ|BJ))?/i)
        if (histMatch) {
          code = histMatch[1]
          break
        }
      }
    }
  }
  if (!code) return null

  // 2. 识别股票名称 (支持多种常见 Markdown、括号与标的引导词格式)
  let name = defaultName || ''
  if (!name) {
    const namePatterns = [
      /[【(（]([A-Za-z\u4e00-\u9fa5\d]+)[)）】]\s*(?:[(（]?(?:sh|sz|bj)?\b\d{6}\b(?:\.(?:SH|SZ|BJ))?[)）]?)/,
      /【([A-Za-z\u4e00-\u9fa5\d]+)\s*[0-9]{6}】/,
      /(?:标的(?:名称|代码)?|股票(?:名称)?|分析标的)[：:\s*]+(?:\*\*)?([A-Za-z\u4e00-\u9fa5\d]+)(?:\*\*)?/,
      /(?:\*\*|__)([A-Za-z\u4e00-\u9fa5\d]+)(?:\*\*|__)\s*[(（]\s*\d{6}\s*[)）]/,
      /([A-Za-z\u4e00-\u9fa5\d]+)\s*[(（]\s*(?:sh|sz|bj)?\d{6}(?:\.(?:SH|SZ|BJ))?\s*[)）]/
    ]
    for (const pat of namePatterns) {
      const match = text.match(pat)
      if (match && match[1] && match[1].length >= 2 && match[1].length <= 12) {
        name = match[1].trim()
        break
      }
    }
    if (!name && lastConsultStock.value?.name) {
      name = lastConsultStock.value.name
    }
    if (!name && code) {
      // 从会话历史反向追溯股票名称
      for (let i = messages.value.length - 1; i >= 0; i--) {
        const histText = messages.value[i].content || messages.value[i].rawContent || ''
        for (const pat of namePatterns) {
          const match = histText.match(pat)
          if (match && match[1] && match[1].length >= 2 && match[1].length <= 12) {
            name = match[1].trim()
            break
          }
        }
        if (name) break
      }
    }
  }

  // 3. 识别买方五维定性判定 (多维金融自然语言模糊匹配)
  let verdict = 'NO_CHANGE'
  let verdictText = '维持现状'
  if (
    text.includes('逻辑强化') ||
    text.includes('THESIS_STRENGTHENED') ||
    text.includes('强烈买入') ||
    text.includes('积极做多') ||
    text.includes('超预期') ||
    text.includes('突破确认') ||
    text.includes('多头共振')
  ) {
    verdict = 'THESIS_STRENGTHENED'
    verdictText = '逻辑强化'
  } else if (
    text.includes('逻辑证伪') ||
    text.includes('THESIS_BROKEN') ||
    text.includes('清仓止损') ||
    text.includes('破位止损') ||
    text.includes('强风控空仓') ||
    text.includes('假说破灭')
  ) {
    verdict = 'THESIS_BROKEN'
    verdictText = '逻辑证伪'
  } else if (
    text.includes('逻辑削弱') ||
    text.includes('THESIS_WEAKENED') ||
    text.includes('谨慎防守') ||
    text.includes('逢高减仓') ||
    text.includes('不及预期') ||
    text.includes('动能衰减') ||
    text.includes('冲高回落')
  ) {
    verdict = 'THESIS_WEAKENED'
    verdictText = '逻辑削弱'
  } else if (
    text.includes('首次建档') ||
    text.includes('新论点') ||
    text.includes('首次立论') ||
    text.includes('NEW_THESIS') ||
    text.includes('首次覆盖') ||
    text.includes('新建仓')
  ) {
    verdict = 'NEW_THESIS'
    verdictText = '首次立论'
  }

  // 4. 提取核心投资假说 (过滤 Markdown 符号)
  // if/else 两分支都必定赋值，故不设初值（no-useless-assignment）
  let coreThesis: string
  const thesisMatch = text.match(/(?:核心投资逻辑|核心假说|核心立论|投资逻辑|核心论点|买方论据)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (thesisMatch) {
    coreThesis = thesisMatch[1].replace(/[#*`]/g, '').trim()
  } else {
    // 提取正文第一段有实质意义的内容
    const cleanLines = text.split('\n')
      .map(l => l.replace(/[#*`]/g, '').trim())
      .filter(l => l.length > 15 && !l.startsWith('好的') && !l.startsWith('收到') && !l.includes('免责声明'))
    coreThesis = cleanLines.length > 0 ? cleanLines[0].slice(0, 160) : text.slice(0, 160).replace(/[#*`]/g, '').trim()
  }

  // 5. 提取重点跟踪催化剂
  let keyCatalysts = ''
  const catMatch = text.match(/(?:催化剂|跟踪催化剂|核心催化|催化事件|关键催化|关键事件)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (catMatch) {
    keyCatalysts = catMatch[1].replace(/[#*`]/g, '').trim()
  }

  // 6. 提取证伪条件与防守止损
  let riskFactors = ''
  const riskMatch = text.match(/(?:证伪条件|风险因素|反转条件|止损条件|防守位|黑天鹅风险|下行风险)[：:\s]+([^\n]+(?:\n[^\n]+)?)/)
  if (riskMatch) {
    riskFactors = riskMatch[1].replace(/[#*`]/g, '').trim()
  }

  return {
    code,
    name: name || code,
    verdict,
    verdictText,
    coreThesis,
    keyCatalysts,
    riskFactors,
  }
}

// 消息卡片点击「存入买方认知账本」
async function handleSaveThesisFromMsg(msg: ChatMessage) {
  const content = msg.content || msg.rawContent || ''
  const thesis = extractThesisFromText(content)
  if (!thesis || !thesis.code) {
    notifyWarning('未在分析结果中识别到明确的股票代码或买方论点')
    return
  }

  // P1: AI 自主写账本需二次确认 + 逐字段 diff 预览
  const confirmed = await new Promise<boolean>((resolve) => {
    dialog.warning({
      title: '确认沉淀买方认知账本',
      content: () =>
        h('div', { style: 'font-size: 14px; line-height: 1.8;' }, [
          h('p', null, [h('strong', null, '即将把以下买方认知沉淀至认知账本：')]),
          h('div', { style: 'margin: 12px 0; padding: 12px; background: rgba(0,0,0,0.04); border-radius: 6px; font-size: 13px;' }, [
            h('div', null, [h('strong', null, '标的：'), ` ${thesis.name || thesis.code} (${thesis.code})`]),
            h('div', null, [h('strong', null, '核心定论：'), ` ${thesis.verdictText}`]),
            h('div', null, [h('strong', null, '核心逻辑：'), ` ${(thesis.coreThesis || '').slice(0, 200)}...`]),
            h('div', null, [h('strong', null, '关键催化：'), ` ${(thesis.keyCatalysts || '无').slice(0, 100)}...`]),
            h('div', null, [h('strong', null, '证伪条件：'), ` ${(thesis.riskFactors || '无').slice(0, 100)}...`]),
            h('div', { style: 'margin-top: 8px; color: #e85d04;' }, [h('strong', null, '⚠️ 写入后将更新标的历史买方认知，请确认无误后再继续。')]),
          ]),
        ]),
      positiveText: '确认写入',
      negativeText: '取消',
      onPositiveClick: () => resolve(true),
      onNegativeClick: () => resolve(false),
      onClose: () => resolve(false),
    })
  })

  if (!confirmed) return

  try {
    const activeCfg = aiConfigStore.activeConfig
    const modelName = activeCfg?.modelName || 'Blades-Agent'
    await SaveStockThesis(
      thesis.code,
      thesis.name || thesis.code,
      thesis.verdict,
      thesis.verdictText,
      thesis.coreThesis,
      thesis.keyCatalysts,
      thesis.riskFactors,
      modelName
    )
    notifySuccess(`🎉 已将【${thesis.name || thesis.code}】买方认知(${thesis.verdictText})归档入库！`)
    // 广播标的认知更新事件，驱动自选股卡片与诊断抽屉实时刷新
    EventsEmit('stockThesisUpdated', { stockCode: thesis.code })
  } catch (err: any) {
    console.error('SaveStockThesis error:', err)
    notifyError(`存入买方认知账本失败: ${err?.message || err}`)
  }
}

// 导出单条 AI 投研回答为高清长图
async function handleExportImage(msg: ChatMessage) {
  const anchorId = 'msg-anchor-' + msg.id
  const el = document.getElementById(anchorId) || document.getElementById('msg-item-' + msg.id)
  if (!el) {
    notifyWarning('未找到对应消息 DOM 节点，请重试')
    return
  }

  // 查找最近的 .t-chat__message-item 或 .assistant-msg 容器
  const target = (el.closest('.t-chat__message-item') || el.closest('.assistant-msg') || el) as HTMLElement

  const savedStyles: Array<{ el: HTMLElement; overflow: string; overflowY: string; height: string; maxHeight: string }> = []
  let p = target.parentElement
  while (p && p !== document.body) {
    const style = getComputedStyle(p)
    if (style.overflow === 'hidden' || style.overflowY === 'hidden' || style.overflowY === 'auto' || style.overflowY === 'scroll') {
      savedStyles.push({
        el: p,
        overflow: p.style.overflow,
        overflowY: p.style.overflowY,
        height: p.style.height,
        maxHeight: p.style.maxHeight
      })
      p.style.overflow = 'visible'
      p.style.overflowY = 'visible'
      p.style.height = 'auto'
      p.style.maxHeight = 'none'
    }
    p = p.parentElement
  }

  const isDark = document.documentElement.getAttribute('theme-mode') === 'dark'
  try {
    const canvas = await html2canvas(target, {
      useCORS: true,
      scale: 2,
      allowTaint: true,
      logging: false,
      backgroundColor: isDark ? '#18181c' : '#ffffff'
    })
    const dataUrl = canvas.toDataURL('image/png')
    const base64 = dataUrl.replace(/^data:image\/png;base64,/, '')
    const safeTime = new Date().toISOString().slice(0, 19).replace(/[:.]/g, '-')

    try {
      const result = await SaveImage(`blades-stock-agent-${safeTime}`, base64)
      if (result && !result.includes('异常') && !result.includes('无法')) {
        notifySuccess(`已导出为 PNG 图片：${result}`)
        return
      }
    } catch (_e) {
      // fallback to browser file download
    }

    const link = document.createElement('a')
    link.href = dataUrl
    link.download = `blades-stock-agent-${safeTime}.png`
    link.click()
    notifySuccess('已导出下载 PNG 投研长图')
  } catch (err: any) {
    console.error('handleExportImage failed:', err)
    notifyError(`导出图片失败: ${err?.message || err}`)
  } finally {
    savedStyles.forEach(({ el, overflow, overflowY, height, maxHeight }) => {
      el.style.overflow = overflow
      el.style.overflowY = overflowY
      el.style.height = height
      el.style.maxHeight = maxHeight
    })
  }
}

  return { lastConsultStock, handleSaveThesisFromMsg, handleExportImage }
}
