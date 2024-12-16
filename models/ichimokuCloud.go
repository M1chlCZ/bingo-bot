package models

type IchimokuCloud int

const (
	Neutral IchimokuCloud = iota
	Bullish
	Bearish
)

// String converts MarketState to a string representation
func (i IchimokuCloud) String() string {
	switch i {
	case Neutral:
		return "neutral"
	case Bullish:
		return "bullish"
	case Bearish:
		return "bearish"
	default:
		return "neutral"
	}
}
