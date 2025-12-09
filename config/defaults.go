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
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Default,
			CandleInterval: "4h",

			RiskRewardThreshold:       1.25,
			DesiredProfit:             1.20,
			HighestPriceFallOffMargin: 0.30,
			FeeRate:                   0.0009,
			PanicSell:                 true,
			SellOnBearish:             true,

			PartialTP1Pct:  0,
			PartialTP1Size: 0,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 10, SlowPeriod: 24, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.2},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
			Keltner:        &algos.KeltnerChannel{Period: 20, Multiplier: 1.8},
		},
	}

	ChaoticMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Chaotic,
			CandleInterval: "1h",

			PanicSell:                 true,
			RiskRewardThreshold:       1.10,
			DesiredProfit:             1.00,
			HighestPriceFallOffMargin: 0.10,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			PartialTP1Pct:  0,
			PartialTP1Size: 0,

			ADR:            &algos.ADRStrategy{Period: 11, Multiplier: 1.8},
			RSI:            &algos.RSIStrategy{Period: 11, Overbought: 65, Oversold: 28},
			MACD:           &algos.MACDStrategy{FastPeriod: 6, SlowPeriod: 19, SignalPeriod: 4},
			BollingerBands: &algos.BollingerBands{Period: 16, Width: 2.8},
			Stochastic:     &algos.StochasticOscillator{Period: 11, DPeriod: 3, Overbought: 85, Oversold: 18},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 8, BasePeriod: 22, SpanBPeriod: 44},
			CCI:            &algos.CCIStrategy{Period: 12, Overbought: 120, Oversold: -120},
			MFI:            &algos.MFIStrategy{Period: 11, Overbought: 75, Oversold: 15},
			Keltner:        &algos.KeltnerChannel{Period: 18, Multiplier: 2.1},
		},
	}

	TrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Trending,
			CandleInterval: "4h",

			RiskRewardThreshold:       1.05,
			DesiredProfit:             1.80,
			HighestPriceFallOffMargin: 0.40,
			FeeRate:                   0.0009,
			PanicSell:                 true,
			SellOnBearish:             true,

			PartialTP1Pct:  0,
			PartialTP1Size: 0,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 10, SlowPeriod: 24, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 26, Width: 2.2},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 20, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
			Keltner:        &algos.KeltnerChannel{Period: 20, Multiplier: 1.9},
		},
	}

	StronglyTrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.StronglyTrending,
			CandleInterval: "4h",

			RiskRewardThreshold:       0.85,
			DesiredProfit:             2.20,
			HighestPriceFallOffMargin: 0.80,
			FeeRate:                   0.0009,
			PanicSell:                 true,
			SellOnBearish:             true,

			PartialTP1Pct:  0,
			PartialTP1Size: 0,

			ADR:            &algos.ADRStrategy{Period: 12, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 68, Oversold: 32},
			MACD:           &algos.MACDStrategy{FastPeriod: 9, SlowPeriod: 26, SignalPeriod: 6},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.0},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 75, Oversold: 25},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
			Keltner:        &algos.KeltnerChannel{Period: 18, Multiplier: 2.0},
		},
	}

	RangeBoundMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.RangeBound,
			CandleInterval: "4h",

			PanicSell:                 true,
			RiskRewardThreshold:       1.30,
			DesiredProfit:             1.10,
			HighestPriceFallOffMargin: 0.40,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			PartialTP1Pct:  0,
			PartialTP1Size: 0,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.25},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 72, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 20, SignalPeriod: 4},
			BollingerBands: &algos.BollingerBands{Period: 26, Width: 2.3},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 20, Overbought: 150, Oversold: -150},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 85, Oversold: 15},
			Keltner:        &algos.KeltnerChannel{Period: 22, Multiplier: 1.8},
		},
	}

	TransitionalMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Transitional,
			CandleInterval: "4h",

			PanicSell:                 true,
			RiskRewardThreshold:       1.40,
			DesiredProfit:             1.30,
			HighestPriceFallOffMargin: 1.20,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			PartialTP1Pct:  0,
			PartialTP1Size: 0,

			ADR:            &algos.ADRStrategy{Period: 12, Multiplier: 1.4},
			RSI:            &algos.RSIStrategy{Period: 12, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 17, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 18, Width: 2.5},
			Stochastic:     &algos.StochasticOscillator{Period: 12, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 7, BasePeriod: 22, SpanBPeriod: 44},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 12, Overbought: 78, Oversold: 22},
			Keltner:        &algos.KeltnerChannel{Period: 18, Multiplier: 1.9},
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
