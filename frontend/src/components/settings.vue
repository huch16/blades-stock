<script setup lang="ts">
import {h, onBeforeUnmount, onMounted, ref, computed, reactive, nextTick} from "vue";
import {
  ExportConfig,
  GetConfig,
  SendDingDingMessageByType,
  UpdateConfig,
  FetchAiModels,
  FetchAiModelInfo,
  TestAiConnection,
  CleanupData,
  BackupData,
  RestoreData,
  GetAiConfigTemplates
} from "../../wailsjs/go/main/App";
import {NTag, NTooltip, NIcon, NCollapse, NCollapseItem, useMessage, useDialog} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import type { AIConfig } from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";
import {useAiConfigStore} from "../stores/aiConfig";

const message = useMessage()
const dialog = useDialog()
const aiConfigStore = useAiConfigStore()

const formRef = ref(null)
const basicExpanded = ref(['basic', 'datasource', 'notify', 'ai'])

// ========================
// 表单状态追踪 (F-01: 未保存提示/恢复默认/取消修改)
// ========================
const originalFormValue = ref({})
const isFormDirty = computed(() => {
  return JSON.stringify(formValue.value) !== JSON.stringify(originalFormValue.value)
})
function markClean() {
  originalFormValue.value = JSON.parse(JSON.stringify(formValue.value))
}
function cancelChanges() {
  if (!isFormDirty.value) return
  dialog.warning({
    title: '放弃未保存的修改？',
    content: '当前有未保存的修改，确定要放弃并恢复到最后保存的状态吗？',
    positiveText: '确定放弃',
    negativeText: '继续编辑',
    onPositiveAction: () => {
      formValue.value = JSON.parse(JSON.stringify(originalFormValue.value))
      message.info('已恢复到最后保存的状态')
    }
  })
}
function resetToDefaults() {
  dialog.warning({
    title: '确认恢复默认值？',
    content: '这将把所有设置重置为出厂默认值，当前未保存的修改将丢失。',
    positiveText: '确认重置',
    negativeText: '取消',
    onPositiveAction: () => {
      GetConfig().then(res => {
        formValue.value = {
          ID: res.ID,
          tushareToken: res.tushareToken,
          emApiKey: res.emApiKey || '',
          tickFlowAPIKey: res.tickFlowAPIKey || '',
          dingPush: { enable: res.dingPushEnable, dingRobot: res.dingRobot },
          localPush: { enable: res.localPushEnable },
          updateBasicInfoOnStart: res.updateBasicInfoOnStart,
          refreshInterval: res.refreshInterval,
          enableDanmu: res.enableDanmu,
          browserPath: res.browserPath,
          enableNews: res.enableNews,
          darkTheme: res.darkTheme,
          enableFund: res.enableFund,
          enablePushNews: res.enablePushNews,
          enableOnlyPushRedNews: res.enableOnlyPushRedNews,
          httpProxy: res.httpProxy,
          httpProxyEnabled: res.httpProxyEnabled,
          disableBrowserSearch: res.disableBrowserSearch ?? false,
          enableAgent: res.enableAgent,
          qgqpBId: res.qgqpBId,
          akshareServiceUrl: res.akshareServiceUrl || 'http://115.190.72.22:8089',
          aDataServiceUrl: res.aDataServiceUrl || 'http://115.190.72.22:8000',
          defaultDataSource: res.defaultDataSource || 'adata',
          embeddingEnabled: !!res.embeddingEnabled,
          embeddingModelName: res.embeddingModelName || '',
          embeddingBaseUrl: res.embeddingBaseUrl || '',
          embeddingApiKey: res.embeddingApiKey || '',
        }
        markClean()
        message.success('已恢复默认值，请点击「保存设置」生效')
      }).catch(err => message.error('重置失败: ' + (err?.message || err)))
    }
  })
}

// ========================
// AI 配置元数据（非业务字段，不入库）
// ========================
interface ConfigMeta {
  loadingModels: boolean
  testing: boolean
  modelOptions: { label: string; value: string }[]
}
const aiConfigMeta = reactive<Record<number, ConfigMeta>>({})
function getMeta(index: number, id: number): ConfigMeta {
  const key = id > 0 ? id : -index - 1
  if (!aiConfigMeta[key]) {
    aiConfigMeta[key] = { loadingModels: false, testing: false, modelOptions: [] }
  }
  return aiConfigMeta[key]
}

// ========================
// AI 相关状态
// ========================
const openAiEnable = ref(false)
const aiConfigs = ref<AIConfig[]>([])
const defaultAiConfigId = ref(0)
const aiConfigOptions = computed(() => aiConfigs.value.map(cfg => ({
  label: cfg.name || `AI 配置 #${aiConfigs.value.indexOf(cfg) + 1}`,
  value: cfg.ID,
  labelProp: cfg.modelName ? `${cfg.modelName}` : ''
})))
const aiPrompt = ref('')
const questionTemplate = ref('{{stockName}}分析和总结')
const crawlTimeOut = ref(30)
const kDays = ref(30)

const aiConfigExpandedNames = ref([])

// ========================
// 其他设置状态
// ========================
const formValue = ref({
  ID: 1,
  tushareToken: '',
  emApiKey: '',
  tickFlowAPIKey: '',
  dingPush: {
    enable: false,
    dingRobot: ''
  },
  localPush: {
    enable: true,
  },
  updateBasicInfoOnStart: false,
  refreshInterval: 1,
  enableDanmu: false,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enablePushNews: true,
  enableOnlyPushRedNews: true,
  httpProxy:"",
  httpProxyEnabled:false,
  enableAgent: false,
  disableBrowserSearch: false,
  qgqpBId: '',
  backtestAccuracyThreshold: 5.0,
  backtestVerifyDays: 2,
  backtestEnabled: true,
  akshareServiceUrl: 'http://115.190.72.22:8089',
  aDataServiceUrl: 'http://115.190.72.22:8000',
  defaultDataSource: 'adata',
  embeddingEnabled: false,
  embeddingModelName: '',
  embeddingBaseUrl: '',
  embeddingApiKey: '',
})

// ========================
// 表单校验规则 (F-01)
// ========================
const formRules = {
  tushareToken: [{ required: false }],
}

// ========================
// AI 配置操作
// ========================
let nextTempId = -1
function addAiConfig() {
  const id = aiConfigs.value.reduce((max, c) => Math.max(max, c.ID || 0), 0) + 1 || nextTempId--
  const newConfig = new data.AIConfig({
    ID: id,
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-reasoner',
    temperature: 0.7,
    maxTokens: 8192,
    timeOut: 300,
    httpProxy: '',
    httpProxyEnabled: false,
    thinking: true,
    embeddingEnabled: false,
    embeddingModelName: '',
    embeddingBaseUrl: '',
    llmConcurrencyLimit: 2,
    subAgentTimeout: 20,
  })
  // 自动生成配置名称（用户手动改名后不再自动覆盖）
  ;(newConfig as any)._nameTouched = false
  const platformName = getPlatformName(newConfig.baseUrl)
  const baseName = platformName || 'AI'
  newConfig.name = baseName + '-' + newConfig.modelName

  aiConfigs.value.push(newConfig)
  const idx = aiConfigs.value.length - 1
  // 展开新配置 (F-10)
  nextTick(() => {
    aiConfigExpandedNames.value = [String(idx)]
  })
  // 如果已配齐 baseUrl+apiKey，自动拉取模型列表
  if (aiConfigs.value[idx].baseUrl && aiConfigs.value[idx].apiKey) {
    fetchAiModels(aiConfigs.value[idx], idx)
  }
}

function removeAiConfig(index) {
  aiConfigs.value = aiConfigs.value.filter((_, i) => i !== index)
  // 同步折叠面板状态 (F-10)
  aiConfigExpandedNames.value = aiConfigExpandedNames.value
    .filter(n => Number(n) !== index)
    .map(n => String(Number(n) > index ? Number(n) - 1 : Number(n)))
}

// 复制一份 AI 配置，方便快速配置多个服务商
function copyAiConfig(index) {
  const src = aiConfigs.value[index]
  if (!src) return
  const clone = new data.AIConfig({ ...src })
  clone.ID = 0
  ;(clone as any)._nameTouched = false
  clone.name = (src.name || `AI 配置 ${index + 1}`) + '（副本）'
  aiConfigs.value.push(clone)
  const idx = aiConfigs.value.length - 1
  nextTick(() => {
    aiConfigExpandedNames.value = [String(idx)]
  })
}

// ========================
// AI 配置向导（三步：选择类型 → 自动配置 → 测试保存）
// ========================
interface AiConfigTemplate {
  type: 'local' | 'cloud' | 'enterprise'
  label: string
  description: string
  baseUrl: string
  modelName: string
  apiKeyRequired: boolean
  envHint: string
}

const wizardVisible = ref(false)
const wizardStep = ref(1) // 1=选择类型, 2=填写详情, 3=测试并保存
const wizardTemplates = ref<AiConfigTemplate[]>([])
const wizardSelectedTemplate = ref<AiConfigTemplate | null>(null)
const wizardForm = ref({
  name: '',
  baseUrl: '',
  apiKey: '',
  modelName: '',
  temperature: 0.7,
  maxTokens: 8192,
  timeOut: 300,
  thinking: false,
  httpProxy: '',
  httpProxyEnabled: false,
  llmConcurrencyLimit: 2,
  subAgentTimeout: 20,
})
const wizardTesting = ref(false)
const wizardTestResult = ref<string | null>(null)

async function openWizard() {
  if (!wizardTemplates.value.length) {
    try {
      const list = await GetAiConfigTemplates()
      wizardTemplates.value = list
    } catch (e) {
      console.error('获取模板失败', e)
      message.error('获取配置模板失败')
      return
    }
  }
  wizardStep.value = 1
  wizardSelectedTemplate.value = null
  wizardForm.value = {
    name: '',
    baseUrl: '',
    apiKey: '',
    modelName: '',
    temperature: 0.7,
    maxTokens: 8192,
    timeOut: 300,
    thinking: false,
    httpProxy: '',
    httpProxyEnabled: false,
    llmConcurrencyLimit: 2,
    subAgentTimeout: 20,
  }
  wizardTestResult.value = null
  wizardVisible.value = true
}

function wizardSelectTemplate(tpl: AiConfigTemplate) {
  wizardSelectedTemplate.value = tpl
  wizardForm.value.name = tpl.label
  wizardForm.value.baseUrl = tpl.baseUrl
  wizardForm.value.modelName = tpl.modelName
  wizardForm.value.apiKey = ''
  wizardForm.value.thinking = tpl.modelName.includes('reasoner') || tpl.modelName.includes('Reasoner')
  wizardStep.value = 2
}

async function wizardFetchModels() {
  if (!wizardForm.value.baseUrl) return
  try {
    const list = await FetchAiModels(wizardForm.value.baseUrl, wizardForm.value.apiKey || 'none')
    if (list.length) {
      wizardForm.value.modelName = list[0]
    }
  } catch (e) {
    console.warn('向导获取模型列表失败', e)
  }
}

async function wizardTestConnection() {
  if (!wizardForm.value.baseUrl || !wizardForm.value.modelName) {
    message.warning('请先填写接口地址和模型名')
    return
  }
  wizardTesting.value = true
  wizardTestResult.value = null
  try {
    const errMsg = await TestAiConnection(wizardForm.value.baseUrl, wizardForm.value.apiKey || 'none', wizardForm.value.modelName)
    if (errMsg) {
      wizardTestResult.value = '❌ 连接失败：' + errMsg
      message.error('连接失败：' + errMsg)
    } else {
      wizardTestResult.value = '✅ 连接成功！配置可用'
      message.success('连接成功！配置可用')
      wizardStep.value = 3
    }
  } catch (e) {
    wizardTestResult.value = '❌ 连接异常：' + (e as Error).message
    message.error('连接异常：' + (e as Error).message)
  } finally {
    wizardTesting.value = false
  }
}

function wizardSaveAndClose() {
  const newConfig = new data.AIConfig({
    ID: aiConfigs.value.reduce((max, c) => Math.max(max, c.ID || 0), 0) + 1 || nextTempId--,
    name: wizardForm.value.name,
    baseUrl: wizardForm.value.baseUrl,
    apiKey: wizardForm.value.apiKey,
    modelName: wizardForm.value.modelName,
    temperature: wizardForm.value.temperature,
    maxTokens: wizardForm.value.maxTokens,
    timeOut: wizardForm.value.timeOut,
    httpProxy: wizardForm.value.httpProxy,
    httpProxyEnabled: wizardForm.value.httpProxyEnabled,
    thinking: wizardForm.value.thinking,
    embeddingEnabled: false,
    embeddingModelName: '',
    embeddingBaseUrl: '',
    llmConcurrencyLimit: wizardForm.value.llmConcurrencyLimit,
    subAgentTimeout: wizardForm.value.subAgentTimeout,
  })
  ;(newConfig as any)._nameTouched = true
  aiConfigs.value.push(newConfig)
  const idx = aiConfigs.value.length - 1
  nextTick(() => {
    aiConfigExpandedNames.value = [String(idx)]
  })
  wizardVisible.value = false
  message.success('配置已添加，请点击「保存设置」持久化')
}

function wizardClose() {
  wizardVisible.value = false
}

// 测试连接：真实调用 /chat/completions 验证 baseUrl + apiKey + modelName 可用（兼容不支持 /models 的服务）
async function testAiConfig(aiConfig, index) {
  if (!aiConfig.baseUrl) {
    message.warning('请先填写接口地址再测试')
    return
  }
  if (!aiConfig.modelName) {
    const list = await fetchAiModels(aiConfig, index, { silent: true })
    if (!list.length) {
      message.warning('模型名为空且无法自动获取，请先在「模型名」中选择或填写')
      return
    }
  }
  const meta = getMeta(index, aiConfig.ID)
  if (meta.testing) return
  meta.testing = true
  try {
    const errMsg = await TestAiConnection(aiConfig.baseUrl, aiConfig.apiKey || 'none', aiConfig.modelName)
    if (errMsg) {
      message.error('连接失败：' + errMsg)
    } else {
      message.success('连接成功！配置可用')
      if (meta.modelOptions.length === 0) fetchAiModels(aiConfig, index, { silent: true })
    }
  } catch (e) {
    console.error('testAiConfig error', e)
    message.error('连接失败：无法访问该接口，请检查网络和地址')
  } finally {
    meta.testing = false
  }
}

// ========================
// 模型列表获取（带请求去重）(F-04)
// ========================
let fetchModelsId = 0
async function fetchAiModels(aiConfig, index, opts: { silent?: boolean } = {}) {
  if (!aiConfig.baseUrl) {
    if (!opts.silent) message.warning('请先填写接口地址')
    return []
  }
  const meta = getMeta(index, aiConfig.ID)
  if (meta.loadingModels) return meta.modelOptions.map(o => o.value)
  meta.loadingModels = true
  const reqId = ++fetchModelsId
  try {
    const list = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey || 'none')
    if (reqId !== fetchModelsId) return []
    const options = (list || []).map(id => ({ label: id, value: id }))
    meta.modelOptions = options
    if (!aiConfig.modelName && options.length > 0) {
      aiConfig.modelName = options[0].value
      onModelNameChange(aiConfig, aiConfig.modelName)
    }
    if (!options.length && !opts.silent) {
      message.warning('未从接口获取到可用模型，可手动填写模型名')
    }
    return list || []
  } catch (e) {
    if (reqId !== fetchModelsId) return []
    console.error('FetchAiModels error', e)
    if (!opts.silent) message.error('获取模型列表失败，可手动填写模型名')
    return []
  } finally {
    meta.loadingModels = false
  }
}

const aiPlatformOptions = [
  { label: 'NVIDIA NIM (https://integrate.api.nvidia.com/v1)', value: 'https://integrate.api.nvidia.com/v1' },
  { label: 'DeepSeek (https://api.deepseek.com)', value: 'https://api.deepseek.com' },
  { label: '硅基流动 (https://api.siliconflow.cn/v1)', value: 'https://api.siliconflow.cn/v1' },
  { label: '智谱AI(GLM) (https://open.bigmodel.cn/api/paas/v4)', value: 'https://open.bigmodel.cn/api/paas/v4' },
  { label: '字节豆包(火山引擎) (https://ark.cn-beijing.volces.com/api/v3)', value: 'https://ark.cn-beijing.volces.com/api/v3' },
  { label: '阿里云百炼 (https://dashscope.aliyuncs.com/compatible-mode/v1)', value: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
  { label: 'Moonshot(月之暗面) (https://api.moonshot.cn/v1)', value: 'https://api.moonshot.cn/v1' },
  { label: '腾讯混元 (https://api.hunyuan.cloud.tencent.com/v1)', value: 'https://api.hunyuan.cloud.tencent.com/v1' },
  { label: '讯飞星火 (https://spark-api-open.xf-yun.com/v1)', value: 'https://spark-api-open.xf-yun.com/v1' },
  { label: '零一万物 (https://api.lingyiwanwu.com/v1)', value: 'https://api.lingyiwanwu.com/v1' },
  { label: 'MiniMax (https://api.minimax.chat/v1)', value: 'https://api.minimax.chat/v1' },
  { label: '小米MiMo TokenPlan (https://token-plan-cn.xiaomimimo.com/v1)', value: 'https://token-plan-cn.xiaomimimo.com/v1' },
  { label: '小米MiMo (https://api.xiaomimimo.com/v1)', value: 'https://api.xiaomimimo.com/v1' },
  { label: '腾讯云TokenHub (https://tokenhub.tencentmaas.com/v1)', value: 'https://tokenhub.tencentmaas.com/v1' },
  { label: 'OpenAI (https://api.openai.com/v1)', value: 'https://api.openai.com/v1' },
  { label: 'Azure OpenAI (https://YOUR_RESOURCE.openai.azure.com)', value: 'https://YOUR_RESOURCE.openai.azure.com' },
  { label: 'OpenRouter (https://openrouter.ai/api/v1)', value: 'https://openrouter.ai/api/v1' },
  { label:'Ollama (http://localhost:11434/v1)', value: 'http://localhost:11434/v1' },
]

function getPlatformName(baseUrl) {
  if (!baseUrl) return ''
  const platform = aiPlatformOptions.find(opt => opt.value === baseUrl)
  if (platform) {
    const idx = platform.label.indexOf(' (')
    return idx > 0 ? platform.label.substring(0, idx) : platform.label
  }
  return ''
}

// 常用向量（Embedding）模型与服务地址建议
const embeddingModelOptions = [
  { label: 'nvidia/nv-embed-v1（NVIDIA，推荐·实测可用）', value: 'nvidia/nv-embed-v1' },
  { label: 'nvidia/nemotron-3-embed-1b（NVIDIA，实测可用）', value: 'nvidia/nemotron-3-embed-1b' },
  { label: 'bge-m3（BAAI 多语言）', value: 'bge-m3' },
  { label: 'bge-large-zh-v1.5', value: 'bge-large-zh-v1.5' },
  { label: 'bge-small-zh-v1.5', value: 'bge-small-zh-v1.5' },
  { label: 'm3e-base（Moka）', value: 'm3e-base' },
  { label: 'text-embedding-3-small（OpenAI）', value: 'text-embedding-3-small' },
  { label: 'text-embedding-ada-002（OpenAI）', value: 'text-embedding-ada-002' },
  { label: 'BAAI/bge-m3（硅基流动）', value: 'BAAI/bge-m3' },
]
const embeddingBaseUrlOptions = [
  { label: 'NVIDIA NIM (https://integrate.api.nvidia.com/v1)', value: 'https://integrate.api.nvidia.com/v1' },
  { label: 'Ollama 本地 (http://localhost:11434/v1)', value: 'http://localhost:11434/v1' },
  { label: '硅基流动 (https://api.siliconflow.cn/v1)', value: 'https://api.siliconflow.cn/v1' },
  { label: 'OpenAI (https://api.openai.com/v1)', value: 'https://api.openai.com/v1' },
  { label: '阿里云百炼 (https://dashscope.aliyuncs.com/compatible-mode/v1)', value: 'https://dashscope.aliyuncs.com/compatible-mode/v1' },
]

function markNameTouched(aiConfig) {
  if (aiConfig) (aiConfig as any)._nameTouched = true
}

function onBaseUrlChange(aiConfig, newBaseUrl) {
  if ((aiConfig as any)._nameTouched) return
  const platformName = getPlatformName(newBaseUrl)
  if (platformName && aiConfig.name && !aiConfig.name.startsWith(platformName)) {
    aiConfig.name = platformName + '-' + aiConfig.name
  } else if (platformName && !aiConfig.name) {
    aiConfig.name = platformName
  }
}

function onModelNameChange(aiConfig, newModelName) {
  if (!newModelName) return
  if (!(aiConfig as any)._nameTouched) {
    const platformName = getPlatformName(aiConfig.baseUrl)
    const baseName = platformName || 'AI'
    aiConfig.name = baseName + '-' + newModelName
  }

  fetchModelInfo(aiConfig, newModelName)
}

let fetchModelInfoId = 0
async function fetchModelInfo(aiConfig, modelName) {
  if (!modelName || !aiConfig.baseUrl) return
  const reqId = ++fetchModelInfoId
  try {
    const info = await FetchAiModelInfo(aiConfig.baseUrl, aiConfig.apiKey || '', modelName)
    if (reqId !== fetchModelInfoId) return
    if (info && info.maxTokens > 0) {
      aiConfig.maxTokens = info.maxTokens
      const sourceLabel = info.source === 'api' ? 'API' : '内置数据'
      message.success(`已自动设置 ${modelName} 的 MaxTokens 为 ${info.maxTokens}（来源：${sourceLabel}）`)
    }
  } catch (e) {
    if (reqId !== fetchModelInfoId) return
    console.error('FetchAiModelInfo error', e)
    message.warning('获取模型信息失败')
  }
}

onMounted(() => {
  GetConfig().then(res => {
    formValue.value.ID = res.ID
    formValue.value.tushareToken = res.tushareToken
    formValue.value.emApiKey = res.emApiKey || ''
    formValue.value.tickFlowAPIKey = res.tickFlowAPIKey || ''
    formValue.value.dingPush = {
      enable: res.dingPushEnable,
      dingRobot: res.dingRobot
    }
    formValue.value.localPush = {
      enable: res.localPushEnable,
    }
    formValue.value.updateBasicInfoOnStart = res.updateBasicInfoOnStart
    formValue.value.refreshInterval = res.refreshInterval

    openAiEnable.value = res.openAiEnable
    aiConfigs.value = res.aiConfigs || []
    aiConfigs.value.forEach(c => { (c as any)._nameTouched = true })
    defaultAiConfigId.value = res.aiConfigId ?? 0
    aiPrompt.value = res.prompt || ''
    questionTemplate.value = res.questionTemplate || '{{stockName}}分析和总结'
    crawlTimeOut.value = res.crawlTimeOut || 30
    kDays.value = res.kDays || 30

    // 对已配齐 baseUrl+apiKey 的配置自动拉取模型列表
    for (let i = 0; i < aiConfigs.value.length; i++) {
      const cfg = aiConfigs.value[i]
      if (cfg.baseUrl && cfg.apiKey) {
        fetchAiModels(cfg, i)
      }
    }

    // 同步 AI 配置到 Pinia Store (F-08)
    if (res.aiConfigs && res.aiConfigs.length > 0) {
      aiConfigStore.configs = res.aiConfigs
      if (!aiConfigStore.activeConfigId && res.aiConfigs[0]?.ID) {
        aiConfigStore.setActiveConfigId(res.aiConfigs[0].ID)
      }
    }

    formValue.value.enableDanmu = res.enableDanmu
    formValue.value.browserPath = res.browserPath
    formValue.value.enableNews = res.enableNews
    formValue.value.darkTheme = res.darkTheme
    formValue.value.enableFund = res.enableFund
    formValue.value.enablePushNews = res.enablePushNews
    formValue.value.enableOnlyPushRedNews = res.enableOnlyPushRedNews
    formValue.value.httpProxy=res.httpProxy;
    formValue.value.httpProxyEnabled=res.httpProxyEnabled;
    formValue.value.disableBrowserSearch = res.disableBrowserSearch ?? false;
    formValue.value.enableAgent = res.enableAgent;
    formValue.value.qgqpBId = res.qgqpBId;
    formValue.value.akshareServiceUrl = res.akshareServiceUrl || 'http://115.190.72.22:8089';
    formValue.value.aDataServiceUrl = res.aDataServiceUrl || 'http://115.190.72.22:8000';
    formValue.value.defaultDataSource = res.defaultDataSource || 'adata';
    formValue.value.embeddingEnabled = !!res.embeddingEnabled;
    formValue.value.embeddingModelName = res.embeddingModelName || '';
    formValue.value.embeddingBaseUrl = res.embeddingBaseUrl || '';
    formValue.value.embeddingApiKey = res.embeddingApiKey || '';
    markClean()
  }).catch(err => {
    message.error('加载配置失败: ' + (err?.message || err))
  })

})

onBeforeUnmount(() => {
  message.destroyAll()
})

async function saveConfig() {
  // 表单校验 (F-01)
  try {
    await formRef.value?.validate()
  } catch {
    message.warning('请检查表单中的必填项')
    return
  }

  // 数值范围校验 (F-09/F-16)
  for (const cfg of aiConfigs.value) {
    if (cfg.temperature < 0 || cfg.temperature > 2) { message.warning(`Temperature 应在 0-2 之间`); return }
    if (cfg.maxTokens < 1 || cfg.maxTokens > 256000) { message.warning(`MaxTokens 应在 1-256000 之间`); return }
    if (cfg.timeOut && (cfg.timeOut < 5 || cfg.timeOut > 3600)) { message.warning(`Timeout 应在 5-3600 秒之间`); return }
  }

  console.log('开始保存设置', formValue.value);
  let config = new data.SettingConfig({
    ID: formValue.value.ID,
    dingPushEnable: formValue.value.dingPush.enable,
    dingRobot: formValue.value.dingPush.dingRobot,
    localPushEnable: formValue.value.localPush.enable,
    updateBasicInfoOnStart: formValue.value.updateBasicInfoOnStart,
    refreshInterval: formValue.value.refreshInterval,
    openAiEnable: openAiEnable.value,
    aiConfigId: defaultAiConfigId.value,
    aiConfigs: aiConfigs.value,
    tushareToken: formValue.value.tushareToken,
    emApiKey: formValue.value.emApiKey,
    tickFlowAPIKey: formValue.value.tickFlowAPIKey,
    prompt: aiPrompt.value,
    questionTemplate: questionTemplate.value,
    crawlTimeOut: crawlTimeOut.value,
    kDays: kDays.value,
    enableDanmu: formValue.value.enableDanmu,
    browserPath: formValue.value.browserPath,
    enableNews: formValue.value.enableNews,
    darkTheme: formValue.value.darkTheme,
    enableFund: formValue.value.enableFund,
    enablePushNews: formValue.value.enablePushNews,
    enableOnlyPushRedNews: formValue.value.enableOnlyPushRedNews,
    httpProxy:formValue.value.httpProxy,
    httpProxyEnabled:formValue.value.httpProxyEnabled,
    disableBrowserSearch: formValue.value.disableBrowserSearch,
    enableAgent: formValue.value.enableAgent,
    qgqpBId: formValue.value.qgqpBId,
    akshareServiceUrl: formValue.value.akshareServiceUrl,
    aDataServiceUrl: formValue.value.aDataServiceUrl,
    defaultDataSource: formValue.value.defaultDataSource,
    embeddingEnabled: formValue.value.embeddingEnabled,
    embeddingModelName: formValue.value.embeddingModelName,
    embeddingBaseUrl: formValue.value.embeddingBaseUrl,
    embeddingApiKey: formValue.value.embeddingApiKey,
  })

  try {
    const res = await UpdateConfig(config)
    if (res === '保存成功！') {
      message.success(res)
      markClean()
      // 保存成功后刷新 Store (F-08/F-32)
      await aiConfigStore.loadConfigs()
    } else {
      message.error(res)
    }
    EventsEmit("updateSettings", config);
  } catch (err) {
    message.error('保存失败: ' + (err?.message || err))
  }
}

function getHeight() {
  return document.documentElement.clientHeight
}

function sendTestNotice() {
  let markdown = "### blades-stock test\n" + new Date()
  let msg = '{' +
      '     "msgtype": "markdown",' +
      '     "markdown": {' +
      '         "title":"blades-stock' + new Date() + '",' +
      '         "text": "' + markdown + '"' +
      '     },' +
      '      "at": {' +
      '          "isAtAll": true' +
      '      }' +
      ' }'

  SendDingDingMessageByType(msg, "test-" + new Date().getTime(), 1).then(res => {
    message.info(res)
  }).catch(err => {
    message.error('发送测试通知失败: ' + (err?.message || err))
  })
}

function exportConfig() {
  ExportConfig().then(res => {
    message.info(res)
  }).catch(err => {
    message.error('导出失败: ' + (err?.message || err))
  })
}

async function cleanup(type) {
  const typeNameMap = {
    'logs': '日志',
    'chat_sessions': 'AI对话历史',
    'ai_results': 'AI分析结果',
    'test_images': '测试图片',
    'browser_temp': '浏览器临时目录',
    'all': '所有中间数据'
  }
  const typeName = typeNameMap[type] || type
  try {
    const confirm = await dialog.warning({
      title: '确认清理',
      content: `确定要清理${typeName}吗？此操作不可恢复。`,
      positiveText: '确认清理',
      negativeText: '取消',
      closable: false
    })
    if (!confirm) return
  } catch {
    return
  }
  CleanupData(type).then(res => {
    message.success(res)
  }).catch(err => {
    message.error('清理失败: ' + (err?.message || err))
  })
}

async function backup(type) {
  const typeNameMap = {
    'followed_stocks': '自选股票',
    'recommend_stocks': '推荐股票',
    'ai_reports': 'AI分析报告',
    'followed_funds': '关注基金',
    'backtest_results': '回测结果',
    'stock_change_history': '异动历史(最近1万条)'
  }
  const typeName = typeNameMap[type] || type
  try {
    const confirm = await dialog.info({
      title: '确认备份',
      content: `确定要备份${typeName}到 CSV 文件吗？`,
      positiveText: '确认备份',
      negativeText: '取消',
      closable: false
    })
    if (!confirm) return
  } catch {
    return
  }
  BackupData(type).then(res => {
    message.success(res)
  }).catch(err => {
    message.error('备份失败: ' + (err?.message || err))
  })
}

async function restore(type) {
  const typeNameMap = {
    'followed_stocks': '自选股票',
    'recommend_stocks': '推荐股票',
    'ai_reports': 'AI分析报告',
    'followed_funds': '关注基金',
    'backtest_results': '回测结果',
    'stock_change_history': '异动历史(最近1万条)'
  }
  const typeName = typeNameMap[type] || type
  try {
    const confirm = await dialog.warning({
      title: '确认恢复',
      content: `确定要从 CSV 恢复 ${typeName} 吗？将覆盖当前数据。`,
      positiveText: '确认恢复',
      negativeText: '取消',
      closable: false
    })
    if (!confirm) return
  } catch {
    return
  }
  RestoreData(type).then(res => {
    message.success(res)
  }).catch(err => {
    message.error('恢复失败: ' + (err?.message || err))
  })
}

function importConfig() {
  let input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    let file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return
    let reader = new FileReader();
    reader.onload = (e) => {
      let config: any
      try {
        config = JSON.parse(e.target?.result as string) // (F-02)
      } catch {
        message.error('配置文件格式错误')
        return
      }
      // 字段校验 (F-07)
      if (!config || typeof config !== 'object') {
        message.error('无效的配置文件')
        return
      }
      formValue.value.ID = config.ID ?? 1
      formValue.value.tushareToken = config.tushareToken ?? ''
      formValue.value.emApiKey = config.emApiKey || ''
      formValue.value.dingPush = {
        enable: !!config.dingPushEnable,
        dingRobot: config.dingRobot ?? '',
      }
      formValue.value.localPush = {
        enable: config.localPushEnable !== false,
      }
      formValue.value.updateBasicInfoOnStart = !!config.updateBasicInfoOnStart
      formValue.value.refreshInterval = config.refreshInterval ?? 1
      openAiEnable.value = !!config.openAiEnable
      aiConfigs.value = (config.aiConfigs || []).map(c => new data.AIConfig({
        ...{ ID: 0, name: '', baseUrl: '', apiKey: '', modelName: '', temperature: 0.7, maxTokens: 8192, timeOut: 300, httpProxy: '', httpProxyEnabled: false, thinking: false, embeddingEnabled: false, embeddingModelName: '', embeddingBaseUrl: '', llmConcurrencyLimit: 2, subAgentTimeout: 20 },
        ...c,
        temperature: Number(c.temperature) || 0.7,
        maxTokens: Number(c.maxTokens) || 8192,
        timeOut: Number(c.timeOut) || 300,
      }))
      aiConfigs.value.forEach(c => { (c as any)._nameTouched = true })
      aiPrompt.value = config.prompt ?? ''
      questionTemplate.value = config.questionTemplate ?? '{{stockName}}分析和总结'
      crawlTimeOut.value = config.crawlTimeOut ?? 30
      kDays.value = config.kDays ?? 30
      formValue.value.enableDanmu = !!config.enableDanmu
      formValue.value.browserPath = config.browserPath ?? ''
      formValue.value.enableNews = !!config.enableNews
      formValue.value.darkTheme = config.darkTheme !== false
      formValue.value.enableFund = !!config.enableFund
      formValue.value.enablePushNews = config.enablePushNews !== false
      formValue.value.enableOnlyPushRedNews = config.enableOnlyPushRedNews !== false
      formValue.value.httpProxy = config.httpProxy ?? ''
      formValue.value.httpProxyEnabled = !!config.httpProxyEnabled
      formValue.value.disableBrowserSearch = !!config.disableBrowserSearch
      formValue.value.enableAgent = !!config.enableAgent
      formValue.value.qgqpBId = config.qgqpBId ?? ''
      formValue.value.akshareServiceUrl = config.akshareServiceUrl ?? 'http://115.190.72.22:8089'
      formValue.value.aDataServiceUrl = config.aDataServiceUrl ?? 'http://115.190.72.22:8000'
      formValue.value.defaultDataSource = config.defaultDataSource ?? 'adata'
      formValue.value.embeddingEnabled = !!config.embeddingEnabled
      formValue.value.embeddingModelName = config.embeddingModelName ?? ''
      formValue.value.embeddingBaseUrl = config.embeddingBaseUrl ?? ''
      formValue.value.embeddingApiKey = config.embeddingApiKey ?? ''
      formValue.value.updateChannel = config.updateChannel || 'release'
      // 导入后同步 Store (F-08)
      if (aiConfigs.value.length > 0) {
        aiConfigStore.configs = aiConfigs.value
        if (!aiConfigStore.activeConfigId && aiConfigs.value[0]?.ID) {
          aiConfigStore.setActiveConfigId(aiConfigs.value[0].ID)
        }
      }
      message.success('导入成功')
    };
    reader.readAsText(file);
  };
  input.click();
}


</script>

<template>
  <n-flex justify="left" style="text-align: left; --wails-draggable:no-drag">
    <n-form ref="formRef" :rules="formRules" :label-placement="'left'" :label-align="'left'">
      <n-collapse v-model:expanded-names="basicExpanded" accordion>
        <n-collapse-item name="basic">
          <template #header>
            <n-flex justify="space-between" align="center">
              <n-text><NTag type="primary" bordered="false">基础设置</NTag></n-text>
              <n-text depth="3" style="font-size: 12px;">Tushare Token、主题、刷新间隔、浏览器路径、指数基金、东财标识</n-text>
            </n-flex>
          </template>
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="10" label="Tushare Token：" path="tushareToken">
              <n-input type="text" placeholder="Tushare api token (如: your_tushare_pro_token)" v-model:value="formValue.tushareToken" clearable/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>Tushare Pro 官网申请的 API Token，用于获取基础行情数据，示例: your_tushare_pro_token</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="启动时更新基础信息：" path="updateBasicInfoOnStart">
              <n-switch v-model:value="formValue.updateBasicInfoOnStart"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>启动时自动更新股票基本信息（代码、名称、行业等）</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="数据刷新间隔：" path="refreshInterval">
              <n-input-number v-model:value="formValue.refreshInterval" placeholder="请输入数据刷新间隔(秒), 如 5" :min="1" :max="3600">
                <template #suffix>秒</template>
              </n-input-number>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>自动刷新行情数据的间隔，建议 1-60 秒，示例: 5</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="暗黑主题：" path="darkTheme">
              <n-switch v-model:value="formValue.darkTheme"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>跟随系统或手动切换深色/浅色模式</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="10" label="浏览器安装路径：" path="browserPath">
              <n-input type="text" placeholder="浏览器安装路径, 如 C:\Program Files\Google\Chrome\Application\chrome.exe" v-model:value="formValue.browserPath" clearable/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>本地浏览器可执行文件路径，用于爬虫/搜索功能</template></n-tooltip>
              </template>
            </n-form-item-gi>
           <n-form-item-gi :span="3" label="指数基金：" path="enableFund">
              <n-switch v-model:value="formValue.enableFund"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>启用基金相关功能（基金列表、净值、排行等）</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="11" label="东财唯一标识：" path="qgqpBId">
              <n-input type="text" placeholder="东财唯一标识" v-model:value="formValue.qgqpBId" clearable/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    打开浏览器,访问东财网站，<br>
                    按F12打开开发人员工具-》网络面板，<br>
                    随便点开一个请求，复制请求cookie中qgqp_b_id对应的值。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
            <n-form-item-gi :span="11" label="东财AI密钥：" path="emApiKey">
              <n-input type="password" placeholder="东方财富AI SaaS API Key" v-model:value="formValue.emApiKey" clearable show-password-on="click"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    访问东方财富妙想AI平台获取API Key。
                    https://ai.eastmoney.com/mxClaw<br>
                    配置后可使用个股业绩点评功能。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-form-item-gi :span="12" label="TickFlow API Key：" path="tickFlowAPIKey">
              <n-input type="password" placeholder="TickFlow API Key (https://tickflow.org)" v-model:value="formValue.tickFlowAPIKey" clearable show-password-on="click"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    获取方法：<br>
                    访问 https://tickflow.org 注册并获取 API Key。<br>
                    配置后可使用 TickFlow 实时行情、K线、财务数据等接口，
                    替代同花顺问财和 Sina 爬虫。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

          </n-grid>
        </n-collapse-item>

        <n-collapse-item name="datasource">
          <template #header>
            <n-flex justify="space-between" align="center">
              <n-text><NTag type="primary" bordered="false">数据源设置</NTag></n-text>
              <n-text depth="3" style="font-size: 12px;">AKShare/AData 代理地址、默认数据源优先级</n-text>
            </n-flex>
          </template>
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="8" label="AKShare 服务地址：" path="akshareServiceUrl">
              <n-input type="text" placeholder="AKShare 代理地址 (如: http://your-vps:8089)" v-model:value="formValue.akshareServiceUrl" clearable/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>AKShare 代理服务地址，提供基础行情/财务/资金流数据，示例: http://your-vps:8089</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="AData 服务地址：" path="aDataServiceUrl">
              <n-input type="text" placeholder="AData 服务地址 (如: http://your-vps:8000)" v-model:value="formValue.aDataServiceUrl" clearable/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>AData 服务地址，提供高性能行情/财务/资金流数据，示例: http://your-vps:8000</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="默认数据源：" path="defaultDataSource">
              <n-select v-model:value="formValue.defaultDataSource" :options="[
                { label: 'AData（VPS :8000）', value: 'adata' },
                { label: 'AKShare（VPS :8089）', value: 'akshare' },
                { label: '自动（MAC→东财→新浪→腾讯→通达信）', value: 'auto' },
              ]"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>默认优先使用的数据源，auto 会按优先级自动降级</template></n-tooltip>
              </template>
            </n-form-item-gi>
          </n-grid>
        </n-collapse-item>

        <n-collapse-item name="notify">
          <template #header>
            <n-flex justify="space-between" align="center">
              <n-text><NTag type="primary" bordered="false">通知设置</NTag></n-text>
              <n-text depth="3" style="font-size: 12px;">钉钉/本地推送、弹幕、快讯、仅红字提醒</n-text>
            </n-flex>
          </template>
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="3" label="钉钉推送：" path="dingPush.enable">
              <n-switch v-model:value="formValue.dingPush.enable"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>启用后通过钉钉机器人推送重要通知</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="本地推送：" path="localPush.enable">
              <n-switch v-model:value="formValue.localPush.enable"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>启用系统原生通知，无需钉钉也能收到提醒</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="弹幕功能：" path="enableDanmu">
              <n-switch v-model:value="formValue.enableDanmu"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>在 K 线图上显示弹幕式实时资讯/异动提示</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="显示滚动快讯：" path="enableNews">
              <n-switch v-model:value="formValue.enableNews"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>启用市场资讯/快讯采集与滚动展示</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="市场资讯提醒：" path="enablePushNews">
              <n-switch v-model:value="formValue.enablePushNews"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>开启后收到市场快讯/资讯推送</template></n-tooltip>
              </template>
            </n-form-item-gi>
            <n-form-item-gi v-if="formValue.enablePushNews" :span="4" label="只提醒红字或关注个股的新闻：" path="enableOnlyPushRedNews">
              <n-switch v-model:value="formValue.enableOnlyPushRedNews"/>
              <template #feedback>
                <n-tooltip placement="top"><template #trigger><n-icon color="#0e7a0d" size="16"><HelpCircleFilledIcon /></n-icon></template><template #default>仅推送涨幅>0%或关注个股的新闻，减少干扰</template></n-tooltip>
              </template>
            </n-form-item-gi>

            <n-form-item-gi :span="22" v-if="formValue.dingPush.enable" label="钉钉机器人接口地址："
                            path="dingPush.dingRobot">
              <n-input placeholder="请输入钉钉机器人接口地址" v-model:value="formValue.dingPush.dingRobot"/>
              <n-button type="primary" @click="sendTestNotice">发送测试通知</n-button>
            </n-form-item-gi>

          </n-grid>
        </n-collapse-item>

        <n-collapse-item name="ai">
          <template #header>
            <n-flex justify="space-between" align="center">
              <n-text><NTag type="primary" bordered="false">AI设置</NTag></n-text>
              <n-text depth="3" style="font-size: 12px;">AI诊股、模型配置、向量模型、回测、提示词、AI配置管理</n-text>
            </n-flex>
          </template>
          <n-grid :cols="24" :x-gap="24" style="text-align: left;">
            <n-form-item-gi :span="24" label="AI诊股：" path="openAI.enable">
              <n-switch v-model:value="openAiEnable"/>
            </n-form-item-gi>

            <n-form-item-gi :span="6" v-if="openAiEnable" label="Crawler Timeout(秒)"
                            title="资讯采集超时时间(秒)" path="openAI.crawlTimeOut">
              <n-input-number :min="30" :max="300" step="1" v-model:value="crawlTimeOut"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" v-if="openAiEnable" title="天数越多消耗tokens越多"
                            label="日K线数据(天)" path="openAI.kDays">
              <n-input-number :min="5" step="1" :max="60" v-model:value="kDays"/>
            </n-form-item-gi>
            <n-form-item-gi :span="2" label="爬虫http代理" path="httpProxyEnabled">
              <n-switch v-model:value="formValue.httpProxyEnabled"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" v-if="formValue.httpProxyEnabled" title="http代理地址"
                            label="http代理地址" path="httpProxy">
              <n-input type="text" placeholder="爬虫http代理地址" v-model:value="formValue.httpProxy" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="禁用浏览器搜索" path="disableBrowserSearch">
              <n-switch v-model:value="formValue.disableBrowserSearch"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    开启后搜索引擎只使用 SearXNG JSON API + 百度 HTTP，不再启动 Edge/Chrome 浏览器进程，可大幅降低内存占用。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>


            <n-gi :span="24" v-if="openAiEnable">
              <n-divider title-placement="left">回测设置</n-divider>
            </n-gi>
            <n-form-item-gi :span="8" v-if="openAiEnable" label="回测准确率阈值(%)" path="backtestAccuracyThreshold">
              <n-input-number :min="0" :max="100" step="0.1" v-model:value="formValue.backtestAccuracyThreshold"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    回测准确率阈值：预测涨跌幅度超过此阈值才被认为是准确预测。<br>
                    例如：设置为5.0时，预测涨跌幅度>5%且方向正确才算是准确预测。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
            <n-form-item-gi :span="8" v-if="openAiEnable" label="回测验证天数" path="backtestVerifyDays">
              <n-input-number :min="1" :max="60" step="1" v-model:value="formValue.backtestVerifyDays"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    回测验证天数：分析后多少个交易日验证预测是否准确。<br>
                    例如：设置为2时，分析后2个交易日验证预测准确性。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
            <n-form-item-gi :span="4" v-if="openAiEnable" label="启用回测" path="backtestEnabled">
              <n-switch v-model:value="formValue.backtestEnabled"/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="openAiEnable">
              <n-divider title-placement="left">默认提示词设置</n-divider>
            </n-gi>
            <n-form-item-gi :span="12" v-if="openAiEnable" label="默认系统提示词" path="openAI.prompt">
              <n-input v-model:value="aiPrompt" type="textarea" :show-count="true"
                       placeholder="请输入系统提示词" :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>
            <n-form-item-gi :span="12" v-if="openAiEnable" label="默认个股分析提示词"
                            path="openAI.questionTemplate">
              <n-input v-model:value="questionTemplate" type="textarea" :show-count="true"
                       placeholder="请输入个股分析提示词:例如{{stockName}}[{{stockCode}}]分析和总结"
                       :autosize="{ minRows: 4, maxRows: 8 }"/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="openAiEnable">
              <n-divider title-placement="left">AI模型服务配置</n-divider>
            </n-gi>
            <n-form-item-gi :span="12" v-if="openAiEnable && aiConfigs.length > 0" label="默认AI配置">
              <n-select v-model:value="defaultAiConfigId" :options="aiConfigOptions" filterable placeholder="选择默认AI配置" clearable />
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    指定用于 AI 对话/分析的默认模型配置。<br>
                    可配置多个服务商（DeepSeek、硅基流动、智谱、豆包、Ollama 本地等），<br>
                    通过「测试连接」可验证配置是否可用。
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>

            <n-gi :span="24" v-if="openAiEnable">
              <n-divider title-placement="left">向量模型配置 (全局独立，用于语义检索/技能匹配/长期记忆)</n-divider>
            </n-gi>
            <n-form-item-gi :span="4" v-if="openAiEnable" label="启用向量模型">
              <n-switch v-model:value="formValue.embeddingEnabled"/>
              <n-tooltip placement="top">
                <template #trigger>
                  <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                    <HelpCircleFilledIcon />
                  </n-icon>
                </template>
                <template #default>
                  <n-gradient-text :type="'warning'">
                  <div style="max-width: 400px;text-align: left">
                    启用后使用 Embedding 模型进行语义向量化：<br>
                    • 技能关键词匹配 (bge-m3 等)<br>
                    • 长期记忆向量存储/检索<br>
                    • 全局独立于每个 Chat 模型配置，通常使用专用 Embedding 服务<br>
                    • 服务地址留空时自动解析：优先使用显式配置了 Embedding 的 AI 配置；<br>
                    &nbsp;&nbsp;否则对 Ollama / 通义 / 豆包 / OpenAI / 硅基流动 自动派生端点<br>
                    • 端点会自动健康检查，不可用时降级为关键词匹配（不再复用纯 Chat 提供商）
                  </div>
                  </n-gradient-text>
                </template>
              </n-tooltip>
            </n-form-item-gi>
            <n-form-item-gi :span="6" v-if="openAiEnable && formValue.embeddingEnabled" label="向量模型名" path="embeddingModelName">
              <n-select
                v-model:value="formValue.embeddingModelName"
                :options="embeddingModelOptions"
                filterable
                tag
                clearable
                placeholder="bge-m3"
              />
            </n-form-item-gi>
            <n-form-item-gi :span="7" v-if="openAiEnable && formValue.embeddingEnabled" label="向量服务地址" path="embeddingBaseUrl">
              <n-select
                v-model:value="formValue.embeddingBaseUrl"
                :options="embeddingBaseUrlOptions"
                filterable
                tag
                clearable
                placeholder="留空则自动解析(Ollama/通义/豆包/OpenAI/硅基流动)"
              />
            </n-form-item-gi>
            <n-form-item-gi :span="7" v-if="openAiEnable && formValue.embeddingEnabled" label="向量服务 ApiKey" path="embeddingApiKey">
              <n-input type="password" placeholder="ApiKey" v-model:value="formValue.embeddingApiKey" clearable show-password-on="click"/>
            </n-form-item-gi>

            <n-gi :span="24" v-if="openAiEnable">
              <n-space vertical>
                <n-collapse v-model:expanded-names="aiConfigExpandedNames" accordion>
                  <n-collapse-item v-for="(aiConfig, index) in aiConfigs" :key="aiConfig.ID || index" :name="String(index)">
                    <template #header>
                      <n-flex justify="space-between" align="center" style="width: 100%;">
                        <n-text>{{ aiConfig.name || `AI 配置 #${index + 1}` }}</n-text>
                        <n-text depth="3" style="font-size: 12px;">{{ aiConfig.modelName || '未选择模型' }}</n-text>
                      </n-flex>
                    </template>
                    <template #header-extra>
                      <n-button type="error" size="tiny" ghost @click.stop="removeAiConfig(index)" style="margin-right: 8px;">删除</n-button>
                    </template>
                    <n-grid :cols="24" :x-gap="24">
                      <n-form-item-gi :span="24" hidden label="配置ID" :path="`openAI.aiConfigs[${index}].ID`">
                        <n-input type="text" placeholder="配置ID" v-model:value="aiConfig.ID" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="配置名称" :path="`openAI.aiConfigs[${index}].name`">
                        <n-input type="text" placeholder="配置名称（可自定义，后续不会自动覆盖）" v-model:value="aiConfig.name"
                                 clearable @update:value="() => markNameTouched(aiConfig)"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="接口地址" :path="`openAI.aiConfigs[${index}].baseUrl`">
                        <n-select
                          v-model:value="aiConfig.baseUrl"
                          :options="aiPlatformOptions"
                          filterable
                          tag
                          clearable
                          placeholder="选择或输入 AI 接口地址（OpenAI 兼容）"
                          @update:value="(val) => onBaseUrlChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="令牌(apiKey)" :path="`openAI.aiConfigs[${index}].apiKey`">
                        <n-input type="password" placeholder="apiKey" v-model:value="aiConfig.apiKey" clearable
                                 show-password-on="click"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="模型名称" :path="`openAI.aiConfigs[${index}].modelName`">
                        <n-select
                          v-model:value="aiConfig.modelName"
                          :options="getMeta(index, aiConfig.ID).modelOptions"
                          filterable
                          tag
                          :loading="getMeta(index, aiConfig.ID).loadingModels"
                          placeholder="点击获取模型列表或手动输入"
                          @click="fetchAiModels(aiConfig, index)"
                          @update:value="(val) => onModelNameChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" label="Temperature" :path="`openAI.aiConfigs[${index}].temperature`">
                        <n-input-number placeholder="0~2" v-model:value="aiConfig.temperature" :step="0.1" :min="0" :max="2"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              采样随机度（0~2）。<br>
                              股票分析等偏严谨任务建议 0.2~0.5，过高易臆造结论；<br>
                              深度思考(reasoning)模型建议 ≤0.7。
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" label="MaxTokens" :path="`openAI.aiConfigs[${index}].maxTokens`">
                        <n-input-number placeholder="maxTokens" v-model:value="aiConfig.maxTokens" :min="1" :max="128000"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              单次回答的最大输出 Token 数。<br>
                              4096 适合一般分析；回答被截断时再调大（如 8192）。<br>
                              过大会更慢、更贵，建议按需设置。
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" label="Timeout(秒)" :path="`openAI.aiConfigs[${index}].timeOut`">
                        <n-input-number :min="60" :max="600" step="1" placeholder="超时(秒)" v-model:value="aiConfig.timeOut"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              单次请求超时秒数。<br>
                              普通场景 120~300 足够；经常超时再加大。
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" label="深度思考">
                        <n-switch v-model:value="aiConfig.thinking"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              启用深度思考模式：<br>
                              适用于 DeepSeek-Reasoner、MiMo-V2.5-Pro 等支持推理的模型。<br>
                              如使用普通模型请关闭此选项
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="6" label="http代理">
                        <n-switch v-model:value="aiConfig.httpProxyEnabled"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="18" v-if="aiConfig.httpProxyEnabled" title="http代理地址" :path="`openAI.aiConfigs[${index}].httpProxy`">
                        <n-input type="text" placeholder="http代理地址（如 http://127.0.0.1:7890）" v-model:value="aiConfig.httpProxy" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="并发限制">
                        <n-input-number v-model:value="aiConfig.llmConcurrencyLimit" :min="1" :max="20" step="1" placeholder="默认2"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              多 Agent 协作分析时的并发调用数（默认 2）。<br>
                              值越大速度越快，但更消耗 Tokens 与接口限额。
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="子Agent超时(秒)">
                        <n-input-number v-model:value="aiConfig.subAgentTimeout" :min="5" :max="300" step="1" placeholder="默认20"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              子 Agent 每次调用的超时时间（秒，默认 20）。<br>
                              过小易中断复杂分析，过大会拖慢整体响应。
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="参数提示">
                        <n-text depth="3" style="font-size: 12px;">
                          建议：分析类任务 Temperature 0.2~0.5；<br>回答被截断再调大 MaxTokens；<br>并发限制默认 2 即可，过高更耗 Token 且易触发限流。
                        </n-text>
                      </n-form-item-gi>
                      <n-gi :span="24">
                        <n-flex justify="end" style="margin-bottom: 8px;">
                          <n-button type="primary" ghost size="small"
                                    :loading="getMeta(index, aiConfig.ID).testing"
                                    @click="testAiConfig(aiConfig, index)">
                            测试连接
                          </n-button>
                          <n-button type="info" ghost size="small" @click="copyAiConfig(index)">
                            复制配置
                          </n-button>
                        </n-flex>
                      </n-gi>
                    </n-grid>
                  </n-collapse-item>
                </n-collapse>
                <n-empty v-if="aiConfigs.length === 0" description="还没有 AI 配置，点击下方按钮添加" style="padding: 12px 0;"/>
                <n-button type="primary" dashed @click="addAiConfig" style="width: 100%;">+ 添加AI配置</n-button>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-divider title-placement="left">数据清理</n-divider>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-text depth="3">清理应用运行过程中产生的中间文件、日志、缓存和历史记录</n-text>
                <n-space justify="center" wrap>
                  <n-button type="warning" ghost @click="cleanup('logs')">清理日志</n-button>
                  <n-button type="warning" ghost @click="cleanup('chat_sessions')">清理AI对话</n-button>
                  <n-button type="warning" ghost @click="cleanup('ai_results')">清理分析结果</n-button>
                  <n-button type="warning" ghost @click="cleanup('test_images')">清理测试图片</n-button>
                  <n-button type="warning" ghost @click="cleanup('browser_temp')">清理浏览器临时目录</n-button>
                  <n-button type="error" @click="cleanup('all')">一键清理全部</n-button>
                </n-space>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-divider title-placement="left">数据备份</n-divider>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-text depth="3">导出用户数据到备份目录下的 CSV 文件（Excel 可直接打开），备份完成后会显示具体路径</n-text>
                <n-space justify="center" wrap>
                  <n-button type="info" ghost @click="backup('followed_stocks')">备份自选股票</n-button>
                  <n-button type="info" ghost @click="backup('recommend_stocks')">备份推荐股票</n-button>
                  <n-button type="info" ghost @click="backup('ai_reports')">备份AI分析报告</n-button>
                  <n-button type="info" ghost @click="backup('followed_funds')">备份关注基金</n-button>
                  <n-button type="info" ghost @click="backup('backtest_results')">备份回测结果</n-button>
                  <n-button type="info" ghost @click="backup('stock_change_history')">备份异动历史</n-button>
                </n-space>
                <n-divider/>
                <n-text depth="3">从最近的备份 CSV 文件恢复数据到数据库（将覆盖当前数据）</n-text>
                <n-space justify="center" wrap>
                  <n-button type="warning" ghost @click="restore('followed_stocks')">恢复自选股票</n-button>
                  <n-button type="warning" ghost @click="restore('recommend_stocks')">恢复推荐股票</n-button>
                  <n-button type="warning" ghost @click="restore('ai_reports')">恢复AI分析报告</n-button>
                  <n-button type="warning" ghost @click="restore('followed_funds')">恢复关注基金</n-button>
                  <n-button type="warning" ghost @click="restore('backtest_results')">恢复回测结果</n-button>
                  <n-button type="warning" ghost @click="restore('stock_change_history')">恢复异动历史</n-button>
                </n-space>
              </n-space>
            </n-gi>

            <n-gi :span="24">
              <n-divider/>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-space justify="center">
                  <n-button type="primary" strong @click="saveConfig" :disabled="!isFormDirty">保存设置</n-button>
                  <n-button type="default" @click="cancelChanges" :disabled="!isFormDirty">取消修改</n-button>
                  <n-button type="warning" @click="resetToDefaults">恢复默认值</n-button>
                  <n-divider vertical style="margin: 0 16px;"/>
                  <n-button type="info" @click="exportConfig">导出配置</n-button>
                  <n-button type="error" @click="importConfig">导入配置</n-button>
                </n-space>
                <n-text depth="3" style="font-size: 12px; text-align: center; display: block;">
                  {{ isFormDirty ? '⚠ 有未保存的修改，请点击保存设置' : '✓ 配置已是最新状态' }}
                </n-text>
              </n-space>
            </n-gi>

          </n-grid>
        </n-collapse-item>
      </n-collapse>
    </n-form>
  </n-flex>
</template>

<style scoped>
.cardHeaderClass {
  font-size: 16px;
  font-weight: bold;
  color: red;
}
</style>


