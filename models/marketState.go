package models

import (
	"github.com/goccy/go-json"
)

type MarketState int

const (
	Default MarketState = iota

	Trending

	Chaotic

	RangeBound

	StronglyTrending

	Transitional
)

func (m MarketState) String() string {
	switch m {
	case Trending:
		return "Trending"
	case Chaotic:
		return "Chaotic"
	case RangeBound:
		return "RangeBound"
	case StronglyTrending:
		return "StronglyTrending"
	case Transitional:
		return "Transitional"
	default:
		return "Default"
	}
}

func (m MarketState) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(m))
}

func (m *MarketState) UnmarshalJSON(data []byte) error {
	var value int
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	switch MarketState(value) {
	case Trending:
		*m = Trending
	case Chaotic:
		*m = Chaotic
	case RangeBound:
		*m = RangeBound
	case StronglyTrending:
		*m = StronglyTrending
	case Transitional:
		*m = Transitional
	default:
		*m = Default
	}
	return nil
}
