package strategies

import (
	"binance_bot/models"
	"fmt"
	"log"
	"math"
)

// SpikeStrategy represents the type of strategy, all fields are required to be filled
// Very early in development and not yet proof tested, only for development purposes
// Could be scrapped in the future
type SpikeStrategy struct {
	AvgPeriod       int     // Number of candles to calculate average size
	VolumeThreshold float64 // Minimum volume to confirm spike
}

func (s *SpikeStrategy) GetStrategyType() StrategyType {
	return SpikeDetectionStrategyType
}

func (s *SpikeStrategy) Calculate(candles []models.CandleStick, pair string, trend bool) (int, error) {
	if len(candles) < s.AvgPeriod+1 {
		return 0, fmt.Errorf("not enough candles to calculate spike")
	}

	avgSize := calculateAverageCandleSize(candles, s.AvgPeriod)

	if detectSpike(candles, avgSize, s.VolumeThreshold) {
		log.Println(pair, "Strong Buy |", pair)
		return 1, nil // BUY signal
	}

	if isReversal(candles) {
		log.Println(pair, "Strong SELL |", pair)
		return -1, nil // SELL signal
	}

	return 0, nil // Hold
}

func calculateAverageCandleSize(candles []models.CandleStick, period int) float64 {
	if len(candles) < period {
		return 0
	}

	totalSize := 0.0
	for i := len(candles) - period; i < len(candles); i++ {
		totalSize += math.Abs(candles[i].High - candles[i].Low) // Use High - Low for total range
	}

	return totalSize / float64(period)
}

func detectSpike(candles []models.CandleStick, avgSize float64, volumeThreshold float64) bool {
	if len(candles) < 1 {
		return false
	}

	latestCandle := candles[len(candles)-1]
	candleSize := math.Abs(latestCandle.High - latestCandle.Low)

	return candleSize > 3*avgSize && latestCandle.Volume > volumeThreshold
}

func isReversal(candles []models.CandleStick) bool {
	if len(candles) < 2 {
		return false
	}

	latestCandle := candles[len(candles)-1]
	previousCandle := candles[len(candles)-2]

	return latestCandle.Close < latestCandle.Open && latestCandle.Close < previousCandle.Close
}
