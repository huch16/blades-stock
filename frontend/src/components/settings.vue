<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, computed, reactive, watch } from "vue";
import { useRouter } from "vue-router";
import {
  NCard, NTabs, NTabPane, NGrid, NGi, NForm, NFormItem,
  NInput, NInputNumber, NSwitch, NButton,
  NSpace, NDivider, NTag, NModal,
  NAlert, useMessage, useDialog
} from "naive-ui";
import { ArrowBackOutline } from "@vicons/ionicons5";
import { isWails, openAuthModal, logoutWebAuth } from "../api/bridge";
import {
  ExportConfig,
  GetConfig,
  SendDingDingMessageByType,
  UpdateConfig,
  FetchAiModels,
  TestAiConnection,
  CleanupData,
  CreateDatabaseSnapshot,
  GetAiConfigTemplates,
} from "../../wailsjs/go/main/App";

const router = useRouter();
const message = useMessage();
const dialog = useDialog();

// ==============================================================================
// 1. 运行时与环境状态 (Runtime Telemetry)
// ==============================================================================
const activeTab = ref<'system' | 'ai' | 'prompt' | 'market'>('system');
const isWebMode = ref(!isWails());
const pingLatency = ref<number | null>(null);
const checkingPing = ref(false);

const tabNameMap: Record<string, string> = {
  system: '基础与运行环境',
  ai: 'AI模型注册表',
  prompt: 'Prompt提示词中枢',
  market: '数据源与预警推送'
};
const activeTabTitle = computed(() => tabNameMap[activeTab.value] || '系统设置中枢');

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
  } catch (_e) {
    pingLatency.value = -1;
  } finally {
    checkingPing.value = false;
  }
}

function goBackToMain() {
  router.push({ name: 'stock' });
}

function navigateTo(routeName: string) {
  router.push({ name: routeName });
}

function handleOpenAuthManage() {
  openAuthModal();
}

async function handleWebLogout() {
  dialog.warning({
    title: '退出登录',
    content: '确定要注销当前的 Web 访问口令凭证并退出系统吗？',
    positiveText: '确认注销',
    negativeText: '取消',
    onPositiveClick: () => {
      logoutWebAuth();
    }
  });
}

// ==============================================================================
// 2. 表单数据模型 (Form State Model)
// ==============================================================================
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
  refreshInterval: 3,
  browserPath: '',
  enableNews: false,
  darkTheme: true,
  enableFund: false,
  enablePushNews: true,
  enableOnlyPushRedNews: true,
  httpProxy: '',
  httpProxyEnabled: false,
  enableAgent: true,
  disableBrowserSearch: false,
  akshareServiceUrl: 'http://127.0.0.1:9000',
  aDataServiceUrl: 'http://127.0.0.1:9100',
  defaultDataSource: 'eastmoney',
  crawlTimeOut: 60,
  kDays: 90,
  // ────────────────────────────────────────────────────────────────────────────
  // ↓ 透传字段（Pass-through）：界面不再暴露，但**必须完整往返**
  //
  // 后端 UpdateConfig 使用 GORM 的 Updates(map{...}) **显式写入每一个列**。
  // 只要请求里没带上某个字段，Go 侧就会拿到零值并把数据库中的原值覆盖掉。
  // 因此以下字段即使界面上不展示，也必须原样加载并原样回传，否则：
  //   一次普通保存就会静默关闭向量检索(embedding)、清空代理、抹掉回测与工具预算配置。
  // ────────────────────────────────────────────────────────────────────────────
  checkUpdate: false,
  updateChannel: '',
  windowWidth: 0,
  windowHeight: 0,
  promptPlazaApiBase: '',
  backtestAccuracyThreshold: 5.0,
  backtestVerifyDays: 2,
  backtestEnabled: true,
  embeddingEnabled: false,
  embeddingModelName: '',
  embeddingBaseUrl: '',
  embeddingApiKey: '',
  mcpTimeout: 8,
  toolRunBudget: 15,
  toolDailyBudget: 200,
  toolDailyCostUSD: 2.0
});

const openAiEnable = ref(true);
const defaultAiConfigId = ref<number | null>(null);
const aiPrompt = ref('');
const questionTemplate = ref('');

// 界面上仅暴露并允许编辑前 6 项（5 项核心身份字段 + 默认标记）；
// 其余为「透传调度字段」——不展示，但必须完整往返（原因同上方 formValue 的注释）。
export interface CleanAIConfig {
  ID: number;
  name: string;
  baseUrl: string;
  apiKey: string;
  modelName: string;
  thinking: boolean;
  // ↓ 透传字段（界面上不展示，保存时必须原样回传，否则会被后端清零）
  maxTokens: number;
  temperature: number;
  timeOut: number;
  httpProxy: string;
  httpProxyEnabled: boolean;
  sessionId: string;
  embeddingEnabled: boolean;
  embeddingModelName: string;
  embeddingBaseUrl: string;
  llmConcurrencyLimit: number;
  subAgentTimeout: number;
  rateLimitPerSecond: number;
  burst: number;
  dailyRequestQuota: number;
  dailyCostQuota: number;
}

/** 新建配置卡片时的确定性默认值（仅用于 ID<=0 的全新条目） */
function newAiConfigDefaults(): Omit<CleanAIConfig, 'ID'> {
  return {
    name: '新建 AI 模型配置',
    baseUrl: 'https://api.deepseek.com/v1',
    apiKey: '',
    modelName: 'deepseek-chat',
    thinking: false,
    maxTokens: 8192,
    temperature: 0.7,
    timeOut: 300,
    httpProxy: '',
    httpProxyEnabled: false,
    sessionId: '',
    embeddingEnabled: false,
    embeddingModelName: '',
    embeddingBaseUrl: '',
    llmConcurrencyLimit: 2,
    subAgentTimeout: 20,
    rateLimitPerSecond: 0,
    burst: 0,
    dailyRequestQuota: 0,
    dailyCostQuota: 0
  };
}

/**
 * 将后端返回的任意形态 AI 配置规范化为完整的 CleanAIConfig。
 * 关键：**保留所有后端返回的字段原值**，只对缺失项补默认值。
 * 若此处只挑 6 个字段，保存时其余列就会被后端 Updates(map) 清零。
 */
function normalizeAiConfig(c: any): CleanAIConfig {
  const d = newAiConfigDefaults();
  return {
    ID: Number(c?.ID ?? c?.id ?? 0) || 0,
    name: c?.name ?? d.name,
    baseUrl: c?.baseUrl ?? d.baseUrl,
    apiKey: c?.apiKey ?? '',
    modelName: c?.modelName ?? d.modelName,
    thinking: Boolean(c?.thinking),
    maxTokens: Number(c?.maxTokens) > 0 ? Number(c.maxTokens) : d.maxTokens,
    temperature: Number(c?.temperature) > 0 ? Number(c.temperature) : d.temperature,
    timeOut: Number(c?.timeOut) > 0 ? Number(c.timeOut) : d.timeOut,
    httpProxy: c?.httpProxy ?? '',
    httpProxyEnabled: Boolean(c?.httpProxyEnabled),
    sessionId: c?.sessionId ?? '',
    embeddingEnabled: Boolean(c?.embeddingEnabled),
    embeddingModelName: c?.embeddingModelName ?? '',
    embeddingBaseUrl: c?.embeddingBaseUrl ?? '',
    llmConcurrencyLimit: Number(c?.llmConcurrencyLimit) > 0 ? Number(c.llmConcurrencyLimit) : d.llmConcurrencyLimit,
    subAgentTimeout: Number(c?.subAgentTimeout) > 0 ? Number(c.subAgentTimeout) : d.subAgentTimeout,
    rateLimitPerSecond: Number(c?.rateLimitPerSecond) || 0,
    burst: Number(c?.burst) || 0,
    dailyRequestQuota: Number(c?.dailyRequestQuota) || 0,
    dailyCostQuota: Number(c?.dailyCostQuota) || 0
  };
}

const aiConfigs = ref<CleanAIConfig[]>([]);

// 加载成功标记：仅当确实从后端拿到过配置时，才允许把（可能为空的）列表写回数据库。
// 否则一旦加载失败而用户在其它页签点了保存，空列表会触发后端 "DELETE FROM ai_config"，
// 导致全部模型配置被静默清空。
const settingsLoaded = ref(false);
const aiConfigsLoaded = ref(false);

// 控制 API Key 的明文显示切换
const showApiKeyMap = reactive<Record<number, boolean>>({});
function toggleShowApiKey(index: number) {
  showApiKeyMap[index] = !showApiKeyMap[index];
}

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

// 主题切换即时响应
watch(() => formValue.value.darkTheme, (isDark) => {
  if (typeof document !== 'undefined') {
    if (isDark) {
      document.documentElement.setAttribute('theme-mode', 'dark');
      document.documentElement.classList.add('dark');
      document.body.classList.add('is-dark');
    } else {
      document.documentElement.setAttribute('theme-mode', 'light');
      document.documentElement.classList.remove('dark');
      document.body.classList.remove('is-dark');
    }
  }
});

// ==============================================================================
// 3. Prompt 版本化预设与实时语法 Linter (Prompt Hub & Linter)
// ==============================================================================
const SYSTEM_PROMPT_PRESETS = {
  quant_mentor: `你是一位拥有20年证券实盘经验的买方量化研报总监与投资导师。坚持「风控第一、客观证据穿透、知行合一」原则。
在分析个股、行业或市场时，必须直接依据真实客观数据，严禁无根据臆测：
1. 【买方业务全景】：穿透前三大主营业务真实供需格局，核验行业商业化阶段，排查产品毛利率与技术替代风险。
2. 【财务排雷审计】：严谨核实近3年真实 ROE、毛利率趋势、扣非净利润与经营现金流匹配度，排除商誉减值与应收账款踩雷隐患。
3. 【筹码博弈与资金】：穿透前十大股东持股变动与机构席位动向，排查控股股东股权质押平仓风险及限售解禁时间表。
4. 【客观证据天梯】：严格引用第一级（法定公告与报表）、第二级（行业财务与研报）真实事实，承认研究缺口，严禁编造任何伪量化综合打分！`,

  antirisk: `你是由买方合规与风控投委会赋能的严谨风控审查专家。
你的首要职责是为投资人识别风险底线与逻辑证伪边界：
1. 【大盘与均线压制风控】：若标的现价运行在 MA20 均线下方，一律实行防守观望策略，严禁给出逢低买入或激进加仓结论！
2. 【财务质量硬排雷】：重点排查存货周转天数异常拉长、应收账款显著超过营业收入、控股股东大比例质押等致命隐患。
3. 【逻辑证伪条件】：每只标的必须列出可被客观检验的 2~3 个逻辑证伪底线（例如：销售毛利率跌破35%则逻辑失效）。`,

  standard: `你是一个专业的证券量化投资分析助手。擅长结合宏观数据、技术指标、资金流向和行业基本面信息，为用户提供客观严谨的投资分析研判与逻辑推演。`
};

const QUESTION_TEMPLATE_PRESETS = {
  comprehensive: `请为 {{stockName}} (代码: {{stockCode}}) 执行全维度买方量化研判：
1. 【基本面与财务成色】：穿透前三季度营业收入、扣非净利润与经营性现金流成色，排查存货、应收与质押风险；
2. 【量化技术与均线形态】：结合现价与 MA5/MA20 均线偏离度，核查是否受均线压制，并给出基于 ATR 波动通道的上轨阻力与止损线；
3. 【证据天梯与研究缺口】：明确引用最新官方公告核实事实，坦承未公开之大客户份额或研发细节等研究缺口；
4. 【认知账本沉淀】：输出格式规范的买方认知假说，明确关键催化剂与逻辑证伪底线。`,

  antirisk: `请对 {{stockName}} (代码: {{stockCode}}) 进行极限排雷与合规风控体检：
1. 股权质押与解禁：核验控股股东质押比例与未来 3~12 个月大额限售股解禁压力；
2. 现金流与盈利成色：核验营业收入与经营性现金流净额匹配度，排查虚增利润嫌疑；
3. 均线趋势防守：分析 MA20 与中长期趋势线支撑，制定严格的防守止损位。`,

  simple: `请输出 {{stockName}} (代码: {{stockCode}}) 的全景投资分析速览与量化风控评级。`
};

function applySystemPromptPreset(key: keyof typeof SYSTEM_PROMPT_PRESETS) {
  if (SYSTEM_PROMPT_PRESETS[key]) {
    aiPrompt.value = SYSTEM_PROMPT_PRESETS[key];
    message.success('已应用系统提示词预设，保存后生效');
  }
}

function applyQuestionTemplatePreset(key: keyof typeof QUESTION_TEMPLATE_PRESETS) {
  if (QUESTION_TEMPLATE_PRESETS[key]) {
    questionTemplate.value = QUESTION_TEMPLATE_PRESETS[key];
    message.success('已应用个股诊断模板预设，保存后生效');
  }
}

// 实时语法 Linter：检查模板是否包含 {{stockCode}} 与 {{stockName}}
const templateLinterStatus = computed(() => {
  const t = questionTemplate.value || '';
  const hasCode = t.includes('{{stockCode}}');
  const hasName = t.includes('{{stockName}}');
  if (hasCode && hasName) {
    return {
      type: 'success' as const,
      text: '✅ 模板语法校验通过：已包含 {{stockCode}} 与 {{stockName}} 占位符',
      valid: true
    };
  }
  if (!hasCode && !hasName) {
    return {
      type: 'error' as const,
      text: '❌ 语法严重告警：缺失必备占位符 {{stockCode}} 与 {{stockName}}，诊断无法获取标的参数！',
      valid: false
    };
  }
  if (!hasCode) {
    return {
      type: 'warning' as const,
      text: '⚠️ 语法告警：缺少必备占位符 {{stockCode}}，后端无法确定股票代码！',
      valid: false
    };
  }
  return {
    type: 'warning' as const,
    text: '⚠️ 语法提示：建议补充占位符 {{stockName}} 以在研报中展示股票名称',
    valid: true
  };
});

// ==============================================================================
// 4. Dirty Check 与快照机制 (Change Tracking)
// ==============================================================================
const originalFormSnapshot = ref<string>('');

function getFullFormSnapshot(): string {
  return JSON.stringify({
    form: formValue.value,
    openAiEnable: openAiEnable.value,
    defaultAiConfigId: defaultAiConfigId.value,
    aiPrompt: aiPrompt.value,
    questionTemplate: questionTemplate.value,
    // 展开全部字段参与脏检查：透传调度字段若被后端变更也应被感知
    aiConfigs: aiConfigs.value.map(c => ({ ...c }))
  });
}

function markClean() {
  originalFormSnapshot.value = getFullFormSnapshot();
}

const isFormDirty = computed(() => {
  if (!originalFormSnapshot.value) return false;
  return originalFormSnapshot.value !== getFullFormSnapshot();
});

// ==============================================================================
// 5. AI 模型矩阵管理 (Model Registry CRUD & Tests)
// ==============================================================================
/** aiConfigMeta 以数组下标为键，删除元素后必须整体重建，
 *  否则“拉取列表/测试连通性”的 loading 与选项会被错位挂到另一张卡片上。 */
function reindexConfigMeta() {
  const snapshot: ConfigMeta[] = aiConfigs.value.map((_, i) => aiConfigMeta[i] ?? { loadingModels: false, testing: false, modelOptions: [] });
  for (const k of Object.keys(aiConfigMeta)) delete aiConfigMeta[Number(k)];
  snapshot.forEach((m, i) => { aiConfigMeta[i] = m; });
  for (const k of Object.keys(showApiKeyMap)) delete showApiKeyMap[Number(k)];
}

// “设为默认”在未保存（ID<=0）卡片上的待定意图：
// 新建卡片在保存前没有数据库 ID，无法直接写入 aiConfigId；
// 这里记住下标，保存并重新加载后按位置解析出真实 ID 再落库。
const pendingDefaultModelIndex = ref<number | null>(null);

async function flushPendingDefaultModel() {
  const idx = pendingDefaultModelIndex.value;
  if (idx === null) return;
  pendingDefaultModelIndex.value = null;
  const target = aiConfigs.value[idx];
  if (!target || !target.ID) return;
  if (defaultAiConfigId.value === target.ID) return;
  defaultAiConfigId.value = target.ID;
  // 必须提交**完整**配置：绝不能发空 aiConfigs，
  // 否则后端 updateAiConfigsTx 会执行 "DELETE FROM ai_config" 清空所有模型配置。
  try {
    await UpdateConfig(buildSavePayload());
    await loadSettings();
  } catch {
    /* 失败不阻断，下次保存会再写一次 */
  }
}

function addAiConfig() {
  const newConfig: CleanAIConfig = { ID: 0, ...newAiConfigDefaults() };
  aiConfigs.value.push(newConfig);
  ensureConfigMeta(aiConfigs.value.length - 1);
  message.info('已新增模型配置卡片，请填入密钥并保存');
}

function removeAiConfig(index: number) {
  dialog.warning({
    title: '确认删除配置',
    content: `确定要删除「${aiConfigs.value[index]?.name || '未命名配置'}」吗？`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: () => {
      const removed = aiConfigs.value[index];
      aiConfigs.value.splice(index, 1);
      reindexConfigMeta();
      // 如果删掉的正是当前默认模型，则清除默认标记（避免指向已不存在的 ID）
      if (removed && defaultAiConfigId.value === removed.ID) {
        defaultAiConfigId.value = null;
      }
      if (pendingDefaultModelIndex.value !== null) {
        if (pendingDefaultModelIndex.value === index) pendingDefaultModelIndex.value = null;
        else if (pendingDefaultModelIndex.value > index) pendingDefaultModelIndex.value -= 1;
      }
      message.success('已移除该模型配置，点击「保存设置」后生效');
    }
  });
}

function copyAiConfig(index: number) {
  const source = aiConfigs.value[index];
  if (!source) return;
  // 完整复制（含透传调度字段），仅重置 ID 与名称
  const copy: CleanAIConfig = {
    ...(JSON.parse(JSON.stringify(source)) as CleanAIConfig),
    ID: 0,
    name: `${source.name} (副本)`
  };
  aiConfigs.value.push(copy);
  ensureConfigMeta(aiConfigs.value.length - 1);
  message.success('已复制配置卡片，点击「保存设置」后生效');
}

function setDefaultModel(cfg: CleanAIConfig, index: number) {
  if (!cfg.ID || cfg.ID <= 0) {
    // 未保存的新卡片：记下位置，保存后自动生效
    pendingDefaultModelIndex.value = index;
    message.info(`已标记 [${cfg.name}] 为默认模型，点击「保存设置」后生效`);
    return;
  }
  defaultAiConfigId.value = cfg.ID;
  message.success(`已设置 [${cfg.name}] 为全局默认模型`);
}

async function fetchAiModels(aiConfig: CleanAIConfig, index: number, opts: { silent?: boolean } = {}) {
  const meta = ensureConfigMeta(index);
  const baseUrl = (aiConfig.baseUrl || '').trim();
  const apiKey = (aiConfig.apiKey || '').trim() || 'none';
  if (!baseUrl) {
    if (!opts.silent) message.warning('请先填写接口地址 (Base URL)');
    return;
  }
  meta.loadingModels = true;
  try {
    const list: string[] = await FetchAiModels(baseUrl, apiKey);
    if (list && list.length > 0) {
      meta.modelOptions = list.map(m => ({ label: m, value: m }));
      if (!opts.silent) message.success(`成功拉取 ${list.length} 个可用模型标识`);
      if (!aiConfig.modelName && list[0]) {
        aiConfig.modelName = list[0];
      }
    } else {
      if (!opts.silent) message.warning('未获取到模型列表，可手动输入模型名称');
    }
  } catch (err: any) {
    if (!opts.silent) message.error('获取模型失败: ' + (err?.message || err));
  } finally {
    meta.loadingModels = false;
  }
}

async function testAiConfig(aiConfig: CleanAIConfig, index: number) {
  const meta = ensureConfigMeta(index);
  const baseUrl = (aiConfig.baseUrl || '').trim();
  const apiKey = (aiConfig.apiKey || '').trim() || 'none';
  const modelName = (aiConfig.modelName || '').trim();

  if (!baseUrl) {
    message.warning('请先填写接口地址 (Base URL)');
    return;
  }
  if (!baseUrl.startsWith('http://') && !baseUrl.startsWith('https://')) {
    message.error('接口地址格式错误，必须以 http:// 或 https:// 开头');
    return;
  }
  if (!modelName) {
    message.warning('请先填写或选择模型标识 (Model Identifier)');
    return;
  }
  meta.testing = true;
  const startTime = Date.now();
  try {
    const errMsg = await TestAiConnection(baseUrl, apiKey, modelName);
    const latency = Date.now() - startTime;
    if (errMsg) {
      message.error(`连接测试失败: ${errMsg}`);
    } else {
      message.success(`连接测试成功！模型 [${modelName}] 响应正常 (耗时: ${latency} ms)`);
    }
  } catch (err: any) {
    message.error('测试异常: ' + (err?.message || err));
  } finally {
    meta.testing = false;
  }
}

// 预设模型模板库快速载入
const wizardVisible = ref(false);
const wizardTemplates = ref<any[]>([]);

async function openModelTemplateWizard() {
  try {
    const tpls = await GetAiConfigTemplates();
    if (tpls && tpls.length > 0) {
      wizardTemplates.value = tpls;
    } else {
      throw new Error('empty templates');
    }
  } catch (_e) {
    wizardTemplates.value = [
      {
        label: 'DeepSeek 官方直连',
        description: '深度求索官方 API，支持 deepseek-chat 与 deepseek-reasoner',
        baseUrl: 'https://api.deepseek.com/v1',
        modelName: 'deepseek-chat',
        apiKeyRequired: true
      },
      {
        label: 'MiniMax 官方直连 (买方投研推荐)',
        description: 'MiniMax M3 / Text-01，支持超长上下文与高吞吐分析',
        baseUrl: 'https://api.minimax.chat/v1',
        modelName: 'MiniMax-M3',
        apiKeyRequired: true
      },
      {
        label: 'NVIDIA NIM 免费企业微服务平台',
        description: '英伟达托管企业级微服务，支持 GLM-5.3-Flash / Llama-3.3-70B',
        baseUrl: 'https://integrate.api.nvidia.com/v1',
        modelName: 'z-ai/glm-5.3-flash',
        apiKeyRequired: true
      },
      {
        label: '硅基流动 SiliconFlow 开源模型聚合',
        description: '极速推理加速平台，聚合 DeepSeek / Qwen 等优秀开源权重',
        baseUrl: 'https://api.siliconflow.cn/v1',
        modelName: 'deepseek-ai/DeepSeek-V3',
        apiKeyRequired: true
      },
      {
        label: 'Ollama 本地大模型部署',
        description: '本地私有化部署 (零网络消耗，数据 100% 物理隔离)',
        baseUrl: 'http://127.0.0.1:11434/v1',
        modelName: 'qwen2.5:7b',
        apiKeyRequired: false
      }
    ];
  }
  wizardVisible.value = true;
}

function applyModelTemplate(tpl: any) {
  aiConfigs.value.push({
    ID: 0,
    name: tpl.label.split(' ')[0],
    baseUrl: tpl.baseUrl,
    apiKey: tpl.apiKeyRequired ? '' : 'none',
    modelName: tpl.modelName,
    thinking: tpl.modelName.includes('reasoner') || tpl.modelName.includes('r1')
  });
  ensureConfigMeta(aiConfigs.value.length - 1);
  wizardVisible.value = false;
  message.success(`已快速载入 [${tpl.label}] 预设`);
}

// ==============================================================================
// 6. 数据维护与快照操作 (Maintenance & Snapshot)
// ==============================================================================
const isCleaning = ref(false);
const isSnapshotting = ref(false);

async function handleCleanup(cleanType: string, label: string) {
  dialog.warning({
    title: '确认清理缓存',
    content: `确定要清理「${label}」吗？清理后系统将在下次请求时从源站重新拉取。`,
    positiveText: '确认清理',
    negativeText: '取消',
    onPositiveClick: async () => {
      isCleaning.value = true;
      try {
        const res = await CleanupData(cleanType);
        message.success(res || '清理缓存完成');
      } catch (err: any) {
        message.error('清理失败: ' + (err?.message || err));
      } finally {
        isCleaning.value = false;
      }
    }
  });
}

async function handleCreateSnapshot() {
  isSnapshotting.value = true;
  try {
    const res = await CreateDatabaseSnapshot();
    message.success(res || '数据库物理快照创建成功！');
  } catch (err: any) {
    message.error('快照创建失败: ' + (err?.message || err));
  } finally {
    isSnapshotting.value = false;
  }
}

// 钉钉测试推送
const testingDing = ref(false);
async function testDingPush() {
  const robotUrl = formValue.value.dingPush.dingRobot?.trim();
  if (!robotUrl) {
    message.warning('请先填写钉钉自定义机器人的 Webhook 地址');
    return;
  }
  testingDing.value = true;
  try {
    const res = await SendDingDingMessageByType(
      '【Blades Stock v3.0.0 测试推送】\n您的量化投研中台与钉钉消息推送链路已连通成功！',
      '000001',
      0
    );
    message.success(res || '已触发钉钉推送请求');
  } catch (err: any) {
    message.error('推送失败: ' + (err?.message || err));
  } finally {
    testingDing.value = false;
  }
}

// ==============================================================================
// 7. 配置加载、导出、导入与保存 (Load, Export, Import, Save)
// ==============================================================================
/**
 * 把后端下发的配置快照写入表单状态。
 *
 * ⚠️ 契约说明（v3.0.0 修复）：后端 `SettingConfig` 是**匿名嵌入 `*Settings`** 的结构体，
 * 序列化后字段直接位于**顶层**（扁平），即：
 *     { "ID":1, "prompt":"...", "aiConfigs":[...], "embeddingEnabled":true, ... }
 * 而**不是** { "settings": { ... } }。
 *
 * 历史缺陷：前端误按嵌套形态读取 `res.settings` 并按其回写，导致：
 *   1) 加载时 `res.settings` 为 undefined → 整个基础设置表单无法回显（全部停留在默认值）；
 *   2) 保存时提交 { settings: {...} } → 后端 `s.Settings == nil` → 直接返回「保存失败: 配置数据为空」。
 * 这里同时兼容扁平（当前契约）与嵌套（投诉前的旧版导出格式），避免再被形态问题打断。
 */
function applySettingsSnapshot(s: any) {
  if (!s) return;
  formValue.value.ID = s.ID || 1;
  formValue.value.tushareToken = s.tushareToken ?? '';
  formValue.value.emApiKey = s.emApiKey ?? '';
  formValue.value.tickFlowAPIKey = s.tickFlowAPIKey ?? '';
  formValue.value.qgqpBId = s.qgqpBId ?? '';
  formValue.value.dingPush.enable = Boolean(s.dingPushEnable);
  formValue.value.dingPush.dingRobot = s.dingRobot ?? '';
  formValue.value.localPush.enable = Boolean(s.localPushEnable);
  formValue.value.updateBasicInfoOnStart = Boolean(s.updateBasicInfoOnStart);
  formValue.value.refreshInterval = Number(s.refreshInterval) > 0 ? Number(s.refreshInterval) : 3;
  formValue.value.browserPath = s.browserPath ?? '';
  formValue.value.enableNews = Boolean(s.enableNews);
  formValue.value.darkTheme = s.darkTheme !== false;
  formValue.value.enableFund = Boolean(s.enableFund);
  formValue.value.enablePushNews = s.enablePushNews !== false;
  formValue.value.enableOnlyPushRedNews = s.enableOnlyPushRedNews !== false;
  formValue.value.httpProxy = s.httpProxy ?? '';
  formValue.value.httpProxyEnabled = Boolean(s.httpProxyEnabled);
  formValue.value.enableAgent = s.enableAgent !== false;
  formValue.value.disableBrowserSearch = Boolean(s.disableBrowserSearch);
  formValue.value.akshareServiceUrl = s.akshareServiceUrl || 'http://127.0.0.1:9000';
  formValue.value.aDataServiceUrl = s.aDataServiceUrl || 'http://127.0.0.1:9100';
  formValue.value.defaultDataSource = s.defaultDataSource || 'eastmoney';
  formValue.value.crawlTimeOut = Number(s.crawlTimeOut) > 0 ? Number(s.crawlTimeOut) : 60;
  formValue.value.kDays = Number(s.kDays) > 0 ? Number(s.kDays) : 90;

  // ── 透传字段：必须原样接住，否则保存时会被后端 Updates(map) 清零 ──
  formValue.value.checkUpdate = Boolean(s.checkUpdate);
  formValue.value.updateChannel = s.updateChannel ?? '';
  formValue.value.windowWidth = Number(s.windowWidth) || 0;
  formValue.value.windowHeight = Number(s.windowHeight) || 0;
  formValue.value.promptPlazaApiBase = s.promptPlazaApiBase ?? '';
  formValue.value.backtestAccuracyThreshold =
    Number(s.backtestAccuracyThreshold) > 0 ? Number(s.backtestAccuracyThreshold) : 5.0;
  formValue.value.backtestVerifyDays = Number(s.backtestVerifyDays) > 0 ? Number(s.backtestVerifyDays) : 2;
  formValue.value.backtestEnabled = s.backtestEnabled !== false;
  formValue.value.embeddingEnabled = Boolean(s.embeddingEnabled);
  formValue.value.embeddingModelName = s.embeddingModelName ?? '';
  formValue.value.embeddingBaseUrl = s.embeddingBaseUrl ?? '';
  formValue.value.embeddingApiKey = s.embeddingApiKey ?? '';
  formValue.value.mcpTimeout = Number(s.mcpTimeout) > 0 ? Number(s.mcpTimeout) : 8;
  formValue.value.toolRunBudget = Number(s.toolRunBudget) > 0 ? Number(s.toolRunBudget) : 15;
  formValue.value.toolDailyBudget = Number(s.toolDailyBudget) > 0 ? Number(s.toolDailyBudget) : 200;
  formValue.value.toolDailyCostUSD =
    Number(s.toolDailyCostUSD) > 0 ? Number(s.toolDailyCostUSD) : 2.0;

  openAiEnable.value = Boolean(s.openAiEnable);
  defaultAiConfigId.value = Number(s.aiConfigId) > 0 ? Number(s.aiConfigId) : null;
  aiPrompt.value = s.prompt || SYSTEM_PROMPT_PRESETS.quant_mentor;
  questionTemplate.value = s.questionTemplate || QUESTION_TEMPLATE_PRESETS.comprehensive;
}

async function loadSettings() {
  try {
    const res: any = await GetConfig();
    // 兼容扁平（当前契约）与嵌套（旧版导出）两种形态
    const rawSettings = res?.settings ?? res;
    if (rawSettings) {
      applySettingsSnapshot(rawSettings);
      settingsLoaded.value = true;
    } else {
      settingsLoaded.value = false;
    }

    if (Array.isArray(res?.aiConfigs)) {
      aiConfigs.value = res.aiConfigs.map(normalizeAiConfig);
      aiConfigsLoaded.value = true;
    } else if (Array.isArray(rawSettings?.aiConfigs)) {
      aiConfigs.value = rawSettings.aiConfigs.map(normalizeAiConfig);
      aiConfigsLoaded.value = true;
    } else {
      aiConfigsLoaded.value = false;
    }

    markClean();
  } catch (err: any) {
    settingsLoaded.value = false;
    aiConfigsLoaded.value = false;
    message.error('加载系统配置失败: ' + (err?.message || err));
  }
}


// 导出脱敏配置文件 (完全杜绝明文密钥导出泄露)
async function handleExportConfig() {
  try {
    const raw = await ExportConfig();
    const blob = new Blob([raw], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `blades-stock-config-v3.0.0-${new Date().toISOString().slice(0, 10)}.json`;
    link.click();
    URL.revokeObjectURL(url);
    message.success('已导出脱敏系统配置 (商业密钥已掩码)');
  } catch (e: any) {
    message.error('导出配置失败: ' + (e?.message || e));
  }
}

// 导入配置
function handleImportConfig() {
  const input = document.createElement('input');
  input.type = 'file';
  input.accept = '.json';
  input.onchange = async (e: any) => {
    const file = e.target.files?.[0];
    if (!file) return;
    try {
      const text = await file.text();
      const parsed = JSON.parse(text);
      // 与 GetConfig/ExportConfig 保持一致：优先扁平形态，兼容旧版嵌套形态
      const importedSettings = parsed?.settings ?? parsed;
      if (importedSettings && typeof importedSettings === 'object') {
        applySettingsSnapshot(importedSettings);
        settingsLoaded.value = true;
      }
      const importedConfigs = Array.isArray(parsed?.aiConfigs)
        ? parsed.aiConfigs
        : Array.isArray(importedSettings?.aiConfigs)
          ? importedSettings.aiConfigs
          : null;
      if (importedConfigs) {
        aiConfigs.value = importedConfigs.map(normalizeAiConfig);
        aiConfigsLoaded.value = true;
        reindexConfigMeta();
      }
      message.success('导入配置预览就绪，点击「保存设置」生效');
    } catch (err: any) {
      message.error('解析配置文件失败: ' + (err?.message || err));
    }
  };
  input.click();
}

/**
 * 构建 UpdateConfig 的提交载荷。
 *
 * ── 形态契约（v3.0.0 修复的关键）────────────────────────────────────────────
 * 后端 `SettingConfig` 是**匿名嵌入** `*Settings` 的结构体，其 JSON 形态是**扁平**的：
 *     { "ID":1, "prompt":"...", "aiConfigs":[ ... ] }
 * 而**不是** `{ settings: { ... }, aiConfigs: [...] }`。
 * 提交嵌套形态会使后端 `s.Settings == nil`，直接返回「保存失败: 配置数据为空」。
 *
 * ── 字段完整性 ────────────────────────────────────────────────────────
 * 后端 `UpdateConfig` / `updateAiConfigsTx` 均使用 GORM `Updates(map{...})`
 * **显式写入每一列**。因此必须展开**全部**已加载字段（含界面上不展示的
 * embedding / backtest / mcp / tool 预算 / update / window 等），
 * 以及每个模型配置的**全部**字段（含 maxTokens / temperature / httpProxy /
 * 配额等），否则缺失列会被静默清零。
 */
function buildSavePayload(): any {
  return {
    ...formValue.value,
    // 展开后校正与表单结构不一致的字段（表内是嵌套对象，后端要扁平布尔）
    localPushEnable: Boolean(formValue.value.localPush?.enable),
    dingPushEnable: Boolean(formValue.value.dingPush?.enable),
    dingRobot: formValue.value.dingPush?.dingRobot || '',
    openAiEnable: openAiEnable.value,
    aiConfigId: defaultAiConfigId.value || 0,
    prompt: aiPrompt.value,
    questionTemplate: questionTemplate.value,
    // aiConfigs：完整回传每一个字段（包括界面不展示的调度字段）
    aiConfigs: aiConfigs.value.map(c => ({
      ...c,
      ID: c.ID || 0,
      name: (c.name || '').trim(),
      baseUrl: (c.baseUrl || '').trim(),
      apiKey: (c.apiKey || '').trim(),
      modelName: (c.modelName || '').trim(),
      thinking: Boolean(c.thinking),
      httpProxyEnabled: Boolean(c.httpProxyEnabled),
      embeddingEnabled: Boolean(c.embeddingEnabled)
    }))
  };
}

// 严格表单前置校验与保存
async function saveSettings() {
  // 1. 刷新频率校验
  if (formValue.value.refreshInterval < 2) {
    message.error('行情刷新频率不得低于 2 秒，推荐 3~5 秒以防止查询拥堵');
    activeTab.value = 'system';
    return;
  }

  // 1.5 加载完整性门禁（防止把空/残缺列表写回，从而误删全部模型配置）
  //     后端 updateAiConfigsTx 对空列表执行 "DELETE FROM ai_config"，
  //     对残缺列表执行 "DELETE ... WHERE id NOT IN (...)"。
  //     若本次会话从未成功加载到配置，则绝不允许提交。
  if (!aiConfigsLoaded.value) {
    message.error('尚未成功加载模型配置，为避免误删已保存的配置，本次保存已被阻止，请先刷新页面或重试加载。');
    activeTab.value = 'ai';
    return;
  }
  if (!settingsLoaded.value) {
    message.error('尚未成功加载系统配置，为避免用默认值覆盖现有配置，本次保存已被阻止，请先刷新页面或重试加载。');
    return;
  }

  // 2. AI 模型校验
  if (openAiEnable.value && aiConfigs.value.length > 0) {
    for (let i = 0; i < aiConfigs.value.length; i++) {
      const c = aiConfigs.value[i];
      if (!c.name || !c.name.trim()) {
        message.error(`模型配置 #${i + 1} 未填写配置别名`);
        activeTab.value = 'ai';
        return;
      }
      if (!c.baseUrl || !c.baseUrl.trim()) {
        message.error(`模型配置 #${i + 1} (${c.name}) 未填写接口地址 (Base URL)`);
        activeTab.value = 'ai';
        return;
      }
      if (!c.baseUrl.startsWith('http://') && !c.baseUrl.startsWith('https://')) {
        message.error(`模型配置 #${i + 1} (${c.name}) 接口地址必须以 http:// 或 https:// 开头`);
        activeTab.value = 'ai';
        return;
      }
      if (!c.modelName || !c.modelName.trim()) {
        message.error(`模型配置 #${i + 1} (${c.name}) 未填写模型标识 (Model Identifier)`);
        activeTab.value = 'ai';
        return;
      }
    }
  }

  isSaving.value = true;
  try {
    const res = await UpdateConfig(buildSavePayload());
    // 后端成功时返回「保存成功！」，失败时返回以「保存失败」开头的字符串
    if (typeof res === 'string' && res.includes('失败')) {
      message.error(res);
      return;
    }
    message.success('系统设置已成功保存并立即生效！');
    markClean();
    await loadSettings();
    // 保存后处理“设为默认”在未保存卡片上的待定意图
    await flushPendingDefaultModel();
  } catch (err: any) {
    message.error('保存设置异常: ' + (err?.message || err));
  } finally {
    isSaving.value = false;
  }
}

function handleDiscardChanges() {
  dialog.info({
    title: '放弃本次修改',
    content: '确定要放弃尚未保存的更改并恢复至当前加载的状态吗？',
    positiveText: '确认放弃',
    negativeText: '继续编辑',
    onPositiveClick: () => {
      loadSettings();
      message.info('已恢复至上次保存的配置');
    }
  });
}

function handleResetDefaults() {
  dialog.warning({
    title: '恢复出厂默认值',
    content: '确定要将所有基础参数与 Prompt 恢复为系统官方预设值吗？（已保存的私有模型配置将被保留）',
    positiveText: '确认恢复',
    negativeText: '取消',
    onPositiveClick: () => {
      formValue.value.refreshInterval = 3;
      formValue.value.updateBasicInfoOnStart = false;
      formValue.value.enablePushNews = true;
      formValue.value.enableOnlyPushRedNews = true;
      formValue.value.defaultDataSource = 'eastmoney';
      formValue.value.akshareServiceUrl = 'http://127.0.0.1:9000';
      formValue.value.aDataServiceUrl = 'http://127.0.0.1:9100';
      aiPrompt.value = SYSTEM_PROMPT_PRESETS.quant_mentor;
      questionTemplate.value = QUESTION_TEMPLATE_PRESETS.comprehensive;
      message.success('已恢复出厂默认预设，请点击「保存设置」生效');
    }
  });
}

// 键盘快捷键监听 (Ctrl+S / Cmd+S)
function handleKeyDown(e: KeyboardEvent) {
  if ((e.ctrlKey || e.metaKey) && e.key === 's') {
    e.preventDefault();
    saveSettings();
  }
}

onMounted(() => {
  loadSettings();
  checkServerPing();
  window.addEventListener('keydown', handleKeyDown);
});

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleKeyDown);
});
</script>

<template>
  <div class="settings-container">
    <!-- 顶部状态与快速导航栏 -->
    <div class="settings-header-bar">
      <div class="header-left">
        <n-button quaternary circle size="medium" class="back-btn" @click="goBackToMain">
          <template #icon>
            <n-icon :component="ArrowBackOutline" />
          </template>
        </n-button>
        <div class="title-block">
          <h2 class="main-title">控制中枢 · {{ activeTabTitle }}</h2>
          <div class="meta-links">
            <span class="link-label">工作区直达：</span>
            <span class="nav-link" @click="navigateTo('stock')">股票看板</span>
            <span class="nav-sep">/</span>
            <span class="nav-link" @click="navigateTo('market')">宏观智能</span>
            <span class="nav-sep">/</span>
            <span class="nav-link" @click="navigateTo('agent')">Agent助手</span>
            <span class="nav-sep">/</span>
            <span class="nav-link" @click="navigateTo('researchIndex')">投研账本</span>
            <span class="nav-sep">/</span>
            <span class="nav-link" @click="navigateTo('fund')">基金行情</span>
          </div>
        </div>
      </div>

      <div class="header-right">
        <!-- 运行时遥测指标 -->
        <n-tag type="info" round size="small" class="status-tag">
          版本 v3.0.0
        </n-tag>
        <n-tag :type="isWebMode ? 'warning' : 'success'" round size="small" class="status-tag">
          {{ isWebMode ? 'Web SaaS' : 'Desktop GUI' }}
        </n-tag>
        <n-tag :type="pingLatency !== null && pingLatency >= 0 ? 'success' : 'error'" round size="small" class="status-tag">
          {{ pingLatency !== null && pingLatency >= 0 ? `${pingLatency}ms` : '探测中' }}
        </n-tag>

        <n-divider vertical style="height: 20px; margin: 0 8px;" />

        <!-- 顶部操作按钮 -->
        <n-button quaternary size="small" @click="handleResetDefaults">恢复默认</n-button>
        <n-button quaternary size="small" @click="handleExportConfig">导出JSON</n-button>
        <n-button quaternary size="small" @click="handleImportConfig">导入配置</n-button>
        <n-button v-if="isFormDirty" quaternary size="small" type="warning" @click="handleDiscardChanges">放弃修改</n-button>
        <n-button type="primary" size="small" :loading="isSaving" @click="saveSettings">
          💾 保存设置 (Ctrl+S)
        </n-button>
      </div>
    </div>

    <!-- 标签页面切换中枢 -->
    <n-tabs v-model:value="activeTab" type="line" animated class="settings-tabs-root">
      <!-- =================================================================== -->
      <!-- TAB 1: 🖥️ 基础与运行环境 -->
      <!-- =================================================================== -->
      <n-tab-pane name="system" tab="🖥️ 基础与运行环境">
        <div class="tab-content-wrapper">
          <!-- 运行时可观测看板 -->
          <n-card title="运行时状态与安全防线" size="small" class="section-card">
            <n-grid cols="1 s:2 m:4" :x-gap="16" :y-gap="12">
              <n-gi>
                <div class="stat-tile">
                  <div class="stat-label">系统核心版本</div>
                  <div class="stat-value text-accent">v3.0.0 生产发布版</div>
                  <div class="stat-desc">Single Source of Truth 纯Go同构</div>
                </div>
              </n-gi>
              <n-gi>
                <div class="stat-tile">
                  <div class="stat-label">部署架构模式</div>
                  <div class="stat-value">{{ isWebMode ? 'Cloud-Native Docker' : 'Desktop GUI' }}</div>
                  <div class="stat-desc">免编译多端适配</div>
                </div>
              </n-gi>
              <n-gi>
                <div class="stat-tile">
                  <div class="stat-label">底层存储引擎</div>
                  <div class="stat-value text-success">SQLite WAL 就绪</div>
                  <div class="stat-desc">权威嵌入式库，无网络开销</div>
                </div>
              </n-gi>
              <n-gi>
                <div class="stat-tile">
                  <div class="stat-label">静默自动备份</div>
                  <div class="stat-value text-success">每日 03:00 轮换</div>
                  <div class="stat-desc">14 天物理快照滚动修剪</div>
                </div>
              </n-gi>
            </n-grid>

            <n-divider style="margin: 14px 0;" />

            <div class="auth-row-bar">
              <div class="auth-info-desc">
                <span class="badge-dot success"></span>
                <span>统一安全认证：当前公网环境已受管理员凭证 (<strong>2266762</strong>) 严密防护</span>
              </div>
              <n-space>
                <n-button size="tiny" quaternary @click="handleOpenAuthManage">修改口令凭据</n-button>
                <n-button v-if="isWebMode" size="tiny" quaternary type="error" @click="handleWebLogout">注销登录</n-button>
              </n-space>
            </div>
          </n-card>

          <!-- 用户偏好设置 -->
          <n-card title="用户交互与全局基础参数" size="small" class="section-card">
            <n-form :model="formValue" label-placement="top" size="small">
              <n-grid cols="1 m:2" :x-gap="24" :y-gap="8">
                <n-gi>
                  <n-form-item label="界面外观主题风格 (即时生效)">
                    <n-switch v-model:value="formValue.darkTheme">
                      <template #checked>深色沉浸主题 (Dark Mode)</template>
                      <template #unchecked>明亮浅色主题 (Light Mode)</template>
                    </n-switch>
                  </n-form-item>
                </n-gi>

                <n-gi>
                  <n-form-item label="行情全息轮询刷新频率 (秒)">
                    <n-space align="center" style="width: 100%;">
                      <n-input-number v-model:value="formValue.refreshInterval" :min="2" :max="30" style="width: 140px;" />
                      <span class="param-hint">推荐 3~5 秒。过小可能引发数据源限流。</span>
                    </n-space>
                  </n-form-item>
                </n-gi>

                <n-gi>
                  <n-form-item label="系统启动时行为">
                    <n-switch v-model:value="formValue.updateBasicInfoOnStart">
                      <template #checked>自动全量同步全市场标的字典</template>
                      <template #unchecked>跳过启动同步，加速启动</template>
                    </n-switch>
                  </n-form-item>
                </n-gi>

                <n-gi>
                  <n-form-item label="全局网络统一代理 (HTTP / Socks5)">
                    <n-space vertical style="width: 100%;">
                      <n-switch v-model:value="formValue.httpProxyEnabled">
                        <template #checked>已启用全局网络代理</template>
                        <template #unchecked>直连网络 (禁用代理)</template>
                      </n-switch>
                      <n-input
                        v-if="formValue.httpProxyEnabled"
                        v-model:value="formValue.httpProxy"
                        placeholder="http://127.0.0.1:7890 或 socks5://127.0.0.1:1080"
                      />
                    </n-space>
                  </n-form-item>
                </n-gi>
              </n-grid>
            </n-form>
          </n-card>

          <!-- 维护与快照中心 -->
          <n-card title="数据缓存维护与主动快照" size="small" class="section-card">
            <p class="section-desc">系统所有关键业务均自动执行落盘与轮换。以下操作供管理员在数据源更新异常或需紧急归档时一键执行：</p>
            <n-space style="margin-top: 10px;">
              <n-button size="small" :loading="isCleaning" @click="handleCleanup('cache', 'K线与快讯行情缓存')">
                🧹 清除应用数据缓存
              </n-button>
              <n-button size="small" :loading="isSnapshotting" type="info" @click="handleCreateSnapshot">
                📸 立即创建 SQLite 物理快照
              </n-button>
            </n-space>
          </n-card>
        </div>
      </n-tab-pane>

      <!-- =================================================================== -->
      <!-- TAB 2: 🤖 AI 模型注册表 -->
      <!-- =================================================================== -->
      <n-tab-pane name="ai" tab="🤖 AI模型注册表">
        <div class="tab-content-wrapper">
          <!-- 顶部总开关与预设载入栏 -->
          <div class="ai-toolbar-bar">
            <div class="ai-toggle-left">
              <n-switch v-model:value="openAiEnable" size="large">
                <template #checked>AI 投研分析与 Agent 引擎：已开启</template>
                <template #unchecked>AI 投研分析与 Agent 引擎：已关闭</template>
              </n-switch>
            </div>
            <div class="ai-toggle-right">
              <n-button quaternary size="small" type="info" @click="openModelTemplateWizard">
                ✨ 预设模板库快速载入
              </n-button>
              <n-button type="primary" size="small" @click="addAiConfig">
                + 新增模型配置
              </n-button>
            </div>
          </div>

          <p class="section-desc" style="margin-top: 4px;">
            遵循 stevia0829 零依赖架构原则，模型卡片仅保留 <strong>5项核心身份字段</strong>（名称、BaseURL、API Key、ModelName、Thinking）。其余调度参数由 Go 运行时确定性自动调优。
          </p>

          <!-- 模型卡片列表 -->
          <div v-if="aiConfigs.length === 0" class="empty-configs-box">
            <p>暂无配置的大模型。请点击右上角「+ 新增模型配置」或「预设模板库快速载入」。</p>
          </div>

          <div v-else class="model-cards-grid">
            <n-card
              v-for="(cfg, idx) in aiConfigs"
              :key="idx"
              size="small"
              class="model-item-card"
              :class="{ 'is-default-card': defaultAiConfigId === cfg.ID }"
            >
              <template #header>
                <div class="card-header-line">
                  <span class="card-title-text">#{{ idx + 1 }} {{ cfg.name || '未命名配置' }}</span>
                  <div class="card-header-tags">
                    <n-tag v-if="defaultAiConfigId === cfg.ID" type="success" size="tiny" round>
                      🎯 全局默认
                    </n-tag>
                    <n-tag v-if="cfg.thinking" type="info" size="tiny" round>
                      🧠 深度思考
                    </n-tag>
                  </div>
                </div>
              </template>

              <template #header-extra>
                <n-space :size="6">
                  <n-button
                    v-if="defaultAiConfigId !== cfg.ID"
                    quaternary
                    size="tiny"
                    type="primary"
                    @click="setDefaultModel(cfg, idx)"
                  >
                    设为默认
                  </n-button>
                  <n-button quaternary size="tiny" @click="copyAiConfig(idx)">复制</n-button>
                  <n-button quaternary size="tiny" type="error" @click="removeAiConfig(idx)">删除</n-button>
                </n-space>
              </template>

              <!-- 精简 5 大核心表单项 -->
              <n-form label-placement="top" size="small" class="model-card-form">
                <n-grid cols="1 m:2" :x-gap="16" :y-gap="8">
                  <!-- 1. 配置名称 -->
                  <n-gi>
                    <n-form-item label="配置别名 (用于界面切换)" required>
                      <n-input v-model:value="cfg.name" placeholder="例如：DeepSeek V3 / MiniMax M3" />
                    </n-form-item>
                  </n-gi>

                  <!-- 2. 模型标识 -->
                  <n-gi>
                    <n-form-item label="模型标识 (Model Identifier)" required>
                      <div class="model-name-group">
                        <n-input v-model:value="cfg.modelName" placeholder="deepseek-chat / MiniMax-M3" style="flex: 1;" />
                        <n-button
                          size="small"
                          :loading="aiConfigMeta[idx]?.loadingModels"
                          @click="fetchAiModels(cfg, idx)"
                        >
                          拉取列表
                        </n-button>
                      </div>
                    </n-form-item>
                  </n-gi>

                  <!-- 3. 接口地址 Base URL -->
                  <n-gi>
                    <n-form-item label="接口地址 (Base URL)" required>
                      <n-input v-model:value="cfg.baseUrl" placeholder="https://api.deepseek.com/v1" />
                    </n-form-item>
                  </n-gi>

                  <!-- 4. 访问密钥 API Key (掩码显示，安全切换) -->
                  <n-gi>
                    <n-form-item label="API 认证密钥 (输入空或保留掩码即不覆盖)">
                      <div class="api-key-group">
                        <n-input
                          v-model:value="cfg.apiKey"
                          :type="showApiKeyMap[idx] ? 'text' : 'password'"
                          placeholder="sk-**** (不修改请保留原值)"
                          style="flex: 1;"
                        />
                        <n-button size="small" @click="toggleShowApiKey(idx)">
                          {{ showApiKeyMap[idx] ? '隐' : '显' }}
                        </n-button>
                      </div>
                    </n-form-item>
                  </n-gi>

                  <!-- 5. 深度思考开关 -->
                  <n-gi :span="2">
                    <div class="thinking-switch-row">
                      <n-switch v-model:value="cfg.thinking" size="small">
                        <template #checked>深度思考 (Thinking / Reasoning)：已启用</template>
                        <template #unchecked>深度思考 (Thinking / Reasoning)：已禁用</template>
                      </n-switch>
                      <n-button
                        size="small"
                        type="primary"
                        secondary
                        :loading="aiConfigMeta[idx]?.testing"
                        @click="testAiConfig(cfg, idx)"
                      >
                        ⚡ 测试连通性
                      </n-button>
                    </div>
                  </n-gi>
                </n-grid>
              </n-form>
            </n-card>
          </div>
        </div>
      </n-tab-pane>

      <!-- =================================================================== -->
      <!-- TAB 3: 🧠 Prompt 提示词中枢 -->
      <!-- =================================================================== -->
      <n-tab-pane name="prompt" tab="🧠 Prompt提示词中枢">
        <div class="tab-content-wrapper">
          <!-- 系统级角色设定 -->
          <n-card title="系统级角色提示词 (System Prompt)" size="small" class="section-card">
            <template #header-extra>
              <n-space :size="6">
                <span class="preset-label-text">一键载入官方买方预设：</span>
                <n-button size="tiny" quaternary @click="applySystemPromptPreset('quant_mentor')">买方量化导师</n-button>
                <n-button size="tiny" quaternary @click="applySystemPromptPreset('antirisk')">合规风控审查</n-button>
                <n-button size="tiny" quaternary @click="applySystemPromptPreset('standard')">标准全能助手</n-button>
              </n-space>
            </template>
            <n-input
              v-model:value="aiPrompt"
              type="textarea"
              :rows="6"
              placeholder="请输入全局系统角色指令..."
              class="code-font-textarea"
            />
            <div class="textarea-stat-bar">
              <span>当前字符数：{{ aiPrompt.length }}</span>
              <span>严禁使用大模型做伪量化算分，坚持三级证据天梯与研究缺口承认原则</span>
            </div>
          </n-card>

          <!-- 个股诊断问题模板与实时语法 Linter -->
          <n-card title="个股全维量化诊断模板 (User Question Template)" size="small" class="section-card">
            <template #header-extra>
              <n-space :size="6">
                <span class="preset-label-text">快速套用模板：</span>
                <n-button size="tiny" quaternary @click="applyQuestionTemplatePreset('comprehensive')">买方全维研判</n-button>
                <n-button size="tiny" quaternary @click="applyQuestionTemplatePreset('antirisk')">极限排雷体检</n-button>
                <n-button size="tiny" quaternary @click="applyQuestionTemplatePreset('simple')">极简速览分析</n-button>
              </n-space>
            </template>

            <!-- 实时模板语法校验徽章 -->
            <div class="linter-badge-row">
              <n-alert :type="templateLinterStatus.type" :show-icon="true" style="margin-bottom: 8px;">
                {{ templateLinterStatus.text }}
              </n-alert>
            </div>

            <n-input
              v-model:value="questionTemplate"
              type="textarea"
              :rows="6"
              placeholder="输入个股诊断提示词模板，必须包含 {{stockName}} 和 {{stockCode}} 占位符..."
              class="code-font-textarea"
            />
            <div class="textarea-stat-bar">
              <span>当前字符数：{{ questionTemplate.length }}</span>
              <span>支持占位符：<code>&#123;&#123;stockName&#125;&#125;</code> (标的名称)、<code>&#123;&#123;stockCode&#125;&#125;</code> (标的代码)</span>
            </div>
          </n-card>
        </div>
      </n-tab-pane>

      <!-- =================================================================== -->
      <!-- TAB 4: 📊 数据源与预警推送 -->
      <!-- =================================================================== -->
      <n-tab-pane name="market" tab="📊 数据源与预警推送">
        <div class="tab-content-wrapper">
          <!-- 行情数据源路由看板 -->
          <n-card title="行情与量化数据源通道状态" size="small" class="section-card">
            <div class="datasource-channel-list">
              <div class="ds-channel-item">
                <div class="ds-left">
                  <span class="badge-dot success"></span>
                  <strong>东方财富直连通道 (Eastmoney Direct)</strong>
                  <n-tag size="tiny" type="success" round style="margin-left: 8px;">系统默认主通道</n-tag>
                </div>
                <div class="ds-desc">内置纯 Go 零依赖直连，覆盖 A股、港股、美股实时盘口与复权 K 线，无需维护外部服务。</div>
              </div>

              <div class="ds-channel-item">
                <div class="ds-left">
                  <span class="badge-dot info"></span>
                  <strong>AData 容器微服务 (本地/容器内端口 9100)</strong>
                </div>
                <div class="ds-desc">
                  端点：<code>{{ formValue.aDataServiceUrl }}</code> (可在 .env 通过 <code>BLADES_STOCK_ADATA_URL</code> 自由覆盖)。
                </div>
              </div>

              <div class="ds-channel-item">
                <div class="ds-left">
                  <span class="badge-dot info"></span>
                  <strong>AKShare 容器微服务 (本地/容器内端口 9000)</strong>
                </div>
                <div class="ds-desc">
                  端点：<code>{{ formValue.akshareServiceUrl }}</code> (可在 .env 通过 <code>BLADES_STOCK_AKSHARE_URL</code> 自由覆盖)。
                </div>
              </div>

              <div class="ds-channel-item">
                <div class="ds-left">
                  <span class="badge-dot warning"></span>
                  <strong>Tushare 机构数据源 (备用通道)</strong>
                </div>
                <div class="ds-input-row">
                  <n-input
                    v-model:value="formValue.tushareToken"
                    type="password"
                    show-password-on="click"
                    placeholder="可选填写 Tushare API Token (不填写则自动使用东财直连)"
                    style="max-width: 480px;"
                  />
                </div>
              </div>
            </div>
          </n-card>

          <!-- 预警与消息推送 -->
          <n-card title="预警与实时快讯推送" size="small" class="section-card">
            <n-form :model="formValue" label-placement="top" size="small">
              <n-grid cols="1 m:2" :x-gap="24" :y-gap="12">
                <n-gi :span="2">
                  <n-form-item label="钉钉机器人 Webhook (行情异动与研报推送)">
                    <n-space vertical style="width: 100%;">
                      <n-switch v-model:value="formValue.dingPush.enable">
                        <template #checked>钉钉预警推送：已启用</template>
                        <template #unchecked>钉钉预警推送：已禁用</template>
                      </n-switch>
                      <div v-if="formValue.dingPush.enable" class="ding-input-group">
                        <n-input
                          v-model:value="formValue.dingPush.dingRobot"
                          type="password"
                          show-password-on="click"
                          placeholder="https://oapi.dingtalk.com/robot/send?access_token=****"
                          style="flex: 1;"
                        />
                        <n-button size="small" :loading="testingDing" @click="testDingPush">测试推送</n-button>
                      </div>
                    </n-space>
                  </n-form-item>
                </n-gi>

                <n-gi>
                  <n-form-item label="桌面系统级原生弹窗通知">
                    <n-switch v-model:value="formValue.localPush.enable">
                      <template #checked>已启用桌面弹窗通知</template>
                      <template #unchecked>已关闭桌面弹窗</template>
                    </n-switch>
                  </n-form-item>
                </n-gi>

                <n-gi>
                  <n-form-item label="快讯过滤规则">
                    <n-switch v-model:value="formValue.enableOnlyPushRedNews">
                      <template #checked>仅推送红头重大要闻 (过滤普通杂音)</template>
                      <template #unchecked>推送全量盘面快讯</template>
                    </n-switch>
                  </n-form-item>
                </n-gi>
              </n-grid>
            </n-form>
          </n-card>
        </div>
      </n-tab-pane>
    </n-tabs>

    <!-- 预设模型模板库快速载入弹窗 -->
    <n-modal
      v-model:show="wizardVisible"
      preset="card"
      title="✨ 一键快速载入主流大模型配置预设"
      style="width: 90%; max-width: 680px;"
    >
      <div class="wizard-modal-content">
        <p class="wizard-subtitle">选择您所使用的云服务商或本地平台，将自动填入标准的 Base URL、模型标识与推荐思考模式：</p>
        <div class="template-list-box">
          <div
            v-for="(tpl, tIdx) in wizardTemplates"
            :key="tIdx"
            class="template-card-choice"
            @click="applyModelTemplate(tpl)"
          >
            <div class="tpl-title">{{ tpl.label }}</div>
            <div class="tpl-desc">{{ tpl.description }}</div>
            <div class="tpl-endpoint"><code>{{ tpl.baseUrl }}</code> · <code>{{ tpl.modelName }}</code></div>
          </div>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<style scoped>
.settings-container {
  padding: 16px 24px;
  background-color: var(--n-color-base, #12141a);
  min-height: 100vh;
  box-sizing: border-box;
}

.settings-header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 14px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-block {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.main-title {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: #f1f3f8;
}

.meta-links {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #8b949e;
}

.nav-link {
  color: #58a6ff;
  cursor: pointer;
  transition: color 0.2s;
}
.nav-link:hover {
  text-decoration: underline;
  color: #79c0ff;
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.status-tag {
  font-family: monospace;
}

.tab-content-wrapper {
  display: flex;
  flex-direction: column;
  gap: 16px;
  margin-top: 12px;
  max-width: 1200px;
}

.section-card {
  border-radius: 8px;
}

.section-desc {
  margin: 0 0 10px 0;
  font-size: 13px;
  color: #8b949e;
  line-height: 1.5;
}

.stat-tile {
  background: rgba(255, 255, 255, 0.03);
  padding: 10px 14px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.06);
}
.stat-label {
  font-size: 12px;
  color: #8b949e;
  margin-bottom: 4px;
}
.stat-value {
  font-size: 15px;
  font-weight: 600;
  color: #e6edf3;
  margin-bottom: 2px;
}
.stat-desc {
  font-size: 11px;
  color: #6e7681;
}

.text-accent {
  color: #58a6ff;
}
.text-success {
  color: #3fb950;
}

.auth-row-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
  color: #8b949e;
}
.auth-info-desc {
  display: flex;
  align-items: center;
  gap: 6px;
}

.badge-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  display: inline-block;
}
.badge-dot.success { background: #3fb950; box-shadow: 0 0 6px #3fb950; }
.badge-dot.info { background: #58a6ff; }
.badge-dot.warning { background: #d29922; }

.param-hint {
  font-size: 12px;
  color: #8b949e;
}

/* AI 卡片样式 */
.ai-toolbar-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: rgba(255, 255, 255, 0.03);
  border-radius: 6px;
  margin-bottom: 8px;
}

.model-cards-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(480px, 1fr));
  gap: 16px;
  margin-top: 12px;
}

.model-item-card {
  border: 1px solid rgba(255, 255, 255, 0.1);
  transition: border-color 0.2s, box-shadow 0.2s;
}
.model-item-card.is-default-card {
  border-color: #3fb950;
  box-shadow: 0 0 10px rgba(63, 185, 80, 0.15);
}

.card-header-line {
  display: flex;
  align-items: center;
  gap: 8px;
}
.card-title-text {
  font-weight: 600;
  font-size: 14px;
  color: #e6edf3;
}
.card-header-tags {
  display: flex;
  gap: 4px;
}

.model-name-group, .api-key-group, .ding-input-group {
  display: flex;
  gap: 8px;
  width: 100%;
}

.thinking-switch-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 4px;
  padding-top: 6px;
  border-top: 1px dashed rgba(255, 255, 255, 0.08);
}

.empty-configs-box {
  padding: 40px;
  text-align: center;
  color: #8b949e;
  border: 1px dashed rgba(255, 255, 255, 0.15);
  border-radius: 8px;
}

/* Prompt 与文本框样式 */
.preset-label-text {
  font-size: 12px;
  color: #8b949e;
}
.code-font-textarea :deep(textarea) {
  font-family: 'JetBrains Mono', 'Fira Code', Consolas, monospace;
  font-size: 12.5px;
  line-height: 1.6;
}
.textarea-stat-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 6px;
  font-size: 12px;
  color: #8b949e;
}

/* 数据源列表样式 */
.datasource-channel-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.ds-channel-item {
  padding: 10px 14px;
  background: rgba(255, 255, 255, 0.02);
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}
.ds-left {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: #e6edf3;
  margin-bottom: 4px;
}
.ds-desc {
  font-size: 12px;
  color: #8b949e;
  padding-left: 15px;
}
.ds-input-row {
  margin-top: 8px;
  padding-left: 15px;
}

/* 预设向导弹窗样式 */
.wizard-modal-content {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.wizard-subtitle {
  font-size: 13px;
  color: #8b949e;
  margin: 0;
}
.template-list-box {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 400px;
  overflow-y: auto;
}
.template-card-choice {
  padding: 12px 14px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}
.template-card-choice:hover {
  background: rgba(88, 166, 255, 0.08);
  border-color: #58a6ff;
}
.tpl-title {
  font-weight: 600;
  font-size: 14px;
  color: #e6edf3;
  margin-bottom: 4px;
}
.tpl-desc {
  font-size: 12px;
  color: #8b949e;
  margin-bottom: 6px;
}
.tpl-endpoint {
  font-size: 11px;
  color: #58a6ff;
}
</style>
