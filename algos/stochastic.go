package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"math"
)

type StochasticOscillator struct {
	Overbought int
	Oversold   int
	Period     int
	DPeriod    int // The period for %D smoothing, commonly 3
}

// Calculate the Stochastic Oscillator (%K and %D).
// %K = (Current Close - Lowest Low over N) / (Highest High over N) * 100
// %D = SMA of %K over DPeriod bars (commonly 3)
func (so *StochasticOscillator) Calculate(candles []models.CandleStick) (k, d float64, err error) {
	if so.DPeriod == 0 {
		so.DPeriod = 3 // Default to 3 if not set
	}

	if len(candles) < so.Period+so.DPeriod-1 {
		return 0, 0, fmt.Errorf("not enough data to calculate Stochastic Oscillator")
	}

	// We'll calculate %K for each of the last (so.DPeriod) candles
	// to get a proper SMA for %D.
	var kValues []float64

	startIndex := len(candles) - so.Period - (so.DPeriod - 1)
	endIndex := len(candles) - 1

	// Calculate %K for each candle needed to form a DPeriod of %K values
	for i := startIndex; i <= endIndex; i++ {
		// Find Highest High and Lowest Low over the last so.Period candles ending at i
		lowIndex := i - so.Period + 1
		if lowIndex < 0 {
			lowIndex = 0
		}

		highestHigh := -math.MaxFloat64
		lowestLow := math.MaxFloat64

		for j := lowIndex; j <= i; j++ {
			if candles[j].High > highestHigh {
				highestHigh = candles[j].High
			}
			if candles[j].Low < lowestLow {
				lowestLow = candles[j].Low
			}
		}

		currentClose := candles[i].Close
		if highestHigh == lowestLow {
			// Avoid division by zero if all prices are the same
			kVal := 50.0 // If there's no range, %K is neutral
			kValues = append(kValues, kVal)
		} else {
			kVal := ((currentClose - lowestLow) / (highestHigh - lowestLow)) * 100.0
			kValues = append(kValues, kVal)
		}
	}

	// The last calculated %K is the most recent candle's %K
	k = kValues[len(kValues)-1]

	// Now calculate %D as the SMA of the last so.DPeriod %K values
	if len(kValues) < so.DPeriod {
		// Not enough %K values to form %D - return %K as %D as fallback
		return k, k, nil
	}

	var sumK float64
	for i := len(kValues) - so.DPeriod; i < len(kValues); i++ {
		sumK += kValues[i]
	}
	d = sumK / float64(so.DPeriod)

	return k, d, nil
}
