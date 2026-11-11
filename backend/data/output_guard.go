package data

import "strings"

// DegenerateOutputError 标记模型流式输出了 <unk> 乱码而非真实内容。
// 在 agent 流水线中视为可重试错误（模型偶发崩坏，重试可能恢复正常）。
type DegenerateOutputError struct{}

func (DegenerateOutputError) Error() string {
	return "模型输出异常：检测到乱码(<unk>)流"
}

// IsDegenerateGarbage 判断文本是否包含词汇表外 token <unk>。
// 正常的中文股票分析回答绝不会出现 <unk>；一旦出现即视为模型崩坏
// （已在 nemotron-3-ultra 上观察到 "Now إن<unk><unk>..." 乱码流）。
func IsDegenerateGarbage(text string) bool {
	return strings.Contains(text, "<unk>")
}

// EmptyOutputError 标记模型本次调用**没有产出任何可见正文**。
//
// 这不是「乱码」（DegenerateOutputError），而是「静默空答」：模型正常
// 结束了这一轮（finish_reason=stop）、耗时可能长达数十秒，但正文一直是空的。
// 实测在 NVIDIA nemotron-3.5-super-vl-120b-a12b 上出现过：同一张卡片、同一
// 个问题，有时返回 2000+ 字符，有时跑满 55 秒却一个字都不给（间歇性，
// 因此可重试）。
//
// 与 DegenerateOutputError 一样，在 agent 流水线中视为可重试错误。
// 两个诊断字段用于把「为什么空」说清楚，避免前端只能渲染无信息量的
// 「（空回复）」：
//   - ProcessChars > 0 → 模型做了过程（思考 / 工具调用）但没写结论
//   - FirstChunkSeen == false → 上游网关/模型根本没回任何东西
//
// 用结构体（而非空结构体）承载诊断，是为了让错误文案能给出**可执行**的
// 建议，而不是笼统的「模型异常」。
type EmptyOutputError struct {
	// ProcessChars 是本次尝试已下发给前端的**过程内容**字符数，
	// 包含两类来源，两者对用户都等价于「有过程、无结论」：
	//  1. 模型自身的 reasoning 增量（若上游/provider 提供了）
	//  2. 由本系统发出的工具调用步骤文本（[STEP]🔧 调用工具… 及返回摘要）
	//
	// 刻意合并为一个计数：错误文案本来就把它们并称为「思考/工具过程」，
	// 拆成两个字段只会多出无意义的组合分支。
	ProcessChars int
	// FirstChunkSeen 表示是否收到过模型的任何响应片段。
	FirstChunkSeen bool
}

func (e EmptyOutputError) Error() string {
	if e.ProcessChars > 0 {
		return "模型未产出正文：只输出了思考/工具过程"
	}
	if !e.FirstChunkSeen {
		return "模型未产出正文：上游未返回任何响应内容"
	}
	return "模型未产出正文（空响应）"
}

// IsEmptyOutput 判断错误是否为「静默空答」。
func IsEmptyOutput(err error) bool {
	_, ok := err.(EmptyOutputError)
	return ok
}
