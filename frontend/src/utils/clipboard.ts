/**
 * 通用剪贴板复制工具
 * 兼容 HTTPS、HTTP (VPS IP 直接访问)、localhost 以及各类 WebView/Wails 环境
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  if (!text) return false

  // 1. 尝试现代 Clipboard API (仅在 HTTPS 或 localhost 安全上下文中可用)
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch (e) {
      console.warn('[copyToClipboard] navigator.clipboard.writeText failed, falling back to execCommand:', e)
    }
  }

  // 2. 传统兼容兜底方案 (支持 HTTP VPS 访问与受限浏览器环境)
  if (typeof document !== 'undefined') {
    try {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.top = '-9999px'
      textarea.style.left = '-9999px'
      textarea.style.opacity = '0'
      textarea.setAttribute('readonly', '')
      document.body.appendChild(textarea)
      textarea.focus()
      textarea.select()
      const successful = document.execCommand('copy')
      document.body.removeChild(textarea)
      return successful
    } catch (err) {
      console.error('[copyToClipboard] execCommand failed:', err)
      return false
    }
  }

  return false
}
