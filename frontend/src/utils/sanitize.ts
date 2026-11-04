import DOMPurify from 'dompurify'

/**
 * 针对 Markdown 渲染结果的工业级 HTML 消毒器
 * 坚决剔除 <script>、<iframe、<object、<embed、javascript:伪协议以及 onerror/onload 等所有行内事件句柄，
 * 彻底切断提示词注入与存储型 XSS 偷取 localStorage 凭证的攻击链。
 * 同时原生保护 MathML、SVG 及大模型输出的尖括号数学变量（如 <R²>、<R^2> 等），杜绝文本空洞与变量吞没。
 */
export function sanitizeMarkdownHtml(html: string): string {
  if (!html) return ''

  // 防护：防止大模型输出的非 HTML 尖括号公式/变量（如 <R²>、<R^2>、< 0.85 等）被 DOMPurify 误当作未知标签剥离
  // 将非标准 HTML 标签开头的尖括号安全转义为实体 &lt; 和 &gt;
  const preProcessed = html.replace(/<(?!\/?(p|h[1-6]|ul|ol|li|blockquote|hr|table|thead|tbody|tr|th|td|b|strong|i|em|del|code|pre|span|div|a|img|details|summary|sub|sup|mark|math|semantics|mrow|msup|msub|msubsup|mi|mn|mo|annotation|mtext|mspace|mover|munder|svg|path|line|text)\b)([^>]{0,60})>/gi, '&lt;$2&gt;')

  return DOMPurify.sanitize(preProcessed, {
    USE_PROFILES: { html: true, mathMl: true, svg: true },
    ALLOWED_TAGS: [
      'p', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
      'ul', 'ol', 'li', 'blockquote', 'hr',
      'table', 'thead', 'tbody', 'tr', 'th', 'td',
      'b', 'strong', 'i', 'em', 'del', 'code', 'pre',
      'span', 'div', 'a', 'img', 'details', 'summary',
      'sub', 'sup', 'mark',
      // MathML & SVG 公式与图表支持
      'math', 'semantics', 'mrow', 'msup', 'msub', 'msubsup',
      'mi', 'mn', 'mo', 'annotation', 'mtext', 'mspace', 'mover', 'munder',
      'svg', 'path', 'line', 'text'
    ],
    ALLOWED_ATTR: [
      'href', 'src', 'alt', 'title', 'class', 'style',
      'target', 'rel', 'width', 'height', 'align',
      'xmlns', 'encoding', 'aria-hidden', 'viewBox', 'd', 'fill', 'stroke'
    ],
    ALLOW_DATA_ATTR: false,
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input'],
    FORBID_ATTR: [
      'onerror', 'onload', 'onclick', 'onmouseover', 'onfocus',
      'onblur', 'onchange', 'onsubmit', 'onkeydown', 'onkeypress', 'onkeyup'
    ]
  })
}
