package config

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/M1chlCZ/bingo-bot/types"
)

// This configuration tries to be more crypto-friendly and trade more frequently.

var (
	// ------------------ DEFAULT ------------------
	DefaultMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.6, // slightly higher than 0.5, so we skip lower-RR trades
			StrategyType:        strategies.CompoundStrategyType,

			RSI: &algos.RSIStrategy{
				Overbought: 70, // slightly higher for crypto
				Oversold:   30,
				Period:     14, // a bit more standard
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   8,  // a little quicker than the classic 12
				SlowPeriod:   21, // a bit faster than 26
				SignalPeriod: 5,  // typical for crypto
			},
			BollingerBands: &algos.BollingerBands{
				Period: 14,
				Width:  2.2, // narrower than 2.0 -> more frequent signals
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52, // standard
			},
			CCI: &algos.CCIStrategy{
				Period:     14,   // standard
				Overbought: 100,  // typical
				Oversold:   -100, // typical
			},
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 80,
				Oversold:   20,
			},

			CandleInterval:            "4h", // faster than 12h for “default”
			DesiredProfit:             2.0,  // aim for a bit more than 1%
			HighestPriceFallOffMargin: 0.5,
			FeeRate:                   0.001,
			MarketState:               models.Default,
		},
	}

	// ------------------ CHAOTIC ------------------
	ChaoticMarketState = types.MarketStateStrategy{
		Enabled: false, // you can enable if you want
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.7,
			StrategyType:        strategies.CompoundStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 65,
				Oversold:   28,
				Period:     10,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   5,
				SlowPeriod:   15,
				SignalPeriod: 4,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  2.5, // slightly wider to handle big “chaotic” swings
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   18,
				Period:     10,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 8,
				BasePeriod:       22,
				SpanBPeriod:      44,
			},
			CCI: &algos.CCIStrategy{
				Period:     10,
				Overbought: 120,
				Oversold:   -120,
			},
			MFI: &algos.MFIStrategy{
				Period:     10,
				Overbought: 75,
				Oversold:   15,
			},
			CandleInterval:            "2h", // “chaotic” might need shorter intervals
			DesiredProfit:             2.0,
			HighestPriceFallOffMargin: 1.2,
			FeeRate:                   0.001,
			MarketState:               models.Chaotic,
		},
	}

	// ------------------ TRENDING ------------------
	TrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 1.0,
			StrategyType:        strategies.CompoundStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 70, // slightly higher
				Oversold:   30,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   8,
				SlowPeriod:   21,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.2,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
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
				Oversold:   -100,
			},
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 80,
				Oversold:   20,
			},
			CandleInterval:            "6h", // or "4h" or "8h" for trending
			DesiredProfit:             4.0,  // more than default if we think it’s a real trend
			HighestPriceFallOffMargin: 1.2,
			FeeRate:                   0.001,
			MarketState:               models.Trending,
		},
	}

	// ------------------ RANGE-BOUND ------------------
	RangeBoundMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.8,
			StrategyType:        strategies.CompoundStrategyType,
			MACD: &algos.MACDStrategy{
				FastPeriod:   9,
				SlowPeriod:   21,
				SignalPeriod: 7,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 75,
				Oversold:   25,
				Period:     14,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 15,
				Width:  2.0,
			},
			RSI: &algos.RSIStrategy{
				Overbought: 68,
				Oversold:   32,
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
			CandleInterval:            "1d", // For range-bound, a bigger timeframe might be okay
			DesiredProfit:             2.5,  // modest
			HighestPriceFallOffMargin: 1.2,
			FeeRate:                   0.001,
			MarketState:               models.RangeBound,
		},
	}

	// ------------------ STRONGLY TRENDING ------------------
	StronglyTrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 1.2,
			StrategyType:        strategies.CompoundStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 72, // strong push might run RSI higher
				Oversold:   28,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   10,
				SlowPeriod:   20,
				SignalPeriod: 6,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  1.8, // narrower bands for a stronger trend might catch quick breakouts
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CCI: &algos.CCIStrategy{
				Period:     20,
				Overbought: 150, // allow strong uptrends
				Oversold:   -150,
			},
			MFI: &algos.MFIStrategy{
				Period:     14,
				Overbought: 85, // allow bigger rallies
				Oversold:   15,
			},
			CandleInterval:            "6h", // for strong trends, 6h might catch bigger moves
			DesiredProfit:             8.0,
			HighestPriceFallOffMargin: 1.2,
			FeeRate:                   0.001,
			MarketState:               models.StronglyTrending,
		},
	}

	// ------------------ TRANSITIONAL ------------------
	TransitionalMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			PanicSell:           false,
			RiskRewardThreshold: 0.6,
			StrategyType:        strategies.CompoundStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   30,
				Period:     10,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   8,
				SlowPeriod:   17,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  2.5,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
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
			DesiredProfit:             2.0, // slightly bigger than 1
			HighestPriceFallOffMargin: 0.8,
			FeeRate:                   0.001,
			MarketState:               models.Transitional,
		},
	}
)
