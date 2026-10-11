import { ref } from 'vue'

export interface FocusedStock {
  code: string
  name: string
  ts_code?: string
  lastUpdated?: number
}

// 响应式当前聚焦股票引用，默认从本地缓存恢复
const STORAGE_KEY = 'blades-stock:current-focused-stock'

function loadSavedStock(): FocusedStock | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw)
  } catch {}
  return null
}

export const currentStock = ref<FocusedStock | null>(loadSavedStock())

/**
 * 格式化标准化代码
 */
export function cleanCode(code: string): string {
  if (!code) return ''
  let c = String(code).trim()
  for (const sep of [' - ', '-', ' ']) {
    if (c.includes(sep)) {
      const parts = c.split(sep)
      c = parts[parts.length - 1].trim()
    }
  }
  return c
}

/**
 * 设置全局当前关注的股票上下文
 */
export function setCurrentStock(stock: { code: string; name?: string; ts_code?: string }) {
  if (!stock || !stock.code) return
  const code = cleanCode(stock.code)
  const name = stock.name || ''
  const updated: FocusedStock = {
    code,
    name,
    ts_code: stock.ts_code || code,
    lastUpdated: Date.now()
  }
  currentStock.value = updated
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(updated))
  } catch {}

  // 跨窗口与非 Vue 上下文事件通知
  window.dispatchEvent(new CustomEvent('blades-stock:stock-focused', { detail: updated }))
}

/**
 * 获取当前全局聚焦的股票
 */
export function getCurrentStock(): FocusedStock | null {
  return currentStock.value
}
