package strategies

import (
	"binance_bot/models"
	"fmt"
)

type RSIStrategy struct {
	Overbought int // RSI threshold for overbought (sell signal)
	Oversold   int // RSI threshold for oversold (buy signal)
	Period     int // Lookback period for RSI
}

// Calculate implements the Strategy interface for RSIStrategy
func (r *RSIStrategy) Calculate(candles []models.CandleStick, pair string) (float64, int, error) {
	rsi, err := calculateRSI(candles, r.Period)
	if err != nil {
		return 0, 0, err
	}

	// Use the latest RSI value for decision-making
	latestRSI := rsi[len(rsi)-1]

	if latestRSI > float64(r.Overbought) {
		return latestRSI, -1, nil // Sell signal
	} else if latestRSI < float64(r.Oversold) {
		return latestRSI, 1, nil // Buy signal
	}
	//log.Println(pair, "HOLD SIGNAL", latestRSI, r.Overbought, r.Oversold)
	return latestRSI, 0, nil // Hold
}

func calculateRSI(candles []models.CandleStick, period int) ([]float64, error) {
	if len(candles) < period {
		return nil, fmt.Errorf("not enough data to calculate RSI: need %d candles, got %d", period, len(candles))
	}

	gains := make([]float64, len(candles)-1)
	losses := make([]float64, len(candles)-1)

	for i := 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			gains[i-1] = change
		} else {
			losses[i-1] = -change
		}
	}

	// Calculate the initial averages
	avgGain := 0.0
	avgLoss := 0.0
	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// Calculate the RSI values
	rsi := make([]float64, len(candles)-period)
	for i := period; i < len(candles); i++ {
		avgGain = (avgGain*(float64(period)-1) + gains[i-1]) / float64(period)
		avgLoss = (avgLoss*(float64(period)-1) + losses[i-1]) / float64(period)

		rs := 0.0
		if avgLoss != 0 {
			rs = avgGain / avgLoss
		}

		rsi[i-period] = 100 - (100 / (1 + rs))
	}

	return rsi, nil
}
