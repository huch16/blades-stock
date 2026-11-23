package data

import (
	"strings"
	"testing"

	"blades-stock/backend/db"
	"blades-stock/backend/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestResolveStrategyScreenName(t *testing.T) {
	// 回归测试：nameMap 未命中时绝不能把代码本身当作名称
	// （曾导致 601318 以名称“601318”混入选股/分析结果）。
	nameMap := map[string]string{
		"600519": "贵州茅台",
		"000001": "平安银行",
		"601318": "",       // DB 行存在但名称空白
		"600036": "600036", // 名称本身就是代码（脏数据）
	}
	cases := []struct {
		code string
		want string
	}{
		{"600519", "贵州茅台"},
		{"000001", "平安银行"},
		{"601318", "未知"}, // 名称空白 -> 标记未知，不回退代码
		{"600036", "未知"}, // 名称==代码 -> 标记未知
		{"999999", "未知"}, // 列表里没有的代码 -> 标记未知
		{"300750", "未知"}, // 兜底池里可能出现但 nameMap 为空的代码
	}
	for _, tc := range cases {
		if got := resolveStrategyScreenName(nameMap, tc.code); got != tc.want {
			t.Errorf("resolveStrategyScreenName(%q) = %q, want %q", tc.code, got, tc.want)
		}
	}
	// nil map 也不应 panic，同样标记未知。
	if got := resolveStrategyScreenName(nil, "601318"); got != "未知" {
		t.Errorf("resolveStrategyScreenName(nil, 601318) = %q, want 未知", got)
	}
	// 名称含空白时应 trim 后返回。
	padded := map[string]string{"601318": "  中国平安  "}
	if got := resolveStrategyScreenName(padded, "601318"); got != "中国平安" {
		t.Errorf("resolveStrategyScreenName(padded) = %q, want 中国平安", got)
	}
}

func TestFallbackScreenByIndicators(t *testing.T) {
	// 测试空输入
	if _, ok := FallbackScreenByIndicators(""); ok {
		t.Errorf("expected false for empty query, got true")
	}

	// 测试量化策略匹配意图（多头、突破、反弹等）
	res, ok := FallbackScreenByIndicators("请帮我筛选均线多头排列的股票")
	// 无论本地是否有足够K线数据计算，FallbackScreenByIndicators 都不应发生 panic
	t.Logf("FallbackScreenByIndicators (多头): ok=%v, res_len=%d", ok, len(res))

	res2, ok2 := FallbackScreenByIndicators("放量突破")
	t.Logf("FallbackScreenByIndicators (放量突破): ok=%v, res_len=%d", ok2, len(res2))

	res3, ok3 := FallbackScreenByIndicators("半导体")
	t.Logf("FallbackScreenByIndicators (半导体行业/概念): ok=%v, res_len=%d", ok3, len(res3))
}

// TestBuildStrategyUniverse_MissingSecUriTycodeLogsError 回归测试：
// buildStrategyUniverse 的选股池查询用 sec_uri_tycode 排序；该列是
// GORM 默认命名把 SECURITYCODE 字段映射出来的，新迁移的库本就有它，
// 但老结构遗留的生产库可能没有。旧代码不查 Find 错误，失败时无声
// 落入 24 只兜底池且无任何日志（第三轮遗留风险 #2）。修复后：回落
// 行为保留，但失败必须记一条 Error 日志。
// （隔离内存库 + 替换包级日志函数变量，不动包内 TestMain 的临时库；
// 注意：不要用 zaptest/observer 交换全局 logger.SugaredLogger 来捕获
// 日志——TestFallbackScreenByIndicators 遗留的 K 线超时孤儿 goroutine
// 仍在后台读该全局，交换会构成 DATA RACE，见 logStrategyScreenPoolError
// 上的注释。）
func TestBuildStrategyUniverse_MissingSecUriTycodeLogsError(t *testing.T) {
	memDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	// 按当前结构建表（GORM 默认命名把 SECURITYCODE 映射成
	// sec_uri_tycode，所以新迁移的库本就有该列），再删掉该列，模拟
	// 老结构遗留的生产库——正是第三轮遗留风险 #2 的场景。
	if err := memDB.AutoMigrate(&models.AllStockInfo{}); err != nil {
		t.Fatalf("AutoMigrate AllStockInfo: %v", err)
	}
	if err := memDB.Create(&models.AllStockInfo{
		SECUCODE: "600519", SECURITYCODE: "600519",
		SECURITYNAMEABBR: "贵州茅台", MAXTRADEDATE: "2026-11-20",
	}).Error; err != nil {
		t.Fatalf("seed AllStockInfo: %v", err)
	}
	if err := memDB.Exec("DROP INDEX IF EXISTS idx_all_stock_info_sec_uri_tycode").Error; err != nil {
		t.Fatalf("drop sec_uri_tycode index: %v", err)
	}
	if err := memDB.Exec("ALTER TABLE all_stock_info DROP COLUMN sec_uri_tycode").Error; err != nil {
		t.Fatalf("drop sec_uri_tycode column: %v", err)
	}
	origDao := db.Dao
	db.Dao = memDB
	t.Cleanup(func() { db.Dao = origDao })

	// 捕获池查询失败的日志：替换包级函数变量（唯一读者是同步调用的
	// buildStrategyUniverse，不与后台 goroutine 冲突）。
	var logged []string
	origLog := logStrategyScreenPoolError
	logStrategyScreenPoolError = func(err error) { logged = append(logged, err.Error()) }
	t.Cleanup(func() { logStrategyScreenPoolError = origLog })

	codes, _ := buildStrategyUniverse(nil)

	// 回落行为保留：返回的应正是 24 只兜底池。
	fb := fallbackUniverse()
	if len(codes) != len(fb) {
		t.Fatalf("buildStrategyUniverse(nil) returned %d codes, want fallback %d: %v", len(codes), len(fb), codes)
	}
	for i := range fb {
		if codes[i] != fb[i] {
			t.Fatalf("codes[%d] = %q, want fallback %q", i, codes[i], fb[i])
		}
	}
	// 关键断言：缺列失败必须记 Error 日志，不能静默。
	found := false
	for _, msg := range logged {
		if strings.Contains(msg, "sec_uri_tycode") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected a pool-error log mentioning sec_uri_tycode, got %v", logged)
	}
}
