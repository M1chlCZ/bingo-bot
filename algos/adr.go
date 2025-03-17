package algos

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/models"
)

type ADRStrategy struct {
	Period     int     `json:"period"`     // Number of days to calculate ADR
	Multiplier float64 `json:"multiplier"` // Threshold multiplier (e.g., 1.0 or 1.5)
}

// Calculate computes ADR and returns latest ADR and trading signal.
func (a *ADRStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < a.Period+1 {
		return 0, 0, fmt.Errorf("not enough candles to calculate ADR, need at least %d", a.Period+1)
	}

	adr, err := calculateADR(candles, a.Period)
	if err != nil {
		return 0, 0, err
	}

	latest := candles[len(candles)-1]
	todayRange := latest.High - latest.Low

	upperThreshold := adr * a.Multiplier
	lowerThreshold := adr * (1 - a.Multiplier*0.5) // example lower threshold logic

	var signal int
	if todayRange > upperThreshold {
		signal = -1 // Sell signal, potentially overextended upwards
	} else if todayRange < lowerThreshold {
		signal = 1 // Buy signal, potentially oversold or compressed range
	} else {
		signal = 0 // Hold
	}

	return adr, signal, nil
}

// calculateADR calculates the Average Daily Range.
func calculateADR(candles []models.CandleStick, period int) (float64, error) {
	if period <= 0 {
		return 0, fmt.Errorf("ADR period must be greater than zero")
	}

	totalRange := 0.0
	count := 0

	// Start calculating from the most recent complete day backwards.
	for i := len(candles) - period - 1; i < len(candles)-1; i++ {
		if i < 0 {
			continue
		}
		dailyRange := candles[i].High - candles[i].Low
		totalRange += dailyRange
		count++
	}

	if count == 0 {
		return 0, fmt.Errorf("no valid candles for ADR calculation")
	}

	return totalRange / float64(count), nil
}
