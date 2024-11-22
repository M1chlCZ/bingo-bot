package models

import "time"

// CandleStick represents OHLCV data
type CandleStick struct {
	Timestamp time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}
