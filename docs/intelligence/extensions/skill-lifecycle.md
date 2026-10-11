# 技能生命周期与版本化 (Skill Lifecycle)

> 记录技能从生成 → 校验 → 向量化 → 存储 → 匹配 → 注入 → 审批/归档的完整生命周期管理。

## 状态机

```
┌─────────┐     生成/手工创建     ┌─────────┐     校验通过     ┌───────────┐
│  Draft  │ ──────────────────▶ │  Draft  │ ──────────────▶ │ Published │
└─────────┘                     └─────────┘                 └─────┬─────┘
      │                                                            │
      │ 删除/废弃                                                  │ 使用/匹配
      ▼                                                            ▼
┌─────────┐                                                    ┌─────────┐
│Archived │                                                    │  Active │
└─────────┘                                                    └─────────┘
```

| 状态 | 含义 | 可操作 | 向量化 |
|------|------|--------|--------|
| `draft` | 草稿/待审核 | 编辑/删除/提交审核 | 否 |
| `published` | 已发布 | 匹配注入/查看/归档 | 是 |
| `archived` | 已归档 | 查看/恢复/永久删除 | 否 |

## 版本化策略

### 版本号规则
- 单调递增整数：`v1`, `v2`, `v3`...
- 同一 `SkillID` 下唯一
- 每次编辑提交 → 新建版本（旧版本保留）

### 版本存储 (`skill_versions` 表)

```sql
CREATE TABLE skill_versions (
    id          BIGSERIAL PRIMARY KEY,
    skill_id    BIGINT NOT NULL REFERENCES skills(id),
    version     INT NOT NULL,              -- 1, 2, 3...
    name        VARCHAR(128),
    description TEXT,
    tool_groups VARCHAR(512),              -- 逗号分隔
    entry_points JSONB,                    -- ["main.go", "handler.go"]
    directive   TEXT,                       -- 注入给 Agent 的指令
    status      VARCHAR(32),               -- draft/published/archived
    source_url  VARCHAR(512),
    source_hash VARCHAR(64),               -- 内容哈希（SHA256）
    created_at  TIMESTAMP DEFAULT NOW(),
    approved_at TIMESTAMP,
    approved_by VARCHAR(128)
);
UNIQUE INDEX uq_skill_version (skill_id, version);
```

### 变更触发新版本

| 操作 | 是否新版本 | 说明 |
|------|------------|------|
| 修改 `directive` | ✅ | 核心行为变更 |
| 修改 `tool_groups` | ✅ | 依赖工具变更 |
| 修改 `description` | ✅ | 语义变更影响匹配 |
| 修改 `name` | ❌ | 仅显示名，不影响逻辑 |
| 修改 `status` | ❌ | 状态流转不算内容变更 |

## 内容哈希去重 (`source_hash`)

```go
func computeSourceHash(req SkillGenerationRequest) string {
    content := req.Readme + req.Structure + req.Summary +
               strings.Join(req.EntryPoints, "") + req.APISummary + req.ModelSummary
    hash := sha256.Sum256([]byte(content))
    return hex.EncodeToString(hash[:])
}
```

- 同一源码/文档重复提交 → 同一 `source_hash` → 识别为已存在技能 → 不重复生成
- 增量更新：`source_hash` 变化 → 判定为内容更新 → 生成新版本

## 向量化流程 (`SkillEmbeddingService`)

### 触发时机
1. 技能首次 `published`
2. 版本更新且内容哈希变化
3. 手工触发重建（Admin UI）

### Embedding 内容

```go
embeddingText := fmt.Sprintf(
    "技能: %s\n描述: %s\n工具组: %s\n入口: %s\nAPI: %s\n模型: %s",
    skill.Name, skill.Description, skill.ToolGroups,
    strings.Join(entryPoints, ", "), skill.APISummary, skill.ModelSummary,
)
```

### 存储 (`skill_embeddings` 表)

```sql
CREATE TABLE skill_embeddings (
    id            BIGSERIAL PRIMARY KEY,
    skill_id      BIGINT NOT NULL UNIQUE REFERENCES skills(id),
    content_hash  VARCHAR(64) NOT NULL,
    embedding     JSONB NOT NULL,          -- []float32 序列化
    updated_at    TIMESTAMP DEFAULT NOW()
);
```

### 匹配算法

```go
// 1. 问题关键词 → 激活工具组集合
groups := ClassifyQuestion(question)

// 2. 向量检索：在激活组内的技能中做 Top-K 相似度搜索
candidates := db.Where("tool_groups && ?", groups).Find(&skills)

// 3. 计算 cosine 相似度
for _, s := range candidates {
    sim := cosineSimilarity(queryEmbedding, s.Embedding)
    if sim > threshold { results = append(results, s) }
}

// 4. Top-K 注入 Directive
sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
injected := results[:min(3, len(results))]
```

## 技能注入协议

### Directive 结构建议

```markdown
## 技能：{Name}

### 触发条件
- 用户提问包含：{关键词1} / {关键词2} / ...

### 可用工具组
{tool_groups}

### 执行步骤
1. {步骤1} → 调用 {工具}
2. {步骤2} → 调用 {工具}
...

### 输出格式
- {字段1}：{说明}
- {字段2}：{说明}

### 示例
用户："帮我分析一下 000001"
→ 步骤1：调用 GetStockInfo(000001)
→ 步骤2：调用 DiagnoseStock(000001)
→ 输出：{结构化分析报告}
```

### 注入点

| 阶段 | 注入内容 | 位置 |
|------|----------|------|
| 系统提示词 | `PromptTemplates.Personality` + 所有 `Published` 技能的 `Directive` 摘要 | `system` message |
| 用户提问时 | Top-3 匹配技能的完整 `Directive` | `system` message (追加) |
| 子 Agent 创建时 | 按领域筛选技能 Directive | `system` message |

## 审批流程

### 自动审批（内置/高优先级）
- `SkillGenerator` 生成 + `SkillValidator` 校验通过 → 自动 `published`
- 内置核心技能（ETF轮动/3D诊断等）部署时预置为 `published`

### 人工审批（用户生成/第三方）
1. 用户通过 `LearnProject` / 手工创建 → `draft`
2. Admin UI 列表展示待审核技能
3. 审核人检查：
   - `ToolGroups` 是否最小必要
   - `Directive` 是否清晰、无歧义
   - 是否有安全风险（如 `RunCommand`、`WriteProjectFile`）
4. 通过 → `published` + 向量化
4. 拒绝 → `archived` + 理由记录

## 迁移与兼容

### Schema 变更
- 新增字段：在 `SkillVersion` 结构体添加 + `AutoMigrate`
- 字段重命名：保留旧字段 + 新字段，迁移脚本回填

### 版本回滚
```go
// 回滚到指定版本
func RollbackSkill(skillID uint, targetVersion int) error {
    v := SkillVersion{}
    if err := db.Where("skill_id=? AND version=?", skillID, targetVersion).First(&v).Error; err != nil {
        return err
    }
    return db.Model(&Skill{}).Where("id=?", skillID).Updates(map[string]interface{}{
        "name": v.Name,
        "description": v.Description,
        "tool_groups": v.ToolGroups,
        "entry_points": v.EntryPoints,
        "directive": v.Directive,
    }).Error
}
```

## 监控指标

| 指标 | 含义 | 告警阈值 |
|------|------|----------|
| `skill.total` | 技能总数 | > 200 时评估冗余 |
| `skill.published` | 发布数 | - |
| `skill.draft` | 草稿数 | > 20 时提醒清理 |
| `skill.match_rate` | 匹配命中率 | < 10% 优化关键词/向量 |
| `skill.embedding_age` | 向量更新时长 | > 7天 触发重建 |
| `skill.version_avg` | 平均版本数 | > 5 说明频繁变更 |

## 相关代码

- `backend/agent/skill_generator.go` — 生成入口
- `backend/agent/skill_validator.go` — 校验逻辑
- `backend/agent/skill_embedding.go` — 向量化服务
- `backend/agent/tools/mcp_skill_tools.go` — MCP 技能桥接
- `backend/data/skill_api.go` — REST API (CRUD/版本/审批/向量重建)
- `backend/models/skill_version.go` — 数据模型
- `backend/models/skill_change.go` — 变更审计日志