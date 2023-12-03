package essentials

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

var DB *sql.DB

func init() {
	var err error
	DB, err = sql.Open("sqlite3", "./cache.db")
	if err != nil {
		log.Printf("Open database error: %v", err)
	}
}
