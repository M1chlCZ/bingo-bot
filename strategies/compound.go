package strategies

import (
	db2 "binance_bot/db"
	"binance_bot/models"
	"fmt"
	"log"
)

// RSIMACDStrategy represents the type of strategy, all fields are required to be filled
type RSIMACDStrategy struct {
	StrategyType StrategyType
	RSI          *RSIStrategy
	MACD         *MACDStrategy
	FeeRate      float64
}

func (cs *RSIMACDStrategy) GetStrategyType() StrategyType {
	return RSIMACDStrategyType
}

func (cs *RSIMACDStrategy) Calculate(candles []models.CandleStick, pair string, trend bool) (int, error) {
	rsiVal, rsiSignal, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		return 0, err
	}
	histogram, signalLine, macdVal, macdSignal, err := cs.MACD.Calculate(candles)

	if err != nil {
		return 0, err
	}

	// Fetch current price and buy price
	currentPrice := candles[len(candles)-1].Close
	trade, _ := db2.SQLiteDB.GetActiveTrade(pair) // Fetch active trade from DB

	if trade != nil {
		fmt.Println("Monitoring trade:", trade.ID, "Pair:", trade.Symbol, "Price:", trade.BuyPrice, "Quantity", trade.Quantity)
		breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
		profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

		// Check if current price is below breakeven
		if currentPrice < breakevenPrice {
			log.Printf("Skipping sell: Current price (%.2f) is below breakeven (%.2f).", currentPrice, breakevenPrice)
			return 0, nil // Hold
		}

		// Check if profit margin is sufficient
		desiredProfit := 1.5 // 1.5% profit margin
		if profitMargin > desiredProfit {
			log.Printf("Selling %s: Current profit margin = %.2f%%.", pair, profitMargin)
			return -1, nil // Sell signal
		} else {
			log.Printf("Skipping sell: Current profit margin = %.2f%%.", profitMargin)
			return 0, nil // Hold
		}
	}

	if rsiSignal > 0 && macdSignal > 0 {
		log.Println(pair, "Strong Buy |", rsiVal, macdVal)
		return 1, nil // Strong BUY
	} else if rsiSignal < 0 && macdSignal < 0 {
		log.Println(pair, "Strong Sell |", rsiVal, macdVal)
		return -1, nil // Strong Sell
	} else if rsiSignal > 0 && macdSignal < 0 {
		return 0, nil // Buy
	} else if rsiSignal < 0 && macdSignal > 0 {
		return 0, nil // Sell
	}

	var macdColor string
	var rsiColor string
	var trendColor string
	rsiThreshold := 70.0
	rsiDownThreshold := 30.0

	if macdVal > signalLine && histogram > 0 {
		macdColor = "\033[32m"
	} else {
		macdColor = "\033[31m"
	}

	if rsiVal > (rsiThreshold-5) || rsiVal < (rsiDownThreshold+5) {
		rsiColor = "\033[38;5;214m"
	} else if rsiVal < rsiThreshold && rsiVal > rsiDownThreshold {
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
	fmt.Printf("%s | HOLD | %s%.6f\033[0m %s%.6f\033[0m\n | %s%.v\u001B[0m", pair, rsiColor, rsiVal, macdColor, macdVal, trendColor, trendText)

	return 0, nil // Hold
}
