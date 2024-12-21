package config

import (
	"binance_bot/analysis"
	"binance_bot/logger"
	"binance_bot/models"
	"binance_bot/types"
	"errors"
	"github.com/go-playground/validator/v10"
	"log"
	"time"
)

type ConfigMultiTrading struct {
	AutoTrading          bool
	Default              types.MarketStateStrategy `validate:"required"`
	Chaotic              types.MarketStateStrategy `validate:"required"`
	Trending             types.MarketStateStrategy `validate:"required"`
	RangeBound           types.MarketStateStrategy `validate:"required"`
	Transitional         types.MarketStateStrategy `validate:"required"`
	StronglyTrending     types.MarketStateStrategy `validate:"required"`
	ExcludedMarkets      []models.TradingPair
	ExcludedQuoteMarkets []string
	IncludedBaseMarkets  []string                 `validate:"required"`
	TradingLoopInterval  time.Duration            `validate:"required"`
	AnalysisLoopInterval time.Duration            `validate:"required"`
	AnalyzerConfig       *analysis.MarketAnalyzer `validate:"required"`
}

func DefaultMultiTradingConfig() ConfigMultiTrading {
	return ConfigMultiTrading{
		AutoTrading:          true,
		Default:              DefaultMarketState,
		Chaotic:              ChaoticMarketState,
		Trending:             TrendingMarketState,
		RangeBound:           RangeBoundMarketState,
		Transitional:         TransitionalMarketState,
		StronglyTrending:     StronglyTrendingMarketState,
		ExcludedMarkets:      []models.TradingPair{},
		ExcludedQuoteMarkets: []string{"USDC", "USDP", "FDUSD"},
		IncludedBaseMarkets:  []string{"USDT"},
		TradingLoopInterval:  10 * time.Second,
		AnalysisLoopInterval: 30 * time.Minute,
		AnalyzerConfig: &analysis.MarketAnalyzer{
			ATRPeriod:                15,
			ADXPeriod:                15,
			HighVolatilityThreshold:  0.03,
			StrongTrendThreshold:     25,
			IchimokuConversionPeriod: 9,
			IchimokuBasePeriod:       26,
			IchimokuSpanBPeriod:      52,
			VolumeThreshold:          10000.0,
			FractalLookback:          20,
		},
	}
}

func (c *ConfigMultiTrading) UpdateStrategy(state models.MarketState, strategy types.MarketStateStrategy) {
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
