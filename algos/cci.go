package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"math"
)

type CCIStrategy struct {
	Period     int     // Lookback for CCI
	Overbought float64 // e.g., +100
	Oversold   float64 // e.g., -100
}

// Calculate CCI and return (latestCCI, signal, error).
// signal = -1 if cci > Overbought, +1 if cci < Oversold, else 0
func (c *CCIStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < c.Period {
		return 0, 0, fmt.Errorf("not enough data for CCI: need %d, got %d", c.Period, len(candles))
	}

	// Compute typical prices
	typicalPrices := make([]float64, len(candles))
	for i := 0; i < len(candles); i++ {
		cndl := candles[i]
		tp := (cndl.High + cndl.Low + cndl.Close) / 3.0
		typicalPrices[i] = tp
	}

	// Compute the SMA of typical prices for the last c.Period
	var sum float64
	startIdx := len(candles) - c.Period
	for i := startIdx; i < len(candles); i++ {
		sum += typicalPrices[i]
	}
	smaTP := sum / float64(c.Period)

	// Mean deviation
	var meanDev float64
	for i := startIdx; i < len(candles); i++ {
		dev := math.Abs(typicalPrices[i] - smaTP)
		meanDev += dev
	}
	meanDev /= float64(c.Period)

	if meanDev == 0 {
		// no volatility
		return 0, 0, nil
	}

	cci := (typicalPrices[len(typicalPrices)-1] - smaTP) / (0.015 * meanDev)

	if cci > c.Overbought {
		return cci, -1, nil // Sell
	} else if cci < c.Oversold {
		return cci, 1, nil // Buy
	}
	return cci, 0, nil // hold
}
