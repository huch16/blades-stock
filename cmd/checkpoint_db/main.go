package main

import (
	"fmt"
	"os"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: checkpoint <db-path>")
		os.Exit(1)
	}
	path := os.Args[1]
	db, err := gorm.Open(sqlite.Open(path+"?_journal_mode=DELETE&_busy_timeout=10000"), &gorm.Config{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", path, err)
		os.Exit(1)
	}
	db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	fmt.Printf("checkpoint: %s\n", path)
}
