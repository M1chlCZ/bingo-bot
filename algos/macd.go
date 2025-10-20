package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type MACDStrategy struct {
	FastPeriod   int `json:"fastPeriod"`
	SlowPeriod   int `json:"slowPeriod"`
	SignalPeriod int `json:"signalPeriod"`
}

// Returns:
//
//	histogram, signalLine, macdLine, signal, error
//
// Signal semantics: 1=buy, -1=sell, 0=hold.
func (m *MACDStrategy) Calculate(candles []models.CandleStick) (histogram float64, signalLine float64, macdLine float64, signal int, err error) {
	macdLine, signalLine, histogram, err = CalculateMACD(candles, m.FastPeriod, m.SlowPeriod, m.SignalPeriod)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	// Small deadband to reduce whipsaw on microscopic crosses (~0.05%)
	const eps = 0.0005

	if macdLine > signalLine*(1.0+eps) && histogram > 0 {
		return histogram, signalLine, macdLine, 1, nil // Buy
	}
	if macdLine < signalLine*(1.0-eps) && histogram < 0 {
		return histogram, signalLine, macdLine, -1, nil // Sell
	}
	return histogram, signalLine, macdLine, 0, nil // Hold
}

// CalculateMACD calculates the (last) MACD line, signal line, and histogram.
// Returns (macdLine, signalLine, histogram, error).
func CalculateMACD(candles []models.CandleStick, fastPeriod, slowPeriod, signalPeriod int) (float64, float64, float64, error) {
	// Validate periods
	if fastPeriod < 2 || slowPeriod < 2 || signalPeriod < 1 {
		return 0, 0, 0, fmt.Errorf("macd: invalid periods (fast=%d slow=%d signal=%d)", fastPeriod, slowPeriod, signalPeriod)
	}
	if fastPeriod >= slowPeriod {
		return 0, 0, 0, fmt.Errorf("macd: fastPeriod (%d) must be < slowPeriod (%d)", fastPeriod, slowPeriod)
	}

	// Need enough candles for slow EMA AND for signal EMA on MACD values.
	// After EMAs, MACD series length ≈ len(candles) - slowPeriod + 1,
	// which must be >= signalPeriod to compute at least one signal value.
	minCandles := slowPeriod + signalPeriod - 1
	if len(candles) < minCandles {
		return 0, 0, 0, fmt.Errorf("macd: not enough data; need >= %d candles, got %d", minCandles, len(candles))
	}

	// Compute EMAs of close
	fastEMA := CalculateEMA(candles, fastPeriod)
	slowEMA := CalculateEMA(candles, slowPeriod)
	if fastEMA == nil || slowEMA == nil {
		return 0, 0, 0, fmt.Errorf("macd: failed to calculate EMAs")
	}
	if len(fastEMA) == 0 || len(slowEMA) == 0 {
		return 0, 0, 0, fmt.Errorf("macd: empty EMA output")
	}

	// Right-align both EMA slices to same length (trim the longer one on the left).
	// This avoids assumptions about which is longer.
	n := len(fastEMA)
	if len(slowEMA) < n {
		n = len(slowEMA)
	}
	fastEMA = fastEMA[len(fastEMA)-n:]
	slowEMA = slowEMA[len(slowEMA)-n:]

	// Build MACD series: fast - slow
	macdValues := make([]float64, n)
	for i := 0; i < n; i++ {
		v := fastEMA[i] - slowEMA[i]
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0, 0, fmt.Errorf("macd: NaN/Inf in MACD series")
		}
		macdValues[i] = v
	}

	// Signal line = EMA(macd, signalPeriod)
	signalSeries := CalculateEMAFromValues(macdValues, signalPeriod)
	if signalSeries == nil || len(signalSeries) == 0 {
		return 0, 0, 0, fmt.Errorf("macd: failed to calculate signal line; macdLen=%d signalPeriod=%d", len(macdValues), signalPeriod)
	}

	// Align last values: signalSeries is shorter by signalPeriod-1; its last aligns to macdValues last.
	macdLast := macdValues[len(macdValues)-1]
	signalLast := signalSeries[len(signalSeries)-1]
	histLast := macdLast - signalLast

	return macdLast, signalLast, histLast, nil
}
