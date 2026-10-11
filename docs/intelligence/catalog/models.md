# AI 模型配置注册表 (Model Registry)

> 单一事实源：记录 blades-stock 支持的 AI 模型服务商、配置字段、前端存储结构、后端验证逻辑。

## 支持的服务商矩阵

| 服务商 | 兼容协议 | 默认 Base URL | 支持模型示例 | 备注 |
|--------|----------|---------------|--------------|------|
| **DeepSeek** | OpenAI 兼容 | `https://api.deepseek.com` | `deepseek-chat`, `deepseek-reasoner` | 推荐主力 |
| **硅基流动** | OpenAI 兼容 | `https://api.siliconflow.cn` | `Qwen/Qwen2.5-72B-Instruct`, `deepseek-ai/DeepSeek-V3` | 国内访问快 |
| **智谱** | OpenAI 兼容 | `https://open.bigmodel.cn/api/paas/v4` | `GLM-4-Flash`, `GLM-4-Plus`, `GLM-4-Air` | 国产大模型 |
| **豆包** | OpenAI 兼容 | `https://ark.cn-beijing.volces.com/api/v3` | `ep-xxxxx` (需在火山创建端点) | 字节跳动 |
| **Ollama** | 原生 | `http://localhost:11434` | `qwen2.5:7b`, `llama3.1:8b`, `bge-m3` | 本地/私有化部署 |
| **OpenAI** | 原生 | `https://api.openai.com` | `gpt-4o`, `gpt-4o-mini`, `o1-preview` | 需翻墙 |
| **Moonshot (Kimi)** | OpenAI 兼容 | `https://api.moonshot.cn/v1` | `moonshot-v1-8k`, `moonshot-v1-32k` | 长上下文 |
| **通义千问** | OpenAI 兼容 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen-max`, `qwen-plus`, `qwen-turbo` | 阿里云 |
| **自定义** | OpenAI 兼容 | 用户填写 | 用户填写 | 任意兼容端点 |

## 前端配置结构 (`frontend/src/stores/aiConfig.ts`)

```typescript
interface AIConfig {
  id?: number;                    // 数据库主键
  name: string;                   // 显示名，如 "DeepSeek-V3"
  provider: string;               // 服务商标识：deepseek / siliconflow / zhipu / doubao / ollama / openai / custom
  baseUrl: string;                // API 基础地址
  apiKey: string;                 // API Key（Ollama 可留空）
  modelName: string;              // 模型名称（如 deepseek-chat）
  httpProxy: string;              // HTTP 代理（可选）
  httpProxyEnabled: boolean;      // 是否启用代理
  thinking: boolean;              // 是否开启思考模式（推理模型）
  isDefault: boolean;             // 是否为默认模型
  createdAt?: string;
  updatedAt?: string;
}
```

## 后端验证与存储 (`backend/data/settings_api.go`)

- **存储表**：`configs` (JSON 字段 `ai_configs`)
- **验证逻辑**：`TestAiConnection(baseUrl, apiKey, modelName)` — 实际发起 `/chat/completions` 请求
- **模型列表获取**：`FetchAiModels(baseUrl, apiKey)` — 尝试调用 `/models`，失败则回退常用列表
- **Embedding 服务**：独立配置 `embeddingEnabled` / `embeddingBaseUrl` / `embeddingModelName`

## 默认模型推荐

| 场景 | 推荐模型 | 理由 |
|------|----------|------|
| 主力推理/分析 | `deepseek-chat` / `GLM-4-Flash` | 推理强、中文好、国内直连 |
| 复杂推理/回测 | `deepseek-reasoner` / `o1-preview` / `GLM-4-Plus` | CoT 原生支持 |
| 本地/隐私 | `qwen2.5:7b` / `llama3.1:8b` (Ollama) | 离线、数据不出本地 |
| Embedding | `bge-m3` (Ollama/硅基流动/智谱) | 中英双语、检索效果佳 |
| 低成本/高并发 | `qwen-turbo` / `GLM-4-Flash` / `moonshot-v1-8k` | 价格极低 |

## 配置模板 (可直接复制到设置页)

### DeepSeek
```json
{
  "name": "DeepSeek-V3",
  "provider": "deepseek",
  "baseUrl": "https://api.deepseek.com",
  "apiKey": "sk-xxxxx",
  "modelName": "deepseek-chat",
  "thinking": false
}
```

### 硅基流动
```json
{
  "name": "SiliconFlow-Qwen72B",
  "provider": "siliconflow",
  "baseUrl": "https://api.siliconflow.cn",
  "apiKey": "sk-xxxxx",
  "modelName": "Qwen/Qwen2.5-72B-Instruct",
  "thinking": false
}
```

### 智谱
```json
{
  "name": "Zhipu-GLM4-Flash",
  "provider": "zhipu",
  "baseUrl": "https://open.bigmodel.cn/api/paas/v4",
  "apiKey": "xxxx.xxxx",
  "modelName": "GLM-4-Flash",
  "thinking": false
}
```

### 豆包
```json
{
  "name": "Doubao-Pro",
  "provider": "doubao",
  "baseUrl": "https://ark.cn-beijing.volces.com/api/v3",
  "apiKey": "xxxx",
  "modelName": "ep-xxxxx",
  "thinking": false
}
```

### Ollama (本地)
```json
{
  "name": "Ollama-Qwen2.5-7B",
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "apiKey": "ollama",
  "modelName": "qwen2.5:7b",
  "thinking": false
}
```

### Ollama Embedding
```json
{
  "name": "Ollama-BGE-M3",
  "provider": "ollama",
  "baseUrl": "http://localhost:11434",
  "apiKey": "ollama",
  "modelName": "bge-m3",
  "thinking": false
}
```

## Embedding 配置

| 字段 | 默认行为 | 说明 |
|------|---------|------|
| `embeddingEnabled` | `false` | 启用后使用 Embedding 进行语义检索/技能匹配/长期记忆 |
| `embeddingModelName` | `bge-m3` | 推荐 `bge-m3` / `text-embedding-3-small` / `text-embedding-ada-002` |
| `embeddingBaseUrl` | 自动推导 | 留空则：优先用显式配置了 Embedding 的 AI 配置；否则对 Ollama/通义/豆包/OpenAI/硅基流动自动派生 `/embeddings` 端点 |

**自动推导逻辑**：
1. 有 AI 配置显式设置了 `embeddingModelName` → 用该配置的 `baseUrl`
2. Ollama → `baseUrl` + `/embeddings`
3. 通义千问 → `https://dashscope.aliyuncs.com/compatible-mode/v1/embeddings`
4. 豆包 → `baseUrl` + `/embeddings`
5. OpenAI → `https://api.openai.com/v1/embeddings`
6. 硅基流动 → `baseUrl` + `/embeddings`

## 多模型策略

- **默认模型** (`defaultAiConfigId`)：用于日常对话、个股分析、推荐
- **Embedding 模型**：独立配置，专用于语义向量化
- **子 Agent 模型**：复用默认模型，或未来可单独配置
- **回测/量化模型**：当前复用默认模型，未来可指定专用推理模型

## 常见问题

| 问题 | 解决方案 |
|------|----------|
| 连接测试失败 | 检查 Base URL 是否含 `/v1` 后缀；API Key 是否正确；代理设置 |
| 模型列表为空 | 手动填写 `modelName`；或检查 `/models` 端点是否支持 |
| Embedding 报 404 | 确认 Base URL 支持 `/embeddings`；Ollama 需 pull `bge-m3` |
| 响应超时 | 增加 `crawlTimeOut`（默认 60s）；检查代理/网络 |
| 推理模型不输出思考过程 | 确认 `thinking: true` 且模型支持（如 `deepseek-reasoner`） |

## 相关代码

- `frontend/src/stores/aiConfig.ts` — Pinia store，前端配置管理
- `backend/data/settings_api.go` — `GetConfig`/`SaveConfig`/`TestAiConnection`/`FetchAiModels`
- `backend/agent/chat_model_factory.go` — 模型客户端工厂
- `backend/agent/embedding_resolver.go` — Embedding 端点自动推导