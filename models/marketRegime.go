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
