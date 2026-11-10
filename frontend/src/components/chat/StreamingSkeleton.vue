<template>
  <div class="streaming-skeleton" :style="{ '--skeleton-lines': lines }">
    <!-- 打字机光标 -->
    <div class="typing-cursor" v-show="showCursor" aria-hidden="true"></div>

    <!-- 段落级骨架行 -->
    <div
      v-for="i in lineCount"
      :key="i"
      class="skeleton-line"
      :class="{ 'skeleton-line-last': i === lineCount }"
      :style="{
        '--delay': `${i * 80}ms`,
        '--width': lineWidths[i - 1] + '%'
      }"
    >
      <div class="skeleton-segment" />
    </div>

    <!-- 结束标记 -->
    <div v-if="showEndMark" class="end-mark" aria-hidden="true">
      <span class="end-dot"></span>
      <span class="end-dot"></span>
      <span class="end-dot"></span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'

interface Props {
  /** 预估行数 */
  lines?: number
  /** 是否显示打字机光标 */
  cursor?: boolean
  /** 完成时显示结束标记 */
  showEnd?: boolean
  /** 字符级打字机模式（而非行级） */
  charMode?: boolean
  /** 自定义内容用于计算真实宽度 */
  content?: string
}

const props = withDefaults(defineProps<Props>(), {
  lines: 5,
  cursor: true,
  showEnd: false,
  charMode: false,
  content: '',
})

const emit = defineEmits<{
  complete: []
}>()

const showCursor = ref(true)
const showEndMark = ref(false)
const lineCount = ref(0)
const lineWidths: number[] = []

let cursorTimer: ReturnType<typeof setInterval> | null = null
let typeTimer: ReturnType<typeof setTimeout> | null = null
let charIndex = 0

// 根据内容估算行数和宽度
function estimateLayout() {
  if (props.content && props.charMode) {
    // 字符级：根据内容长度估算
    const charsPerLine = 40 // 近似值
    const totalLines = Math.max(1, Math.ceil(props.content.length / charsPerLine))
    lineCount.value = Math.min(totalLines, 12)

    // 生成宽度分布
    lineWidths.length = 0
    for (let i = 0; i < lineCount.value; i++) {
      const remaining = props.content.length - i * charsPerLine
      const width = Math.min(100, (remaining / charsPerLine) * 100)
      lineWidths.push(Math.max(20, width))
    }
  } else {
    // 行级：固定行数，随机宽度模拟自然文本
    lineCount.value = props.lines
    lineWidths.length = 0
    for (let i = 0; i < props.lines; i++) {
      // 最后一行较短
      if (i === props.lines - 1) {
        lineWidths.push(40 + Math.random() * 40)
      } else {
        lineWidths.push(70 + Math.random() * 25)
      }
    }
  }
}

// 启动打字机光标闪烁
function startCursor() {
  if (!props.cursor) return
  cursorTimer = setInterval(() => {
    showCursor.value = !showCursor.value
  }, 530)
}

// 停止光标，显示结束标记
function stopCursor() {
  if (cursorTimer) {
    clearInterval(cursorTimer)
    cursorTimer = null
  }
  showCursor.value = false
  if (props.showEnd) {
    showEndMark.value = true
  }
  emit('complete')
}

// 字符级打字机动画
function startCharTypewriter() {
  if (!props.charMode || !props.content) return

  const text = props.content
  const totalChars = text.length
  const baseSpeed = 30 // ms per char
  const variance = 0.3

  function typeNext() {
    if (charIndex >= totalChars) {
      stopCursor()
      return
    }

    // 更新显示的字符数（通过 CSS 变量控制宽度）
    const progress = charIndex / totalChars
    const currentLine = Math.floor(progress * lineCount.value)
    if (currentLine < lineWidths.length) {
      lineWidths[currentLine] = Math.min(100, progress * 100 * (lineCount.value / (currentLine + 1)))
    }

    charIndex++
    // 人类打字速度变化
    const delay = baseSpeed * (1 + (Math.random() - 0.5) * variance * 2)
    typeTimer = setTimeout(typeNext, delay)
  }

  typeNext()
}

onMounted(() => {
  estimateLayout()
  startCursor()

  if (props.charMode) {
    // 字符级模式：延迟启动打字机
    setTimeout(startCharTypewriter, 300)
  } else {
    // 行级模式：逐行显示动画
    // lineCount 已通过响应式触发 CSS 动画
    // 完成后停止光标
    const totalTime = props.lines * 200 + 500
    setTimeout(stopCursor, totalTime)
  }
})

onUnmounted(() => {
  if (cursorTimer) clearInterval(cursorTimer)
  if (typeTimer) clearTimeout(typeTimer)
})

// 响应内容变化（流式更新时）
watch(() => props.content, (newContent) => {
  if (newContent && props.charMode) {
    estimateLayout()
  }
}, { deep: false })
</script>

<style scoped>
.streaming-skeleton {
  font-family: inherit;
  line-height: 1.7;
  min-height: 60px;
}

/* 光标 */
.typing-cursor {
  display: inline-block;
  width: 2px;
  height: 1.2em;
  background: var(--bs-brand, #0052d9);
  margin-left: 2px;
  animation: cursor-blink 1.06s infinite;
  vertical-align: text-bottom;
}

@keyframes cursor-blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

/* 骨架行 */
.skeleton-line {
  display: flex;
  align-items: center;
  height: 1.5em;
  margin: 4px 0;
  opacity: 0;
  transform: translateX(-20px);
  animation: line-fade-in 0.4s ease-out forwards;
  animation-delay: var(--delay, 0ms);
}

.skeleton-line-last {
  /* 最后一行特殊处理 */
}

.skeleton-segment {
  height: 0.9em;
  border-radius: 4px;
  background: linear-gradient(
    90deg,
    var(--bs-bg-secondary, rgba(140,140,140,0.08)) 0%,
    var(--bs-bg-tertiary, rgba(140,140,140,0.12)) 50%,
    var(--bs-bg-secondary, rgba(140,140,140,0.08)) 100%
  );
  background-size: 200% 100%;
  width: var(--width, 85%);
  animation: shimmer 1.5s infinite;
}

@keyframes line-fade-in {
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@keyframes shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}

/* 结束标记 */
.end-mark {
  display: flex;
  justify-content: center;
  gap: 4px;
  margin-top: 12px;
  padding-top: 8px;
  border-top: 1px dashed var(--bs-border-component);
  opacity: 0;
  animation: end-mark-in 0.3s ease-out 0.2s forwards;
}

.end-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--bs-text-disabled);
  animation: dot-bounce 0.6s infinite ease-in-out;
}

.end-dot:nth-child(2) { animation-delay: 0.1s; }
.end-dot:nth-child(3) { animation-delay: 0.2s; }

@keyframes end-mark-in {
  to { opacity: 1; }
}

@keyframes dot-bounce {
  0%, 80%, 100% { transform: scale(1); opacity: 0.5; }
  40% { transform: scale(1.3); opacity: 1; }
}

/* 减少动画偏好 */
@media (prefers-reduced-motion: reduce) {
  .typing-cursor,
  .skeleton-segment,
  .end-dot {
    animation: none !important;
  }
  .skeleton-line {
    animation: none !important;
    opacity: 1;
    transform: none;
  }
  .end-mark {
    animation: none !important;
    opacity: 1;
  }
}
</style>