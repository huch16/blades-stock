package main

import (
	"encoding/json"
	"fmt"
	"os"

	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	db.Init("")

	tushareToken := os.Getenv("TUSHARE_TOKEN")
	if tushareToken == "" {
		fmt.Fprintln(os.Stderr, "错误: 请设置环境变量 TUSHARE_TOKEN")
		os.Exit(1)
	}

	wudaoBearer := os.Getenv("WUDAO_BEARER_TOKEN")
	if wudaoBearer == "" {
		fmt.Fprintln(os.Stderr, "错误: 请设置环境变量 WUDAO_BEARER_TOKEN")
		os.Exit(1)
	}

	wudaoHeaders, _ := json.Marshal(map[string]string{
		"Authorization": "Bearer " + wudaoBearer,
	})

	var wdCount int64
	if err := db.Dao.Model(&models.MCPServer{}).Where("name = ?", "悟道数据").Count(&wdCount).Error; err != nil {
		fmt.Fprintf(os.Stderr, "查询 悟道数据 失败: %v\n", err)
		os.Exit(1)
	}

	if wdCount > 0 {
		if err := db.Dao.Model(&models.MCPServer{}).Where("name = ?", "悟道数据").Updates(map[string]any{
			"url":         "https://stock.quicktiny.cn/api/mcp",
			"headers":     string(wudaoHeaders),
			"type":        "",
			"enable":      true,
			"status":      "untested",
			"description": "悟道A股MCP数据服务，提供股票行情、财务数据、市场分析等工具",
		}).Error; err != nil {
			fmt.Fprintf(os.Stderr, "更新 悟道数据 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[OK] 悟道数据 已更新 URL/Headers")
	} else {
		server := models.MCPServer{
			Name:        "悟道数据",
			Description: "悟道A股MCP数据服务，提供股票行情、财务数据、市场分析等工具",
			URL:         "https://stock.quicktiny.cn/api/mcp",
			Headers:     string(wudaoHeaders),
			Type:        "",
			Enable:      true,
			Status:      "untested",
		}
		if err := db.Dao.Create(&server).Error; err != nil {
			fmt.Fprintf(os.Stderr, "创建 悟道数据 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[OK] 悟道数据 已添加，ID=%d\n", server.ID)
	}

	env, _ := json.Marshal(map[string]string{
		"TUSHARE_TOKEN": tushareToken,
	})

	var sfCount int64
	if err := db.Dao.Model(&models.MCPServer{}).Where("name = ?", "smart-financial-mcp").Count(&sfCount).Error; err != nil {
		fmt.Fprintf(os.Stderr, "查询 smart-financial-mcp 失败: %v\n", err)
		os.Exit(1)
	}

	if sfCount > 0 {
		if err := db.Dao.Model(&models.MCPServer{}).Where("name = ?", "smart-financial-mcp").Updates(map[string]any{
			"env":    string(env),
			"status": "available",
		}).Error; err != nil {
			fmt.Fprintf(os.Stderr, "更新 smart-financial-mcp 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[OK] smart-financial-mcp 已更新 Token")
	} else {
		server := models.MCPServer{
			Name:        "smart-financial-mcp",
			Description: "基于 Tushare Pro 的智能股票数据 MCP 服务器",
			Type:        "stdio",
			Command:     "uvx",
			Args:        `["smart-financial-mcp"]`,
			Env:         string(env),
			Enable:      true,
			Status:      "available",
		}
		if err := db.Dao.Create(&server).Error; err != nil {
			fmt.Fprintf(os.Stderr, "创建 smart-financial-mcp 失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[OK] smart-financial-mcp 已添加，ID=%d\n", server.ID)
	}

	fmt.Println("\n配置完成！重启 blades-stock 后生效")
	fmt.Println("提示: 启动后在 研究中心 → MCP服务 中点击 \"测试\" 验证连接")
}
