package strategies

import (
	"binance_bot/models"
	"fmt"
)

type StochasticOscillator struct {
	Overbought int // Overbought threshold (e.g., 80)
	Oversold   int // Oversold threshold (e.g., 20)
	Period     int // Lookback period
}

// Calculate generates a signal based on the stochastic oscillator
func (s *StochasticOscillator) Calculate(candles []models.CandleStick) (string, int, error) {
	k, d, err := calculateStochasticOscillator(candles, s.Period)
	if err != nil {
		return "", 0, err
	}

	str := fmt.Sprintf("K: %.2f D: %.2f", k, d)

	if k > float64(s.Overbought) && d > float64(s.Overbought) {
		return str, -1, nil // Sell signal
	} else if k < float64(s.Oversold) && d < float64(s.Oversold) {
		return str, 1, nil // Buy signal
	}

	return str, 0, nil // Hold
}

func calculateStochasticOscillator(candles []models.CandleStick, period int) (float64, float64, error) {
	if len(candles) < period {
		return 0, 0, fmt.Errorf("not enough data to calculate stochastic oscillator: need %d candles, got %d", period, len(candles))
	}

	var highestHigh, lowestLow float64
	highestHigh = candles[len(candles)-period].High
	lowestLow = candles[len(candles)-period].Low

	for i := len(candles) - period; i < len(candles); i++ {
		if candles[i].High > highestHigh {
			highestHigh = candles[i].High
		}
		if candles[i].Low < lowestLow {
			lowestLow = candles[i].Low
		}
	}

	// %K calculation
	lastClose := candles[len(candles)-1].Close
	k := (lastClose - lowestLow) / (highestHigh - lowestLow) * 100

	// %D calculation (3-period SMA of %K)
	if len(candles) < period+3 {
		return k, k, nil // Not enough data for %D, return %K as %D
	}

	var sumK float64
	for i := len(candles) - 3; i < len(candles); i++ {
		cls := candles[i].Close
		low := candles[i-period].Low
		high := candles[i-period].High
		percentK := (cls - low) / (high - low) * 100
		sumK += percentK
	}
	d := sumK / 3

	return k, d, nil
}
