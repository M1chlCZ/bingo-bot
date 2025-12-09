package models

type MarketRegime int

const (
	TrendingRegime MarketRegime = iota
	RangeBoundRegime
	UnknownRegime
	MarketHighVolatilityRegime
	MarketLowVolatilityRegime
	MarketMixedRegime
)

func (mr MarketRegime) String() string {
	switch mr {
	case TrendingRegime:
		return "TrendingRegime"
	case RangeBoundRegime:
		return "Range-BoundRegime"
	case UnknownRegime:
		return "UnknownRegime"
	case MarketHighVolatilityRegime:
		return "High-volatilityRegime"
	case MarketLowVolatilityRegime:
		return "Low-volatilityRegime"
	case MarketMixedRegime:
		return "MixedRegime"
	default:
		return "Invalid Market Regime"
	}
}
