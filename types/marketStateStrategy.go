package types

import (
	"binance_bot/strategies"
	"errors"
	"github.com/go-playground/validator/v10"
)

type MarketStateStrategy struct {
	Enabled  bool                         `validate:"required"`
	Strategy *strategies.CompoundStrategy `validate:"required_if=Enabled true"`
}

func (m MarketStateStrategy) IsZero() bool {
	return !m.Enabled && m.Strategy == nil
}

func (m MarketStateStrategy) Validate() error {
	if !m.Enabled {
		// When not enabled, no need to validate the strategy
		return nil
	}

	// If enabled, then ensure Strategy is not nil and valid
	if m.Strategy == nil {
		return errors.New("strategy is required when Enabled is true")
	}

	// If Strategy is not nil and you still want to validate it with validator:
	v := validator.New()
	return v.Struct(m)
}
