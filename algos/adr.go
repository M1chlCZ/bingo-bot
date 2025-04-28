package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"math"
)

type ADRStrategy struct {
	Period     int     `json:"period"`     // rolling window for ADR / ADX
	Multiplier float64 `json:"multiplier"` // threshold multiplier
}

// ---------------- ADR ----------------------------

func (a *ADRStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < a.Period+1 {
		return 0, 0, fmt.Errorf("not enough candles to calculate ADR, need at least %d", a.Period+1)
	}
	adr, err := calculateADR(candles, a.Period)
	if err != nil {
		return 0, 0, err
	}
	latest := candles[len(candles)-1]
	todayRange := latest.High - latest.Low

	upper := adr * a.Multiplier
	lower := adr * (1 - a.Multiplier*0.5)

	var signal int
	switch {
	case todayRange > upper:
		signal = -1 // likely over‑extended
	case todayRange < lower:
		signal = 1 // compressed range → expansion likely
	default:
		signal = 0
	}
	return adr, signal, nil
}

func calculateADR(candles []models.CandleStick, period int) (float64, error) {
	if period <= 0 {
		return 0, fmt.Errorf("ADR period must be > 0")
	}
	if len(candles) < period {
		return 0, fmt.Errorf("not enough candles for ADR")
	}
	total := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		total += candles[i].High - candles[i].Low
	}
	return total / float64(period), nil
}

// CalculateADX returns the most recent Average Directional Index value.
// ADX ≥ 25 is often interpreted as a strong trend.
func (a *ADRStrategy) CalculateADX(candles []models.CandleStick) (float64, error) {
	p := a.Period
	if p == 0 {
		p = 14
	}
	if len(candles) < p+1 {
		return 0, fmt.Errorf("not enough candles for ADX")
	}

	var (
		trs  []float64 // True Range
		pdms []float64 // +DM
		ndms []float64 // -DM
	)

	// --- 1 · raw TR, +DM, -DM -------------------
	for i := 1; i < len(candles); i++ {
		high := candles[i].High
		low := candles[i].Low
		close := candles[i-1].Close

		upMove := high - candles[i-1].High
		downMove := candles[i-1].Low - low

		var pdm, ndm float64
		if upMove > downMove && upMove > 0 {
			pdm = upMove
		}
		if downMove > upMove && downMove > 0 {
			ndm = downMove
		}
		tr := math.Max(high-low, math.Max(math.Abs(high-close), math.Abs(low-close)))

		trs = append(trs, tr)
		pdms = append(pdms, pdm)
		ndms = append(ndms, ndm)
	}

	// --- 2 · smoothed TR/+DM/-DM (Wilder’s) -----
	smooth := func(src []float64, period int) []float64 {
		out := make([]float64, len(src))
		sum := 0.0
		for i := 0; i < len(src); i++ {
			if i < period {
				sum += src[i]
				if i == period-1 {
					out[i] = sum
				}
			} else {
				sum = out[i-1] - out[i-1]/float64(period) + src[i]
				out[i] = sum
			}
		}
		return out
	}

	str := smooth(trs, p)
	spdm := smooth(pdms, p)
	sndm := smooth(ndms, p)

	// --- 3 · DI and DX ---------------------------
	dis := make([]float64, len(str))
	for i := p - 1; i < len(str); i++ {
		if str[i] == 0 {
			continue
		}
		plusDI := 100 * (spdm[i] / str[i])
		minusDI := 100 * (sndm[i] / str[i])
		dx := 100 * math.Abs(plusDI-minusDI) / (plusDI + minusDI)
		dis[i] = dx
	}

	// --- 4 · ADX (smoothed DX) -------------------
	adxs := smooth(dis, p)

	last := adxs[len(adxs)-1] / float64(p) // Wilder divides smoothed sum by period
	return last, nil
}
