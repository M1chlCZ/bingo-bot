package analysis

import (
	"github.com/M1chlCZ/bingo-bot/models"
)

type PairAnalysis struct {
	Pair        *models.TradingPair // Reference to the trading pair
	MarketState models.MarketState  // Current market state for the pair
	Volatility  float64             // Calculated volatility
	LastUpdated int64               // Timestamp of the last market analysis
	ATR         float64             // Average True Range
	ADX         float64             // Average Directional Index
}
