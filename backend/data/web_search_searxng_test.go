package data

import (
	"blades-stock/backend/datasource"
	"testing"
)

func TestGetSearXNGEndpoints(t *testing.T) {
	endpoints := datasource.GetSearXNGEndpoints()
	if len(endpoints) == 0 {
		t.Fatal("expected at least one SearXNG endpoint")
	}

	foundDefault := false
	for _, ep := range endpoints {
		if ep == "https://search.woha.men" {
			foundDefault = true
			break
		}
	}

	if !foundDefault {
		t.Fatalf("expected https://search.woha.men in endpoints, got: %v", endpoints)
	}
}

func TestSearXNGOnlineSearch(t *testing.T) {
	api := NewWebSearchApi(10)
	results := api.SearchSearXNG("贵州茅台", 3)
	if len(results) == 0 {
		t.Log("Warning: No results returned or network unavailable (skip hard fail in offline environment)")
		return
	}

	t.Logf("Successfully fetched %d results from SearXNG (search.woha.men):", len(results))
	for i, r := range results {
		t.Logf(" [%d] %s: %s (%s)", i+1, r.Title, r.Url, r.Source)
	}

	if results[0].Title == "" || results[0].Url == "" {
		t.Errorf("Expected non-empty Title and Url, got %+v", results[0])
	}
}
