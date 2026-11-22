package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// TestIsValidMarketReport_TruncatesErrorReason 验证无效报告的
// 错误原因被截断：旧实现把 ❌ 分支的整篇研报原文原样塞进
// error，调用方再原样打日志，几千字的研报会整篇刷进日志。
func TestIsValidMarketReport_TruncatesErrorReason(t *testing.T) {
	long := "❌ 生成失败：" + strings.Repeat("这是一段很长的无效研报正文内容，包含大量重复文字。", 100)
	valid, reason := isValidMarketReport(long)
	if valid {
		t.Fatal("report with ❌ should be invalid")
	}
	if utf8.RuneCountInString(reason) > 300 {
		t.Errorf("error reason should be truncated to ~200 chars, got %d runes", utf8.RuneCountInString(reason))
	}
	// 截断后仍以 ❌ 开头：截断的是尾部，不是错误标记本身。
	if !strings.HasPrefix(reason, "❌") {
		t.Errorf("reason should keep the ❌ marker at the head, got %q", reason)
	}
}
