<template>
  <div class="chat-sender" :class="{ 'chat-sender--loading': loading }">
    <div class="chat-sender-body">
      <textarea
        ref="textareaRef"
        class="chat-sender-textarea"
        :value="modelValue"
        :placeholder="placeholder"
        :rows="minRows"
        :disabled="disabled"
        @input="handleInput"
        @keydown="handleKeydown"
        @compositionstart="isComposing = true"
        @compositionend="isComposing = false"
      ></textarea>

      <div class="chat-sender-actions">
        <slot name="suffix"></slot>
      </div>
    </div>
    <div class="chat-sender-hint">
      Enter 发送 · Shift + Enter 换行
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * ChatSender —— 零外部 UI 库依赖的对话输入框
 *
 * 用于替代 @tdesign-vue-next/chat 的 <t-chat-sender>。
 * 该替换是「拆除 TDesign 355KB 双 UI 库税」的关键一步：
 * t-chat-sender 的 import 会把整个 @tdesign-vue-next/chat + tdesign-vue-next
 * (node_modules 222MB，产物 raw 1.17MB / br 355KB) 拖入 Agent 路由。
 * 本组件只依赖原生 textarea + Vue，零新增依赖，同时补齐 TDesign 缺失的组合输入
 * (IME) 保护与自适应高度能力。
 */
import { ref, watch, nextTick, onMounted } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    loading?: boolean
    disabled?: boolean
    minRows?: number
    maxRows?: number
  }>(),
  {
    placeholder: '',
    loading: false,
    disabled: false,
    minRows: 1,
    maxRows: 6
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  send: []
  stop: []
}>()

const textareaRef = ref<HTMLTextAreaElement | null>(null)
const isComposing = ref(false)

/** 自适应高度：按内容滚动高度在 minRows/maxRows 之间伸缩 */
function autoResize() {
  const el = textareaRef.value
  if (!el) return
  const style = window.getComputedStyle(el)
  const lineHeight = parseFloat(style.lineHeight) || 22
  const paddingTop = parseFloat(style.paddingTop) || 0
  const paddingBottom = parseFloat(style.paddingBottom) || 0
  const borderY = (parseFloat(style.borderTopWidth) || 0) + (parseFloat(style.borderBottomWidth) || 0)

  const minHeight = lineHeight * props.minRows + paddingTop + paddingBottom + borderY
  const maxHeight = lineHeight * props.maxRows + paddingTop + paddingBottom + borderY

  el.style.height = 'auto'
  const next = Math.min(Math.max(el.scrollHeight, minHeight), maxHeight)
  el.style.height = `${next}px`
  el.style.overflowY = el.scrollHeight > maxHeight ? 'auto' : 'hidden'
}

function handleInput(event: Event) {
  const target = event.target as HTMLTextAreaElement
  emit('update:modelValue', target.value)
  autoResize()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key !== 'Enter') return
  // 组合输入（中文/日文 IME）过程中绝不触发发送，避免候选词确认被误判为回车发送
  if (isComposing.value || event.isComposing || (event as any).keyCode === 229) return
  // Shift + Enter 插入换行，交给浏览器默认行为
  if (event.shiftKey || event.ctrlKey || event.metaKey || event.altKey) return

  event.preventDefault()
  if (props.loading) {
    emit('stop')
    return
  }
  emit('send')
}

watch(
  () => props.modelValue,
  () => nextTick(autoResize)
)

onMounted(() => {
  autoResize()
})
</script>

<style scoped>
.chat-sender {
  width: 100%;
}

.chat-sender-body {
  display: flex;
  align-items: flex-end;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--td-border-level-2-color, #dcdcdc);
  border-radius: 12px;
  background: var(--td-bg-color-container, #ffffff);
  transition: border-color 0.2s, box-shadow 0.2s;
}

.chat-sender-body:focus-within {
  border-color: var(--td-brand-color, #0052d9);
  box-shadow: 0 0 0 3px var(--td-brand-color-focus, rgba(0, 82, 217, 0.14));
}

.chat-sender--loading .chat-sender-body {
  border-color: #ffb020;
}

.chat-sender-textarea {
  flex: 1 1 auto;
  min-width: 0;
  resize: none;
  border: none;
  outline: none;
  background: transparent;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.6;
  color: var(--td-text-color-primary, #1f2329);
  padding: 6px 2px;
  max-height: 200px;
  overflow-y: hidden;
}

.chat-sender-textarea::placeholder {
  color: #9aa4b2;
}

.chat-sender-textarea:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.chat-sender-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 0 0 auto;
  padding-bottom: 2px;
}

.chat-sender-hint {
  margin-top: 4px;
  font-size: 11px;
  color: #9aa4b2;
  text-align: right;
  user-select: none;
}
</style>
