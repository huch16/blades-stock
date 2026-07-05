//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"

	bladesmc "github.com/go-kratos/blades/contrib/mcp"
	bladesTools "github.com/go-kratos/blades/tools"
)

func parseJSON[T any](s string) T {
	var r T
	if s == "" {
		return r
	}
	json.Unmarshal([]byte(s), &r)
	return r
}

func main() {
	logger.InitLogger()
	db.Init("G:\\stock\\blades-stock\\data\\stock.db")

	var servers []models.MCPServer
	db.Dao.Where("enable = ? AND id = ?", true, 7).Find(&servers)
	if len(servers) == 0 {
		fmt.Println("tushareMcp (ID=7) not found or disabled")
		return
	}
	srv := servers[0]
	fmt.Printf("=== tushareMcp Config ===\n")
	fmt.Printf("ID=%d Name=%s URL=%s\n", srv.ID, srv.Name, srv.URL)
	fmt.Printf("Type=%s Headers=%s\n\n", srv.Type, srv.Headers)

	headers := parseJSON[map[string]string](srv.Headers)

	client, err := bladesmc.NewClient(bladesmc.ClientConfig{
		Name:      srv.Name,
		Transport: bladesmc.TransportHTTP,
		Endpoint:  srv.URL,
		Headers:   headers,
		Timeout:   30 * time.Second,
	})
	if err != nil {
		fmt.Printf("NewClient error: %v\n", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Connecting...")
	if err := client.Connect(ctx); err != nil {
		fmt.Printf("Connect error: %v\n", err)
		return
	}
	defer client.Close()

	fmt.Println("Resolving tools...")
	tools, err := client.Resolve(ctx)
	if err != nil {
		fmt.Printf("Resolve error: %v\n", err)
		return
	}

	fmt.Printf("\n=== tushareMcp Tools: Total=%d ===\n\n", len(tools))

	// Group tools
	type group struct {
		cat   string
		tools []bladesTools.Tool
	}
	groupMap := map[string][]bladesTools.Tool{}
	for _, t := range tools {
		cat := categorize(t.Name(), t.Description())
		groupMap[cat] = append(groupMap[cat], t)
	}

	order := []string{
		"📉 行情", "💧 资金流", "🏦 财务", "📈 指数",
		"💰 基金", "🏷️ 概念板块", "👥 股东/龙虎", "📋 公告/新闻",
		"📦 其他",
	}
	catNames := map[string]string{
		"📉 行情":      "日线/分钟/实时行情",
		"💧 资金流":     "主力资金/大单/融资融券",
		"🏦 财务":      "资产负债/利润表/财务指标",
		"📈 指数":      "指数日线/成分股/期货",
		"💰 基金":      "基金净值/持仓/规模",
		"🏷️ 概念板块":   "行业/概念板块/板块资金流",
		"👥 股东/龙虎":  "股东/龙虎榜",
		"📋 公告/新闻":  "公告/新闻/研报",
		"📦 其他":      "",
	}

	for _, cat := range order {
		tlist, ok := groupMap[cat]
		if !ok || len(tlist) == 0 {
			continue
		}
		fmt.Printf("━━━ %s %s ━━━\n", cat, catNames[cat])
		for _, t := range tlist {
			desc := trimDesc(t.Description())
			fmt.Printf("  %-30s  %s\n", t.Name(), desc)
		}
		fmt.Println()
	}

	// Full list
	fmt.Println("=== 完整工具列表 ===")
	for i, t := range tools {
		fmt.Printf("%3d. %-30s  %s\n", i+1, t.Name(), trimDesc(t.Description()))
	}
}

func categorize(name, desc string) string {
	name = strings.ToLower(name)
	desc = strings.ToLower(desc)

	if strings.Contains(desc, "期货") || strings.Contains(desc, "futures") ||
		strings.Contains(desc, "ih ") || strings.Contains(desc, "if ") ||
		strings.Contains(desc, "ic ") || strings.Contains(desc, "im ") ||
		strings.Contains(name, "future") || strings.Contains(name, "index_daily") {
		return "📈 指数"
	}
	if strings.Contains(desc, "期权") || strings.Contains(desc, "option") {
		return "📈 指数"
	}
	if strings.Contains(desc, "基金") || strings.Contains(desc, "fund") || strings.Contains(name, "fund") {
		return "💰 基金"
	}
	if strings.Contains(desc, "资金流向") || strings.Contains(desc, "主力") ||
		strings.Contains(desc, "大单") || strings.Contains(desc, "ddx") ||
		strings.Contains(desc, "融资") && strings.Contains(desc, "融券") {
		return "💧 资金流"
	}
	if strings.Contains(desc, "财务") || strings.Contains(desc, "资产负债") ||
		strings.Contains(desc, "利润") || strings.Contains(desc, "fina") ||
		strings.Contains(name, "balan") || strings.Contains(name, "income") {
		return "🏦 财务"
	}
	if strings.Contains(desc, "日线") || strings.Contains(desc, "daily") ||
		strings.Contains(desc, "weekly") || strings.Contains(desc, "monthly") ||
		strings.Contains(desc, "分钟") || strings.Contains(desc, "实时") ||
		strings.Contains(desc, "分时") || strings.Contains(desc, "tick") ||
		strings.Contains(desc, "当前价") || strings.Contains(desc, "实时行情") {
		return "📉 行情"
	}
	if strings.Contains(desc, "股东") || strings.Contains(desc, "holder") ||
		strings.Contains(desc, "top10") || strings.Contains(desc, "龙虎") {
		return "👥 股东/龙虎"
	}
	if strings.Contains(desc, "公告") || strings.Contains(desc, "news") ||
		strings.Contains(desc, "disclosure") || strings.Contains(desc, "研报") {
		return "📋 公告/新闻"
	}
	if strings.Contains(desc, "概念") || strings.Contains(desc, "板块") ||
		strings.Contains(desc, "行业") && strings.Contains(desc, "资金") {
		return "🏷️ 概念板块"
	}
	return "📦 其他"
}

func trimDesc(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 85 {
		return s[:82] + "..."
	}
	return s
}
