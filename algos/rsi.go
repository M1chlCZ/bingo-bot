package algos

import (
	"fmt"
	"math"

	"github.com/M1chlCZ/bingo-bot/models"
)

type RSIStrategy struct {
	Overbought int `json:"overbought"`
	Oversold   int `json:"oversold"`
	Period     int `json:"period"`
}

func (r *RSIStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	rsiValues, err := calculateRSI(candles, r.Period)
	if err != nil {
		return 0, 0, err
	}
	latest := rsiValues[len(rsiValues)-1]

	switch {
	case latest > float64(r.Overbought):
		return latest, -1, nil
	case latest < float64(r.Oversold):
		return latest, 1, nil
	default:
		return latest, 0, nil
	}
}

func calculateRSI(candles []models.CandleStick, period int) ([]float64, error) {
	if period < 2 {
		return nil, fmt.Errorf("RSI period must be >= 2 (got %d)", period)
	}
	if len(candles) < period+1 {
		return nil, fmt.Errorf("not enough data to calculate RSI: need at least %d candles, got %d", period+1, len(candles))
	}

	avgGain := 0.0
	avgLoss := 0.0
	for i := 1; i <= period; i++ {
		change := candles[i].Close - candles[i-1].Close
		if change > 0 {
			avgGain += change
		} else {
			avgLoss -= change // change is <= 0
		}
	}
	avgGain /= float64(period)
	avgLoss /= float64(period)

	nOut := len(candles) - period
	rsi := make([]float64, nOut)

	toRSI := func(g, l float64) float64 {
		const eps = 1e-12
		switch {
		case g < eps && l < eps:
			return 50.0
		case l < eps:
			return 100.0
		}
		rs := g / l
		val := 100.0 - (100.0 / (1.0 + rs))
		if math.IsNaN(val) || math.IsInf(val, 0) {
			return 50.0
		}
		if val < 0 {
			return 0
		}
		if val > 100 {
			return 100
		}
		return val
	}

	rsi[0] = toRSI(avgGain, avgLoss)

	for i := period + 1; i < len(candles); i++ {
		change := candles[i].Close - candles[i-1].Close
		gain := 0.0
		loss := 0.0
		if change > 0 {
			gain = change
		} else {
			loss = -change
		}

		avgGain = ((avgGain * float64(period-1)) + gain) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + loss) / float64(period)

		outIdx := i - period

		if outIdx < 1 || outIdx >= nOut {
			return nil, fmt.Errorf("RSI: internal index out of range (%d of %d)", outIdx, nOut)
		}
		rsi[outIdx] = toRSI(avgGain, avgLoss)
	}

	return rsi, nil
}
