//go:build ignore

package main

import (
	"fmt"
	"os"
	"time"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

func main() {
	db.Init("")
	logger.InitLogger()

	codes := os.Args[1:]

	strategies, err := data.GetAvailableStrategies()
	if err != nil {
		fmt.Printf("[FAIL] 加载策略失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已加载 %d 个策略:", len(strategies))
	for _, s := range strategies {
		fmt.Printf(" %s", s.Name)
	}
	fmt.Println()

	start := time.Now()
	outcome, err := data.ScreenByStrategy("趋势跟随", codes, 10)
	if err != nil {
		fmt.Printf("[FAIL] ScreenByStrategy: %v\n", err)
		os.Exit(1)
	}
	elapsed := time.Since(start)

	fmt.Printf("\n=== 策略选股结果: %s ===\n", outcome.StrategyName)
	fmt.Printf("分类: %s | 基础分: %.0f | 风险: %s | 持仓周期: %s\n", outcome.Category, outcome.BaseScore, outcome.RiskLevel, outcome.HoldPeriod)
	fmt.Printf("判断标准: %s\n", outcome.Criteria)
	fmt.Printf("扫描股票数: %d | 耗时: %v\n", outcome.ScannedCount, elapsed.Round(time.Millisecond))

	if len(outcome.Results) == 0 {
		fmt.Println("无命中结果")
		return
	}
	fmt.Printf("%-8s %-10s %-6s %-8s %-8s %-8s %s\n", "代码", "名称", "评分", "收盘", "涨跌幅", "量比", "命中条件")
	for _, r := range outcome.Results {
		fmt.Printf("%-8s %-10s %-6.1f %-8s %-8s %-8s %s\n", r.Code, r.Name, r.Score, r.Close, r.ChangePct, r.VolumeRatio, r.Reason)
	}
}
