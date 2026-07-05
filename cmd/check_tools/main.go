package main

import (
    "fmt"
    "os"
    "blades-stock/backend/db"
    "blades-stock/backend/models"
)

func main() {
    db.Init("")
    var tools []models.MCPServerTool
    if err := db.Dao.Find(&tools).Error; err != nil {
        fmt.Fprintf(os.Stderr, "查询 tools 失败: %v\n", err)
        os.Exit(1)
    }
    fmt.Printf("共 %d 条工具记录\n", len(tools))
    for _, t := range tools {
        fmt.Printf("- %s\n", t.ToolName)
    }
}
