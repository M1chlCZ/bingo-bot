package strategies

import (
	"errors"
	"github.com/goccy/go-json"
)

type StrategyType struct {
	Value string
}

var (
	CompoundStrategyType       = StrategyType{"compound"}
	SpikeDetectionStrategyType = StrategyType{"spike-detection"}
)

func (s StrategyType) String() string {
	return s.Value
}

func (s StrategyType) IsValid() bool {
	switch s {
	case CompoundStrategyType, SpikeDetectionStrategyType:
		return true
	default:
		return false
	}
}

func (s StrategyType) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *StrategyType) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	switch value {
	case CompoundStrategyType.String():
		s.Value = value
	case SpikeDetectionStrategyType.String():
		s.Value = value
	default:
		return errors.New("invalid strategy type")
	}

	return nil
}
