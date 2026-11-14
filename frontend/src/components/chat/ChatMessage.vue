<template>
  <component
    :is="message.role === 'user' ? UserMessage : AssistantMessage"
    :message="message"
    :avatar="avatar"
    :name="name"
    :is-streaming="isStreaming"
    @copy="onCopy"
    @retry="onRetry"
    @reference="onReference"
    @edit="onEdit"
    @resend="onResend"
    @save-thesis="onSaveThesis"
    @export-image="onExportImage"
    @continue-wait="onContinueWait"
    @stop="onStop"
  />
</template>

<script setup lang="ts">
import UserMessage from './UserMessage.vue'
import AssistantMessage from './AssistantMessage.vue'
import { computed } from 'vue'
import { useAiConfigStore } from '@/stores/aiConfig'
import type { ChatMessage } from './types'

const props = defineProps<{
  message: ChatMessage
  isStreaming?: boolean
  currentUserAvatar?: string
  currentUserName?: string
}>()

const emit = defineEmits<{
  copy: [content: string]
  retry: [msgId: string]
  reference: [msgId: string]
  edit: [msg: ChatMessage]
  resend: [msg: ChatMessage]
  saveThesis: [msg: ChatMessage]
  exportImage: [msg: ChatMessage]
  // 看门狗超时后的用户选择（继续等待 / 停止本轮）
  continueWait: [msgId: string]
  stop: [msgId: string]
}>()

const aiConfigStore = useAiConfigStore()

// 计算头像和名称
const avatar = computed(() => {
  if (props.message.role === 'assistant') {
    return props.message.avatar || aiConfigStore.activeConfig?.baseUrl?.includes('localhost') ? '🏠' : '🤖'
  }
  return props.currentUserAvatar || props.message.avatar
})

const name = computed(() => {
  if (props.message.role === 'assistant') {
    return props.message.name || 'AI Assistant'
  }
  return props.currentUserName || props.message.name
})

function onCopy(content: string) {
  emit('copy', content)
}

function onRetry() {
  emit('retry', props.message.id)
}

function onReference() {
  emit('reference', props.message.id)
}

function onEdit(msg: ChatMessage) {
  emit('edit', msg)
}

function onResend(msg: ChatMessage) {
  emit('resend', msg)
}

function onSaveThesis(msg: ChatMessage) {
  emit('saveThesis', msg)
}

function onExportImage(msg: ChatMessage) {
  emit('exportImage', msg)
}

function onContinueWait() {
  emit('continueWait', props.message.id)
}

function onStop() {
  emit('stop', props.message.id)
}
</script>

<style scoped>
/* 无额外样式，仅作为分发组件 */
</style>