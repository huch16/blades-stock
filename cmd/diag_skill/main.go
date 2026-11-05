package main

import (
	"blades-stock/backend/db"
	"fmt"
)

type AIConfig struct {
	ID        uint
	Name      string
	MaxTokens int
}

func main() {
	db.Init("data/stock.db")
	fmt.Println("=== AI 配置 MaxTokens 验证 ===")
	var cfgs []AIConfig
	db.Dao.Table("ai_config").Find(&cfgs)
	for _, c := range cfgs {
		safeMax := c.MaxTokens
		if safeMax <= 0 {
			safeMax = 120000
		}
		avail := int(float64(safeMax) * 0.85)
		maxInput := avail - 20000
		if maxInput < 4000 {
			maxInput = 4000
		}
		hist := maxInput - 6900
		status := "❌"
		if hist >= 8000 {
			status = "✅"
		} else if hist >= 0 {
			status = "⚠️"
		}
		fmt.Printf("%s ID=%-2d %-30s MaxTokens=%-7d maxInput=%-5d history=%-5d tok\n",
			status, c.ID, c.Name, c.MaxTokens, maxInput, hist)
	}
}
