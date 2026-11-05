package main

import (
	"fmt"
	"os"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	// 初始化 DB（相对路径 data/stock.db）
	db.Init("")

	// 找到悟道 MCP 服务器的 ID
	var server models.MCPServer
	if err := db.Dao.Where("name = ?", "悟道数据").First(&server).Error; err != nil {
		fmt.Fprintf(os.Stderr, "未找到悟道数据服务器: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("悟道数据服务器 ID = %d\n", server.ID)

	// 基础工具白名单（约 27 条，您可以自行增删）
	keepTools := map[string]bool{
		"stock_search":                 true,
		"kline":                        true,
		"minute_data":                  true,
		"stock_rank":                   true,
		"market_overview":              true,
		"trading_calendar":             true,
		"index_market":                 true,
		"etf_market":                   true,
		"convertible_bond_market":      true,
		"intraday_main_flow":           true,
		"short_term_emotion":           true,
		"news_hotlist":                 true,
		"cls_news":                     true,
		"official_announcements":       true,
		"official_interactions":        true,
		"official_disclosure_evidence": true,
		"watchlist_list":               true,
		"watchlist_group_list":         true,
		"watchlist_group_create":       true,
		"watchlist_group_rename":       true,
		"watchlist_group_delete":       true,
		"watchlist_add":                true,
		"watchlist_remove":             true,
		"watchlist_update_stock":       true,
	}

	// 删除不在白名单中的工具记录
	if err := db.Dao.
		Where("mcp_server_id = ?", server.ID).
		Not("tool_name IN ?", keys(keepTools)).
		Delete(&models.MCPServerTool{}).Error; err != nil {
		fmt.Fprintf(os.Stderr, "删除多余工具失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("已删除除白名单之外的工具记录")

	// 查询剩余工具，将其打印出来供您确认
	var remaining []models.MCPServerTool
	db.Dao.Where("mcp_server_id = ?", server.ID).Find(&remaining)
	fmt.Printf("当前保留的工具数量: %d\n", len(remaining))
	for _, t := range remaining {
		fmt.Println(" -", t.ToolName)
	}
}

// 辅助函数：返回 map 键的切片
func keys(m map[string]bool) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
