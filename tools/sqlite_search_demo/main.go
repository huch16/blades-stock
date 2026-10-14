//go:build ignore

package main

import (
	"context"
	"fmt"
	"log"

	"blades-stock/backend/agent"
	"blades-stock/backend/data"
	"blades-stock/backend/db"
	"blades-stock/backend/logger"
)

func main() {
	// Initialize DB
	db.Init("")

	// Create SQLite retriever
	sqlPath := "data/vectors.db"
	retr, err := agent.NewSQLiteRetriever(sqlPath)
	if err != nil {
		log.Fatalf("Failed to create SQLite retriever: %v", err)
	}
	defer retr.(*agent.SQLiteRetriever).Close()

	// Test search
	ctx := context.Background()

	// Test 1: Search for "ETF"
	fmt.Println("=== Search for 'ETF' ===")
	results, err := retr.Search(ctx, []float32{1.0}, 5)
	if err != nil {
		log.Printf("Search error: %v", err)
	} else {
		for i, doc := range results {
			fmt.Printf("%d. ID: %s, Source: %s, Score: %.4f\n", i+1, doc.ID, doc.Source, doc.Score)
			fmt.Printf("   Text preview: %.100s...\n", doc.Text)
		}
	}

	// Test 2: Search for "Serenity"
	fmt.Println("\n=== Search for 'Serenity' ===")
	results, err = retr.Search(ctx, []float32{0.5}, 5)
	if err != nil {
		log.Printf("Search error: %v", err)
	} else {
		for i, doc := range results {
			fmt.Printf("%d. ID: %s, Source: %s, Score: %.4f\n", i+1, doc.ID, doc.Source, doc.Score)
			fmt.Printf("   Text preview: %.100s...\n", doc.Text)
		}
	}

	// Test 3: Get stats
	fmt.Println("\n=== Retriever Stats ===")
	stats := retr.Stats()
	for k, v := range stats {
		fmt.Printf("%s: %v\n", k, v)
	}

	// Test 4: Test via IndexDocuments flow (re-index with real embedding)
	fmt.Println("\n=== Testing IndexDocuments flow ===")
	testDocs := []agent.Document{
		{
			ID:     "test:demo1",
			Source: "test",
			Text:   "This is a test document about stock trading strategies",
			Meta:   map[string]string{"category": "strategy"},
		},
		{
			ID:     "test:demo2",
			Source: "test",
			Text:   "Another document about ETF rotation and momentum investing",
			Meta:   map[string]string{"category": "etf"},
		},
	}

	// Create mock embedding service
	embedSvc := &agent.MockEmbeddingService{}
	err = retr.IndexDocuments(ctx, testDocs)
	if err != nil {
		log.Printf("IndexDocuments error: %v", err)
	} else {
		fmt.Println("Indexed 2 test documents successfully")
	}

	// Search again
	results, err = retr.Search(ctx, []float32{1.0}, 10)
	if err != nil {
		log.Printf("Search error: %v", err)
	} else {
		fmt.Printf("Total documents after re-index: %d\n", len(results))
		for i, doc := range results {
			fmt.Printf("%d. ID: %s, Source: %s, Score: %.4f\n", i+1, doc.ID, doc.Source, doc.Score)
		}
	}

	// Verify manifest in main DB
	fmt.Println("\n=== Checking index_manifests in main DB ===")
	var manifests []struct {
		DocID       string
		Source      string
		ContentHash string
		Backend     string
		IndexedAt   string
	}
	db.Dao.Table("index_manifests").Select("doc_id, source, content_hash, backend, indexed_at").Find(&manifests)
	for _, m := range manifests {
		fmt.Printf("DocID: %s, Source: %s, Hash: %.16s..., Backend: %s, At: %s\n",
			m.DocID, m.Source, m.ContentHash, m.Backend, m.IndexedAt)
	}
}