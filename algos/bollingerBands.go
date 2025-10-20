package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type BollingerBands struct {
	Period int     `json:"period"` // lookback, must be >= 2
	Width  float64 `json:"width"`  // band width multiplier (typically 2.0)
}

func (bb *BollingerBands) Calculate(candles []models.CandleStick) (lowerBand, middleBand, upperBand float64, err error) {
	if bb.Period < 2 {
		return 0, 0, 0, fmt.Errorf("bollinger: period must be >= 2 (got %d)", bb.Period)
	}
	if bb.Width <= 0 {
		return 0, 0, 0, fmt.Errorf("bollinger: width must be > 0 (got %.4f)", bb.Width)
	}
	if len(candles) < bb.Period {
		return 0, 0, 0, fmt.Errorf("bollinger: not enough data, need %d candles, got %d", bb.Period, len(candles))
	}

	start := len(candles) - bb.Period

	n := 0.0
	mean := 0.0
	M2 := 0.0

	for i := start; i < len(candles); i++ {
		c := candles[i]
		tp := (c.High + c.Low + c.Close) / 3.0

		n++
		delta := tp - mean
		mean += delta / n
		M2 += delta * (tp - mean)
	}

	if n < 2 {

		return 0, 0, 0, fmt.Errorf("bollinger: insufficient points after windowing")
	}

	variance := M2 / n
	if variance < 0 {

		variance = 0
	}
	stdDev := math.Sqrt(variance)

	middleBand = mean
	upperBand = middleBand + bb.Width*stdDev
	lowerBand = middleBand - bb.Width*stdDev

	if math.IsNaN(middleBand) || math.IsNaN(upperBand) || math.IsNaN(lowerBand) {
		return 0, 0, 0, fmt.Errorf("bollinger: NaN encountered (mean=%.6f std=%.6f)", middleBand, stdDev)
	}

	return lowerBand, middleBand, upperBand, nil
}
