package strategies

// StrategyType defines a type-safe enum-like structure for strategies
type StrategyType struct {
	value string
}

// StrategyType constants
var (
	RSIMACDStrategyType        = StrategyType{"rsi-macd"}
	SpikeDetectionStrategyType = StrategyType{"spike-detection"}
)

// String returns the string representation of the StrategyType
func (s StrategyType) String() string {
	return s.value
}

// IsValid checks if a given value is a valid StrategyType
func (s StrategyType) IsValid() bool {
	switch s {
	case RSIMACDStrategyType, SpikeDetectionStrategyType:
		return true
	default:
		return false
	}
}
