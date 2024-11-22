package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

type Database interface {
	InitDB() (*sql.DB, error)
	LogTrade(db *sql.DB, symbol, side string, amount, price float64) error
}

type SQLite struct {
	DB *sql.DB
}

var SQLiteDB SQLite

func InitDB() error {
	// Ensure the database file is created in the mounted volume
	dbPath := "./data/trades.db"

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}

	// Create the trades table if it doesn't exist
	query := `
    CREATE TABLE IF NOT EXISTS trades (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        symbol TEXT,
        side TEXT,
        amount REAL,
        price REAL,
        timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
    );`
	_, err = db.Exec(query)
	if err != nil {
		return err
	}

	SQLiteDB.DB = db
	return nil
}

func (s *SQLite) LogTrade(symbol, side string, amount, price float64) error {
	query := `INSERT INTO trades (symbol, side, amount, price) VALUES (?, ?, ?, ?)`
	_, err := s.DB.Exec(query, symbol, side, amount, price)
	return err
}
