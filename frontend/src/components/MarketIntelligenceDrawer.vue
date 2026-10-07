<script setup>
import { computed, h, onBeforeUnmount, ref, watch } from 'vue'
import {
  NDrawer,
  NDrawerContent,
  NSpin,
  NCard,
  NFlex,
  NTag,
  NDivider,
  NButton,
  NSpace,
  NEmpty,
  useMessage
} from 'naive-ui'
import {
  GeneratePreMarketBriefing,
  GeneratePostMarketReview,
  GetLatestMarketIntelligenceReport,
  GetMarketLifecycleStatus,
  GetAiConfigs
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import { MdPreview } from 'md-editor-v3'

const props = defineProps({
  show: { type: Boolean, default: false },
  reportType: { type: String, default: 'pre_market_briefing' } // pre_market_briefing | post_market_review
})

const emit = defineEmits(['update:show', 'openFloatingAgentWithQuestion'])

const message = useMessage()
const loading = ref(false)
const report = ref(null)
const lifecycle = ref(null)
const aiConfigs = ref([])
const selectedAiConfigId = ref(0)
const aiConfigReady = ref(false)

const reportTitle = computed(() => {
  if (!report.value) return '智能市场研报'
  if (report.value.type === 'pre_market_briefing') return '🌅 盘前智能前瞻晨报'
  if (report.value.type === 'post_market_review') return '🌆 盘后收盘深度复盘'
  return report.value.title || '智能市场研报'
})

const phaseTagType = computed(() => {
  const phase = lifecycle.value?.phase
  if (!phase) return 'default'
  if (phase === 'pre_market') return 'info'
  if (phase === 'morning_trading' || phase === 'afternoon_trading') return 'success'
  if (phase === 'post_market') return 'warning'
  if (phase === 'closed') return 'error'
  return 'default'
})

watch(
  () => props.show,
  async (visible) => {
    if (visible) {
      await loadAiConfigs()
      await refreshAll()
    }
  }
)

async function loadAiConfigs() {
  try {
    const configs = await GetAiConfigs()
    aiConfigs.value = configs || []
    if (aiConfigs.value.length > 0) {
      selectedAiConfigId.value = aiConfigs.value[0].ID
    }
    aiConfigReady.value = true
  } catch (e) {
    console.error('GetAiConfigs error:', e)
  }
}

async function refreshAll() {
  await Promise.all([loadLatest(), loadLifecycle()])
}

async function loadLatest() {
  try {
    const latest = await GetLatestMarketIntelligenceReport()
    if (latest && latest.type === props.reportType) {
      report.value = latest
    }
  } catch (e) {
    console.error('GetLatestMarketIntelligenceReport error:', e)
  }
}

async function loadLifecycle() {
  try {
    lifecycle.value = await GetMarketLifecycleStatus()
  } catch (e) {
    console.error('GetMarketLifecycleStatus error:', e)
  }
}

async function generate() {
  if (!selectedAiConfigId.value) {
    message.warning('请先选择 AI 配置')
    return
  }
  loading.value = true
  try {
    let generated = null
    if (props.reportType === 'pre_market_briefing') {
      generated = await GeneratePreMarketBriefing(selectedAiConfigId.value)
    } else {
      generated = await GeneratePostMarketReview(selectedAiConfigId.value)
    }
    if (generated && generated.content) {
      report.value = generated
      message.success('已生成最新智能研报')
    } else {
      message.warning('AI 暂时未返回有效内容，请稍后再试')
    }
  } catch (e) {
    console.error('generate error:', e)
    message.error('生成失败: ' + (e?.message || e))
  } finally {
    loading.value = false
  }
}

function handleClose() {
  emit('update:show', false)
}

function askAgentAssistant() {
  const title = report.value?.title || reportTitle.value
  emit(
    'openFloatingAgentWithQuestion',
    `请就最近一份${title}做精炼复述并给出今天需要重点关注的板块和个股。`
  )
  handleClose()
}

// 监听后端主动推送的智能研报事件
EventsOn('marketIntelligenceReport', (incoming) => {
  if (!incoming) return
  if (incoming.type === props.reportType) {
    report.value = incoming
  }
})

onBeforeUnmount(() => {
  EventsOff('marketIntelligenceReport')
})
</script>

<template>
  <NDrawer
    :show="show"
    @update:show="handleClose"
    :width="600"
    placement="right"
  >
    <NDrawerContent
      :title="reportTitle"
      :native-scrollbar="false"
      closable
    >
      <NSpace vertical size="medium">
          <NCard size="small" :bordered="false" embedded>
            <NSpace align="center" justify="space-between">
              <NSpace align="center">
                <NTag :type="phaseTagType" round size="small">
                  {{ lifecycle?.phaseName || '市场阶段' }}
                </NTag>
                <span style="font-size: 12px; color: #888;">
                  {{ lifecycle?.marketStateDesc }}
                </span>
              </NSpace>
              <span v-if="lifecycle?.nextEvent" style="font-size: 12px; color: #888;">
                ⏭ {{ lifecycle.nextEvent }} ({{ lifecycle.nextEventTime }})
              </span>
            </NSpace>
          </NCard>

          <NSpace align="center" justify="space-between">
            <span style="font-size: 12px; color: #888;">
              <span v-if="report">
                上次生成：{{ new Date(report.generatedAt).toLocaleString() }}
                · 来源：{{ report.source }}
              </span>
              <span v-else>暂无历史研报，点击下方按钮立即生成</span>
            </span>
          </NSpace>

          <NSpace v-if="report.summary" align="center">
            <NTag :type="props.reportType === 'pre_market_briefing' ? 'info' : 'warning'" size="small" round>
              摘要
            </NTag>
            <span style="font-size: 13px; line-height: 1.5;">
              {{ report.summary }}
            </span>
          </NSpace>

          <NDivider style="margin: 4px 0;" />

          <NSpin :show="loading">
            <NEmpty
              v-if="!report || !report.content"
              description="尚未生成任何智能研报"
            >
              <template #extra>
                <NButton type="primary" @click="generate" :loading="loading">
                  立即生成 {{ reportTitle }}
                </NButton>
              </template>
            </NEmpty>
            <div v-else style="font-size: 14px; line-height: 1.6;">
              <MdPreview
                :modelValue="report.content"
                preview-theme="github"
                :show-code-row-number="false"
                style="background: transparent;"
              />
            </div>
          </NSpin>
        </NSpace>

      <template #footer>
        <NSpace justify="space-between" align="center">
          <span style="font-size: 12px; color: #888;">
            {{ report ? '🧠 由量化AI驱动 · 自动同步 ai_response_result 历史' : '' }}
          </span>
          <NSpace>
            <NButton @click="askAgentAssistant" v-if="report" tertiary>
              唤起AI助手复盘
            </NButton>
            <NButton @click="loadLatest" tertiary v-if="report">
              重新加载
            </NButton>
            <NButton @click="generate" type="primary" :loading="loading">
              {{ report ? '重新生成' : '立即生成' }}
            </NButton>
          </NSpace>
        </NSpace>
      </template>
    </NDrawerContent>
  </NDrawer>
</template>