//go:build integration

package services

import (
	"strings"
	"testing"
	"time"
)

// TestPreflightDialFailsFast 依赖真实网络行为（TEST-NET-1 地址 192.0.2.1
// 须真实不可达才通过；沙箱网络会接受任意 TCP，导致本测试误失败）。
// 按 AGENTS.md 约定打 integration 标签：默认 `go test -skip integration`
// 不跑，`make test-integration`（-tags=integration）才跑。断言一字未改。
// R-02 从 market_report_async_test.go 原样移入本文件。
// 跳过的是环境依赖，不是缺陷；在有网 CI 仍须跑。

// 契约五：preflightDial 必须对不可解析的地址立即给出可读原因，
// 且对黑洞地址在 4 秒内失败（而不是挂满整个测试预算）。
func TestPreflightDialFailsFast(t *testing.T) {
	if msg := preflightDial("not a url"); msg == "" {
		t.Error("无法解析的地址必须返回原因")
	}
	start := time.Now()
	msg := preflightDial("http://192.0.2.1:8080/v1") // TEST-NET-1，不可路由
	elapsed := time.Since(start)
	if msg == "" {
		t.Error("黑洞地址必须返回失败原因")
	}
	if elapsed > 6*time.Second {
		t.Errorf("预检必须在 4s 级完成，实际 %v", elapsed)
	}
	if !strings.Contains(msg, "超时") && !strings.Contains(msg, "无法连接") {
		t.Errorf("原因应明确指出超时或无法连接，实际 %q", msg)
	}
}
