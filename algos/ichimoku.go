package algos

import (
	"binance_bot/models"
	"fmt"
)

type IchimokuStrategy struct {
	ConversionPeriod int
	BasePeriod       int
	SpanBPeriod      int
}

// IchimokuResult holds computed values for interpretation
type IchimokuResult struct {
	Tenkan       float64
	Kijun        float64
	SpanA        float64
	SpanB        float64
	Chikou       float64
	CurrentPrice float64
	Bullish      bool
	Bearish      bool
}

func (i *IchimokuStrategy) Calculate(candles []models.CandleStick) (IchimokuResult, error) {
	if len(candles) < i.SpanBPeriod {
		return IchimokuResult{}, fmt.Errorf("not enough candles for Ichimoku calculation")
	}

	tenkan := midpoint(candles[len(candles)-i.ConversionPeriod:])
	kijun := midpoint(candles[len(candles)-i.BasePeriod:])
	spanA := (tenkan + kijun) / 2.0

	// SpanB is midpoint of last 52 periods (standard setting)
	spanB := midpoint(candles[len(candles)-i.SpanBPeriod:])
	currentPrice := candles[len(candles)-1].Close

	// Chikou Span (lagging line) is usually price shifted back 26 periods
	// Check we have data:
	chikouIndex := len(candles) - 1 - i.BasePeriod
	var chikou float64
	if chikouIndex >= 0 {
		chikou = candles[chikouIndex].Close
	} else {
		chikou = currentPrice // fallback if not enough data
	}

	// Interpret the signals:
	// Price above cloud (above both SpanA and SpanB) and Tenkan > Kijun => Bullish
	// Price below cloud and Tenkan < Kijun => Bearish
	// Else, neutral
	bullish := currentPrice > spanA && currentPrice > spanB && tenkan > kijun
	bearish := currentPrice < spanA && currentPrice < spanB && tenkan < kijun

	return IchimokuResult{
		Tenkan:       tenkan,
		Kijun:        kijun,
		SpanA:        spanA,
		SpanB:        spanB,
		Chikou:       chikou,
		CurrentPrice: currentPrice,
		Bullish:      bullish,
		Bearish:      bearish,
	}, nil
}

// midpoint finds (HighestHigh + LowestLow)/2 for a given slice of candles
func midpoint(c []models.CandleStick) float64 {
	high := c[0].High
	low := c[0].Low
	for _, candle := range c {
		if candle.High > high {
			high = candle.High
		}
		if candle.Low < low {
			low = candle.Low
		}
	}
	return (high + low) / 2.0
}
