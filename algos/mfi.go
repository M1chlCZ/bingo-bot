package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type MFIStrategy struct {
	Overbought int `json:"overbought"`
	Oversold   int `json:"oversold"`
	Period     int `json:"period"`
}

// signal: -1 if MFI > Overbought, +1 if MFI < Oversold, else 0.
func (m *MFIStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if m.Period < 2 {
		return 0, 0, fmt.Errorf("MFI: period must be >= 2 (got %d)", m.Period)
	}
	// Need at least Period+1 candles to form Period money-flow windows (TR-like)
	if len(candles) < m.Period+1 {
		return 0, 0, fmt.Errorf("not enough data for MFI: need %d, got %d", m.Period+1, len(candles))
	}

	// Build positive/negative raw money flow over each bar (except the very first)
	nFlow := len(candles) - 1
	posFlow := make([]float64, nFlow)
	negFlow := make([]float64, nFlow)

	for i := 1; i < len(candles); i++ {
		prev := candles[i-1]
		curr := candles[i]

		typPrev := (prev.High + prev.Low + prev.Close) / 3.0
		typCurr := (curr.High + curr.Low + curr.Close) / 3.0
		rawMF := typCurr * curr.Volume
		if rawMF < 0 || math.IsNaN(rawMF) || math.IsInf(rawMF, 0) {
			rawMF = 0 // defensive
		}

		switch {
		case typCurr > typPrev:
			posFlow[i-1] = rawMF
			negFlow[i-1] = 0
		case typCurr < typPrev:
			posFlow[i-1] = 0
			negFlow[i-1] = rawMF
		default:
			// equal typical price → neither positive nor negative
			posFlow[i-1] = 0
			negFlow[i-1] = 0
		}
	}

	// Rolling sums over 'Period' of pos/neg flow on the posFlow/negFlow arrays.
	// Window ends at index i, starting at i = Period-1.
	win := m.Period
	if nFlow < win {
		return 0, 0, fmt.Errorf("internal: flow length %d < window %d", nFlow, win)
	}

	sumPos, sumNeg := 0.0, 0.0
	for i := 0; i < win; i++ {
		sumPos += posFlow[i]
		sumNeg += negFlow[i]
	}

	// We only need the latest MFI, but keeping the rolling logic simple & clear.
	latestMFI := mfiFromSums(sumPos, sumNeg)

	for i := win; i < nFlow; i++ {
		sumPos += posFlow[i] - posFlow[i-win]
		sumNeg += negFlow[i] - negFlow[i-win]
		latestMFI = mfiFromSums(sumPos, sumNeg)
	}

	if math.IsNaN(latestMFI) || math.IsInf(latestMFI, 0) {
		latestMFI = 50 // neutral fallback
	}

	signal := 0
	if latestMFI > float64(m.Overbought) {
		signal = -1
	} else if latestMFI < float64(m.Oversold) {
		signal = 1
	}

	return latestMFI, signal, nil
}

// mfiFromSums computes the MFI given windowed sums of positive/negative money flow.
func mfiFromSums(sumPos, sumNeg float64) float64 {
	// Handle degenerate cases first
	if sumPos == 0 && sumNeg == 0 {
		return 50
	}
	if sumNeg == 0 {
		return 100
	}
	if sumPos == 0 {
		return 0
	}
	ratio := sumPos / sumNeg
	return 100.0 - (100.0 / (1.0 + ratio))
}
