package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type KeltnerChannel struct {
	Period     int     `json:"period"`     // lookback for EMA & ATR, e.g. 20
	Multiplier float64 `json:"multiplier"` // ATR multiplier, e.g. 1.5 – 2.0
}

func emaLast(values []float64, period int) float64 {
	if period <= 0 || len(values) == 0 {
		return 0
	}
	if len(values) < period {
		period = len(values)
	}

	start := len(values) - period

	sum := 0.0
	for i := start; i < len(values); i++ {
		sum += values[i]
	}
	ema := sum / float64(period)

	alpha := 2.0 / (float64(period) + 1.0)
	for i := start; i < len(values); i++ {
		ema = alpha*values[i] + (1.0-alpha)*ema
	}
	return ema
}

func (kc *KeltnerChannel) Calculate(candles []models.CandleStick) (lower, middle, upper float64, err error) {
	if kc.Period < 2 {
		return 0, 0, 0, fmt.Errorf("keltner: period must be >= 2 (got %d)", kc.Period)
	}
	if kc.Multiplier <= 0 {
		return 0, 0, 0, fmt.Errorf("keltner: multiplier must be > 0 (got %.4f)", kc.Multiplier)
	}
	if len(candles) < kc.Period+1 {
		return 0, 0, 0, fmt.Errorf("keltner: not enough data, need at least %d+1 candles, got %d", kc.Period, len(candles))
	}

	tps := make([]float64, len(candles))
	for i, c := range candles {
		tps[i] = (c.High + c.Low + c.Close) / 3.0
	}

	middle = emaLast(tps, kc.Period)
	if math.IsNaN(middle) || math.IsInf(middle, 0) {
		return 0, 0, 0, fmt.Errorf("keltner: invalid EMA (mid=%.6f)", middle)
	}

	atr, err := ATR(candles, kc.Period)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("keltner: ATR failed: %w", err)
	}
	if atr < 0 {
		atr = 0
	}

	upper = middle + kc.Multiplier*atr
	lower = middle - kc.Multiplier*atr

	if math.IsNaN(lower) || math.IsNaN(upper) {
		return 0, 0, 0, fmt.Errorf("keltner: NaN bands (mid=%.6f, atr=%.6f)", middle, atr)
	}

	return lower, middle, upper, nil
}
