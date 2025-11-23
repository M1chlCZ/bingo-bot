package strategies

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/M1chlCZ/bingo-bot/algos"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
)

type CompoundStrategy struct {
	StrategyType              StrategyType                `validate:"required" json:"strategyType"`
	RSI                       *algos.RSIStrategy          `validate:"required" json:"rsi"`
	MACD                      *algos.MACDStrategy         `validate:"required" json:"macd"`
	Stochastic                *algos.StochasticOscillator `validate:"required" json:"stochastic"`
	BollingerBands            *algos.BollingerBands       `validate:"required" json:"bollingerBands"`
	Ichimoku                  *algos.IchimokuStrategy     `validate:"required" json:"ichimoku"`
	CCI                       *algos.CCIStrategy          `validate:"required" json:"cci"`
	MFI                       *algos.MFIStrategy          `validate:"required" json:"mfi"`
	ADR                       *algos.ADRStrategy          `validate:"required" json:"adr"`
	Keltner                   *algos.KeltnerChannel       `json:"keltnerChannel"`
	ADX                       *algos.ADXStrategy          `json:"adx"`
	MarketState               models.MarketState          `validate:"marketStateEnum" json:"marketState"`
	RiskRewardThreshold       float64                     `validate:"gte=0" json:"riskRewardThreshold"`
	FeeRate                   float64                     `validate:"gte=0" json:"feeRate"`
	DesiredProfit             float64                     `validate:"gte=0" json:"desiredProfit"`
	HighestPriceFallOffMargin float64                     `validate:"gte=0" json:"highestPriceFallOffMargin"`
	CandleInterval            string                      `validate:"required" json:"candleInterval"`
	PanicSell                 bool                        `json:"panicSell"`
	SellOnBearish             bool                        `json:"sellOnBearish"`
	localIndicators           CurrentIndicators
	PendingRepo               PendingBuyRepo       `json:"-"`
	LastTrailExit             map[string]time.Time `json:"-"`
	PartialTP1Pct             float64              `json:"-"`
	PartialTP1Size            float64              `json:"-"`
}

type CurrentIndicators struct {
	RSIVal          float64
	Histogram       float64
	PrevHistogram   float64
	HistSlope       float64
	PrevRSI         float64
	RsiSlope        float64
	SignalLine      float64
	MacdLine        float64
	MacdIndicator   int
	StochasticK     float64
	StochasticD     float64
	LowerBand       float64
	MiddleBand      float64
	UpperBand       float64
	BandwidthPct    float64
	IchimokuRes     algos.IchimokuResult
	CCIVal          float64
	CCISignal       int
	MFIVal          float64
	MFiSignal       int
	ADRVal          float64
	ADRSignal       int
	CandleSticks    []models.CandleStick
	PrevMiddleBand  float64
	MiddleBandSlope float64

	EMA20      float64
	EMA50      float64
	EMA200     float64
	EMASlope20 float64
	EMASlope50 float64

	ADX     float64
	PlusDI  float64
	MinusDI float64

	KCLower float64
	KCMid   float64
	KCUpper float64
	KCSlope float64
	KCPos   float64
}

var (
	defaultPendingRepo PendingBuyRepo
	repoOnce           sync.Once
)

func getPendingRepo() PendingBuyRepo {
	repoOnce.Do(func() {
		defaultPendingRepo = NewPendingBuyRepo()
	})
	return defaultPendingRepo
}

func (cs *CompoundStrategy) ensureRepo() PendingBuyRepo {
	if cs.PendingRepo == nil {
		cs.PendingRepo = getPendingRepo()
	}
	return cs.PendingRepo
}

func (cs *CompoundStrategy) touchTrailExit(symbol string) {
	if cs.LastTrailExit == nil {
		cs.LastTrailExit = make(map[string]time.Time)
	}
	cs.LastTrailExit[symbol] = time.Now()
}

func (cs *CompoundStrategy) sinceTrailExit(symbol string) time.Duration {
	if cs.LastTrailExit == nil {
		return time.Hour * 24
	}
	if t, ok := cs.LastTrailExit[symbol]; ok {
		return time.Since(t)
	}
	return time.Hour * 24
}

func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, marketState models.MarketState, pendingCoolDown time.Duration) (int, error) {
	if len(candles) == 0 {
		return 0, nil
	}

	currentPrice := candles[len(candles)-1].Close
	logger.DebugColorf(logger.Cyan, "State: %s, Pair: %s, CurrentPrice: %.4f", marketState.String(), pair, currentPrice)

	trade, _ := db2.SQLiteDB.GetActiveTrade(pair)
	isActive, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair)
	if err != nil {
		logger.Errorf("Error checking active trade: %v", err.Error())
		isActive = false
	}

	currentIndicators, err := cs.getIndicators(candles, pair)
	if err != nil {
		logger.DebugColorf(logger.BrightRed, "Error calculating indicators: %v", err.Error())
		return 0, err
	}
	cs.localIndicators = currentIndicators

	bullishConditions := cs.checkBullishConditions(marketState, currentIndicators, currentPrice, pair)
	bearishConditions := cs.checkBearishConditions(marketState, currentIndicators, currentPrice)

	if trade != nil {
		return cs.checkActiveTrade(trade, currentPrice, bearishConditions && !bullishConditions, marketState)
	}

	logger.DebugColorf(logger.BrightBlack,
		"%s | %s | Ichi(B:%t, Br:%t), MACD=%d (hist=%.4f, slope=%.4f), RSI=%.2f (Δ=%.2f), Stoch=%.2f/%.2f, CCI=%.2f, MFI=%.2f",
		pair, marketState.String(),
		currentIndicators.IchimokuRes.Bullish, currentIndicators.IchimokuRes.Bearish,
		currentIndicators.MacdIndicator, currentIndicators.Histogram, currentIndicators.HistSlope,
		currentIndicators.RSIVal, currentIndicators.RsiSlope, currentIndicators.StochasticK, currentIndicators.StochasticD,
		currentIndicators.CCIVal, currentIndicators.MFIVal,
	)

	bought := cs.evaluatePendingBuys(pair, currentPrice, currentIndicators, cs.scalePendingCooldown(pendingCoolDown, marketState))
	if bought == 1 {
		return 1, nil
	}

	if bullishConditions {
		repo := cs.ensureRepo()

		if repo.ExistsWithCondition(pair, func(pb *PendingBuy) bool {
			if time.Since(pb.TriggerTime) > 15*time.Minute {
				return false
			}
			diff := 0.0
			if pb.TriggerPrice > 0 {
				diff = math.Abs(currentPrice-pb.TriggerPrice) / pb.TriggerPrice
			}
			return diff <= 0.003 // <= 0.3% is "same idea"
		}) {
			logger.DebugColorf(logger.Yellow, "[PENDING BUY] %s => another very recent candidate already exists", pair)
			return 0, nil
		}

		pricePos := 0.5
		den := currentIndicators.UpperBand - currentIndicators.LowerBand
		if den != 0 {
			pricePos = (currentPrice - currentIndicators.LowerBand) / den
			if pricePos < 0 {
				pricePos = 0
			} else if pricePos > 1 {
				pricePos = 1
			}
		}

		atr := getATRSafe(currentIndicators.CandleSticks, currentIndicators.ADRVal, currentIndicators.MiddleBand)

		conf := cs.entryScore(currentIndicators, currentPrice)

		priority := 3
		switch marketState {
		case models.StronglyTrending:
			priority = 5
		case models.Trending:
			priority = 4
		case models.Transitional:
			priority = 3
		case models.RangeBound:
			priority = 3
		case models.Chaotic:
			priority = 2
		default:
			priority = 3
		}
		if pricePos <= 0.33 {
			priority++
			if priority > 5 {
				priority = 5
			}
		}

		newPb := &PendingBuy{
			Pair:            pair,
			TriggerPrice:    currentPrice,
			TriggerTime:     time.Now(),
			RsiVal:          currentIndicators.RSIVal,
			MacdLine:        currentIndicators.MacdLine,
			MacdSignal:      currentIndicators.SignalLine,
			MarketState:     marketState,
			ATR:             atr,
			BollingerWidth:  den,
			StochasticK:     currentIndicators.StochasticK,
			StochasticD:     currentIndicators.StochasticD,
			CCIValue:        currentIndicators.CCIVal,
			MFIValue:        currentIndicators.MFIVal,
			TrendStrength:   math.Abs(currentIndicators.MacdLine), // crude proxy
			PricePosition:   pricePos,
			VolumeRatio:     0,
			ConfidenceScore: conf,
			Priority:        priority,
			CreatedBy:       "CompoundStrategy",
		}

		if replaced, _ := repo.AddOrReplace(newPb); replaced != nil {
			logger.InfoColorf(
				logger.BrightGreen,
				"[PENDING BUY REPLACED] %s => price=%.4f, state=%v",
				pair, currentPrice, marketState,
			)
		} else {
			logger.InfoColorf(
				logger.Blue,
				"[PENDING BUY ADDED] %s => price=%.4f, state=%v",
				pair, currentPrice, marketState,
			)
		}

		return 0, nil
	}

	if bearishConditions && isActive {
		return -1, nil
	}
	logger.DebugColorf(logger.Yellow, "[DEFAULT HOLD] %s => cci=%.1f, mfi=%.1f => no trade", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
	return 0, nil
}

func (cs *CompoundStrategy) calculateDynamicRiskReward(ci CurrentIndicators) float64 {
	cp := 0.0
	if n := len(ci.CandleSticks); n > 0 {
		cp = ci.CandleSticks[n-1].Close
	}

	if cp <= 0 {
		return cs.RiskRewardThreshold
	}

	atrAbs, err := algos.ATR(ci.CandleSticks, 14)
	var atrPct float64
	if err == nil && cp > 0 {
		atrPct = atrAbs / cp
	} else if ci.MiddleBand > 0 && cp > 0 {
		atrPct = ci.ADRVal / ci.MiddleBand
	} else {
		return cs.RiskRewardThreshold
	}

	dynRR := cs.RiskRewardThreshold
	switch {
	case atrPct > 0.06:
		dynRR *= 1.30 // very volatile → demand more reward
	case atrPct > 0.04:
		dynRR *= 1.15
	case atrPct < 0.02:
		dynRR *= 0.85 // sleepy → accept a bit less
	}
	return dynRR
}

var (
	pairCooldownMu        sync.Map
	cooldownCleanupOnce   sync.Once
	cooldownCleanupTicker *time.Ticker
)

func startCooldownCleanup() {
	cooldownCleanupOnce.Do(func() {
		cooldownCleanupTicker = time.NewTicker(30 * time.Minute)
		go func() {
			for range cooldownCleanupTicker.C {

			}
		}()
	})
}

func getATRSafe(candles []models.CandleStick, adrVal, middleBand float64) float64 {
	if atr, err := algos.ATR(candles, 14); err == nil && atr > 0 {
		return atr
	}

	if adrVal > 0 {
		return adrVal
	}

	if middleBand > 0 {
		return middleBand * 0.01
	}

	if n := len(candles); n > 0 && candles[n-1].Close > 0 {
		return candles[n-1].Close * 0.01
	}

	return 1e-9
}

func (cs *CompoundStrategy) isInCooldownPeriod(pair string) bool {
	muIface, _ := pairCooldownMu.LoadOrStore(pair, &sync.Mutex{})
	mu := muIface.(*sync.Mutex)

	mu.Lock()
	defer mu.Unlock()

	if ok, tm, _ := db2.SQLiteDB.WasLastTradeLoss(pair); ok {
		if time.Since(tm) < 20*time.Minute { // ↑ a bit longer to avoid revenge trades
			logger.InfoColorf(logger.Red,
				"[COOL-DOWN] %s: last trade red → wait (%.1f min)",
				pair, time.Since(tm).Minutes())
			return true
		}
	}
	return false
}

func atLeast(n int, flags ...bool) bool {
	cnt := 0
	for _, f := range flags {
		if f {
			cnt++
		}
	}
	return cnt >= n
}

func (cs *CompoundStrategy) entryScore(ci CurrentIndicators, currentPrice float64) float64 {
	score := 0.0

	if ci.ADRSignal > 0 {
		score += 0.35
	}
	if ci.MacdIndicator == 1 {
		score += 0.5
	}
	if ci.HistSlope > 0 {
		score += 0.20
	}
	if ci.RsiSlope > 0 {
		score += 0.10
	}
	if ci.IchimokuRes.Bullish {
		score += 0.10
	}

	if ci.MiddleBandSlope > 0 {
		score += 0.12
	}

	if currentPrice <= ci.MiddleBand*1.01 {
		score += 0.10
	}

	if ci.RSIVal > 40 && ci.RSIVal < 65 {
		score += 0.12
	}
	if ci.MFIVal > 30 && ci.MFIVal < 70 {
		score += 0.04
	}
	if ci.CCIVal > -100 && ci.CCIVal < 100 {
		score += 0.02
	}
	if ci.KCSlope > 0 {
		score += 0.06
	}

	if ci.KCUpper > ci.KCLower && ci.KCPos > 0.20 && ci.KCPos < 0.80 {
		score += 0.04
	}

	if score > 1.0 {
		return 1.0
	}
	return score
}

func (cs *CompoundStrategy) nearLowerBand(ci CurrentIndicators, currentPrice float64) bool {
	if ci.UpperBand <= ci.LowerBand {
		return false
	}
	width := ci.UpperBand - ci.LowerBand
	return currentPrice <= (ci.LowerBand + 0.15*width)
}

func (cs *CompoundStrategy) scalePendingCooldown(base time.Duration, state models.MarketState) time.Duration {
	if base <= 0 {
		return 0
	}
	switch state {
	case models.StronglyTrending:
		return time.Duration(float64(base) * 0.5)
	case models.Trending:
		return time.Duration(float64(base) * 0.7)
	case models.Transitional:
		return base
	case models.RangeBound:
		return time.Duration(float64(base) * 1.2)
	case models.Chaotic:
		return time.Duration(float64(base) * 1.4)
	default:
		return base
	}
}

func (cs *CompoundStrategy) checkBullishConditions(
	state models.MarketState,
	ci CurrentIndicators,
	currentPrice float64,
	pair string,
) bool {

	dynRR := cs.calculateDynamicRiskReward(ci)

	mfiOK := ci.MFIVal > float64(cs.MFI.Oversold) &&
		ci.MFIVal < float64(cs.MFI.Overbought)
	cciOK := ci.CCIVal < cs.CCI.Overbought &&
		ci.CCIVal > cs.CCI.Oversold
	volumeOk := mfiOK || cciOK

	emaAlignment := ci.MacdLine > ci.SignalLine

	if cs.isInCooldownPeriod(pair) {
		return false
	}

	switch state {
	case models.StronglyTrending:
		return cs.checkBullishStronglyTrending(ci, currentPrice, dynRR, volumeOk, emaAlignment, state)
	case models.Trending:
		return cs.checkBullishTrending(ci, currentPrice, dynRR, volumeOk, state)
	case models.RangeBound:
		return cs.checkBullishRangeBound(ci, currentPrice, dynRR, state)
	case models.Chaotic:
		return cs.checkBullishChaotic(ci, currentPrice, dynRR, state)
	case models.Transitional:
		return cs.checkBullishTransitional(ci, currentPrice, dynRR, emaAlignment, state)
	default:
		return cs.checkBullishConditionsDefault(state, ci, currentPrice, dynRR, volumeOk)
	}
}

func (cs *CompoundStrategy) checkBullishTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool,
	state models.MarketState,
) bool {

	var score float64 = 0.0
	requiredScore := 7.5

	mbSlopeOK := ci.MiddleBandSlope > 0.0001   // zpřísněno z -0.0001
	macdOK := ci.MacdLine > ci.SignalLine*0.99 // zpřísněno z 0.97
	trendOK := mbSlopeOK && macdOK

	adxOK := ci.ADX >= 18 && ci.PlusDI > ci.MinusDI
	if adxOK {
		score += 0.7
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓ADX [+0.7] ADX=%.1f +DI=%.1f -DI=%.1f => %.1f", ci.ADX, ci.PlusDI, ci.MinusDI, score)
	} else {
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✗ADX ADX=%.1f", ci.ADX)
	}

	emaStack := ci.EMA20 >= ci.EMA50 && ci.EMA50 >= ci.EMA200
	if emaStack {
		score += 0.8
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓EMA Stack [+0.8] => %.1f", score)
	}
	if ci.EMASlope20 > 0 && ci.EMASlope50 > 0 {
		score += 0.4
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓EMA Slopes [+0.4] => %.1f", score)
	}

	if trendOK {
		score += 2.0
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓1 Trend [+2.0] => %.1f", score)
		score += 0.8 // sníženo z 1.2
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓1 Trend [+0.8] => %.1f | partial", score)
	} else {
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✗1 Trend [+0] => %.1f", score)
		return false
	}

	priceOK := currentPrice >= ci.IchimokuRes.Kijun*0.998 // zpřísněno z 0.995
	if priceOK {
		score += 1.0
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓2 Price [+1.0] => %.1f", score)
	} else if currentPrice >= ci.IchimokuRes.Kijun*0.993 {
		score += 0.4 // sníženo z 0.6
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓2 Price [+0.4] => %.1f | close enough", score)
	} else {
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓2 Price [+0] => %.1f", score)
	}

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	logger.DebugColorf(logger.Cyan, "[TRENDING] ✓3 ATR | ATR:%.6f", atr)

	adrOK := ci.ADRSignal >= 0
	if adrOK {
		score += 0.8
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓4 ADR [+0.8] => %.1f", score)
	} else {
		score += 0.0 // odstraněno 0.2 za weak
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓4 ADR [+0.0] => %.1f | weak", score)
	}

	macdMomOK := ci.MacdIndicator == 1 && ci.HistSlope > 0.0              // zpřísněno
	rsiMomOK := ci.HistSlope > 0.0 && ci.RsiSlope > 0.0 && ci.RSIVal < 68 // zpřísněno z 72
	momentumOK := macdMomOK || rsiMomOK

	if macdMomOK && rsiMomOK {
		score += 1.5
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓5 Momentum [+1.5] => %.1f | both strong", score)
	} else if momentumOK {
		score += 0.8 // sníženo z 1.0
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓5 Momentum [+0.8] => %.1f | one ok", score)
	} else {
		score += 0.0 // odstraněno 0.3
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓5 Momentum [+0.0] => %.1f | weak", score)
	}

	notOverextended := currentPrice < ci.UpperBand+0.10*atr // zpřísněno z 0.15
	if notOverextended {
		score += 0.5
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓6 Extension [+0.5] => %.1f", score)
	}

	pullbackEntry := currentPrice <= ci.MiddleBand*1.010 && currentPrice >= ci.MiddleBand*0.975 // zpřísněno

	boAccepted := false
	var boCloseOK, boLowOK, boBullish, boVolume bool
	if n := len(ci.CandleSticks); n >= 3 {
		prevHigh := math.Max(ci.CandleSticks[n-2].High, ci.CandleSticks[n-3].High)
		boLevel := prevHigh + 0.08*atr // zpřísněno z 0.10
		last := ci.CandleSticks[n-1]

		avgVol := 0.0
		for i := n - 10; i < n-1; i++ {
			if i >= 0 {
				avgVol += ci.CandleSticks[i].Volume
			}
		}
		avgVol /= math.Min(10, float64(n-1))

		boCloseOK = last.Close >= boLevel
		boLowOK = last.Low >= prevHigh-0.08*atr                    // zpřísněno z 0.12
		boBullish = last.Close > last.Open*0.998                   // zpřísněno z 0.995
		boVolume = last.Volume > avgVol*1.0                        // zpřísněno z 0.90
		boAccepted = boCloseOK && boLowOK && boBullish && boVolume // zpřísněno: všechny podmínky

		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓7 Breakout | Close>=BO:%t, Low ok:%t, Bull:%t, Vol:%t => %t",
			boCloseOK, boLowOK, boBullish, boVolume, boAccepted)
	}

	ridingTrend := ci.MacdLine > ci.SignalLine*1.0 && ci.HistSlope > 0.0001 && ci.MiddleBandSlope > 0.0002 // zpřísněno

	entryOK := pullbackEntry || boAccepted || ridingTrend

	if pullbackEntry {
		score += 1.5
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓8 Entry [+1.5] => %.1f | pullback", score)
	} else if boAccepted {
		score += 1.5
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓8 Entry [+1.5] => %.1f | breakout", score)
	} else if ridingTrend {
		score += 1.2
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓8 Entry [+1.2] => %.1f | riding trend", score)
	} else {
		score += 0.0 // odstraněno fallback scoring
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓8 Entry [+0.0] => %.1f | weak", score)
	}

	if !entryOK {
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓8b Entry penalty | No good entry => REJECT")
		return false
	}

	if volCCIok {
		score += 0.8
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓9 Vol [+0.8] => %.1f", score)
	} else {
		score += 0.0 // odstraněno 0.2
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓9 Vol [+0.0] => %.1f | weak", score)
	}

	target := ci.UpperBand
	stop := currentPrice - 1.5*atr // zpřísněno z 1.8
	rr := cs.calcRR(state, currentPrice, stop, target)
	rrRequired := dynamicRR * 0.90 // zpřísněno z 0.85

	rsiMid := ci.RSIVal > 40 && ci.RSIVal < 68 // zpřísněno z 75
	if rr > rrRequired && rsiMid {
		score += 1.2
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓10 RR [+1.2] => %.1f | RR:%.3f, RSI:%.2f",
			score, rr, ci.RSIVal)
	} else if rr > rrRequired*0.90 && rsiMid {
		score += 0.6 // sníženo z 0.8
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓10 RR [+0.6] => %.1f | acceptable", score)
	} else {
		score += 0.0 // odstraněno 0.3
		logger.DebugColorf(logger.Cyan, "[TRENDING] ✓10 RR [+0.0] => %.1f | weak", score)
	}

	result := score >= requiredScore
	logger.DebugColorf(logger.Cyan, "[TRENDING] ✓11 FINAL | Score:%.1f vs Required:%.1f => %t",
		score, requiredScore, result)

	if result {
		entryType := "momentum"
		if pullbackEntry {
			entryType = "pullback"
		} else if boAccepted {
			entryType = "breakout"
		} else if ridingTrend {
			entryType = "riding"
		}

		logger.InfoColorf(logger.Green, "[TRENDING BUY SIGNAL ✓] Price:%.4f, Score:%.1f/%.1f, RR:%.2f, RSI:%.1f, Entry:%s",
			currentPrice, score, requiredScore, rr, ci.RSIVal, entryType)
	}
	return result
}

func (cs *CompoundStrategy) checkBullishStronglyTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool, emaUp bool,
	state models.MarketState,
) bool {

	var score float64 = 0.0
	requiredScore := 7.0

	mbOK := ci.MiddleBandSlope > 0.0002                                                                     // zpřísněno z -0.0002
	ichiOK := ci.IchimokuRes.Bullish && currentPrice > math.Max(ci.IchimokuRes.SpanA, ci.IchimokuRes.SpanB) // zpřísněno
	macdOK := ci.MacdLine > ci.SignalLine*0.99                                                              // zpřísněno z 0.97
	trendOK := mbOK && ichiOK && macdOK

	adxOK := ci.ADX >= 22 && ci.PlusDI > ci.MinusDI
	if adxOK {
		score += 1.0
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓ADX [+1.0] ADX=%.1f", ci.ADX)
	} else {
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✗ADX ADX=%.1f", ci.ADX)
	}

	emaStack := ci.EMA20 >= ci.EMA50 && ci.EMA50 >= ci.EMA200
	if emaStack {
		score += 0.9
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓EMA Stack [+0.9] => %.1f", score)
	}
	if ci.EMASlope20 > 0 && ci.EMASlope50 > 0 {
		score += 0.5
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓EMA Slopes [+0.5] => %.1f", score)
	}

	if trendOK {
		score += 2.5
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓1 Trend [+2.5] => %.1f | full", score)
		score += 1.5 // sníženo z 1.8
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓1 Trend [+1.5] => %.1f | 2/3", score)
	} else if macdOK && ichiOK {
		score += 0.8 // sníženo z 1.0
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓1 Trend [+0.8] => %.1f | minimal", score)
	} else {
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✗1 Trend [+0] => %.1f", score)
		return false
	}

	priceAboveKijun := currentPrice > ci.IchimokuRes.Kijun*0.998   // zpřísněno z 0.995
	priceAboveTenkan := currentPrice > ci.IchimokuRes.Tenkan*0.998 // zpřísněno z 0.995
	if priceAboveKijun && priceAboveTenkan {
		score += 1.5
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓2 Price [+1.5] => %.1f", score)
	} else if priceAboveKijun || priceAboveTenkan {
		score += 0.6 // sníženo z 1.0
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓2 Price [+0.6] => %.1f", score)
	} else {
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓2 Price [+0.0] => %.1f", score)
	}

	entryScore := cs.entryScore(ci, currentPrice)
	if emaUp && entryScore >= 0.68 && ci.HistSlope > 0.0 { // zpřísněno z 0.60
		score += 1.8
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓3 Score [+1.8] => %.1f", score)
	} else if emaUp && entryScore >= 0.60 { // zpřísněno z 0.52
		score += 1.0 // sníženo z 1.2
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓3 Score [+1.0] => %.1f", score)
	} else if entryScore >= 0.60 {
		score += 0.5 // sníženo z 0.8
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓3 Score [+0.5] => %.1f", score)
	}

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓4 ATR | ATR:%.6f", atr)

	pullbackTenkan := currentPrice <= ci.IchimokuRes.Tenkan*1.005 && currentPrice >= ci.IchimokuRes.Tenkan*0.980 // zpřísněno
	pullbackKijun := currentPrice <= ci.IchimokuRes.Kijun*1.005 && currentPrice >= ci.IchimokuRes.Kijun*0.980    // zpřísněno
	pullbackMB := currentPrice <= ci.MiddleBand*1.012 && currentPrice >= ci.MiddleBand*0.980                     // zpřísněno
	pullbackEntry := pullbackTenkan || pullbackKijun || pullbackMB

	boAccepted := false
	if n := len(ci.CandleSticks); n >= 3 {
		prevHigh := math.Max(ci.CandleSticks[n-2].High, ci.CandleSticks[n-3].High)
		boLevel := prevHigh + 0.10*atr // zpřísněno z 0.08
		last := ci.CandleSticks[n-1]
		avgVol := 0.0
		for i := n - 10; i < n-1; i++ {
			if i >= 0 {
				avgVol += ci.CandleSticks[i].Volume
			}
		}
		avgVol /= math.Min(10, float64(n-1))

		boCloseOK := last.Close >= boLevel
		boLowOK := last.Low >= prevHigh-0.08*atr                   // zpřísněno z 0.10
		boBullish := last.Close > last.Open*0.998                  // zpřísněno z 0.995
		boVolume := last.Volume > avgVol*1.05                      // zpřísněno z 0.95
		boAccepted = boCloseOK && boLowOK && boBullish && boVolume // všechny podmínky
	}

	strongMomentum := ci.MacdIndicator == 1 && // zpřísněno
		ci.HistSlope > 0.0001 && ci.RsiSlope > 0.0 // zpřísněno

	if (pullbackEntry || boAccepted) && strongMomentum {
		score += 1.5
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓5 Entry [+1.5] => %.1f", score)
	} else if pullbackEntry || boAccepted {
		score += 0.8 // sníženo
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓5 Entry [+0.8] => %.1f", score)
	} else {
		score += 0.0 // odstraněno 0.5
		logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓5 Entry [+0.0] => %.1f", score)
	}

	maxPrice := ci.UpperBand + 0.20*atr // zpřísněno z 0.30
	notOverextended := currentPrice <= maxPrice
	if notOverextended {
		score += 0.8
	} else {
		score += 0.0 // odstraněno fallback
	}
	logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓6 Extension [+%.1f] => %.1f",
		map[bool]float64{true: 0.8, false: 0.0}[notOverextended], score)

	stop := currentPrice - 1.5*atr                               // zpřísněno z 1.8
	target := math.Max(ci.UpperBand*1.020, currentPrice+2.5*atr) // zpřísněno z 2.2
	rr := cs.calcRR(state, currentPrice, stop, target)
	rrRequired := dynamicRR * 0.90 // zpřísněno z 0.80
	rrOK := rr > rrRequired
	if rrOK {
		score += 1.5
	} else if rr > rrRequired*0.90 {
		score += 0.7 // sníženo z 1.0
	} else {
		score += 0.0 // odstraněno 0.3
	}
	logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓7 RR [+%.1f] => %.1f | RR:%.3f",
		map[bool]float64{true: 1.5, false: map[bool]float64{true: 0.7, false: 0.0}[rr > rrRequired*0.90]}[rrOK],
		score, rr)

	rsiOK := ci.RSIVal <= 68 && ci.RSIVal >= 40 // zpřísněno z 72
	if rsiOK {
		score += 0.5
	} else {
		score += 0.0 // odstraněno fallback
	}
	logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓8 RSI [+%.1f] => %.1f",
		map[bool]float64{true: 0.5, false: 0.0}[rsiOK], score)

	if volCCIok {
		score += 0.5
	} else {
		score += 0.0 // odstraněno 0.2
	}
	logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓9 Vol [+%.1f] => %.1f",
		map[bool]float64{true: 0.5, false: 0.0}[volCCIok], score)

	result := score >= requiredScore
	logger.DebugColorf(logger.Magenta, "[ST-TREND] ✓10 FINAL | Score:%.1f vs Required:%.1f => %t",
		score, requiredScore, result)

	if result {
		logger.InfoColorf(logger.Green, "[ST-TREND BUY SIGNAL ✓] Price:%.4f, Score:%.1f/%.1f, EntryScore:%.3f, RR:%.2f",
			currentPrice, score, requiredScore, entryScore, rr)
	}
	return result
}

func (cs *CompoundStrategy) checkBullishRangeBound(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	state models.MarketState,
) bool {

	nearLower := cs.nearLowerBand(ci, currentPrice)
	rsiOversold := ci.RSIVal < 35
	cciOversold := ci.CCIVal <= cs.CCI.Oversold+10
	stochOversold := ci.StochasticK < 22
	mfiOversold := ci.MFIVal < 35

	oversoldCount := 0
	if rsiOversold {
		oversoldCount++
	}
	if cciOversold {
		oversoldCount++
	}
	if stochOversold {
		oversoldCount++
	}
	if mfiOversold {
		oversoldCount++
	}

	lowBounce := nearLower && oversoldCount >= 2
	logger.DebugColorf(logger.Yellow, "[RANGE] ✓1 LowBounce | NearLower:%t, RSI<35:%.2f:%t, CCI<=%.0f:%.2f:%t, Stoch<22:%.2f:%t, MFI<35:%.2f:%t, Count:%d/4>=2 => %t",
		nearLower, ci.RSIVal, rsiOversold, cs.CCI.Oversold+10, ci.CCIVal, cciOversold,
		ci.StochasticK, stochOversold, ci.MFIVal, mfiOversold, oversoldCount, lowBounce)

	adrConfirmation := ci.ADRSignal == 1
	logger.DebugColorf(logger.Yellow, "[RANGE] ✓2 ADR | ADRSignal:%d==1:%t", ci.ADRSignal, adrConfirmation)

	reversalPattern := false
	var hammer, engulfing bool
	if n := len(ci.CandleSticks); n >= 2 {
		last := ci.CandleSticks[n-1]
		prev := ci.CandleSticks[n-2]

		rng := last.High - last.Low
		if rng > 0 {
			body := math.Abs(last.Close - last.Open)
			lower := math.Min(last.Open, last.Close) - last.Low
			lowerFrac := lower / rng

			hammer = lowerFrac > 0.5 && body/rng < 0.3 && last.Close > last.Open
			engulfing = last.Close > last.Open && prev.Close < prev.Open &&
				last.Close > prev.Open && last.Open < prev.Close
			reversalPattern = hammer || engulfing

			logger.DebugColorf(logger.Yellow, "[RANGE] ✓3 Pattern | Hammer(lowerFrac:%.3f>0.5, bodyFrac:%.3f<0.3, bull:%t):%t, Engulf:%t => %t",
				lowerFrac, body/rng, last.Close > last.Open, hammer, engulfing, reversalPattern)
		}
	} else {
		logger.DebugColorf(logger.Yellow, "[RANGE] ✓3 Pattern | Insufficient candles => false")
	}

	conditionsOK := lowBounce && adrConfirmation && reversalPattern
	logger.DebugColorf(logger.Yellow, "[RANGE] ✓4 Conditions | LowBounce:%t & ADR:%t & Pattern:%t => %t",
		lowBounce, adrConfirmation, reversalPattern, conditionsOK)
	if !conditionsOK {
		return false
	}

	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	rrOK := rr > dynamicRR*1.1
	logger.DebugColorf(logger.Yellow, "[RANGE] ✓5 RR | Stop:%.4f, Target:%.4f, RR:%.3f > %.3f*1.1:%.3f:%t",
		stop, target, rr, dynamicRR, dynamicRR*1.1, rrOK)

	if rrOK {
		logger.InfoColorf(logger.Green, "[RANGE BUY SIGNAL ✓] Price:%.4f, RR:%.2f, Pattern:%s",
			currentPrice, rr, map[bool]string{true: "Hammer", false: "Engulfing"}[hammer])
	}
	return rrOK
}

func (cs *CompoundStrategy) checkBullishTransitional(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	emaAlignment bool,
	state models.MarketState,
) bool {
	score := cs.entryScore(ci, currentPrice)

	scoreOK := score >= 0.72
	histOK := ci.HistSlope > 0
	rsiSlopeOK := ci.RsiSlope > 0
	adrOK := ci.ADRSignal > 0
	priceOK := currentPrice >= ci.MiddleBand*0.998

	mainOK := emaAlignment && scoreOK && histOK && rsiSlopeOK && adrOK && priceOK
	logger.DebugColorf(logger.BrightCyan, "[TRANS] ✓1 Main | EMA:%t, Score:%.3f>=0.72:%t, HistSlope:%.6f>0:%t, RsiSlope:%.3f>0:%t, ADR:%d>0:%t, Price:%.4f>=MB*0.998:%.4f:%t => %t",
		emaAlignment, score, scoreOK, ci.HistSlope, histOK, ci.RsiSlope, rsiSlopeOK,
		ci.ADRSignal, adrOK, currentPrice, ci.MiddleBand*0.998, priceOK, mainOK)
	if !mainOK {
		return false
	}

	volumeOK := true
	if n := len(ci.CandleSticks); n >= 2 {
		last := ci.CandleSticks[n-1]
		avgVol := 0.0
		for i := n - 10; i < n-1; i++ {
			if i >= 0 {
				avgVol += ci.CandleSticks[i].Volume
			}
		}
		avgVol /= math.Min(10, float64(n-1))
		volumeOK = last.Volume >= avgVol*0.9
		logger.DebugColorf(logger.BrightCyan, "[TRANS] ✓2 Volume | LastVol:%.0f >= AvgVol*0.9:%.0f:%t",
			last.Volume, avgVol*0.9, volumeOK)
		if !volumeOK {
			return false
		}
	}

	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	rrOK := rr > dynamicRR*1.08
	logger.DebugColorf(logger.BrightCyan, "[TRANS] ✓3 RR | Stop:%.4f, Target:%.4f, RR:%.3f > %.3f*1.08:%.3f:%t",
		stop, target, rr, dynamicRR, dynamicRR*1.08, rrOK)

	if rrOK {
		logger.InfoColorf(logger.Green, "[TRANS BUY SIGNAL ✓] Price:%.4f, Score:%.3f, RR:%.2f",
			currentPrice, score, rr)
	}
	return rrOK
}

func (cs *CompoundStrategy) checkBullishChaotic(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	state models.MarketState,
) bool {
	score := cs.entryScore(ci, currentPrice)

	scoreOK := score >= 0.78
	histOK := ci.HistSlope > 0
	rsiSlopeOK := ci.RsiSlope > 0
	rsiInRange := ci.RSIVal < 58 && ci.RSIVal > 35

	scoreCondOK := scoreOK && histOK && rsiSlopeOK && rsiInRange
	logger.DebugColorf(logger.BrightYellow, "[CHAOTIC] ✓1 Score | Score:%.3f>=0.78:%t, HistSlope:%.6f>0:%t, RsiSlope:%.3f>0:%t, RSI:%.2f in[35,58]:%t => %t",
		score, scoreOK, ci.HistSlope, histOK, ci.RsiSlope, rsiSlopeOK,
		ci.RSIVal, rsiInRange, scoreCondOK)
	if !scoreCondOK {
		return false
	}

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	logger.DebugColorf(logger.BrightYellow, "[CHAOTIC] ✓2 ATR | ATR:%.6f", atr)

	nearMB := currentPrice <= ci.MiddleBand*1.005
	lowerQuarter := currentPrice <= ci.LowerBand+0.25*(ci.UpperBand-ci.LowerBand)
	nearSupport := nearMB || lowerQuarter
	logger.DebugColorf(logger.BrightYellow, "[CHAOTIC] ✓3 Support | Price:%.4f <= MB*1.005:%.4f:%t OR <= LB+0.25*width:%.4f:%t => %t",
		currentPrice, ci.MiddleBand*1.005, nearMB, ci.LowerBand+0.25*(ci.UpperBand-ci.LowerBand), lowerQuarter, nearSupport)
	if !nearSupport {
		return false
	}

	target := ci.UpperBand
	stop := currentPrice - 2.5*atr
	rr := cs.calcRR(state, currentPrice, stop, target)
	rrOK := rr > (dynamicRR * 1.25)
	logger.DebugColorf(logger.BrightYellow, "[CHAOTIC] ✓4 RR | Stop:%.4f, Target:%.4f, RR:%.3f > %.3f*1.25:%.3f:%t",
		stop, target, rr, dynamicRR, dynamicRR*1.25, rrOK)

	if rrOK {
		logger.InfoColorf(logger.Green, "[CHAOTIC BUY SIGNAL ✓] Price:%.4f, Score:%.3f, RR:%.2f, RSI:%.1f",
			currentPrice, score, rr, ci.RSIVal)
	}
	return rrOK
}

func (cs *CompoundStrategy) checkBullishConditionsDefault(
	state models.MarketState,
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volumeOk bool,
) bool {
	score := cs.entryScore(ci, currentPrice)

	scoreOK := score >= 0.60
	histOK := ci.HistSlope > 0
	condOK := scoreOK && histOK
	logger.DebugColorf(logger.BrightBlack, "[DEFAULT] ✓1 Conditions | Score:%.3f>=0.60:%t, HistSlope:%.6f>0:%t => %t",
		score, scoreOK, ci.HistSlope, histOK, condOK)
	if !condOK {
		return false
	}

	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	rrOK := rr > (dynamicRR * 0.95)

	logger.DebugColorf(logger.BrightBlack, "[DEFAULT] ✓2 RR | Stop:%.4f, Target:%.4f, RR:%.3f > %.3f*0.95:%.3f:%t",
		stop, target, rr, dynamicRR, dynamicRR*0.95, rrOK)
	logger.DebugColorf(logger.BrightBlack, "[DEFAULT] ✓3 Volume | VolCCI:%t", volumeOk)

	result := volumeOk && rrOK
	if result {
		logger.InfoColorf(logger.Green, "[DEFAULT BUY SIGNAL ✓] Price:%.4f, Score:%.3f, RR:%.2f",
			currentPrice, score, rr)
	}
	return result
}

func (cs *CompoundStrategy) checkBearishStronglyTrending(ci CurrentIndicators, currentPrice float64) bool {

	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > 80 && currentPrice > ci.MiddleBand
}

func (cs *CompoundStrategy) checkBearishTrending(ci CurrentIndicators) bool {

	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > float64(cs.RSI.Overbought)
}

func (cs *CompoundStrategy) checkBearishRangeBound(ci CurrentIndicators, currentPrice float64) bool {

	return currentPrice >= ci.UpperBand || ci.RSIVal > 70 || ci.CCIVal > cs.CCI.Overbought
}

func (cs *CompoundStrategy) checkBearishChaotic(ci CurrentIndicators, currentPrice float64) bool {

	return currentPrice >= ci.UpperBand && ci.RSIVal > 72 && ci.HistSlope < 0
}

func (cs *CompoundStrategy) checkBearishTransitional(ci CurrentIndicators) bool {
	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > 55 && ci.MFIVal > 70
}

func (cs *CompoundStrategy) checkBearishDefault(ci CurrentIndicators) bool {
	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > float64(cs.RSI.Overbought)
}

func (cs *CompoundStrategy) checkBearishConditions(
	state models.MarketState,
	ci CurrentIndicators,
	currentPrice float64,
) bool {
	switch state {
	case models.StronglyTrending:
		return cs.checkBearishStronglyTrending(ci, currentPrice)
	case models.Trending:
		return cs.checkBearishTrending(ci)
	case models.RangeBound:
		return cs.checkBearishRangeBound(ci, currentPrice)
	case models.Chaotic:
		return cs.checkBearishChaotic(ci, currentPrice)
	case models.Transitional:
		return cs.checkBearishTransitional(ci)
	default:
		return cs.checkBearishDefault(ci)
	}
}

func (cs *CompoundStrategy) calcRR(state models.MarketState, currentPrice, stop, target float64) float64 {
	risk := (currentPrice - stop) / currentPrice
	reward := (target - currentPrice) / currentPrice
	if risk <= 0 || reward <= 0 {
		return 0
	}
	return reward / risk
}

func (cs *CompoundStrategy) calculateRiskRewardRatioForSell(
	currentPrice, stop, target float64,
) float64 {
	if stop <= currentPrice || target >= currentPrice {
		logger.Warnf("calcRR-sell: invalid levels (stop %.4f, target %.4f, cp %.4f)",
			stop, target, currentPrice)
		return 0
	}
	risk := (stop - currentPrice) / currentPrice
	reward := (currentPrice - target) / currentPrice
	return reward / risk
}

func (cs *CompoundStrategy) GetRSI(candles []models.CandleStick, pair string) (float64, int, error) {
	return cs.RSI.Calculate(candles, pair)
}

func (cs *CompoundStrategy) GetMACD(candles []models.CandleStick) (float64, float64, float64, int, error) {
	return cs.MACD.Calculate(candles)
}

func (cs *CompoundStrategy) GetStochastic(candles []models.CandleStick) (float64, float64, error) {
	return cs.Stochastic.Calculate(candles)
}

func (cs *CompoundStrategy) GetBollingerBands(candles []models.CandleStick) (float64, float64, float64, error) {
	return cs.BollingerBands.Calculate(candles)
}

func (cs *CompoundStrategy) GetMFI(candles []models.CandleStick) (float64, int, error) {
	return cs.MFI.Calculate(candles, "")
}

func (cs *CompoundStrategy) GetCCI(candles []models.CandleStick) (float64, int, error) {
	return cs.CCI.Calculate(candles, "")
}

func (cs *CompoundStrategy) GetIchimoku(candles []models.CandleStick) (algos.IchimokuResult, error) {
	return cs.Ichimoku.Calculate(candles)
}

func (cs *CompoundStrategy) GetCandleInterval() string {
	return cs.CandleInterval
}

func (cs *CompoundStrategy) evaluatePendingBuys(
	pair string,
	currentPrice float64,
	indicators CurrentIndicators,
	pendingCoolDown time.Duration,
) int {
	repo := cs.ensureRepo()

	repo.UpdateSnapshot(pair, indicators, currentPrice)

	microBreakoutOK := func() bool {
		c := indicators.CandleSticks
		n := len(c)
		if n < 3 {
			return false
		}
		atr := getATRSafe(c, indicators.ADRVal, indicators.MiddleBand)
		prevHigh := math.Max(c[n-2].High, c[n-3].High)
		last := c[n-1]
		boLevel := prevHigh + 0.10*atr

		avgVol := 0.0
		minIdx := n - 8
		if minIdx < 0 {
			minIdx = 0
		}
		for i := minIdx; i < n-1; i++ {
			avgVol += c[i].Volume
		}
		count := float64(n - 1 - minIdx)
		if count > 0 {
			avgVol /= count
		}

		return last.Close >= boLevel &&
			last.Low >= prevHigh-0.06*atr &&
			indicators.MacdLine > indicators.SignalLine*0.99 &&
			indicators.HistSlope > -0.00001 &&
			last.Volume > avgVol*0.95
	}

	ok := func(pb *PendingBuy) bool {
		if pendingCoolDown > 0 && time.Since(pb.TriggerTime) < pendingCoolDown {
			return false
		}

		if pb.UpdateCount >= 3 {
			if !pb.ShouldBuyNow(currentPrice, indicators) {
				logger.DebugColorf(logger.Yellow,
					"[TREND CHECK] %s => Not ready yet (Quality:%.2f, Dir:%s, Updates:%d)",
					pair, pb.GetTrendQuality(), pb.TrendHistory.TrendDirection, pb.UpdateCount)
				return false
			}

			logger.InfoColorf(logger.Green,
				"[TREND CHECK ✓] %s => Trend quality passed! Proceeding with entry checks...",
				pair)
		}

		chaseThreshold := 1.020
		switch pb.MarketState {
		case models.StronglyTrending:
			chaseThreshold = 1.035
		case models.Trending:
			chaseThreshold = 1.025
		case models.RangeBound:
			chaseThreshold = 1.012
		case models.Chaotic:
			chaseThreshold = 1.015
		case models.Transitional:
			chaseThreshold = 1.015
		default:
			chaseThreshold = 1.020
		}
		if pb.TriggerPrice > 0 && currentPrice > pb.TriggerPrice*chaseThreshold {
			logger.Debugf("[PENDING] %s => price moved too far: %.4f > %.4f",
				pair, currentPrice, pb.TriggerPrice*chaseThreshold)
			return false
		}

		atr := pb.ATR
		if atr <= 0 {
			atr = getATRSafe(indicators.CandleSticks, indicators.ADRVal, indicators.MiddleBand)
		}

		minBreakout := math.Max(pb.TriggerPrice*1.0008, pb.TriggerPrice+0.08*atr)
		breakoutOK := currentPrice >= minBreakout

		accept := false
		if c := indicators.CandleSticks; len(c) >= 3 {
			prevHigh := math.Max(c[len(c)-2].High, c[len(c)-3].High)
			last := c[len(c)-1]
			acceptLevel := prevHigh + 0.08*atr
			accept = (last.Close >= acceptLevel) &&
				(last.Low >= prevHigh-0.06*atr)
		}

		macdStrong := indicators.MacdLine > indicators.SignalLine*1.01
		rsiHealthy := indicators.RSIVal > 38 && indicators.RSIVal < 70
		histAccel := indicators.HistSlope > -0.00001
		momentumOK := (macdStrong && histAccel) || (indicators.RsiSlope > 0 && rsiHealthy)

		if pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending {
			if microBreakoutOK() || momentumOK {
				breakoutOK = true
				accept = true
			}
		}

		if !((breakoutOK && accept) || momentumOK) {
			return false
		}

		maxExtension := 1.012
		if pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending {
			maxExtension = 1.018
		}

		overExtBB := indicators.UpperBand > 0 && currentPrice >= indicators.UpperBand*maxExtension
		overExtKC := indicators.KCUpper > 0 && currentPrice >= indicators.KCUpper*maxExtension

		if overExtBB || overExtKC {
			logger.Debugf("[PENDING] %s => overextended: cp=%.4f (BB>=%.4f? %t, KC>=%.4f? %t)",
				pair,
				currentPrice,
				indicators.UpperBand*maxExtension, overExtBB,
				indicators.KCUpper*maxExtension, overExtKC,
			)
			return false
		}

		if n := len(indicators.CandleSticks); n >= 2 {
			last := indicators.CandleSticks[n-1]
			avgVol := 0.0
			for i := n - 10; i < n-1; i++ {
				if i >= 0 {
					avgVol += indicators.CandleSticks[i].Volume
				}
			}
			avgVol /= math.Min(10, float64(n-1))
			if last.Volume < avgVol*0.70 {
				logger.Debugf("[PENDING] %s => low volume: %.2f < %.2f",
					pair, last.Volume, avgVol*0.70)
				return false
			}
		}

		return cs.checkBullishConditions(pb.MarketState, indicators, currentPrice, pair)
	}

	if confirmed := repo.Confirm(pair, ok); confirmed != nil {
		if !cs.finalEntrySanity(indicators) {
			logger.Warnf("[PENDING BUY ABORTED] %s => final sanity check failed", pair)
			return 0
		}

		if !cs.finalBuyValidation(indicators, currentPrice, confirmed) {
			logger.Warnf("[PENDING BUY ABORTED] %s => final buy validation failed", pair)
			return 0
		}

		logger.InfoColorf(
			logger.Blue,
			"[PENDING BUY CONFIRMED] %s => Buying at %.4f (trigger %.4f, state %s, age %.1fm, score %.2f, quality %.2f)",
			confirmed.Pair, currentPrice, confirmed.TriggerPrice, confirmed.MarketState.String(),
			time.Since(confirmed.TriggerTime).Minutes(), confirmed.ConfidenceScore, confirmed.GetTrendQuality(),
		)
		return 1
	}

	all := repo.GetAll(pair)
	removed := 0
	for _, pb := range all {
		shouldCancel := false
		reason := ""

		if pb.UpdateCount >= 5 {
			trendQuality := pb.GetTrendQuality()
			if trendQuality < 0.40 {
				shouldCancel = true
				reason = fmt.Sprintf("trend quality too low (%.2f)", trendQuality)
			} else if pb.TrendHistory != nil && pb.TrendHistory.TrendDirection == "DOWN" {
				shouldCancel = true
				reason = "trend reversed to DOWN"
			}
		}

		if !shouldCancel {
			switch pb.MarketState {
			case models.StronglyTrending:
				if currentPrice > pb.TriggerPrice*1.04 || time.Since(pb.TriggerTime) > 18*time.Minute {
					shouldCancel = true
					reason = "expired in strong trend"
				}
			case models.Trending:
				if currentPrice > pb.TriggerPrice*1.03 || time.Since(pb.TriggerTime) > 15*time.Minute {
					shouldCancel = true
					reason = "expired in trend"
				}
			case models.Chaotic:
				if currentPrice > pb.TriggerPrice*1.025 || time.Since(pb.TriggerTime) > 6*time.Minute {
					shouldCancel = true
					reason = "chaos opportunity expired"
				}
			case models.Transitional:
				if currentPrice > pb.TriggerPrice*1.025 || time.Since(pb.TriggerTime) > 12*time.Minute {
					shouldCancel = true
					reason = "transitional opportunity expired"
				}
			default:
				if currentPrice > pb.TriggerPrice*1.03 || time.Since(pb.TriggerTime) > 15*time.Minute {
					shouldCancel = true
					reason = "standard opportunity expired"
				}
			}
		}

		if !shouldCancel && !cs.checkBullishConditions(pb.MarketState, indicators, currentPrice, pair) {
			if time.Since(pb.TriggerTime) > 5*time.Minute {
				shouldCancel = true
				reason = "conditions no longer met (aged)"
			}
		}

		if shouldCancel && repo.Remove(pair, pb.ID) {
			logger.Warnf("[PENDING BUY CANCELLED] %s => %s (%.4f -> %.4f, age %.1fm)",
				pair, reason, pb.TriggerPrice, currentPrice, time.Since(pb.TriggerTime).Minutes())
			removed++
		}
	}

	if removed > 0 {
		logger.Debugf("[PENDING BUY] %s => pruned %d entries", pair, removed)
	}

	return 0
}

func (cs *CompoundStrategy) finalBuyValidation(
	indicators CurrentIndicators,
	currentPrice float64,
	pb *PendingBuy,
) bool {

	if n := len(indicators.CandleSticks); n >= 3 {
		last := indicators.CandleSticks[n-1]
		prev2High := math.Max(indicators.CandleSticks[n-2].High, indicators.CandleSticks[n-3].High)
		if currentPrice > prev2High*1.015 {
			return false
		}
		rng := last.High - last.Low
		if rng > 0 {
			upper := last.High - math.Max(last.Open, last.Close)
			upperFrac := upper / rng
			if upperFrac > 0.55 && currentPrice >= indicators.UpperBand*0.98 {
				return false
			}
		}
	}

	if indicators.MacdLine < indicators.SignalLine || indicators.HistSlope <= 0 {
		return false
	}

	overbought := 65.0
	switch pb.MarketState {
	case models.StronglyTrending, models.Trending:
		overbought = 68.0
	case models.Chaotic:
		overbought = 60.0
	}
	if indicators.RSIVal > overbought {
		return false
	}

	atr := pb.ATR
	if atr <= 0 {
		atr = getATRSafe(indicators.CandleSticks, indicators.ADRVal, indicators.MiddleBand)
	}

	stopMult := 2.0
	targetMult := 2.5
	switch pb.MarketState {
	case models.StronglyTrending:
		stopMult = 2.2
		targetMult = 2.8
	case models.Trending:
		stopMult = 2.3
		targetMult = 2.6
	case models.Transitional:
		stopMult = 2.1
		targetMult = 2.4
	case models.RangeBound:
		stopMult = 1.9
		targetMult = 2.1
	case models.Chaotic:
		stopMult = 2.5
		targetMult = 2.7
	}
	stop := currentPrice - stopMult*atr
	target := math.Max(indicators.UpperBand*1.01, currentPrice+targetMult*atr)
	rr := cs.calcRR(pb.MarketState, currentPrice, stop, target)
	minRR := cs.calculateDynamicRiskReward(indicators)

	switch pb.MarketState {
	case models.Chaotic:
		minRR *= 1.20
	case models.Transitional, models.RangeBound:
		minRR *= 1.08
	case models.Trending:
		minRR *= 0.98
	case models.StronglyTrending:
		minRR *= 0.95
	default:

	}
	if rr < minRR {
		logger.Debugf("[FINAL BUY] %s => RR too low: %.2f < %.2f", pb.Pair, rr, minRR)
		return false
	}

	minScore := 0.60
	switch pb.MarketState {
	case models.StronglyTrending:
		minScore = 0.66
	case models.Trending:
		minScore = 0.63
	case models.Chaotic:
		minScore = 0.75
	case models.Transitional:
		minScore = 0.70
	case models.RangeBound:
		minScore = 0.62
	}
	if pb.ConfidenceScore < minScore {
		logger.Debugf("[FINAL BUY] %s => score too low: %.2f < %.2f", pb.Pair, pb.ConfidenceScore, minScore)
		return false
	}

	return true
}

func (cs *CompoundStrategy) finalEntrySanity(indicators CurrentIndicators) bool {
	candles := indicators.CandleSticks
	if n := len(candles); n >= 2 {
		current := candles[n-1]
		previous := candles[n-2]

		rng := current.High - current.Low
		if rng <= 0 {
			return true
		}

		body := math.Abs(current.Close - current.Open)
		upper := current.High - math.Max(current.Open, current.Close)
		lower := math.Min(current.Open, current.Close) - current.Low
		upperFrac := upper / rng
		lowerFrac := lower / rng
		bodyFrac := body / rng

		if upperFrac >= 0.58 && bodyFrac <= 0.25 && current.Close < current.Open {
			return false
		}

		isPrevBullish := previous.Close > previous.Open
		isCurrentBearish := current.Close < current.Open
		if isPrevBullish && isCurrentBearish && bodyFrac > 0.55 && lowerFrac < 0.15 {
			prevBody := math.Abs(previous.Close - previous.Open)
			if body > prevBody*1.2 {
				return false
			}
		}

		if current.Close > current.Open && lowerFrac >= 0.25 {
			return true // Hammer/bullish with support
		}
	}
	return true
}

func (cs *CompoundStrategy) checkBuyConfirmation(
	indicators CurrentIndicators,
	currentPrice float64,
) bool {

	candles := indicators.CandleSticks

	if n := len(candles); n >= 1 {
		c := candles[n-1]
		rng := c.High - c.Low
		if rng > 0 {
			body := math.Abs(c.Close - c.Open)
			upper := c.High - math.Max(c.Open, c.Close)
			upperFrac := upper / rng
			if upperFrac >= 0.6 && body/rng <= 0.3 {
				return false
			}
			if c.Close < c.Open && body/rng > 0.6 && upperFrac < 0.1 {
				return false
			}
		}
	}

	confirmations := 0

	if indicators.MFIVal > 20 && indicators.MFIVal < 80 {
		confirmations++
	}

	if indicators.MacdLine > indicators.SignalLine && indicators.HistSlope > 0 {
		confirmations++
	}

	if currentPrice > indicators.LowerBand && currentPrice <= indicators.MiddleBand*1.02 {
		confirmations++
	}

	if n := len(indicators.CandleSticks); n >= 3 {
		l1 := indicators.CandleSticks[n-1].Low
		l2 := indicators.CandleSticks[n-2].Low
		h1 := indicators.CandleSticks[n-1].High
		h2 := indicators.CandleSticks[n-2].High
		if l1 > l2 && currentPrice > math.Max(h1, h2)*1.0005 {
			confirmations++
		}
	}

	return confirmations >= 2
}

func (cs *CompoundStrategy) alreadyInPendingBuys(pair string) bool {
	return cs.ensureRepo().Exists(pair)
}

func (cs *CompoundStrategy) trackPriceExtremes(symbol string, currentPrice float64) (athPrice, atlPrice float64, lastAthTime time.Time, err error) {

	athPrice, err = db2.SQLiteDB.GetAth(symbol)
	if err != nil || currentPrice > athPrice {
		logger.InfoColorf(logger.Green, "New HIGH price for %s: %.8f", symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAth(symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATH price for %s: %v", symbol, e)
		}
		athPrice = currentPrice
	}

	atlPrice, err = db2.SQLiteDB.GetAtl(symbol)
	if err != nil || currentPrice < atlPrice {
		logger.InfoColorf(logger.BrightRed, "New LOW price for %s: %.8f", symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAtl(symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATL price for %s: %v", symbol, e)
		}
		atlPrice = currentPrice
	}

	lastAthTime, err = db2.SQLiteDB.GetLastATHTimestamp(symbol)
	if err != nil {
		logger.Errorf("Error getting last ATH time for %s: %v", symbol, err)
	}

	return athPrice, atlPrice, lastAthTime, nil
}

func (cs *CompoundStrategy) checkEarlyExitCondition(
	trade *models.ActiveTrade,
	currentPrice float64,
	state models.MarketState,
	mstate string,
) bool {
	tradeDuration := time.Since(trade.Timestamp)
	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	profitMargin := (currentPrice - breakevenPrice) / breakevenPrice * 100

	if profitMargin >= 0.30 {
		logger.DebugColorf(
			logger.BrightYellow,
			"[EARLY EXIT SKIP] %s: PM=%.2f%% >= 0.30%% (state=%s, momo=%s) → let trend/targets manage",
			trade.Symbol, profitMargin, state.String(), mstate,
		)
		return false
	}

	maxHoldingTime := 45 * time.Minute
	switch state {
	case models.StronglyTrending:
		maxHoldingTime = 5 * time.Hour
	case models.Trending:
		maxHoldingTime = 4 * time.Hour
	case models.Transitional:
		maxHoldingTime = 90 * time.Minute
	case models.RangeBound:
		maxHoldingTime = 45 * time.Minute
	case models.Chaotic:
		maxHoldingTime = 30 * time.Minute
	default:
		maxHoldingTime = 60 * time.Minute
	}

	if tradeDuration <= maxHoldingTime {
		logger.DebugColorf(
			logger.BrightYellow,
			"[EARLY EXIT WAIT] %s: age=%v ≤ max=%v (state=%s, momo=%s, PM=%.2f%%)",
			trade.Symbol, tradeDuration, maxHoldingTime, state.String(), mstate, profitMargin,
		)
		return false
	}

	const minimalAcceptableLoss = -0.35 // -0.35% under breakeven
	if profitMargin < minimalAcceptableLoss {
		logger.DebugColorf(
			logger.BrightYellow,
			"[EARLY EXIT SKIP] %s: PM=%.2f%% < minimalAcceptableLoss=%.2f%% → let other protections handle",
			trade.Symbol, profitMargin, minimalAcceptableLoss,
		)
		return false
	}

	if state == models.StronglyTrending || state == models.Trending {
		if mstate != "DOWN" {
			logger.InfoColorf(
				logger.BrightYellow,
				"[EARLY EXIT SKIP] %s: Trend state=%s, momo=%s, age=%v, PM=%.2f%% → allow more time",
				trade.Symbol, state.String(), mstate, tradeDuration, profitMargin,
			)
			return false
		}
	}

	logger.InfoColorf(
		logger.BrightYellow,
		"[EARLY EXIT] %s: Holding too long (%v), current margin=%.2f%% (state=%s, momo=%s)",
		trade.Symbol, tradeDuration, profitMargin, state.String(), mstate,
	)
	return true
}

func (cs *CompoundStrategy) checkPanicSellCondition(profitMargin float64) bool {
	if !cs.PanicSell {
		return false
	}
	return profitMargin < -cs.HighestPriceFallOffMargin
}

func (cs *CompoundStrategy) checkAthFallOffSellCondition(profitMargin, profitMarginATH float64) bool {
	return profitMarginATH < -cs.HighestPriceFallOffMargin && profitMargin > 0
}

func (cs *CompoundStrategy) checkTimeSinceSellCondition(state models.MarketState, symbol string, profitMargin float64, lastAthTime time.Time) bool {
	if profitMargin < 0 {
		return false
	}
	return cs.getTimeSinceATHSell(symbol, lastAthTime, state) && profitMargin >= 0
}

func (cs *CompoundStrategy) checkDesiredProfitSellCondition(profitMargin float64, state models.MarketState) (bool, float64) {
	adjustedProfit := cs.DesiredProfit

	switch state {
	case models.StronglyTrending:
		adjustedProfit *= 1.8 // Let winners run more
	case models.Trending:
		adjustedProfit *= 1.4
	case models.Transitional:
		adjustedProfit *= 1.1
	case models.Chaotic:
		adjustedProfit *= 0.75
	case models.RangeBound:
		adjustedProfit *= 0.9
	}

	return profitMargin > adjustedProfit, adjustedProfit
}

func (cs *CompoundStrategy) checkBearishSignalSellCondition(profitMargin float64, bearishSignal bool, state models.MarketState) bool {
	if !cs.SellOnBearish || !bearishSignal {
		return false
	}

	minProfitForExit := -cs.HighestPriceFallOffMargin / 2

	switch state {
	case models.StronglyTrending:
		minProfitForExit = -cs.HighestPriceFallOffMargin * 0.6
	case models.Chaotic:
		minProfitForExit = -cs.HighestPriceFallOffMargin * 0.3
	case models.Transitional:
		minProfitForExit = -cs.HighestPriceFallOffMargin * 0.4
	default:
		minProfitForExit = -cs.HighestPriceFallOffMargin / 2
	}

	return profitMargin > minProfitForExit
}

func (cs *CompoundStrategy) checkActiveTrade(trade *models.ActiveTrade, currentPrice float64, bearishSignal bool, state models.MarketState) (int, error) {

	momoState := func(ci CurrentIndicators) string {
		upMACD := ci.MacdLine > ci.SignalLine
		upHist := ci.HistSlope > 0
		upRSI := ci.RsiSlope > 0
		upIchi := ci.IchimokuRes.Bullish

		downMACD := ci.MacdLine < ci.SignalLine
		downHist := ci.HistSlope < 0
		downRSI := ci.RsiSlope < 0

		upCnt := 0
		if upMACD {
			upCnt++
		}
		if upHist {
			upCnt++
		}
		if upRSI {
			upCnt++
		}
		if upIchi {
			upCnt++
		}

		downCnt := 0
		if downMACD {
			downCnt++
		}
		if downHist {
			downCnt++
		}
		if downRSI {
			downCnt++
		}

		switch {
		case upCnt >= 3 && downCnt == 0:
			return "UP"
		case downCnt >= 2:
			return "DOWN"
		default:
			return "NEUTRAL"
		}
	}

	mstate := momoState(cs.localIndicators)

	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	pmBuy := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

	athPrice, atlPrice, lastAthTime, _ := cs.trackPriceExtremes(trade.Symbol, currentPrice)
	if athPrice <= 0 {
		athPrice = currentPrice
	}
	if atlPrice <= 0 {
		atlPrice = currentPrice
	}

	pmFromATH := (currentPrice - athPrice) / athPrice * 100
	pmPeak := (athPrice - trade.BuyPrice) / trade.BuyPrice * 100

	logger.Infof("[Trade Monitor] %s | State=%s | Buy=%.4f | Curr=%.4f | PM=%.2f%% | PM_ATH=%.2f%% | MOMO=%s",
		trade.Symbol, state.String(), trade.BuyPrice, currentPrice, pmBuy, pmFromATH, mstate)

	if pmBuy < 0 {
		upliftFromAtl := (currentPrice - atlPrice) / atlPrice * 100
		logger.InfoColorf(logger.BrightYellow, "[DRAWDOWN] %s: PM=%.2f%%, UpliftFromATL=%.2f%% (ATL=%.4f)",
			trade.Symbol, pmBuy, upliftFromAtl, atlPrice)
	}

	atr := getATRSafe(cs.localIndicators.CandleSticks, cs.localIndicators.ADRVal, cs.localIndicators.MiddleBand)

	trailMult := 1.8
	switch state {
	case models.StronglyTrending:
		trailMult = 2.2
	case models.Trending:
		trailMult = 1.9
	case models.Transitional:
		trailMult = 1.6
	case models.RangeBound:
		trailMult = 1.3
	case models.Chaotic:
		trailMult = 1.1
	}

	trailingActive := pmBuy > 0 && athPrice > trade.BuyPrice && atr > 0
	trailingStop := 0.0
	if trailingActive {
		trailingStop = athPrice - trailMult*atr

		minTrailTrigger := 0.35 // %
		switch state {
		case models.StronglyTrending:
			minTrailTrigger = 0.50
		case models.Trending:
			minTrailTrigger = 0.35
		case models.Transitional:
			minTrailTrigger = 0.30
		case models.RangeBound:
			minTrailTrigger = 0.25
		case models.Chaotic:
			minTrailTrigger = 0.20
		}

		if pmBuy >= minTrailTrigger {
			minStop := breakevenPrice * 1.0012 // small buffer over BE (~0.12%)
			if trailingStop < minStop {
				trailingStop = minStop
			}
		} else {

			tslCap := currentPrice - 1.3*atr
			if trailingStop > tslCap {
				trailingStop = tslCap
			}
		}

		ageMin := time.Since(lastAthTime).Minutes()
		switch {
		case ageMin > 60:
			trailingStop = math.Max(trailingStop, currentPrice-1.1*atr)
		case ageMin > 30:
			trailingStop = math.Max(trailingStop, currentPrice-1.3*atr)
		case ageMin > 15:
			trailingStop = math.Max(trailingStop, currentPrice-1.5*atr)
		}

		isTrend := state == models.StronglyTrending || state == models.Trending
		if isTrend && pmBuy >= 1.2 {
			lock := trade.BuyPrice * 1.0040
			if trailingStop < lock {
				trailingStop = lock
			}
		}
	}

	if cs.checkPanicSellCondition(pmBuy) {
		logger.InfoColorf(logger.BrightRed, "[PANIC SELL] %s: PM=%.2f%%", trade.Symbol, pmBuy)
		return -1, nil
	}
	if cs.checkEarlyExitCondition(trade, currentPrice, state, mstate) {
		return -1, nil
	}

	if trailingActive && trailingStop > 0 && currentPrice <= trailingStop {
		logger.InfoColorf(logger.BrightRed, "[TRAIL STOP] %s: cp=%.4f <= tsl=%.4f (state=%s, PM=%.2f%%, momo=%s)",
			trade.Symbol, currentPrice, trailingStop, state.String(), pmBuy, mstate)

		if (state == models.StronglyTrending || state == models.Trending) && cs.sinceTrailExit(trade.Symbol) > 3*time.Minute {
			cs.enqueueTrendReentry(trade.Symbol, currentPrice, state)
			cs.touchTrailExit(trade.Symbol)
		}
		return -1, nil
	}

	if pmBuy > 0 && athPrice > trade.BuyPrice {
		if cs.checkAthFallOffSellCondition(pmBuy, pmFromATH) {
			isTrend := state == models.StronglyTrending || state == models.Trending
			if !isTrend || mstate != "UP" {
				logger.InfoColorf(logger.BrightRed, "[ATH FALLOFF] %s: Drop %.2f%% from ATH (momo=%s)", trade.Symbol, pmFromATH, mstate)
				return -1, nil
			}
		}
	}

	g2rArm := 0.45
	switch state {
	case models.StronglyTrending:
		g2rArm = 0.60
	case models.Trending:
		g2rArm = 0.50
	case models.Transitional:
		g2rArm = 0.45
	case models.RangeBound:
		g2rArm = 0.40
	case models.Chaotic:
		g2rArm = 0.35
	}
	beBuf := 0.0012 // ~0.12%
	if pmPeak >= g2rArm && currentPrice <= breakevenPrice*(1.0+beBuf) {
		logger.InfoColorf(logger.BrightRed, "[G2R LOCK EXIT] %s: pmPeak=%.2f%%, cp near/under BE", trade.Symbol, pmPeak)
		return -1, nil
	}

	if cs.checkTimeSinceSellCondition(state, trade.Symbol, pmBuy, lastAthTime) {
		isTrend := state == models.StronglyTrending || state == models.Trending
		if !isTrend || (cs.localIndicators.HistSlope <= 0 || cs.localIndicators.RsiSlope <= 0 || mstate != "UP") {
			return -1, nil
		}
	}

	if cs.checkBearishSignalSellCondition(pmBuy, bearishSignal, state) {
		isTrend := state == models.StronglyTrending || state == models.Trending
		if !isTrend {
			logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: State=%s, PM=%.2f%%, momo=%s",
				trade.Symbol, state.String(), pmBuy, mstate)
			return -1, nil
		}
		switch mstate {
		case "DOWN":
			logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: Trend %s, momo=DOWN, PM=%.2f%%",
				trade.Symbol, state.String(), pmBuy)
			return -1, nil
		case "NEUTRAL":
			if cs.localIndicators.MacdLine < cs.localIndicators.SignalLine && cs.localIndicators.HistSlope < 0 {
				logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: Trend %s, momo=NEUTRAL (confirm), PM=%.2f%%",
					trade.Symbol, state.String(), pmBuy)
				return -1, nil
			}
		}
	}

	if currentPrice < breakevenPrice {
		logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven, PM=%.2f%%", trade.Symbol, pmBuy)
		return 0, nil
	}

	if cs.PartialTP1Pct > 0 {
		target := cs.PartialTP1Pct
		nearUpper := cs.localIndicators.UpperBand > 0 && currentPrice >= cs.localIndicators.UpperBand*0.999
		momoCooling := cs.localIndicators.HistSlope <= 0.0 || cs.localIndicators.RsiSlope <= 0.0 ||
			cs.localIndicators.MacdLine <= cs.localIndicators.SignalLine
		if pmBuy >= target && (momoCooling || nearUpper) {
			logger.InfoColorf(logger.BrightBlack, "[SCALP EXIT] %s: PM=%.2f%% (target=%.2f%%), cooling=%t, nearUpper=%t",
				trade.Symbol, pmBuy, target, momoCooling, nearUpper)

			return -2, nil
		}
	}

	if met, adjusted := cs.checkDesiredProfitSellCondition(pmBuy, state); met {
		logger.InfoColorf(logger.BrightBlack, "[PROFIT SELL] %s: PM=%.2f%% vs target %.2f%% (momo=%s)",
			trade.Symbol, pmBuy, adjusted, mstate)
		return -2, nil
	}

	logger.InfoColorf(logger.BrightBlack, "[HOLD] %s: PM=%.2f%% | ATH age %.1fm | momo=%s",
		trade.Symbol, pmBuy, time.Since(lastAthTime).Minutes(), mstate)
	return 0, nil
}

func (cs *CompoundStrategy) enqueueTrendReentry(pair string, currentPrice float64, state models.MarketState) {
	repo := cs.ensureRepo()
	ci := cs.localIndicators

	if repo.ExistsWithCondition(pair, func(pb *PendingBuy) bool {
		if time.Since(pb.TriggerTime) < 10*time.Minute {
			return true
		}
		if pb.TriggerPrice > 0 {
			diff := math.Abs(currentPrice-pb.TriggerPrice) / pb.TriggerPrice
			return diff < 0.004 // <0.4 %
		}
		return false
	}) {
		return
	}

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)

	priceNearMB := currentPrice <= ci.MiddleBand*1.015
	momentumOK := ci.MacdLine >= ci.SignalLine || ci.HistSlope > 0
	notOverextended := currentPrice <= ci.UpperBand+0.3*atr

	if !(priceNearMB && momentumOK && notOverextended) {
		return
	}

	pb := &PendingBuy{
		Pair:           pair,
		TriggerPrice:   currentPrice,
		TriggerTime:    time.Now(),
		RsiVal:         ci.RSIVal,
		MacdLine:       ci.MacdLine,
		MacdSignal:     ci.SignalLine,
		MarketState:    state,
		ATR:            atr,
		BollingerWidth: ci.UpperBand - ci.LowerBand,
		StochasticK:    ci.StochasticK,
		StochasticD:    ci.StochasticD,
		CCIValue:       ci.CCIVal,
		MFIValue:       ci.MFIVal,
		TrendStrength:  math.Abs(ci.MacdLine),
		PricePosition: func() float64 {
			w := ci.UpperBand - ci.LowerBand
			if w <= 0 {
				return 0.5
			}
			p := (currentPrice - ci.LowerBand) / w
			if p < 0 {
				p = 0
			} else if p > 1 {
				p = 1
			}
			return p
		}(),
		VolumeRatio:     0,
		ConfidenceScore: cs.entryScore(ci, currentPrice),
		Priority:        5,
		CreatedBy:       "TrendReentry",
	}
	if replaced, _ := repo.AddOrReplace(pb); replaced != nil {
		logger.InfoColorf(logger.BrightGreen, "[RE-ENTRY REPLACED] %s => price=%.4f, state=%v", pair, currentPrice, state)
	} else {
		logger.InfoColorf(logger.BrightGreen, "[RE-ENTRY ADDED] %s => price=%.4f, state=%v", pair, currentPrice, state)
	}
}

func (cs *CompoundStrategy) getTimeSinceATHSell(symbol string, timeSinceAth time.Time, state models.MarketState) bool {
	switch state {
	case models.StronglyTrending:
		if time.Since(timeSinceAth) > 60*time.Minute {
			logger.InfoColorf(logger.BrightRed, "[TIME ATH SELL] %s: Not reached new ATH in last 60 minutes", symbol)
			return true
		}
	case models.Trending:
		if time.Since(timeSinceAth) > 45*time.Minute {
			logger.InfoColorf(logger.BrightRed, "[TIME ATH SELL] %s: Not reached new ATH in last 45 minutes", symbol)
			return true
		}
	case models.RangeBound:
		if time.Since(timeSinceAth) > 30*time.Minute {
			logger.InfoColorf(logger.BrightRed, "[TIME ATH SELL] %s: Not reached new ATH in last 30 minutes", symbol)
			return true
		}
	case models.Chaotic:
		if time.Since(timeSinceAth) > 15*time.Minute {
			logger.InfoColorf(logger.BrightRed, "[TIME ATH SELL] %s: Not reached new ATH in last 15 minutes", symbol)
			return true
		}
	case models.Transitional:
		if time.Since(timeSinceAth) > 15*time.Minute {
			logger.InfoColorf(logger.BrightRed, "[TIME ATH SELL] %s: Not reached new ATH in last 15 minutes", symbol)
			return true
		}
	case models.Default:
		if time.Since(timeSinceAth) > 20*time.Minute {
			return true
		}
	}
	return false
}

func (cs *CompoundStrategy) getIndicators(candles []models.CandleStick, pair string) (CurrentIndicators, error) {
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		return CurrentIndicators{}, fmt.Errorf("RSI: %w", err)
	}

	macdHist, sigLn, macdLn, macdInd, err1 := cs.MACD.Calculate(candles)
	if err1 != nil {
		return CurrentIndicators{}, fmt.Errorf("MACD: %w", err1)
	}

	var prevHist, prevRSI float64
	if n := len(candles); n >= 2 {
		if pv, _, _, _, e := cs.MACD.Calculate(candles[:n-1]); e == nil {
			prevHist = pv
		}
		if rv, _, e := cs.RSI.Calculate(candles[:n-1], pair); e == nil {
			prevRSI = rv
		}
	}

	stochK, stochD, err2 := cs.Stochastic.Calculate(candles)
	if err2 != nil {
		return CurrentIndicators{}, fmt.Errorf("stochastic: %w", err2)
	}

	lowB, midB, upB, err3 := cs.BollingerBands.Calculate(candles)
	if err3 != nil {
		return CurrentIndicators{}, fmt.Errorf("bollinger: %w", err3)
	}

	cciVal, cciSig, err4 := cs.CCI.Calculate(candles, pair)
	if err4 != nil {
		return CurrentIndicators{}, fmt.Errorf("CCI: %w", err4)
	}

	mfiVal, mfiSig, err5 := cs.MFI.Calculate(candles, pair)
	if err5 != nil {
		return CurrentIndicators{}, fmt.Errorf("MFI: %w", err5)
	}

	adrVal, adrSig, err6 := cs.ADR.Calculate(candles, pair)
	if err6 != nil {
		return CurrentIndicators{}, fmt.Errorf("ADR: %w", err6)
	}

	bwPct := 0.0
	den := midB
	if den == 0 {
		if n := len(candles); n > 0 {
			den = candles[n-1].Close
		}
	}
	if den != 0 {
		bwPct = (upB - lowB) / den
	}

	prevMid := 0.0
	if n := len(candles); n >= 2 {
		if _, pm, _, e := cs.BollingerBands.Calculate(candles[:n-1]); e == nil {
			prevMid = pm
		}
	}
	mbSlope := midB - prevMid

	cl := closesOf(candles)
	ema20 := emaLast(cl, 20)
	ema50 := emaLast(cl, 50)
	ema200 := emaLast(cl, 200)
	emaSlope20 := emaSlopeLast(cl, 20)
	emaSlope50 := emaSlopeLast(cl, 50)

	adx, pdi, mdi := adxLast(candles, 14)

	var kcLower, kcMid, kcUpper, kcSlope, kcPos float64
	if cs.Keltner != nil {
		if l, m, u, err := cs.Keltner.Calculate(candles); err == nil {
			kcLower, kcMid, kcUpper = l, m, u

			if n := len(candles); n >= 2 {
				if _, pm, _, errPrev := cs.Keltner.Calculate(candles[:n-1]); errPrev == nil {
					kcSlope = m - pm
				}
			}

			if u > l {
				cp := candles[len(candles)-1].Close
				kcPos = (cp - l) / (u - l)
				if kcPos < 0 {
					kcPos = 0
				} else if kcPos > 1 {
					kcPos = 1
				}
			}
		}
	}

	return CurrentIndicators{
		RSIVal:        rsiVal,
		PrevRSI:       prevRSI,
		RsiSlope:      rsiVal - prevRSI,
		Histogram:     macdHist,
		PrevHistogram: prevHist,
		HistSlope:     macdHist - prevHist,
		SignalLine:    sigLn,
		MacdLine:      macdLn,
		MacdIndicator: macdInd,
		StochasticK:   stochK,
		StochasticD:   stochD,
		LowerBand:     lowB,
		MiddleBand:    midB,
		UpperBand:     upB,
		BandwidthPct:  bwPct,
		CCIVal:        cciVal,
		CCISignal:     cciSig,
		MFIVal:        mfiVal,
		MFiSignal:     mfiSig,
		ADRVal:        adrVal,
		ADRSignal:     adrSig,
		CandleSticks:  candles,
		IchimokuRes:   func() algos.IchimokuResult { r, _ := cs.Ichimoku.Calculate(candles); return r }(),

		PrevMiddleBand:  prevMid,
		MiddleBandSlope: mbSlope,

		EMA20:      ema20,
		EMA50:      ema50,
		EMA200:     ema200,
		EMASlope20: emaSlope20,
		EMASlope50: emaSlope50,

		ADX:     adx,
		PlusDI:  pdi,
		MinusDI: mdi,

		KCLower: kcLower,
		KCMid:   kcMid,
		KCUpper: kcUpper,
		KCSlope: kcSlope,
		KCPos:   kcPos,
	}, nil
}

func (cs *CompoundStrategy) GetLatestData() CurrentIndicators {
	return cs.localIndicators
}

func (cs *CompoundStrategy) UnmarshalJSON(data []byte) error {
	type auxCompoundStrategy struct {
		StrategyType              StrategyType                `json:"strategyType"`
		RSI                       *algos.RSIStrategy          `json:"rsi"`
		MACD                      *algos.MACDStrategy         `json:"macd"`
		Stochastic                *algos.StochasticOscillator `json:"stochastic"`
		BollingerBands            *algos.BollingerBands       `json:"bollingerBands"`
		Ichimoku                  *algos.IchimokuStrategy     `json:"ichimoku"`
		CCI                       *algos.CCIStrategy          `json:"cci"`
		MFI                       *algos.MFIStrategy          `json:"mfi"`
		MarketState               models.MarketState          `json:"marketState"`
		RiskRewardThreshold       float64                     `json:"riskRewardThreshold"`
		FeeRate                   float64                     `json:"feeRate"`
		DesiredProfit             float64                     `json:"desiredProfit"`
		HighestPriceFallOffMargin float64                     `json:"highestPriceFallOffMargin"`
		CandleInterval            string                      `json:"candleInterval"`
		PanicSell                 bool                        `json:"panicSell"`
		SellOnBearish             bool                        `json:"sellOnBearish"`
	}

	var aux auxCompoundStrategy
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	cs.StrategyType = aux.StrategyType
	cs.RSI = aux.RSI
	cs.MACD = aux.MACD
	cs.Stochastic = aux.Stochastic
	cs.BollingerBands = aux.BollingerBands
	cs.Ichimoku = aux.Ichimoku
	cs.CCI = aux.CCI
	cs.MFI = aux.MFI
	cs.MarketState = aux.MarketState
	cs.RiskRewardThreshold = aux.RiskRewardThreshold
	cs.FeeRate = aux.FeeRate
	cs.DesiredProfit = aux.DesiredProfit
	cs.HighestPriceFallOffMargin = aux.HighestPriceFallOffMargin
	cs.CandleInterval = aux.CandleInterval
	cs.PanicSell = aux.PanicSell
	cs.SellOnBearish = aux.SellOnBearish

	return nil
}

func (cs *CompoundStrategy) Clone() interfaces.Strategy {
	newCS := &CompoundStrategy{
		StrategyType: cs.StrategyType,
		ADR: &algos.ADRStrategy{
			Period:     cs.ADR.Period,
			Multiplier: cs.ADR.Multiplier,
		},
		RSI: &algos.RSIStrategy{
			Overbought: cs.RSI.Overbought,
			Oversold:   cs.RSI.Oversold,
			Period:     cs.RSI.Period,
		},
		MACD: &algos.MACDStrategy{
			FastPeriod:   cs.MACD.FastPeriod,
			SlowPeriod:   cs.MACD.SlowPeriod,
			SignalPeriod: cs.MACD.SignalPeriod,
		},
		Stochastic: &algos.StochasticOscillator{
			Overbought: cs.Stochastic.Overbought,
			Oversold:   cs.Stochastic.Oversold,
			Period:     cs.Stochastic.Period,
			DPeriod:    cs.Stochastic.DPeriod,
		},
		BollingerBands: &algos.BollingerBands{
			Period: cs.BollingerBands.Period,
			Width:  cs.BollingerBands.Width,
		},
		Ichimoku: &algos.IchimokuStrategy{
			ConversionPeriod: cs.Ichimoku.ConversionPeriod,
			BasePeriod:       cs.Ichimoku.BasePeriod,
			SpanBPeriod:      cs.Ichimoku.SpanBPeriod,
		},
		CCI: &algos.CCIStrategy{
			Period:     cs.CCI.Period,
			Overbought: cs.CCI.Overbought,
			Oversold:   cs.CCI.Oversold,
		},
		MFI: &algos.MFIStrategy{
			Period:     cs.MFI.Period,
			Overbought: cs.MFI.Overbought,
			Oversold:   cs.MFI.Oversold,
		},
		Keltner:                   cs.Keltner,
		ADX:                       cs.ADX,
		MarketState:               cs.MarketState,
		RiskRewardThreshold:       cs.RiskRewardThreshold,
		FeeRate:                   cs.FeeRate,
		DesiredProfit:             cs.DesiredProfit,
		HighestPriceFallOffMargin: cs.HighestPriceFallOffMargin,
		CandleInterval:            cs.CandleInterval,
		PanicSell:                 cs.PanicSell,
		SellOnBearish:             cs.SellOnBearish,
	}
	if cs.PartialTP1Pct <= 0 {
		newCS.PartialTP1Pct = 0.9
	} else {
		newCS.PartialTP1Pct = cs.PartialTP1Pct
	}
	if cs.PartialTP1Size <= 0 || cs.PartialTP1Size > 1 {
		newCS.PartialTP1Size = 0.33
	} else {
		newCS.PartialTP1Size = cs.PartialTP1Size
	}
	err := newCS.Validate()
	if err != nil {
		log.Panic("Error validating cloned strategy: ", err)
	}
	return newCS
}

func (cs *CompoundStrategy) GetMarketState() models.MarketState {
	return cs.MarketState
}

func validMarketState(fl validator.FieldLevel) bool {
	state := fl.Field().Int() // the int value
	return state >= 0 && state <= 5
}

func (cs *CompoundStrategy) Validate() error {
	v := validator.New()
	err := v.RegisterValidation("marketStateEnum", validMarketState)
	if err != nil {
		panic(err)
	}
	return v.Struct(cs)
}

func emaLast(closes []float64, period int) float64 {
	if period <= 1 || len(closes) == 0 {
		return 0
	}
	alpha := 2.0 / (float64(period) + 1.0)
	ema := closes[0]
	for i := 1; i < len(closes); i++ {
		ema = alpha*closes[i] + (1-alpha)*ema
	}
	return ema
}

func emaSlopeLast(closes []float64, period int) float64 {
	if len(closes) < 2 {
		return 0
	}
	emaNow := emaLast(closes, period)
	emaPrev := emaLast(closes[:len(closes)-1], period)
	return emaNow - emaPrev // raw slope; sign & magnitude are enough for our scoring
}

func adxLast(c []models.CandleStick, period int) (adx, plusDI, minusDI float64) {
	n := len(c)
	if period < 2 || n < period+2 {
		return 0, 0, 0
	}

	var tr, pdm, mdm []float64
	tr = make([]float64, n)
	pdm = make([]float64, n)
	mdm = make([]float64, n)

	for i := 1; i < n; i++ {
		highDiff := c[i].High - c[i-1].High
		lowDiff := c[i-1].Low - c[i].Low

		upMove := math.Max(highDiff, 0)
		downMove := math.Max(lowDiff, 0)

		if upMove > downMove {
			pdm[i] = upMove
			mdm[i] = 0
		} else if downMove > upMove {
			pdm[i] = 0
			mdm[i] = downMove
		} else {
			pdm[i], mdm[i] = 0, 0
		}

		hiLo := c[i].High - c[i].Low
		hiPc := math.Abs(c[i].High - c[i-1].Close)
		loPc := math.Abs(c[i].Low - c[i-1].Close)
		tr[i] = math.Max(hiLo, math.Max(hiPc, loPc))
	}

	sumTR, sumPDM, sumMDM := 0.0, 0.0, 0.0
	for i := 1; i <= period; i++ {
		sumTR += tr[i]
		sumPDM += pdm[i]
		sumMDM += mdm[i]
	}

	smTR := sumTR
	smPDM := sumPDM
	smMDM := sumMDM

	var dx float64
	for i := period + 1; i < n; i++ {
		smTR = smTR - (smTR / float64(period)) + tr[i]
		smPDM = smPDM - (smPDM / float64(period)) + pdm[i]
		smMDM = smMDM - (smMDM / float64(period)) + mdm[i]

		if smTR <= 0 {
			continue
		}
		plusDI = 100.0 * (smPDM / smTR)
		minusDI = 100.0 * (smMDM / smTR)
		den := plusDI + minusDI
		if den == 0 {
			continue
		}
		dx = 100.0 * math.Abs((plusDI-minusDI)/den)
	}

	adx = dx
	return adx, plusDI, minusDI
}

func closesOf(c []models.CandleStick) []float64 {
	out := make([]float64, 0, len(c))
	for _, k := range c {
		out = append(out, k.Close)
	}
	return out
}
