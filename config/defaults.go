package config

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/M1chlCZ/bingo-bot/types"
)

// This configuration tries to be more crypto-friendly and trade more frequently.
// Adjust intervals, thresholds, and periods to be more lenient

var (
	// DefaultMarketState ---------------- DEFAULT ----------------
	DefaultMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			// Basic risk/logic config
			PanicSell:           false,
			RiskRewardThreshold: 0.5,
			StrategyType:        strategies.RSIMACDStrategyType,

			// RSI
			RSI: &algos.RSIStrategy{
				Overbought: 65,
				Oversold:   28,
				Period:     12,
			},
			// MACD
			MACD: &algos.MACDStrategy{
				FastPeriod:   6,
				SlowPeriod:   15,
				SignalPeriod: 3,
			},
			// Bollinger
			BollingerBands: &algos.BollingerBands{
				Period: 12,
				Width:  2.2,
			},
			// Stochastic
			Stochastic: &algos.StochasticOscillator{
				Overbought: 65,
				Oversold:   28,
				Period:     10,
			},
			// Ichimoku
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 7,
				BasePeriod:       24,
				SpanBPeriod:      44,
			},
			// CCI
			CCI: &algos.CCIStrategy{
				Period:     14,
				Overbought: 100,
				Oversold:   -100,
			},
			// MFI
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 80,
				Oversold:   20,
			},

			CandleInterval:            "12h",
			DesiredProfit:             1.0,
			HighestPriceFallOffMargin: 0.5,
			FeeRate:                   0.001,
			MarketState:               models.Default,
		},
	}

	// ChaoticMarketState ---------------- CHAOTIC ----------------
	ChaoticMarketState = types.MarketStateStrategy{
		Enabled: false,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.7,
			RSI: &algos.RSIStrategy{
				Overbought: 60,
				Oversold:   28,
				Period:     10,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   5,
				SlowPeriod:   12,
				SignalPeriod: 4,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 7,
				Width:  2.2,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   18,
				Period:     7,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 7,
				BasePeriod:       22,
				SpanBPeriod:      44,
			},
			CCI: &algos.CCIStrategy{
				Period:     7,
				Overbought: 120,
				Oversold:   -120,
			},
			MFI: &algos.MFIStrategy{
				Period:     7,
				Overbought: 75,
				Oversold:   15,
			},
			CandleInterval:            "4h",
			DesiredProfit:             1.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.Chaotic,
		},
	}

	// TrendingMarketState ---------------- TRENDING ----------------
	TrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 1.0,
			RSI: &algos.RSIStrategy{
				Overbought: 68,
				Oversold:   32,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   10,
				SlowPeriod:   20,
				SignalPeriod: 7,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.3,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 75,
				Oversold:   25,
				Period:     14,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CCI: &algos.CCIStrategy{
				Period:     20,
				Overbought: 100,
				Oversold:   -25,
			},
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 80,
				Oversold:   20,
			},
			CandleInterval:            "12h",
			DesiredProfit:             5.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.Trending,
		},
	}

	// RangeBoundMarketState ---------------- RANGE-BOUND ----------------
	RangeBoundMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.8,
			MACD: &algos.MACDStrategy{
				FastPeriod:   9,
				SlowPeriod:   21,
				SignalPeriod: 7,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 60,
				Oversold:   40,
				Period:     14,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 15,
				Width:  2.0,
			},
			RSI: &algos.RSIStrategy{
				Overbought: 65,
				Oversold:   35,
				Period:     10,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CCI: &algos.CCIStrategy{
				Period:     14,
				Overbought: 100,
				Oversold:   -100,
			},
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 80,
				Oversold:   20,
			},
			CandleInterval:            "1d",
			DesiredProfit:             2.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.RangeBound,
		},
	}

	// StronglyTrendingMarketState ---------------- STRONGLY TRENDING ----------------
	StronglyTrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           true,
			RiskRewardThreshold: 1.2,
			RSI: &algos.RSIStrategy{
				Overbought: 65,
				Oversold:   30,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   10,
				SlowPeriod:   20,
				SignalPeriod: 6,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  1.8,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 75,
				Oversold:   25,
				Period:     14,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CCI: &algos.CCIStrategy{
				Period:     20,
				Overbought: 150,
				Oversold:   -150,
			},
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 85,
				Oversold:   15,
			},
			CandleInterval:            "12h",
			DesiredProfit:             8.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.StronglyTrending,
		},
	}

	// TransitionalMarketState ---------------- TRANSITIONAL ----------------
	TransitionalMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.6,
			RSI: &algos.RSIStrategy{
				Overbought: 80,
				Oversold:   20,
				Period:     10,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   8,
				SlowPeriod:   17,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  3.0,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   15,
				Period:     10,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 7,
				BasePeriod:       22,
				SpanBPeriod:      44,
			},
			CCI: &algos.CCIStrategy{
				Period:     14,
				Overbought: 100,
				Oversold:   -100,
			},
			MFI: &algos.MFIStrategy{
				Period:     10,
				Overbought: 80,
				Oversold:   20,
			},
			CandleInterval:            "4h",
			DesiredProfit:             1.0,
			HighestPriceFallOffMargin: 0.8,
			FeeRate:                   0.001,
			MarketState:               models.Transitional,
		},
	}
)
