package types

import (
	"errors"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
)

type MarketStateStrategy struct {
	Enabled  bool                `json:"enabled"`
	Strategy interfaces.Strategy `validate:"required_if=Enabled true" json:"strategy"`
}

func (m *MarketStateStrategy) IsZero() bool {
	return !m.Enabled && m.Strategy == nil
}

func (m *MarketStateStrategy) Validate() error {
	if !m.Enabled {

		return nil
	}

	if m.Strategy == nil {
		return errors.New("strategy is required when Enabled is true")
	}

	v := validator.New()
	return v.Struct(m)
}

func (m *MarketStateStrategy) UnmarshalJSON(data []byte) error {
	type auxMarketStateStrategy struct {
		Enabled  bool            `json:"enabled"`
		Strategy json.RawMessage `json:"strategy"`
	}

	var aux auxMarketStateStrategy
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	m.Enabled = aux.Enabled

	if len(aux.Strategy) == 0 {

		return nil
	}

	var detect map[string]interface{}
	if err := json.Unmarshal(aux.Strategy, &detect); err != nil {
		return err
	}

	switch detect["strategyType"] {
	case strategies.CompoundStrategyType.String():
		var cs strategies.CompoundStrategy
		if err := json.Unmarshal(aux.Strategy, &cs); err != nil {
			return err
		}
		m.Strategy = &cs

	case strategies.SpikeDetectionStrategyType.String():
		var cs strategies.SpikeStrategy
		if err := json.Unmarshal(aux.Strategy, &cs); err != nil {
			return err
		}
		m.Strategy = &cs

	default:
		return fmt.Errorf("unknown strategy type: %v", detect["strategyType"])
	}

	return nil
}
