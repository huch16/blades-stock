<template>
  <div class="chat-markdown" v-html="html"></div>
</template>

<script setup lang="ts">
/**
 * ChatMarkdown —— Agent 对话流内的轻量 Markdown 渲染器
 *
 * 设计目标（替代 TDesign ChatContent，彻底拆除 @tdesign-vue-next/chat 依赖）：
 *  1. 极轻：基于 markdown-it 默认预置（raw ~110 KB / br ~36 KB），
 *     不再为「2 个聊天组件」支付 TDesign 355 KB 或 md-editor-v3 404 KB 的双 UI 库税；
 *  2. 复用全站消毒管线：exports 统一走 sanitizeMarkdownHtml（DOMPurify + 公式尖括号实体化防护）；
 *  3. 主题自适应：跟随 App.vue applyThemeMode() 在 <html> 上写入的 theme-mode 属性；
 *  4. 失败降级：渲染管线异常时回退为纯文本，绝不冒泡到 ErrorBoundary。
 */
import { computed } from 'vue'
import { renderChatMarkdown } from '@/utils/chatMarkdown'

const props = withDefaults(
  defineProps<{
    content?: string
  }>(),
  {
    content: ''
  }
)

const html = computed(() => {
  try {
    return renderChatMarkdown(props.content || '')
  } catch (err) {
    // 渲染管线异常时绝不抛出（否则会被 ErrorBoundary 捕获并整屏熔断）
    console.warn('[ChatMarkdown] Markdown 渲染失败，已降级为纯文本:', err)
    const escaped = String(props.content || '')
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
    return `<pre class="chat-markdown-fallback">${escaped}</pre>`
  }
})
</script>

<style scoped>
.chat-markdown {
  width: 100%;
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 13.5px;
  line-height: 1.72;
  color: var(--chat-text, var(--bs-text-primary, #2c3e50));
  word-break: break-word;
}

.chat-markdown :deep(> :first-child) {
  margin-top: 0;
}

.chat-markdown :deep(> :last-child) {
  margin-bottom: 0;
}

.chat-markdown :deep(h1) {
  font-size: 18px;
  font-weight: 700;
  margin: 12px 0 8px;
  line-height: 1.4;
}

.chat-markdown :deep(h2) {
  font-size: 16px;
  font-weight: 700;
  margin: 11px 0 7px;
  line-height: 1.4;
}

.chat-markdown :deep(h3) {
  font-size: 14.5px;
  font-weight: 600;
  margin: 10px 0 6px;
  line-height: 1.45;
}

.chat-markdown :deep(h4),
.chat-markdown :deep(h5),
.chat-markdown :deep(h6) {
  font-size: 13.5px;
  font-weight: 600;
  margin: 9px 0 5px;
}

.chat-markdown :deep(p) {
  margin: 6px 0;
}

.chat-markdown :deep(ul),
.chat-markdown :deep(ol) {
  margin: 6px 0;
  padding-left: 22px;
}

.chat-markdown :deep(li) {
  margin: 2px 0;
}

.chat-markdown :deep(li > p) {
  margin: 2px 0;
}

.chat-markdown :deep(strong) {
  font-weight: 700;
  color: var(--chat-text-strong, #1f2329);
}

.chat-markdown :deep(del),
.chat-markdown :deep(s) {
  opacity: 0.65;
}

.chat-markdown :deep(a) {
  color: var(--bs-brand, #2080f0);
  text-decoration: none;
  border-bottom: 1px solid rgba(32, 128, 240, 0.35);
}

.chat-markdown :deep(a:hover) {
  border-bottom-color: var(--bs-brand, #2080f0);
}

.chat-markdown :deep(code) {
  font-family: 'JetBrains Mono', 'Fira Code', Menlo, Consolas, monospace;
  font-size: 12.5px;
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(127, 127, 127, 0.14);
  color: #c7254e;
}

.chat-markdown :deep(pre) {
  margin: 8px 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: #f6f8fa;
  border: 1px solid rgba(127, 127, 127, 0.18);
  overflow-x: auto;
  max-width: 100%;
}

.chat-markdown :deep(pre code) {
  display: block;
  padding: 0;
  background: transparent;
  color: #24292e;
  font-size: 12.5px;
  line-height: 1.6;
  white-space: pre;
}

.chat-markdown :deep(table) {
  font-size: 12.5px;
  margin: 8px 0;
  border-collapse: collapse;
  display: block;
  overflow-x: auto;
  max-width: 100%;
}

.chat-markdown :deep(th),
.chat-markdown :deep(td) {
  border: 1px solid rgba(127, 127, 127, 0.3);
  padding: 4px 8px;
  text-align: left;
}

.chat-markdown :deep(th) {
  background: rgba(127, 127, 127, 0.1);
  font-weight: 600;
}

.chat-markdown :deep(blockquote) {
  margin: 8px 0;
  padding: 4px 12px;
  border-left: 3px solid var(--bs-brand, #2080f0);
  background: rgba(32, 128, 240, 0.06);
  color: var(--chat-text-weak, #5c6b7a);
}

.chat-markdown :deep(hr) {
  border: none;
  border-top: 1px solid rgba(127, 127, 127, 0.25);
  margin: 12px 0;
}

.chat-markdown :deep(img) {
  max-width: 100%;
  border-radius: 6px;
}

.chat-markdown-fallback {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 0;
  background: transparent;
  border: none;
  padding: 0;
  font-family: inherit;
  font-size: 13.5px;
  line-height: 1.72;
}

/* ── 深色模式适配（由 <html theme-mode="dark"> 驱动）── */
:global(html[theme-mode='dark']) .chat-markdown {
  color: #d8dee9;
}

:global(html[theme-mode='dark']) .chat-markdown :deep(code) {
  background: rgba(255, 255, 255, 0.12);
  color: #ff7b72;
}

:global(html[theme-mode='dark']) .chat-markdown :deep(pre) {
  background: #1b1f27;
  border-color: rgba(255, 255, 255, 0.12);
}

:global(html[theme-mode='dark']) .chat-markdown :deep(pre code) {
  color: #e6edf3;
}

:global(html[theme-mode='dark']) .chat-markdown :deep(th) {
  background: rgba(255, 255, 255, 0.07);
}

:global(html[theme-mode='dark']) .chat-markdown :deep(th),
:global(html[theme-mode='dark']) .chat-markdown :deep(td) {
  border-color: rgba(255, 255, 255, 0.18);
}

:global(html[theme-mode='dark']) .chat-markdown :deep(blockquote) {
  background: rgba(32, 128, 240, 0.12);
  color: #9fb0c3;
}
</style>
