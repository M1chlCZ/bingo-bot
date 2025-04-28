package interfaces

import (
	"github.com/M1chlCZ/bingo-bot/models"
	//"github.com/M1chlCZ/bingo-bot/strategies"

	//"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/adshao/go-binance/v2"
	"time"
)

// Exchange interface defines methods our bot needs from an exchange.
// This is a simplified interface for basic exchange operations.
type Exchange interface {
	// FetchCandles retrieves historical candlestick data for a trading pair.
	//
	// Parameters:
	//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
	//   - interval: The time interval for each candle (e.g., "1m", "1h", "1d")
	//   - limit: Maximum number of candles to retrieve
	//
	// Returns:
	//   - []models.CandleStick: Array of candlestick data
	//   - error: Any error encountered during the operation
	FetchCandles(symbol string, interval string, limit int) ([]models.CandleStick, error)

	// CreateOrder places a new order on the exchange.
	//
	// Parameters:
	//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
	//   - orderType: Type of order (e.g., "MARKET", "LIMIT")
	//   - side: Order side ("BUY" or "SELL")
	//   - amount: Quantity to buy or sell
	//
	// Returns:
	//   - error: Any error encountered during the operation
	CreateOrder(symbol, orderType, side string, amount float64) error

	// CreateLimitOrder places a limit order on the exchange.
	//
	// Parameters:
	//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
	//   - side: Order side ("BUY" or "SELL")
	//   - quantity: Amount to buy or sell
	//   - price: Limit price for the order
	//
	// Returns:
	//   - int64: Order ID
	//   - error: Any error encountered during the operation
	CreateLimitOrder(symbol, side, quantity, price string) (int64, error)

	// CreateStopLossLimitOrder places a stop-loss limit order on the exchange.
	//
	// Parameters:
	//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
	//   - side: Order side ("BUY" or "SELL")
	//   - quantity: Amount to buy or sell
	//   - price: Limit price for the order
	//   - stopLoss: Stop price that triggers the limit order
	//
	// Returns:
	//   - int64: Order ID
	//   - error: Any error encountered during the operation
	CreateStopLossLimitOrder(symbol, side, quantity, price, stopLoss string) (int64, error)

	// MonitorOrder checks the status of an existing order.
	//
	// Parameters:
	//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
	//   - orderID: ID of the order to monitor
	//
	// Returns:
	//   - bool: True if the order is filled, false otherwise
	//   - error: Any error encountered during the operation
	MonitorOrder(symbol string, orderID int64) (bool, error)

	// CancelOrder cancels an existing order on the exchange.
	//
	// Parameters:
	//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
	//   - orderID: ID of the order to cancel
	//
	// Returns:
	//   - error: Any error encountered during the operation
	CancelOrder(symbol string, orderID int64) error

	// GetBalance retrieves the current balance for a specific asset.
	//
	// Parameters:
	//   - asset: Asset symbol (e.g., "BTC", "USDT")
	//
	// Returns:
	//   - float64: Available balance of the asset
	//   - error: Any error encountered during the operation
	GetBalance(asset string) (float64, error)
}

// Strategy interface defines the contract for implementing different trading strategies.
// Strategies analyze market data and generate trading signals.
type Strategy interface {
	// Calculate analyzes candlestick data and generates a trading signal.
	//
	// Parameters:
	//   - candles: Historical candlestick data for analysis
	//   - pair: Trading pair symbol (e.g., "BTCUSDT")
	//   - marketState: Current state of the market (trending, ranging, etc.)
	//   - pendingCoolDown: Minimum time to wait before reconsidering a pending buy
	//
	// Returns:
	//   - int: Trading signal (-1 for sell, 0 for hold, 1 for buy)
	//   - error: Any error encountered during calculation
	Calculate(candles []models.CandleStick, pair string, marketState models.MarketState, pendingCoolDown time.Duration) (int, error)

	// GetCandleInterval returns the time interval used by this strategy.
	//
	// Returns:
	//   - string: Candle interval (e.g., "1m", "5m", "1h", "1d")
	GetCandleInterval() string

	// Clone creates a deep copy of the strategy.
	// This is important to prevent data sharing between different trading pairs.
	//
	// Returns:
	//   - Strategy: A new instance of the strategy with the same configuration
	Clone() Strategy

	// GetMarketState returns the market state this strategy is designed for.
	//
	// Returns:
	//   - models.MarketState: The market state (trending, ranging, etc.)
	GetMarketState() models.MarketState
}

// ExchangeClient interface defines a comprehensive set of methods for interacting with a cryptocurrency exchange.
// This interface extends the basic Exchange interface with additional functionality for trading pair management,
// market data retrieval, and order execution.
type ExchangeClient interface {
	// AddTradingPair adds a new trading pair to the client's tracked pairs.
	//
	// Parameters:
	//   - pair: Trading pair to add
	//
	// Returns:
	//   - error: Any error encountered during the operation
	AddTradingPair(pair models.TradingPair) error

	// GetCurrentPrice retrieves the current market price for a trading pair.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//
	// Returns:
	//   - float64: Current price
	//   - error: Any error encountered during the operation
	GetCurrentPrice(symbol string) (float64, error)

	// FetchCandles retrieves historical candlestick data for a trading pair.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//   - interval: Time interval for each candle (e.g., "1m", "1h", "1d")
	//   - limit: Maximum number of candles to retrieve
	//   - priority: Whether this request should have priority (may affect rate limiting)
	//
	// Returns:
	//   - []models.CandleStick: Array of candlestick data
	//   - error: Any error encountered during the operation
	FetchCandles(symbol, interval string, limit int, priority bool) ([]models.CandleStick, error)

	// GetBalance retrieves the current balance for a specific asset.
	//
	// Parameters:
	//   - asset: Asset symbol (e.g., "BTC", "USDT")
	//
	// Returns:
	//   - float64: Available balance of the asset
	//   - error: Any error encountered during the operation
	GetBalance(asset string) (float64, error)

	// CreateOrder places a new order on the exchange.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//   - orderType: Type of order (e.g., "MARKET", "LIMIT")
	//   - side: Order side ("BUY" or "SELL")
	//   - amount: Quantity to buy or sell as a string
	//
	// Returns:
	//   - float64: Executed price
	//   - error: Any error encountered during the operation
	CreateOrder(symbol, orderType, side string, amount string) (float64, error)

	// CreateMarketOrder places a market order on the exchange.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//   - side: Order side ("BUY" or "SELL")
	//   - quantity: Amount to buy or sell
	//
	// Returns:
	//   - int64: Order ID
	//   - float64: Executed price
	//   - error: Any error encountered during the operation
	CreateMarketOrder(symbol, side, quantity string) (int64, float64, error)

	// IsTickerTooNew checks if a trading pair is too new for safe trading.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//
	// Returns:
	//   - bool: True if the pair is established enough for trading, false if too new
	//   - error: Any error encountered during the operation
	IsTickerTooNew(symbol string) (bool, error)

	// GetTradingPairs returns all trading pairs currently tracked by the client.
	//
	// Returns:
	//   - map[string]*models.TradingPair: Map of trading pairs indexed by symbol
	GetTradingPairs() map[string]*models.TradingPair

	// FetchMarkets retrieves available markets from the exchange based on filters.
	//
	// Parameters:
	//   - tickers: List of base assets to include (e.g., ["BTC", "ETH"])
	//   - excludedMarkets: List of quote assets to exclude (e.g., ["EUR", "BNB"])
	//   - excludedTradingPairs: Specific trading pairs to exclude
	//
	// Returns:
	//   - []models.TradingPair: List of trading pairs matching the criteria
	//   - error: Any error encountered during the operation
	FetchMarkets(tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) ([]models.TradingPair, error)

	// FetchActiveTrades retrieves active trades for a specific symbol.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//
	// Returns:
	//   - []*binance.TradeV3: List of active trades
	//   - error: Any error encountered during the operation
	FetchActiveTrades(symbol string) ([]*binance.TradeV3, error)

	// FetchHistoricalCandles retrieves historical candlestick data within a specific time range.
	//
	// Parameters:
	//   - symbol: Trading pair symbol (e.g., "BTCUSDT")
	//   - interval: Time interval for each candle (e.g., "1m", "1h", "1d")
	//   - startTime: Beginning of the time range
	//   - endTime: End of the time range
	//   - limit: Maximum number of candles to retrieve
	//
	// Returns:
	//   - []models.CandleStick: Array of candlestick data
	//   - error: Any error encountered during the operation
	FetchHistoricalCandles(symbol string, interval string, startTime, endTime time.Time, limit int) ([]models.CandleStick, error)
}
