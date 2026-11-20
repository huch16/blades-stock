package main

import (
	"blades-stock/backend/db"
	"blades-stock/backend/models"
	"fmt"
	"log"
	"strings"
)

func main() {
	// Initialize database
	db.Init("data/stock.db")

	fmt.Println("=== Clean Invalid AIResponseResult Records ===")
	fmt.Println()

	// Get all AIResponseResult records where is_del = 0
	var records []models.AIResponseResult
	result := db.Dao.Find(&records)
	if result.Error != nil {
		log.Fatalf("Error fetching records: %v", result.Error)
	}

	fmt.Printf("Total records: %d\n", len(records))

	// Find invalid records (stock code is invalid)
	invalidCount := 0
	validCount := 0

	for _, record := range records {
		stockCode := strings.TrimSpace(record.StockCode)

		// Check if stock code is invalid
		if stockCode == "" ||
			stockCode == "市场资讯" ||
			stockCode == "agent" ||
			!isValidStockCode(stockCode) {

			fmt.Printf("[INVALID] ID=%d, Stock=%s, Question=%s\n",
				record.ID, record.StockCode, record.Question[:min(50, len(record.Question))])
			invalidCount++

			// Soft delete this record (set is_del = 1)
			record.IsDel = 1 // Soft delete flag
			saveResult := db.Dao.Save(&record)
			if saveResult.Error != nil {
				log.Printf("Error soft-deleting record ID=%d: %v", record.ID, saveResult.Error)
			} else {
				fmt.Printf("  -> Soft deleted\n")
			}
		} else {
			validCount++
		}
	}

	fmt.Println()
	fmt.Printf("Invalid records soft-deleted: %d\n", invalidCount)
	fmt.Printf("Valid records remaining: %d\n", validCount)
	fmt.Println()
	fmt.Println("=== Cleanup Completed ===")
	fmt.Println("NOTE: Records are soft-deleted (is_del=1), not permanently deleted.")
}

// isValidStockCode checks if a string looks like a valid stock code
func isValidStockCode(code string) bool {
	// Valid formats: sh600000, sz000001, sh000001, etc.
	if len(code) < 8 {
		return false
	}

	prefix := code[:2]
	if prefix != "sh" && prefix != "sz" {
		return false
	}

	// Remaining should be digits
	for _, c := range code[2:] {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// min returns the smaller of two integers
