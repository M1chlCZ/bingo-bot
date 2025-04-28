package config

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/M1chlCZ/bingo-bot/types"
)

// This configuration tries to be more crypto-friendly and trade more frequently.

var (
	// ---------- DEFAULT ----------
	DefaultMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Default,
			CandleInterval: "4h",

			RiskRewardThreshold:       0.6,
			DesiredProfit:             1.6,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 21, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.2},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
		},
	}

	// ---------- CHAOTIC ----------
	ChaoticMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Chaotic,
			CandleInterval: "1h",

			RiskRewardThreshold:       0.9,
			DesiredProfit:             1.2,
			HighestPriceFallOffMargin: 0.6,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			ADR:            &algos.ADRStrategy{Period: 7, Multiplier: 1.8},
			RSI:            &algos.RSIStrategy{Period: 10, Overbought: 65, Oversold: 28},
			MACD:           &algos.MACDStrategy{FastPeriod: 6, SlowPeriod: 19, SignalPeriod: 4},
			BollingerBands: &algos.BollingerBands{Period: 12, Width: 2.8},
			Stochastic:     &algos.StochasticOscillator{Period: 11, DPeriod: 3, Overbought: 85, Oversold: 18},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 8, BasePeriod: 22, SpanBPeriod: 44},
			CCI:            &algos.CCIStrategy{Period: 10, Overbought: 120, Oversold: -120},
			MFI:            &algos.MFIStrategy{Period: 10, Overbought: 75, Oversold: 15},
		},
	}

	// ---------- TRENDING ----------
	TrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Trending,
			CandleInterval: "4h",

			RiskRewardThreshold:       1.1,
			DesiredProfit:             4.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			ADR:            &algos.ADRStrategy{Period: 14, Multiplier: 1.2},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 40},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 21, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.2},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 20, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
		},
	}

	// ---------- RANGE‑BOUND ----------
	RangeBoundMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.RangeBound,
			CandleInterval: "4h",

			RiskRewardThreshold:       0.7,
			DesiredProfit:             1.5,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			ADR:            &algos.ADRStrategy{Period: 10, Multiplier: 1.5},
			RSI:            &algos.RSIStrategy{Period: 10, Overbought: 68, Oversold: 32},
			MACD:           &algos.MACDStrategy{FastPeriod: 9, SlowPeriod: 26, SignalPeriod: 6},
			BollingerBands: &algos.BollingerBands{Period: 20, Width: 2.0},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 75, Oversold: 25},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 80, Oversold: 20},
		},
	}

	// ---------- STRONGLY TRENDING ----------
	StronglyTrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.StronglyTrending,
			CandleInterval: "4h",

			RiskRewardThreshold:       1.3,
			DesiredProfit:             6.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.0009,
			SellOnBearish:             true,

			ADR:            &algos.ADRStrategy{Period: 7, Multiplier: 1.2},
			RSI:            &algos.RSIStrategy{Period: 14, Overbought: 70, Oversold: 40},
			MACD:           &algos.MACDStrategy{FastPeriod: 5, SlowPeriod: 21, SignalPeriod: 4},
			BollingerBands: &algos.BollingerBands{Period: 21, Width: 2.3},
			Stochastic:     &algos.StochasticOscillator{Period: 14, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 9, BasePeriod: 26, SpanBPeriod: 52},
			CCI:            &algos.CCIStrategy{Period: 20, Overbought: 150, Oversold: -150},
			MFI:            &algos.MFIStrategy{Period: 14, Overbought: 85, Oversold: 15},
		},
	}

	// ---------- TRANSITIONAL ----------
	TransitionalMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			StrategyType:   strategies.CompoundStrategyType,
			MarketState:    models.Transitional,
			CandleInterval: "4h",

			RiskRewardThreshold:       0.8,
			DesiredProfit:             2.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.0009,
			SellOnBearish:             false,

			ADR:            &algos.ADRStrategy{Period: 10, Multiplier: 1.4},
			RSI:            &algos.RSIStrategy{Period: 10, Overbought: 70, Oversold: 30},
			MACD:           &algos.MACDStrategy{FastPeriod: 8, SlowPeriod: 17, SignalPeriod: 5},
			BollingerBands: &algos.BollingerBands{Period: 18, Width: 2.5},
			Stochastic:     &algos.StochasticOscillator{Period: 12, DPeriod: 3, Overbought: 80, Oversold: 20},
			Ichimoku:       &algos.IchimokuStrategy{ConversionPeriod: 7, BasePeriod: 22, SpanBPeriod: 44},
			CCI:            &algos.CCIStrategy{Period: 14, Overbought: 100, Oversold: -100},
			MFI:            &algos.MFIStrategy{Period: 10, Overbought: 80, Oversold: 20},
		},
	}

	// ---------- BACKTESTING ----------
	BacktestConfig = struct {
		DefaultSymbol        string  // Default trading pair symbol
		DefaultInterval      string  // Default candle interval
		DefaultStrategy      string  // Default strategy to use
		DefaultBalance       float64 // Default initial balance
		FetchLookbackDays    int     // Default number of days to look back when fetching data
		BacktestLookbackDays int     // Default number of days to look back when running a backtest
	}{
		DefaultSymbol:        "BTCUSDT",
		DefaultInterval:      "1h",
		DefaultStrategy:      "compound", // Compound strategy is the default
		DefaultBalance:       10000.0,
		FetchLookbackDays:    365, // 1 year
		BacktestLookbackDays: 180, // 6 months
	}
)
