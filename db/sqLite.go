package db

import (
	"binance_bot/models"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

type Database interface {
	InitDB() (*sql.DB, error)
	LogTrade(db *sql.DB, symbol, side string, amount, price float64) error
}

type SQLite struct {
	DB *sql.DB
}

var SQLiteDB SQLite

// InitDB initializes the SQLite database
func InitDB() error {
	// Ensure the database file is created in the mounted volume
	dbPath := "/app/data/trades.db" // Adjusted to match the Docker mount

	log.Printf("Initializing database at %s", dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Printf("Error opening database: %v", err)
		return err
	}

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
		log.Printf("Error creating trades table: %v", err)
		return err
	}

	query = `
    CREATE TABLE IF NOT EXISTS active_trades (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    buy_price REAL NOT NULL,
    quantity REAL NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("Error creating active_trades table: %v", err)
		return err
	}

	query = `CREATE TABLE IF NOT EXISTS completed_trades (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,
    buy_price REAL NOT NULL,
    sell_price REAL NOT NULL,
    quantity REAL NOT NULL,
    profit_loss REAL NOT NULL,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
);`
	_, err = db.Exec(query)
	if err != nil {
		log.Printf("Error creating completed_trades table: %v", err)
		return err
	}

	log.Println("Database initialized successfully.")
	SQLiteDB.DB = db
	return nil
}

// Deprecated: LogTrade logs a trade to the SQLite database
func (s *SQLite) LogTrade(symbol, side string, amount, price float64) error {
	query := `INSERT INTO trades (symbol, side, amount, price) VALUES (?, ?, ?, ?)`
	_, err := s.DB.Exec(query, symbol, side, amount, price)
	return err
}

// LogActiveTrade logs an active trade to the SQLite database
func (s *SQLite) LogActiveTrade(symbol string, buyPrice, quantity float64) error {
	_, err := s.DB.Exec(`
        INSERT INTO active_trades (symbol, buy_price, quantity)
        VALUES (?, ?, ?)
    `, symbol, buyPrice, quantity)
	return err
}

// GetActiveTrade fetches the active trade for a given symbol
func (s *SQLite) GetActiveTrade(symbol string) (*models.ActiveTrade, error) {
	query := `SELECT id, symbol, buy_price, quantity FROM active_trades WHERE symbol = ? LIMIT 1`
	row := s.DB.QueryRow(query, symbol)

	var trade models.ActiveTrade
	err := row.Scan(&trade.ID, &trade.Symbol, &trade.BuyPrice, &trade.Quantity)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active trade found for symbol: %s", symbol)
		}
		return nil, fmt.Errorf("error fetching active trade for symbol %s: %v", symbol, err)
	}

	return &trade, nil
}

// GetActiveTrades fetches all active trades for a given symbol
func (s *SQLite) GetActiveTrades(symbol string) ([]*models.ActiveTrade, error) {
	query := `SELECT id, symbol, buy_price, quantity FROM active_trades WHERE symbol = ?`
	rows, err := s.DB.Query(query, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*models.ActiveTrade
	for rows.Next() {
		var trade models.ActiveTrade
		err := rows.Scan(&trade.ID, &trade.Symbol, &trade.BuyPrice, &trade.Quantity)
		if err != nil {
			return nil, err
		}
		trades = append(trades, &trade)
	}
	return trades, nil
}

// RemoveActiveTrade removes an active trade from the SQLite database
func (s *SQLite) RemoveActiveTrade(id int) error {
	_, err := s.DB.Exec(`DELETE FROM active_trades WHERE id = ?`, id)
	return err
}

// LogCompletedTrade logs a completed trade to the SQLite database
func (s *SQLite) LogCompletedTrade(symbol string, buyPrice, sellPrice, quantity, profitLoss float64) error {
	query := `INSERT INTO completed_trades (symbol, buy_price, sell_price, quantity, profit_loss) VALUES (?, ?, ?, ?, ?)`
	_, err := s.DB.Exec(query, symbol, buyPrice, sellPrice, quantity, profitLoss)
	return err
}
