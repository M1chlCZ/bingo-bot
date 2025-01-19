package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"math"
)

type BollingerBands struct {
	Period int     `json:"period"`
	Width  float64 `json:"width"`
}

func (bb *BollingerBands) Calculate(candles []models.CandleStick) (lowerBand, middleBand, upperBand float64, err error) {
	if len(candles) < bb.Period {
		return 0, 0, 0, fmt.Errorf("not enough data to calculate Bollinger Bands")
	}

	// Calculate the typical prices (TP) for the last N candles
	var tps []float64
	startIndex := len(candles) - bb.Period
	for i := startIndex; i < len(candles); i++ {
		c := candles[i]
		tp := (c.High + c.Low + c.Close) / 3.0
		tps = append(tps, tp)
	}

	// Calculate the SMA of the TP
	sum := 0.0
	for _, tp := range tps {
		sum += tp
	}
	middleBand = sum / float64(bb.Period)

	// Calculate the standard deviation of the TP
	// Using sample standard deviation: sqrt(sum((x - mean)^2)/(N-1))
	var varianceSum float64
	for _, tp := range tps {
		deviation := tp - middleBand
		varianceSum += deviation * deviation
	}
	// Note: if you prefer population std dev, use N instead of (N-1)
	stdDev := math.Sqrt(varianceSum / float64(bb.Period-1))

	// Calculate upper and lower bands
	upperBand = middleBand + bb.Width*stdDev
	lowerBand = middleBand - bb.Width*stdDev

	return lowerBand, middleBand, upperBand, nil
}
