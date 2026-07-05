package main

import (
	"fmt"
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"log"
	"strings"
)

func main() {
	// Initialize database
	db.Init("data/stock.db")

	fmt.Println("=== View Prompt Templates ===")
	fmt.Println()

	// Get all prompt templates
	var templates []models.PromptTemplate
	result := db.Dao.Find(&templates)
	if result.Error != nil {
		log.Fatalf("Error fetching prompt templates: %v", result.Error)
	}

	fmt.Printf("Total prompt templates: %d\n\n", len(templates))

	for i, t := range templates {
		fmt.Printf("[%d] ID=%d, Name=%s\n", i+1, t.ID, t.Name)
		fmt.Printf("  Content (first 200 chars): %s...\n\n", truncateString(t.Content, 200))
	}

	fmt.Println("=== View Completed ===")
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
