package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
)

type MACDStrategy struct {
	FastPeriod   int `json:"fastPeriod"`
	SlowPeriod   int `json:"slowPeriod"`
	SignalPeriod int `json:"signalPeriod"`
}

// Calculate generates a signal based on MACD crossovers
func (m *MACDStrategy) Calculate(candles []models.CandleStick) (histogram float64, signalLine float64, macdLine float64, signal int, err error) {
	macdLine, signalLine, histogram, err = CalculateMACD(candles, m.FastPeriod, m.SlowPeriod, m.SignalPeriod)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	if macdLine > signalLine && histogram > 0 {
		return histogram, signalLine, macdLine, 1, nil // Buy signal
	} else if macdLine < signalLine && histogram < 0 {
		return histogram, signalLine, macdLine, -1, nil // Sell signal
	}

	return histogram, signalLine, macdLine, 0, nil // Hold
}

// CalculateMACD calculates the MACD line, signal line, and histogram from a series of candles
func CalculateMACD(candles []models.CandleStick, fastPeriod, slowPeriod, signalPeriod int) (float64, float64, float64, error) {
	// Check if there are enough candles for the slow period
	if len(candles) < slowPeriod {
		return 0, 0, 0, fmt.Errorf("not enough data to calculate MACD: need at least %d candles, got %d", slowPeriod, len(candles))
	}

	// Calculate EMAs
	fastEMA := CalculateEMA(candles, fastPeriod)
	slowEMA := CalculateEMA(candles, slowPeriod)

	// Ensure fastEMA and slowEMA have valid lengths
	if fastEMA == nil || slowEMA == nil {
		return 0, 0, 0, fmt.Errorf("failed to calculate EMAs: fastEMA=%v, slowEMA=%v", len(fastEMA), len(slowEMA))
	}

	// Align the lengths of fastEMA and slowEMA
	alignmentStart := len(fastEMA) - len(slowEMA)
	if alignmentStart < 0 || len(fastEMA) < len(slowEMA) {
		return 0, 0, 0, fmt.Errorf("misaligned EMA lengths: fastEMA=%d, slowEMA=%d", len(fastEMA), len(slowEMA))
	}
	fastEMA = fastEMA[alignmentStart:]

	// Ensure the lengths match
	if len(fastEMA) != len(slowEMA) || len(fastEMA) == 0 {
		return 0, 0, 0, fmt.Errorf("aligned EMA lengths still mismatch or are zero: fastEMA=%d, slowEMA=%d", len(fastEMA), len(slowEMA))
	}

	// MACD Line = Fast EMA - Slow EMA
	macdValues := make([]float64, len(fastEMA))
	for i := range fastEMA {
		macdValues[i] = fastEMA[i] - slowEMA[i]
	}

	// Calculate Signal Line (EMA of MACD values)
	signalLine := CalculateEMAFromValues(macdValues, signalPeriod)
	if len(signalLine) == 0 {
		return 0, 0, 0, fmt.Errorf("failed to calculate Signal Line: insufficient MACD values")
	}

	// Ensure there's at least one value to return
	if len(macdValues) == 0 || len(signalLine) == 0 {
		return 0, 0, 0, fmt.Errorf("not enough data to calculate MACD or Signal Line")
	}

	// Histogram = MACD Line - Signal Line
	macdLine := macdValues[len(macdValues)-1]
	histogram := macdLine - signalLine[len(signalLine)-1]

	return macdLine, signalLine[len(signalLine)-1], histogram, nil
}

// Note: EMA calculation functions have been moved to ema.go
