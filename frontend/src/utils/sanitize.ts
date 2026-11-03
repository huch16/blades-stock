import DOMPurify from 'dompurify'

/**
 * 针对 Markdown 渲染结果的工业级 HTML 消毒器
 * 坚决剔除 <script>、<iframe、<object、<embed、javascript:伪协议以及 onerror/onload 等所有行内事件句柄，
 * 彻底切断提示词注入与存储型 XSS 偷取 localStorage 凭证的攻击链。
 */
export function sanitizeMarkdownHtml(html: string): string {
  if (!html) return ''
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'p', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li', 'blockquote', 'hr',
      'table', 'thead', 'tbody', 'tr', 'th', 'td',
      'b', 'strong', 'i', 'em', 'del', 'code', 'pre',
      'span', 'div', 'a', 'img', 'details', 'summary',
      'sub', 'sup', 'mark'
    ],
    ALLOWED_ATTR: [
      'href', 'src', 'alt', 'title', 'class', 'style',
      'target', 'rel', 'width', 'height', 'align'
    ],
    ALLOW_DATA_ATTR: false,
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input'],
    FORBID_ATTR: [
      'onerror', 'onload', 'onclick', 'onmouseover', 'onfocus',
      'onblur', 'onchange', 'onsubmit', 'onkeydown', 'onkeypress', 'onkeyup'
    ]
  })
}
