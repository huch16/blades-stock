// 本地会话持久化：会话列表 CRUD、当前会话消息计算属性、localStorage 存取。
// 由 agent-chat.vue 抽出（R-22c），逻辑逐字搬运；共享状态由组件以 ctx 传入。
import { computed, nextTick, ref } from 'vue'
import { genMsgId, type ChatMessage } from '../components/chat'

// ========== 数据接口与持久化类型 ==========
export interface ChatSession {
  id: string
  title: string
  updatedAt: number
  messages: ChatMessage[]
}

export function useAgentChatSessions(ctx) {
  const showSessionDrawer = ctx.showSessionDrawer
  const scrollToBottom = ctx.scrollToBottom

const STORAGE_KEY = 'blades_stock_chat_sessions_v2'
const ACTIVE_SESSION_KEY = 'blades_stock_active_session_id_v2'
const sessions = ref<ChatSession[]>([])
const currentSessionId = ref<string>('')
// 响应式消息列表：直接绑定当前激活会话的 messages
const messages = computed<ChatMessage[]>({
  get() {
    const s = sessions.value.find(item => item.id === currentSessionId.value)
    return s ? s.messages : []
  },
  set(val: ChatMessage[]) {
    const s = sessions.value.find(item => item.id === currentSessionId.value)
    if (s) {
      s.messages = val
      s.updatedAt = Date.now()
      saveSessionsToStorage()
    }
  }
})
const currentSessionTitle = computed(() => {
  const s = sessions.value.find(item => item.id === currentSessionId.value)
  return s ? s.title : '默认对话'
})
// ========== 本地会话持久化逻辑 ==========
function loadSessionsFromStorage() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      sessions.value = JSON.parse(raw)
    }
  } catch (e) {
    console.error('Failed to parse sessions from localStorage', e)
  }

  if (!sessions.value || sessions.value.length === 0) {
    const defaultSession: ChatSession = {
      id: genMsgId('session'),
      title: '新对话',
      updatedAt: Date.now(),
      messages: [],
    }
    sessions.value = [defaultSession]
    currentSessionId.value = defaultSession.id
  } else {
    const savedActive = localStorage.getItem(ACTIVE_SESSION_KEY)
    const exists = sessions.value.find(s => s.id === savedActive)
    currentSessionId.value = exists ? exists.id : sessions.value[0].id
  }
}

function saveSessionsToStorage() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions.value))
    localStorage.setItem(ACTIVE_SESSION_KEY, currentSessionId.value)
  } catch (e) {
    console.error('Failed to save sessions to localStorage', e)
  }
}

function createNewSession() {
  const newSession: ChatSession = {
    id: genMsgId('session'),
    title: '新对话',
    updatedAt: Date.now(),
    messages: [],
  }
  sessions.value.unshift(newSession)
  currentSessionId.value = newSession.id
  saveSessionsToStorage()
  showSessionDrawer.value = false
}

function switchSession(id: string) {
  currentSessionId.value = id
  saveSessionsToStorage()
  showSessionDrawer.value = false
  nextTick(() => {
    scrollToBottom(false)
  })
}

function deleteSession(id: string) {
  if (sessions.value.length <= 1) {
    const s = sessions.value[0]
    s.title = '新对话'
    s.messages = []
    s.updatedAt = Date.now()
    saveSessionsToStorage()
    return
  }
  sessions.value = sessions.value.filter(s => s.id !== id)
  if (currentSessionId.value === id) {
    currentSessionId.value = sessions.value[0].id
  }
  saveSessionsToStorage()
}

function formatSessionTime(ts: number): string {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getMonth() + 1}/${d.getDate()} ${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`
}

  return { sessions, currentSessionId, messages, currentSessionTitle, loadSessionsFromStorage, saveSessionsToStorage, createNewSession, switchSession, deleteSession, formatSessionTime }
}
