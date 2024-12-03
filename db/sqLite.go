package db

import (
	"binance_bot/logger"
	"binance_bot/models"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

const dbVersion = 2 // Increment this whenever a new schema change is added

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
	dbPath := "/app/data/trades.db" // Adjusted to match the Docker mount

	logger.Infof("Initializing database at %s", dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Infof("Error opening database: %v Trying to create local file", err)
		dbPath = "./trades.db"
		db, err = sql.Open("sqlite3", dbPath)
		if err != nil {
			logger.Infof("Error opening database: %v", err)
			return err
		}
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
		logger.Infof("Error creating trades table: %v", err)
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
		logger.Infof("Error creating active_trades table: %v", err)
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
		logger.Infof("Error creating completed_trades table: %v", err)
		return err
	}

	// Update the database schema to the latest version

	if err := UpdateSchema(db, dbVersion); err != nil {
		return fmt.Errorf("failed to update database schema: %v", err)
	}

	log.Println("Database initialized successfully.")
	SQLiteDB.DB = db
	return nil
}

// GetVersionDB retrieves the current database schema version using PRAGMA user_version
func GetVersionDB(db *sql.DB) (int, error) {
	var version int
	err := db.QueryRow("PRAGMA user_version;").Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get database version: %v", err)
	}
	return version, nil
}

func UpdateSchema(db *sql.DB, targetVersion int) error {
	currentVersion, err := GetVersionDB(db)
	if err != nil {
		return err
	}

	// Apply migrations step by step
	for currentVersion < targetVersion {
		currentVersion++
		switch currentVersion {
		case 1:
			// Example migration for version 1
			query := `
			ALTER TABLE completed_trades ADD COLUMN rsi REAL;
			ALTER TABLE completed_trades ADD COLUMN macd REAL;
			ALTER TABLE completed_trades ADD COLUMN stochastic REAL;
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 1: %v", err)
			}
		case 2:
			// Example migration for version 2
			query := `
			CREATE TABLE IF NOT EXISTS trade_audit (
			    id INTEGER PRIMARY KEY AUTOINCREMENT,
			    symbol TEXT NOT NULL,
			    action TEXT NOT NULL,
			    details TEXT,
			    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
			);
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 2: %v", err)
			}
		// Add more cases here for future versions
		default:
			return fmt.Errorf("unsupported target version: %d", currentVersion)
		}

		// Update user_version after each migration
		if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d;", currentVersion)); err != nil {
			return fmt.Errorf("failed to update user_version to %d: %v", currentVersion, err)
		}
	}

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
	query := `INSERT INTO active_trades (symbol, buy_price, quantity) VALUES (?, ?, ?)`
	result, err := s.DB.Exec(query, symbol, buyPrice, quantity)
	if err != nil {
		logger.Infof("Error inserting active trade: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Infof("Inserted active trade for %s. Rows affected: %d", symbol, rowsAffected)
	return nil
}

// LogCompletedTrade logs a completed trade to the SQLite database
func (s *SQLite) LogCompletedTrade(symbol string, buyPrice, sellPrice, quantity, profitLoss, rsi, macd, stochastic float64) error {
	query := `
        INSERT INTO completed_trades (symbol, buy_price, sell_price, quantity, profit_loss, rsi, macd, stochastic)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err := s.DB.Exec(query, symbol, buyPrice, sellPrice, quantity, profitLoss, rsi, macd, stochastic)
	if err != nil {
		return fmt.Errorf("failed to log completed trade: %v", err)
	}
	return nil
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
