import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

import type { data } from '../../wailsjs/go/models'
import { GetAiConfigs } from '../../wailsjs/go/main/App'

type AIConfig = data.AIConfig

const STORAGE_KEY = 'aiConfigStore_activeConfigId'

export const useAiConfigStore = defineStore('aiConfig', () => {
  const configs = ref<AIConfig[]>([])

  const savedId = localStorage.getItem(STORAGE_KEY)
  const activeConfigId = ref<number>(savedId ? parseInt(savedId, 10) || 0 : 0)

  const loading = ref(false)

  const error = ref<string | null>(null)

  const activeConfig = computed<AIConfig | undefined>(() => {
    if (activeConfigId.value <= 0) return undefined
    return configs.value.find(c => c.ID === activeConfigId.value)
  })

  const hasActiveConfig = computed(() => activeConfig.value !== undefined)

  const isActiveValid = computed(() => {
    const c = activeConfig.value
    return !!c && c.ID > 0 && !!c.baseUrl && !!c.apiKey && !!c.modelName
  })

  const isEmpty = computed(() => configs.value.length === 0)

  const activeModelName = computed(() => activeConfig.value?.modelName ?? '')

  const activeBaseUrl = computed(() => activeConfig.value?.baseUrl ?? '')

  async function loadConfigs(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const list = await GetAiConfigs() as AIConfig[]
      configs.value = list || []

      if (activeConfigId.value <= 0 && configs.value.length > 0) {
        activeConfigId.value = configs.value[0].ID
      } else if (activeConfigId.value > 0 && !configs.value.find(c => c.ID === activeConfigId.value)) {
        activeConfigId.value = configs.value.length > 0 ? configs.value[0].ID : 0
      }
    } catch (e: any) {
      error.value = e?.message || '加载 AI 配置失败'
      console.error('[aiConfigStore] loadConfigs failed:', e)
      throw e
    } finally {
      loading.value = false
    }
  }

  function setActiveConfigId(id: number): void {
    activeConfigId.value = id
    if (id > 0) {
      localStorage.setItem(STORAGE_KEY, String(id))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  }

  async function refreshConfigs(): Promise<void> {
    await loadConfigs()
  }

  function getNextId(): number {
    if (configs.value.length === 0) return 1
    return Math.max(...configs.value.map(c => c.ID || 0)) + 1
  }

  function addConfig(cfg: AIConfig): void {
    if (!cfg.ID) cfg.ID = getNextId()
    configs.value.push(cfg)
  }

  function updateConfig(id: number, patch: Partial<AIConfig>): void {
    const idx = configs.value.findIndex(c => c.ID === id)
    if (idx >= 0) {
      configs.value[idx] = { ...configs.value[idx], ...patch }
    }
  }

  function removeConfig(id: number): void {
    const idx = configs.value.findIndex(c => c.ID === id)
    if (idx >= 0) {
      configs.value.splice(idx, 1)
      if (activeConfigId.value === id) {
        activeConfigId.value = configs.value.length > 0 ? configs.value[0].ID : 0
      }
    }
  }

  function reset(): void {
    configs.value = []
    activeConfigId.value = 0
    localStorage.removeItem(STORAGE_KEY)
    loading.value = false
    error.value = null
  }

  return {
    configs,
    activeConfigId,
    loading,
    error,
    activeConfig,
    hasActiveConfig,
    isActiveValid,
    isEmpty,
    activeModelName,
    activeBaseUrl,
    loadConfigs,
    fetchAiConfigs: loadConfigs,
    setActiveConfigId,
    refreshConfigs,
    addConfig,
    updateConfig,
    removeConfig,
    reset,
  }
})
