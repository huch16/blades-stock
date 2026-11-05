<template>
  <div class="msg-wrapper user" :id="anchorId">
    <div class="msg-content">
      <div class="msg-header">
        <span class="msg-time">{{ formatTime(message.datetime) }}</span>
        <span class="msg-name">{{ name || '您' }}</span>
      </div>

      <div class="msg-bubble">
        <div v-if="!isEditing" class="msg-text">
          {{ message.content }}
        </div>
        <div v-else class="msg-edit-box">
          <textarea
            v-model="editDraft"
            class="msg-edit-textarea"
            rows="3"
            @keydown.enter.ctrl.prevent="saveEdit"
            @keydown.esc="cancelEdit"
          />
          <div class="msg-edit-actions">
            <button class="btn-cancel" @click="cancelEdit">取消</button>
            <button class="btn-save" @click="saveEdit">重新发送</button>
          </div>
        </div>
      </div>

      <!-- 悬停操作栏 -->
      <div class="msg-actions" v-if="!isEditing">
        <button class="action-btn" title="复制" @click="onCopy">📋</button>
        <button class="action-btn" title="编辑" @click="startEdit">✏️</button>
        <button class="action-btn" title="重新发送" @click="onResend">🔄</button>
        <button class="action-btn" title="引用" @click="onReference">🔗</button>
      </div>
    </div>

    <div class="msg-avatar">
      <n-avatar :size="36" :src="avatar" round>
        {{ (name || '您').charAt(0) }}
      </n-avatar>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, inject, onMounted, onUnmounted } from 'vue'
import { NAvatar } from 'naive-ui'
import type { ChatMessage } from './types'
import type { RegisterMessageElement, UnregisterMessageElement } from './useMessageAnchor'

const props = defineProps<{
  message: ChatMessage
  avatar?: string
  name?: string
  isStreaming?: boolean
}>()

const emit = defineEmits<{
  copy: [content: string]
  edit: [msg: ChatMessage]
  resend: [msg: ChatMessage]
  reference: [msgId: string]
}>()

const registerElement = inject<RegisterMessageElement>('registerMessageElement')
const unregisterElement = inject<UnregisterMessageElement>('unregisterMessageElement')

const anchorId = computed(() => `msg-${props.message.id}`)

onMounted(() => {
  const el = document.getElementById(anchorId.value)
  if (el) registerElement?.(anchorId.value, el)
})

onUnmounted(() => {
  unregisterElement?.(anchorId.value)
})

const isEditing = ref(false)
const editDraft = ref('')

function formatTime(dt?: string): string {
  if (!dt) return ''
  try {
    const d = new Date(dt)
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  } catch {
    return ''
  }
}

function onCopy() {
  navigator.clipboard.writeText(props.message.content).then(() => {
    emit('copy', props.message.content)
  })
}

function startEdit() {
  editDraft.value = props.message.content
  isEditing.value = true
}

function cancelEdit() {
  isEditing.value = false
}

function saveEdit() {
  if (editDraft.value.trim()) {
    const updated = { ...props.message, content: editDraft.value.trim() }
    emit('resend', updated)
    isEditing.value = false
  }
}

function onResend() {
  emit('resend', props.message)
}

function onReference() {
  emit('reference', props.message.id)
}
</script>

<style scoped lang="less">
.msg-wrapper.user {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-bottom: 20px;
  align-items: flex-start;
}

.msg-avatar {
  flex-shrink: 0;
}

.msg-content {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
  max-width: 75%;
}

.msg-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--td-text-color-secondary);
}

.msg-name {
  font-weight: 500;
  color: var(--td-text-color-primary);
}

.msg-time {
  color: var(--td-text-color-disabled);
}

.msg-bubble {
  background: linear-gradient(135deg, #1d4ed8 0%, #1e40af 100%);
  color: #f0f6fc;
  padding: 8px 14px;
  border-radius: 12px 2px 12px 12px;
  word-break: break-word;
  white-space: pre-wrap;
  font-size: 13.5px;
  line-height: 1.55;
  box-shadow: 0 2px 8px rgba(0, 40, 150, 0.25);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.msg-actions {
  display: flex;
  gap: 6px;
  opacity: 0;
  transition: opacity 0.2s;
}

.msg-wrapper:hover .msg-actions {
  opacity: 1;
}

.action-btn {
  background: transparent;
  border: 1px solid var(--td-component-border);
  border-radius: 4px;
  cursor: pointer;
  padding: 2px 6px;
  font-size: 11px;
  color: var(--td-text-color-secondary);
  transition: all 0.15s;

  &:hover {
    background: var(--td-bg-color-component-hover);
    color: var(--td-brand-color);
  }
}

.msg-edit-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 260px;
}

.msg-edit-textarea {
  width: 100%;
  padding: 8px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.4);
  background: rgba(255, 255, 255, 0.1);
  color: #fff;
  font-family: inherit;
  font-size: 13px;
  resize: vertical;
  outline: none;

  &:focus {
    border-color: #fff;
  }
}

.msg-edit-actions {
  display: flex;
  justify-content: flex-end;
  gap: 6px;
}

.btn-cancel,
.btn-save {
  padding: 4px 10px;
  border-radius: 4px;
  font-size: 12px;
  cursor: pointer;
  border: none;
}

.btn-cancel {
  background: rgba(255, 255, 255, 0.2);
  color: #fff;
}

.btn-save {
  background: #fff;
  color: var(--td-brand-color, #0052d9);
  font-weight: 500;
}
</style>
