package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

// ATR measures market volatility and can be used to set stop-loss levels or identify potential breakouts.
type ATRStrategy struct {
	Period     int     `json:"period"`     // The number of periods to use for ATR calculation
	Multiplier float64 `json:"multiplier"` // Multiplier for ATR to determine significant price movements
}

// ATR returns the latest Wilder ATR value for the given period.
// Keeps the original signature but switches implementation to canonical Wilder smoothing:
//
// 1) TR[i] = max(H-L, |H-prevClose|, |L-prevClose|)
// 2) ATR_seed = average(TR[0:p])
// 3) ATR_t = (ATR_{t-1}*(p-1) + TR_t) / p
func ATR(candles []models.CandleStick, period int) (float64, error) {
	if period <= 0 {
		return 0, fmt.Errorf("ATR period must be greater than zero")
	}
	// Need at least p+1 candles to compute p TR values (TR starts from index 1)
	if len(candles) < period+1 {
		return 0, fmt.Errorf("not enough data to calculate ATR: need at least %d candles, got %d", period+1, len(candles))
	}

	// Build TR series for all consecutive pairs
	tr := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		h := candles[i].High
		l := candles[i].Low
		pc := candles[i-1].Close
		trueRange := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		tr = append(tr, trueRange)
	}

	// Seed ATR as simple average of the first `period` TRs
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	atr := sum / float64(period)

	// Wilder smoothing forward to the end
	for i := period; i < len(tr); i++ {
		atr = ((atr * float64(period-1)) + tr[i]) / float64(period)
	}

	return atr, nil
}

// calculateATR computes the full Wilder ATR series (one value per candle after the seed).
// Returns a slice aligned so that atr[0] is the first ATR after seeding, and
// len(atr) == len(candles) - period.
func calculateATR(candles []models.CandleStick, period int) ([]float64, error) {
	if period <= 0 {
		return nil, fmt.Errorf("ATR period must be greater than zero")
	}
	if len(candles) < period+1 {
		return nil, fmt.Errorf("not enough data to calculate ATR: need at least %d candles, got %d", period+1, len(candles))
	}

	// Build TR series
	tr := make([]float64, 0, len(candles)-1)
	for i := 1; i < len(candles); i++ {
		h := candles[i].High
		l := candles[i].Low
		pc := candles[i-1].Close
		trueRange := math.Max(h-l, math.Max(math.Abs(h-pc), math.Abs(l-pc)))
		tr = append(tr, trueRange)
	}

	// Seed ATR with SMA of first `period` TRs
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	seed := sum / float64(period)

	// Allocate output: one ATR value per additional TR after the seed
	// If there are N TR values, we produce N - period + 1 ATR values (including the seed position).
	// To stay consistent with your original docstring, we return len == len(candles)-period.
	atr := make([]float64, len(candles)-period)
	atr[0] = seed

	// Wilder smoothing forward
	prev := seed
	outIdx := 1
	for i := period; i < len(tr); i++ {
		next := ((prev * float64(period-1)) + tr[i]) / float64(period)
		if outIdx >= len(atr) {
			// Should not happen, but guard anyway
			break
		}
		atr[outIdx] = next
		prev = next
		outIdx++
	}

	return atr, nil
}
