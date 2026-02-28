package utils

import (
	"database/sql"
	"fmt"

	// Import database drivers
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// ConnectPostgres initializes a connection to a PostgreSQL database.
func ConnectPostgres(host, port, user, password, dbname string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

// ConnectSQLite initializes a connection to a SQLite3 database.
func ConnectSQLite(filepath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", filepath)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}
