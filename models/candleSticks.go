package models

import "time"

// CandleStick represents a single candlestick in a financial chart, containing OHLCV 
// (Open, High, Low, Close, Volume) data for a specific time period.
// This is a fundamental data structure used throughout the trading system for
// technical analysis, strategy calculation, and market state determination.
type CandleStick struct {
	// Timestamp represents the opening time of the candlestick period
	Timestamp time.Time

	// Open is the opening price at the beginning of the period
	Open float64

	// High is the highest price reached during the period
	High float64

	// Low is the lowest price reached during the period
	Low float64

	// Close is the closing price at the end of the period
	Close float64

	// Volume is the trading volume during the period
	Volume float64
}
