package tools

import (
	"strings"
	"testing"
)

func TestTruncateToolResult_Short(t *testing.T) {
	short := "12345"
	got := truncateToolResult("TestTool", short)
	if got != short {
		t.Errorf("short result should pass through unchanged, got %q", got)
	}
}

func TestTruncateToolResult_ExactlyAtLimit(t *testing.T) {
	s := strings.Repeat("a", maxToolResultLen)
	got := truncateToolResult("TestTool", s)
	if got != s {
		t.Errorf("result exactly at limit should pass through, got len=%d", len(got))
	}
}

func TestTruncateToolResult_LongKeepsHeadAndTail(t *testing.T) {
	long := strings.Repeat("头", maxToolResultLen*2)
	got := truncateToolResult("TestTool", long)
	if len(got) >= len(long) {
		t.Fatalf("long result should be truncated, got len=%d vs original=%d", len(got), len(long))
	}
	if !strings.Contains(got, "中间已截断") {
		t.Errorf("truncated result should contain marker")
	}
	// 保留尾部最新数据
	if !strings.HasSuffix(got, "头") {
		t.Errorf("truncated result should keep tail data")
	}
	// 头部保留表头上下文
	if !strings.HasPrefix(got, "头") {
		t.Errorf("truncated result should keep head context")
	}
}

func TestTruncateToolResult_UTF8Safe(t *testing.T) {
	long := strings.Repeat("中", maxToolResultLen*2+7)
	got := truncateToolResult("TestTool", long)
	for _, r := range got {
		if r == '\uFFFD' {
			t.Errorf("truncated result contains replacement char (UTF-8 mid-rune corruption)")
		}
	}
}

func TestTruncateToolResult_ChineseLongerThanBytes(t *testing.T) {
	// 中文按 rune 截断，字符数超限但字节数未超时不应触发
	long := strings.Repeat("中", maxToolResultLen*2)
	got := truncateToolResult("TestTool", long)
	if len(got) >= len(long) {
		t.Fatalf("multi-byte runes must still truncate on rune count, got len=%d", len(got))
	}
}
