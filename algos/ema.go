package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

func CalculateEMA(candles []models.CandleStick, period int) []float64 {
	if period < 2 || len(candles) < period {
		return nil
	}
	vals := make([]float64, len(candles))
	for i := range candles {
		vals[i] = candles[i].Close
	}
	return CalculateEMAFromValues(vals, period)
}

func CalculateEMAFromValues(values []float64, period int) []float64 {
	if period < 2 || len(values) < period {
		return nil
	}
	return emaOnValues(values, period)
}

func emaOnValues(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n-period+1)

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	prev := sum / float64(period)
	out[0] = prev

	if math.IsNaN(prev) || math.IsInf(prev, 0) {

		return nil
	}

	alpha := 2.0 / (float64(period) + 1.0) // EMA multiplier

	oi := 1
	for i := period; i < n; i++ {
		next := (values[i]-prev)*alpha + prev
		if math.IsNaN(next) || math.IsInf(next, 0) {
			return nil
		}
		out[oi] = next
		prev = next
		oi++
	}
	return out
}

type EMAStrategy struct {
	Period       int  `json:"period"`       // The number of periods to use for EMA calculation (used when UseCrossover=false)
	FastPeriod   int  `json:"fastPeriod"`   // Period for the fast EMA in a crossover strategy
	SlowPeriod   int  `json:"slowPeriod"`   // Period for the slow EMA in a crossover strategy
	UseCrossover bool `json:"useCrossover"` // Whether to use EMA crossover for signals
}

func (e *EMAStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < 2 {
		return 0, 0, fmt.Errorf("ema: need at least 2 candles")
	}

	const eps = 0.0005

	if e.UseCrossover {

		if e.FastPeriod < 2 || e.SlowPeriod < 2 || e.FastPeriod >= e.SlowPeriod {
			return 0, 0, fmt.Errorf("invalid EMA periods: fast=%d, slow=%d", e.FastPeriod, e.SlowPeriod)
		}
		if len(candles) < e.SlowPeriod {
			return 0, 0, fmt.Errorf("not enough candles: need >= %d, got %d", e.SlowPeriod, len(candles))
		}

		fastEMA := CalculateEMA(candles, e.FastPeriod)
		slowEMA := CalculateEMA(candles, e.SlowPeriod)
		if fastEMA == nil || slowEMA == nil {
			return 0, 0, fmt.Errorf("failed to calculate EMA (fast or slow)")
		}
		if len(fastEMA) < 2 || len(slowEMA) < 2 {
			return 0, 0, fmt.Errorf("not enough EMA points to detect crossover")
		}

		if len(fastEMA) > len(slowEMA) {
			fastEMA = fastEMA[len(fastEMA)-len(slowEMA):]
		} else if len(slowEMA) > len(fastEMA) {
			slowEMA = slowEMA[len(slowEMA)-len(fastEMA):]
		}
		if len(fastEMA) < 2 { // recheck after alignment
			return 0, 0, fmt.Errorf("not enough aligned EMA points")
		}

		fc := fastEMA[len(fastEMA)-1]
		fp := fastEMA[len(fastEMA)-2]
		sc := slowEMA[len(slowEMA)-1]
		sp := slowEMA[len(slowEMA)-2]

		if fp <= sp && fc > sc*(1.0+eps) {
			return fc, 1, nil
		}

		if fp >= sp && fc < sc*(1.0-eps) {
			return fc, -1, nil
		}

		return fc, 0, nil
	}

	if e.Period < 2 {
		return 0, 0, fmt.Errorf("EMA period must be >= 2 (got %d)", e.Period)
	}
	if len(candles) < e.Period {
		return 0, 0, fmt.Errorf("not enough candles: need >= %d, got %d", e.Period, len(candles))
	}

	emaVals := CalculateEMA(candles, e.Period)
	if emaVals == nil || len(emaVals) < 2 {
		return 0, 0, fmt.Errorf("not enough EMA points for price crossover")
	}

	curEMA := emaVals[len(emaVals)-1]
	prevEMA := emaVals[len(emaVals)-2]
	curPrice := candles[len(candles)-1].Close
	prevPrice := candles[len(candles)-2].Close

	if prevPrice <= prevEMA && curPrice > curEMA*(1.0+eps) {
		return curEMA, 1, nil
	}

	if prevPrice >= prevEMA && curPrice < curEMA*(1.0-eps) {
		return curEMA, -1, nil
	}

	return curEMA, 0, nil
}
