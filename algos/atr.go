package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type ATRStrategy struct {
	Period     int     `json:"period"`     // The number of periods to use for ATR calculation
	Multiplier float64 `json:"multiplier"` // Multiplier for ATR to determine significant price movements
}

func ATR(candles []models.CandleStick, period int) (float64, error) {
	if period <= 0 {
		return 0, fmt.Errorf("ATR period must be greater than zero")
	}

	if len(candles) < period+1 {
		return 0, fmt.Errorf("not enough data to calculate ATR: need at least %d candles, got %d", period+1, len(candles))
	}

	tr := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		h := candles[i].High
		l := candles[i].Low
		pc := candles[i-1].Close
		trueRange := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		tr = append(tr, trueRange)
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	atr := sum / float64(period)

	for i := period; i < len(tr); i++ {
		atr = ((atr * float64(period-1)) + tr[i]) / float64(period)
	}

	return atr, nil
}

func calculateATR(candles []models.CandleStick, period int) ([]float64, error) {
	if period <= 0 {
		return nil, fmt.Errorf("ATR period must be greater than zero")
	}
	if len(candles) < period+1 {
		return nil, fmt.Errorf("not enough data to calculate ATR: need at least %d candles, got %d", period+1, len(candles))
	}

	tr := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		h := candles[i].High
		l := candles[i].Low
		pc := candles[i-1].Close
		trueRange := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		tr = append(tr, trueRange)
	}

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	seed := sum / float64(period)

	atr := make([]float64, len(candles)-period)
	atr[0] = seed

	prev := seed
	outIdx := 1
	for i := period; i < len(tr); i++ {
		next := ((prev * float64(period-1)) + tr[i]) / float64(period)
		if outIdx >= len(atr) {

			break
		}
		atr[outIdx] = next
		prev = next
		outIdx++
	}

	return atr, nil
}
