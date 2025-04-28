package models

import (
	"github.com/goccy/go-json"
)

// MarketState represents the current state or condition of a market,
// which influences the trading strategy selection and decision-making process.
// Different market states require different approaches to trading.
type MarketState int

const (
	// Default is the fallback market state when no specific state can be determined
	Default MarketState = iota

	// Trending indicates a market with a clear directional movement
	// Suitable for trend-following strategies
	Trending

	// Chaotic represents a volatile market with unpredictable price movements
	// Requires more conservative position sizing and tighter risk management
	Chaotic

	// RangeBound indicates a market oscillating between support and resistance levels
	// Suitable for mean-reversion strategies
	RangeBound

	// StronglyTrending represents a market with strong momentum in one direction
	// Optimal for aggressive trend-following strategies
	StronglyTrending

	// Transitional indicates a market that is changing from one state to another
	// Requires cautious approach as the new trend establishes
	Transitional
)

// String returns a string representation of the market state.
// This is useful for logging and debugging purposes.
//
// Returns:
//   - string: Human-readable name of the market state
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

// MarshalJSON implements the json.Marshaler interface for MarketState.
// This allows MarketState values to be properly serialized to JSON.
//
// Returns:
//   - []byte: JSON representation of the market state as an integer
//   - error: Any error encountered during marshaling
func (m MarketState) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(m))
}

// UnmarshalJSON implements the json.Unmarshaler interface for MarketState.
// This allows MarketState values to be properly deserialized from JSON.
//
// Parameters:
//   - data: JSON data to unmarshal
//
// Returns:
//   - error: Any error encountered during unmarshaling
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
