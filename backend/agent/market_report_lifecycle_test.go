package agent

import (
	"strings"
	"testing"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

// 固化 v3.0.0 修复：市场智能研报的**遥测污染**与**删除/读取不一致**。
//
// 事故背景（用户截图实证）：
//
//	报告位置只剩一行
//	  “[STEP]🔄 容灾切换：主模型 [gemini-3.8-flash] 异常(…429 Token pool is empty…)”
//	正文为空，标题下方写着「历史智能分析报告存档」、来源为模型名。
//	同时用户反馈：新的分析生不出来、旧的报告也删不掉。
//
// 两个独立缺陷叠加：
//  1. [STEP] 是**遥测行**，但清理逻辑只过滤 [STATUS]，于是它被当成推理内容
//     拼进了最终报告（isValidMarketReport 也因“字符数够”而放行）。
//  2. 归档兜底读取用「chat_id LIKE %x% OR stock_name LIKE %y%」双条件，
//     而删除只用 chat_id 前缀 → 删掉之后读回来又在，且会跨类型串号
//     （ETF轮动决策 tab 拿到的是「ETF轮动盘前内参」）。

func resetReportState() {
	ClearReport("")
	if db.Dao != nil {
		db.Dao.Unscoped().Where("1 = 1").Delete(&models.AIResponseResult{})
	}
}

// 契约一：遥测行必须被识别并从正文中剥离。
func TestStripTelemetryLines(t *testing.T) {
	raw := strings.Join([]string{
		"### 一、隔夜外盘",
		"[STATUS]🛡️ 正在执行纯 Go 确定性扫描...",
		"美股三大指数集体收涨。",
		"[STEP]🔄 容灾切换：主模型 [gemini-3.8-flash] 异常(429 Token pool is empty)，已切换为 [minimax]",
		"[DONE]",
		"结论：维持中性偏积极。",
	}, "\n")
	got := stripTelemetryLines(raw)
	for _, bad := range []string{"[STATUS]", "[STEP]", "[DONE]", "容灾切换"} {
		if strings.Contains(got, bad) {
			t.Errorf("遥测内容 %q 未被剥离:\n%s", bad, got)
		}
	}
	for _, keep := range []string{"隔夜外盘", "集体收涨", "中性偏积极"} {
		if !strings.Contains(got, keep) {
			t.Errorf("正文内容 %q 被误删:\n%s", keep, got)
		}
	}
}

// 契约二：只有遥测行、没有正文的报告必须判定为**无效**（旧实现会放行）。
func TestIsValidMarketReportRejectsTelemetryOnly(t *testing.T) {
	telemetryOnly := "[STEP]🔄 容灾切换：主模型 [gemini-3.8-flash] 异常(网关限流或额度耗尽 (HTTP 429: 【网关调度受限】Token pool is empty))，已无缝切换为备用模型 [minimaxai/minimax-m3.1-flash]。"
	if valid, reason := isValidMarketReport(telemetryOnly); valid {
		t.Fatalf("只有遥测行的报告必须判为无效（否则会被当成有效研报展示）。reason=%q", reason)
	}

	// 网关限流类字样同样必须拦截
	for _, marker := range []string{
		"Token pool is empty",
		"网关限流",
		"账号池",
		"额度耗尽",
		"容灾切换",
	} {
		if valid, _ := isValidMarketReport(strings.Repeat("内容 ", 60) + marker); valid {
			t.Errorf("含网关/降级标记 %q 的报告必须判为无效", marker)
		}
	}

	// 真正的研报必须仍然放行（防止“一律拒绝”式的假修复）
	good := strings.Repeat("### 隔夜外盘与全球资产联动\n美股三大指数集体收涨，中概金龙指数上行。\n", 6)
	if valid, reason := isValidMarketReport(good); !valid {
		t.Fatalf("正常研报被误判为无效: %q", reason)
	}
}

// 契约三：读取与删除必须使用同一套归档归属判定，且不得跨类型串号。
func TestReportArchiveReadClearConsistency(t *testing.T) {
	resetReportState()

	// 造两条归档：① ETF轮动**决策** ② 盘前 ETF 内参（stock_name 同样含“ETF轮动”）
	rows := []models.AIResponseResult{
		{
			ChatId:    "etf_rotation_2026-11-07",
			ModelName: "gemini-3.8-flash",
			StockCode: "510300.SH",
			StockName: "ETF轮动量化决策",
			Content:   strings.Repeat("### ETF轮动决策正文\n动量矩阵与投委会评分。\n", 6),
		},
		{
			ChatId:    "pre_market_etf_2026-11-07",
			ModelName: "gemini-3.8-flash",
			StockCode: "510300.SH",
			StockName: "ETF轮动盘前内参",
			Content:   strings.Repeat("### 盘前ETF内参正文\n隔夜外盘与竞价风向。\n", 6),
		},
	}
	for i := range rows {
		rows[i].CreatedAt = time.Now()
		if err := db.Dao.Create(&rows[i]).Error; err != nil {
			t.Fatalf("写入归档失败: %v", err)
		}
	}

	svc := NewMarketLifecycleService(nil)

	// ① 跨类型串号防护：ETF轮动决策 tab 不得拿到「盘前ETF内参」
	dec := svc.GetLatestReportByType("etf_rotation_decision")
	if dec == nil {
		t.Fatal("应能读到 ETF轮动决策的归档")
	}
	if strings.Contains(dec.Title, "盘前") {
		t.Fatalf("跨类型串号：ETF轮动决策读到了盘前报告，title=%q", dec.Title)
	}

	// ② 删除后必须真的读不到（旧实现会被 stock_name 条件捞回来）
	if got := ClearReportAndArchive("etf_rotation_decision"); !strings.Contains(got, "已清除") {
		t.Fatalf("清除返回异常: %q", got)
	}
	if again := svc.GetLatestReportByType("etf_rotation_decision"); again != nil {
		t.Fatalf("清除后仍能读到 ETF轮动决策归档（删除与读取判定不一致）: title=%q", again.Title)
	}

	// ③ 其它类型不得被误删
	if pre := svc.GetLatestReportByType("pre_market_briefing"); pre == nil {
		t.Fatal("清除 etf_rotation_decision 时误删了 pre_market_briefing")
	}
}

// 契约四：ClearReportAndArchive("") 清空全部内存缓存与全部研报类归档。
func TestClearAllReports(t *testing.T) {
	resetReportState()

	for _, c := range []struct{ chatID, name string }{
		{"pre_market_etf_2026-11-07", "ETF轮动盘前内参"},
		{"post_market_2026-11-07", "盘后复盘"},
		{"etf_rotation_2026-11-07", "ETF轮动量化决策"},
		{"serenity_2026-11-07_512480.SH", "Serenity-半导体"},
	} {
		r := models.AIResponseResult{
			ChatId: c.chatID, StockName: c.name,
			Content: strings.Repeat("### 研报正文\n客观数据与结论。\n", 6),
		}
		r.CreatedAt = time.Now()
		if err := db.Dao.Create(&r).Error; err != nil {
			t.Fatalf("写入归档失败: %v", err)
		}
	}

	if got := ClearReportAndArchive(""); !strings.Contains(got, "已清除全部") {
		t.Fatalf("全量清除返回异常: %q", got)
	}
	svc := NewMarketLifecycleService(nil)
	for _, t2 := range []string{
		"pre_market_briefing", "post_market_review",
		"etf_rotation_decision", "serenity_industry_analysis",
	} {
		if r := svc.GetLatestReportByType(t2); r != nil {
			t.Errorf("全量清除后 %s 仍能读到报告: title=%q", t2, r.Title)
		}
	}
	var remaining int64
	db.Dao.Unscoped().Model(&models.AIResponseResult{}).
		Where("chat_id LIKE 'pre_market_%' OR chat_id LIKE 'post_market_%' OR chat_id LIKE 'etf_rotation_%' OR chat_id LIKE 'serenity_%'").
		Count(&remaining)
	if remaining != 0 {
		t.Errorf("全量清除后归档仍残留 %d 条研报记录", remaining)
	}
}

// 契约五：ClearReport 的返回值必须如实反映“清了几份”，且重复清除不报错。
func TestClearReportIsIdempotent(t *testing.T) {
	resetReportState()
	cacheReport(&MarketIntelligenceReport{Type: "etf_rotation_decision", Content: strings.Repeat("正文 ", 50)})
	if n := ClearReport("etf_rotation_decision"); n != 1 {
		t.Fatalf("首次清除应返回 1，实际 %d", n)
	}
	if n := ClearReport("etf_rotation_decision"); n != 0 {
		t.Fatalf("重复清除应返回 0，实际 %d", n)
	}
	if n := ClearReport(""); n != 0 {
		t.Fatalf("空缓存全量清除应返回 0，实际 %d", n)
	}
}
