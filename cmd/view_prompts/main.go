package main

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"fmt"
	"log"
)

func main() {
	db.Init("data/stock.db")

	fmt.Println("=== Skills (Prompt Templates) ===")
	fmt.Println()

	var skills []models.Skill
	result := db.Dao.Where("enable = ?", true).Find(&skills)
	if result.Error != nil {
		log.Fatalf("Error fetching skills: %v", result.Error)
	}

	fmt.Printf("Total enabled skills: %d\n\n", len(skills))

	for i, s := range skills {
		fmt.Printf("[%d] ID=%d, Name=%s, Category=%s\n", i+1, s.ID, s.Name, s.Category)
		fmt.Printf("  Description: %s\n", truncateString(s.Description, 200))
		if s.SystemPrompt != "" {
			fmt.Printf("  SystemPrompt (first 200 chars): %s...\n", truncateString(s.SystemPrompt, 200))
		}
		fmt.Println()
	}

	fmt.Println("=== Done ===")
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Try to truncate at character boundary
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}
