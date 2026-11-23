# 反思打分 golden 集（reflection_golden.json）

## 这是什么

一小套已知答案的对照集，用来回答第四轮遗留的问题：**LLM 打分是否比确定性
打分更能抓住「编造数据」**。共 8 条：4 条 fabricated（回答含有工具证据中
不存在的数字/事实，甚至与证据矛盾），4 条 quality（只引用证据数字，
缺数据时明说）。

## 对照怎么跑

`go test -run 'TestReflectionGoldenComparison' ./backend/agent/`

- 每条用 `GenerateReflection` 全路径各跑一次：一次 `llm == nil`（确定性打分），
  一次注入 mock 评分器（`mockGroundingScorer`，见 reflection_golden_test.go）。
- mock 评分器**不是答案 key**：它只从 prompt 里读出「工具证据」和「回答」两节，
  检查回答里的每个数字是否出现在证据/问题中，没出现过的数字扣 accuracy。
  它模拟的是「给定证据时，一个会核对数字的裁判」应有的行为。
- 判定标准（写在测试断言里）：mock 应把 4/4 条 fabricated 打成低分
  （score -1，accuracy ≤ 4），且不误伤 quality；确定性打分预期做不到
  （它对数字是加分项：含 `%`/`亿`/`万` 会抬 accuracy）。

## 重要限制：这套对照目前只能证明「框架 + 判定逻辑」，不能证明「生产收益」

1. **没有真实模型**：本机无可用的模型 API Key，mock 只是规则实现。
   真实对照必须部署后用生产模型跑一次（见下）。
2. **生产 toolsUsed 目前只传工具名**：`postProcessResponse` 传给
   `GenerateReflection` 的是工具名拼接，不是工具返回的数据。也就是说，
   生产里的 LLM 裁判看不到证据数字，只能靠常识/内部一致性判断编造，
   分辨力会弱于本对照里的 mock。想让生产裁判达到对照水平，需要后续
   把「工具返回的关键数字摘要」也传进反思 prompt（另立项，不在本轮范围）。

## 部署后跑真实对照的步骤

1. 部署本轮到 Instabox / VPS，确认设置页里有一张带凭证的 DeepSeek 卡
   （反思选卡优先 DeepSeek；没有 DeepSeek 卡时用首张带凭证卡）。
2. 在服务器上跑（不要在开发机上跑——测试不碰外部 AI 是门禁）：
   `BLADES_STOCK_REFLECTION_GOLDEN_REAL_LLM=1 go test -run 'TestReflectionGoldenComparison_Real' ./backend/agent/ -v`
   该测试默认跳过，只有环境变量置 1 时才调真实模型；它直接复用生产选卡
   （`defaultLLMReflectionFunc`）和同一套 golden 集，输出同样的对照表。
   8 次调用 ≈ ¥0.04，可忽略。
3. 对照表里看两列：真实 LLM 是否把 F1–F4 打低分、是否误伤 Q1–Q4。
   只有真实对照证明收益，才考虑把采样率从 0.1 往上调（改环境变量
   `BLADES_STOCK_REFLECTION_LLM_SAMPLE_RATE`）；否则保持 1/10 采样 +
   每日 20 次上限的受限状态。

## 扩展

加 case 只需往 JSON 里加条目（`kind` 为 `fabricated` 或 `quality`，
`fabricatedFacts` 列出编造点供人阅读）。mock 的扣分规则若改，
同步改测试里的断言阈值并说明原因。
