package models

type ActiveTrade struct {
	ID       int     `json:"id" db:"id"`
	OrderID  *string `json:"order_id" db:"order_id"`
	Symbol   string  `json:"symbol" db:"symbol"`
	BuyPrice float64 `json:"buy_price" db:"buy_price"`
	Quantity float64 `json:"quantity" db:"quantity"`
}
