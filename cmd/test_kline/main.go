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
	fmt.Println("=== Testing FetchKLineWithFallback for sh600671 ===")
	result := data.FetchKLineWithFallback("sh600671", "天目药业", "101", 30, "")
	if result == nil || result.Data == nil || len(*result.Data) == 0 {
		fmt.Println("Result: EMPTY")
	} else {
		fmt.Printf("Result: %d records, source=%s\n", len(*result.Data), result.Source)
		for i, k := range *result.Data {
			if i < 3 {
				fmt.Printf("  %s: open=%s close=%s high=%s low=%s\n", k.Day, k.Open, k.Close, k.High, k.Low)
			}
		}
	}
}
