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

func (s *SMAStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if s.UseCrossover {

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

		fc, fp := fastSMA[len(fastSMA)-1], fastSMA[len(fastSMA)-2]
		sc, sp := slowSMA[len(slowSMA)-1], slowSMA[len(slowSMA)-2]

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

		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = out[i-1]
		}
		out[i] = v
	}

	return out, nil
}

func crossedUp(aPrev, aCurr, bPrev, bCurr float64, rel float64) bool {

	eps := rel * math.Max(1.0, math.Abs(bCurr))
	return aPrev <= bPrev+eps && aCurr > bCurr+eps
}

func crossedDown(aPrev, aCurr, bPrev, bCurr float64, rel float64) bool {
	eps := rel * math.Max(1.0, math.Abs(bCurr))
	return aPrev >= bPrev-eps && aCurr < bCurr-eps
}
