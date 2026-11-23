import { ref, computed, watch, onMounted, onUnmounted, unref, type Ref, type ComputedRef } from 'vue'
import type { ChatMessage } from './types'

export type MaybeRefOrGetter<T> = T | Ref<T> | ComputedRef<T> | (() => T)

/**
 * 消息元素锚点注册/注销签名（供 provide/inject 跨组件传递）。
 *
 * 修复（@typescript-eslint/no-unsafe-function-type）：注入点此前写作内置 `Function`，
 * 导致子组件调用 registerElement(...) 时参数完全不校验。
 */
export type RegisterMessageElement = (id: string, el: HTMLElement) => void
export type UnregisterMessageElement = (id: string) => void

function toValue<T>(source: MaybeRefOrGetter<T>): T {
  return typeof source === 'function' ? (source as () => T)() : unref(source)
}

/**
 * 消息锚点导航 Composable
 * 功能：
 * - 监听滚动，自动高亮当前可见消息
 * - URL hash 同步（深度链接分享）
 * - 键盘导航（j/k 上下跳转）
 * - 生成锚点导航栏数据
 */
export function useMessageAnchor(messagesSource: MaybeRefOrGetter<ChatMessage[]>) {
  function getMessages(): ChatMessage[] {
    const list = toValue(messagesSource)
    return Array.isArray(list) ? list : []
  }

  // 当前高亮的消息 ID
  const activeAnchor = ref<string | null>(null)

  // 是否正在平滑滚动（避免滚动监听干扰）
  let isScrolling = false
  let scrollTimeout: ReturnType<typeof setTimeout> | null = null

  // 消息元素引用 Map
  const messageElements = new Map<string, HTMLElement>()

  // 注册消息元素
  function registerElement(id: string, el: HTMLElement) {
    messageElements.set(id, el)
  }

  // 注销消息元素
  function unregisterElement(id: string) {
    messageElements.delete(id)
  }

  // 滚动到指定消息
  function scrollToMessage(id: string, behavior: ScrollBehavior = 'smooth') {
    const el = messageElements.get(id)
    if (!el) return false

    isScrolling = true
    activeAnchor.value = id

    // 注意：SPA 使用 createWebHashHistory，绝对禁止修改 window.location.hash，否则会彻底冲垮路由并触发原生锚点跳跃！
    el.scrollIntoView({ behavior, block: 'start', inline: 'nearest' })

    // 滚动结束后重置标志
    if (scrollTimeout) clearTimeout(scrollTimeout)
    scrollTimeout = setTimeout(() => {
      isScrolling = false
    }, 500)

    return true
  }

  // 滚动到下一条消息
  function scrollToNext() {
    const msgs = getMessages()
    const ids = msgs.map(m => m.id)
    const currentIndex = activeAnchor.value ? ids.indexOf(activeAnchor.value) : -1
    const nextIndex = Math.min(currentIndex + 1, ids.length - 1)
    if (nextIndex >= 0 && nextIndex < ids.length) {
      scrollToMessage(ids[nextIndex])
    }
  }

  // 滚动到上一条消息
  function scrollToPrev() {
    const msgs = getMessages()
    const ids = msgs.map(m => m.id)
    const currentIndex = activeAnchor.value ? ids.indexOf(activeAnchor.value) : ids.length
    const prevIndex = Math.max(currentIndex - 1, 0)
    if (prevIndex >= 0 && prevIndex < ids.length) {
      scrollToMessage(ids[prevIndex])
    }
  }

  // 滚动到最新消息 (自然时间序中最后一个是最新)
  function scrollToLatest() {
    const msgs = getMessages()
    if (msgs.length > 0) {
      scrollToMessage(msgs[msgs.length - 1].id)
    }
  }

  // 滚动到最旧消息 (自然时间序中第一个是最旧)
  function scrollToOldest() {
    const msgs = getMessages()
    if (msgs.length > 0) {
      scrollToMessage(msgs[0].id)
    }
  }

  // 计算当前可见消息（基于 IntersectionObserver）
  const visibleMessages = ref<Set<string>>(new Set())

  // 交叉观察器
  let observer: IntersectionObserver | null = null

  function initObserver() {
    if (typeof IntersectionObserver === 'undefined') return

    observer = new IntersectionObserver(
      (entries) => {
        if (isScrolling) return

        entries.forEach((entry) => {
          const id = entry.target.id
          if (!id) return

          if (entry.isIntersecting) {
            visibleMessages.value.add(id)
          } else {
            visibleMessages.value.delete(id)
          }
        })

        // 更新 activeAnchor：选择最靠近视口中心的消息
        updateActiveAnchor()
      },
      {
        root: null, // viewport
        rootMargin: '-20% 0px -20% 0px', // 视口中间 60% 区域
        threshold: [0, 0.25, 0.5, 0.75, 1],
      }
    )

    // 观察所有已注册元素
    messageElements.forEach((el) => {
      if (el.id) observer!.observe(el)
    })
  }

  function updateActiveAnchor() {
    if (visibleMessages.value.size === 0) return

    let closestId: string | null = null
    let minDistance = Infinity

    visibleMessages.value.forEach((id) => {
      const el = messageElements.get(id)
      if (!el) return

      const rect = el.getBoundingClientRect()
      const viewportCenter = window.innerHeight / 2
      const elementCenter = rect.top + rect.height / 2
      const distance = Math.abs(elementCenter - viewportCenter)

      if (distance < minDistance) {
        minDistance = distance
        closestId = id
      }
    })

    if (closestId && closestId !== activeAnchor.value) {
      activeAnchor.value = closestId
      // 仅更新内存态指示器，绝对不覆写 URL hash，避免冲垮路由与触发浏览器原生跳跃
    }
  }

  // 键盘导航
  function handleKeydown(e: KeyboardEvent) {
    // 只在未聚焦输入框时响应
    if (e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        e.target?.getAttribute('contenteditable') === 'true') {
      return
    }

    switch (e.key) {
      case 'j':
      case 'ArrowDown':
        e.preventDefault()
        scrollToNext()
        break
      case 'k':
      case 'ArrowUp':
        e.preventDefault()
        scrollToPrev()
        break
      // g / Shift+G 按常规语义（与 vim 的 gg/G、Home/End 一致）：
      // g 回顶部 = 最旧消息，Shift+G 到底部 = 最新消息。
      // 此前两者写反了（g 到最新、Shift+G 到最旧）：虽与 UI 标注一致，
      // 但与 Home（最旧）/End（最新）互相矛盾，属有意设计还是写反已无从分辨，
      // 第四轮决定统一为常规语义，文案同步改。
      case 'g':
        if (e.shiftKey) {
          e.preventDefault()
          scrollToLatest()
        } else {
          e.preventDefault()
          scrollToOldest()
        }
        break
      // Home/End 按平台标准语义：聊天区是自然时间线（旧消息在上、
      // 新消息在下），Home 回顶部 = 最旧消息，End 到底部 = 最新消息。
      // 此前两者写反了（Home 去了最新、End 去了最旧）。
      case 'Home':
        e.preventDefault()
        scrollToOldest()
        break
      case 'End':
        e.preventDefault()
        scrollToLatest()
        break
    }
  }

  // 生成导航栏数据
  const anchorNavItems = computed(() => {
    const msgs = getMessages()
    // 编号必须用消息在完整时间线里的真实位置（idx + 1），不能用过滤后
    // 列表的下标：过滤会跳过短用户消息，`msgs.length - index` 算出来的
    // “AI 回复 N”与实际第 N 条消息对不上。
    return msgs
      .map((msg, idx) => ({ msg, idx }))
      .filter(({ msg }) => msg.role === 'assistant' || msg.content.length > 100) // 只显示 AI 回复或长用户消息
      .slice(0, 20) // 最多 20 个
      .map(({ msg, idx }) => ({
        id: msg.id,
        label: msg.role === 'assistant'
          ? `AI 回复 ${idx + 1}`
          : `用户: ${msg.content.slice(0, 30)}...`,
        time: msg.datetime,
        role: msg.role,
        isActive: activeAnchor.value === msg.id,
      }))
  })

  // 生命周期
  onMounted(() => {
    initObserver()
    window.addEventListener('keydown', handleKeydown)
  })

  onUnmounted(() => {
    if (observer) {
      observer.disconnect()
      observer = null
    }
    window.removeEventListener('keydown', handleKeydown)
    if (scrollTimeout) clearTimeout(scrollTimeout)
  })

  // 当 messages 变化时，重新观察新元素
  watch(() => getMessages().length, () => {
    if (observer) {
      messageElements.forEach((el) => {
        if (el.id) observer.observe(el)
      })
    }
  })

  return {
    activeAnchor,
    visibleMessages,
    anchorNavItems,
    registerElement,
    unregisterElement,
    scrollToMessage,
    scrollToNext,
    scrollToPrev,
    scrollToLatest,
    scrollToOldest,
  }
}

/** 锚点导航项类型 */
export interface AnchorNavItem {
  id: string
  label: string
  time: string
  role: 'assistant' | 'user'
  isActive: boolean
}