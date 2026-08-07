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
