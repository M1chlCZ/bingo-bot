package config

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/M1chlCZ/bingo-bot/types"
)

var (
	DefaultMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:              strategies.CompoundStrategyType,
			MarketState:               models.Default,
			CandleInterval:            "1h",
			PanicSell:                 true,
			RiskRewardThreshold:       1.20,
			DesiredProfit:             1.60,
			HighestPriceFallOffMargin: 0.90,
			FeeRate:                   0.0009,
			SellOnBearish:             true,
			PartialTP1Pct:             0.90,
			PartialTP1Size:            0.35,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 10, SlowPeriod: 24, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.2},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
		},
	}

	ChaoticMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:              strategies.CompoundStrategyType,
			MarketState:               models.Chaotic,
			CandleInterval:            "1h",
			PanicSell:                 true,
			RiskRewardThreshold:       1.00,
			DesiredProfit:             1.15,
			HighestPriceFallOffMargin: 0.90,
			FeeRate:                   0.0009,
			SellOnBearish:             true,
			PartialTP1Pct:             0.85,
			PartialTP1Size:            0.45,

			ADR:            &algos.ADRStrategy{Period: 11, Multiplier: 1.8},
			RSI:            &algos.RSIStrategy{Period: 11, Overbought: 65, Oversold: 28},
			MACD:           &algos.MACDStrategy{FastPeriod: 6, SlowPeriod: 19, SignalPeriod: 4},
			BollingerBands: &algos.BollingerBands{Period: 16, Width: 2.8},
			Stochastic:     &algos.StochasticOscillator{Period: 11, DPeriod: 3, Overbought: 85, Oversold: 18},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 8, BasePeriod: 22, SpanBPeriod: 44},
			CCI:            &algos.CCIStrategy{Period: 12, Overbought: 120, Oversold: -120},
			MFI:            &algos.MFIStrategy{Period: 11, Overbought: 75, Oversold: 15},
		},
	}

	TrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:              strategies.CompoundStrategyType,
			MarketState:               models.Trending,
			CandleInterval:            "1h",
			RiskRewardThreshold:       0.95,
			DesiredProfit:             2.80,
			HighestPriceFallOffMargin: 0.80,
			FeeRate:                   0.0009,
			SellOnBearish:             true,
			PartialTP1Pct:             1.00,
			PartialTP1Size:            0.33,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 10, SlowPeriod: 24, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 22, Width: 2.2},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 20, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
		},
	}

	StronglyTrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:              strategies.CompoundStrategyType,
			MarketState:               models.StronglyTrending,
			CandleInterval:            "1h",
			RiskRewardThreshold:       0.70,
			DesiredProfit:             3.00,
			HighestPriceFallOffMargin: 0.80,
			FeeRate:                   0.0009,
			SellOnBearish:             true,
			PartialTP1Pct:             1.10,
			PartialTP1Size:            0.30,

			ADR:            &algos.ADRStrategy{Period: 12, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 68, Oversold: 32},
			MACD:           &algos.MACDStrategy{FastPeriod: 9, SlowPeriod: 26, SignalPeriod: 6},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.0},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 75, Oversold: 25},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
		},
	}

	RangeBoundMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:              strategies.CompoundStrategyType,
			MarketState:               models.RangeBound,
			CandleInterval:            "1h",
			PanicSell:                 true,
			RiskRewardThreshold:       1.25,
			DesiredProfit:             1.30,
			HighestPriceFallOffMargin: 0.65,
			FeeRate:                   0.0009,
			SellOnBearish:             true,
			PartialTP1Pct:             0.85,
			PartialTP1Size:            0.40,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.25},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 72, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 20, SignalPeriod: 4},
			BollingerBands: &algos.BollingerBands{Period: 26, Width: 2.3},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 20, Overbought: 150, Oversold: -150},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 85, Oversold: 15},
		},
	}

	TransitionalMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:              strategies.CompoundStrategyType,
			MarketState:               models.Transitional,
			CandleInterval:            "1h",
			PanicSell:                 true,
			RiskRewardThreshold:       1.35,
			DesiredProfit:             1.50,
			HighestPriceFallOffMargin: 0.90,
			FeeRate:                   0.0009,
			SellOnBearish:             true,
			PartialTP1Pct:             0.95,
			PartialTP1Size:            0.35,

			ADR:            &algos.ADRStrategy{Period: 12, Multiplier: 1.4},
			RSI:            &algos.RSIStrategy{Period: 12, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 17, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 18, Width: 2.5},
			Stochastic:     &algos.StochasticOscillator{Period: 12, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 7, BasePeriod: 22, SpanBPeriod: 44},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 12, Overbought: 78, Oversold: 22},
		},
	}

	BacktestConfig = struct {
		DefaultSymbol        string
		DefaultInterval      string
		DefaultStrategy      string
		DefaultBalance       float64
		FetchLookbackDays    int
		BacktestLookbackDays int
	}{
		DefaultSymbol:        "BTCUSDT",
		DefaultInterval:      "1h",
		DefaultStrategy:      "compound",
		DefaultBalance:       10000.0,
		FetchLookbackDays:    365,
		BacktestLookbackDays: 120,
	}
)
