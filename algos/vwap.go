package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
	"time"
)

type VWAPStrategy struct {
	Deviation float64 `json:"deviation"` // e.g., 0.02 for 2%

	ResetPeriod string `json:"resetPeriod"`
}

func (v *VWAPStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < 2 {
		return 0, 0, fmt.Errorf("not enough data to calculate VWAP: need at least 2 candles")
	}

	vwapValues, err := calculateVWAP(candles, v.ResetPeriod)
	if err != nil {
		return 0, 0, err
	}

	if len(vwapValues) == 0 {
		return 0, 0, fmt.Errorf("no VWAP values calculated")
	}

	currentVWAP := vwapValues[len(vwapValues)-1]
	currentPrice := candles[len(candles)-1].Close

	deviationPercent := (currentPrice - currentVWAP) / currentVWAP

	if deviationPercent > v.Deviation {

		return currentVWAP, -1, nil
	} else if deviationPercent < -v.Deviation {

		return currentVWAP, 1, nil
	}

	return currentVWAP, 0, nil
}

func calculateVWAP(candles []models.CandleStick, resetPeriod string) ([]float64, error) {
	if len(candles) == 0 {
		return nil, fmt.Errorf("no candles provided for VWAP calculation")
	}

	vwap := make([]float64, len(candles))

	cumulativePV := 0.0 // Price * Volume
	cumulativeVolume := 0.0

	var currentPeriod time.Time
	if resetPeriod != "none" {
		currentPeriod = getPeriodStart(candles[0].Timestamp, resetPeriod)
	}

	for i, candle := range candles {

		if resetPeriod != "none" {
			periodStart := getPeriodStart(candle.Timestamp, resetPeriod)
			if !periodStart.Equal(currentPeriod) {

				cumulativePV = 0.0
				cumulativeVolume = 0.0
				currentPeriod = periodStart
			}
		}

		typicalPrice := (candle.High + candle.Low + candle.Close) / 3.0

		pv := typicalPrice * candle.Volume
		cumulativePV += pv
		cumulativeVolume += candle.Volume

		if cumulativeVolume > 0 {
			vwap[i] = cumulativePV / cumulativeVolume
		} else {

			vwap[i] = typicalPrice
		}
	}

	return vwap, nil
}

func getPeriodStart(timestamp time.Time, period string) time.Time {
	switch period {
	case "day":

		return time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
	case "week":

		daysToMonday := int(timestamp.Weekday() - time.Monday)
		if daysToMonday < 0 {
			daysToMonday += 7
		}
		mondayDate := timestamp.AddDate(0, 0, -daysToMonday)
		return time.Date(mondayDate.Year(), mondayDate.Month(), mondayDate.Day(), 0, 0, 0, 0, timestamp.Location())
	case "month":

		return time.Date(timestamp.Year(), timestamp.Month(), 1, 0, 0, 0, 0, timestamp.Location())
	default:

		return time.Time{}
	}
}
