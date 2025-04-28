package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
)

// CalculateEMA computes the Exponential Moving Average for the given candles and period.
func CalculateEMA(candles []models.CandleStick, period int) []float64 {
	if len(candles) < period {
		return nil // Not enough data to calculate EMA
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

// CalculateEMAFromValues computes the Exponential Moving Average from arbitrary values.
func CalculateEMAFromValues(values []float64, period int) []float64 {
	if len(values) < period {
		return nil // Not enough data to calculate EMA
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

// EMAStrategy represents a trading strategy based on the Exponential Moving Average (EMA) indicator.
// EMA gives more weight to recent prices, making it more responsive to new information than a simple moving average.
type EMAStrategy struct {
	Period       int  `json:"period"`       // The number of periods to use for EMA calculation
	FastPeriod   int  `json:"fastPeriod"`   // Period for the fast EMA in a crossover strategy
	SlowPeriod   int  `json:"slowPeriod"`   // Period for the slow EMA in a crossover strategy
	UseCrossover bool `json:"useCrossover"` // Whether to use EMA crossover for signals
}

// Calculate computes the EMA value and returns a signal based on the strategy configuration.
// If UseCrossover is true, it generates signals based on fast EMA crossing slow EMA.
// Otherwise, it generates signals based on price crossing the EMA.
// Returns:
// - float64: The current EMA value (or fast EMA if using crossover)
// - int: Signal (-1 for sell, 0 for hold, 1 for buy)
// - error: Any error encountered during calculation
func (e *EMAStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if e.UseCrossover {
		// Use EMA crossover strategy
		if e.FastPeriod <= 0 || e.SlowPeriod <= 0 || e.FastPeriod >= e.SlowPeriod {
			return 0, 0, fmt.Errorf("invalid EMA periods: fast=%d, slow=%d", e.FastPeriod, e.SlowPeriod)
		}

		// Calculate fast EMA
		fastEMA := CalculateEMA(candles, e.FastPeriod)
		if fastEMA == nil || len(fastEMA) < 2 {
			return 0, 0, fmt.Errorf("failed to calculate fast EMA or not enough data")
		}

		// Calculate slow EMA
		slowEMA := CalculateEMA(candles, e.SlowPeriod)
		if slowEMA == nil || len(slowEMA) < 2 {
			return 0, 0, fmt.Errorf("failed to calculate slow EMA or not enough data")
		}

		// Align the lengths of fastEMA and slowEMA
		alignmentStart := len(fastEMA) - len(slowEMA)
		if alignmentStart < 0 {
			// If slow EMA is longer (shouldn't happen normally), align fast EMA
			return 0, 0, fmt.Errorf("unexpected EMA length mismatch: fastEMA=%d, slowEMA=%d", len(fastEMA), len(slowEMA))
		}
		fastEMA = fastEMA[alignmentStart:]

		// Get current and previous values
		currentFastEMA := fastEMA[len(fastEMA)-1]
		previousFastEMA := fastEMA[len(fastEMA)-2]
		currentSlowEMA := slowEMA[len(slowEMA)-1]
		previousSlowEMA := slowEMA[len(slowEMA)-2]

		// Check for crossover
		// Buy signal: Fast EMA crosses above Slow EMA
		if previousFastEMA <= previousSlowEMA && currentFastEMA > currentSlowEMA {
			return currentFastEMA, 1, nil
		}
		// Sell signal: Fast EMA crosses below Slow EMA
		if previousFastEMA >= previousSlowEMA && currentFastEMA < currentSlowEMA {
			return currentFastEMA, -1, nil
		}

		return currentFastEMA, 0, nil // No crossover, hold
	} else {
		// Use price crossing EMA strategy
		if e.Period <= 0 {
			return 0, 0, fmt.Errorf("EMA period must be greater than zero")
		}

		emaValues := CalculateEMA(candles, e.Period)
		if emaValues == nil || len(emaValues) < 2 || len(candles) < 2 {
			return 0, 0, fmt.Errorf("not enough data for EMA signal calculation")
		}

		currentEMA := emaValues[len(emaValues)-1]
		previousEMA := emaValues[len(emaValues)-2]
		currentPrice := candles[len(candles)-1].Close
		previousPrice := candles[len(candles)-2].Close

		// Buy signal: Price crosses above EMA
		if previousPrice <= previousEMA && currentPrice > currentEMA {
			return currentEMA, 1, nil
		}
		// Sell signal: Price crosses below EMA
		if previousPrice >= previousEMA && currentPrice < currentEMA {
			return currentEMA, -1, nil
		}

		return currentEMA, 0, nil // No crossover, hold
	}
}
