# Memory/Reflection/Evolution 系统验证报告 — 2026-07-09

## 目标
全面审查 blades-stock Agent 的记忆、反思、进化三大子系统的代码实现，验证完整性和正确性。

## 1. 对话记忆系统 (ChatMemoryService)

### 1.1 短期对话记忆 (`chat_memory.go`)
- **存储**：`ChatMemory` 表（session_id, role, content, embedding）
- **检索策略**：
  - 语义检索（默认）：`embeddingService.Embed(query)` → 余弦相似度排序 → top-N（maxMemory/2）
  - 时间检索（降级）：`GetRecentChatMemory(sessionID, maxMemory)` 按时间倒序
- **压缩机制**：`NeedsCompress()` 当消息 >30 条触发 → `CompressHistory()` 保留后半部分 + 前半部分生成摘要写入 `ChatMemorySummary` 表
- **注入位置**：`agent_api.go:170` → `bladeHistory` 数组（user/assistant/system 角色）

### 1.2 工作记忆 (`blades_agent_memory_impl.go`)
- **内存缓存**：`MemoryManager.workingMem`（环形缓冲，maxWorking=20）
- **持久化**：`AgentMemory` 表（category="working"）
- **注入位置**：`postProcessResponse` fire-and-forget → `DefaultMemoryManager().Add(sessionID, role, content)`
- **输出**：`WorkingPrompt()` 生成最近 20 轮摘要（每条截断 200 字符）

### 1.3 长期事实记忆 (`blades_agent_memory_impl.go`)
- **存储**：`AgentMemory` 表（category="fact"）
- **管理**：`StoreFact(sessionID, content)` → 自动递增 `AgentSession.FactCount`
- **检索**：`RetrieveFacts(sessionID, limit=10)` → `MemoryPrompt()` 格式化为"【长期记忆】"文本
- **清理**：`CleanupOldSessions(days)` 按 `created_at` 批量删除

### 1.4 分析报告记忆 (`analysis_memory.go`)
- **数据源**：`AIResponseResult` 表（历史 AI 分析报告）
- **触发**：`extractStockInfo(question)` 提取股票代码/名称 → `BuildAutoInjectContext(stockCode, stockName)`
- **注入**：`bladeHistory` 前插入 SystemMessage，每条报告截断 500 字符，最多 5 条
- **回测记录**：`BacktestService.GetStockAccuracy(stockCode)` → 历史准确率注入 sysPrompt

## 2. 反思系统 (Evaluator)

### 2.1 自动评分 (`blades_agent_reflection_impl.go`)
- **关键词评分**：正向词（成功/完成/获取到...）+1，负向词（失败/错误/超时...）-1
- **LLM 反思**（可选）：`GenerateReflection` 构建 prompt → LLM 返回 JSON `{score, insight, improvement}`
- **降级**：LLM 不可用时使用 `defaultInsight(score)` / `defaultImprovement(score)`
- **持久化**：`AgentTrace` 表（session_id, question, answer, score, feedback, duration_ms）

### 2.2 反馈记录
- `RecordFeedback(traceID, score, feedback)` → 更新 `AgentTrace` 记录
- `GetHighQualityTraces(limit)` → score > 0 的轨迹
- `GetLowQualityTraces(limit)` → score < 0 的轨迹

### 2.3 后处理触发 (`agent_api.go:postProcessResponse`)
- fire-and-forget goroutine，不阻塞响应流
- 并行执行：工作记忆存储 + 反思评分 + 工具使用记录

## 3. 进化系统 (Evolver)

### 3.1 工具排名 (`blades_agent_evolution_impl.go`)
- **表**：`agent_tool_ranks`（tool_name, success_count, fail_count, avg_duration_ms）
- **记录**：`RecordToolUse(toolName, success, durationMs)` → 增量更新
- **排名**：`RankTools()` → 按 success_count 降序

### 3.2 Prompt 优化
- `OptimizePrompt(trace, llm)` → 分析低分轨迹 → LLM 生成优化建议 JSON
- 输出：`{analysis, optimizedPrompt}`

### 3.3 技能生成
- **从轨迹提取**：`GenerateSkillFromTraces(llm)` → 高分轨迹 → LLM → JSON 技能草稿
- **从项目学习**：`LearnFromProject(projectName, analysis, llm)` → 项目分析报告 → LLM → 自动写入 `Skill` 表
- **LLM 回调**：`LLMFromConfig()` 从第一个可用 AI 配置创建 HTTP 客户端

### 3.4 周报 (`ProposeEvolutionPlan`)
- 统计本周高/低分轨迹数量
- 提取低分轨迹的改进建议
- 提取高分轨迹的成功模式
- 返回 `EvolutionPlan` 结构体

## 4. Skill 语义匹配系统

### 4.1 Embedding 服务 (`embedding_service.go`)
- OpenAI 兼容 API（`/v1/embeddings`）
- 自动去除 `/chat/completions` 后缀
- 返回 `[]float32` 向量

### 4.2 Skill 嵌入 (`skill_embedding.go`)
- **表**：`skill_embeddings`（skill_id, embedding）
- **刷新**：`RefreshEmbeddings()` → 并行嵌入所有启用 Skill 的 `description + systemPrompt`
- **匹配**：`MatchSkills(question, topK)` → 问题嵌入 → 余弦相似度 >0.3 → 排序 → top-K
- **降级**：`keywordMatch(skills, question, topK)` → `TriggerKeywords` 逗号分隔匹配
- **阈值**：0.3 相似度（bge-m3 模型）

### 4.3 注入流程 (`agent.go:buildSkillPrompt`)
- `buildSkillPrompt(question, embedSvc)` → 匹配 Skill → Directive 优先（~800-2000 chars）→ SystemPrompt 降级（~15k chars）
- 注入位置：`sysPrompt` 中 `settingConfig.Prompt` 之后、强制规则之前

## 5. 并行调度器 (`parallel_dispatcher.go`)

### 5.1 触发条件
- `shouldParallelize(question)` → 非基础工具组 >= 2 个

### 5.2 执行流程
1. `decomposeIntoSubPrompts` → 每组生成专项子 prompt
2. `semaphore.NewWeighted(concurrency)` → 限制并发 LLM 调用（默认 2）
3. 每个子 Agent 独立 model + tools + 60s 超时
4. `buildParallelContext` → 汇总数据注入主 Agent sysPrompt

### 5.3 降级处理
- 信号量获取失败 → `[并发受限]`
- 模型创建失败 → `[模型创建失败]`
- 子 Agent 超时 → `[超时]`
- 无数据 → `[无数据]`
- 有部分数据 → 不标记 fallback，主 Agent 可补充调用

## 6. 数据库表汇总

| 表名 | 用途 | 自动迁移 |
|------|------|---------|
| `chat_memories` | 对话消息+嵌入 | ✅ |
| `chat_memory_summaries` | 压缩摘要 | ✅ |
| `agent_memories` | 工作记忆+事实 | ✅ |
| `agent_sessions` | 会话摘要+事实计数 | ✅ |
| `agent_traces` | 执行轨迹+反思评分 | ✅ |
| `agent_tool_ranks` | 工具排名统计 | ✅ |
| `skill_embeddings` | Skill 语义嵌入 | ✅ |

## 7. 编译验证
- `go build ./...` ✅ EXIT:0
- `go vet ./...` ✅ EXIT:0

## 结论
Memory/Reflection/Evolution 三大子系统代码实现完整：
- **4 层记忆**：短期对话（语义检索+压缩）→ 工作记忆（环形缓冲）→ 长期事实 → 分析报告记忆
- **2 级反思**：自动关键词评分 + LLM 深度反思
- **3 维进化**：工具排名 → Prompt 优化 → 自动技能生成
- **Skill 匹配**：bge-m3 语义匹配（阈值 0.3）+ 关键词降级
- **并行调度**：多组问题并行子 Agent + 信号量限流 + 超时降级
