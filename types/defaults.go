package types

import (
	"binance_bot/algos"
	"binance_bot/models"
	"binance_bot/strategies"
)

var (
	DefaultMarketState = MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 1.5,
			StrategyType:        strategies.RSIMACDStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   12,
				SlowPeriod:   26,
				SignalPeriod: 9,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.5,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   15,
				Period:     14,
			},
			// Standard Ichimoku settings for Default (neutral baseline)
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CandleInterval:            "12h",
			DesiredProfit:             2.0,
			HighestPriceFallOffMargin: 0.8,
			FeeRate:                   0.001,
			MarketState:               models.Default,
		},
	}

	ChaoticMarketState = MarketStateStrategy{
		Enabled: false,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 1.8,
			RSI: &algos.RSIStrategy{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   6,
				SlowPeriod:   13,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  3.0,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 90,
				Oversold:   10,
				Period:     7,
			},
			// More responsive Ichimoku settings for Chaotic markets
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 7,
				BasePeriod:       22,
				SpanBPeriod:      44,
			},
			CandleInterval:            "4h",
			DesiredProfit:             3.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.Chaotic,
		},
	}

	TrendingMarketState = MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 1.1,
			RSI: &algos.RSIStrategy{
				Overbought: 75,
				Oversold:   25,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   12,
				SlowPeriod:   26,
				SignalPeriod: 9,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.5,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			// Standard Ichimoku settings for Trending (classic Ichimoku environment)
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CandleInterval:            "1d",
			DesiredProfit:             10.0,
			HighestPriceFallOffMargin: 5.0,
			FeeRate:                   0.001,
			MarketState:               models.Trending,
		},
	}

	RangeBoundMarketState = MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 1.3,
			MACD: &algos.MACDStrategy{
				FastPeriod:   9,
				SlowPeriod:   21,
				SignalPeriod: 7,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 65,
				Oversold:   35,
				Period:     14,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.2,
			},
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			// Standard Ichimoku for Range-bound, helps detect breakouts
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CandleInterval:            "4h",
			DesiredProfit:             2.5,
			HighestPriceFallOffMargin: 3.0,
			FeeRate:                   0.001,
			MarketState:               models.RangeBound,
		},
	}
)
