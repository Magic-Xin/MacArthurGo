package essentials

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

var db *sql.DB

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

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
	if !validIdentifier(table) {
		return fmt.Errorf("invalid table name %q", table)
	}

	cmd := "CREATE TABLE IF NOT EXISTS " + quoteIdentifier(table) + "("
	for i, k := range *key {
		if !validIdentifier(k) {
			return fmt.Errorf("invalid column name %q", k)
		}
		if i > 0 {
			cmd += ","
		}
		cmd += quoteIdentifier(k) + " " + (*value)[i]
	}
	cmd += ")"

	_, err := db.Exec(cmd)
	if err != nil {
		return err
	}
	return nil
}

func InsertDB(table string, keys []string, values []any) error {
	if len(keys) != len(values) {
		return errors.New("key length not equal to value length")
	}
	if !validIdentifier(table) || !validIdentifiers(keys) {
		return errors.New("invalid database identifier")
	}

	quoted := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	for i, key := range keys {
		quoted[i] = quoteIdentifier(key)
		placeholders[i] = "?"
	}
	cmd := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quoteIdentifier(table), strings.Join(quoted, ", "), strings.Join(placeholders, ", "))

	_, err := db.Exec(cmd, values...)
	if err != nil {
		return err
	}
	return nil
}

func SelectDB(table string, target string, whereColumn string, whereValue any) ([]map[string]any, error) {
	if !validIdentifiers([]string{table, target, whereColumn}) {
		return nil, errors.New("invalid database identifier")
	}
	cmd := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", quoteIdentifier(target), quoteIdentifier(table), quoteIdentifier(whereColumn))
	query, err := db.Query(cmd, whereValue)
	if err != nil {
		return nil, err
	}
	defer query.Close()

	columns, err := query.Columns()
	if err != nil {
		return nil, err
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
			return nil, err
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
	if err = query.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func UpdateDB(table string, whereColumn string, whereValue any, keys []string, values []any) error {
	if len(keys) != len(values) {
		return errors.New("key length not equal to value length")
	}
	if !validIdentifier(table) || !validIdentifier(whereColumn) || !validIdentifiers(keys) {
		return errors.New("invalid database identifier")
	}

	assignments := make([]string, len(keys))
	for i, key := range keys {
		assignments[i] = quoteIdentifier(key) + " = ?"
	}
	cmd := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quoteIdentifier(table), strings.Join(assignments, ", "), quoteIdentifier(whereColumn))
	args := append(append([]any{}, values...), whereValue)
	_, err := db.Exec(cmd, args...)
	if err != nil {
		return err
	}

	return nil
}

func DeleteExpired(table string, arg string, expiration int64, interval int64) {
	if !validIdentifier(table) || !validIdentifier(arg) {
		log.Printf("Database cleanup skipped: invalid identifier")
		return
	}
	if expiration <= 0 {
		expiration = 24 * 60 * 60
	}
	if interval <= 0 {
		interval = 30 * 60
	}

	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	for {
		cmd := fmt.Sprintf("DELETE FROM %s WHERE %s < ?", quoteIdentifier(table), quoteIdentifier(arg))
		_, err := db.Exec(cmd, time.Now().Unix()-expiration)
		if err != nil {
			log.Printf("Database delete error: %v", err)
		}
		<-ticker.C
	}
}

func validIdentifier(value string) bool {
	return identifierPattern.MatchString(value)
}

func validIdentifiers(values []string) bool {
	for _, value := range values {
		if !validIdentifier(value) {
			return false
		}
	}
	return true
}

func quoteIdentifier(value string) string {
	return `"` + value + `"`
}
