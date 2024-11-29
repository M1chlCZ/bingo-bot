package strategies

import (
	"binance_bot/models"
	"fmt"
)

type MACDStrategy struct {
	FastPeriod   int
	SlowPeriod   int
	SignalPeriod int
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
	if len(candles) < slowPeriod {
		return 0, 0, 0, fmt.Errorf("not enough data to calculate MACD: need %d candles, got %d", slowPeriod, len(candles))
	}

	// Calculate EMAs
	fastEMA := calculateEMA(candles, fastPeriod)
	slowEMA := calculateEMA(candles, slowPeriod)

	// Align the lengths of fastEMA and slowEMA
	alignmentStart := len(fastEMA) - len(slowEMA)
	if alignmentStart < 0 || len(fastEMA) < len(slowEMA) {
		return 0, 0, 0, fmt.Errorf("misaligned EMA lengths: fastEMA=%d, slowEMA=%d", len(fastEMA), len(slowEMA))
	}
	fastEMA = fastEMA[alignmentStart:]

	// Ensure the lengths match
	if len(fastEMA) != len(slowEMA) {
		return 0, 0, 0, fmt.Errorf("aligned EMA lengths still mismatch: fastEMA=%d, slowEMA=%d", len(fastEMA), len(slowEMA))
	}

	// MACD Line = Fast EMA - Slow EMA
	macdValues := make([]float64, len(fastEMA))
	for i := range fastEMA {
		macdValues[i] = fastEMA[i] - slowEMA[i]
	}

	// Signal Line = EMA of MACD Line
	signalLine := calculateEMAFromValues(macdValues, signalPeriod)
	if len(signalLine) == 0 {
		return 0, 0, 0, fmt.Errorf("failed to calculate Signal Line")
	}

	// Histogram = MACD Line - Signal Line
	macdLine := macdValues[len(macdValues)-1]
	histogram := macdLine - signalLine[len(signalLine)-1]

	return macdLine, signalLine[len(signalLine)-1], histogram, nil
}

// Helper function to calculate EMA
func calculateEMA(candles []models.CandleStick, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	ema := make([]float64, len(candles))
	multiplier := 2.0 / (float64(period) + 1.0)

	// First EMA value is the simple moving average of the first period
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	ema[period-1] = sum / float64(period)

	// Calculate the rest of the EMA values
	for i := period; i < len(candles); i++ {
		ema[i] = ((candles[i].Close - ema[i-1]) * multiplier) + ema[i-1]
	}

	return ema[period-1:]
}

// Helper function to calculate EMA from arbitrary values
func calculateEMAFromValues(values []float64, period int) []float64 {
	if len(values) < period {
		return nil
	}

	ema := make([]float64, len(values))
	multiplier := 2.0 / (float64(period) + 1.0)

	// First EMA value is the simple moving average of the first period
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += values[i]
	}
	ema[period-1] = sum / float64(period)

	// Calculate the rest of the EMA values
	for i := period; i < len(values); i++ {
		ema[i] = ((values[i] - ema[i-1]) * multiplier) + ema[i-1]
	}

	return ema[period-1:]
}
