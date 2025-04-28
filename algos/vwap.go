package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"time"
)

// VWAPStrategy represents a trading strategy based on the Volume Weighted Average Price (VWAP) indicator.
// VWAP gives the average price a security has traded at throughout the period, weighted by volume.
type VWAPStrategy struct {
	// Deviation is the percentage above/below VWAP to generate signals
	Deviation float64 `json:"deviation"` // e.g., 0.02 for 2%

	// ResetPeriod determines when to reset the VWAP calculation
	// Common values: "day", "week", "none"
	ResetPeriod string `json:"resetPeriod"`
}

// Calculate computes the VWAP value and returns a signal based on price deviation from VWAP.
// Returns:
// - float64: The current VWAP value
// - int: Signal (-1 for sell, 0 for hold, 1 for buy)
// - error: Any error encountered during calculation
func (v *VWAPStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < 2 {
		return 0, 0, fmt.Errorf("not enough data to calculate VWAP: need at least 2 candles")
	}

	// Calculate VWAP
	vwapValues, err := calculateVWAP(candles, v.ResetPeriod)
	if err != nil {
		return 0, 0, err
	}

	if len(vwapValues) == 0 {
		return 0, 0, fmt.Errorf("no VWAP values calculated")
	}

	currentVWAP := vwapValues[len(vwapValues)-1]
	currentPrice := candles[len(candles)-1].Close

	// Calculate the deviation percentage
	deviationPercent := (currentPrice - currentVWAP) / currentVWAP

	// Generate signals based on deviation from VWAP
	if deviationPercent > v.Deviation {
		// Price is significantly above VWAP - potential sell signal
		return currentVWAP, -1, nil
	} else if deviationPercent < -v.Deviation {
		// Price is significantly below VWAP - potential buy signal
		return currentVWAP, 1, nil
	}

	// Price is within normal range of VWAP - hold
	return currentVWAP, 0, nil
}

// calculateVWAP computes the Volume Weighted Average Price for the given candles.
// VWAP = ∑(Price * Volume) / ∑(Volume)
// where Price is typically the average of high, low, and close prices.
func calculateVWAP(candles []models.CandleStick, resetPeriod string) ([]float64, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("no candles provided for VWAP calculation")
	}

	// Initialize result array
	vwap := make([]float64, len(candles))

	// Initialize cumulative values
	cumulativePV := 0.0 // Price * Volume
	cumulativeVolume := 0.0

	// Track the current period for resets
	var currentPeriod time.Time
	if resetPeriod != "none" {
		currentPeriod = getPeriodStart(candles[0].Timestamp, resetPeriod)
	}

	// Calculate VWAP for each candle
	for i, candle := range candles {
		// Check if we need to reset the calculation for a new period
		if resetPeriod != "none" {
			periodStart := getPeriodStart(candle.Timestamp, resetPeriod)
			if !periodStart.Equal(currentPeriod) {
				// Reset for new period
				cumulativePV = 0.0
				cumulativeVolume = 0.0
				currentPeriod = periodStart
			}
		}

		// Calculate typical price: (High + Low + Close) / 3
		typicalPrice := (candle.High + candle.Low + candle.Close) / 3.0

		// Update cumulative values
		pv := typicalPrice * candle.Volume
		cumulativePV += pv
		cumulativeVolume += candle.Volume

		// Calculate VWAP
		if cumulativeVolume > 0 {
			vwap[i] = cumulativePV / cumulativeVolume
		} else {
			// If no volume, use typical price as fallback
			vwap[i] = typicalPrice
		}
	}

	return vwap, nil
}

// getPeriodStart returns the start time of the period containing the given timestamp.
func getPeriodStart(timestamp time.Time, period string) time.Time {
	switch period {
	case "day":
		// Start of day in local timezone
		return time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
	case "week":
		// Start of week (assuming week starts on Monday)
		daysToMonday := int(timestamp.Weekday() - time.Monday)
		if daysToMonday < 0 {
			daysToMonday += 7
		}
		mondayDate := timestamp.AddDate(0, 0, -daysToMonday)
		return time.Date(mondayDate.Year(), mondayDate.Month(), mondayDate.Day(), 0, 0, 0, 0, timestamp.Location())
	case "month":
		// Start of month
		return time.Date(timestamp.Year(), timestamp.Month(), 1, 0, 0, 0, 0, timestamp.Location())
	default:
		// No reset, return zero time
		return time.Time{}
	}
}
