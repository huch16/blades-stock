//go:build ignore

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"blades-stock/backend/agent/tools"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

func main() {
	db.Init("data/stock.db")
	logger.InitLogger()

	all := tools.GetAllDataTools()
	found := false
	for _, t := range all {
		if t.Name() == "GetETFMomentumRanking" {
			found = true
		}
	}
	fmt.Printf("GetETFMomentumRanking in GetAllDataTools: %v (total=%d)\n", found, len(all))

	g := tools.ParseToolGroups("stock_analysis,money_flow,news_research,market")
	filtered := tools.FilterToolsByGroups(all, g)
	fmt.Printf("after skill group filter: total=%d\n", len(filtered))
	has := false
	for _, t := range filtered {
		if t.Name() == "GetETFMomentumRanking" {
			has = true
		}
	}
	fmt.Printf("GetETFMomentumRanking survives filter: %v\n", has)

	out, err := tools.GetETFMomentumRankingTool().Handle(context.Background(), `{"count":25}`)
	if err != nil {
		fmt.Printf("tool err: %v\n", err)
	} else {
		fmt.Printf("tool output (%d chars):\n%s\n", len(out), out)
	}
	return

	st := data.NewSettingsApi().Config
	fmt.Printf("qgqp_b_id set=%v\n", st.QgqpBId != "")

	etfCount := int64(0)
	db.Dao.Model(&models.AllStockInfo{}).Where("SECURITYNAMEABBR LIKE ?", "%ETF%").Count(&etfCount)
	fmt.Printf("all_stock_info 含 ETF 名称条数=%d\n", etfCount)

	var etfs []models.AllStockInfo
	db.Dao.Where("SECURITYNAMEABBR LIKE ?", "%ETF%").Limit(15).Find(&etfs)
	for _, e := range etfs {
		fmt.Printf("  ETF %s %s market=%s date=%s\n", e.SECUCODE, e.SECURITYNAMEABBR, e.MARKET, e.MAXTRADEDATE)
	}

	for _, kw := range []string{"云计算", "红利", "科技"} {
		res := data.NewSearchStockApi(kw).SearchETF(10)
		b, _ := json.Marshal(res)
		s := string(b)
		if len(s) > 300 {
			s = s[:300] + "..."
		}
		fmt.Printf("SearchETF(%q): %s\n", kw, s)
	}

	codes := []string{
		"sh510300", "sh510500", "sh512100", "sh515000", "sh515880", "sh512880",
		"sh510050", "sh512000", "sh515030", "sh588000", "sh562500", "sh563360",
		"sh513100", "sh510880", "sh512890", "sh516160", "sz159915", "sz159919",
		"sz159920", "sz159527", "sz159725", "sz159516", "sz159992", "sz159852",
		"sz159869", "sz159998", "sz159865", "sz159938", "sz159995", "sz159605",
	}
	for _, code := range codes {
		res := data.FetchKLineWithFallback(code, "probe", "101", 30, "")
		if res == nil || res.Data == nil || len(*res.Data) < 22 {
			fmt.Printf("FAIL code=%-10s n=0\n", code)
			continue
		}
		d := *res.Data
		last := d[len(d)-1]
		close0, _ := strconv.ParseFloat(last.Close, 64)
		weighted := 0.0
		wsum := 0.0
		for i := 0; i <= 20; i++ {
			idx := len(d) - 1 - i
			c, _ := strconv.ParseFloat(d[idx].Close, 64)
			w := math.Pow(0.5, float64(i)/7.0)
			weighted += c * w
			wsum += w
		}
		base := 0.0
		if len(d) > 21 {
			base, _ = strconv.ParseFloat(d[len(d)-22].Close, 64)
		}
		mom := 0.0
		if base > 0 {
			mom = (close0 - base) / base * 100
		}
		wavg := weighted / wsum
		wmom := 0.0
		if base > 0 {
			wmom = (wavg - base) / base * 100
		}
		fmt.Printf("%-10s %-8s last=%s lastClose=%-6s base21=%-6.4f wMom21=%6.2f%% mom21=%6.2f%%\n",
			code, res.Source, last.Day, last.Close, base, wmom, mom)
	}
}
