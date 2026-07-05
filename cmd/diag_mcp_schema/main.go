//go:build ignore

package main

import (
	"fmt"

	"blades-stock/backend/db"
	"blades-stock/backend/logger"
	"blades-stock/backend/models"
)

func main() {
	logger.InitLogger()
	db.Init("G:\\stock\\blades-stock\\data\\stock.db")

	var servers []models.MCPServer
	db.Dao.Where("enable = ?", true).Find(&servers)
	for _, s := range servers {
		fmt.Printf("ID=%d Name=%s Type=%s URL=%s\n", s.ID, s.Name, s.Type, s.URL)
		fmt.Printf("  Headers: %s\n", s.Headers)
		fmt.Printf("  Env: %s\n", s.Env)
		fmt.Printf("  Args: %s\n", s.Args)
		fmt.Println()
	}
}
