<template>
  <Transition name="anchor-nav">
    <aside
      v-if="showNav && navItems.length > 0"
      class="message-anchor-nav"
      :class="{ collapsed }"
      role="navigation"
      aria-label="消息导航"
    >
      <!-- 折叠/展开切换 -->
      <button
        class="nav-toggle"
        @click="collapsed = !collapsed"
        :aria-expanded="!collapsed"
        :aria-label="collapsed ? '展开导航' : '折叠导航'"
      >
        <svg v-if="collapsed" viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
          <path d="M15.41 16.59L10.83 12l4.58-4.59L14 6l-6 6 6 6 1.41-1.41z"/>
        </svg>
        <svg v-else viewBox="0 0 24 24" width="18" height="18" fill="currentColor">
          <path d="M8.59 16.59L13.17 12 8.59 7.41 10 6l6 6-6 6-1.41-1.41z"/>
        </svg>
      </button>

      <!-- 导航列表 -->
      <div class="nav-list" v-show="!collapsed">
        <div class="nav-header">
          <span class="nav-title">消息导航</span>
          <span class="nav-count">{{ navItems.length }}</span>
        </div>

        <div class="nav-items" role="listbox" aria-label="消息列表">
          <button
            v-for="item in navItems"
            :key="item.id"
            class="nav-item"
            :class="{ active: item.isActive }"
            role="option"
            :aria-selected="item.isActive"
            @click="scrollToItem(item.id)"
            @mouseenter="hoveredId = item.id"
            @mouseleave="hoveredId = null"
          >
            <span class="nav-item-role" :class="item.role">
              {{ item.role === 'assistant' ? '🤖' : '👤' }}
            </span>
            <div class="nav-item-content">
              <span class="nav-item-label">{{ item.label }}</span>
              <span class="nav-item-time">{{ formatTime(item.time) }}</span>
            </div>
            <span v-if="item.isActive" class="nav-active-indicator" aria-hidden="true">▸</span>
          </button>
        </div>

        <!-- 底部快捷键提示 -->
        <div class="nav-shortcuts">
          <kbd>J</kbd>/<kbd>K</kbd> 上下
          <kbd>G</kbd> 最新
          <kbd>Shift+G</kbd> 最旧
        </div>
      </div>
    </aside>
  </Transition>

  <!-- 悬浮滚动按钮（导航折叠时显示） -->
  <Transition name="fab">
    <button
      v-if="collapsed || !showNav"
      class="fab-scroll"
      @click="scrollToLatest"
      :aria-label="'滚动到最新消息'"
      :title="'最新消息 (G)'"
    >
      <svg viewBox="0 0 24 24" width="20" height="20" fill="currentColor">
        <path d="M7.41 15.41L12 10.83l4.59 4.58L18 14l-6-6-6 6z"/>
      </svg>
      <span class="fab-unread" v-if="unreadCount > 0">{{ unreadCount > 9 ? '9+' : unreadCount }}</span>
    </button>
  </Transition>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { AnchorNavItem } from './useMessageAnchor'

interface Props {
  /** 导航项数据 */
  items: AnchorNavItem[]
  /** 是否显示导航 */
  show?: boolean
  /** 未读消息数 */
  unreadCount?: number
  /** 是否自动折叠（移动端） */
  autoCollapse?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  items: () => [],
  show: true,
  unreadCount: 0,
  autoCollapse: true,
})

const emit = defineEmits<{
  scroll: [id: string]
}>()

// 滚动到最新消息（同时只发 scroll 事件：父组件只监听 scroll，
// 旧代码额外发的 scrollToLatest 没有任何监听方，是死事件）
function scrollToLatest() {
  if (props.items.length > 0) {
    const last = props.items[props.items.length - 1]
    emit('scroll', last.id)
  }
}

// 折叠状态（默认折叠，保持主界面清爽清空）
const collapsed = ref(true)
const hoveredId = ref<string | null>(null)

// 移动端自动折叠
const isMobile = ref(false)

function checkMobile() {
  isMobile.value = window.innerWidth < 768
  if (props.autoCollapse && isMobile.value) {
    collapsed.value = true
  }
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})

// 滚动到指定项
function scrollToItem(id: string) {
  emit('scroll', id)
}

// 格式化时间
function formatTime(dt: string): string {
  if (!dt) return ''
  const date = new Date(dt)
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

// 计算导航项
const navItems = computed(() => props.items)

// 导航可见性
const showNav = computed(() => props.show && props.items.length > 0)
</script>

<style scoped>
/* 主容器 */
.message-anchor-nav {
  position: fixed;
  right: 16px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 100;
  background: var(--bs-bg-container);
  border: 1px solid var(--bs-border-component);
  border-radius: 12px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
  font-family: inherit;
  transition: width 0.3s ease, opacity 0.2s ease;
  overflow: hidden;

  /* 展开状态 */
  width: 280px;
  opacity: 1;

  /* 折叠状态 */
  &.collapsed {
    width: 48px;
    opacity: 0.9;
  }

  @media (max-width: 768px) {
    right: 8px;
    top: auto;
    bottom: 80px;
    transform: none;
    width: 44px;
    border-radius: 22px;

    &:not(.collapsed) {
      width: 260px;
      bottom: 80px;
      right: 8px;
      border-radius: 12px;
    }
  }
}

/* 折叠切换按钮 */
.nav-toggle {
  position: absolute;
  right: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bs-bg-component);
  border: none;
  border-left: 1px solid var(--bs-border-component);
  border-radius: 0 12px 12px 0;
  cursor: pointer;
  color: var(--bs-text-secondary);
  transition: all 0.2s;
  z-index: 10;

  &:hover {
    background: var(--bs-brand-light);
    color: var(--bs-brand);
  }

  &:focus-visible {
    outline: 2px solid var(--bs-brand);
    outline-offset: -2px;
  }

  @media (max-width: 768px) {
    top: auto;
    bottom: 0;
    transform: none;
    border-left: none;
    border-top: 1px solid var(--bs-border-component);
    border-radius: 0 0 12px 12px;
    width: 44px;
    height: 44px;
  }
}

.nav-toggle svg {
  transition: transform 0.3s ease;
}

.collapsed .nav-toggle svg {
  transform: rotate(180deg);
}

/* 导航列表 */
.nav-list {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.nav-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  border-bottom: 1px solid var(--bs-border-component);
  background: var(--bs-bg-component);
  flex-shrink: 0;
}

.nav-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--bs-text-primary);
}

.nav-count {
  font-size: 11px;
  color: var(--bs-text-disabled);
  background: var(--bs-bg-secondary);
  padding: 2px 8px;
  border-radius: 10px;
}

.nav-items {
  flex: 1;
  overflow-y: auto;
  padding: 6px;
  max-height: 400px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 10px;
  border-radius: 8px;
  background: transparent;
  border: none;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s;

  &:hover {
    background: var(--bs-bg-component-hover);
  }

  &:focus-visible {
    outline: 2px solid var(--bs-brand);
    outline-offset: -2px;
  }

  &.active {
    background: var(--bs-brand-light);
  }
}

.nav-item-role {
  font-size: 14px;
  flex-shrink: 0;
  width: 20px;
  text-align: center;
}

.nav-item-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.nav-item-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--bs-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.nav-item-time {
  font-size: 10px;
  color: var(--bs-text-disabled);
}

.nav-active-indicator {
  color: var(--bs-brand);
  font-size: 12px;
  flex-shrink: 0;
}

/* 快捷键提示 */
.nav-shortcuts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 10px;
  border-top: 1px solid var(--bs-border-component);
  background: var(--bs-bg-component);
  flex-shrink: 0;
  font-size: 10px;
  color: var(--bs-text-disabled);
}

.nav-shortcuts kbd {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 6px;
  background: var(--bs-bg-secondary);
  border: 1px solid var(--bs-border-component);
  border-radius: 4px;
  font-family: inherit;
  font-size: 10px;
  color: var(--bs-text-secondary);
  box-shadow: 0 1px 0 var(--bs-text-disabled);
}

/* 悬浮按钮 */
.fab-scroll {
  position: fixed;
  right: 16px;
  bottom: 80px;
  z-index: 90;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--bs-brand);
  color: #fff;
  border: none;
  box-shadow: 0 4px 16px rgba(0, 82, 217, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  animation: fab-in 0.3s ease-out;
  transition: transform 0.2s, box-shadow 0.2s;

  &:hover {
    transform: scale(1.05);
    box-shadow: 0 6px 20px rgba(0, 82, 217, 0.4);
  }

  &:active {
    transform: scale(0.95);
  }

  &:focus-visible {
    outline: 2px solid var(--bs-brand);
    outline-offset: 2px;
  }

  @media (max-width: 768px) {
    right: 12px;
    bottom: 100px;
    width: 44px;
    height: 44px;
  }
}

.fab-unread {
  position: absolute;
  top: -2px;
  right: -2px;
  min-width: 18px;
  height: 18px;
  padding: 0 4px;
  background: #f56c6c;
  color: #fff;
  border-radius: 9px;
  font-size: 10px;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px solid var(--bs-bg-container);
}

@keyframes fab-in {
  from {
    opacity: 0;
    transform: translateY(20px) scale(0.9);
  }
  to {
    opacity: 1;
    transform: translateY(0) scale(1);
  }
}

/* 过渡动画 */
.anchor-nav-enter-active,
.anchor-nav-leave-active {
  transition: all 0.3s ease;
}

.anchor-nav-enter-from {
  opacity: 0;
  transform: translateY(-50%) translateX(20px);
}

.anchor-nav-leave-to {
  opacity: 0;
  transform: translateY(-50%) translateX(20px);
}

.fab-enter-active,
.fab-leave-active {
  transition: all 0.2s ease;
}

.fab-enter-from {
  opacity: 0;
  transform: scale(0.8) translateY(10px);
}

.fab-leave-to {
  opacity: 0;
  transform: scale(0.8) translateY(10px);
}

/* 减少动画偏好 */
@media (prefers-reduced-motion: reduce) {
  .message-anchor-nav,
  .nav-toggle svg,
  .nav-item,
  .fab-scroll {
    transition: none !important;
    animation: none !important;
  }
}
</style>