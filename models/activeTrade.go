package models

import (
	"strconv"
	"time"
)

type ActiveTrade struct {
	ID             int       `json:"id" db:"id"`
	OrderID        *string   `json:"order_id" db:"order_id"`
	Symbol         string    `json:"symbol" db:"symbol"`
	BuyPrice       float64   `json:"buy_price" db:"buy_price"`
	Quantity       float64   `json:"quantity" db:"quantity"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
	RSI            float64   `json:"rsi" db:"rsi"`
	Macd           float64   `json:"macd" db:"macd"`
	Stochastic     float64   `json:"stochastic" db:"stochastic"`
	LowerBound     float64   `json:"lower_bound" db:"lower_bound"`
	MiddleBound    float64   `json:"middle_bound" db:"middle_bound"`
	UpperBound     float64   `json:"upper_bound" db:"upper_bound"`
	MFI            float64   `json:"mfi" db:"mfi"`
	CCI            float64   `json:"cci" db:"cci"`
	IchimokuKijun  float64   `json:"ichimoku_kijun" db:"ichimoku_kijun"`
	IchimokuTenkan float64   `json:"ichimoku_tenkan" db:"ichimoku_tenkan"`
}

func (a *ActiveTrade) GetOrderID() int64 {
	if a.OrderID == nil {
		return 0
	}
	ordID, err := strconv.ParseInt(*a.OrderID, 10, 64)
	if err != nil {
		return 0
	}

	return ordID
}
