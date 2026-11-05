import MarkdownIt from 'markdown-it'
import { sanitizeMarkdownHtml } from './sanitize'

/**
 * chatMarkdown —— Agent 对话流的轻量 Markdown 渲染管线
 *
 * ── 为什么不用 md-editor-v3 / TDesign ChatContent ─────────────────────────────
 * · `@tdesign-vue-next/chat` 的 ChatContent：连带整个 tdesign-vue-next
 *   （产物 raw 1.17 MB / br 355 KB）拖入 Agent 路由，性价比极低。
 * · `md-editor-v3` 的 MdPreview：面向「编辑器」场景，连带 highlight.js 全语言包 +
 *   @vavt 扩展 + medium-zoom，产物 raw 1.31 MB / br 404 KB，对聊天流同样过重。
 *
 * 对话流真正需要的只是：段落、标题、列表、表格、引用、行内代码、代码块、链接、图片。
 * 这些 `markdown-it` 默认预置即可 100% 覆盖，体积仅约 raw 110 KB / br 36 KB
 * —— 相对 TDesign 方案节省约 90%。语法高亮交由 CSS 承担（投研文本以表格/要点为主，
 * 代码块占比极低），避免为长尾场景付出 highlight.js 的全部语言包成本。
 *
 * 安全：`html: false` 从源头禁止原始 HTML 注入，渲染结果再经 sanitizeMarkdownHtml
 * （DOMPurify + 公式尖括号实体化）二次过滤，形成纵深防御。
 */

let mdInstance: MarkdownIt | null = null

function createRenderer(): MarkdownIt {
  const md = new MarkdownIt({
    // 禁止原始 HTML：LLM 输出可能被提示词注入污染，从源头掐断 <script>/<img onerror> 链路
    html: false,
    // 单个换行即视为软换行 → 渲染为 <br>，符合 IM 对话阅读习惯
    breaks: true,
    // 自动识别裸 URL / 股票代码链接
    linkify: true,
    typographer: false
  })

  // 外链一律 target=_blank + rel=noopener noreferrer，杜绝 reverse tabnabbing
  const defaultLinkOpen =
    md.renderer.rules.link_open ||
    ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options))

  md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
    const token = tokens[idx]
    const href = token.attrGet('href') || ''
    const isExternal = /^https?:\/\//i.test(href)
    if (isExternal) {
      token.attrSet('target', '_blank')
      token.attrSet('rel', 'noopener noreferrer nofollow')
    }
    return defaultLinkOpen(tokens, idx, options, env, self)
  }

  return md
}

export function getChatMarkdownRenderer(): MarkdownIt {
  if (!mdInstance) mdInstance = createRenderer()
  return mdInstance
}

/** 将 Markdown 源文本渲染为经过安全消毒的 HTML 字符串 */
export function renderChatMarkdown(source: string): string {
  const text = source == null ? '' : String(source)
  if (!text) return ''
  const raw = getChatMarkdownRenderer().render(text)
  return sanitizeMarkdownHtml(raw)
}
