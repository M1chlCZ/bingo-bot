package strategies

import (
	"binance_bot/models"
	"fmt"
	"log"
)

type CompoundStrategy struct {
	RSI  *RSIStrategy
	MACD *MACDStrategy
}

func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, trend bool) (int, error) {
	rsiVal, rsiSignal, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		return 0, err
	}
	histogram, signalLine, macdVal, macdSignal, err := cs.MACD.Calculate(candles)

	if err != nil {
		return 0, err
	}

	if rsiSignal > 0 && macdSignal > 0 {
		log.Println(pair, "Strong Buy |", rsiVal, macdVal)
		return 1, nil // Strong BUY
	} else if rsiSignal < 0 && macdSignal < 0 {
		log.Println(pair, "Strong Sell |", rsiVal, macdVal)
		return -1, nil // Strong SELL
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
