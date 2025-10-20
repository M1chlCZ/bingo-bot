package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

// a slice aligned so that the first value corresponds to candles[period-1].
// The output length is len(candles) - period + 1.
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

// CalculateEMAFromValues computes the EMA for an arbitrary series of values and returns
// a slice aligned so that the first output corresponds to values[period-1].
// The output length is len(values) - period + 1.
func CalculateEMAFromValues(values []float64, period int) []float64 {
	if period < 2 || len(values) < period {
		return nil
	}
	return emaOnValues(values, period)
}

// ---- Internal helpers ----

// emaOnValues implements a canonical EMA with SMA seed for the first value.
// Returns len(values) - period + 1 points, aligned to period-1..end.
func emaOnValues(values []float64, period int) []float64 {
	n := len(values)
	out := make([]float64, n-period+1)

	// Seed: SMA of first 'period' values
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	prev := sum / float64(period)
	out[0] = prev

	if math.IsNaN(prev) || math.IsInf(prev, 0) {
		// If input contains NaNs, bail defensively
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

// EMAStrategy represents a trading strategy based on the Exponential Moving Average (EMA) indicator.
// EMA gives more weight to recent prices, making it more responsive to new information than a simple moving average.
type EMAStrategy struct {
	Period       int  `json:"period"`       // The number of periods to use for EMA calculation (used when UseCrossover=false)
	FastPeriod   int  `json:"fastPeriod"`   // Period for the fast EMA in a crossover strategy
	SlowPeriod   int  `json:"slowPeriod"`   // Period for the slow EMA in a crossover strategy
	UseCrossover bool `json:"useCrossover"` // Whether to use EMA crossover for signals
}

// Calculate computes the EMA value and returns a signal based on the strategy configuration.
// If UseCrossover is true, it generates signals based on fast EMA crossing slow EMA.
// Otherwise, it generates signals based on price crossing the EMA.
// Returns:
//   - float64: The current EMA value (fast EMA if using crossover)
//   - int: Signal (-1 for sell, 0 for hold, 1 for buy)
//   - error: Any error encountered during calculation
func (e *EMAStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < 2 {
		return 0, 0, fmt.Errorf("ema: need at least 2 candles")
	}

	// Small deadband to reduce flip-flopping on tiny crosses (≈0.05% default).
	const eps = 0.0005

	if e.UseCrossover {
		// Validate periods
		if e.FastPeriod < 2 || e.SlowPeriod < 2 || e.FastPeriod >= e.SlowPeriod {
			return 0, 0, fmt.Errorf("invalid EMA periods: fast=%d, slow=%d", e.FastPeriod, e.SlowPeriod)
		}
		if len(candles) < e.SlowPeriod {
			return 0, 0, fmt.Errorf("not enough candles: need >= %d, got %d", e.SlowPeriod, len(candles))
		}

		// Compute EMAs
		fastEMA := CalculateEMA(candles, e.FastPeriod)
		slowEMA := CalculateEMA(candles, e.SlowPeriod)
		if fastEMA == nil || slowEMA == nil {
			return 0, 0, fmt.Errorf("failed to calculate EMA (fast or slow)")
		}
		if len(fastEMA) < 2 || len(slowEMA) < 2 {
			return 0, 0, fmt.Errorf("not enough EMA points to detect crossover")
		}

		// Align by trimming the longer series on the left
		// so that both series end at the same candle.
		if len(fastEMA) > len(slowEMA) {
			fastEMA = fastEMA[len(fastEMA)-len(slowEMA):]
		} else if len(slowEMA) > len(fastEMA) {
			slowEMA = slowEMA[len(slowEMA)-len(fastEMA):]
		}
		if len(fastEMA) < 2 { // recheck after alignment
			return 0, 0, fmt.Errorf("not enough aligned EMA points")
		}

		// Current & previous values
		fc := fastEMA[len(fastEMA)-1]
		fp := fastEMA[len(fastEMA)-2]
		sc := slowEMA[len(slowEMA)-1]
		sp := slowEMA[len(slowEMA)-2]

		// Apply small deadband to avoid signaling on microscopic crosses
		// Buy: fast crosses above slow with a margin
		if fp <= sp && fc > sc*(1.0+eps) {
			return fc, 1, nil
		}
		// Sell: fast crosses below slow with a margin
		if fp >= sp && fc < sc*(1.0-eps) {
			return fc, -1, nil
		}

		return fc, 0, nil
	}

	// Single EMA mode: price vs EMA
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

	// Buy: price crosses above EMA with small margin
	if prevPrice <= prevEMA && curPrice > curEMA*(1.0+eps) {
		return curEMA, 1, nil
	}
	// Sell: price crosses below EMA with small margin
	if prevPrice >= prevEMA && curPrice < curEMA*(1.0-eps) {
		return curEMA, -1, nil
	}

	return curEMA, 0, nil
}
