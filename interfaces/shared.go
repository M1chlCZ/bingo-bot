package interfaces

import "binance_bot/models"

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
	Calculate(candles []models.CandleStick, pair string, trend bool) (signal int, err error)
}