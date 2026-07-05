<script setup lang="ts">
import {h, onBeforeUnmount, onMounted, ref, computed, reactive, nextTick} from "vue";
import {
  ExportConfig,
  GetConfig,
  SendDingDingMessageByType,
  UpdateConfig,
  FetchAiModels,
  FetchAiModelInfo,
  CleanupData
} from "../../wailsjs/go/main/App";
import {NTag, NTooltip, NIcon, useMessage, useDialog} from "naive-ui";
import {data, models} from "../../wailsjs/go/models";
import type { AIConfig } from "../../wailsjs/go/models";
import {EventsEmit} from "../../wailsjs/runtime";
import {HelpCircleFilledIcon, HelpIcon} from "tdesign-icons-vue-next";
import {useAiConfigStore} from "../stores/aiConfig";

const message = useMessage()
const dialog = useDialog()
const aiConfigStore = useAiConfigStore()

const formRef = ref(null)

// ========================
// AI 配置元数据（非业务字段，不入库）
// ========================
interface ConfigMeta {
  loadingModels: boolean
  modelOptions: { label: string; value: string }[]
}
const aiConfigMeta = reactive<Record<number, ConfigMeta>>({})
function getMeta(index: number, id: number): ConfigMeta {
  const key = id > 0 ? id : -index - 1
  if (!aiConfigMeta[key]) {
    aiConfigMeta[key] = { loadingModels: false, modelOptions: [] }
  }
  return aiConfigMeta[key]
}

// ========================
// AI 相关状态
// ========================
const openAiEnable = ref(false)
const aiConfigs = ref<AIConfig[]>([])
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
  iwencaiApiKey: '',
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
  qgqpBId: '',
  akshareServiceUrl: '',
  backtestAccuracyThreshold: 5.0,
  backtestVerifyDays: 2,
  backtestEnabled: true,
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
  aiConfigs.value.push(new data.AIConfig({
    ID: id,
    name: '',
    baseUrl: 'https://api.deepseek.com',
    apiKey: '',
    modelName: 'deepseek-reasoner',
    temperature: 0.1,
    maxTokens: 8192,
    timeOut: 300,
    httpProxy:"",
    httpProxyEnabled:false,
    thinking: true,
    embeddingEnabled: false,
    embeddingModelName: '',
    embeddingBaseUrl: '',
  }))
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

// ========================
// 模型列表获取（带请求去重）(F-04)
// ========================
let fetchModelsId = 0
async function fetchAiModels(aiConfig, index) {
  if (!aiConfig.baseUrl || !aiConfig.apiKey) {
    message.warning('请先填写接口地址和 apiKey')
    return
  }
  const meta = getMeta(index, aiConfig.ID)
  if (meta.loadingModels) return
  meta.loadingModels = true
  const reqId = ++fetchModelsId
  try {
    const list = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey)
    if (reqId !== fetchModelsId) return // 已过期
    const options = (list || []).map(id => ({ label: id, value: id }))
    meta.modelOptions = options
    if (!aiConfig.modelName && options.length > 0) {
      aiConfig.modelName = options[0].value
      onModelNameChange(aiConfig, aiConfig.modelName)
    }
    if (!options.length) {
      message.warning('未从接口获取到可用模型，请检查地址和 apiKey')
    }
  } catch (e) {
    if (reqId !== fetchModelsId) return
    console.error('FetchAiModels error', e)
    message.error('获取模型列表失败，请检查接口地址和 apiKey')
  } finally {
    if (reqId === fetchModelsId) {
      meta.loadingModels = false
    }
  }
}

const aiPlatformOptions = [
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

function onBaseUrlChange(aiConfig, newBaseUrl) {
  const platformName = getPlatformName(newBaseUrl)
  if (platformName && aiConfig.name && !aiConfig.name.startsWith(platformName)) {
    aiConfig.name = platformName + '-' + aiConfig.name
  } else if (platformName && !aiConfig.name) {
    aiConfig.name = platformName
  }
}

function onModelNameChange(aiConfig, newModelName) {
  if (!newModelName) return
  const platformName = getPlatformName(aiConfig.baseUrl)
  const baseName = platformName || 'AI'

  if (!aiConfig.name) {
    aiConfig.name = baseName + '-' + newModelName
  } else if (aiConfig.name === platformName) {
    aiConfig.name = platformName + '-' + newModelName
  } else {
    const parts = aiConfig.name.split('-')
    if (parts.length >= 2 && parts[0] === platformName) {
      parts[parts.length - 1] = newModelName
      aiConfig.name = parts.join('-')
    } else if (!aiConfig.name.endsWith(newModelName)) {
      aiConfig.name = aiConfig.name + '-' + newModelName
    }
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
    formValue.value.iwencaiApiKey = res.iwencaiApiKey || ''
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
    formValue.value.enableAgent = res.enableAgent;
    formValue.value.qgqpBId = res.qgqpBId;
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
    if (cfg.maxTokens < 1 || cfg.maxTokens > 128000) { message.warning(`MaxTokens 应在 1-128000 之间`); return }
    if (cfg.timeOut < 60 || cfg.timeOut > 600) { message.warning(`Timeout 应在 60-600 秒之间`); return }
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
    aiConfigs: aiConfigs.value,
    tushareToken: formValue.value.tushareToken,
    iwencaiApiKey: formValue.value.iwencaiApiKey,
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
    enableAgent: formValue.value.enableAgent,
    qgqpBId: formValue.value.qgqpBId,
  })

  try {
    const res = await UpdateConfig(config)
    if (res === '保存成功！') {
      message.success(res)
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
      formValue.value.iwencaiApiKey = config.iwencaiApiKey || ''
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
        ...{ ID: 0, name: '', baseUrl: '', apiKey: '', modelName: '', temperature: 0.1, maxTokens: 8192, timeOut: 300, httpProxy: '', httpProxyEnabled: false, thinking: false, embeddingEnabled: false, embeddingModelName: '', embeddingBaseUrl: '' },
        ...c,
        temperature: Number(c.temperature) || 0.1,
        maxTokens: Number(c.maxTokens) || 8192,
        timeOut: Number(c.timeOut) || 300,
      }))
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
      formValue.value.enableAgent = !!config.enableAgent
      formValue.value.qgqpBId = config.qgqpBId ?? ''
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
      <n-space vertical size="large">
        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '基础设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="10" label="Tushare Token：" path="tushareToken">
              <n-input type="text" placeholder="Tushare api token" v-model:value="formValue.tushareToken" clearable/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="启动时更新基础信息：" path="updateBasicInfoOnStart">
              <n-switch v-model:value="formValue.updateBasicInfoOnStart"/>
            </n-form-item-gi>
            <n-form-item-gi :span="4" label="数据刷新间隔：" path="refreshInterval">
              <n-input-number v-model:value="formValue.refreshInterval" placeholder="请输入数据刷新间隔(秒)" :min="1" :max="3600">
                <template #suffix>秒</template>
              </n-input-number>
            </n-form-item-gi>
            <n-form-item-gi :span="6" label="暗黑主题：" path="darkTheme">
              <n-switch v-model:value="formValue.darkTheme"/>
            </n-form-item-gi>
            <n-form-item-gi :span="10" label="浏览器安装路径：" path="browserPath">
              <n-input type="text" placeholder="浏览器安装路径" v-model:value="formValue.browserPath" clearable/>
            </n-form-item-gi>
           <n-form-item-gi :span="3" label="指数基金：" path="enableFund">
              <n-switch v-model:value="formValue.enableFund"/>
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

            <n-form-item-gi :span="11" label="问财API密钥：" path="iwencaiApiKey">
              <n-input type="password" placeholder="同花顺问财开放平台API Key" v-model:value="formValue.iwencaiApiKey" clearable show-password-on="click"/>
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
                    访问同花顺问财开放平台：<br>
                    <a href="https://open.iwencai.com" target="_blank" style="color: #63e2b7">https://www.iwencai.com/skillhub</a><br>
                    注册并登录后，在控制台获取API Key。<br>
                    配置后可使用问财智能选股、行情查询、研报搜索等功能。
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
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => '通知设置')" size="small">
          <n-grid :cols="24" :x-gap="24" style="text-align: left">
            <n-form-item-gi :span="3" label="钉钉推送：" path="dingPush.enable">
              <n-switch v-model:value="formValue.dingPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="本地推送：" path="localPush.enable">
              <n-switch v-model:value="formValue.localPush.enable"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="弹幕功能：" path="enableDanmu">
              <n-switch v-model:value="formValue.enableDanmu"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="显示滚动快讯：" path="enableNews">
              <n-switch v-model:value="formValue.enableNews"/>
            </n-form-item-gi>
            <n-form-item-gi :span="3" label="市场资讯提醒：" path="enablePushNews">
              <n-switch v-model:value="formValue.enablePushNews"/>
            </n-form-item-gi>
            <n-form-item-gi v-if="formValue.enablePushNews" :span="4" label="只提醒红字或关注个股的新闻：" path="enableOnlyPushRedNews">
              <n-switch v-model:value="formValue.enableOnlyPushRedNews"/>
            </n-form-item-gi>

            <n-form-item-gi :span="22" v-if="formValue.dingPush.enable" label="钉钉机器人接口地址："
                            path="dingPush.dingRobot">
              <n-input placeholder="请输入钉钉机器人接口地址" v-model:value="formValue.dingPush.dingRobot"/>
              <n-button type="primary" @click="sendTestNotice">发送测试通知</n-button>
            </n-form-item-gi>

          </n-grid>
        </n-card>

        <n-card :title="() => h(NTag, { type: 'primary', bordered: false }, () => 'AI设置')" size="small">
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
                        <n-input type="text" placeholder="配置名称" v-model:value="aiConfig.name" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="接口地址" :path="`openAI.aiConfigs[${index}].baseUrl`">
                        <n-select
                          v-model:value="aiConfig.baseUrl"
                          :options="aiPlatformOptions"
                          filterable
                          tag
                          clearable
                          placeholder="选择或输入AI接口地址"
                          @update:value="(val) => onBaseUrlChange(aiConfig, val)"
                        />
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="令牌(apiKey)" :path="`openAI.aiConfigs[${index}].apiKey`">
                        <n-input type="password" placeholder="apiKey" v-model:value="aiConfig.apiKey" clearable
                                 show-password-on="click"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="8" label="模型名称" :path="`openAI.aiConfigs[${index}].modelName`">
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
                      <n-form-item-gi :span="5" label="Temperature" :path="`openAI.aiConfigs[${index}].temperature`">
                        <n-input-number placeholder="temperature" v-model:value="aiConfig.temperature" :step="0.1" :min="0" :max="2"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="MaxTokens" :path="`openAI.aiConfigs[${index}].maxTokens`">
                        <n-input-number placeholder="maxTokens" v-model:value="aiConfig.maxTokens" :min="1" :max="128000"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="5" label="Timeout(秒)" :path="`openAI.aiConfigs[${index}].timeOut`">
                        <n-input-number :min="60" :max="600" step="1" placeholder="超时(秒)" v-model:value="aiConfig.timeOut"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="深度思考">
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
                      <n-form-item-gi :span="12" label="http代理" :path="`openAI.aiConfigs[${index}].httpProxyEnabled`">
                        <n-switch v-model:value="aiConfig.httpProxyEnabled"/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" v-if="aiConfig.httpProxyEnabled" title="http代理地址" :path="`openAI.aiConfigs[${index}].httpProxy`">
                        <n-input type="text" placeholder="http代理地址" v-model:value="aiConfig.httpProxy" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" label="语义向量记忆">
                        <n-switch v-model:value="aiConfig.embeddingEnabled"/>
                        <n-tooltip placement="top">
                          <template #trigger>
                            <n-icon color="#0e7a0d" size="20" style="margin-left: 8px;">
                              <HelpCircleFilledIcon />
                            </n-icon>
                          </template>
                          <template #default>
                            <n-gradient-text :type="'warning'">
                            <div style="max-width: 400px;text-align: left">
                              启用语义向量记忆后，Agent 会基于当前问题语义检索
                              最相关的历史对话（而非简单的最近N条），
                              需要配置 Embedding 模型（如 bge-m3）。
                            </div>
                            </n-gradient-text>
                          </template>
                        </n-tooltip>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" v-if="aiConfig.embeddingEnabled" label="Embedding模型名" :path="`openAI.aiConfigs[${index}].embeddingModelName`">
                        <n-input type="text" placeholder="bge-m3" v-model:value="aiConfig.embeddingModelName" clearable/>
                      </n-form-item-gi>
                      <n-form-item-gi :span="12" v-if="aiConfig.embeddingEnabled" label="Embedding服务地址" :path="`openAI.aiConfigs[${index}].embeddingBaseUrl`">
                        <n-input type="text" placeholder="留空则使用AI服务地址" v-model:value="aiConfig.embeddingBaseUrl" clearable/>
                      </n-form-item-gi>
                    </n-grid>
                  </n-collapse-item>
                </n-collapse>
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
              <n-divider/>
            </n-gi>

            <n-gi :span="24">
              <n-space vertical>
                <n-space justify="center">
                  <n-button type="primary" strong @click="saveConfig">保存设置</n-button>
                  <n-button type="info" @click="exportConfig">导出配置</n-button>
                  <n-button type="error" @click="importConfig">导入配置</n-button>
                </n-space>
              </n-space>
            </n-gi>

          </n-grid>
        </n-card>
      </n-space>
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