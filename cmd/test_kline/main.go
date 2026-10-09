package main

import (
	"fmt"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

func main() {
	db.Init("data/stock.db")
	logger.InitLogger()

	for _, pair := range [][2]string{
		{"600671", "天目药业"},
		{"510300", "沪深300ETF"},
		{"159915", "创业板ETF"},
	} {
		code, name := pair[0], pair[1]
		res := data.FetchKLineWithFallback(code, name, "101", 30, "")
		if res != nil && res.Data != nil && len(*res.Data) > 0 {
			last := (*res.Data)[len(*res.Data)-1]
			fmt.Printf("✓ %s (%s): %d bars, src=%s, last=%s open=%s close=%s\n", code, name, len(*res.Data), res.Source, last.Day, last.Open, last.Close)
		} else {
			fmt.Printf("✗ %s (%s): FAILED\n", code, name)
		}
	}
}
