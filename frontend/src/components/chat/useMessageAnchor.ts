import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { ChatMessage } from './types'

/**
 * 消息锚点导航 Composable
 * 功能：
 * - 监听滚动，自动高亮当前可见消息
 * - URL hash 同步（深度链接分享）
 * - 键盘导航（j/k 上下跳转）
 * - 生成锚点导航栏数据
 */
export function useMessageAnchor(messages: ChatMessage[]) {
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

  // 获取消息元素
  function getElement(id: string): HTMLElement | undefined {
    return messageElements.get(id)
  }

  // 滚动到指定消息
  function scrollToMessage(id: string, behavior: ScrollBehavior = 'smooth') {
    const el = messageElements.get(id)
    if (!el) return false

    isScrolling = true
    activeAnchor.value = id

    // 更新 URL hash（不触发页面跳转）
    history.replaceState(null, '', `#${id}`)

    el.scrollIntoView({ behavior, block: 'center', inline: 'nearest' })

    // 滚动结束后重置标志
    if (scrollTimeout) clearTimeout(scrollTimeout)
    scrollTimeout = setTimeout(() => {
      isScrolling = false
    }, 500)

    return true
  }

  // 滚动到下一条消息
  function scrollToNext() {
    const ids = messages.map(m => m.id)
    const currentIndex = activeAnchor.value ? ids.indexOf(activeAnchor.value) : -1
    const nextIndex = Math.min(currentIndex + 1, ids.length - 1)
    if (nextIndex >= 0) {
      scrollToMessage(ids[nextIndex])
    }
  }

  // 滚动到上一条消息
  function scrollToPrev() {
    const ids = messages.map(m => m.id)
    const currentIndex = activeAnchor.value ? ids.indexOf(activeAnchor.value) : ids.length
    const prevIndex = Math.max(currentIndex - 1, 0)
    if (prevIndex >= 0) {
      scrollToMessage(ids[prevIndex])
    }
  }

  // 滚动到最新消息
  function scrollToLatest() {
    if (messages.length > 0) {
      scrollToMessage(messages[0].id) // messages[0] 是最新的（倒序）
    }
  }

  // 滚动到最旧消息
  function scrollToOldest() {
    if (messages.length > 0) {
      scrollToMessage(messages[messages.length - 1].id)
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
      // 静默更新 URL hash（不触发滚动）
      history.replaceState(null, '', `#${closestId}`)
    }
  }

  // 处理 URL hash 变化（深度链接进入）
  function handleHashChange() {
    const hash = window.location.hash.slice(1) // 移除 #
    if (hash && messageElements.has(hash)) {
      // 延迟滚动，等待 DOM 就绪
      setTimeout(() => {
        scrollToMessage(hash, 'auto') // 即时滚动，不动画
      }, 100)
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
      case 'g':
        if (e.shiftKey) {
          e.preventDefault()
          scrollToOldest()
        } else {
          e.preventDefault()
          scrollToLatest()
        }
        break
      case 'Home':
        e.preventDefault()
        scrollToLatest()
        break
      case 'End':
        e.preventDefault()
        scrollToOldest()
        break
    }
  }

  // 生成导航栏数据
  const anchorNavItems = computed(() => {
    return messages
      .filter(m => m.role === 'assistant' || m.content.length > 100) // 只显示 AI 回复或长用户消息
      .slice(0, 20) // 最多 20 个
      .map((msg, index) => ({
        id: msg.id,
        label: msg.role === 'assistant'
          ? `AI 回复 ${messages.length - index}`
          : `用户: ${msg.content.slice(0, 30)}...`,
        time: msg.datetime,
        role: msg.role,
        isActive: activeAnchor.value === msg.id,
      }))
  })

  // 生命周期
  onMounted(() => {
    initObserver()
    window.addEventListener('hashchange', handleHashChange)
    window.addEventListener('keydown', handleKeydown)

    // 初始 hash
    handleHashChange()
  })

  onUnmounted(() => {
    if (observer) {
      observer.disconnect()
      observer = null
    }
    window.removeEventListener('hashchange', handleHashChange)
    window.removeEventListener('keydown', handleKeydown)
    if (scrollTimeout) clearTimeout(scrollTimeout)
  })

  // 当 messages 变化时，重新观察新元素
  watch(() => messages.length, () => {
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