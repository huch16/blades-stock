package main

import (
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"sort"

	_ "github.com/glebarez/sqlite"
)

func bytesToFloat32sLE(b []byte) []float32 {
	n := len(b) / 4
	out := make([]float32, n)
	for i := 0; i < n; i++ {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:(i+1)*4]))
	}
	return out
}

func main() {
	dbPath := flag.String("db", "data/vectors.db", "sqlite db path")
	queryID := flag.String("query", "", "use this doc id as query (e.g. doc:README.md); if empty uses first row")
	topk := flag.Int("topk", 5, "top K")
	flag.Parse()

	db, err := sql.Open("sqlite", *dbPath+"?_journal_mode=WAL")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	var qVec []float32
	var qID string
	if *queryID != "" {
		qID = *queryID
		row := db.QueryRow("SELECT vector FROM vectors WHERE id = ?", qID)
		var blob []byte
		if err := row.Scan(&blob); err != nil {
			log.Fatalf("query vector for %s failed: %v", qID, err)
		}
		qVec = bytesToFloat32sLE(blob)
	} else {
		row := db.QueryRow("SELECT id, vector FROM vectors LIMIT 1")
		var blob []byte
		if err := row.Scan(&qID, &blob); err != nil {
			log.Fatalf("no rows in vectors: %v", err)
		}
		qVec = bytesToFloat32sLE(blob)
	}
	fmt.Printf("Query doc id: %s, vector len=%d\n", qID, len(qVec))

	rows, err := db.Query("SELECT id, text, meta, vector FROM vectors")
	if err != nil {
		log.Fatalf("select vectors: %v", err)
	}
	defer rows.Close()

	type item struct {
		id    string
		text  string
		score float32
		meta  string
	}
	var items []item
	for rows.Next() {
		var id, text, meta string
		var blob []byte
		if err := rows.Scan(&id, &text, &meta, &blob); err != nil {
			continue
		}
		vec := bytesToFloat32sLE(blob)
		if len(vec) != len(qVec) {
			continue
		}
		var dot, na, nb float32
		for i := range vec {
			dot += vec[i] * qVec[i]
			na += vec[i] * vec[i]
			nb += qVec[i] * qVec[i]
		}
		var score float32
		if na == 0 || nb == 0 {
			score = 0
		} else {
			score = dot / (float32(math.Sqrt(float64(na))) * float32(math.Sqrt(float64(nb))))
		}
		items = append(items, item{id: id, text: text, score: score, meta: meta})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].score > items[j].score })

	n := *topk
	if n > len(items) {
		n = len(items)
	}
	fmt.Printf("Top %d results:\n", n)
	for i := 0; i < n; i++ {
		fmt.Printf("%d) id=%s score=%.6f\n", i+1, items[i].id, items[i].score)
		var m map[string]string
		_ = json.Unmarshal([]byte(items[i].meta), &m)
		if title, ok := m["title"]; ok {
			fmt.Printf("   title: %s\n", title)
		}
	}
}
