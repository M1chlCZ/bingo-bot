package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
)

// SMAStrategy represents a trading strategy based on the Simple Moving Average (SMA) indicator.
// SMA calculates the average price over a specified period, giving equal weight to each price point.
type SMAStrategy struct {
	Period       int  `json:"period"`       // The number of periods to use for SMA calculation
	FastPeriod   int  `json:"fastPeriod"`   // Period for the fast SMA in a crossover strategy
	SlowPeriod   int  `json:"slowPeriod"`   // Period for the slow SMA in a crossover strategy
	UseCrossover bool `json:"useCrossover"` // Whether to use SMA crossover for signals
}

// Calculate computes the SMA value and returns a signal based on the strategy configuration.
// If UseCrossover is true, it generates signals based on fast SMA crossing slow SMA.
// Otherwise, it generates signals based on price crossing the SMA.
// Returns:
// - float64: The current SMA value (or fast SMA if using crossover)
// - int: Signal (-1 for sell, 0 for hold, 1 for buy)
// - error: Any error encountered during calculation
func (s *SMAStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if s.UseCrossover {
		// Use SMA crossover strategy
		if s.FastPeriod <= 0 || s.SlowPeriod <= 0 || s.FastPeriod >= s.SlowPeriod {
			return 0, 0, fmt.Errorf("invalid SMA periods: fast=%d, slow=%d", s.FastPeriod, s.SlowPeriod)
		}

		// Calculate fast SMA
		fastSMA, err := calculateSMA(candles, s.FastPeriod)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to calculate fast SMA: %w", err)
		}

		// Calculate slow SMA
		slowSMA, err := calculateSMA(candles, s.SlowPeriod)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to calculate slow SMA: %w", err)
		}

		if len(fastSMA) < 2 || len(slowSMA) < 2 {
			return 0, 0, fmt.Errorf("not enough SMA values calculated")
		}

		// Get current and previous values
		currentFastSMA := fastSMA[len(fastSMA)-1]
		previousFastSMA := fastSMA[len(fastSMA)-2]
		currentSlowSMA := slowSMA[len(slowSMA)-1]
		previousSlowSMA := slowSMA[len(slowSMA)-2]

		// Check for crossover
		// Buy signal: Fast SMA crosses above Slow SMA
		if previousFastSMA <= previousSlowSMA && currentFastSMA > currentSlowSMA {
			return currentFastSMA, 1, nil
		}
		// Sell signal: Fast SMA crosses below Slow SMA
		if previousFastSMA >= previousSlowSMA && currentFastSMA < currentSlowSMA {
			return currentFastSMA, -1, nil
		}

		return currentFastSMA, 0, nil // No crossover, hold
	} else {
		// Use price crossing SMA strategy
		if s.Period <= 0 {
			return 0, 0, fmt.Errorf("SMA period must be greater than zero")
		}

		smaValues, err := calculateSMA(candles, s.Period)
		if err != nil {
			return 0, 0, err
		}

		if len(smaValues) < 2 || len(candles) < 2 {
			return 0, 0, fmt.Errorf("not enough data for SMA signal calculation")
		}

		currentSMA := smaValues[len(smaValues)-1]
		previousSMA := smaValues[len(smaValues)-2]
		currentPrice := candles[len(candles)-1].Close
		previousPrice := candles[len(candles)-2].Close

		// Buy signal: Price crosses above SMA
		if previousPrice <= previousSMA && currentPrice > currentSMA {
			return currentSMA, 1, nil
		}
		// Sell signal: Price crosses below SMA
		if previousPrice >= previousSMA && currentPrice < currentSMA {
			return currentSMA, -1, nil
		}

		return currentSMA, 0, nil // No crossover, hold
	}
}

// calculateSMA computes the Simple Moving Average for the given candles and period.
// SMA is calculated as the sum of closing prices over the period divided by the period.
func calculateSMA(candles []models.CandleStick, period int) ([]float64, error) {
	if period <= 0 {
		return nil, fmt.Errorf("SMA period must be greater than zero")
	}

	if len(candles) < period {
		return nil, fmt.Errorf("not enough data to calculate SMA: need at least %d candles, got %d", period, len(candles))
	}

	// Calculate SMA for each window of 'period' candles
	smaCount := len(candles) - period + 1
	sma := make([]float64, smaCount)

	// Calculate first SMA
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	sma[0] = sum / float64(period)

	// Calculate subsequent SMAs using a sliding window
	// This is more efficient than recalculating the sum for each window
	for i := 1; i < smaCount; i++ {
		// Remove the oldest price and add the newest price
		sum = sum - candles[i-1].Close + candles[i+period-1].Close
		sma[i] = sum / float64(period)
	}

	return sma, nil
}
