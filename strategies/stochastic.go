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
	// Calculate %K and %D
	kValues, d, err := calculateStochasticOscillator(candles, s.Period)
	if err != nil {
		return "", 0, err
	}

	// Most recent %K value
	k := kValues[len(kValues)-1]

	// Logging the values
	str := fmt.Sprintf("K: %.2f D: %.2f", k, d)

	// Generate buy/sell signals
	if k > float64(s.Overbought) && d > float64(s.Overbought) {
		return str, -1, nil // Sell signal
	} else if k < float64(s.Oversold) && d < float64(s.Oversold) {
		return str, 1, nil // Buy signal
	}

	return str, 0, nil // Hold
}

func calculateStochasticOscillator(candles []models.CandleStick, period int) ([]float64, float64, error) {
	if len(candles) < period {
		return nil, 0, fmt.Errorf("not enough data to calculate stochastic oscillator: need %d candles, got %d", period, len(candles))
	}

	var kValues []float64

	// Calculate %K for each period
	for i := period; i <= len(candles); i++ {
		periodCandles := candles[i-period : i]

		// Find the highest high and lowest low in the period
		highestHigh, lowestLow := periodCandles[0].High, periodCandles[0].Low
		for _, candle := range periodCandles {
			if candle.High > highestHigh {
				highestHigh = candle.High
			}
			if candle.Low < lowestLow {
				lowestLow = candle.Low
			}
		}

		// Calculate %K for the current period
		lastClose := periodCandles[len(periodCandles)-1].Close
		k := (lastClose - lowestLow) / (highestHigh - lowestLow) * 100
		kValues = append(kValues, k)
	}

	// Calculate %D (3-period SMA of %K)
	if len(kValues) < 3 {
		return kValues, kValues[len(kValues)-1], nil // Return only %K if not enough for %D
	}

	var sumK float64
	for i := len(kValues) - 3; i < len(kValues); i++ {
		sumK += kValues[i]
	}
	d := sumK / 3

	return kValues, d, nil
}
