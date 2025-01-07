package config

import (
	"errors"
	"github.com/M1chlCZ/bingo-bot/analysis"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/types"
	"github.com/go-playground/validator/v10"
	"log"
	"time"
)

type MultiTrading struct {
	AutoTrading           bool
	Default               types.MarketStateStrategy `validate:"required"`
	Chaotic               types.MarketStateStrategy `validate:"required"`
	Trending              types.MarketStateStrategy `validate:"required"`
	RangeBound            types.MarketStateStrategy `validate:"required"`
	Transitional          types.MarketStateStrategy `validate:"required"`
	StronglyTrending      types.MarketStateStrategy `validate:"required"`
	ExcludedMarkets       []models.TradingPair
	ExcludedQuoteMarkets  []string
	IncludedBaseMarkets   []string                 `validate:"required"`
	TradingLoopInterval   time.Duration            `validate:"required"`
	AnalysisLoopInterval  time.Duration            `validate:"required"`
	AnalyzerConfig        *analysis.MarketAnalyzer `validate:"required"`
	ThresholdStopTrading  float64                  `validate:"required"` //percentage
	ThresholdStartTrading float64                  `validate:"required"`
}

func DefaultMultiTradingConfig() MultiTrading {
	return MultiTrading{
		AutoTrading:           true,
		Default:               DefaultMarketState,
		Chaotic:               ChaoticMarketState,
		Trending:              TrendingMarketState,
		RangeBound:            RangeBoundMarketState,
		Transitional:          TransitionalMarketState,
		StronglyTrending:      StronglyTrendingMarketState,
		ExcludedMarkets:       []models.TradingPair{},
		ExcludedQuoteMarkets:  []string{"USDC", "USDP", "FDUSD"},
		IncludedBaseMarkets:   []string{"USDT"},
		TradingLoopInterval:   10 * time.Second,
		AnalysisLoopInterval:  30 * time.Minute,
		ThresholdStartTrading: 0.25,
		ThresholdStopTrading:  0.75,
		AnalyzerConfig: analysis.NewMarketAnalyzer(analysis.MarketAnalyzer{
			ATRPeriod:                15,
			ADXPeriod:                15,
			HighVolatilityThreshold:  0.03,
			StrongTrendThreshold:     25,
			IchimokuConversionPeriod: 9,
			IchimokuBasePeriod:       26,
			IchimokuSpanBPeriod:      52,
			VolumeThreshold:          10000.0,
			FractalLookback:          20,
			// optional MFI and CCI
			MFIPeriod:     14,
			MFIOverbought: 80,
			MFIOversold:   20,
			CCIPeriod:     20,
			CCIOverbought: 100,
			CCIOversold:   -100,
		}),
	}
}

func (c *MultiTrading) UpdateStrategy(state models.MarketState, strategy types.MarketStateStrategy) {
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
	case models.Transitional:
		c.Transitional = strategy
	case models.StronglyTrending:
		c.StronglyTrending = strategy

	}
}

func (c *MultiTrading) UpdateAnalyzerConfig(analyzerConfig *analysis.MarketAnalyzer) {
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

func (c *MultiTrading) UpdateTradingLoopInterval(interval time.Duration) {
	c.TradingLoopInterval = interval
}

func (c *MultiTrading) UpdateExcludedMarkets(markets []models.TradingPair) {
	c.ExcludedMarkets = markets
}

func (c *MultiTrading) UpdateExcludedQuoteMarkets(markets []string) {
	c.ExcludedQuoteMarkets = markets
}

func (c *MultiTrading) UpdateIncludedBaseMarkets(markets []string) {
	c.IncludedBaseMarkets = markets
}
