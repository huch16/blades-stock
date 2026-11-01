<template>
  <CollapsibleSection
    :title="`🔧 工具调用 (${calls.length})`"
    :expanded="expanded"
    @toggle="$emit('toggle', !expanded)"
    class="section-tools"
  >
    <div class="tools-timeline">
      <TransitionGroup name="tool-item" tag="div" class="tools-timeline-inner">
        <div
          v-for="(call, idx) in calls"
          :key="call.id || `call-${idx}`"
          class="tool-item"
          :class="[call.status, { 'tool-item-streaming': isStreaming(call) }]"
        >
          <!-- 时间轴连接线 -->
          <div class="timeline-connector" v-if="idx < calls.length - 1">
            <div class="connector-line" :class="{ 'connector-active': isStreaming(call) }" />
            <div class="connector-dot" :class="call.status" />
          </div>

          <div class="tool-card">
            <!-- 头部：图标、名称、状态、耗时 -->
            <div class="tool-header">
              <div class="tool-icon-wrapper">
                <span class="tool-icon">{{ getToolIcon(call.function.name) }}</span>
                <span v-if="isStreaming(call)" class="streaming-ring" aria-hidden="true"></span>
              </div>
              <div class="tool-info">
                <span class="tool-name">{{ call.function.name }}</span>
                <span class="tool-status" :class="call.status">
                  <span class="status-dot" aria-hidden="true"></span>
                  {{ getToolCallStatusText(call.status) }}
                </span>
              </div>
              <div class="tool-meta">
                <span class="tool-time" v-if="call.duration">{{ formatDuration(call.duration) }}</span>
                <span class="tool-index">#{{ idx + 1 }}</span>
              </div>
            </div>

            <!-- 进度条（执行中时显示） -->
            <div v-if="isStreaming(call)" class="tool-progress">
              <div class="progress-bar">
                <div class="progress-fill" :style="{ width: progressWidth(call) + '%' }" />
              </div>
              <span class="progress-text">执行中...</span>
            </div>

            <!-- 参数详情 -->
            <details class="tool-details" v-if="showArgs[call.id || idx]" open>
              <summary class="details-summary">
                <span>📥 参数</span>
                <t-button size="tiny" variant="text" theme="default" @click.stop="handleCopyArgs(call)">
                  复制
                </t-button>
              </summary>
              <pre class="tool-args">{{ formatArgs(call.function.arguments) }}</pre>
            </details>

            <!-- 结果详情 -->
            <details class="tool-details" v-if="call.result && showResult[call.id || idx]">
              <summary class="details-summary">
                <span>📤 结果</span>
                <t-button size="tiny" variant="text" theme="default" @click.stop="handleCopyResult(call)">
                  复制
                </t-button>
              </summary>
              <pre class="tool-result">{{ call.result }}</pre>
            </details>

            <!-- 错误信息 -->
            <div v-if="call.status === 'error' && call.result" class="tool-error">
              <span class="error-icon">⚠️</span>
              <pre class="error-text">{{ call.result }}</pre>
            </div>
          </div>
        </div>
      </TransitionGroup>

      <!-- 空状态 -->
      <div v-if="calls.length === 0" class="tools-empty">
        暂无工具调用记录
      </div>
    </div>
  </CollapsibleSection>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import CollapsibleSection from './CollapsibleSection.vue'
import {
  formatDuration,
  getToolCallStatusText,
  type ToolCall
} from './types'

const props = defineProps<{
  calls: ToolCall[]
  expanded?: boolean
}>()

const emit = defineEmits<{
  toggle: [expanded: boolean]
  'copy-args': [args: string]
  'copy-result': [result: string]
}>()

// 展开状态
const internalExpanded = ref(props.expanded ?? false)
const expanded = computed({
  get() {
    return props.expanded !== undefined ? props.expanded : internalExpanded.value
  },
  set(val: boolean) {
    if (props.expanded === undefined) internalExpanded.value = val
    emit('toggle', val)
  }
})

// 追踪哪些详情已展开
const showArgs = ref<Record<string | number, boolean>>({})
const showResult = ref<Record<string | number, boolean>>({})

// 判断是否正在流式执行
function isStreaming(call: ToolCall): boolean {
  return call.status === 'pending'
}

// 进度条宽度（模拟）
const progressMap = ref<Record<string | number, number>>({})
const activeTimers = new Map<string | number, ReturnType<typeof setInterval>>()

function progressWidth(call: ToolCall): number {
  const key = call.id || props.calls.indexOf(call)
  return progressMap.value[key] || 0
}

// 模拟进度动画
onMounted(() => {
  props.calls.forEach((call, idx) => {
    if (isStreaming(call)) {
      const key = call.id || idx
      let progress = 0
      const interval = setInterval(() => {
        progress += Math.random() * 15 + 5
        if (progress >= 100) {
          progress = 100
          clearInterval(interval)
          activeTimers.delete(key)
        }
        progressMap.value[key] = progress
      }, 200)
      activeTimers.set(key, interval)
    }
  })
})

onBeforeUnmount(() => {
  activeTimers.forEach((timer) => clearInterval(timer))
  activeTimers.clear()
})

function formatArgs(args: string): string {
  try {
    return JSON.stringify(JSON.parse(args), null, 2)
  } catch {
    return args
  }
}

function getToolIcon(name: string): string {
  const icons: Record<string, string> = {
    'get_stock_data': '📈',
    'get_kline': '📊',
    'search_web': '🔍',
    'search_news': '📰',
    'analyze_stock': '🧠',
    'backtest': '📉',
    'calculate': '🧮',
  }
  return icons[name] || '🔧'
}

function handleCopyArgs(call: ToolCall) {
  const args = formatArgs(call.function.arguments)
  emit('copy-args', args)
}

function handleCopyResult(call: ToolCall) {
  if (call.result) {
    emit('copy-result', call.result)
  }
}
</script>

<style scoped lang="less">
/* 时间轴容器 */
.tools-timeline {
  padding: 4px 0 4px 20px; /* 左侧留出连接线空间 */
}

.tools-timeline-inner {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

/* 连接线 */
.timeline-connector {
  position: absolute;
  left: -20px;
  top: 0;
  bottom: 0;
  width: 20px;
  pointer-events: none;
}

.connector-line {
  position: absolute;
  left: 9px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--td-component-border);
  transition: background 0.3s;
}

.connector-line.connector-active {
  background: var(--td-brand-color, #0052d9);
  background: linear-gradient(
    180deg,
    var(--td-brand-color) 0%,
    var(--td-brand-color-light) 100%
  );
}

.connector-dot {
  position: absolute;
  left: 6px;
  top: 8px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--td-text-color-disabled);
  border: 2px solid var(--td-bg-color-container);
  transition: all 0.3s;
}

.connector-dot.pending {
  background: #e6a23c;
  box-shadow: 0 0 6px rgba(230, 162, 60, 0.5);
  animation: dot-pulse 1.2s infinite;
}

.connector-dot.success {
  background: #67c23a;
  box-shadow: 0 0 6px rgba(103, 194, 58, 0.5);
}

.connector-dot.error {
  background: #f56c6c;
  box-shadow: 0 0 6px rgba(245, 108, 108, 0.5);
}

@keyframes dot-pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.2); }
}

/* 工具卡片 */
.tool-item {
  position: relative;
  animation: tool-item-enter 0.3s ease-out;
}

@keyframes tool-item-enter {
  from {
    opacity: 0;
    transform: translateX(-20px);
  }
  to {
    opacity: 1;
    transform: translateX(0);
  }
}

.tool-card {
  background: var(--td-bg-color-container);
  border: 1px solid var(--td-component-border);
  border-radius: 8px;
  overflow: hidden;
  transition: all 0.2s;

  &:hover {
    box-shadow: 0 4px 12px rgba(0,0,0,0.06);
  }
}

.tool-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: var(--td-bg-color-component);
  border-bottom: 1px solid var(--td-component-border);
}

.tool-icon-wrapper {
  position: relative;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 6px;
  flex-shrink: 0;
}

.tool-icon {
  font-size: 14px;
  z-index: 1;
}

.streaming-ring {
  position: absolute;
  inset: -2px;
  border: 2px solid var(--td-brand-color);
  border-radius: 8px;
  border-right-color: transparent;
  border-bottom-color: transparent;
  animation: ring-spin 1s linear infinite;
}

@keyframes ring-spin {
  to { transform: rotate(360deg); }
}

.tool-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tool-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--td-text-color-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tool-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 10px;
  width: fit-content;
}

.tool-status .status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  animation: status-dot-pulse 1.5s infinite;
}

.tool-status.pending {
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.15);
  border: 1px solid rgba(245, 158, 11, 0.25);
}

.tool-status.success {
  color: #10b981;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.25);
}

.tool-status.error {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.25);
}

@keyframes status-dot-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.tool-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  color: var(--td-text-color-disabled);
  flex-shrink: 0;
}

.tool-time {
  font-variant-numeric: tabular-nums;
}

.tool-index {
  opacity: 0.6;
}

/* 进度条 */
.tool-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: var(--td-brand-color-light, #e8f5e9);
  border-bottom: 1px solid var(--td-component-border);
}

.progress-bar {
  flex: 1;
  height: 4px;
  background: var(--td-border-level-1-color);
  border-radius: 2px;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--td-brand-color), var(--td-brand-color-hover));
  border-radius: 2px;
  transition: width 0.3s ease;
  animation: progress-shimmer 1.5s infinite;
}

@keyframes progress-shimmer {
  0% { box-shadow: 0 0 0 transparent; }
  50% { box-shadow: 0 0 8px var(--td-brand-color); }
  100% { box-shadow: 0 0 0 transparent; }
}

.progress-text {
  font-size: 11px;
  color: var(--td-brand-color);
  font-weight: 500;
}

/* 详情折叠 */
.tool-details {
  padding: 0 12px 8px;
}

.details-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  cursor: pointer;
  font-size: 12px;
  font-weight: 500;
  color: var(--td-text-color-secondary);
  user-select: none;
  list-style: none;

  &::-webkit-details-marker { display: none; }

  &:hover { color: var(--td-brand-color); }
}

.tool-args,
.tool-result {
  margin-top: 6px;
  padding: 10px;
  background: var(--td-bg-color-secondarycontainer, rgba(0,0,0,0.03));
  border-radius: 6px;
  font-size: 11px;
  line-height: 1.5;
  overflow-x: auto;
  max-height: 300px;
  white-space: pre-wrap;
  word-break: break-word;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
}

.tool-result {
  background: var(--td-success-color-light, rgba(34, 197, 94, 0.08));
}

/* 错误状态 */
.tool-error {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  background: var(--td-error-color-light, rgba(245, 108, 108, 0.08));
  border-top: 1px solid var(--td-error-color-light, rgba(245, 108, 108, 0.2));
}

.error-icon { font-size: 14px; flex-shrink: 0; }

.error-text {
  flex: 1;
  margin: 0;
  font-size: 11px;
  color: var(--td-error-color, #f56c6c);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  white-space: pre-wrap;
  word-break: break-word;
}

/* 空状态 */
.tools-empty {
  text-align: center;
  padding: 24px;
  color: var(--td-text-color-disabled);
  font-size: 13px;
}

/* 进入/离开动画 */
.tool-item-enter-active,
.tool-item-leave-active {
  transition: all 0.3s ease;
}

.tool-item-enter-from,
.tool-item-leave-to {
  opacity: 0;
  transform: translateX(-20px);
}

.tool-item-leave-active {
  position: absolute;
  width: 100%;
}

/* 减少动画偏好 */
@media (prefers-reduced-motion: reduce) {
  .tool-item,
  .streaming-ring,
  .progress-fill,
  .connector-dot.pending,
  .status-dot {
    animation: none !important;
  }
  .tool-item-enter-active {
    transition: none;
  }
}
</style>