package models

// TradingPair represents a single trading pair configuration
type TradingPair struct {
	Symbol         string
	BaseAsset      string
	QuoteAsset     string
	TradeAmount    float64
	MinNotional    float64
	PricePrecision int
	QtyPrecision   int
}

func NewTradingPair(symbol string) TradingPair {
	// Initialize a new trading pair with the symbol, values will be fetched from the exchange
	return TradingPair{symbol, "", "", 0, 0, 0, 0}
}
