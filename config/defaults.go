package config

import (
	"binance_bot/algos"
	"binance_bot/models"
	"binance_bot/strategies"
	"binance_bot/types"
)

// This configuration tries to be more crypto-friendly and trade more frequently.
// Adjusted intervals, thresholds, and periods to be more lenient.

var (
	// Default: A neutral baseline, now more willing to trade
	DefaultMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 0.5, // Lower RR threshold to accept more trades
			StrategyType:        strategies.RSIMACDStrategyType,
			RSI: &algos.RSIStrategy{
				Overbought: 68, // Slightly lower overbought to generate SELL signals sooner
				Oversold:   32, // Slightly higher oversold to generate BUY signals sooner
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   8,  // Faster MACD for quicker signals
				SlowPeriod:   17, // Shorter slow period
				SignalPeriod: 5,  // Quicker signal line
			},
			BollingerBands: &algos.BollingerBands{
				Period: 15,
				Width:  2.5, // Narrower than 3.5 to see more band touches
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 75, // Lower overbought, more frequent sells
				Oversold:   25, // Higher oversold, more frequent buys
				Period:     14,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CandleInterval:            "6h", // More frequent checks than 12h
			DesiredProfit:             1.0,  // Lower profit target to secure quick wins
			HighestPriceFallOffMargin: 0.5,  // Smaller panic sell margin
			FeeRate:                   0.001,
			MarketState:               models.Default,
		},
	}

	// Chaotic: Very short intervals and very permissive conditions to trade mean-reversions often
	ChaoticMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 0.7, // Slightly lenient
			RSI: &algos.RSIStrategy{
				Overbought: 72,
				Oversold:   28,
				Period:     10, // Shorter RSI period for quicker signals
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   5,
				SlowPeriod:   12,
				SignalPeriod: 4, // Very fast MACD to catch sudden moves
			},
			BollingerBands: &algos.BollingerBands{
				Period: 7,
				Width:  2.8, // Slightly narrower than default chaotic
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 85,
				Oversold:   15,
				Period:     7,
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 7,
				BasePeriod:       22,
				SpanBPeriod:      44,
			},
			CandleInterval:            "1h", // Faster interval for quick trades
			DesiredProfit:             2.0,  // Slight profit target
			HighestPriceFallOffMargin: 1.0,
			FeeRate:                   0.001,
			MarketState:               models.Chaotic,
		},
	}

	// Trending: Slightly less strict to allow more trades in confirmed trends
	TrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 1.0, // Lower than 1.1 for more trades
			RSI: &algos.RSIStrategy{
				Overbought: 68,
				Oversold:   32,
				Period:     14,
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   10,
				SlowPeriod:   20,
				SignalPeriod: 7, // Slightly faster MACD than standard
			},
			BollingerBands: &algos.BollingerBands{
				Period: 20,
				Width:  2.3, // Slightly narrower to catch pullbacks in trend
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
			CandleInterval:            "12h", // Keep a decent interval
			DesiredProfit:             5.0,   // Lower than 10 to secure profits sooner
			HighestPriceFallOffMargin: 3.0,   // Less margin than 5.0
			FeeRate:                   0.001,
			MarketState:               models.Trending,
		},
	}

	// RangeBound: More frequent trades at band extremes
	RangeBoundMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 0.8, // More lenient
			MACD: &algos.MACDStrategy{
				FastPeriod:   9,
				SlowPeriod:   21,
				SignalPeriod: 7,
			},
			Stochastic: &algos.StochasticOscillator{
				Overbought: 60, // Even lower to generate trades sooner
				Oversold:   40, // Higher oversold for more buys
				Period:     14,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 15,
				Width:  2.0, // Narrow bands for frequent touches
			},
			RSI: &algos.RSIStrategy{
				Overbought: 65,
				Oversold:   35,
				Period:     10, // Shorter RSI period for quick signals
			},
			Ichimoku: &algos.IchimokuStrategy{
				ConversionPeriod: 9,
				BasePeriod:       26,
				SpanBPeriod:      52,
			},
			CandleInterval:            "8h", // Slightly shorter than 12h for more signals
			DesiredProfit:             2.0,  // Lower desired profit to realize gains in a range
			HighestPriceFallOffMargin: 2.0,  // Slightly lower than 3.0
			FeeRate:                   0.001,
			MarketState:               models.RangeBound,
		},
	}

	// StronglyTrending: Quickly capitalize on strong trends with somewhat relaxed conditions
	StronglyTrendingMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 1.2, // Slightly lower than 1.5 to get more trades
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
				Width:  1.8, // Even narrower to catch small pullbacks
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
			CandleInterval:            "8h", // Slightly more frequent updates than 1d
			DesiredProfit:             8.0,  // Lower than 15 to secure profits
			HighestPriceFallOffMargin: 4.0,  // Less margin to prevent big drawdowns
			FeeRate:                   0.001,
			MarketState:               models.StronglyTrending,
		},
	}

	// Transitional: Very low threshold to encourage taking contrarian trades
	TransitionalMarketState = types.MarketStateStrategy{
		Enabled: true,
		Strategy: &strategies.CompoundStrategy{
			RiskRewardThreshold: 0.6, // Even more lenient
			RSI: &algos.RSIStrategy{
				Overbought: 80,
				Oversold:   20,
				Period:     10, // Shorter period for quick signals
			},
			MACD: &algos.MACDStrategy{
				FastPeriod:   8,
				SlowPeriod:   17,
				SignalPeriod: 5,
			},
			BollingerBands: &algos.BollingerBands{
				Period: 10,
				Width:  3.0, // Wide bands in transitional markets to find extremes
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
			CandleInterval:            "4h", // Frequent checks for transitions
			DesiredProfit:             1.0,  // Small quick profits
			HighestPriceFallOffMargin: 0.8,
			FeeRate:                   0.001,
			MarketState:               models.Transitional,
		},
	}
)
