package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"blades-stock/backend/quant"
)

func main() {
	apiKey := flag.String("key", "", "TickFlow API Key (default: $TICKFLOW_API_KEY)")
	startDate := flag.String("start", "2026-01-05", "回测开始日期")
	endDate := flag.String("end", time.Now().Format("2006-01-02"), "回测结束日期")
	topN := flag.Int("top", 3, "同时持有 ETF 数量")
	momentumDays := flag.Int("days", 21, "动量回看窗口（交易日）")
	initCash := flag.Float64("cash", 100000, "初始资金")
	fees := flag.Float64("fees", 0.8, "单边交易成本（‰）")
	slippage := flag.Float64("slippage", 0.5, "单边滑点（‰）")
	jsonOut := flag.Bool("json", false, "输出 JSON 格式")
	customSymbols := flag.String("symbols", "", "自定义 ETF 代码列表，逗号分隔（如 510300.SH,588000.SH）")
	flag.Parse()

	key := *apiKey
	if key == "" {
		key = os.Getenv("TICKFLOW_API_KEY")
	}
	if key == "" {
		fmt.Println("错误：请通过 -key 参数或 TICKFLOW_API_KEY 环境变量提供 TickFlow API Key")
		os.Exit(1)
	}

	cfg := quant.DefaultETFMomentumConfig()
	cfg.StartDate = *startDate
	cfg.EndDate = *endDate
	cfg.TopN = *topN
	cfg.MomentumDays = *momentumDays
	cfg.InitCash = *initCash
	cfg.Fees = *fees
	cfg.Slippage = *slippage
	cfg.TickFlowAPIKey = key

	if *customSymbols != "" {
		syms := splitAndTrim(*customSymbols)
		cfg.Symbols = syms
		cfg.Names = make([]string, len(syms))
		for i, s := range syms {
			cfg.Names[i] = s
		}
	}

	fmt.Println("========================================")
	fmt.Println(" ETF 动量轮动回测")
	fmt.Println("========================================")
	fmt.Printf("数据源: TickFlow (%d 只 ETF)\n", len(cfg.Symbols))
	fmt.Printf("区间: %s ~ %s\n", cfg.StartDate, cfg.EndDate)
	fmt.Printf("动量窗口: %d 日 | Top %d | 资金: %.0f\n", cfg.MomentumDays, cfg.TopN, cfg.InitCash)
	fmt.Printf("成本: %.1f‰ + %.1f‰ (单边)\n", cfg.Fees, cfg.Slippage)
	fmt.Println()

	result, err := quant.RunETFMomentumBacktest(cfg)
	if err != nil {
		fmt.Printf("回测失败: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		jsonStr, err := result.ToJSON()
		if err != nil {
			fmt.Printf("JSON 序列化失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(jsonStr)
	} else {
		fmt.Print(result.String())
	}
}

func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\uff0c", ",")
	return strings.Split(s, ",")
}
