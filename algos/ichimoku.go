package algos

import (
	"fmt"

	"github.com/M1chlCZ/bingo-bot/models"
)

type IchimokuStrategy struct {
	ConversionPeriod int `json:"conversionPeriod"`
	BasePeriod       int `json:"basePeriod"`
	SpanBPeriod      int `json:"spanBPeriod"`
}

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
		fmt.Printf("Ichimoku: not enough candles: have %d, need %d\n", len(candles), i.SpanBPeriod)
		return IchimokuResult{}, fmt.Errorf("not enough candles for Ichimoku calculation")
	}

	tenkan := midpoint(candles[len(candles)-i.ConversionPeriod:])
	kijun := midpoint(candles[len(candles)-i.BasePeriod:])
	spanA := (tenkan + kijun) / 2.0

	spanB := midpoint(candles[len(candles)-i.SpanBPeriod:])
	currentPrice := candles[len(candles)-1].Close

	chikouIndex := len(candles) - 1 - i.BasePeriod
	var chikou float64
	if chikouIndex >= 0 {
		chikou = candles[chikouIndex].Close
	} else {
		chikou = currentPrice
	}

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
