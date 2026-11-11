# 历史遗留：prompt_templates 目录（非生效配置）

⚠️ **这里的 .txt 文件不参与任何运行逻辑，不要修改它们来改变系统行为。**

## 为什么放在这里

这四个文件由提交 `02973f9`（Add agent memory/reflection/evolution layers + bug fixes）
随「记忆 / 反思 / 进化」三层一起加入仓库，原意是作为提示词的独立配置源。

但实现时提示词被**内联进了 Go 常量**，这些文件从未被任何代码读取：

- 现在：`grep -rn "prompt_templates" --include=*.go` 只命中一个**数据库表名**
  （`db.Dao.Table("prompt_templates")`），与目录无关；
- 历史上：`git log -S "prompt_templates/" --all -- '*.go'` 为空 —— **从未被读取过**。

它们此前留在仓库根部，容易被误当成「生效的提示词配置」而修改后毫无效果。
为消除这个陷阱，已移入 `docs/archive/`。

## 内容与真实实现的对应关系

| 文件 | 真实生效位置 |
|---|---|
| `skill_generation.txt` | `backend/agent/blades_agent_evolution_impl.go` → `GenerateSkillFromTraces`（内联 prompt） |
| `prompt_evolution.txt` | `backend/agent/blades_agent_evolution_impl.go` → `OptimizePrompt`（内联 prompt） |
| `reflection_prompt.txt` | `backend/agent/blades_agent_reflection_impl.go` → `GenerateReflection`（内联 prompt） |
| `memory_system_prompt.txt` | `backend/agent/blades_agent_memory_impl.go`（记忆层系统提示，内联） |

> 注意：内联 prompt 与这些 .txt 的内容**已经不一致**（内联版是后来演化的版本）。
> 要改提示词，改 Go 里那一份。

## 与「反思 / 进化闭环」当前状态的关系

`reflection_prompt.txt` 与 `prompt_evolution.txt` 对应的**反思评分**与
**Prompt 进化建议**目前处于未接线（DORMANT）状态 —— 其唯一调用点已在
提交 `a158883`（strip agent reflection token burn）中被移除。
详见 `GenerateReflection` 与 `maybeGenerateEvolutionAdvice` 的函数注释。

其中「确定性评分 + 写长期记忆」是**零 token** 的、被误伤删除的部分；
恢复时应只恢复该部分，不要连带恢复 LLM 打分。
