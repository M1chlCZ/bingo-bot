package strategies

import (
	db2 "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/models"
	"sync"
)

var highestPrices sync.Map

// CompoundStrategy represents the type of strategy, all fields are required to be filled
// TODO: Deal with RSI or MACD strategy being nil
// TODO: Implement StochasticOscillator strategy
type CompoundStrategy struct {
	StrategyType StrategyType
	RSI          *RSIStrategy
	MACD         *MACDStrategy
	// Fee rate for selling
	FeeRate float64
	// Desired profit margin before selling
	DesiredProfit float64
	// Sell if price falls below highest price since sale was made by a certain margin
	HighestPriceFallOffMargin float64
}

func (cs *CompoundStrategy) GetStrategyType() StrategyType {
	return RSIMACDStrategyType
}

func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, trend bool) (int, error) {
	var macdColor string
	var rsiColor string
	var trendColor string
	rsiThreshold := cs.RSI.Overbought
	rsiDownThreshold := cs.RSI.Oversold

	rsiVal, rsiSignal, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		return 0, err
	}
	histogram, signalLine, macdVal, macdSignal, err := cs.MACD.Calculate(candles)

	if err != nil {
		return 0, err
	}

	if macdVal > signalLine && histogram > 0 {
		macdColor = "\033[32m"
	} else {
		macdColor = "\033[31m"
	}

	if rsiVal > (float64(rsiThreshold)-5) || rsiVal < (float64(rsiDownThreshold)+5) {
		rsiColor = "\033[38;5;214m"
	} else if rsiVal < float64(rsiThreshold) && rsiVal > float64(rsiDownThreshold) {
		rsiColor = "\033[31m"
	} else {
		rsiColor = "\033[32m"
	}

	if trend {
		trendColor = "\033[32m"
	} else {
		trendColor = "\033[31m"
	}

	var trendText string
	if trend {
		trendText = "Uptrend"
	} else {
		trendText = "Downtrend"
	}
	logger.Infof("%s | HOLD | %s%.6f\033[0m %s%.6f\033[0m\n | %s%.v\u001B[0m \n", pair, rsiColor, rsiVal, macdColor, macdVal, trendColor, trendText)

	// Fetch current price and buy price
	currentPrice := candles[len(candles)-1].Close
	trade, _ := db2.SQLiteDB.GetActiveTrade(pair) // Fetch active trade from DB

	if trade != nil && !(rsiSignal > 0 && macdSignal > 0) { // If trade is active and not a strong buy signal
		logger.Infof("Monitoring trade ID: %d | Pair: %s | Price: %.2f | Quantity %.2f", trade.ID, trade.Symbol, trade.BuyPrice, trade.Quantity)
		breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
		profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

		// Get or update the new high price since the trade was filled
		athPrice, ok := highestPrices.Load(pair)
		if !ok || currentPrice > athPrice.(float64) {
			highestPrices.Store(pair, currentPrice)
			athPrice = currentPrice
			logger.Infof("New HIGH price for %s: %.2f\n", pair, currentPrice)
		} else {
			athPrice = athPrice.(float64)
		}

		// Calculate profit margin relative to ATH
		profitMarginATH := (currentPrice - athPrice.(float64)) / athPrice.(float64) * 100

		// Sell if price falls below highest price by a certain margin
		if cs.HighestPriceFallOffMargin != 0 {
			if profitMarginATH < -cs.HighestPriceFallOffMargin {
				logger.Infof("Selling %s: Current price (%.2f) is 5%% below ATH (%.2f). \n", pair, currentPrice, athPrice)
				highestPrices.Delete(pair)
				return -1, nil // Sell signal
			}
		}

		//Check if current price is below breakeven
		if currentPrice < breakevenPrice {
			//logger.Infof("Skipping sell: Current price (%.2f) is below breakeven (%.2f).", currentPrice, breakevenPrice)
			return 0, nil // Hold
		}

		//Check if profit margin is sufficient
		desiredProfit := 5.0 // 25% profit margin
		if cs.DesiredProfit != 0 {
			desiredProfit = cs.DesiredProfit
		}
		if profitMargin > desiredProfit {
			logger.Infof("Selling %s: Current profit margin = %.2f%%. \n", pair, profitMargin)
			highestPrices.Delete(pair)
			return -1, nil // Sell signal
		} else {
			logger.Warnf("Skipping sell: Current profit margin = %.2f%%. | Desired profit margin = %.2f%% \n", profitMargin, cs.DesiredProfit)
			return 0, nil // Hold
		}
	}

	if rsiSignal > 0 && macdSignal > 0 {
		logger.Info(pair, "Strong Buy |", rsiVal, macdVal, "\n")
		return 1, nil // Strong BUY
	} else if rsiSignal < 0 && macdSignal < 0 {
		logger.Info(pair, "Strong Sell |", rsiVal, macdVal, "\n")
		return -1, nil // Strong Sell
	} else if rsiSignal > 0 && macdSignal < 0 {
		return 0, nil // Buy
	} else if rsiSignal < 0 && macdSignal > 0 {
		return 0, nil // Sell
	}

	return 0, nil // Hold
}
