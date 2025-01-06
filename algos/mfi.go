package algos

import (
	"binance_bot/models"
	"fmt"
)

type MFIStrategy struct {
	Overbought int
	Oversold   int
	Period     int
}

// Calculate returns (latestMFI, signal, error)
func (m *MFIStrategy) Calculate(candles []models.CandleStick, _ string) (float64, int, error) {
	if len(candles) < m.Period+1 {
		return 0, 0, fmt.Errorf("not enough data for MFI: need %d, got %d", m.Period+1, len(candles))
	}

	// MoneyFlow arrays
	posFlow := make([]float64, len(candles)-1)
	negFlow := make([]float64, len(candles)-1)

	for i := 1; i < len(candles); i++ {
		typicalPrev := (candles[i-1].High + candles[i-1].Low + candles[i-1].Close) / 3.0
		typicalCurr := (candles[i].High + candles[i].Low + candles[i].Close) / 3.0
		moneyFlow := typicalCurr * candles[i].Volume

		if typicalCurr > typicalPrev {
			posFlow[i-1] = moneyFlow
			negFlow[i-1] = 0
		} else {
			posFlow[i-1] = 0
			negFlow[i-1] = moneyFlow
		}
	}

	// initial sums
	avgPosFlow := 0.0
	avgNegFlow := 0.0

	for i := 0; i < m.Period; i++ {
		avgPosFlow += posFlow[i]
		avgNegFlow += negFlow[i]
	}

	// MFI array
	mfiCount := len(candles) - m.Period
	mfiValues := make([]float64, mfiCount)

	// first MFI
	mfiValues[0] = calcMFI(avgPosFlow, avgNegFlow)

	// subsequent MFI with rolling sums
	for i := m.Period; i < len(posFlow); i++ {
		avgPosFlow = avgPosFlow - posFlow[i-m.Period] + posFlow[i]
		avgNegFlow = avgNegFlow - negFlow[i-m.Period] + negFlow[i]

		idx := i - (m.Period - 1)
		if idx < 0 || idx >= len(mfiValues) {
			return 0, 0, fmt.Errorf("index error in MFI")
		}
		mfiValues[idx] = calcMFI(avgPosFlow, avgNegFlow)
	}

	latestMFI := mfiValues[len(mfiValues)-1]
	var signal int
	if latestMFI > float64(m.Overbought) {
		signal = -1 // SELL
	} else if latestMFI < float64(m.Oversold) {
		signal = 1 // BUY
	}

	return latestMFI, signal, nil
}

func calcMFI(posFlow, negFlow float64) float64 {
	if negFlow == 0 {
		return 100
	}
	moneyFlowRatio := posFlow / negFlow
	return 100 - (100 / (1 + moneyFlowRatio))
}
