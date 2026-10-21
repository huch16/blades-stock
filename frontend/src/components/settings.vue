<script setup lang="ts">
import { h, onBeforeUnmount, onMounted, ref, computed, reactive, nextTick, watch } from "vue";
import {
  NCard, NTabs, NTabPane, NGrid, NGi, NForm, NFormItem, NFormItemGi,
  NInput, NInputNumber, NSwitch, NSelect, NButton, NButtonGroup,
  NSpace, NFlex, NDivider, NTag, NTooltip, NIcon, NModal, NSpin,
  NStatistic, NAlert, NPopconfirm, useMessage, useDialog
} from "naive-ui";
import { data, models } from "../../wailsjs/go/models";
import type { AIConfig } from "../../wailsjs/go/models";
import { EventsEmit } from "../../wailsjs/runtime";
import { HelpCircleFilledIcon } from "tdesign-icons-vue-next";
import { useAiConfigStore } from "../stores/aiConfig";
import { isWails, getAuthToken, openAuthModal, logoutWebAuth } from "../api/bridge";
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
  GetAiConfigTemplates,
  GetBackupOverview,
  ListBackups,
  CreateIntelligentBackup,
  CreateDatabaseSnapshot,
  RestoreBackupFile,
  DeleteBackupFile,
  GetBackupContent,
  ImportIntelligentBackup,
  TriggerSemanticDistillation,
  GetDistillationStats
} from "../../wailsjs/go/main/App";

const message = useMessage();
const dialog = useDialog();
const aiConfigStore = useAiConfigStore();

// ========================
// 导航与环境状态
// ========================
const activeTab = ref<'system' | 'market' | 'ai' | 'notify' | 'backup'>('system');
const isWebMode = ref(!isWails());
const currentAuthToken = ref(getAuthToken());
const pingLatency = ref<number | null>(null);
const checkingPing = ref(false);

async function checkServerPing() {
  if (isWails()) {
    pingLatency.value = 0;
    return;
  }
  checkingPing.value = true;
  const t0 = performance.now();
  try {
    const res = await fetch('/api/health?t=' + Date.now(), { cache: 'no-store' });
    if (res.ok) {
      pingLatency.value = Math.round(performance.now() - t0);
    } else {
      pingLatency.value = -1;
    }
  } catch (e) {
    pingLatency.value = -1;
  } finally {
    checkingPing.value = false;
  }
}

function handleOpenAuthManage() {
  openAuthModal();
}

async function handleWebLogout() {
  dialog.warning({
    title: '退出登录',
    content: '确定要注销当前的 Web 访问凭证并退出系统吗？',
    positiveText: '确认注销',
    negativeText: '取消',
    onPositiveClick: () => {
      logoutWebAuth();
    }
  });
}

// ========================
// 投研提示词预设库
// ========================
const SYSTEM_PROMPT_PRESETS = {
  tushare_mentor: `你是一位拥有20年证券实操经验的资深量化投研专家与投资导师，秉持“风险控制第一，数据穿透为本”的原则。
在分析个股、板块或宏观市场时，必须通过工具调用获取真实客观的数据（严禁凭记忆编造数据）：
1. 涉及基本面体检与排雷时，优先组合调用 Tushare MCP 黄金工具：质押平仓风险（pledge_stat）、未来大额限售解禁（share_float）、真实ROE与销售毛利率（fina_indicator）、经营现金流质量（cashflow）、非标审计意见（fina_audit）；
2. 涉及资金与机构筹码时，优先调用：股东户数连续季度吸筹/分散趋势（stk_holdernumber）、两融杠杆买入（margin_detail）、北向资金（moneyflow_hsgt / hsgt_top10），异动个股穿透龙虎榜机构专用席位买卖净额（top_list / top_inst）；
3. 涉及财报前瞻与预期差时，调用业绩预告（forecast）与业绩快报（express）挖掘超预期增长潜能；
4. 任何操作建议均须附带第一支撑位、止损位与核心风险警示。`,
  standard: `你是一个专业股票助手，擅长结合量化数据、技术指标、资金流向和基本面信息，为用户提供客观严谨的投资分析和建议。`
};

const QUESTION_TEMPLATE_PRESETS = {
  comprehensive: `请对 {{stockName}}[{{stockCode}}] 进行全维度深度量化投研与排雷体检：
1. 基本面与排雷体检：调用 Tushare pledge_stat 核查控股股东质押比例，share_float 检查未来一年限售解禁日程，fina_indicator 查询近3年真实 ROE 与毛利率趋势，cashflow 检验经营现金流健康度，排除财务暴雷隐患；
2. 资金与机构席位追踪：调用 stk_holdernumber 穿透近3季度股东户数是集中还是发散（主力吸筹/出逃判定），margin_detail 查看两融杠杆多空动态，异动时调用 top_list 与 top_inst 穿透龙虎榜机构专用席位买卖净额；
3. 财报预期差与前瞻：调用 forecast 与 express 挖掘近期业绩预告是否超预期，结合 K 线趋势给出量化评分、操作建议与第一止损位。`,
  antirisk: `请对 {{stockName}}[{{stockCode}}] 进行严密的基本面防爆雷体检：
1. 股权质押排雷：调用 Tushare pledge_stat 检查控股股东最新质押比例及平仓风险；
2. 限售解禁排雷：调用 Tushare share_float 筛查未来 3~12 个月内是否有大额限售股解禁；
3. 财务真实性：调用 fina_indicator 与 cashflow 对比营收净利增速与经营现金流匹配度，排除财务造假嫌疑。`,
  funds: `请重点穿透 {{stockName}}[{{stockCode}}] 的资金博弈与筹码结构：
1. 筹码集中度：调用 Tushare stk_holdernumber 穿透近3季度股东户数是集中还是发散（主力吸筹/出货判定）；
2. 机构席位追踪：调用 Tushare top_list 与 top_inst 检索近期龙虎榜是否有机构专用席位大额买入；
3. 杠杆多空对比：结合两融 margin_detail 杠杆做多资金流向与超大单主力净流，给出多空力量对比。`,
  simple: `{{stockName}}[{{stockCode}}] 全方位量化诊断与投资分析总结`
};

function applyQuestionTemplatePreset(key: keyof typeof QUESTION_TEMPLATE_PRESETS) {
  if (QUESTION_TEMPLATE_PRESETS[key]) {
    questionTemplate.value = QUESTION_TEMPLATE_PRESETS[key];
    message.success('已填入个股分析模板，请点击底栏「保存设置」生效');
  }
}

function applySystemPromptPreset(key: keyof typeof SYSTEM_PROMPT_PRESETS) {
  if (SYSTEM_PROMPT_PRESETS[key]) {
    aiPrompt.value = SYSTEM_PROMPT_PRESETS[key];
    message.success('已填入系统提示词模板，请点击底栏「保存设置」生效');
  }
}

// ========================
// 核心表单状态与默认值
// ========================
const formRef = ref<any>(null);
const isSaving = ref(false);

const formValue = ref({
  ID: 1,
  tushareToken: '',
  emApiKey: '',
  tickFlowAPIKey: '',
  qgqpBId: '',
  dingPush: {
    enable: false,
    dingRobot: ''
  },
  localPush: {
    enable: true
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
  httpProxy: '',
  httpProxyEnabled: false,
  enableAgent: false,
  disableBrowserSearch: false,
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
  crawlTimeOut: 60,
  kDays: 90
});

const defaultDataSourceOptions = [
  { label: 'AData (开源极速行情)', value: 'adata' },
  { label: 'AKShare (金融大数据服务)', value: 'akshare' },
  { label: 'EastMoney (东方财富实时)', value: 'eastmoney' },
  { label: 'Sina (新浪财经接口)', value: 'sina' }
];

const formRules = {
  refreshInterval: [
    { required: true, type: 'number', message: '刷新间隔不能为空', trigger: ['blur', 'change'] }
  ],
  tushareToken: [
    { required: false, message: '请输入 Tushare Token', trigger: ['blur'] }
  ]
};

// ========================
// AI 配置矩阵与状态
// ========================
const openAiEnable = ref(false);
const defaultAiConfigId = ref<number | null>(null);
const aiPrompt = ref('');
const questionTemplate = ref('');
const aiConfigs = ref<any[]>([]);

interface ConfigMeta {
  loadingModels: boolean;
  testing: boolean;
  modelOptions: { label: string; value: string }[];
}
const aiConfigMeta = reactive<Record<number, ConfigMeta>>({});

function ensureConfigMeta(idx: number): ConfigMeta {
  if (!aiConfigMeta[idx]) {
    aiConfigMeta[idx] = {
      loadingModels: false,
      testing: false,
      modelOptions: []
    };
  }
  return aiConfigMeta[idx];
}

// ========================
// 脏检查与变更追踪 (Dirty Check)
// ========================
const originalFormSnapshot = ref<string>('');

function getFullFormSnapshot(): string {
  return JSON.stringify({
    form: formValue.value,
    openAiEnable: openAiEnable.value,
    defaultAiConfigId: defaultAiConfigId.value,
    aiPrompt: aiPrompt.value,
    questionTemplate: questionTemplate.value,
    aiConfigs: aiConfigs.value.map(c => ({
      ID: c.ID,
      name: c.name,
      baseUrl: c.baseUrl,
      apiKey: c.apiKey,
      modelName: c.modelName,
      temperature: c.temperature,
      maxTokens: c.maxTokens,
      timeOut: c.timeOut,
      thinking: c.thinking,
      httpProxy: c.httpProxy,
      httpProxyEnabled: c.httpProxyEnabled,
      llmConcurrencyLimit: c.llmConcurrencyLimit,
      subAgentTimeout: c.subAgentTimeout,
      rateLimitPerSecond: c.rateLimitPerSecond,
      burst: c.burst,
      dailyRequestQuota: c.dailyRequestQuota,
      dailyCostQuota: c.dailyCostQuota
    }))
  });
}

function markClean() {
  originalFormSnapshot.value = getFullFormSnapshot();
}

const isFormDirty = computed(() => {
  if (!originalFormSnapshot.value) return false;
  return originalFormSnapshot.value !== getFullFormSnapshot();
});

const dirtyCount = computed(() => {
  if (!isFormDirty.value || !originalFormSnapshot.value) return 0;
  try {
    const snap = JSON.parse(originalFormSnapshot.value);
    let count = 0;
    for (const k in formValue.value) {
      if (JSON.stringify((formValue.value as any)[k]) !== JSON.stringify(snap.form?.[k])) {
        count++;
      }
    }
    if (openAiEnable.value !== snap.openAiEnable) count++;
    if (defaultAiConfigId.value !== snap.defaultAiConfigId) count++;
    if (aiPrompt.value !== snap.aiPrompt) count++;
    if (questionTemplate.value !== snap.questionTemplate) count++;
    if (JSON.stringify(aiConfigs.value) !== JSON.stringify(snap.aiConfigs)) count++;
    return Math.max(1, count);
  } catch (e) {
    return 1;
  }
});

// ========================
// AI 配置项操作
// ========================
function addAiConfig() {
  const newConfig = {
    ID: 0,
    name: '新建 AI 模型配置',
    baseUrl: 'https://api.minimax.chat/v1',
    apiKey: '',
    modelName: 'MiniMax-M3',
    temperature: 0.7,
    maxTokens: 8192,
    timeOut: 300,
    thinking: false,
    httpProxy: '',
    httpProxyEnabled: false,
    llmConcurrencyLimit: 2,
    subAgentTimeout: 20,
    rateLimitPerSecond: 0,
    burst: 0,
    dailyRequestQuota: 0,
    dailyCostQuota: 0
  };
  aiConfigs.value.push(newConfig);
  ensureConfigMeta(aiConfigs.value.length - 1);
  if (!defaultAiConfigId.value) {
    defaultAiConfigId.value = 0;
  }
  message.info('已新增配置卡片，请完善参数后保存');
}

function removeAiConfig(index: number) {
  dialog.warning({
    title: '确认删除配置',
    content: `确定要删除「${aiConfigs.value[index]?.name || '未命名配置'}」吗？`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: () => {
      aiConfigs.value.splice(index, 1);
      delete aiConfigMeta[index];
      message.success('已移除该模型配置');
    }
  });
}

function copyAiConfig(index: number) {
  const source = aiConfigs.value[index];
  if (!source) return;
  const copy = {
    ...JSON.parse(JSON.stringify(source)),
    ID: 0,
    name: `${source.name} (副本)`
  };
  aiConfigs.value.push(copy);
  ensureConfigMeta(aiConfigs.value.length - 1);
  message.success('已复制配置卡片');
}

function getPlatformName(baseUrl: string): string {
  if (!baseUrl) return '';
  const lower = baseUrl.toLowerCase();
  if (lower.includes('minimax')) return 'MiniMax 官方直连';
  if (lower.includes('nvidia') || lower.includes('nim')) return 'NVIDIA NIM 极速平台';
  if (lower.includes('deepseek')) return 'DeepSeek 官方';
  if (lower.includes('siliconflow')) return '硅基流动 SiliconFlow';
  if (lower.includes('bigmodel') || lower.includes('zhipu')) return '智谱 AI (GLM)';
  if (lower.includes('volces') || lower.includes('ark')) return '字节跳动 (火山引擎)';
  if (lower.includes('aliyun') || lower.includes('dashscope')) return '阿里云百炼 (通义千问)';
  if (lower.includes('openai.com')) return 'OpenAI 官方';
  if (lower.includes('openrouter')) return 'OpenRouter (聚合)';
  if (lower.includes('localhost:11434') || lower.includes('127.0.0.1:11434')) return 'Ollama 本地部署';
  if (lower.includes('localhost:1234') || lower.includes('127.0.0.1:1234')) return 'LM Studio 本地部署';
  return '自定义网关';
}

async function fetchAiModels(aiConfig: any, index: number, opts: { silent?: boolean } = {}) {
  const meta = ensureConfigMeta(index);
  if (!aiConfig.baseUrl) {
    if (!opts.silent) message.warning('请先填写接口地址 (BaseUrl)');
    return;
  }
  meta.loadingModels = true;
  try {
    const list: string[] = await FetchAiModels(aiConfig.baseUrl, aiConfig.apiKey || '', aiConfig.httpProxyEnabled ? aiConfig.httpProxy : '');
    if (list && list.length > 0) {
      meta.modelOptions = list.map(m => ({ label: m, value: m }));
      if (!opts.silent) message.success(`成功拉取 ${list.length} 个可用模型`);
      if (!aiConfig.modelName && list[0]) {
        aiConfig.modelName = list[0];
      }
    } else {
      if (!opts.silent) message.warning('未获取到模型列表，请手动输入模型名称');
    }
  } catch (err: any) {
    if (!opts.silent) message.error('拉取模型失败: ' + (err?.message || err));
  } finally {
    meta.loadingModels = false;
  }
}

async function testAiConfig(aiConfig: any, index: number) {
  const meta = ensureConfigMeta(index);
  if (!aiConfig.baseUrl) {
    message.warning('请先填写接口地址');
    return;
  }
  meta.testing = true;
  try {
    const cfg = {
      ...aiConfig,
      maxTokens: Number(aiConfig.maxTokens) || 4096,
      temperature: Number(aiConfig.temperature) || 0.7,
      timeOut: Number(aiConfig.timeOut) || 300,
      thinking: Boolean(aiConfig.thinking),
      httpProxy: aiConfig.httpProxyEnabled ? aiConfig.httpProxy : '',
      httpProxyEnabled: Boolean(aiConfig.httpProxyEnabled)
    };
    const res = await TestAiConnection(cfg);
    if (res?.success) {
      message.success(`连接测试成功！模型响应耗时: ${res.latencyMs || 0} ms`);
    } else {
      message.error(`连接测试失败: ${res?.error || '无返回响应'}`);
    }
  } catch (err: any) {
    message.error('测试异常: ' + (err?.message || err));
  } finally {
    meta.testing = false;
  }
}

// ========================
// AI 配置向导 (Wizard Modal)
// ========================
interface AiConfigTemplate {
  type: string;
  label: string;
  description: string;
  baseUrl: string;
  modelName: string;
  apiKeyRequired: boolean;
  envHint: string;
}

const wizardVisible = ref(false);
const wizardStep = ref(1);
const wizardTemplates = ref<AiConfigTemplate[]>([]);
const wizardSelectedTemplate = ref<AiConfigTemplate | null>(null);
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
  subAgentTimeout: 20
});
const wizardTesting = ref(false);
const wizardTestResult = ref<string | null>(null);

async function openWizard() {
  if (!wizardTemplates.value.length) {
    try {
      const list = await GetAiConfigTemplates();
      wizardTemplates.value = list || [];
    } catch (e) {
      console.error('获取模板失败', e);
      message.error('获取配置模板失败');
      return;
    }
  }
  wizardStep.value = 1;
  wizardSelectedTemplate.value = null;
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
    subAgentTimeout: 20
  };
  wizardTestResult.value = null;
  wizardVisible.value = true;
}

function wizardSelectTemplate(tpl: AiConfigTemplate) {
  wizardSelectedTemplate.value = tpl;
  wizardForm.value.name = tpl.label.split('（')[0].trim();
  wizardForm.value.baseUrl = tpl.baseUrl;
  wizardForm.value.modelName = tpl.modelName;
  wizardForm.value.apiKey = '';
  wizardForm.value.thinking = false;
  wizardStep.value = 2;
}

async function wizardTestConnection() {
  wizardTesting.value = true;
  wizardTestResult.value = null;
  try {
    const res = await TestAiConnection({
      ...wizardForm.value,
      ID: 0,
      thinking: Boolean(wizardForm.value.thinking)
    });
    if (res?.success) {
      wizardTestResult.value = `✅ 连接成功！响应延迟: ${res.latencyMs || 0} ms`;
      message.success('连通性测试通过！');
    } else {
      wizardTestResult.value = `❌ 连接失败: ${res?.error || '无详细错误'}`;
      message.error('连通性测试未通过，请检查配置');
    }
  } catch (err: any) {
    wizardTestResult.value = `❌ 异常: ${err?.message || err}`;
  } finally {
    wizardTesting.value = false;
  }
}

function wizardSaveAndClose() {
  if (!wizardForm.value.name) {
    message.warning('请填写配置名称');
    return;
  }
  if (!wizardForm.value.baseUrl) {
    message.warning('请填写接口地址');
    return;
  }
  aiConfigs.value.push({
    ...wizardForm.value,
    ID: 0
  });
  ensureConfigMeta(aiConfigs.value.length - 1);
  wizardVisible.value = false;
  message.success('已添加新模型配置，请点击底栏「保存设置」生效！');
}

// ========================
// 知识结晶与语义向量蒸馏
// ========================
const distillStats = ref<any>({
  totalSessions: 0,
  distilledSessions: 0,
  totalCrystals: 0,
  crystallizedTokens: 0,
  autoDistillEnabled: false
});
const distilling = ref(false);

async function fetchDistillStats() {
  try {
    const s = await GetDistillationStats();
    if (s) distillStats.value = s;
  } catch (err) {
    console.error('获取知识蒸馏状态失败', err);
  }
}

async function handleTriggerDistill() {
  distilling.value = true;
  try {
    const count = await TriggerSemanticDistillation(false);
    message.success(`语义蒸馏完成！本次共结晶固化 ${count} 个对话轮次。`);
    await fetchDistillStats();
  } catch (err: any) {
    message.error('触发语义蒸馏失败: ' + (err?.message || err));
  } finally {
    distilling.value = false;
  }
}

// ========================
// 智能数据资产与灾备中心
// ========================
const loadingBackupCenter = ref(false);
const backupOverview = ref<any>(null);
const backups = ref<any[]>([]);

async function fetchBackupCenterData() {
  loadingBackupCenter.value = true;
  try {
    const [ov, list] = await Promise.all([
      GetBackupOverview(),
      ListBackups()
    ]);
    backupOverview.value = ov;
    backups.value = list || [];
  } catch (e: any) {
    console.error('加载灾备中心数据失败', e);
  } finally {
    loadingBackupCenter.value = false;
  }
}

function triggerBrowserDownload(fileName: string, content: string, isBinary = false) {
  let blob: Blob;
  if (isBinary) {
    const byteChars = atob(content);
    const byteNumbers = new Array(byteChars.length);
    for (let i = 0; i < byteChars.length; i++) {
      byteNumbers[i] = byteChars.charCodeAt(i);
    }
    blob = new Blob([new Uint8Array(byteNumbers)], { type: 'application/octet-stream' });
  } else {
    blob = new Blob([content], { type: 'application/json;charset=utf-8' });
  }
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

async function handleCreateIntelligentBackup() {
  dialog.info({
    title: '生成全量智能资产包',
    content: '全量智能资产包将打包导出：系统核心配置、所有 AI模型配置与密钥、AI技能与提示词、所有历史对话与长期记忆、全部深度研报、自选股池与量化回测记录。确认生成吗？',
    positiveText: '开始生成',
    negativeText: '取消',
    onPositiveClick: async () => {
      const loadingMsg = message.loading('正在打包全量智能资产包...', { duration: 0 });
      try {
        const res = await CreateIntelligentBackup();
        loadingMsg.destroy();
        message.success(res || '全量智能资产包生成成功！');
        await fetchBackupCenterData();
      } catch (err: any) {
        loadingMsg.destroy();
        message.error('生成失败: ' + (err?.message || err));
      }
    }
  });
}

async function handleCreateDatabaseSnapshot() {
  dialog.info({
    title: '创建数据库原子物理快照',
    content: '将在后台执行 WAL 事务合并与物理对齐，生成无锁纯净的 SQLite 镜像快照（.db）。确认生成吗？',
    positiveText: '创建快照',
    negativeText: '取消',
    onPositiveClick: async () => {
      const loadingMsg = message.loading('正在执行无锁物理快照...', { duration: 0 });
      try {
        const res = await CreateDatabaseSnapshot();
        loadingMsg.destroy();
        message.success(res || '数据库快照创建成功！');
        await fetchBackupCenterData();
      } catch (err: any) {
        loadingMsg.destroy();
        message.error('快照创建失败: ' + (err?.message || err));
      }
    }
  });
}

async function downloadBackup(item: any) {
  try {
    message.loading('正在准备下载...', { duration: 1500 });
    if (!isWails()) {
      const token = currentAuthToken.value || localStorage.getItem('blades_web_auth_token') || '';
      const downloadUrl = `/api/backup/download?file=${encodeURIComponent(item.fileName)}&token=${encodeURIComponent(token)}`;
      const link = document.createElement('a');
      link.href = downloadUrl;
      link.setAttribute('download', item.fileName);
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      message.success(`已开始下载: ${item.fileName}`);
      return;
    }
    const isBin = item.type === 'db_snapshot';
    const content = await GetBackupContent(item.fileName, isBin);
    if (!content) {
      message.error('读取备份文件内容为空');
      return;
    }
    triggerBrowserDownload(item.fileName, content, isBin);
    message.success(`备份文件已导出至本地: ${item.fileName}`);
  } catch (err: any) {
    message.error('下载失败: ' + (err?.message || err));
  }
}

async function restoreBackupItem(item: any) {
  const isBundle = item.type === 'intel_bundle';
  const confirmText = isBundle
    ? `确定要从全量资产包「${item.fileName}」恢复系统数据吗？包含的模型配置、技能提示词、对话记忆与研报将被安全合并更新。`
    : `确定要恢复数据库物理快照「${item.fileName}」吗？系统将在恢复前自动创建安全副本保护。`;

  dialog.warning({
    title: '确认恢复数据',
    content: confirmText,
    positiveText: '确认恢复',
    negativeText: '取消',
    onPositiveClick: async () => {
      const loadingMsg = message.loading('正在恢复数据，请稍候...', { duration: 0 });
      try {
        let res = '';
        if (isBundle) {
          const content = await GetBackupContent(item.fileName, false);
          res = await ImportIntelligentBackup(content);
        } else {
          res = await RestoreBackupFile(item.fileName);
        }
        loadingMsg.destroy();
        dialog.success({
          title: '恢复成功',
          content: res || '数据已成功恢复，建议刷新页面或重启客户端以加载最新配置。'
        });
        await loadConfig();
        await fetchBackupCenterData();
      } catch (err: any) {
        loadingMsg.destroy();
        message.error('恢复失败: ' + (err?.message || err));
      }
    }
  });
}

async function deleteBackupItem(item: any) {
  dialog.error({
    title: '确认删除归档',
    content: `确定要永久删除备份文件「${item.fileName}」吗？此操作不可逆。`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await DeleteBackupFile(item.fileName);
        message.success('已删除备份文件: ' + item.fileName);
        await fetchBackupCenterData();
      } catch (err: any) {
        message.error('删除失败: ' + (err?.message || err));
      }
    }
  });
}

function uploadAndRestoreBackup() {
  const input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = async (re) => {
      const content = re.target?.result as string;
      if (!content) {
        message.error('所选文件为空');
        return;
      }
      dialog.warning({
        title: '确认导入外部智能资产包',
        content: `已读取文件「${file.name}」(${Math.round(file.size / 1024)} KB)，确定要将其中的配置、技能、会话与研报合并恢复到当前系统吗？`,
        positiveText: '确认合并导入',
        negativeText: '取消',
        onPositiveClick: async () => {
          const loadingMsg = message.loading('正在导入资产包并更新数据库...', { duration: 0 });
          try {
            const res = await ImportIntelligentBackup(content);
            loadingMsg.destroy();
            dialog.success({
              title: '导入恢复成功',
              content: res
            });
            await loadConfig();
            await fetchBackupCenterData();
          } catch (err: any) {
            loadingMsg.destroy();
            message.error('导入失败: ' + (err?.message || err));
          }
        }
      });
    };
    reader.readAsText(file);
  };
  input.click();
}

async function cleanup(type: string) {
  const typeNameMap: Record<string, string> = {
    logs: '日志',
    chat_sessions: 'AI对话历史',
    ai_results: 'AI分析结果',
    test_images: '测试图片',
    browser_temp: '浏览器临时目录',
    all: '所有中间缓存数据'
  };
  const typeName = typeNameMap[type] || type;
  dialog.warning({
    title: '确认清理缓存',
    content: `确定要清理${typeName}吗？此操作不可恢复。`,
    positiveText: '确认清理',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        const res = await CleanupData(type);
        message.success(res || '清理完成');
        await fetchBackupCenterData();
      } catch (err: any) {
        message.error('清理失败: ' + (err?.message || err));
      }
    }
  });
}

// ========================
// 保存与持久化逻辑
// ========================
async function saveConfig() {
  if (isSaving.value) return;
  try {
    if (formRef.value) {
      await formRef.value.validate();
    }
  } catch (valErr: any) {
    if (Array.isArray(valErr) && valErr.length > 0 && valErr[0]?.[0]?.message) {
      message.warning('请检查表单：' + valErr[0][0].message);
      return;
    }
  }

  // 严格校验 AI 配置项
  for (let i = 0; i < aiConfigs.value.length; i++) {
    const cfg = aiConfigs.value[i];
    if (!cfg.name || !cfg.name.trim()) {
      message.warning(`第 ${i + 1} 项 AI 配置未填写配置名称`);
      activeTab.value = 'ai';
      return;
    }
    if (!cfg.baseUrl || !cfg.baseUrl.trim()) {
      message.warning(`配置「${cfg.name}」未填写接口地址`);
      activeTab.value = 'ai';
      return;
    }
    if (!cfg.modelName || !cfg.modelName.trim()) {
      message.warning(`配置「${cfg.name}」未填写模型名称`);
      activeTab.value = 'ai';
      return;
    }
    cfg.temperature = Number(cfg.temperature);
    if (isNaN(cfg.temperature) || cfg.temperature < 0 || cfg.temperature > 2) {
      cfg.temperature = 0.7;
    }
    cfg.maxTokens = Number(cfg.maxTokens);
    if (isNaN(cfg.maxTokens) || cfg.maxTokens < 256) {
      cfg.maxTokens = 4096;
    }
    cfg.timeOut = Number(cfg.timeOut);
    if (isNaN(cfg.timeOut) || cfg.timeOut < 10) {
      cfg.timeOut = 300;
    }
  }

  isSaving.value = true;
  const configToSave: any = {
    ...formValue.value,
    openAiEnable: openAiEnable.value,
    aiConfigId: defaultAiConfigId.value || 0,
    prompt: aiPrompt.value,
    questionTemplate: questionTemplate.value,
    aiConfigs: aiConfigs.value.map(c => ({
      ...c,
      ID: c.ID || 0,
      thinking: Boolean(c.thinking),
      httpProxyEnabled: Boolean(c.httpProxyEnabled)
    }))
  };

  try {
    const res = await UpdateConfig(configToSave);
    message.success(res || '设置保存成功！');
    markClean();
    aiConfigStore.fetchAiConfigs();
    EventsEmit('config_updated');
  } catch (err: any) {
    message.error('保存失败: ' + (err?.message || err));
  } finally {
    isSaving.value = false;
  }
}

async function cancelChanges() {
  if (!isFormDirty.value) return;
  dialog.warning({
    title: '放弃未保存的修改？',
    content: '当前有未保存的设置变动，确定要放弃并恢复到最后保存的状态吗？',
    positiveText: '确定放弃',
    negativeText: '继续编辑',
    onPositiveClick: () => {
      if (originalFormSnapshot.value) {
        try {
          const snap = JSON.parse(originalFormSnapshot.value);
          if (snap.form) formValue.value = JSON.parse(JSON.stringify(snap.form));
          if (snap.openAiEnable !== undefined) openAiEnable.value = snap.openAiEnable;
          if (snap.defaultAiConfigId !== undefined) defaultAiConfigId.value = snap.defaultAiConfigId;
          if (snap.aiPrompt !== undefined) aiPrompt.value = snap.aiPrompt;
          if (snap.questionTemplate !== undefined) questionTemplate.value = snap.questionTemplate;
          if (snap.aiConfigs) aiConfigs.value = JSON.parse(JSON.stringify(snap.aiConfigs));
          message.info('已恢复为修改前的配置快照');
        } catch (e) {
          console.error('快照恢复失败', e);
        }
      }
    }
  });
}

async function resetToDefaults() {
  dialog.warning({
    title: '确认恢复出厂默认值？',
    content: '这将把所有配置参数重置为出厂初始值，未保存修改将丢失。确认恢复吗？',
    positiveText: '确认恢复',
    negativeText: '取消',
    onPositiveClick: async () => {
      await loadConfig();
      message.success('已恢复出厂初始设置');
    }
  });
}

function exportConfig() {
  ExportConfig().then((res: any) => {
    if (!res) {
      message.error('导出失败: 配置内容为空');
      return;
    }
    const fileName = `blades-stock-config-${new Date().toISOString().slice(0, 10)}.json`;
    triggerBrowserDownload(fileName, res);
    message.success('配置文件导出成功！');
  }).catch((err: any) => {
    message.error('导出失败: ' + (err?.message || err));
  });
}

function importConfig() {
  const input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = (e) => {
    const file = (e.target as HTMLInputElement).files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = async (re) => {
      try {
        const config = JSON.parse(re.target?.result as string);
        dialog.warning({
          title: '确认导入配置文件',
          content: `即将导入文件「${file.name}」，这将覆盖当前配置。确认继续吗？`,
          positiveText: '确认导入',
          negativeText: '取消',
          onPositiveClick: async () => {
            await UpdateConfig(config);
            message.success('配置导入成功！');
            await loadConfig();
          }
        });
      } catch (err: any) {
        message.error('解析配置文件失败: ' + (err?.message || err));
      }
    };
    reader.readAsText(file);
  };
  input.click();
}

function sendTestNotice() {
  if (!formValue.value.dingPush.dingRobot) {
    message.warning('请先填写钉钉机器人的 Webhook 地址');
    return;
  }
  SendDingDingMessageByType('test', '【Blades Stock】量化投研通知测试：钉钉机器人链路配置正确！', '测试消息').then((res: any) => {
    message.success(res || '测试消息发送成功！');
  }).catch((err: any) => {
    message.error('发送失败: ' + (err?.message || err));
  });
}

async function loadConfig() {
  try {
    const res: any = await GetConfig();
    if (!res) return;

    formValue.value = {
      ID: res.ID || 1,
      tushareToken: res.tushareToken || '',
      emApiKey: res.emApiKey || '',
      tickFlowAPIKey: res.tickFlowAPIKey || '',
      qgqpBId: res.qgqpBId || '',
      dingPush: {
        enable: Boolean(res.dingPush?.enable),
        dingRobot: res.dingPush?.dingRobot || ''
      },
      localPush: {
        enable: res.localPush?.enable !== false
      },
      updateBasicInfoOnStart: Boolean(res.updateBasicInfoOnStart),
      refreshInterval: res.refreshInterval || 1,
      enableDanmu: false,
      browserPath: res.browserPath || '',
      enableNews: Boolean(res.enableNews),
      darkTheme: res.darkTheme !== false,
      enableFund: Boolean(res.enableFund),
      enablePushNews: res.enablePushNews !== false,
      enableOnlyPushRedNews: res.enableOnlyPushRedNews !== false,
      httpProxy: res.httpProxy || '',
      httpProxyEnabled: Boolean(res.httpProxyEnabled),
      enableAgent: Boolean(res.enableAgent),
      disableBrowserSearch: Boolean(res.disableBrowserSearch),
      backtestAccuracyThreshold: res.backtestAccuracyThreshold ?? 5.0,
      backtestVerifyDays: res.backtestVerifyDays ?? 2,
      backtestEnabled: res.backtestEnabled !== false,
      akshareServiceUrl: res.akshareServiceUrl || 'http://115.190.72.22:8089',
      aDataServiceUrl: res.aDataServiceUrl || 'http://115.190.72.22:8000',
      defaultDataSource: res.defaultDataSource || 'adata',
      embeddingEnabled: Boolean(res.embeddingEnabled),
      embeddingModelName: res.embeddingModelName || '',
      embeddingBaseUrl: res.embeddingBaseUrl || '',
      embeddingApiKey: res.embeddingApiKey || '',
      crawlTimeOut: res.crawlTimeOut || 60,
      kDays: res.kDays || 90
    };

    openAiEnable.value = Boolean(res.openAI?.enable);
    defaultAiConfigId.value = res.openAI?.defaultAiConfigId || res.aiConfigId || null;
    aiPrompt.value = res.openAI?.prompt || SYSTEM_PROMPT_PRESETS.tushare_mentor;
    questionTemplate.value = res.openAI?.questionTemplate || QUESTION_TEMPLATE_PRESETS.comprehensive;

    const cfgs = (res.openAI?.aiConfigs || []).map((c: any) => ({
      ID: c.ID || 0,
      name: c.name || 'AI 配置',
      baseUrl: c.baseUrl || 'https://api.minimax.chat/v1',
      apiKey: c.apiKey || '',
      modelName: c.modelName || 'MiniMax-M3',
      temperature: c.temperature ?? 0.7,
      maxTokens: c.maxTokens || 8192,
      timeOut: c.timeOut || 300,
      thinking: Boolean(c.thinking),
      httpProxy: c.httpProxy || '',
      httpProxyEnabled: Boolean(c.httpProxyEnabled),
      llmConcurrencyLimit: c.llmConcurrencyLimit || 2,
      subAgentTimeout: c.subAgentTimeout || 20,
      rateLimitPerSecond: c.rateLimitPerSecond || 0,
      burst: c.burst || 0,
      dailyRequestQuota: c.dailyRequestQuota || 0,
      dailyCostQuota: c.dailyCostQuota || 0
    }));

    aiConfigs.value = cfgs;
    cfgs.forEach((_: any, idx: number) => ensureConfigMeta(idx));

    if (!defaultAiConfigId.value && cfgs.length > 0) {
      defaultAiConfigId.value = cfgs[0].ID;
    }

    markClean();
  } catch (err: any) {
    message.error('加载系统配置失败: ' + (err?.message || err));
  }
}

function handleKeydown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault();
    if (isFormDirty.value) {
      saveConfig();
    }
  }
}

onMounted(async () => {
  window.addEventListener('keydown', handleKeydown);
  await loadConfig();
  await checkServerPing();
  await fetchBackupCenterData();
  await fetchDistillStats();
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeydown);
});
</script>

<template>
  <div class="settings-container">
    <!-- ============================================== -->
    <!-- 顶部：平台运行环境感知与双模状态栏             -->
    <!-- ============================================== -->
    <div class="runtime-status-bar">
      <div class="runtime-left">
        <div class="runtime-badge-group">
          <n-tag v-if="!isWebMode" type="success" size="small" round :bordered="false">
            🖥️ 桌面原生运行时 (Desktop GUI)
          </n-tag>
          <n-tag v-else type="info" size="small" round :bordered="false">
            🌐 云端 SaaS 运行时 (Web Service)
          </n-tag>
          <n-tag type="default" size="small" round :bordered="false" class="version-tag">
            Blades v1.6.0
          </n-tag>
        </div>
        <div class="storage-indicator">
          <span class="status-dot"></span>
          <span class="storage-text">SQLite WAL 存储就绪</span>
        </div>
      </div>

      <div class="runtime-right">
        <n-space align="center" size="small">
          <div v-if="pingLatency !== null" class="ping-badge">
            <span :class="['ping-dot', pingLatency > 200 ? 'warn' : 'ok']"></span>
            <span class="ping-text">{{ pingLatency === 0 ? '本地直连 (0ms)' : `${pingLatency} ms` }}</span>
          </div>
          <n-button size="tiny" quaternary type="primary" :loading="checkingPing" @click="checkServerPing">
            测试延迟
          </n-button>
          <n-divider vertical v-if="isWebMode" />
          <n-button v-if="isWebMode" size="tiny" type="primary" secondary @click="handleOpenAuthManage">
            🔐 密码门禁管理
          </n-button>
          <n-button v-if="isWebMode" size="tiny" quaternary type="error" @click="handleWebLogout">
            安全注销
          </n-button>
        </n-space>
      </div>
    </div>

    <!-- ============================================== -->
    <!-- 主体：模块化现代五选项卡 (Tabs Layout)         -->
    <!-- ============================================== -->
    <n-form
      ref="formRef"
      :model="formValue"
      :rules="formRules"
      label-placement="top"
      class="settings-main-form"
    >
      <n-tabs v-model:value="activeTab" type="segment" animated class="settings-tabs">
        <!-- ------------------------------------------------ -->
        <!-- Tab 1: 基础与运行环境 (System & Environment)     -->
        <!-- ------------------------------------------------ -->
        <n-tab-pane name="system" tab="🖥️ 基础与运行环境">
          <div class="tab-pane-content">
            <!-- 外观与界面交互 -->
            <n-card title="外观与界面交互" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="8" label="界面主题风格">
                  <n-space align="center">
                    <n-switch v-model:value="formValue.darkTheme">
                      <template #checked>🌙 极致暗黑模式</template>
                      <template #unchecked>☀️ 亮色清新模式</template>
                    </n-switch>
                  </n-space>
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="行情刷新频率 (秒)" path="refreshInterval">
                  <n-input-number
                    v-model:value="formValue.refreshInterval"
                    :min="1"
                    :max="60"
                    style="width: 100%;"
                  >
                    <template #suffix>秒/次</template>
                  </n-input-number>
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="启动自动同步">
                  <n-switch v-model:value="formValue.updateBasicInfoOnStart">
                    <template #checked>已开启（启动时更新全市场基础信息）</template>
                    <template #unchecked>已关闭（加速启动）</template>
                  </n-switch>
                </n-form-item-gi>
              </n-grid>
            </n-card>

            <!-- 运行时与网络环境 -->
            <n-card title="本地环境与全局代理" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="16" label="本地浏览器安装路径 (桌面抓取专用)">
                  <n-input
                    v-model:value="formValue.browserPath"
                    placeholder="例如: C:\Program Files\Google\Chrome\Application\chrome.exe"
                    clearable
                  />
                  <template #feedback>
                    <span class="field-feedback">留空则自动从系统默认路径检测 Chrome / Edge 浏览器</span>
                  </template>
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="禁用无头浏览器爬虫">
                  <n-switch v-model:value="formValue.disableBrowserSearch">
                    <template #checked>已禁用 (仅使用标准 HTTP 请求)</template>
                    <template #unchecked>已允许 (支持高级无头抓取)</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="6" label="全局网络代理开关">
                  <n-switch v-model:value="formValue.httpProxyEnabled">
                    <template #checked>已启用网络代理</template>
                    <template #unchecked>直接连接 (直连网络)</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="18" v-if="formValue.httpProxyEnabled" label="HTTP/Socks 代理地址">
                  <n-input
                    v-model:value="formValue.httpProxy"
                    placeholder="例如: http://127.0.0.1:7890 或 socks5://127.0.0.1:1080"
                    clearable
                  />
                </n-form-item-gi>
              </n-grid>
            </n-card>
          </div>
        </n-tab-pane>

        <!-- ------------------------------------------------ -->
        <!-- Tab 2: 行情与量化数据源 (Market & Pipelines)     -->
        <!-- ------------------------------------------------ -->
        <n-tab-pane name="market" tab="📊 行情与量化数据源">
          <div class="tab-pane-content">
            <!-- Tushare Pro 官方数据中枢 -->
            <n-card size="small" class="settings-card highlight-card">
              <template #header>
                <div class="card-header-flex">
                  <span class="card-title">📈 Tushare Pro 权威金融大数据中枢</span>
                  <n-tag type="primary" size="tiny" round>核心排雷与机构席位</n-tag>
                </div>
              </template>
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="18" label="Tushare API Token" path="tushareToken">
                  <n-input
                    v-model:value="formValue.tushareToken"
                    type="password"
                    show-password-on="click"
                    placeholder="在此输入您的 Tushare Pro 访问令牌 (Token)"
                    clearable
                  />
                  <template #feedback>
                    <span class="field-feedback">用于质押排雷 (pledge_stat)、限售解禁 (share_float)、股东户数与龙虎榜机构专用席位穿透</span>
                  </template>
                </n-form-item-gi>

                <n-form-item-gi :span="6" label="数据源接入状态">
                  <div class="token-status-box">
                    <n-tag :type="formValue.tushareToken ? 'success' : 'warning'" size="small">
                      {{ formValue.tushareToken ? '✓ Token 已配置' : '⚠️ 未配置 (基础行情直连)' }}
                    </n-tag>
                  </div>
                </n-form-item-gi>
              </n-grid>
            </n-card>

            <!-- 东方财富与高频分时接口 -->
            <n-card title="高频分时与盘口行情源" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="12" label="东方财富网页凭证唯一标识 (qgqp_b_id)">
                  <n-input
                    v-model:value="formValue.qgqpBId"
                    placeholder="东财资金流与行情 cookie 标识"
                    clearable
                  />
                  <template #feedback>
                    <span class="field-feedback">可选，用于东方财富超大单资金博弈流数据穿透</span>
                  </template>
                </n-form-item-gi>

                <n-form-item-gi :span="12" label="TickFlow Level-2 API Key">
                  <n-input
                    v-model:value="formValue.tickFlowAPIKey"
                    type="password"
                    show-password-on="click"
                    placeholder="输入 TickFlow 高频数据服务密钥"
                    clearable
                  />
                  <template #feedback>
                    <span class="field-feedback">用于极速十档买卖盘与逐笔委托量化诊断</span>
                  </template>
                </n-form-item-gi>
              </n-grid>
            </n-card>

            <!-- 第三方开源量化服务与视图开关 -->
            <n-card title="远程服务路由与投研视图" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="8" label="默认行情分发引擎">
                  <n-select
                    v-model:value="formValue.defaultDataSource"
                    :options="defaultDataSourceOptions"
                  />
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="AKShare 远程微服务">
                  <n-input
                    v-model:value="formValue.akshareServiceUrl"
                    placeholder="http://115.190.72.22:8089"
                  />
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="AData 远程微服务">
                  <n-input
                    v-model:value="formValue.aDataServiceUrl"
                    placeholder="http://115.190.72.22:8000"
                  />
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="指数与基金投研跟踪">
                  <n-switch v-model:value="formValue.enableFund">
                    <template #checked>已启用 ETF与公募基金矩阵</template>
                    <template #unchecked>已关闭</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="实时资讯快讯条">
                  <n-switch v-model:value="formValue.enableNews">
                    <template #checked>顶部展示滚动快讯</template>
                    <template #unchecked>隐藏快讯条</template>
                  </n-switch>
                </n-form-item-gi>
              </n-grid>
            </n-card>
          </div>
        </n-tab-pane>

        <!-- ------------------------------------------------ -->
        <!-- Tab 3: AI模型与智能体矩阵 (AI Agent & Matrix)     -->
        <!-- ------------------------------------------------ -->
        <n-tab-pane name="ai" tab="🤖 AI模型与智能体矩阵">
          <div class="tab-pane-content">
            <!-- AI 全局总控与默认模型 -->
            <n-card size="small" class="settings-card highlight-card">
              <div class="ai-header-bar">
                <div class="ai-header-left">
                  <span class="card-title">🧠 智能体与大模型网关总控</span>
                  <n-switch v-model:value="openAiEnable" size="large" style="margin-left: 12px;">
                    <template #checked>AI 研报与多智能体已就绪</template>
                    <template #unchecked>AI 功能已挂起</template>
                  </n-switch>
                </div>
                <div class="ai-header-right">
                  <n-button type="primary" secondary size="small" @click="openWizard">
                    ⚡ 快速添加模型向导
                  </n-button>
                  <n-button type="primary" dashed size="small" @click="addAiConfig" style="margin-left: 8px;">
                    + 手动新建配置
                  </n-button>
                </div>
              </div>

              <div v-if="openAiEnable && aiConfigs.length > 0" class="default-model-selector">
                <n-divider style="margin: 12px 0;" />
                <n-grid :cols="24" :x-gap="20" align-items="center">
                  <n-form-item-gi :span="12" label="🎯 全局默认投研大模型">
                    <n-select
                      v-model:value="defaultAiConfigId"
                      :options="aiConfigs.map(c => ({
                        label: `${c.name} (${c.modelName}) [${getPlatformName(c.baseUrl)}]`,
                        value: c.ID || 0
                      }))"
                      placeholder="请指定主推理模型"
                    />
                  </n-form-item-gi>
                  <n-form-item-gi :span="6" label="智能体并发上限">
                    <n-input-number
                      v-model:value="formValue.crawlTimeOut"
                      :min="1"
                      :max="10"
                      placeholder="并发数"
                    >
                      <template #suffix>路并行</template>
                    </n-input-number>
                  </n-form-item-gi>
                  <n-form-item-gi :span="6" label="K线回溯分析天数">
                    <n-input-number
                      v-model:value="formValue.kDays"
                      :min="30"
                      :max="365"
                    >
                      <template #suffix>天历史</template>
                    </n-input-number>
                  </n-form-item-gi>
                </n-grid>
              </div>
            </n-card>

            <!-- 多模型配置卡片列表 -->
            <div v-if="openAiEnable" class="ai-config-matrix">
              <div v-for="(aiConfig, index) in aiConfigs" :key="aiConfig.ID || index" class="ai-config-item">
                <n-card size="small" class="settings-card model-card" :bordered="true">
                  <template #header>
                    <div class="model-card-header">
                      <div class="model-card-title-group">
                        <span class="model-idx">#{{ index + 1 }}</span>
                        <span class="model-name">{{ aiConfig.name || '未命名配置' }}</span>
                        <n-tag size="tiny" type="info" round>{{ getPlatformName(aiConfig.baseUrl) }}</n-tag>
                        <n-tag v-if="defaultAiConfigId === (aiConfig.ID || 0)" size="tiny" type="success" round>
                          ★ 默认主模型
                        </n-tag>
                      </div>
                      <div class="model-card-actions">
                        <n-button
                          size="tiny"
                          type="primary"
                          secondary
                          :loading="ensureConfigMeta(index).testing"
                          @click="testAiConfig(aiConfig, index)"
                        >
                          连通测试
                        </n-button>
                        <n-button
                          size="tiny"
                          quaternary
                          type="info"
                          :loading="ensureConfigMeta(index).loadingModels"
                          @click="fetchAiModels(aiConfig, index)"
                        >
                          拉取模型列表
                        </n-button>
                        <n-button size="tiny" quaternary type="default" @click="copyAiConfig(index)">
                          复制
                        </n-button>
                        <n-button size="tiny" quaternary type="error" @click="removeAiConfig(index)">
                          删除
                        </n-button>
                      </div>
                    </div>
                  </template>

                  <!-- 卡片内部网格表单 -->
                  <n-grid :cols="24" :x-gap="16" :y-gap="10">
                    <n-form-item-gi :span="6" label="配置别名">
                      <n-input v-model:value="aiConfig.name" placeholder="例如: MiniMax-M3 投研专用" />
                    </n-form-item-gi>

                    <n-form-item-gi :span="10" label="接口地址 (Base URL)">
                      <n-input v-model:value="aiConfig.baseUrl" placeholder="https://api.minimax.chat/v1" />
                    </n-form-item-gi>

                    <n-form-item-gi :span="8" label="令牌 (API Key)">
                      <n-input
                        v-model:value="aiConfig.apiKey"
                        type="password"
                        show-password-on="click"
                        placeholder="sk-..."
                      />
                    </n-form-item-gi>

                    <n-form-item-gi :span="8" label="模型名称 (Model Identifier)">
                      <n-input
                        v-if="!ensureConfigMeta(index).modelOptions.length"
                        v-model:value="aiConfig.modelName"
                        placeholder="例如: MiniMax-M3 或 z-ai/glm-5.3-flash"
                      />
                      <n-select
                        v-else
                        v-model:value="aiConfig.modelName"
                        :options="ensureConfigMeta(index).modelOptions"
                        filterable
                        tag
                      />
                    </n-form-item-gi>

                    <n-form-item-gi :span="4" label="采样温度 (Temp)">
                      <n-input-number
                        v-model:value="aiConfig.temperature"
                        :min="0"
                        :max="2"
                        :step="0.1"
                      />
                    </n-form-item-gi>

                    <n-form-item-gi :span="4" label="最大 Tokens">
                      <n-input-number
                        v-model:value="aiConfig.maxTokens"
                        :min="1024"
                        :max="128000"
                        :step="1024"
                      />
                    </n-form-item-gi>

                    <n-form-item-gi :span="4" label="超时时长 (秒)">
                      <n-input-number
                        v-model:value="aiConfig.timeOut"
                        :min="10"
                        :max="1800"
                        :step="30"
                      />
                    </n-form-item-gi>

                    <n-form-item-gi :span="4" label="深度思考开关">
                      <n-switch v-model:value="aiConfig.thinking">
                        <template #checked>开启 Thinking</template>
                        <template #unchecked>禁用思维链</template>
                      </n-switch>
                    </n-form-item-gi>

                    <n-form-item-gi :span="6" label="模型专用 HTTP 代理">
                      <n-switch v-model:value="aiConfig.httpProxyEnabled">
                        <template #checked>使用代理</template>
                        <template #unchecked>不使用代理</template>
                      </n-switch>
                    </n-form-item-gi>

                    <n-form-item-gi :span="18" v-if="aiConfig.httpProxyEnabled" label="代理地址">
                      <n-input v-model:value="aiConfig.httpProxy" placeholder="http://127.0.0.1:7890" />
                    </n-form-item-gi>
                  </n-grid>
                </n-card>
              </div>
            </div>

            <!-- 投研提示词预设库 -->
            <n-card v-if="openAiEnable" title="量化投研提示词工程中枢" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="12" label="全局系统角色提示词 (System Prompt)">
                  <div class="prompt-header-tools">
                    <span class="prompt-hint">一键应用标准预设模板：</span>
                    <n-button-group size="tiny">
                      <n-button type="primary" secondary @click="applySystemPromptPreset('tushare_mentor')">
                        🎓 Tushare 量化投研导师
                      </n-button>
                      <n-button secondary @click="applySystemPromptPreset('standard')">
                        🤖 标准证券助手
                      </n-button>
                    </n-button-group>
                  </div>
                  <n-input
                    v-model:value="aiPrompt"
                    type="textarea"
                    :rows="6"
                    placeholder="输入系统级角色指令..."
                  />
                </n-form-item-gi>

                <n-form-item-gi :span="12" label="个股全维量化诊断问题模板 (User Question Template)">
                  <div class="prompt-header-tools">
                    <span class="prompt-hint">一键应用个股分析模板：</span>
                    <n-button-group size="tiny">
                      <n-button type="primary" secondary @click="applyQuestionTemplatePreset('comprehensive')">
                        🔥 全维排雷与投研
                      </n-button>
                      <n-button type="error" secondary @click="applyQuestionTemplatePreset('antirisk')">
                        🛡️ 深度防爆雷
                      </n-button>
                      <n-button type="warning" secondary @click="applyQuestionTemplatePreset('funds')">
                        💰 筹码与席位
                      </n-button>
                      <n-button secondary @click="applyQuestionTemplatePreset('simple')">
                        ⚡ 经典简约
                      </n-button>
                    </n-button-group>
                  </div>
                  <n-input
                    v-model:value="questionTemplate"
                    type="textarea"
                    :rows="6"
                    placeholder="输入个股诊断提示词模板..."
                  />
                </n-form-item-gi>
              </n-grid>
            </n-card>

            <!-- 量化回测验证与向量嵌入 -->
            <n-card v-if="openAiEnable" title="量化回测与语义嵌入中枢" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="4" label="启用自动回测验证">
                  <n-switch v-model:value="formValue.backtestEnabled" />
                </n-form-item-gi>

                <n-form-item-gi :span="10" label="回测准确率胜率阈值 (%)">
                  <n-input-number
                    v-model:value="formValue.backtestAccuracyThreshold"
                    :min="1"
                    :max="100"
                    :step="0.5"
                  >
                    <template #suffix>% 准确度</template>
                  </n-input-number>
                </n-form-item-gi>

                <n-form-item-gi :span="10" label="回测验证历史窗口天数">
                  <n-input-number
                    v-model:value="formValue.backtestVerifyDays"
                    :min="1"
                    :max="30"
                  >
                    <template #suffix>天后表现验证</template>
                  </n-input-number>
                </n-form-item-gi>

                <n-form-item-gi :span="24">
                  <n-divider style="margin: 8px 0;" />
                </n-form-item-gi>

                <n-form-item-gi :span="4" label="语义向量嵌入模型">
                  <n-switch v-model:value="formValue.embeddingEnabled">
                    <template #checked>已启用</template>
                    <template #unchecked>已关闭</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="6" v-if="formValue.embeddingEnabled" label="向量模型名称">
                  <n-input v-model:value="formValue.embeddingModelName" placeholder="text-embedding-3-small" />
                </n-form-item-gi>

                <n-form-item-gi :span="7" v-if="formValue.embeddingEnabled" label="向量服务 Base URL">
                  <n-input v-model:value="formValue.embeddingBaseUrl" placeholder="https://api.openai.com/v1" />
                </n-form-item-gi>

                <n-form-item-gi :span="7" v-if="formValue.embeddingEnabled" label="向量服务 API Key">
                  <n-input
                    v-model:value="formValue.embeddingApiKey"
                    type="password"
                    show-password-on="click"
                    placeholder="sk-..."
                  />
                </n-form-item-gi>
              </n-grid>
            </n-card>
          </div>
        </n-tab-pane>

        <!-- ------------------------------------------------ -->
        <!-- Tab 4: 预警与消息推送 (Alerts & Notifications)   -->
        <!-- ------------------------------------------------ -->
        <n-tab-pane name="notify" tab="🔔 预警与消息推送">
          <div class="tab-pane-content">
            <!-- 钉钉机器人 -->
            <n-card size="small" class="settings-card highlight-card">
              <template #header>
                <div class="card-header-flex">
                  <span class="card-title">🤖 钉钉群机器人实时预警通道</span>
                  <n-tag type="info" size="tiny" round>支持异动/涨停/炸板推送</n-tag>
                </div>
              </template>
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="6" label="启用钉钉推送">
                  <n-switch v-model:value="formValue.dingPush.enable">
                    <template #checked>已启用推送</template>
                    <template #unchecked>已关闭推送</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="14" v-if="formValue.dingPush.enable" label="钉钉自定义机器人 Webhook 地址">
                  <n-input
                    v-model:value="formValue.dingPush.dingRobot"
                    type="password"
                    show-password-on="click"
                    placeholder="https://oapi.dingtalk.com/robot/send?access_token=..."
                    clearable
                  />
                </n-form-item-gi>

                <n-form-item-gi :span="4" v-if="formValue.dingPush.enable" label="链路测试">
                  <n-button type="primary" secondary style="width: 100%;" @click="sendTestNotice">
                    发送测试告警
                  </n-button>
                </n-form-item-gi>
              </n-grid>
            </n-card>

            <!-- 本地桌面与快讯过滤 -->
            <n-card title="桌面气泡通知与快讯过滤规则" size="small" class="settings-card">
              <n-grid :cols="24" :x-gap="20" :y-gap="12">
                <n-form-item-gi :span="8" label="桌面系统级原生弹窗通知">
                  <n-switch v-model:value="formValue.localPush.enable">
                    <template #checked>开启桌面原生推送</template>
                    <template #unchecked>静默模式</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="8" label="市场重大突发快讯提醒">
                  <n-switch v-model:value="formValue.enablePushNews">
                    <template #checked>实时弹窗提醒</template>
                    <template #unchecked>不提醒普通新闻</template>
                  </n-switch>
                </n-form-item-gi>

                <n-form-item-gi :span="8" v-if="formValue.enablePushNews" label="快讯内容智能筛选过滤">
                  <n-switch v-model:value="formValue.enableOnlyPushRedNews">
                    <template #checked>仅提醒红字突发或自选股</template>
                    <template #unchecked>接收全部新闻广播</template>
                  </n-switch>
                </n-form-item-gi>
              </n-grid>
            </n-card>
          </div>
        </n-tab-pane>

        <!-- ------------------------------------------------ -->
        <!-- Tab 5: 数据资产与灾备中心 (Data Assets & Recovery) -->
        <!-- ------------------------------------------------ -->
        <n-tab-pane name="backup" tab="🛡️ 数据资产与灾备中心">
          <div class="tab-pane-content">
            <!-- 核心数据资产统计驾驶舱 -->
            <n-card size="small" class="settings-card highlight-card">
              <template #header>
                <div class="card-header-flex">
                  <span class="card-title">📊 平台核心数据资产全景总览</span>
                  <n-button size="tiny" quaternary type="primary" :loading="loadingBackupCenter" @click="fetchBackupCenterData">
                    刷新资产看板
                  </n-button>
                </div>
              </template>
              <n-grid :cols="7" :x-gap="12" class="asset-stat-grid">
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="AI 大模型配置" :value="backupOverview?.aiConfigCount ?? '-'">
                      <template #suffix>项</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="提示词与技能" :value="backupOverview?.skillCount ?? '-'">
                      <template #suffix>个</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="AI 对话会话" :value="backupOverview?.sessionCount ?? '-'">
                      <template #suffix>条</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="长期记忆与结晶" :value="backupOverview?.memoryCount ?? '-'">
                      <template #suffix>条</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="深度分析研报" :value="backupOverview?.reportCount ?? '-'">
                      <template #suffix>篇</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="自选股票池" :value="backupOverview?.followedStockCount ?? '-'">
                      <template #suffix>只</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
                <n-gi>
                  <n-card size="small" embedded :bordered="false" class="stat-mini-card">
                    <n-statistic label="量化回测记录" :value="backupOverview?.backtestCount ?? '-'">
                      <template #suffix>次</template>
                    </n-statistic>
                  </n-card>
                </n-gi>
              </n-grid>
            </n-card>

            <!-- 一键灾备与快照创建操作条 -->
            <n-card size="small" class="settings-card">
              <div class="backup-action-bar">
                <div class="action-buttons-group">
                  <n-button type="primary" strong @click="handleCreateIntelligentBackup">
                    📦 生成全量智能资产包 (JSON)
                  </n-button>
                  <n-button type="info" secondary @click="handleCreateDatabaseSnapshot" style="margin-left: 10px;">
                    ⚡ 创建数据库无锁物理快照 (SQLite)
                  </n-button>
                  <n-button type="warning" secondary @click="uploadAndRestoreBackup" style="margin-left: 10px;">
                    📂 导入外部智能资产包并合并
                  </n-button>
                </div>
                <div class="action-right-hint">
                  <n-tag size="small" type="success" :bordered="false">物理对齐与完整性校验通过</n-tag>
                </div>
              </div>
            </n-card>

            <!-- 历史备份归档列表 -->
            <n-card title="备份归档与快照历史 (支持自动生命周期修剪)" size="small" class="settings-card">
              <div v-if="!backups.length" class="empty-backup-box">
                <n-text depth="3">暂无历史备份归档，请点击上方按钮一键创建</n-text>
              </div>
              <div v-else class="backup-list-box">
                <table class="modern-backup-table">
                  <thead>
                    <tr>
                      <th>备份文件名</th>
                      <th style="width: 130px;">类型</th>
                      <th style="width: 100px;">文件大小</th>
                      <th style="width: 170px;">创建时间</th>
                      <th style="width: 200px; text-align: center;">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="item in backups" :key="item.fileName">
                      <td>
                        <div class="backup-name-cell">
                          <span class="file-icon">{{ item.type === 'intel_bundle' ? '📦' : '🗄️' }}</span>
                          <span class="file-name" :title="item.summary">{{ item.fileName }}</span>
                        </div>
                      </td>
                      <td>
                        <n-tag :type="item.type === 'intel_bundle' ? 'primary' : 'success'" size="tiny" round>
                          {{ item.typeDesc }}
                        </n-tag>
                      </td>
                      <td>{{ item.sizeHuman }}</td>
                      <td>{{ item.modTime }}</td>
                      <td style="text-align: center;">
                        <n-space justify="center" size="tiny">
                          <n-button size="tiny" type="primary" secondary @click="downloadBackup(item)">
                            下载
                          </n-button>
                          <n-button size="tiny" type="warning" secondary @click="restoreBackupItem(item)">
                            恢复
                          </n-button>
                          <n-button size="tiny" type="error" quaternary @click="deleteBackupItem(item)">
                            删除
                          </n-button>
                        </n-space>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </n-card>

            <!-- 语义蒸馏与缓存清理 -->
            <n-grid :cols="24" :x-gap="20" class="sub-maintenance-grid">
              <n-gi :span="12">
                <n-card title="知识结晶与长期记忆蒸馏" size="small" class="settings-card">
                  <p class="section-desc">将零散的多轮投研对话提炼为结构化量化认知，结晶固化沉淀到长期向量记忆库中。</p>
                  <n-space justify="space-between" align="center" style="margin-top: 10px;">
                    <div>
                      <n-tag type="info" size="small">已提炼会话: {{ distillStats.distilledSessions }} / {{ distillStats.totalSessions }}</n-tag>
                      <n-tag type="success" size="small" style="margin-left: 8px;">固化知识元: {{ distillStats.totalCrystals }}</n-tag>
                    </div>
                    <n-button type="primary" secondary :loading="distilling" @click="handleTriggerDistill">
                      立即触发蒸馏
                    </n-button>
                  </n-space>
                </n-card>
              </n-gi>

              <n-gi :span="12">
                <n-card title="中间数据与系统缓存深度清理" size="small" class="settings-card">
                  <p class="section-desc">清理调试日志、历史截图、无头浏览器临时数据等，释放磁盘与数据库存储占用。</p>
                  <n-space wrap size="small" style="margin-top: 10px;">
                    <n-button size="tiny" type="warning" secondary @click="cleanup('logs')">清理日志</n-button>
                    <n-button size="tiny" type="warning" secondary @click="cleanup('test_images')">清理测试图</n-button>
                    <n-button size="tiny" type="warning" secondary @click="cleanup('browser_temp')">清理浏览器缓存</n-button>
                    <n-button size="tiny" type="error" secondary @click="cleanup('all')">一键全量清理</n-button>
                  </n-space>
                </n-card>
              </n-gi>
            </n-grid>
          </div>
        </n-tab-pane>
      </n-tabs>
    </n-form>

    <!-- ============================================== -->
    <!-- 悬浮吸底操作栏 (Sticky Bottom Action Bar)       -->
    <!-- ============================================== -->
    <div class="sticky-action-bar">
      <div class="bar-left">
        <transition name="fade">
          <div v-if="isFormDirty" class="dirty-badge">
            <span class="dirty-indicator-dot"></span>
            <span class="dirty-text">检测到 {{ dirtyCount }} 项未保存的设置变动</span>
          </div>
          <div v-else class="clean-badge">
            <span class="clean-indicator-dot"></span>
            <span class="clean-text">配置已与数据库完整同步</span>
          </div>
        </transition>
      </div>

      <div class="bar-right">
        <n-space align="center" size="medium">
          <n-button size="small" quaternary type="warning" @click="resetToDefaults">
            恢复出厂值
          </n-button>
          <n-button size="small" quaternary type="default" @click="exportConfig">
            导出配置(JSON)
          </n-button>
          <n-button size="small" quaternary type="default" @click="importConfig">
            导入配置
          </n-button>
          <n-button size="small" type="default" :disabled="!isFormDirty" @click="cancelChanges">
            放弃修改
          </n-button>
          <n-button
            size="small"
            type="primary"
            strong
            :loading="isSaving"
            :disabled="!isFormDirty"
            class="save-btn"
            @click="saveConfig"
          >
            💾 保存设置 (Ctrl + S)
          </n-button>
        </n-space>
      </div>
    </div>

    <!-- ============================================== -->
    <!-- AI 大模型快速创建向导弹窗 (Wizard Modal)        -->
    <!-- ============================================== -->
    <n-modal
      v-model:show="wizardVisible"
      preset="card"
      title="⚡ 快速添加 AI 大模型配置向导"
      style="width: 720px; max-width: 95vw;"
      :mask-closable="false"
    >
      <!-- Step 1: 选择厂商模板 -->
      <div v-if="wizardStep === 1">
        <n-alert type="info" :bordered="false" style="margin-bottom: 16px;">
          请从以下推荐厂商与服务中选择预设模板，系统将自动填充最优通信端点与模型规格：
        </n-alert>
        <div class="wizard-template-grid">
          <div
            v-for="tpl in wizardTemplates"
            :key="tpl.label"
            class="wizard-tpl-item"
            @click="wizardSelectTemplate(tpl)"
          >
            <div class="tpl-item-header">
              <span class="tpl-title">{{ tpl.label }}</span>
              <n-tag size="tiny" :type="tpl.type === 'cloud' ? 'primary' : 'success'">{{ tpl.type }}</n-tag>
            </div>
            <p class="tpl-desc">{{ tpl.description }}</p>
          </div>
        </div>
      </div>

      <!-- Step 2: 填写与测试 -->
      <div v-else-if="wizardStep === 2">
        <n-form :model="wizardForm" label-placement="top">
          <n-grid :cols="24" :x-gap="16" :y-gap="10">
            <n-form-item-gi :span="12" label="配置名称">
              <n-input v-model:value="wizardForm.name" />
            </n-form-item-gi>
            <n-form-item-gi :span="12" label="模型名称 (Model Identifier)">
              <n-input v-model:value="wizardForm.modelName" />
            </n-form-item-gi>
            <n-form-item-gi :span="24" label="接口地址 (Base URL)">
              <n-input v-model:value="wizardForm.baseUrl" />
            </n-form-item-gi>
            <n-form-item-gi :span="24" label="令牌 (API Key)">
              <n-input
                v-model:value="wizardForm.apiKey"
                type="password"
                show-password-on="click"
                placeholder="sk-..."
              />
              <template #feedback>
                <span class="field-feedback">{{ wizardSelectedTemplate?.envHint }}</span>
              </template>
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="并发限制">
              <n-input-number v-model:value="wizardForm.llmConcurrencyLimit" :min="1" :max="10" />
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="子Agent超时(秒)">
              <n-input-number v-model:value="wizardForm.subAgentTimeout" :min="5" :max="120" />
            </n-form-item-gi>
            <n-form-item-gi :span="8" label="深度思考模式">
              <n-switch v-model:value="wizardForm.thinking">
                <template #checked>开启 Thinking</template>
                <template #unchecked>关闭</template>
              </n-switch>
            </n-form-item-gi>
          </n-grid>
        </n-form>

        <div v-if="wizardTestResult" style="margin-top: 12px;">
          <n-alert :type="wizardTestResult.startsWith('✅') ? 'success' : 'error'" :bordered="false">
            {{ wizardTestResult }}
          </n-alert>
        </div>
      </div>

      <template #footer>
        <n-flex justify="space-between" align="center">
          <div>
            <n-button v-if="wizardStep === 2" size="small" @click="wizardStep = 1">
              上一步 (重新选厂商)
            </n-button>
          </div>
          <n-space>
            <n-button size="small" @click="wizardVisible = false">取消</n-button>
            <n-button
              v-if="wizardStep === 2"
              size="small"
              type="info"
              secondary
              :loading="wizardTesting"
              @click="wizardTestConnection"
            >
              测试连通性
            </n-button>
            <n-button
              v-if="wizardStep === 2"
              size="small"
              type="primary"
              strong
              @click="wizardSaveAndClose"
            >
              完成并加入配置列表
            </n-button>
          </n-space>
        </n-flex>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.settings-container {
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - 64px);
  position: relative;
  padding: 16px 20px 80px 20px;
  box-sizing: border-box;
}

/* 顶部状态栏 */
.runtime-status-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 16px;
  background: rgba(128, 128, 128, 0.05);
  border: 1px solid rgba(128, 128, 128, 0.15);
  border-radius: 8px;
  margin-bottom: 16px;
}

.runtime-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.runtime-badge-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.version-tag {
  font-weight: bold;
}

.storage-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  opacity: 0.85;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #18a058;
}

.ping-badge {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  padding: 2px 8px;
  background: rgba(128, 128, 128, 0.08);
  border-radius: 4px;
}

.ping-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.ping-dot.ok {
  background: #18a058;
}

.ping-dot.warn {
  background: #f0a020;
}

/* 主体表单与选项卡 */
.settings-main-form {
  flex: 1;
}

.settings-tabs {
  margin-top: 4px;
}

.tab-pane-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 12px;
}

.settings-card {
  border-radius: 8px;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.highlight-card {
  border-left: 3px solid #18a058;
}

.card-header-flex {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.card-title {
  font-size: 15px;
  font-weight: 600;
}

.field-feedback {
  font-size: 12px;
  opacity: 0.7;
}

.token-status-box {
  display: flex;
  align-items: center;
  height: 34px;
}

/* AI 专区 */
.ai-header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.ai-header-left {
  display: flex;
  align-items: center;
}

.ai-config-matrix {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.model-card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.model-card-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.model-idx {
  font-size: 13px;
  font-weight: bold;
  opacity: 0.6;
}

.model-name {
  font-size: 14px;
  font-weight: 600;
}

.prompt-header-tools {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.prompt-hint {
  font-size: 12px;
  opacity: 0.7;
}

/* 灾备中心 */
.asset-stat-grid {
  margin-top: 4px;
}

.stat-mini-card {
  border-radius: 6px;
  text-align: center;
}

.backup-action-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modern-backup-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.modern-backup-table th {
  text-align: left;
  padding: 8px 12px;
  background: rgba(128, 128, 128, 0.05);
  font-weight: 600;
  border-bottom: 1px solid rgba(128, 128, 128, 0.15);
}

.modern-backup-table td {
  padding: 8px 12px;
  border-bottom: 1px solid rgba(128, 128, 128, 0.08);
}

.backup-name-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-icon {
  font-size: 16px;
}

.empty-backup-box {
  padding: 30px;
  text-align: center;
}

.section-desc {
  font-size: 12px;
  opacity: 0.7;
  margin: 0;
}

/* 悬浮吸底操作栏 */
.sticky-action-bar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  height: 60px;
  background: rgba(28, 28, 30, 0.88);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border-top: 1px solid rgba(128, 128, 128, 0.2);
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0 28px;
  z-index: 1000;
  box-shadow: 0 -4px 16px rgba(0, 0, 0, 0.15);
}

.bar-left {
  display: flex;
  align-items: center;
}

.dirty-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #f0a020;
  font-size: 13px;
  font-weight: 500;
}

.dirty-indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #f0a020;
  animation: pulse-dot 1.5s infinite;
}

@keyframes pulse-dot {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(240, 160, 32, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 6px rgba(240, 160, 32, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(240, 160, 32, 0); }
}

.clean-badge {
  display: flex;
  align-items: center;
  gap: 8px;
  color: #18a058;
  font-size: 13px;
}

.clean-indicator-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #18a058;
}

.save-btn {
  padding: 0 20px;
  font-weight: 600;
}

/* Wizard 向导样式 */
.wizard-template-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
  max-height: 400px;
  overflow-y: auto;
  padding: 4px;
}

.wizard-tpl-item {
  border: 1px solid rgba(128, 128, 128, 0.2);
  border-radius: 6px;
  padding: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  background: rgba(128, 128, 128, 0.03);
}

.wizard-tpl-item:hover {
  border-color: #18a058;
  background: rgba(24, 160, 88, 0.08);
  transform: translateY(-2px);
}

.tpl-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.tpl-title {
  font-weight: 600;
  font-size: 14px;
}

.tpl-desc {
  font-size: 12px;
  opacity: 0.75;
  margin: 0;
  line-height: 1.4;
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.2s ease;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>
