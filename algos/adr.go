package algos

import (
	"fmt"
	"math"
	"sort"

	"github.com/M1chlCZ/bingo-bot/models"
)

type ADRStrategy struct {
	Period     int     `json:"period"`     // rolling window for ADR / ADX (typical: 14)
	Multiplier float64 `json:"multiplier"` // expansion threshold, e.g. 1.5 → today range >= 1.5×ADR
}

// Calculate returns (ADR, signal).
// signal:  -1 = over-extended day (range expansion), 0 = neutral, 1 = compressed day (range likely to expand soon)
func (a *ADRStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	p := a.Period
	if p <= 0 {
		p = 14
	}
	if len(candles) < p {
		return 0, 0, fmt.Errorf("not enough candles to calculate ADR, need at least %d", p)
	}

	adr, err := calculateADRRobust(candles, p)
	if err != nil || adr <= 0 {
		if err == nil {
			err = fmt.Errorf("invalid ADR (%.6f)", adr)
		}
		return 0, 0, err
	}

	latest := candles[len(candles)-1]
	todayRange := latest.High - latest.Low
	if todayRange <= 0 {
		return adr, 0, nil
	}

	ratio := todayRange / adr
	if a.Multiplier <= 0 {
		a.Multiplier = 1.5
	}

	// Expansion when today's range is significantly larger than ADR.
	if ratio >= a.Multiplier {
		return adr, -1, nil
	}

	// Compression when today's range is much smaller than ADR.
	// 0.70 is a decent, conservative default for crypto.
	if ratio <= 0.70 {
		return adr, 1, nil
	}

	return adr, 0, nil
}

// calculateADRRobust computes a trimmed-mean ADR over the last `period` candles,
// reducing the impact of single outliers (crypto spikes).
func calculateADRRobust(candles []models.CandleStick, period int) (float64, error) {
	if period <= 0 {
		return 0, fmt.Errorf("ADR period must be > 0")
	}
	if len(candles) < period {
		return 0, fmt.Errorf("not enough candles for ADR")
	}

	ranges := make([]float64, 0, period)
	for i := len(candles) - period; i < len(candles); i++ {
		r := candles[i].High - candles[i].Low
		if r > 0 {
			ranges = append(ranges, r)
		}
	}
	if len(ranges) == 0 {
		return 0, nil
	}

	// Trim 10% on each side (min 1 element when big enough)
	sort.Float64s(ranges)
	trim := int(math.Floor(0.10 * float64(len(ranges))))
	start := trim
	end := len(ranges) - trim
	if end <= start {
		start = 0
		end = len(ranges)
	}
	sum := 0.0
	for _, v := range ranges[start:end] {
		sum += v
	}
	return sum / float64(end-start), nil
}

// ---------------- ADX (Wilder) ------------------

// CalculateADX returns the most recent Average Directional Index value (Wilder).
// ADX ≥ ~25 is often interpreted as a “tradable/strong” trend.
func (a *ADRStrategy) CalculateADX(candles []models.CandleStick) (float64, error) {
	p := a.Period
	if p <= 0 {
		p = 14
	}
	if len(candles) < p+1 {
		return 0, fmt.Errorf("not enough candles for ADX (need >= %d)", p+1)
	}

	// 1) Raw TR, +DM, -DM
	tr := make([]float64, 0, len(candles)-1)
	pdm := make([]float64, 0, len(candles)-1)
	ndm := make([]float64, 0, len(candles)-1)

	for i := 1; i < len(candles); i++ {
		h := candles[i].High
		l := candles[i].Low
		pc := candles[i-1].Close

		upMove := h - candles[i-1].High
		downMove := candles[i-1].Low - l

		plusDM := 0.0
		minusDM := 0.0
		if upMove > downMove && upMove > 0 {
			plusDM = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM = downMove
		}

		trueRange := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))

		tr = append(tr, trueRange)
		pdm = append(pdm, plusDM)
		ndm = append(ndm, minusDM)
	}

	if len(tr) < p {
		return 0, fmt.Errorf("not enough TR samples")
	}

	// 2) Wilder smoothing of TR, +DM, -DM
	tr14 := wilderSmooth(tr, p)
	pdm14 := wilderSmooth(pdm, p)
	ndm14 := wilderSmooth(ndm, p)
	if len(tr14) == 0 || len(pdm14) == 0 || len(ndm14) == 0 {
		return 0, fmt.Errorf("smoothing failed")
	}

	// 3) Build DX series (aligned)
	dxSeries := make([]float64, 0, len(tr14))
	for i := 0; i < len(tr14); i++ {
		t := tr14[i]
		if t <= 0 {
			dxSeries = append(dxSeries, 0)
			continue
		}
		plus := 100.0 * (pdm14[i] / t)
		minus := 100.0 * (ndm14[i] / t)
		den := plus + minus
		if den <= 0 {
			dxSeries = append(dxSeries, 0)
			continue
		}
		dx := 100.0 * math.Abs(plus-minus) / den
		dxSeries = append(dxSeries, dx)
	}

	// 4) ADX = Wilder-smoothed DX (seed with avg of first p DX)
	adx := wilderADX(dxSeries, p)
	return adx, nil
}

// wilderSmooth applies Wilder's smoothing to src with period p, returning an aligned series
// of the same length as src starting at index p-1 (we keep full length for simplicity).
func wilderSmooth(src []float64, p int) []float64 {
	if p <= 0 || len(src) < p {
		return nil
	}
	out := make([]float64, len(src))
	// seed: sum of first p values
	sum := 0.0
	for i := 0; i < p; i++ {
		sum += src[i]
	}
	out[p-1] = sum
	// Wilder smoothing for subsequent values
	for i := p; i < len(src); i++ {
		sum = out[i-1] - out[i-1]/float64(p) + src[i]
		out[i] = sum
	}
	// values before p-1 are zero (unaligned); that's fine for our usage
	return out
}

// wilderADX computes the final ADX from the DX series using Wilder's method.
// Seed ADX = average of first p DX values, then ADX_t = ((ADX_{t-1}*(p-1)) + DX_t) / p
func wilderADX(dx []float64, p int) float64 {
	if p <= 0 || len(dx) < p {
		return 0
	}
	// seed
	sum := 0.0
	for i := 0; i < p; i++ {
		sum += dx[i]
	}
	adx := sum / float64(p)
	// Wilder forward
	for i := p; i < len(dx); i++ {
		adx = ((adx * float64(p-1)) + dx[i]) / float64(p)
	}
	return adx
}
