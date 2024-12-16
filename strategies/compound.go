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
	Ichimoku                  *algos.IchimokuStrategy     `validate:"required"`
	MarketState               models.MarketState          `validate:"required"`
	RiskRewardThreshold       float64                     `validate:"gte=0"`
	FeeRate                   float64                     `validate:"gte=0"`
	DesiredProfit             float64                     `validate:"gte=0"`
	HighestPriceFallOffMargin float64                     `validate:"gte=0"`
	CandleInterval            string                      `validate:"required"`
}

func (cs *CompoundStrategy) GetStrategyType() StrategyType {
	return RSIMACDStrategyType
}

func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, marketState models.MarketState) (int, error) {
	if len(candles) == 0 {
		return 0, nil
	}

	currentPrice := candles[len(candles)-1].Close

	// Fetch existing trade
	trade, _ := db2.SQLiteDB.GetActiveTrade(pair)
	isActive, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair)
	if err != nil {
		logger.Errorf("Error checking active trade: %v", err)
		isActive = false
	}

	// Calculate indicators
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating RSI: %v", err)
		return 0, err
	}

	_, _, _, macdIndicatorSignal, err := cs.MACD.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating MACD: %v", err)
		return 0, err
	}

	stochasticK, _, err := cs.Stochastic.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Stochastic: %v", err)
		return 0, err
	}

	lowerBand, _, upperBand, err := cs.BollingerBands.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Bollinger Bands: %v", err)
		return 0, err
	}

	// Calculate Ichimoku
	ichimokuRes, err := cs.Ichimoku.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Ichimoku: %v", err)
		return 0, err
	}

	logger.Infof("Ichimoku Result: %+v", ichimokuRes)

	// Indicator scoring
	buyScore := 0
	sellScore := 0

	// RSI
	if rsiVal < float64(cs.RSI.Oversold) {
		buyScore++
	} else if rsiVal > float64(cs.RSI.Overbought) {
		sellScore++
	}

	// MACD
	if macdIndicatorSignal == 1 {
		buyScore++
	} else if macdIndicatorSignal == -1 {
		sellScore++
	}

	// Stochastic
	if int(stochasticK) < cs.Stochastic.Oversold {
		buyScore++
	} else if int(stochasticK) > cs.Stochastic.Overbought {
		sellScore++
	}

	// Bollinger Band proximity
	distToLower := (currentPrice - lowerBand) / lowerBand
	distToUpper := (upperBand - currentPrice) / upperBand
	if distToLower < 0.01 {
		buyScore++
	}
	if distToUpper < 0.01 {
		sellScore++
	}

	// Ichimoku signal
	if ichimokuRes.Bullish {
		buyScore += 2 // Ichimoku bullish signal is strong
	}
	if ichimokuRes.Bearish {
		sellScore += 2 // Ichimoku bearish signal is strong
	}

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

	// Risk/Reward consideration using Bollinger as approximate S/L and T/P
	riskRewardThreshold := cs.RiskRewardThreshold
	if buyScore > sellScore && buyScore >= 3 {
		target := upperBand
		stop := lowerBand
		risk := (currentPrice - stop) / currentPrice
		reward := (target - currentPrice) / currentPrice
		logger.Debugf("BUY Risk: %.2f%%, Reward: %.2f%% Ration: %.4f", risk*100, reward*100, reward/risk)
		if reward > 0 && risk > 0 {
			rr := reward / risk
			if rr > riskRewardThreshold {
				logger.Infof("[BUY] %s: BuyScore=%d, RR=%.2f, Ichimoku Confirmed Bullish", pair, buyScore, rr)
				return 1, nil
			}
		}
	}

	if sellScore > buyScore && sellScore >= 3 && isActive {
		target := lowerBand
		stop := upperBand
		risk := (stop - currentPrice) / currentPrice
		reward := (currentPrice - target) / currentPrice
		logger.Debugf("SELL Risk: %.2f%%, Reward: %.2f%% Ration: %.4f", risk*100, reward*100, reward/risk)
		if reward > 0 && risk > 0 {
			rr := reward / risk
			if rr > riskRewardThreshold {
				logger.Infof("[SELL] %s: SellScore=%d, RR=%.2f, Ichimoku Confirmed Bearish", pair, sellScore, rr)
				return -1, nil
			}
		}
	}

	logger.Debugf("HOLD %s: No strong buy/sell signal. BuyScore=%d, SellScore=%d", pair, buyScore, sellScore)
	return 0, nil
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
