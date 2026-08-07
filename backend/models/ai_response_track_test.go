package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestIsStockReportCode 校验可回测个股代码判定：
// 6 位数字（A股/基金）、sh/sz/bj 前缀、5 位港股、大写美股为真；
// "市场分析"/"agent"/空串等泛化报告伪代码为假。
func TestIsStockReportCode(t *testing.T) {
	cases := []struct {
		code string
		want bool
	}{
		{"601868", true},
		{"000001", true},
		{"510300", true},
		{"sh600000", true},
		{"SZ000001", true},
		{"bj430047", true},
		{"sh601868", true},
		{"00700", true},
		{"09988", true},
		{"AAPL", true},
		{"MSFT", true},
		{"TSLA", true},
		{" 601868 ", true},
		{"agent", false},
		{"市场分析", false},
		{"", false},
		{"   ", false},
		{"GB_TSLA", false},
		{"00700.HK", false},
		{"AAPL.US", false},
	}
	for _, c := range cases {
		if got := IsStockReportCode(c.code); got != c.want {
			t.Errorf("IsStockReportCode(%q) = %v, want %v", c.code, got, c.want)
		}
	}
}

// TestStockReportCodeSQLMatchesPredicate 用内存 SQLite 校验 SQL 过滤条件
// 与 Go 判定函数语义一致，防止 Go/SQL 两侧口径漂移。
func TestStockReportCodeSQLMatchesPredicate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&AIResponseResult{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	codes := []string{
		"601868", "000001", "510300", "sh600000", "SZ000001", "bj430047",
		"00700", "09988", "AAPL", "MSFT",
		"agent", "市场分析", "", "GB_TSLA", "00700.HK", "AAPL.US",
	}
	for i, code := range codes {
		if err := db.Create(&AIResponseResult{
			StockCode: code,
			StockName: "n" + string(rune('a'+i)),
			Content:   "c",
		}).Error; err != nil {
			t.Fatalf("insert %q: %v", code, err)
		}
	}

	var got []AIResponseResult
	if err := db.Where(StockReportCodeSQL()).Find(&got).Error; err != nil {
		t.Fatalf("query: %v", err)
	}

	gotSet := make(map[string]bool)
	for _, r := range got {
		gotSet[r.StockCode] = true
	}

	// 期望集合 = Go 谓词判为真的所有已插入代码
	wantSet := make(map[string]bool)
	for _, code := range codes {
		if IsStockReportCode(code) {
			wantSet[code] = true
		}
	}

	if len(gotSet) != len(wantSet) {
		t.Fatalf("SQL 命中 %d 条, Go 谓词期望 %d 条\nSQL 命中: %v\n期望: %v",
			len(gotSet), len(wantSet), gotSet, wantSet)
	}
	for code := range wantSet {
		if !gotSet[code] {
			t.Errorf("SQL 漏掉应命中的代码: %q", code)
		}
	}
	for code := range gotSet {
		if !wantSet[code] {
			t.Errorf("SQL 多命中不应命中的代码: %q", code)
		}
	}
}
