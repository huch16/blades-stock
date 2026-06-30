package main

import (
	"encoding/json"
	"fmt"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	db.Init("")

	tushareToken := "dca1afb47b186d365ea451cd07e61806ea3d98a82abfc7839a568dbd"

	// 更新 悟道数据 -> available
	db.Dao.Model(&models.MCPServer{}).Where("name = ?", "悟道数据").Update("status", "available")

	// 配置 smart-financial-mcp
	env, _ := json.Marshal(map[string]string{
		"TUSHARE_TOKEN": tushareToken,
	})

	var count int64
	db.Dao.Model(&models.MCPServer{}).Where("name = ?", "smart-financial-mcp").Count(&count)
	if count > 0 {
		db.Dao.Model(&models.MCPServer{}).Where("name = ?", "smart-financial-mcp").Updates(map[string]any{
			"env":    string(env),
			"status": "available",
		})
		fmt.Println("smart-financial-mcp 已更新 Token")
	} else {
		server := models.MCPServer{
			Name:        "smart-financial-mcp",
			Description: "基于 Tushare Pro 的智能股票数据 MCP 服务器，提供行情、财务、指标等数据查询",
			Type:        "stdio",
			Command:     "uvx",
			Args:        `["smart-financial-mcp"]`,
			Env:         string(env),
			Enable:      true,
			Status:      "available",
		}
		if err := db.Dao.Create(&server).Error; err != nil {
			fmt.Printf("创建 smart-financial-mcp 失败: %v\n", err)
			return
		}
		fmt.Printf("smart-financial-mcp 已添加，ID=%d\n", server.ID)
	}

	fmt.Println("配置完成！重启 blades-stock 后生效")
}
