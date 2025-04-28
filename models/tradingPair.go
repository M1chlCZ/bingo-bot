package models

// TradingPair represents a single trading pair configuration with all necessary
// parameters for trading operations. This structure contains exchange-specific details
// such as precision requirements and minimum order sizes.
type TradingPair struct {
	// Symbol is the trading pair identifier (e.g., "BTCUSDT")
	Symbol string `json:"symbol"`

	// BaseAsset is the primary currency being traded (e.g., "BTC" in "BTCUSDT")
	BaseAsset string `json:"baseAsset"`

	// QuoteAsset is the currency used for pricing (e.g., "USDT" in "BTCUSDT")
	QuoteAsset string `json:"quoteAsset"`

	// TradeAmount is the default amount to trade for this pair
	TradeAmount float64 `json:"tradeAmount"`

	// MinNotional is the minimum order value required by the exchange
	// (price * quantity must be >= MinNotional)
	MinNotional float64 `json:"minNotional"`

	// PricePrecision is the number of decimal places allowed for price values
	PricePrecision int `json:"pricePrecision"`

	// QtyPrecision is the number of decimal places allowed for quantity values
	QtyPrecision int `json:"qtyPrecision"`
}

// NewTradingPair creates a new trading pair instance with the specified symbol.
// Other fields are initialized with empty or zero values and should be populated
// with actual values from the exchange before use.
//
// Parameters:
//   - symbol: The trading pair symbol (e.g., "BTCUSDT")
//
// Returns:
//   - TradingPair: A new trading pair instance with the specified symbol
func NewTradingPair(symbol string) TradingPair {
	return TradingPair{symbol, "", "", 0, 0, 0, 0}
}
