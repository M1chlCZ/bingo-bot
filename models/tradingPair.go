package models

// TradingPair represents a single trading pair configuration
type TradingPair struct {
	Symbol         string  `json:"symbol"`
	BaseAsset      string  `json:"baseAsset"`
	QuoteAsset     string  `json:"quoteAsset"`
	TradeAmount    float64 `json:"tradeAmount"`
	MinNotional    float64 `json:"minNotional"`
	PricePrecision int     `json:"pricePrecision"`
	QtyPrecision   int     `json:"qtyPrecision"`
}

func NewTradingPair(symbol string) TradingPair {
	// Initialize a new trading pair with the symbol, values will be fetched from the exchange
	return TradingPair{symbol, "", "", 0, 0, 0, 0}
}
