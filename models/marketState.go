package models

type MarketState int

const (
	Default MarketState = iota
	Trending
	Chaotic
	RangeBound
)

// String converts MarketState to a string representation
func (m MarketState) String() string {
	switch m {
	case Trending:
		return "Trending"
	case Chaotic:
		return "Chaotic"
	case RangeBound:
		return "Range-Bound"
	default:
		return "Default"
	}
}
