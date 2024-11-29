package interfaces

import (
	"binance_bot/models"
	"binance_bot/strategies"
)

// Exchange interface defines methods our bot needs from an exchange
type Exchange interface {
	FetchCandles(symbol string, interval string, limit int) ([]models.CandleStick, error)
	CreateOrder(symbol, orderType, side string, amount float64) error
	CreateLimitOrder(symbol, side, quantity, price string) (int64, error)
	CreateStopLossLimitOrder(symbol, side, quantity, price, stopLoss string) (int64, error)
	MonitorOrder(symbol string, orderID int64) (bool, error)
	CancelOrder(symbol string, orderID int64) error
	GetBalance(asset string) (float64, error)
}

// Strategy interface for implementing different trading strategies
type Strategy interface {
	GetStrategyType() strategies.StrategyType
	Calculate(candles []models.CandleStick, pair string, trend bool) (signal int, err error)
}

type ExchangeClient interface {
	AddTradingPair(pair models.TradingPair) error
	GetCurrentPrice(symbol string) (float64, error)
	FetchCandles(symbol, interval string, limit int) ([]models.CandleStick, error)
	GetBalance(asset string) (float64, error)
	CreateOrder(symbol, orderType, side string, amount string) (float64, error)
	CreateMarketOrder(symbol, side, quantity string) (float64, error)
	CreateLimitOrder(symbol, side, quantity, price string) (int64, error)
	CreateStopLossLimitOrder(symbol, side, quantity, price, stopLoss string) (int64, error)
	MonitorOrder(symbol string, orderID int64) (bool, error)
	CancelOrder(symbol string, orderID int64) error
	GetFeeRate() (float64, error)
	GetTradingPairs() map[string]*models.TradingPair
}
