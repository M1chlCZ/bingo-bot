package algos

import (
	"binance_bot/models"
	"fmt"
	"math"
)

type BollingerBands struct {
	Period int
	Width  float64
}

func (bb *BollingerBands) Calculate(candles []models.CandleStick) (lowerBand, middleBand, upperBand float64, err error) {
	if len(candles) < bb.Period {
		return 0, 0, 0, fmt.Errorf("not enough data to calculate Bollinger Bands")
	}

	// Calculate SMA (middle band)
	total := 0.0
	for i := len(candles) - bb.Period; i < len(candles); i++ {
		total += candles[i].Close
	}
	middleBand = total / float64(bb.Period)

	// Calculate standard deviation
	sumSquares := 0.0
	for i := len(candles) - bb.Period; i < len(candles); i++ {
		sumSquares += math.Pow(candles[i].Close-middleBand, 2)
	}
	stdDev := math.Sqrt(sumSquares / float64(bb.Period))

	// Calculate upper and lower bands
	lowerBand = middleBand - bb.Width*stdDev
	upperBand = middleBand + bb.Width*stdDev
	return lowerBand, middleBand, upperBand, nil
}
