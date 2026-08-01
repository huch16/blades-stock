package main

import (
	"fmt"
	"os"

	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	db.Init("")

	var st struct {
		ID           uint
		TushareToken string `gorm:"column:tushare_token"`
	}
	if err := db.Dao.Table("settings").Last(&st).Error; err != nil {
		fmt.Fprintf(os.Stderr, "读取 settings 失败: %v\n", err)
		os.Exit(1)
	}
	if st.TushareToken == "" {
		fmt.Fprintln(os.Stderr, "settings 中 tushare_token 为空，请先在 设置 里填写 Tushare Token")
		os.Exit(1)
	}

	const name = "Tushare官方MCP"
	url := fmt.Sprintf("https://api.tushare.pro/mcp/?token=%s", st.TushareToken)

	// 记录现有工具的启用状态，TestConnection 会重建工具表（默认全启用），需恢复。
	existingEnabled := map[string]bool{}
	var toolRows []models.MCPServerTool
	if err := db.Dao.Find(&toolRows).Error; err == nil {
		for _, t := range toolRows {
			existingEnabled[t.ToolName] = t.Enabled
		}
	}

	api := data.NewMCPServerApi()

	var existing models.MCPServer
	err := db.Dao.Where("name = ?", name).First(&existing).Error

	if err == nil {
		existing.URL = url
		existing.Type = "streamable-http"
		existing.Enable = true
		existing.Status = "untested"
		if e := api.Update(&existing); e != nil {
			fmt.Fprintf(os.Stderr, "更新 %s 失败: %v\n", name, e)
			os.Exit(1)
		}
		fmt.Printf("[OK] 已更新 %s (id=%d)\n", name, existing.ID)
	} else {
		server := models.MCPServer{
			Name:        name,
			Description: "Tushare 官方 MCP 服务，提供 A股/指数/基金/期货/宏观等数据工具",
			URL:         url,
			Type:        "streamable-http",
			Enable:      true,
			Status:      "untested",
		}
		if e := api.Create(&server); e != nil {
			fmt.Fprintf(os.Stderr, "创建 %s 失败: %v\n", name, e)
			os.Exit(1)
		}
		fmt.Printf("[OK] 已添加 %s (id=%d)\n", name, server.ID)
	}

	servers := api.GetAll()
	var target *models.MCPServer
	for i := range servers {
		if servers[i].Name == name {
			target = &servers[i]
			break
		}
	}
	if target == nil {
		fmt.Fprintln(os.Stderr, "未找到刚写入的 MCP server 记录")
		os.Exit(1)
	}

	msg, err := api.TestConnection(target.ID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "测试连接失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[OK]", msg)

	tools := api.GetToolsByServerID(target.ID)
	fmt.Printf("共保存 %d 个工具到 mcp_server_tools\n", len(tools))

	// 恢复之前记录的启用状态（若工具之前被显式禁用过）
	var restored int
	for _, t := range tools {
		if prev, ok := existingEnabled[t.ToolName]; ok {
			if prev != t.Enabled {
				if err := db.Dao.Model(&models.MCPServerTool{}).
					Where("id = ?", t.ID).
					Update("enabled", prev).Error; err == nil {
					restored++
				}
			}
		}
	}
	if restored > 0 {
		fmt.Printf("已恢复 %d 个工具的历史启用状态\n", restored)
	}

	for _, t := range tools {
		fmt.Printf("  - %s\n", t.ToolName)
	}
}
