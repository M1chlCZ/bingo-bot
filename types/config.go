package types

import (
	"binance_bot/analysis"
	"binance_bot/logger"
	"binance_bot/models"
	"errors"
	"github.com/go-playground/validator/v10"
	"log"
	"time"
)

type ConfigMultiTrading struct {
	AutoTrading          bool
	Default              MarketStateStrategy `validate:"required"`
	Chaotic              MarketStateStrategy `validate:"required"`
	Trending             MarketStateStrategy `validate:"required"`
	RangeBound           MarketStateStrategy `validate:"required"`
	ExcludedMarkets      []models.TradingPair
	ExcludedQuoteMarkets []string
	IncludedBaseMarkets  []string
	TradingLoopInterval  time.Duration
	AnalysisLoopInterval time.Duration
	AnalyzerConfig       *analysis.MarketAnalyzer
}

func DefaultMultiTradingConfig() ConfigMultiTrading {
	return ConfigMultiTrading{
		AutoTrading:          true,
		Default:              DefaultMarketState,
		Chaotic:              ChaoticMarketState,
		Trending:             TrendingMarketState,
		RangeBound:           RangeBoundMarketState,
		ExcludedMarkets:      []models.TradingPair{},
		ExcludedQuoteMarkets: []string{"USDC", "USDP", "FDUSD"},
		IncludedBaseMarkets:  []string{"USDT"},
		TradingLoopInterval:  2 * time.Minute,
		AnalysisLoopInterval: 30 * time.Minute,
		AnalyzerConfig: &analysis.MarketAnalyzer{
			ATRPeriod:                14,
			ADXPeriod:                14,
			HighVolatilityThreshold:  0.03,
			StrongTrendThreshold:     25,
			IchimokuConversionPeriod: 9,
			IchimokuBasePeriod:       26,
			IchimokuSpanBPeriod:      52,
			VolumeThreshold:          100000.0,
			FractalLookback:          20,
		},
	}
}

func (c *ConfigMultiTrading) UpdateStrategy(state models.MarketState, strategy MarketStateStrategy) {
	err := strategy.Validate()
	if err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, fe := range validationErrors {
				logger.Errorf("Validation failed for field '%s': violated '%s' rule", fe.Field(), fe.Tag())
			}
		}
		log.Fatalf("Invalid strategy configuration for market state %s", state)
	}
	switch state {
	case models.Default:
		c.Default = strategy
	case models.Chaotic:
		c.Chaotic = strategy
	case models.Trending:
		c.Trending = strategy
	case models.RangeBound:
		c.RangeBound = strategy
	}
}

func (c *ConfigMultiTrading) UpdateAnalyzerConfig(analyzerConfig *analysis.MarketAnalyzer) {
	err := analyzerConfig.Validate()
	if err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, fe := range validationErrors {
				logger.Errorf("Validation failed for field '%s': violated '%s' rule", fe.Field(), fe.Tag())
			}
		}
		log.Fatalf("Invalid analyzer configuration")
	}
	c.AnalyzerConfig = analyzerConfig
}

func (c *ConfigMultiTrading) UpdateTradingLoopInterval(interval time.Duration) {
	c.TradingLoopInterval = interval
}

func (c *ConfigMultiTrading) UpdateExcludedMarkets(markets []models.TradingPair) {
	c.ExcludedMarkets = markets
}

func (c *ConfigMultiTrading) UpdateExcludedQuoteMarkets(markets []string) {
	c.ExcludedQuoteMarkets = markets
}

func (c *ConfigMultiTrading) UpdateIncludedBaseMarkets(markets []string) {
	c.IncludedBaseMarkets = markets
}
