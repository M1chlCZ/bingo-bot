package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"math"
)

type RSIStrategy struct {
	Overbought int // RSI threshold for overbought (sell signal)
	Oversold   int // RSI threshold for oversold (buy signal)
	Period     int // Lookback period for RSI
}

// Calculate returns the latest RSI and a signal based on overbought/oversold levels
func (r *RSIStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	rsiValues, err := calculateRSI(candles, r.Period)
	if err != nil {
		return 0, 0, err
	}

	latestRSI := rsiValues[len(rsiValues)-1]

	if latestRSI > float64(r.Overbought) {
		return latestRSI, -1, nil // Sell signal
	} else if latestRSI < float64(r.Oversold) {
		return latestRSI, 1, nil // Buy signal
	}
	return latestRSI, 0, nil // Hold
}

// calculateRSI computes the RSI for each candle starting from the first point
// where we can calculate period averages. It returns an array of RSI values
// aligned so that rsi[0] corresponds to the first candle where RSI is available.
func calculateRSI(candles []models.CandleStick, period int) ([]float64, error) {
	if period <= 0 {
		return nil, fmt.Errorf("RSI period must be greater than zero")
	}
	if len(candles) < period+1 {
		return nil, fmt.Errorf("not enough data to calculate RSI: need at least %d candles, got %d", period+1, len(candles))
	}

	// Calculate gains and losses for each period
	gains := make([]float64, len(candles)-1)
	losses := make([]float64, len(candles)-1)

	for i := 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			gains[i-1] = change
			losses[i-1] = 0
		} else {
			gains[i-1] = 0
			losses[i-1] = -change
		}
	}

	// Calculate initial average gain and loss over the first 'period'
	avgGain := 0.0
	avgLoss := 0.0
	if len(gains) < period {
		return nil, fmt.Errorf("not enough data to form the initial averages")
	}

	for i := 0; i < period; i++ {
		avgGain += gains[i]
		avgLoss += losses[i]
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	// RSI array starts after 'period' candles of data have been processed
	rsiCount := len(candles) - period
	rsi := make([]float64, rsiCount)

	// Calculate RSI for the first available point
	rs := 0.0
	if avgLoss == 0 {
		// If avgLoss is 0, no down moves in this period. RSI = 100.
		// This can happen in strongly bullish conditions.
		rs = math.Inf(1)
	} else {
		rs = avgGain / avgLoss
	}
	rsi[0] = 100 - (100 / (1 + rs))

	// For each subsequent candle, update avgGain and avgLoss with Wilder’s smoothing and calculate RSI
	for i := period; i < len(candles)-1; i++ {
		// Update avgGain and avgLoss (Wilder's smoothing)
		avgGain = ((avgGain * float64(period-1)) + gains[i]) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + losses[i]) / float64(period)

		if avgLoss == 0 {
			rs = math.Inf(1) // All gains, no losses
		} else {
			rs = avgGain / avgLoss
		}

		rsiIndex := i - (period - 1)
		if rsiIndex < 0 || rsiIndex >= len(rsi) {
			// Sanity check
			return nil, fmt.Errorf("index out of range while calculating RSI")
		}
		rsi[rsiIndex] = 100 - (100 / (1 + rs))
	}

	return rsi, nil
}
