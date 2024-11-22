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
