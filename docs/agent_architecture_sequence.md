# Blades-Stock Agent 核心时序与运行时约束架构规范

本文档详尽记录了 `blades-stock` 系统中 Agent 调用链的完整生命周期、时序交互、消息流转拓扑、关键超时/重试约束点以及代码锚点，方便开发调试、性能优化与生产排障。

---

## 1. 运行时约束与弹性控制矩阵 (Runtime Constraints Matrix)

| 约束维度 | 关键参数 / 规则 | 对应代码位置 | 默认阈值 / 行为 | 设计目的与防范风险 |
| :--- | :--- | :--- | :--- | :--- |
| **会话总超时** | `TimeOutSec` (Session Context) | `backend/agent/agent_api.go` | `max(AIConfig.TimeOut, 900s)` | 兜底长时思考（Thinking 模式）与多轮工具调用，防长任务过早夭折；防死锁挂起 |
| **工具迭代步数** | `WithMaxIterations` | `backend/agent/agent.go` | `max(len(tools)*2 + 10, 30)` | 彻底杜绝 LLM 在复杂多工具编排中陷入死循环无限调用；保证推理预算收敛 |
| **MCP 建连超时** | `Connect(ctx, 5s)` | `backend/agent/agent.go` | `5 秒` | 防止外部 MCP 进程僵死或网络不可达时阻塞 Agent 启动主流程 |
| **MCP 解析超时** | `Resolve(ctx, 5s)` | `backend/agent/agent.go` | `5 秒` | 防止大型 MCP Server 工具列表拉取过慢阻塞会话 |
| **MCP 工具缓存** | `mcpToolsCache` | `backend/agent/agent.go` | `TTL = 5 分钟` | 避免每次用户提问都重新协商 MCP 工具 schema，消除毫秒级重复握手开销 |
| **瞬时故障重试** | `maxAttempts` (Backoff Retry) | `backend/agent/agent_api.go` | `最多 3 次` (指数退避) | 针对 `429`, `5xx`, `unexpected end of JSON input`, `EOF` 进行重试（仅在首包未流出时触发） |
| **多模型热降级** | `FallbackModelProvider` | `backend/agent/fallback_provider.go` | 自动依次切换备用模型 | 主模型在流式首包断流/崩溃时，基于 `iter.Seq2` 透明切换至备用模型输出 |
| **乱码检测熔断** | `IsDegenerateGarbage` | `backend/agent/agent_api.go` | 抛出 `DegenerateOutputError` | 识别模型输出大量 `<unk>` 重复乱码流，立即熔断当前生成并触发退避重试 |
| **工具入参安全** | `Param Defensive Sanitizer` | `backend/agent/tools/*` | 空串转 `"{}"`，不抛中断 Error | 防范 LLM 偶发生成空 JSON 引发反序列化失败导致 Agent 崩溃 |
| **异步后处理** | `postProcessResponse` | `backend/agent/agent_api.go` | `sync.WaitGroup` + 独立 `recover` | 记忆入库、自反思打分、工具权重进化与研报沉淀完全异步，零阻塞前端交互响应 |

---

## 2. 交互时序图 (Sequence Diagrams)

### 2.1 Mermaid 版本
```mermaid
sequenceDiagram
  participant User as User (UI)
  participant Front as Frontend (Vue)
  participant Wails as Wails App (App.NewChatStream)
  participant AgentAPI as StockAiAgent API (ChatWithContext)
  participant Match as SkillMatch / EmbeddingService
  participant Builder as GetStockAiAgent (agent.GetStockAiAgent)
  participant ToolSel as ToolSelector (getToolsByQuestion)
  participant MCP as MCP Connector (connectMCPServer)
  participant Blades as Blades Agent (Agent.Run)
  participant Model as ModelProvider (LLM)
  participant Tool as Tool (Local / MCP)
  participant DB as DB / Data APIs
  participant Post as PostProcessor (postProcessResponse)

  User->>Front: 输入问题 / 点击分析
  Front->>Wails: 调用 NewChatStream(...)
  Wails->>AgentAPI: ChatWithContext(question, aiConfigId, ...)
  AgentAPI->>Match: matchSkills(question, EmbeddingService)
  Match-->>AgentAPI: matchedSkills
  AgentAPI->>Builder: GetStockAiAgent(aiConfig, question, agentMode, matchedSkills)
  Builder->>ToolSel: getToolsByQuestion(question, matchedSkills)
  ToolSel->>MCP: getMCPTools() (并发查询所有已启用 MCP)
  note right of MCP: MCP connect / resolve per-server\nConnect & Resolve timeout = 5s\nmcpTools cache TTL = 5min
  MCP-->>ToolSel: mcpTools (或空)
  ToolSel-->>Builder: filtered & reordered tools
  Builder->>Blades: blades.NewAgent(... WithTools(allTools) ...)
  note right of Blades: MaxIterations = len(allTools)*2 + 10 (min 30)
  AgentAPI->>Blades: invocation (UserMessage + SystemPrompt + History + Metadata)
  note right of AgentAPI: Apply AI-config timeout\nTimeOutSec (>=900s floor)
  Blades->>Model: streaming request
  Model-->>Blades: streaming deltas (assistant text) / tool-invoke signals
  Blades->>Tool: call tool handler when model requests
  Tool->>DB: fetch KLine / finance / news ... (或调用外部服务)
  Tool-->>Blades: tool response (wrapped)
  Blades-->>AgentAPI: ToolPart in stream (Tool name + response)
  AgentAPI-->>Front: stream ReasoningContent / Content / STEP updates
  alt Model transient error or degenerate (<unk>) and no content streamed
    AgentAPI->>Model: retry runOnce (max 3 attempts)
    note right of AgentAPI: Retry conditions:\n- isTransientModelError (429/5xx/overload/断流)\n- 或 degenerate garbage 输出\n- 且未已流出 assistant 内容
  end
  Blades-->>AgentAPI: final answer
  AgentAPI->>Post: postProcessResponse (fire-and-forget)
  Post->>DB: save memory, save AIResponseResult, update tool ranking, run eval
  AgentAPI-->>Front: final DONE event
```

### 2.2 PlantUML 版本
```plantuml
@startuml
title Blades-stock: User -> Frontend -> Agent 时序（含超时/重试标注）

participant "User (UI)" as User
participant "Frontend (Vue)" as Front
participant "Wails App\n(App.NewChatStream)" as Wails
participant "StockAiAgent API\n(ChatWithContext)" as AgentAPI
participant "SkillMatch / Embedding" as Match
participant "GetStockAiAgent\n(agent.GetStockAiAgent)" as Builder
participant "ToolSelector\n(getToolsByQuestion)" as ToolSel
participant "MCP Connector\n(connectMCPServer)" as MCP
participant "Blades Agent\n(Agent.Run)" as Blades
participant "ModelProvider\n(LLM)" as Model
participant "Tool (Local/MCP/DB/HTTP)" as Tool
participant "PostProcessor\n(postProcessResponse)" as Post

User -> Front : 问题/分析请求
Front -> Wails : 调用 NewChatStream(...)
Wails -> AgentAPI : ChatWithContext(question,aiConfigId,...)

AgentAPI -> Match : matchSkills(question, EmbeddingService)
Match --> AgentAPI : matchedSkills

AgentAPI -> Builder : GetStockAiAgent(aiConfig, question, agentMode, matchedSkills)
Builder -> ToolSel : getToolsByQuestion(question, matchedSkills)
ToolSel -> MCP : getMCPTools() (并发请求每个启用 MCP server)
note right of MCP
  MCP connect/resolve timeout = 5s
  mcpTools cache TTL = 5 minutes
end note
MCP --> ToolSel : mcpTools (或空)
ToolSel --> Builder : filtered & reordered tools

Builder -> Blades : blades.NewAgent(..., WithTools(allTools), WithMaxIterations)
note right of Blades
  MaxIterations = len(allTools)*2 + 10 (min 30)
end note

AgentAPI -> Blades : Invocation (UserMessage + SystemPrompt + History + Metadata)
note right of AgentAPI
  Agent total timeout := AIConfig.TimeOut (default 300s) \n but floored to 900s (15min)
end note

Blades -> Model : streaming request
Model --> Blades : streaming deltas (assistant text) + tool-invoke signals

alt model issues & tool calls
  Blades -> Tool : 调用工具 (local 或 MCP)
  Tool -> Tool : 内部 DB/API/网络调用
  Tool --> Blades : tool response
  Blades --> AgentAPI : ToolPart in stream (name + response)
  AgentAPI --> Front : stream ReasoningContent / Content / STEP updates
end

alt transient or degenerate (<unk>) and no content streamed
  AgentAPI -> Model : retry runOnce (最多 3 次)
  note right of AgentAPI
    retry 条件: isTransientModelError (429/5xx/overload) \n 或 degenerate garbage 输出
    retry 上限 = 3 attempts
  end note
end

Blades --> AgentAPI : final answer
AgentAPI -> Post : postProcessResponse (异步)
Post -> Post : 保存 memory、评分、持久化 AI 报告、更新工具排名
AgentAPI --> Front : final DONE event

@enduml
```

---

## 3. 消息流与决策状态机 (Flowcharts)

### 3.1 Mermaid 流程图
```mermaid
flowchart TD
  A[User 用户发起请求 (Frontend)] --> B[Wails 桥接入口 (NewChatStream)]
  B --> C[StockAiAgent.ChatWithContext]
  
  C --> D{是否开启 Embedding 匹配?}
  D -->|是| E[EmbeddingService.MatchSkills 向量匹配]
  D -->|否/降级| F[Keyword Fallback 关键字匹配]
  
  E --> G[构建 System Prompt\n(SkillPrompt + SystemRules + GlobalSensor + MarketBrief)]
  F --> G
  
  G --> H[GetStockAiAgent -> 构建 Agent 实例]
  H --> I[getToolsByQuestion 按需路由工具]
  
  I --> J[tools.ClassifyQuestion 问题意图分类]
  I --> K[mergeSkillToolGroups 强合并 Skill 工具组]
  J --> L[FilterToolsByGroupsCapped 截断筛选]
  K --> L
  L --> L2[reorderToolsByRank 历史成功率重排]
  
  I --> M[getMCPTools 并发获取 MCP 工具 (缓存 5min)]
  M -->|遍历已启用 MCP 服务| N[connectMCPServer]
  N -->|stdio / HTTP| O[NewClient + Connect 5s + Resolve 5s]
  O --> M
  
  L2 --> P[创建 blades.Agent\n(注入 ModelProvider / Middleware / MaxIterations)]
  M --> P
  
  P --> Q[preFetchInParallel 并发预采集数据]
  Q --> R[blades.Agent.Run 核心流式运行]
  
  R --> S[ModelProvider 流式生成]
  S -->|工具调用请求| T[Tool Handler (Local 或 MCP)]
  T --> U[底层数据源 (Data APIs / DB / External HTTP)]
  U --> T
  T --> R
  
  R --> V{是否触发乱码或瞬时错误?}
  V -->|是 且 无流出文本| W[runOnce 退避重试 (最多 3 次)]
  W --> R
  V -->|否 (正常响应)| X[实时推送 StreamMessage 至前端]
  
  X --> Y[postProcessResponse 异步后处理]
  Y --> Z[持久化记忆 + 结果自反思评估 + 智能研报落库 + 工具权重进化]

  subgraph Guardians [🛡️ 关键保护哨兵]
    TimeoutNote["⏱️ 会话超时: TimeOutSec (下限 900s)"]
    MaxIterNote["🔄 迭代防爆: MaxIterations = len(tools)*2 + 10 (下限 30)"]
    MCPTimeoutNote["🔌 MCP 哨兵: Connect/Resolve 双 5s 超时, 缓存 TTL=5min"]
    RetryNote["🔁 重试哨兵: 瞬时故障 (429/5xx/断流/截断) 重试 <= 3 次"]
  end

  C -.-> TimeoutNote
  P -.-> MaxIterNote
  O -.-> MCPTimeoutNote
  W -.-> RetryNote
```

### 3.2 PlantUML 流程图
```plantuml
@startuml
title Blades-stock: 流程图 (消息流 / 超时 / 重试 / 缓存)

start
:User 在前端发起请求;
:Frontend 调用 Wails App.NewChatStream;
:App -> StockAiAgent.ChatWithContext;

if (Match Skills?) then (yes)
  :EmbeddingService.MatchSkills;
else (no)
  :关键词回退匹配;
endif

:构建 SystemPrompt (skillPrompt + SystemRules + market brief + history);

:调用 GetStockAiAgent -> 构建 Agent;
:执行 getToolsByQuestion:
:  - ClassifyQuestion -> groups;
:  - mergeSkillToolGroups;
:  - FilterToolsByGroupsCapped;
:  - reorderToolsByRank;

fork
  :getMCPTools() 并发连接每个 MCP server;
  note right
    MCP Connect/Resolve Timeout = 5s;
    mcpTools cache TTL = 5min;
  end note
fork again
  :计算 MaxIterations = len(allTools)*2 + 10 (min 30);
end fork

:可选 preFetchInParallel (并发子任务);
:调用 blades.Agent.Run(Invocation) -> 向 ModelProvider 发起 streaming 请求;

while (streaming)
  if (Model 请求工具) then (yes)
    :调用工具 (Local 或 MCP);
    :工具从 DB/API/外网获取数据;
    :工具返回 -> Agent 收到 ToolPart -> 向前端发送 STEP 更新;
  else (no)
    :模型输出 assistant delta -> 处理并 stream 到前端;
  endif

  if (检测到 degenerate 或 transient error 且未流出任何内容) then (retry)
    :重试 runOnce，最多 3 次;
  endif
endwhile

:收到最终答案 -> 异步 postProcessResponse;
:结束 (向前端发送 DONE);
stop

@enduml
```

---

## 4. 关键代码映射表

- **Wails App 绑定入口**：`app.go` -> `NewChatStream(...)`
- **Agent 流式调度入口**：`backend/agent/agent_api.go` -> `ChatWithContext(...)`
- **技能语义匹配与提示词生成**：`backend/agent/agent.go` -> `matchSkills(...)` / `BuildShortSkillPrompt(...)`
- **Agent 工厂构建与中间件配置**：`backend/agent/agent.go` -> `GetStockAiAgent(...)`
- **工具意图分类与过滤**：`backend/agent/agent.go` -> `getToolsByQuestion(...)` / `backend/agent/tools/tool_groups_config.go`
- **MCP 连接协商与白名单**：`backend/agent/agent.go` -> `connectMCPServer(...)` / `getMCPTools(...)`
- **多模型流式降级**：`backend/agent/fallback_provider.go` -> `FallbackModelProvider.NewStreaming(...)`
- **执行循环与瞬时故障判定**：`backend/agent/agent_api.go` -> `runOnce(...)` / `isTransientModelError(...)`
- **异步闭环后处理**：`backend/agent/agent_api.go` -> `postProcessResponse(...)`
- **股票推荐沉淀管道**：`backend/agent/recommend_extractor.go` -> `ExtractAndSaveRecommendationsFromReport(...)`
