package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type CCIStrategy struct {
	Period     int     `json:"period"`
	Overbought float64 `json:"overbought"`
	Oversold   float64 `json:"oversold"`
}

func (c *CCIStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if c.Period < 2 {
		return 0, 0, fmt.Errorf("CCI: period must be >= 2 (got %d)", c.Period)
	}
	if len(candles) < c.Period {
		return 0, 0, fmt.Errorf("CCI: not enough data, need %d candles, got %d", c.Period, len(candles))
	}

	start := len(candles) - c.Period

	sumTP := 0.0
	for i := start; i < len(candles); i++ {
		cd := candles[i]
		sumTP += (cd.High + cd.Low + cd.Close) / 3.0
	}
	smaTP := sumTP / float64(c.Period)

	meanDev := 0.0
	for i := start; i < len(candles); i++ {
		cd := candles[i]
		tp := (cd.High + cd.Low + cd.Close) / 3.0
		meanDev += math.Abs(tp - smaTP)
	}
	meanDev /= float64(c.Period)

	if meanDev == 0 || math.IsNaN(meanDev) || math.IsInf(meanDev, 0) {
		return 0, 0, nil
	}

	last := candles[len(candles)-1]
	curTP := (last.High + last.Low + last.Close) / 3.0

	cci := (curTP - smaTP) / (0.015 * meanDev)
	if math.IsNaN(cci) || math.IsInf(cci, 0) {
		return 0, 0, nil
	}

	if cci > c.Overbought {
		return cci, -1, nil // sell / overbought
	}
	if cci < c.Oversold {
		return cci, 1, nil // buy / oversold
	}
	return cci, 0, nil
}
