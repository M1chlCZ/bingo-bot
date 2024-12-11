package algos

import (
	"binance_bot/models"
	"fmt"
)

type StochasticOscillator struct {
	Overbought int
	Oversold   int
	Period     int
}

func (so *StochasticOscillator) Calculate(candles []models.CandleStick) (k, d float64, err error) {
	if len(candles) < so.Period {
		return 0, 0, fmt.Errorf("not enough data to calculate Stochastic Oscillator")
	}

	var highestHigh, lowestLow float64
	highestHigh = candles[len(candles)-so.Period].High
	lowestLow = candles[len(candles)-so.Period].Low

	for i := len(candles) - so.Period; i < len(candles); i++ {
		if candles[i].High > highestHigh {
			highestHigh = candles[i].High
		}
		if candles[i].Low < lowestLow {
			lowestLow = candles[i].Low
		}
	}

	lastClose := candles[len(candles)-1].Close
	k = (lastClose - lowestLow) / (highestHigh - lowestLow) * 100

	// %D (3-period SMA of %K)
	if len(candles) < so.Period+3 {
		return k, k, nil // Not enough data for %D
	}
	sumK := 0.0
	for i := len(candles) - 3; i < len(candles); i++ {
		sumK += k
	}
	d = sumK / 3
	return k, d, nil
}
