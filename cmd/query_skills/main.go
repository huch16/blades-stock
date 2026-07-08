package main

import (
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
)

func main() {
	db.Init("data/stock.db")

	var skills []models.Skill
	db.Dao.Find(&skills)
	for _, s := range skills {
		fmt.Printf("===== ID=%d | %s =====\n", s.ID, s.Name)
		fmt.Printf("Category: %s\n", s.Category)
		fmt.Printf("Description: %s\n", s.Description)
		fmt.Printf("Keywords: %s\n", s.TriggerKeywords)
		fmt.Printf("Directive (%d chars):\n%s\n", len(s.Directive), s.Directive)
		fmt.Printf("SystemPrompt (%d chars):\n", len(s.SystemPrompt))
		if len(s.SystemPrompt) > 500 {
			fmt.Printf("%s...\n", s.SystemPrompt[:500])
		} else {
			fmt.Println(s.SystemPrompt)
		}
		fmt.Println()
	}
}
