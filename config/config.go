package config

import (
	"errors"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/analysis"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/types"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"os"
	"time"
)

type MultiTrading struct {
	AutoTrading      bool                      `json:"autoTrading" validate:"boolean"`
	Default          types.MarketStateStrategy `validate:"required" json:"default"`
	Chaotic          types.MarketStateStrategy `validate:"required" json:"chaotic"`
	Trending         types.MarketStateStrategy `validate:"required" json:"trending"`
	RangeBound       types.MarketStateStrategy `validate:"required" json:"rangeBound"`
	Transitional     types.MarketStateStrategy `validate:"required" json:"transitional"`
	StronglyTrending types.MarketStateStrategy `validate:"required" json:"stronglyTrending"`

	ExcludedMarkets      []models.TradingPair `validate:"dive" json:"excludedMarkets"`
	ExcludedQuoteMarkets []string             `validate:"dive,min=1" json:"excludedQuoteMarkets"`
	IncludedBaseMarkets  []string             `validate:"required,dive,min=1" json:"includedBaseMarkets"`

	TradingLoopInterval  time.Duration            `validate:"required,min=1" json:"tradingLoopInterval"`
	AnalysisLoopInterval time.Duration            `validate:"required,min=1" json:"analysisLoopInterval"`
	AnalyzerConfig       *analysis.MarketAnalyzer `validate:"required" json:"analyzerConfig"`

	ThresholdStopTrading  float64 `validate:"gte=0" json:"thresholdStopTrading"`
	ThresholdStartTrading float64 `validate:"gte=0" json:"thresholdStartTrading"`

	PendingBuyCoolDown time.Duration `validate:"required,min=1" json:"pendingBuyCoolDown"`
	MaxDailyTrades     int           `validate:"gte=0" json:"maxDailyTrades"`
	MaxTotalTrades     int           `validate:"gte=0" json:"maxTotalTrades"`
}

func DefaultMultiTradingConfig() MultiTrading {
	return MultiTrading{
		AutoTrading: true,

		Default:          DefaultMarketState,
		Chaotic:          ChaoticMarketState,
		Trending:         TrendingMarketState,
		RangeBound:       RangeBoundMarketState,
		Transitional:     TransitionalMarketState,
		StronglyTrending: StronglyTrendingMarketState,

		ExcludedMarkets:      []models.TradingPair{},
		ExcludedQuoteMarkets: []string{"USDT", "USDP", "FDUSD", "TUSD", "EUR", "EURI"},
		IncludedBaseMarkets:  []string{"USDC"},

		TradingLoopInterval:  15 * time.Second,
		AnalysisLoopInterval: 120 * time.Minute,

		ThresholdStartTrading: 0,
		ThresholdStopTrading:  0,

		PendingBuyCoolDown: 2 * time.Minute,
		MaxDailyTrades:     40,
		MaxTotalTrades:     40,

		AnalyzerConfig: analysis.NewMarketAnalyzer(analysis.MarketAnalyzer{
			EMAPeriods:               []int{9, 21, 55},
			ATRPeriod:                14,
			ADXPeriod:                14,
			HighVolatilityThreshold:  0.035,
			StrongTrendThreshold:     25,
			IchimokuConversionPeriod: 9,
			IchimokuBasePeriod:       26,
			IchimokuSpanBPeriod:      52,
			VolumeThreshold:          15000,
			FractalLookback:          20,

			MFIPeriod:     14,
			MFIOverbought: 80,
			MFIOversold:   20,
			CCIPeriod:     20,
			CCIOverbought: 100,
			CCIOversold:   -100,
		}),
	}
}

func MultiTradingConfigFromJSON(filePath string) (MultiTrading, error) {
	configJSON, err := os.ReadFile(filePath)
	if err != nil {
		return MultiTrading{}, err
	}
	var config MultiTrading
	err = json.Unmarshal(configJSON, &config)
	if err != nil {
		return MultiTrading{}, err
	}
	fmt.Println(config)
	err = config.Validate()
	if err != nil {
		return MultiTrading{}, err
	}
	return config, nil
}

func (c *MultiTrading) UpdateStrategy(state models.MarketState, strategy types.MarketStateStrategy) error {
	// Validate the market state
	if state < models.Default || state > models.StronglyTrending {
		return fmt.Errorf("invalid market state: %v", state)
	}

	err := strategy.Validate()
	if err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, fe := range validationErrors {
				logger.Errorf("Validation failed for field '%s': violated '%s' rule", fe.Field(), fe.Tag())
			}
		}
		return fmt.Errorf("invalid strategy configuration for market state %s: %w", state, err)
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

	return nil
}

func (c *MultiTrading) UpdateAnalyzerConfig(analyzerConfig *analysis.MarketAnalyzer) error {
	if analyzerConfig == nil {
		return fmt.Errorf("analyzer config cannot be nil")
	}

	err := analyzerConfig.Validate()
	if err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			for _, fe := range validationErrors {
				logger.Errorf("Validation failed for field '%s': violated '%s' rule", fe.Field(), fe.Tag())
			}
		}
		return fmt.Errorf("invalid analyzer configuration: %w", err)
	}
	c.AnalyzerConfig = analyzerConfig
	return nil
}

func (c *MultiTrading) UpdateTradingLoopInterval(interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("trading loop interval must be positive")
	}
	c.TradingLoopInterval = interval
	return nil
}

func (c *MultiTrading) UpdateAnalysisLoopInterval(interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("analysis loop interval must be positive")
	}
	c.AnalysisLoopInterval = interval
	return nil
}

func (c *MultiTrading) UpdateExcludedMarkets(markets []models.TradingPair) error {
	// Validate each trading pair in the excluded markets
	for i, market := range markets {
		if market.Symbol == "" {
			return fmt.Errorf("excluded market at index %d has empty symbol", i)
		}
		if market.BaseAsset == "" {
			return fmt.Errorf("excluded market at index %d has empty base asset", i)
		}
		if market.QuoteAsset == "" {
			return fmt.Errorf("excluded market at index %d has empty quote asset", i)
		}
	}
	c.ExcludedMarkets = markets
	return nil
}

func (c *MultiTrading) UpdateExcludedQuoteMarkets(markets []string) error {
	for _, market := range markets {
		if market == "" {
			return fmt.Errorf("excluded quote market cannot be empty")
		}
	}
	c.ExcludedQuoteMarkets = markets
	return nil
}

func (c *MultiTrading) UpdateIncludedBaseMarkets(markets []string) error {
	if len(markets) == 0 {
		return fmt.Errorf("included base markets cannot be empty")
	}
	for _, market := range markets {
		if market == "" {
			return fmt.Errorf("included base market cannot be empty")
		}
	}
	c.IncludedBaseMarkets = markets
	return nil
}

func validMarketState(fl validator.FieldLevel) bool {
	state := fl.Field().Int() // the int value
	return state >= 0 && state <= 5
}

// validPositiveDuration validates that a time.Duration is positive
func validPositiveDuration(fl validator.FieldLevel) bool {
	duration := fl.Field().Interface().(time.Duration)
	return duration > 0
}

// validThresholds validates that ThresholdStartTrading is less than or equal to ThresholdStopTrading
func validThresholds(sl validator.StructLevel) {
	config := sl.Current().Interface().(MultiTrading)
	if config.ThresholdStartTrading > config.ThresholdStopTrading && config.ThresholdStopTrading > 0 {
		sl.ReportError(config.ThresholdStartTrading, "ThresholdStartTrading", "thresholdStartTrading", "lte_threshold_stop", "")
	}
}

// validMaxTrades validates that MaxDailyTrades is less than or equal to MaxTotalTrades
func validMaxTrades(sl validator.StructLevel) {
	config := sl.Current().Interface().(MultiTrading)
	if config.MaxDailyTrades > config.MaxTotalTrades && config.MaxTotalTrades > 0 {
		sl.ReportError(config.MaxDailyTrades, "MaxDailyTrades", "maxDailyTrades", "lte_max_total", "")
	}
}

func (c *MultiTrading) Validate() error {
	validate := validator.New()

	// Register custom validation functions
	if err := validate.RegisterValidation("marketStateEnum", validMarketState); err != nil {
		return fmt.Errorf("failed to register marketStateEnum validation: %w", err)
	}

	if err := validate.RegisterValidation("min", validPositiveDuration); err != nil {
		return fmt.Errorf("failed to register min validation: %w", err)
	}

	// Register struct-level validations
	validate.RegisterStructValidation(validThresholds, MultiTrading{})
	validate.RegisterStructValidation(validMaxTrades, MultiTrading{})

	return validate.Struct(c)
}
