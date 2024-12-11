package strategies

import (
	"binance_bot/algos"
	db2 "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/models"
	"github.com/go-playground/validator/v10"
)

//var highestPrices sync.Map

type CompoundStrategy struct {
	StrategyType              StrategyType                `validate:"required"`
	RSI                       *algos.RSIStrategy          `validate:"required"`
	MACD                      *algos.MACDStrategy         `validate:"required"`
	Stochastic                *algos.StochasticOscillator `validate:"required"`
	BollingerBands            *algos.BollingerBands       `validate:"required"`
	MarketState               models.MarketState          `validate:"required"`
	FeeRate                   float64                     `validate:"gte=0"`
	DesiredProfit             float64                     `validate:"gte=0"`
	HighestPriceFallOffMargin float64                     `validate:"gte=0"`
	CandleInterval            string                      `validate:"required"`
}

func (cs *CompoundStrategy) GetStrategyType() StrategyType {
	return RSIMACDStrategyType
}

func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, marketState models.MarketState) (int, error) {
	var rsiSignal int
	var rsiVal float64
	var err error
	if cs.RSI != nil {
		rsiVal, rsiSignal, err = cs.RSI.Calculate(candles, pair)
		if err != nil {
			logger.Debugf("Error calculating RSI: %v", err)
			return 0, err
		}
	} else {
		rsiSignal = 0
	}

	macdHistogram, macdSignalLine, macdVal, _, err := cs.MACD.Calculate(candles)
	if err != nil {
		logger.Debugf("Error calculating MACD: %v", err)
		return 0, err
	}

	logger.Debugf("MACD Histogram: %.8f, Signal Line: %.8f, MACD: %.8f", macdHistogram, macdSignalLine, macdVal)

	stochasticK, _, err := cs.Stochastic.Calculate(candles)
	if err != nil {
		logger.Debugf("Error calculating Stochastic Oscillator: %v", err)
		return 0, err
	}

	logger.Debugf("Stochastic K: %.2f", stochasticK)

	lowerBand, middleBand, upperBand, err := cs.BollingerBands.Calculate(candles)
	if err != nil {
		logger.Debugf("Error calculating Bollinger Bands: %v", err)
		return 0, err
	}

	logger.Debugf("Bollinger Bands: Lower=%.2f, Middle=%.2f, Upper=%.2f", lowerBand, middleBand, upperBand)

	currentPrice := candles[len(candles)-1].Close
	trade, _ := db2.SQLiteDB.GetActiveTrade(pair) // Fetch active trade from DB
	isActive, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair)

	//strongBuy := rsiSignal > 0 && macdSignal > 0
	//strongSell := rsiSignal < 0 && macdSignal < 0

	// Active trade monitoring and sell signals
	if trade != nil {
		logger.Infof("Monitoring trade ID: %d | Pair: %s | Buy Price: %.2f | Quantity: %.2f", trade.ID, trade.Symbol, trade.BuyPrice, trade.Quantity)

		// Calculate breakeven price (including fees)
		breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)

		// Calculate profit margin from the buy price
		profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

		// Update or fetch the ATH price
		athPrice, err := db2.SQLiteDB.GetAth(trade.Symbol)
		if err != nil || currentPrice > athPrice {
			err = db2.SQLiteDB.SetUpdateAth(trade.Symbol, currentPrice)
			if err != nil {
				logger.Errorf("Error updating ATH price for %s: %v", trade.Symbol, err)
			} else {
				athPrice = currentPrice
				logger.Infof("New ATH price for %s: %.2f", trade.Symbol, currentPrice)
			}
		}

		// Calculate profit margin relative to ATH
		profitMarginATH := (currentPrice - athPrice) / athPrice * 100

		// Log the current state for debugging
		logger.Debugf(
			"Current Price: %.2f | Breakeven Price: %.2f | Profit Margin: %.2f%% | ATH: %.2f | Profit Margin from ATH: %.2f%% | Highest Price Fall Off Margin: %.2f%%",
			currentPrice, breakevenPrice, profitMargin, athPrice, profitMarginATH, cs.HighestPriceFallOffMargin,
		)

		// Trigger a sell if the current price falls below a percentage of the ATH
		//if profitMarginATH < -cs.HighestPriceFallOffMargin {
		//	logger.Infof(
		//		"Selling %s: Current price (%.2f) dropped %.2f%% below ATH (%.2f).",
		//		pair, currentPrice, -profitMarginATH, athPrice,
		//	)
		//	highestPrices.Delete(pair)
		//	return -1, nil // Sell signal
		//}

		//panic sell if the current price falls below a percentage of the ATH
		if profitMargin < -cs.HighestPriceFallOffMargin {
			logger.Infof(
				"Selling %s: Current price (%.2f) dropped %.2f%% below ATH (%.2f).",
				pair, currentPrice, -profitMarginATH, athPrice,
			)
			return -2, nil // Sell signal
		}

		// Skip sell if below breakeven
		if currentPrice < breakevenPrice {
			logger.Infof(
				"Holding %s: Current price (%.2f) is below breakeven (%.2f). Profit Margin: %.2f%%",
				pair, currentPrice, breakevenPrice, profitMargin,
			)
			return 0, nil // Hold
		}

		// Sell if profit margin exceeds desired threshold
		if profitMargin > cs.DesiredProfit {
			logger.Infof(
				"Selling %s: Current profit margin = %.2f%% exceeds desired profit margin = %.2f%%.",
				pair, profitMargin, cs.DesiredProfit,
			)
			return -1, nil // Sell signal
		}

		// If none of the conditions are met, continue holding
		logger.Warnf(
			"Holding %s: Current profit margin = %.2f%%, Desired profit margin = %.2f%%.",
			pair, profitMargin, cs.DesiredProfit,
		)
		return 0, nil // Hold
	}

	// Decision-making based on market state and indicators
	switch marketState {
	// Strong BUY Signal
	case models.Trending:
		if macdVal > macdSignalLine && macdHistogram > 0 &&
			int(stochasticK) < cs.Stochastic.Overbought &&
			rsiVal < 70 && currentPrice < upperBand {
			logger.Infof("[Trending] Strong BUY signal for %s | MACD=%.2f, SignalLine=%.2f, StochasticK=%.2f, RSI=%.2f",
				pair, macdVal, macdSignalLine, stochasticK, rsiVal)
			return 1, nil
		}

		// Strong SELL Signal
		if macdVal < macdSignalLine && macdHistogram < 0 &&
			int(stochasticK) > cs.Stochastic.Oversold &&
			rsiVal > 30 && currentPrice > lowerBand {
			if !isActive {
				return 0, nil
			}
			logger.Infof("[Trending] Strong SELL signal for %s | MACD=%.2f, SignalLine=%.2f, StochasticK=%.2f, RSI=%.2f",
				pair, macdVal, macdSignalLine, stochasticK, rsiVal)
			return -1, nil
		}

		// Default HOLD condition
		logger.Debugf("[Trending] HOLD signal for %s | MACD=%.2f, SignalLine=%.2f, StochasticK=%.2f, RSI=%.2f",
			pair, macdVal, macdSignalLine, stochasticK, rsiVal)
		return 0, nil
	case models.Chaotic:
		if int(stochasticK) > cs.Stochastic.Overbought && currentPrice > upperBand {
			logger.Infof("[Chaotic] SELL signal for %s | Stochastic K=%.2f, Overbought=%d, CurrentPrice=%.2f", pair, stochasticK, cs.Stochastic.Overbought, currentPrice)
			return -1, nil
		}
		if int(stochasticK) < cs.Stochastic.Oversold && currentPrice < lowerBand {
			if !isActive {
				return 0, nil
			}
			logger.Infof("[Chaotic] BUY signal for %s | Stochastic K=%.2f, Oversold=%d, CurrentPrice=%.2f", pair, stochasticK, cs.Stochastic.Oversold, currentPrice)
			return 1, nil
		}
	case models.RangeBound:
		// Check for BUY signal
		if currentPrice <= lowerBand && macdVal > macdSignalLine && int(stochasticK) < cs.Stochastic.Oversold && int(rsiVal) < cs.RSI.Oversold {
			logger.Infof("[RangeBound] BUY signal for %s | CurrentPrice=%.2f, LowerBand=%.2f, MACD=%.2f, StochasticK=%.2f, RSI=%.2f",
				pair, currentPrice, lowerBand, macdVal, stochasticK, rsiVal)
			return 1, nil
		}

		// Check for SELL signal
		if currentPrice >= upperBand && macdVal < macdSignalLine && int(stochasticK) > cs.Stochastic.Overbought && int(rsiVal) > cs.RSI.Overbought {
			if !isActive {
				logger.Infof("[RangeBound] HOLD signal for %s | CurrentPrice=%.2f, UpperBand=%.2f, MACD=%.2f, StochasticK=%.2f, RSI=%.2f",
					pair, currentPrice, upperBand, macdVal, stochasticK, rsiVal)
				return 0, nil
			}
			logger.Infof("[RangeBound] SELL signal for %s | CurrentPrice=%.2f, UpperBand=%.2f, MACD=%.2f, StochasticK=%.2f, RSI=%.2f",
				pair, currentPrice, upperBand, macdVal, stochasticK, rsiVal)
			return -1, nil
		}

		// Default HOLD signal if no conditions are met
		logger.Debugf("[RangeBound] HOLD signal for %s | No clear signals detected. CurrentPrice=%.2f, LowerBand=%.2f, UpperBand=%.2f, MACD=%.2f, StochasticK=%.2f, RSI=%.2f",
			pair, currentPrice, lowerBand, upperBand, macdVal, stochasticK, rsiVal)
		return 0, nil
	default:
		if macdVal > macdSignalLine && macdHistogram > 0 && int(stochasticK) < cs.Stochastic.Overbought && rsiSignal > 0 {
			logger.Infof("[Default] BUY signal for %s | MACD=%.2f, SignalLine=%.2f, StochasticK=%.2f, RSI=%d", pair, macdVal, macdSignalLine, stochasticK, rsiSignal)
			return 1, nil
		}
		if macdVal < macdSignalLine && macdHistogram < 0 && int(stochasticK) > cs.Stochastic.Oversold && rsiSignal < 0 {
			if !isActive {
				return 0, nil
			}
			logger.Infof("[Default] SELL signal for %s | MACD=%.2f, SignalLine=%.2f, StochasticK=%.2f, RSI=%d", pair, macdVal, macdSignalLine, stochasticK, rsiSignal)
			return -1, nil
		}
	}

	logger.Debugf("HOLD for %s | Market State: %s | MACD=%.2f, RSI=%d, StochasticK=%.2f", pair, marketState.String(), macdVal, rsiSignal, stochasticK)
	return 0, nil // Hold
}

func (cs *CompoundStrategy) GetRSI(candles []models.CandleStick, pair string) (float64, int, error) {
	return cs.RSI.Calculate(candles, pair)
}

func (cs *CompoundStrategy) GetMACD(candles []models.CandleStick) (float64, float64, float64, int, error) {
	return cs.MACD.Calculate(candles)
}

func (cs *CompoundStrategy) GetStochastic(candles []models.CandleStick) (float64, float64, error) {
	return cs.Stochastic.Calculate(candles)
}

func (cs *CompoundStrategy) GetBollingerBands(candles []models.CandleStick) (float64, float64, float64, error) {
	return cs.BollingerBands.Calculate(candles)
}

func (cs *CompoundStrategy) GetCandleInterval() string {
	return cs.CandleInterval
}

func (cs *CompoundStrategy) Validate() error {
	v := validator.New()
	return v.Struct(cs)
}
