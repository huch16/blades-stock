package models

import "time"

// AgentRunRecord 一次 Agent / 研报运行的**可观测记录**。
//
// 为什么需要它（用户反复反馈「到底哪个模型在跑」）：
//
//	界面上显示的是**请求的**模型（选择器里的值），而实际执行时可能因为
//	「指定卡片已被删除」「采用全局默认」「回落第一张卡片」「请求级 thinking 覆盖」
//	而在用**另一个**模型。出问题后只能翻容器日志，且报告里连模型名都没有。
//
// 这张表把事实固化下来：请求 → 实际解析结果 → 备用链 → 结束原因 → 耗时/篇幅，
// 让“哪个模型真的跑了”变成一个可以直接查询与展示的字段，而不是靠猜。
type AgentRunRecord struct {
	ID        uint      `gorm:"primarykey" json:"ID"`
	CreatedAt time.Time `json:"createdAt"`

	// RunID 会话/运行标识；Kind 运行类型（chat / pre_market / post_market / etf_rotation / serenity / summarize / cron）
	RunID string `json:"runId" gorm:"index;size:96"`
	Kind  string `json:"kind" gorm:"size:32;index"`

	// 模型解析结果：请求的 vs 实际使用的
	RequestedID  int    `json:"requestedId"`
	ResolvedID   uint   `json:"resolvedId"`
	Source       string `json:"source" gorm:"size:32"` // explicit / global_default / first_available
	FallbackUsed bool   `json:"fallbackUsed"`
	ModelName    string `json:"modelName" gorm:"size:160"`
	BaseURL      string `json:"baseUrl" gorm:"size:255"`
	Thinking     bool   `json:"thinking"`
	// ToolsInjected 本次实际注入给模型的工具能力摘要（可读、可复现）。
	// 形如："base:12 stock_analysis:26 n=41 est_tokens=1834"。
	// 有了它，「模型当时能看到什么」不再只能靠翻容器日志。
	ToolsInjected string `json:"toolsInjected" gorm:"size:512"`
	// Resolution 人类可读的解析依据；FallbackChain 本 run 实际挂载的备用模型链
	Resolution    string `json:"resolution" gorm:"size:512"`
	FallbackChain string `json:"fallbackChain" gorm:"size:512"`

	// 运行结果
	FinishReason string `json:"finishReason" gorm:"size:32"`
	DurationMs   int64  `json:"durationMs"`
	ContentLen   int    `json:"contentLen"`
	Success      bool   `json:"success"`
	ErrorMsg     string `json:"errorMsg" gorm:"size:512"`
}

// TableName 显式指定表名，避免 GORM 复数化带来的歧义
func (AgentRunRecord) TableName() string {
	return "agent_run_record"
}
