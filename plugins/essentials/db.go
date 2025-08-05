package essentials

import (
	"database/sql"
	"errors"
	"log"
	"strconv"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

var db *sql.DB

func init() {
	var err error
	db, err = sql.Open("sqlite3", "./cache.db")
	if err != nil {
		log.Printf("Open database error: %v", err)
	}
}

func CreateDB(table string, key *[]string, value *[]string) error {
	if len(*key) != len(*value) {
		return errors.New("key length not equal to value length")
	}

	cmd := "CREATE TABLE IF NOT EXISTS " + table + "("
	for i, k := range *key {
		if i > 0 {
			cmd += ","
		}
		cmd += k + " " + (*value)[i]
	}
	cmd += ")"

	_, err := db.Exec(cmd)
	if err != nil {
		return err
	}
	return nil
}

func InsertDB(table string, key *[]string, value *[]string) error {
	if len(*key) != len(*value) {
		return errors.New("key length not equal to value length")
	}

	cmd := "INSERT INTO " + table + "(" + strings.Join(*key, ", ") + ") VALUES ("
	for i, v := range *value {
		if i > 0 {
			cmd += ", "
		}
		cmd += "'" + v + "'"
	}
	cmd += ")"

	_, err := db.Exec(cmd)
	if err != nil {
		return err
	}
	return nil
}

func SelectDB(table string, target string, arg string) *[]map[string]any {
	cmd := "SELECT " + target + " FROM " + table + " WHERE " + arg
	query, err := db.Query(cmd)
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

func UpdateDB(table string, target string, targetValue string, key *[]string, keyValue *[]string) error {
	cmd := "UPDATE " + table + " SET "
	for i, k := range *key {
		if i > 0 {
			cmd += ", "
		}
		cmd += k + " = '" + (*keyValue)[i] + "'"
	}
	cmd += " WHERE " + target + " = '" + targetValue + "'"

	_, err := db.Exec(cmd)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpired(table string, arg string, expiration int64, interval int64) {
	for {
		cmd := "DELETE FROM " + table + " WHERE " + strconv.FormatInt(time.Now().Unix(), 10) + " - " + arg + " > " + strconv.FormatInt(expiration, 10)
		_, err := db.Exec(cmd)
		if err != nil {
			log.Printf("Database delete error: %v", err)
			return
		}

		time.Sleep(time.Duration(interval) * time.Second)
	}
}
