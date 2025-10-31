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

func (m *MACDStrategy) Calculate(candles []models.CandleStick) (histogram float64, signalLine float64, macdLine float64, signal int, err error) {
	macdLine, signalLine, histogram, err = CalculateMACD(candles, m.FastPeriod, m.SlowPeriod, m.SignalPeriod)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	const eps = 0.0005

	if macdLine > signalLine*(1.0+eps) && histogram > 0 {
		return histogram, signalLine, macdLine, 1, nil // Buy
	}
	if macdLine < signalLine*(1.0-eps) && histogram < 0 {
		return histogram, signalLine, macdLine, -1, nil // Sell
	}
	return histogram, signalLine, macdLine, 0, nil // Hold
}

func CalculateMACD(candles []models.CandleStick, fastPeriod, slowPeriod, signalPeriod int) (float64, float64, float64, error) {

	if fastPeriod < 2 || slowPeriod < 2 || signalPeriod < 1 {
		return 0, 0, 0, fmt.Errorf("macd: invalid periods (fast=%d slow=%d signal=%d)", fastPeriod, slowPeriod, signalPeriod)
	}
	if fastPeriod >= slowPeriod {
		return 0, 0, 0, fmt.Errorf("macd: fastPeriod (%d) must be < slowPeriod (%d)", fastPeriod, slowPeriod)
	}

	minCandles := slowPeriod + signalPeriod - 1
	if len(candles) < minCandles {
		return 0, 0, 0, fmt.Errorf("macd: not enough data; need >= %d candles, got %d", minCandles, len(candles))
	}

	fastEMA := CalculateEMA(candles, fastPeriod)
	slowEMA := CalculateEMA(candles, slowPeriod)
	if fastEMA == nil || slowEMA == nil {
		return 0, 0, 0, fmt.Errorf("macd: failed to calculate EMAs")
	}
	if len(fastEMA) == 0 || len(slowEMA) == 0 {
		return 0, 0, 0, fmt.Errorf("macd: empty EMA output")
	}

	n := len(fastEMA)
	if len(slowEMA) < n {
		n = len(slowEMA)
	}
	fastEMA = fastEMA[len(fastEMA)-n:]
	slowEMA = slowEMA[len(slowEMA)-n:]

	macdValues := make([]float64, n)
	for i := 0; i < n; i++ {
		v := fastEMA[i] - slowEMA[i]
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0, 0, fmt.Errorf("macd: NaN/Inf in MACD series")
		}
		macdValues[i] = v
	}

	signalSeries := CalculateEMAFromValues(macdValues, signalPeriod)
	if signalSeries == nil || len(signalSeries) == 0 {
		return 0, 0, 0, fmt.Errorf("macd: failed to calculate signal line; macdLen=%d signalPeriod=%d", len(macdValues), signalPeriod)
	}

	macdLast := macdValues[len(macdValues)-1]
	signalLast := signalSeries[len(signalSeries)-1]
	histLast := macdLast - signalLast

	return macdLast, signalLast, histLast, nil
}
