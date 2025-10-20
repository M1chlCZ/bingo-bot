package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type SMAStrategy struct {
	Period       int  `json:"period"`       // window for single-SMA mode
	FastPeriod   int  `json:"fastPeriod"`   // fast window for crossover mode
	SlowPeriod   int  `json:"slowPeriod"`   // slow window for crossover mode
	UseCrossover bool `json:"useCrossover"` // use SMA crossover if true
}

// Calculate computes the SMA value and returns a signal based on the configuration.
// Returns current SMA value (or fast SMA in crossover mode), signal (-1 sell, 0 hold, 1 buy), error.
func (s *SMAStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if s.UseCrossover {
		// ---- Crossover mode: need at least SlowPeriod+1 candles to get 2 SMA points ----
		if s.FastPeriod <= 0 || s.SlowPeriod <= 0 || s.FastPeriod >= s.SlowPeriod {
			return 0, 0, fmt.Errorf("invalid SMA periods: fast=%d, slow=%d", s.FastPeriod, s.SlowPeriod)
		}
		if len(candles) < s.SlowPeriod+1 {
			return 0, 0, fmt.Errorf("not enough data: need >= %d candles, got %d", s.SlowPeriod+1, len(candles))
		}

		fastSMA, err := calculateSMA(candles, s.FastPeriod)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to calculate fast SMA: %w", err)
		}
		slowSMA, err := calculateSMA(candles, s.SlowPeriod)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to calculate slow SMA: %w", err)
		}
		if len(fastSMA) < 2 || len(slowSMA) < 2 {
			return 0, 0, fmt.Errorf("not enough SMA values for crossover check")
		}

		// Use last two points for each
		fc, fp := fastSMA[len(fastSMA)-1], fastSMA[len(fastSMA)-2]
		sc, sp := slowSMA[len(slowSMA)-1], slowSMA[len(slowSMA)-2]

		// Hysteresis to avoid flip-flops on tiny touches (relative 0.05%)
		const rel = 0.0005
		up := crossedUp(fp, fc, sp, sc, rel)
		down := crossedDown(fp, fc, sp, sc, rel)

		switch {
		case up:
			return fc, 1, nil
		case down:
			return fc, -1, nil
		default:
			return fc, 0, nil
		}
	}

	// ---- Single-SMA (price vs SMA) mode: need at least Period+1 candles for 2 SMA points ----
	if s.Period <= 0 {
		return 0, 0, fmt.Errorf("SMA period must be greater than zero")
	}
	if len(candles) < s.Period+1 || len(candles) < 2 {
		return 0, 0, fmt.Errorf("not enough data for SMA: need >= %d candles, got %d", s.Period+1, len(candles))
	}

	smaValues, err := calculateSMA(candles, s.Period)
	if err != nil {
		return 0, 0, err
	}
	if len(smaValues) < 2 {
		return 0, 0, fmt.Errorf("not enough SMA values for signal")
	}

	currSMA, prevSMA := smaValues[len(smaValues)-1], smaValues[len(smaValues)-2]
	currPx, prevPx := candles[len(candles)-1].Close, candles[len(candles)-2].Close

	// Relative deadband based on SMA level (avoid jitter around the line)
	const rel = 0.0005
	up := crossedUp(prevPx, currPx, prevSMA, currSMA, rel)
	down := crossedDown(prevPx, currPx, prevSMA, currSMA, rel)

	switch {
	case up:
		return currSMA, 1, nil
	case down:
		return currSMA, -1, nil
	default:
		return currSMA, 0, nil
	}
}

// calculateSMA computes the Simple Moving Average for the given candles and period.
// Returns a slice of length len(candles)-period+1 (right-aligned to the input).
func calculateSMA(candles []models.CandleStick, period int) ([]float64, error) {
	if period <= 0 {
		return nil, fmt.Errorf("SMA period must be greater than zero")
	}
	if len(candles) < period {
		return nil, fmt.Errorf("not enough data to calculate SMA: need at least %d candles, got %d", period, len(candles))
	}

	n := len(candles) - period + 1
	out := make([]float64, n)

	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	out[0] = sum / float64(period)

	for i := 1; i < n; i++ {
		sum += candles[i+period-1].Close - candles[i-1].Close
		v := sum / float64(period)
		// defensive clamp for NaN/Inf
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = out[i-1]
		}
		out[i] = v
	}

	return out, nil
}

// crossedUp returns true if series A crossed above series B between previous and current,
// using a small relative hysteresis to avoid micro-crosses.
func crossedUp(aPrev, aCurr, bPrev, bCurr float64, rel float64) bool {
	// consider magnitudes around current level to set epsilon
	eps := rel * math.Max(1.0, math.Abs(bCurr))
	return aPrev <= bPrev+eps && aCurr > bCurr+eps
}

// crossedDown is the inverse.
func crossedDown(aPrev, aCurr, bPrev, bCurr float64, rel float64) bool {
	eps := rel * math.Max(1.0, math.Abs(bCurr))
	return aPrev >= bPrev-eps && aCurr < bCurr-eps
}
