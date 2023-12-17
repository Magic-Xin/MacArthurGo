package essentials

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
	"time"
)

var DB *sql.DB

func init() {
	var err error
	DB, err = sql.Open("sqlite3", "./cache.db")
	if err != nil {
		log.Printf("Open database error: %v", err)
	}
}

func InsertDB(table string, key *[]string, value ...any) {
	if len(*key) != len(value) {
		log.Printf("Database insert error: key length not equal to value length")
		return
	}
	placeHolder := make([]string, len(*key))
	for i := range placeHolder {
		placeHolder[i] = "?"
	}
	stmt, err := DB.Prepare("INSERT INTO " + table + "(" + strings.Join(*key, ", ") +
		") VALUES (" + strings.Join(placeHolder, ", ") + ")")
	if err != nil {
		log.Printf("Database insert prepare error: %v", err)
		return
	}
	_, err = stmt.Exec(value...)
	if err != nil {
		log.Printf("Database insert exec error: %v", err)
		return
	}
}

func SelectDB(arg string, params ...any) *[]map[string]any {
	query, err := DB.Query(arg, params...)
	if err != nil {
		log.Printf("Database query error: %v", err)
		return nil
	}

	columns, err := query.Columns()
	if err != nil {
		log.Printf("Database query columns error: %v", err)
		return nil
	}

	values := make([]any, len(columns))
	valuePtr := make([]any, len(columns))
	result := make([]map[string]any, 0)
	for query.Next() {
		for i := range columns {
			valuePtr[i] = &values[i]
		}
		err = query.Scan(valuePtr...)
		if err != nil {
			log.Printf("Database query scan error: %v", err)
			return nil
		}
		row := make(map[string]any)
		for i, col := range columns {
			var v any
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			row[col] = v
		}
		result = append(result, row)
	}
	return &result
}

func UpdateDB(arg string, params ...any) {
	stmt, err := DB.Prepare(arg)
	if err != nil {
		log.Printf("Database update prepare error: %v", err)
		return
	}
	_, err = stmt.Exec(params...)
	if err != nil {
		log.Printf("Database update exec error: %v", err)
		return
	}
}

func DeleteExpired(arg string, expiration int64, interval int64) {
	for {
		stmt, err := DB.Prepare(arg)
		if err != nil {
			log.Printf("Database delete prepare error: %v", err)
			return
		}

		_, err = stmt.Exec(time.Now().Unix(), expiration)
		if err != nil {
			log.Printf("Database delete exec error: %v", err)
			return
		}
		time.Sleep(time.Duration(interval) * time.Second)
	}
}
