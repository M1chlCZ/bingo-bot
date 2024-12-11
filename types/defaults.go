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
			StrategyType: strategies.RSIMACDStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   35,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   12,
				SlowPeriod:   26,
				SignalPeriod: 9,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.0,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			CandleInterval:            "1h",
			DesiredProfit:             1.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.Default,
		},
	}
	ChaoticMarketState = MarketStateStrategy{
		Enabled: false,
		Strategy: &strategies.CompoundStrategy{
			MACD: &algos.MACDStrategy{
				FastPeriod:   6,
				SlowPeriod:   13,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  2.5,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   15,
				Period:     7,
			},
			CandleInterval:            "30m",
			DesiredProfit:             2.5,
			HighestPriceFallOffMargin: 1.5,
			FeeRate:                   0.001,
			MarketState:               models.Chaotic,
		},
	}
	TrendingMarketState = MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   12,
				SlowPeriod:   26,
				SignalPeriod: 9,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.0,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 80,
				Oversold:   20,
				Period:     14,
			},
			CandleInterval:            "4h",
			DesiredProfit:             8.0,
			HighestPriceFallOffMargin: 5.0,
			FeeRate:                   0.001,
			MarketState:               models.Trending,
		},
	}
	RangeBoundMarketState = MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			MACD: &algos.MACDStrategy{
				FastPeriod:   9,
				SlowPeriod:   21,
				SignalPeriod: 7,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.0,
			},
			RSI: &algos.RSIStrategy{
				Overbought: 70,
				Oversold:   30,
				Period:     14,
			},
			CandleInterval:            "1h",
			DesiredProfit:             2.0,
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.RangeBound,
		},
	}
)
