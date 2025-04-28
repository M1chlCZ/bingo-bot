package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"math"
)

// ATRStrategy represents a trading strategy based on the Average True Range (ATR) indicator.
// ATR measures market volatility and can be used to set stop-loss levels or identify potential breakouts.
type ATRStrategy struct {
	Period     int     `json:"period"`     // The number of periods to use for ATR calculation
	Multiplier float64 `json:"multiplier"` // Multiplier for ATR to determine significant price movements
}

// Calculate computes the ATR value and returns a signal based on price movements relative to ATR.
// Returns:
// - float64: The current ATR value
// - int: Signal (-1 for sell, 0 for hold, 1 for buy)
// - error: Any error encountered during calculation
func ATR(candles []models.CandleStick, period int) (float64, error) {
	if len(candles) < period+1 {
		return 0, fmt.Errorf("too few candles")
	}
	var trSum float64
	for i := len(candles) - period; i < len(candles); i++ {
		high := candles[i].High
		low := candles[i].Low
		prevClose := candles[i-1].Close
		tr := math.Max(high-low, math.Max(
			math.Abs(high-prevClose),
			math.Abs(low-prevClose)))
		trSum += tr
	}
	return trSum / float64(period), nil
}

// calculateATR computes the Average True Range for the given candles and period.
// ATR is calculated as the exponential moving average of the true range.
// True Range is the greatest of:
// 1. Current High - Current Low
// 2. |Current High - Previous Close|
// 3. |Current Low - Previous Close|
func calculateATR(candles []models.CandleStick, period int) ([]float64, error) {
	if period <= 0 {
		return nil, fmt.Errorf("ATR period must be greater than zero")
	}

	if len(candles) < period+1 {
		return nil, fmt.Errorf("not enough data to calculate ATR: need at least %d candles, got %d", period+1, len(candles))
	}

	// Calculate True Range for each candle
	trueRanges := make([]float64, len(candles)-1)

	for i := 1; i < len(candles); i++ {
		// Calculate the three differences
		highLowDiff := candles[i].High - candles[i].Low
		highCloseDiff := math.Abs(candles[i].High - candles[i-1].Close)
		lowCloseDiff := math.Abs(candles[i].Low - candles[i-1].Close)

		// True Range is the maximum of the three differences
		trueRanges[i-1] = math.Max(highLowDiff, math.Max(highCloseDiff, lowCloseDiff))
	}

	// Calculate ATR using simple moving average for the first period
	if len(trueRanges) < period {
		return nil, fmt.Errorf("not enough data to calculate initial ATR")
	}

	// Calculate the first ATR as a simple average of the first 'period' true ranges
	firstATR := 0.0
	for i := 0; i < period; i++ {
		firstATR += trueRanges[i]
	}
	firstATR /= float64(period)

	// ATR array starts after 'period' candles of data have been processed
	atrCount := len(candles) - period
	atr := make([]float64, atrCount)
	atr[0] = firstATR

	// Calculate subsequent ATR values using the smoothing formula:
	// ATR(t) = [(ATR(t-1) * (period-1)) + TR(t)] / period
	for i := 1; i < atrCount; i++ {
		trIndex := i + period - 1
		if trIndex >= len(trueRanges) {
			return nil, fmt.Errorf("index out of range while calculating ATR")
		}

		atr[i] = ((atr[i-1] * float64(period-1)) + trueRanges[trIndex]) / float64(period)
	}

	return atr, nil
}
