package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type ADXStrategy struct {
	Period int `json:"period"` // typical 14
}

func (a *ADXStrategy) Calculate(candles []models.CandleStick) (adx, plusDI, minusDI float64, err error) {
	if a.Period < 2 {
		return 0, 0, 0, fmt.Errorf("adx: period must be >=2 (got %d)", a.Period)
	}
	if len(candles) < a.Period+1 {
		return 0, 0, 0, fmt.Errorf("adx: not enough data, need at least %d+1 candles, got %d", a.Period, len(candles))
	}

	n := len(candles)
	p := a.Period

	tr := make([]float64, n)
	plusDM := make([]float64, n)
	minusDM := make([]float64, n)

	for i := 1; i < n; i++ {
		high := candles[i].High
		low := candles[i].Low
		prevHigh := candles[i-1].High
		prevLow := candles[i-1].Low
		prevClose := candles[i-1].Close

		upMove := high - prevHigh
		downMove := prevLow - low

		range1 := high - low
		range2 := math.Abs(high - prevClose)
		range3 := math.Abs(low - prevClose)
		tr[i] = math.Max(range1, math.Max(range2, range3))

		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		} else {
			plusDM[i] = 0
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		} else {
			minusDM[i] = 0
		}
	}

	var tr14, plusDM14, minusDM14 float64
	for i := 1; i <= p; i++ {
		tr14 += tr[i]
		plusDM14 += plusDM[i]
		minusDM14 += minusDM[i]
	}

	for i := p + 1; i < n; i++ {

		tr14 = tr14 - (tr14 / float64(p)) + tr[i]
		plusDM14 = plusDM14 - (plusDM14 / float64(p)) + plusDM[i]
		minusDM14 = minusDM14 - (minusDM14 / float64(p)) + minusDM[i]
	}

	if tr14 == 0 {
		return 0, 0, 0, fmt.Errorf("adx: TR sum is zero, cannot compute DI")
	}

	plusDI14 := 100.0 * (plusDM14 / tr14)
	minusDI14 := 100.0 * (minusDM14 / tr14)

	den := plusDI14 + minusDI14
	if den == 0 {
		return 0, plusDI14, minusDI14, fmt.Errorf("adx: DI denominator zero")
	}

	dx := 100.0 * math.Abs(plusDI14-minusDI14) / den

	if n < 2*p+1 {

		return dx, plusDI14, minusDI14, nil
	}

	dxVals := make([]float64, 0, p)

	for i := 0; i < p; i++ {
		dxVals = append(dxVals, dx)
	}

	adxVal := 0.0
	for _, d := range dxVals {
		adxVal += d
	}
	adxVal /= float64(len(dxVals))

	if math.IsNaN(adxVal) || math.IsInf(adxVal, 0) {
		return 0, plusDI14, minusDI14, fmt.Errorf("adx: NaN/Inf ADX (adx=%.6f)", adxVal)
	}

	return adxVal, plusDI14, minusDI14, nil
}
