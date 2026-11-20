package util

// TruncateString 按 rune 截断字符串到 maxLen；超出时追加 suffix（为空则不追加）。
// agent 侧传入 "\n\n...(数据过长，已截断)"，一次性工具传入 ""。
func TruncateString(s string, maxLen int, suffix string) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + suffix
}
