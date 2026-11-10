<template>
  <div class="collapsible-section">
    <div
      class="collapsible-header"
      @click="handleToggle"
      role="button"
      tabindex="0"
      @keydown.enter="handleToggle"
      @keydown.space.prevent="handleToggle"
    >
      <svg
        class="collapsible-arrow"
        :class="{ expanded }"
        viewBox="0 0 24 24"
        width="16"
        height="16"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M9 6l6 6-6 6z" />
      </svg>
      <span class="collapsible-title">{{ title }}</span>
    </div>
    <transition name="collapse">
      <div v-show="expanded" class="collapsible-content">
        <slot />
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = defineProps<{
  title: string
  expanded?: boolean
  defaultExpanded?: boolean
}>()

const emit = defineEmits<{
  toggle: [expanded: boolean]
}>()

// 内部状态：优先使用受控 props，否则用内部 ref
const isControlled = computed(() => props.expanded !== undefined)
const internalExpanded = ref(props.defaultExpanded ?? false)

const expanded = computed({
  get() {
    return isControlled.value ? (props.expanded as boolean) : internalExpanded.value
  },
  set(val: boolean) {
    if (!isControlled.value) internalExpanded.value = val
    emit('toggle', val)
  }
})

function handleToggle() {
  expanded.value = !expanded.value
}

// 同步外部 props 变化
watch(() => props.expanded, (val) => {
  if (val !== undefined && !isControlled.value) {
    internalExpanded.value = val
  }
}, { immediate: true })
</script>

<style scoped>
.collapsible-section {
  border: 1px solid var(--bs-border-component);
  border-radius: 8px;
  background: var(--bs-bg-container-hover);
  overflow: hidden;
}

.collapsible-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  cursor: pointer;
  font-size: 13px;
  font-weight: 500;
  color: var(--bs-text-secondary);
  background: linear-gradient(135deg, rgba(56, 173, 169, 0.06) 0%, rgba(46, 139, 87, 0.06) 100%);
  border-bottom: 1px solid var(--bs-border-component);
  user-select: none;
  transition: background 0.2s;

  &:hover {
    background: linear-gradient(135deg, rgba(56, 173, 169, 0.12) 0%, rgba(46, 139, 87, 0.12) 100%);
  }

  &:focus-visible {
    outline: 2px solid var(--bs-brand);
    outline-offset: -2px;
  }
}

.collapsible-arrow {
  transition: transform 0.2s;
  flex-shrink: 0;
  color: var(--bs-text-disabled);
}

.collapsible-arrow.expanded {
  transform: rotate(90deg);
}

.collapsible-title {
  font-weight: 500;
}

.collapsible-content {
  padding: 14px;
  max-height: 400px;
  overflow-y: auto;
  font-size: 13px;
  line-height: 1.6;
}

/* CSS transition for v-show (纯透明度过渡，绝对不使用 translateY 抖动位移) */
.collapse-enter-active,
.collapse-leave-active {
  transition: opacity 0.15s ease;
}

.collapse-enter-from,
.collapse-leave-to {
  opacity: 0;
}
</style>