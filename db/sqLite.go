package db

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

const dbVersion = 13 // Increment this whenever a new schema change is added

type Database interface {
	InitDB() (*sql.DB, error)
	LogTrade(db *sql.DB, symbol, side string, amount, price float64) error
}

type SQLite struct {
	DB *sql.DB
	mu sync.RWMutex
}

var SQLiteDB SQLite

func InitDB() error {
	dbPath := "/app/data/trades.db" // Adjusted to match the Docker mount

	// folder exists
	if _, err := os.Stat("/app/data"); os.IsNotExist(err) {
		logger.Warnf("Running without docker, using local db file")
		dbPath = "./trades.db"
	}

	logger.Infof("Initializing database at %s", dbPath)

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		logger.Infof("Error opening database: %v", err)
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
		return fmt.Errorf("failed to update database schema: %w", err)
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
		return 0, fmt.Errorf("failed to get database version: %w", err)
	}
	return version, nil
}

//nolint:funlen, it's ok
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
				return fmt.Errorf("failed to apply migration for version 1: %w", err)
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
		case 3:
			query := `
			ALTER TABLE completed_trades ADD COLUMN lowerBound REAL;
			ALTER TABLE completed_trades ADD COLUMN middleBound REAL;
			ALTER TABLE completed_trades ADD COLUMN upperBound REAL;
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 1: %v", err)
			}
		case 4:
			query := `CREATE TABLE IF NOT EXISTS pair_ath (
    				  id INTEGER PRIMARY KEY AUTOINCREMENT,
    				  symbol TEXT NOT NULL,
    				  ath_price REAL NOT NULL,
                      timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
			);`
			_, err = db.Exec(query)
			if err != nil {
				logger.Infof("Error creating completed_trades table: %v", err)
				return err
			}
		case 5:
			query := `DROP TABLE IF EXISTS pair_ath;
				CREATE TABLE pair_ath (
					symbol TEXT PRIMARY KEY,
					ath_price REAL NOT NULL,
					timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
				);`

			_, err = db.Exec(query)
			if err != nil {
				logger.Infof("Error creating completed_trades table: %v", err)
				return err
			}
		case 6:
			_, err := db.Exec("PRAGMA journal_mode = WAL;")
			if err != nil {
				logger.Infof("Error creating completed_trades table: %v", err)
				return err
			}
		case 7:
			query := `
			ALTER TABLE active_trades ADD COLUMN order_id INTEGER;
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 1: %v", err)
			}
		case 8:
			query := `CREATE TABLE IF NOT EXISTS pair_atl (
    				  id INTEGER PRIMARY KEY AUTOINCREMENT,
    				  symbol TEXT NOT NULL,
    				  atl_price REAL NOT NULL,
                      timestamp DATETIME DEFAULT CURRENT_TIMESTAMP);`
			_, err := db.Exec(query)
			if err != nil {
				logger.Infof("Error creating pair_atl table: %v", err)
				return err
			}
		case 9:
			query := `DROP TABLE IF EXISTS pair_atl;
				CREATE TABLE IF NOT EXISTS pair_atl (
					symbol TEXT PRIMARY KEY,
					atl_price REAL NOT NULL,
					timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
				);`

			_, err := db.Exec(query)
			if err != nil {
				logger.Infof("Error creating pair_atl table: %v", err)
				return err
			}
		case 10:
			query := `ALTER TABLE completed_trades ADD COLUMN buy_timestamp DATETIME;`
			_, err := db.Exec(query)
			if err != nil {
				logger.Infof("Error creating pair_atl table: %v", err)
				return err
			}
		case 11:
			// Example migration for version 1
			query := `
			ALTER TABLE completed_trades ADD COLUMN mfi REAL;
			ALTER TABLE completed_trades ADD COLUMN cci REAL;
			ALTER TABLE completed_trades ADD COLUMN ichimoku_kijun REAL;
			ALTER TABLE completed_trades ADD COLUMN ichimoku_tenkan REAL;
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 1: %w", err)
			}
		case 12:
			query := `
			ALTER TABLE active_trades ADD COLUMN rsi REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN macd REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN stochastic REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN lowerBound REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN middleBound REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN upperBound REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN mfi REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN cci REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN ichimoku_kijun REAL DEFAULT 0;
			ALTER TABLE active_trades ADD COLUMN ichimoku_tenkan REAL DEFAULT 0;
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 1: %w", err)
			}
		case 13:
			query := `
			ALTER TABLE completed_trades ADD COLUMN order_id INTEGER;
			`
			if _, err := db.Exec(query); err != nil {
				return fmt.Errorf("failed to apply migration for version 1: %v", err)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `INSERT INTO trades (symbol, side, amount, price) VALUES (?, ?, ?, ?)`
	_, err := s.DB.Exec(query, symbol, side, amount, price)
	return err
}

// LogActiveTrade logs an active trade to the SQLite database
func (s *SQLite) LogActiveTrade(symbol string, buyPrice, quantity float64, orderID int64, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimokuTenkan, ichimokuKijun float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `INSERT INTO active_trades (symbol, buy_price, quantity, order_id, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimoku_kijun, ichimoku_tenkan) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := s.DB.Exec(query, symbol, buyPrice, quantity, orderID, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimokuKijun, ichimokuTenkan)
	if err != nil {
		logger.Infof("Error inserting active trade: %v", err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	logger.Infof("Inserted active trade for %s. Rows affected: %d", symbol, rowsAffected)
	return nil
}

// LogCompletedTrade logs a completed trade to the SQLite database
func (s *SQLite) LogCompletedTrade(symbol string, buyPrice, sellPrice, quantity, profitLoss, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimokuTenkan, ichimokuKijun float64, buyTimestamp, orderID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `
        INSERT INTO completed_trades (symbol, buy_price, sell_price, quantity, profit_loss, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimoku_kijun, ichimoku_tenkan, buy_timestamp, order_id)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err := s.DB.Exec(query, symbol, buyPrice, sellPrice, quantity, profitLoss, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimokuKijun, ichimokuTenkan, buyTimestamp, orderID)
	if err != nil {
		return fmt.Errorf("failed to log completed trade: %v", err)
	}
	return nil
}

// GetActiveTrade fetches the active trade for a given symbol
func (s *SQLite) GetActiveTrade(symbol string) (*models.ActiveTrade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT id, order_id, symbol, buy_price, quantity, timestamp, rsi, macd, stochastic, lowerBound, middleBound, upperBound, mfi, cci, ichimoku_kijun, ichimoku_tenkan FROM active_trades WHERE symbol = ? LIMIT 1`
	row := s.DB.QueryRow(query, symbol)

	var trade models.ActiveTrade
	err := row.Scan(&trade.ID, &trade.OrderID, &trade.Symbol, &trade.BuyPrice, &trade.Quantity, &trade.Timestamp, &trade.RSI, &trade.Macd, &trade.Stochastic, &trade.LowerBound, &trade.MiddleBound, &trade.UpperBound, &trade.MFI, &trade.CCI, &trade.IchimokuKijun, &trade.IchimokuTenkan)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no active trade found for symbol: %s", symbol)
		}
		return nil, fmt.Errorf("error fetching active trade for symbol %s: %v", symbol, err)
	}

	return &trade, nil
}

// GetActiveTradesCount fetches the number of active trades
func (s *SQLite) GetActiveTradesCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT COUNT(*) FROM active_trades`
	var count int
	err := s.DB.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// GetTodaysTradeCount fetches the number of trades for the current day
func (s *SQLite) GetTodaysTradeCount() (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT COUNT(*) FROM active_trades WHERE date(timestamp) = date('now')`
	var count int
	err := s.DB.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}
	query2 := `SELECT COUNT(*) FROM completed_trades WHERE date(buy_timestamp) = date('now')`
	var count2 int
	err = s.DB.QueryRow(query2).Scan(&count2)
	if err != nil {
		return 0, err
	}
	return count + count2, nil
}

// GetActiveTrades fetches all active trades for a given symbol
func (s *SQLite) GetActiveTrades(symbol string) ([]*models.ActiveTrade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT id,order_id, symbol, buy_price, quantity, timestamp FROM active_trades WHERE symbol = ?`
	rows, err := s.DB.Query(query, symbol)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*models.ActiveTrade
	for rows.Next() {
		var trade models.ActiveTrade
		err := rows.Scan(&trade.ID, &trade.OrderID, &trade.Symbol, &trade.BuyPrice, &trade.Quantity, &trade.Timestamp)
		if err != nil {
			return nil, err
		}
		trades = append(trades, &trade)
	}
	return trades, nil
}

// GetAllActiveTrades fetches all active trades
func (s *SQLite) GetAllActiveTrades() ([]*models.ActiveTrade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT id, order_id, symbol, buy_price, quantity, timestamp FROM active_trades WHERE 1`
	rows, err := s.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*models.ActiveTrade
	for rows.Next() {
		var trade models.ActiveTrade
		err := rows.Scan(&trade.ID, &trade.OrderID, &trade.Symbol, &trade.BuyPrice, &trade.Quantity, &trade.Timestamp)
		if err != nil {
			return nil, err
		}
		trades = append(trades, &trade)
	}
	return trades, nil
}

// IsCurrentlyActiveTrade checks if an active trade exists for the given symbol
func (s *SQLite) IsCurrentlyActiveTrade(symbol string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT COUNT(*) FROM active_trades WHERE symbol = ?`
	var count int
	err := s.DB.QueryRow(query, symbol).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLite) GetAth(symbol string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT ath_price FROM pair_ath WHERE symbol = ?`
	var ath float64
	err := s.DB.QueryRow(query, symbol).Scan(&ath)
	if err != nil {
		return 0, err
	}
	return ath, nil
}

func (s *SQLite) SetUpdateAth(symbol string, ath float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `INSERT INTO pair_ath (symbol, ath_price, timestamp) VALUES (?, ?, ?) ON CONFLICT(symbol) DO UPDATE SET ath_price = excluded.ath_price, timestamp = excluded.timestamp;`
	_, err := s.DB.Exec(query, symbol, ath, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLite) RemoveAth(symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `DELETE FROM pair_ath WHERE symbol = ?`
	_, err := s.DB.Exec(query, symbol)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLite) GetAtl(symbol string) (float64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT atl_price FROM pair_atl WHERE symbol = ?`
	var ath float64
	err := s.DB.QueryRow(query, symbol).Scan(&ath)
	if err != nil {
		logger.Errorf("Error fetching ATL for %s: %v", symbol, err)
		return 0, err
	}
	return ath, nil
}

func (s *SQLite) SetUpdateAtl(symbol string, atl float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `INSERT INTO pair_atl (symbol, atl_price, timestamp) VALUES (?, ?, ?) ON CONFLICT(symbol) DO UPDATE SET atl_price = excluded.atl_price, timestamp = excluded.timestamp;`
	_, err := s.DB.Exec(query, symbol, atl, time.Now())
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLite) RemoveAtl(symbol string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	query := `DELETE FROM pair_atl WHERE symbol = ?`
	_, err := s.DB.Exec(query, symbol)
	if err != nil {
		return err
	}
	return nil
}

// RemoveActiveTrade removes an active trade from the SQLite database
func (s *SQLite) RemoveActiveTrade(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.DB.Exec(`DELETE FROM active_trades WHERE id = ?`, id)
	return err
}

// RenameAllSymbolsUSDTtoUSDC finds all distinct symbols in the trades table
// that end with "USDT" and updates them to end with "USDC".
func (s *SQLite) RenameAllSymbolsUSDTtoUSDC() error {
	// Acquire write lock since we are both reading and updating
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Fetch all distinct symbols from trades
	selectQuery := `SELECT DISTINCT symbol FROM active_trades`
	rows, err := s.DB.Query(selectQuery)
	if err != nil {
		return fmt.Errorf("error retrieving distinct symbols: %v", err)
	}
	defer rows.Close()

	var symbols []string
	for rows.Next() {
		var sym string
		if err := rows.Scan(&sym); err != nil {
			return fmt.Errorf("error scanning symbol: %v", err)
		}
		symbols = append(symbols, sym)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %v", err)
	}

	// 2. For each symbol that ends with "USDT", rename it to end with "USDC"
	for _, oldSymbol := range symbols {
		if strings.HasSuffix(oldSymbol, "USDT") {
			newSymbol := strings.Replace(oldSymbol, "USDT", "USDC", 1)

			// 3. Execute the UPDATE query
			updateQuery := `UPDATE active_trades SET symbol = ? WHERE symbol = ?`
			stmt, err := s.DB.Prepare(updateQuery)
			if err != nil {
				return fmt.Errorf("error preparing update query for symbol %s: %v", oldSymbol, err)
			}

			_, execErr := stmt.Exec(newSymbol, oldSymbol)
			stmt.Close()
			if execErr != nil {
				return fmt.Errorf("error updating symbol from %s to %s: %v", oldSymbol, newSymbol, execErr)
			}
		}
	}

	return nil
}

// GetLastATHTimestamp fetches the last ATH timestamp for a given symbol
func (s *SQLite) GetLastATHTimestamp(symbol string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT timestamp FROM pair_ath WHERE symbol = ?`
	var timestamp string
	err := s.DB.QueryRow(query, symbol).Scan(&timestamp)
	if err != nil {
		return time.Time{}, err
	}

	timestampParsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, err
	}

	return timestampParsed, nil
}

// GetLastBuySymbol fetches the last ATH timestamp for a given symbol
func (s *SQLite) GetLastBuySymbol(symbol string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT timestamp FROM active_trades WHERE symbol = ?`
	var timestamp string
	err := s.DB.QueryRow(query, symbol).Scan(&timestamp)
	if err != nil {
		return time.Time{}, err
	}

	timestampParsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, err
	}

	return timestampParsed, nil
}

// GetLastSellSymbol fetches the last ATH timestamp for a given symbol
func (s *SQLite) GetLastSellSymbol(symbol string) (time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	query := `SELECT timestamp FROM completed_trades WHERE symbol = ?`
	var timestamp string
	err := s.DB.QueryRow(query, symbol).Scan(&timestamp)
	if err != nil {
		return time.Time{}, err
	}

	timestampParsed, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return time.Time{}, err
	}

	return timestampParsed, nil
}

// WasLastTradeLoss fetches the last trade's timestamp and profit/loss for a given symbol
func (s *SQLite) WasLastTradeLoss(symbol string) (bool, time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		tsStr string
		pnl   float64
	)

	query := `SELECT timestamp, profit_loss
              FROM completed_trades
              WHERE symbol = ?
              ORDER BY timestamp DESC
              LIMIT 1`

	err := s.DB.QueryRow(query, symbol).Scan(&tsStr, &pnl)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, time.Time{}, nil // no trades yet
		}
		return false, time.Time{}, fmt.Errorf("failed to fetch last trade: %w", err)
	}

	if pnl >= 0 {
		return false, time.Time{}, nil // last trade was profitable → no cool‑down
	}

	ts, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return true, time.Time{}, fmt.Errorf("invalid timestamp in DB: %w", err)
	}

	return true, ts, nil
}
