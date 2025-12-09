package strategies

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/analysis"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
)

type CompoundStrategy struct {
	StrategyType              StrategyType                `validate:"required" json:"strategyType"`
	Analyzer                  *analysis.MarketAnalyzer    `json:"analyzer"`
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
	EquityGuard               *EquityGuard         `json:"-"`
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

	MultiTFTrendScore float64
	MarketRegime      models.MarketRegime
	VolatilityRegime  models.VolatilityRegime
	MarketStateHTF    models.MarketState

	PriceActionQuality float64
	MomentumQuality    float64
	NoiseLevel         float64

	VolumeProfile       models.VolumeProfile
	HasBullishStructure bool
	HasBearishDiv       bool

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

	defaultEquityGuard *EquityGuard
	equityOnce         sync.Once
)

const (
	bullishRelaxMin = 0.95
	bullishRelaxMax = 1.05
)

func relaxMin(v float64) float64 { return v * bullishRelaxMin }
func relaxMax(v float64) float64 { return v * bullishRelaxMax }

func getPendingRepo() PendingBuyRepo {
	repoOnce.Do(func() {
		defaultPendingRepo = NewPendingBuyRepo()
	})
	return defaultPendingRepo
}

func getEquityGuard() *EquityGuard {
	equityOnce.Do(func() {
		defaultEquityGuard = NewEquityGuardDefault()
	})
	return defaultEquityGuard
}

func (cs *CompoundStrategy) ensureRepo() PendingBuyRepo {
	if cs.PendingRepo == nil {
		cs.PendingRepo = getPendingRepo()
	}
	return cs.PendingRepo
}

func (cs *CompoundStrategy) ensureEquityGuard() *EquityGuard {
	if cs.EquityGuard == nil {
		cs.EquityGuard = getEquityGuard()
	}
	return cs.EquityGuard
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

	repo := cs.ensureRepo()
	repo.RecordMarketTrend(pair, marketState, currentIndicators, currentPrice)
	bias := repo.GetTrendBias(pair)

	eq := cs.ensureEquityGuard()
	blocked, blockReason := eq.ShouldBlockNewEntry(pair)

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

	cs.logCompactState(pair, marketState, currentPrice, currentIndicators, bias, len(repo.GetAll(pair)), trade, bullishConditions, bearishConditions)

	if blocked {
		logger.InfoColorf(
			logger.Red,
			"[EQUITY-GUARD BLOCK] %s => no new longs (reason: %s)",
			pair, blockReason,
		)
		return 0, nil
	}

	bought := cs.evaluatePendingBuys(pair, currentPrice, currentIndicators, cs.scalePendingCooldown(pendingCoolDown, marketState), marketState)
	if bought == 1 {
		return 1, nil
	}

	if bullishConditions {
		if repo.ExistsWithCondition(pair, func(pb *PendingBuy) bool {
			if time.Since(pb.TriggerTime) > 15*time.Minute {
				return false
			}
			diff := 0.0
			if pb.TriggerPrice > 0 {
				diff = math.Abs(currentPrice-pb.TriggerPrice) / pb.TriggerPrice
			}
			return diff <= 0.003
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
		if bias.Direction == "UP" {
			conf = clamp01(conf + bias.Strength*0.20)
		}

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
		if bias.Direction == "UP" && bias.Strength > 0.60 {
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
			TrendStrength:   math.Abs(currentIndicators.MacdLine),
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
		dynRR *= 1.30
	case atrPct > 0.04:
		dynRR *= 1.15
	case atrPct < 0.02:
		dynRR *= 0.85
	}

	logger.DebugColorf(
		logger.BrightBlack,
		"[DYN RR] cp=%.6f atrPct=%.4f baseRR=%.3f dynRR=%.3f",
		cp, atrPct, cs.RiskRewardThreshold, dynRR,
	)

	return dynRR
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
	if ok, tm, _ := db2.SQLiteDB.WasLastTradeLoss(pair); ok {
		if time.Since(tm) < 20*time.Minute {
			logger.InfoColorf(logger.Red,
				"[COOL-DOWN] %s: last trade red → wait (%.1f min)",
				pair, time.Since(tm).Minutes())
			return true
		}
	}
	return false
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
		logger.Debugf("[BULLISH REJECT] %s state=%s reason=cooldown", pair, state.String())
		return false
	}

	if skip, reason := cs.shouldSkipLongDueToChop(state, ci, pair); skip {
		logger.DebugColorf(
			logger.BrightBlack,
			"[BULLISH REJECT] %s state=%s reason=chop-filter: %s",
			pair, state.String(), reason,
		)
		return false
	}

	if ok, reason := cs.passesHTFBullishGate(state, ci, pair); !ok {
		logger.DebugColorf(
			logger.BrightBlack,
			"[BULLISH REJECT] %s state=%s reason=HTF-gate: %s (HTF=%s, regime=%s, mTf=%.2f)",
			pair, state.String(), reason,
			ci.MarketStateHTF.String(), ci.MarketRegime.String(), ci.MultiTFTrendScore,
		)
		return false
	}

	switch state {
	case models.StronglyTrending:
		res := cs.checkBullishStronglyTrending(ci, currentPrice, dynRR, volumeOk, emaAlignment, state)
		logger.Debugf("[BULLISH RESULT] %s state=StronglyTrending res=%t rsi=%.1f hist=%.6f adx=%.1f rr=%.3f",
			pair, res, ci.RSIVal, ci.HistSlope, ci.ADX, dynRR)
		return res
	case models.Trending:
		res := cs.checkBullishTrending(ci, currentPrice, dynRR, volumeOk, state)
		logger.Debugf("[BULLISH RESULT] %s state=Trending res=%t rsi=%.1f hist=%.6f adx=%.1f rr=%.3f",
			pair, res, ci.RSIVal, ci.HistSlope, ci.ADX, dynRR)
		return res
	case models.RangeBound:
		res := cs.checkBullishRangeBound(ci, currentPrice, dynRR, state)
		logger.Debugf("[BULLISH RESULT] %s state=RangeBound res=%t rsi=%.1f cci=%.1f rr=%.3f",
			pair, res, ci.RSIVal, ci.CCIVal, dynRR)
		return res
	case models.Chaotic:
		res := cs.checkBullishChaotic(ci, currentPrice, dynRR, state)
		logger.Debugf("[BULLISH RESULT] %s state=Chaotic res=%t rsi=%.1f hist=%.6f rr=%.3f",
			pair, res, ci.RSIVal, ci.HistSlope, dynRR)
		return res
	case models.Transitional:
		res := cs.checkBullishTransitional(ci, currentPrice, dynRR, emaAlignment, state)
		logger.Debugf("[BULLISH RESULT] %s state=Transitional res=%t rsi=%.1f hist=%.6f rr=%.3f",
			pair, res, ci.RSIVal, ci.HistSlope, dynRR)
		return res
	default:
		res := cs.checkBullishConditionsDefault(state, ci, currentPrice, dynRR, volumeOk)
		logger.Debugf("[BULLISH RESULT] %s state=Default res=%t rsi=%.1f hist=%.6f rr=%.3f",
			pair, res, ci.RSIVal, ci.HistSlope, dynRR)
		return res
	}
}

func (cs *CompoundStrategy) checkBullishTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool,
	state models.MarketState,
) bool {

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	if atr <= 0 {
		logger.DebugColorf(logger.Cyan, "[TRENDING] reject: ATR<=0")
		return false
	}

	width := ci.UpperBand - ci.LowerBand
	pricePos := 0.5
	if width > 0 {
		pricePos = clamp01((currentPrice - ci.LowerBand) / width)
	}

	trendAlign := cs.trendAlignmentScore(ci, state)

	minAlign := 0.55
	if ci.ADX >= 30 {
		minAlign = 0.53
	}
	if ci.ADX >= 40 && ci.PriceActionQuality >= 0.45 {
		minAlign = 0.50
	}

	logger.DebugColorf(
		logger.Cyan,
		"[TRENDING PRECHECK] price=%.4f trendAlign=%.2f (min=%.2f) noise=%.2f regime=%s HTF=%s mTf=%.2f PAQ=%.2f MQ=%.2f ADX=%.1f",
		currentPrice,
		trendAlign,
		minAlign,
		ci.NoiseLevel,
		ci.MarketRegime.String(),
		ci.MarketStateHTF.String(),
		ci.MultiTFTrendScore,
		ci.PriceActionQuality,
		ci.MomentumQuality,
		ci.ADX,
	)

	if trendAlign < minAlign {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: trendAlign too low (%.2f < %.2f)", trendAlign, minAlign)
		return false
	}

	if ci.NoiseLevel >= 0.80 {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: noise too high (%.2f >= 0.80)", ci.NoiseLevel)
		return false
	}

	minPAQ := 0.35
	minMQ := 0.28

	if ci.ADX < 20 {
		minPAQ += 0.05
	}
	if ci.ADX >= 40 {
		minPAQ -= 0.03
		minMQ -= 0.03
	}

	if minPAQ < 0.28 {
		minPAQ = 0.28
	}
	if minMQ < 0.22 {
		minMQ = 0.22
	}

	if ci.PriceActionQuality < minPAQ {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: PAQ too low (%.2f < %.2f)", ci.PriceActionQuality, minPAQ)
		return false
	}
	if ci.MomentumQuality < minMQ {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: MQ too low (%.2f < %.2f)", ci.MomentumQuality, minMQ)
		return false
	}

	if ci.MarketRegime == models.RangeBoundRegime &&
		ci.VolatilityRegime == models.HighVolatilityRegime && ci.ADX < 20 {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: range+highVol+lowADX (regime=%s vol=%s ADX=%.1f)",
			ci.MarketRegime.String(), ci.VolatilityRegime.String(), ci.ADX)
		return false
	}

	mid, stLower, _ := cs.supertrendBand(ci, 2.5)
	if mid > 0 && stLower > 0 {
		c := ci.CandleSticks
		n := len(c)
		okBand := true
		for i := n - 5; i < n; i++ {
			if i < 0 {
				continue
			}
			if c[i].Close < stLower {
				okBand = false
				break
			}
		}
		if !okBand {
			logger.DebugColorf(logger.Cyan,
				"[TRENDING] reject: recent closes under supertrend band (mid=%.4f stLower=%.4f)",
				mid, stLower)
			return false
		}
	}

	pullbackEntry := false
	if mid > 0 {
		if currentPrice >= mid*0.98 && currentPrice <= mid*1.02 {
			pullbackEntry = true
		}
	} else if ci.MiddleBand > 0 {
		if currentPrice >= ci.MiddleBand*0.98 && currentPrice <= ci.MiddleBand*1.02 {
			pullbackEntry = true
		}
	}

	breakoutEntry := false
	if c := ci.CandleSticks; len(c) >= 3 {
		n := len(c)
		prevHigh := math.Max(c[n-2].High, c[n-3].High)
		last := c[n-1]

		boLevel := prevHigh + 0.08*atr

		avgVol := 0.0
		count := 0.0
		for i := n - 10; i < n-1; i++ {
			if i >= 0 {
				avgVol += c[i].Volume
				count++
			}
		}
		if count > 0 {
			avgVol /= count
		}

		boCloseOK := last.Close >= boLevel
		boLowOK := last.Low >= prevHigh-0.06*atr
		boBullish := last.Close > last.Open*0.998
		boVolume := avgVol > 0 && last.Volume >= avgVol*1.0

		breakoutEntry = boCloseOK && boLowOK && boBullish && boVolume

		logger.DebugColorf(
			logger.Cyan,
			"[TRENDING] breakout check: boClose=%t boLow=%t bull=%t vol=%t (boLvl=%.4f last=%.4f prevHigh=%.4f)",
			boCloseOK, boLowOK, boBullish, boVolume, boLevel, last.Close, prevHigh,
		)
	}

	ridingTrend := ci.MacdLine > ci.SignalLine &&
		ci.HistSlope > 0.00005 &&
		ci.MiddleBandSlope > 0.0001 &&
		pricePos > 0.40 && pricePos < 0.90

	macdMomOK := ci.MacdIndicator == 1 && ci.HistSlope > 0.0
	rsiMomOK := ci.RsiSlope > 0.0 && ci.RSIVal > 30 && ci.RSIVal < 72
	momentumOK := macdMomOK || rsiMomOK

	if !momentumOK && !pullbackEntry {
		logger.DebugColorf(
			logger.Cyan,
			"[TRENDING] reject: momentum weak and no proper pullback (MACDmom=%t RSImom=%t)",
			macdMomOK, rsiMomOK,
		)
		return false
	}

	volPenalty := 1.0
	if !volCCIok {
		volPenalty = 0.9
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] vol/CCI weak → edge penalty applied")
	}

	overExtBB := ci.UpperBand > 0 && currentPrice >= ci.UpperBand+0.10*atr
	overExtKC := ci.KCUpper > 0 && currentPrice >= ci.KCUpper+0.10*atr
	if overExtBB || overExtKC {
		logger.DebugColorf(
			logger.Cyan,
			"[TRENDING] reject: overextended (cp=%.4f, BBext=%t KCext=%t)",
			currentPrice, overExtBB, overExtKC,
		)
		return false
	}

	stop := currentPrice - 1.5*atr
	target := math.Max(ci.UpperBand*1.01, currentPrice+2.3*atr)
	rr := cs.calcRR(state, currentPrice, stop, target)
	if rr <= 0 {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: RR<=0 (stop=%.4f target=%.4f)", stop, target)
		return false
	}

	minRR := dynamicRR * 0.90
	if rr < minRR {
		logger.DebugColorf(
			logger.Cyan,
			"[TRENDING] reject: RR too low (%.2f < %.2f) dynRR=%.2f",
			rr, minRR, dynamicRR,
		)
		return false
	}

	edge := cs.edgeScore(ci, state, currentPrice, stop, target, dynamicRR) * volPenalty
	minEdge := 1.05

	logger.DebugColorf(
		logger.Cyan,
		"[TRENDING] RR=%.2f dynRR=%.2f edge=%.2f (minEdge=%.2f) pricePos=%.2f pullback=%t breakout=%t riding=%t volCCIok=%t",
		rr, dynamicRR, edge, minEdge, pricePos, pullbackEntry, breakoutEntry, ridingTrend, volCCIok,
	)

	if edge < minEdge {
		logger.DebugColorf(logger.Cyan,
			"[TRENDING] reject: EdgeScore too low (%.2f < %.2f)", edge, minEdge)
		return false
	}

	hasPattern := pullbackEntry || breakoutEntry || (ridingTrend && momentumOK)

	strongTrendFallback := !hasPattern &&
		momentumOK &&
		trendAlign >= (minAlign+0.05) &&
		rr >= dynamicRR*1.10 &&
		edge >= (minEdge+0.10)

	if !hasPattern && !strongTrendFallback {
		logger.DebugColorf(
			logger.Cyan,
			"[TRENDING] reject: no valid entry pattern (pullback=%t breakout=%t riding=%t fallback=%t)",
			pullbackEntry, breakoutEntry, ridingTrend, strongTrendFallback,
		)
		return false
	}

	logger.InfoColorf(
		logger.Green,
		"[TRENDING BUY SIGNAL ✓] Price=%.4f RR=%.2f Edge=%.2f TrendAlign=%.2f pattern=%s fallback=%t",
		currentPrice,
		rr,
		edge,
		trendAlign,
		func() string {
			switch {
			case pullbackEntry:
				return "pullback"
			case breakoutEntry:
				return "breakout"
			case ridingTrend:
				return "riding"
			default:
				return "none"
			}
		}(),
		strongTrendFallback,
	)

	return true
}

func (cs *CompoundStrategy) checkBullishStronglyTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool, emaUp bool,
	state models.MarketState,
) bool {

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	if atr <= 0 {
		logger.DebugColorf(logger.Magenta, "[ST-TREND] reject: ATR<=0")
		return false
	}

	width := ci.UpperBand - ci.LowerBand
	pricePos := 0.5
	if width > 0 {
		pricePos = clamp01((currentPrice - ci.LowerBand) / width)
	}

	trendAlign := cs.trendAlignmentScore(ci, state)
	if emaUp {
		trendAlign += 0.05
		if trendAlign > 1.0 {
			trendAlign = 1.0
		}
	}

	minAlign := 0.60 // base for StronglyTrending
	if ci.ADX >= 45 {
		minAlign = 0.58
	}
	if emaUp && ci.ADX >= 50 && ci.PriceActionQuality >= 0.50 {
		minAlign = 0.55
	}

	logger.DebugColorf(
		logger.Magenta,
		"[ST-TREND PRECHECK] price=%.4f trendAlign=%.2f (min=%.2f) noise=%.2f ADX=%.1f regime=%s HTF=%s mTf=%.2f PAQ=%.2f MQ=%.2f",
		currentPrice,
		trendAlign,
		minAlign,
		ci.NoiseLevel,
		ci.ADX,
		ci.MarketRegime.String(),
		ci.MarketStateHTF.String(),
		ci.MultiTFTrendScore,
		ci.PriceActionQuality,
		ci.MomentumQuality,
	)

	if trendAlign < minAlign {
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] reject: trendAlign too low (%.2f < %.2f)", trendAlign, minAlign)
		return false
	}

	if ci.NoiseLevel >= 0.75 {
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] reject: noise too high (%.2f >= 0.75)", ci.NoiseLevel)
		return false
	}

	if ci.ADX < 22 {
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] reject: ADX too weak for strong trend (%.1f < 22)", ci.ADX)
		return false
	}

	minPAQ := 0.43
	minMQ := 0.43
	if ci.ADX >= 45 {
		minPAQ -= 0.05
		minMQ -= 0.05
	}
	if minPAQ < 0.36 {
		minPAQ = 0.36
	}
	if minMQ < 0.36 {
		minMQ = 0.36
	}

	if ci.PriceActionQuality < minPAQ || ci.MomentumQuality < minMQ {
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] reject: PAQ/MQ too low (PAQ=%.2f<%.2f MQ=%.2f<%.2f)",
			ci.PriceActionQuality, minPAQ, ci.MomentumQuality, minMQ)
		return false
	}

	mid, stLower, stUpper := cs.supertrendBand(ci, 3.0)
	if mid > 0 && stLower > 0 {
		c := ci.CandleSticks
		n := len(c)
		okBand := true
		for i := n - 6; i < n; i++ {
			if i < 0 {
				continue
			}
			if c[i].Close < stLower {
				okBand = false
				break
			}
		}
		if !okBand {
			logger.DebugColorf(logger.Magenta,
				"[ST-TREND] reject: recent closes under strong-trend band (mid=%.4f stLower=%.4f)",
				mid, stLower)
			return false
		}
		if currentPrice > stUpper+0.5*atr {
			logger.DebugColorf(logger.Magenta,
				"[ST-TREND] reject: price too far above band (cp=%.4f stUpper=%.4f)",
				currentPrice, stUpper)
			return false
		}
	}

	pullbackTenkan := ci.IchimokuRes.Tenkan > 0 &&
		currentPrice >= ci.IchimokuRes.Tenkan*0.98 &&
		currentPrice <= ci.IchimokuRes.Tenkan*1.02

	pullbackKijun := ci.IchimokuRes.Kijun > 0 &&
		currentPrice >= ci.IchimokuRes.Kijun*0.98 &&
		currentPrice <= ci.IchimokuRes.Kijun*1.02

	pullbackMB := ci.MiddleBand > 0 &&
		currentPrice >= ci.MiddleBand*0.98 &&
		currentPrice <= ci.MiddleBand*1.02

	pullbackEntry := pullbackTenkan || pullbackKijun || pullbackMB

	breakoutEntry := false
	if c := ci.CandleSticks; len(c) >= 3 {
		n := len(c)
		prevHigh := math.Max(c[n-2].High, c[n-3].High)
		last := c[n-1]

		boLevel := prevHigh + 0.10*atr

		avgVol := 0.0
		count := 0.0
		for i := n - 12; i < n-1; i++ {
			if i >= 0 {
				avgVol += c[i].Volume
				count++
			}
		}
		if count > 0 {
			avgVol /= count
		}

		boCloseOK := last.Close >= boLevel
		boLowOK := last.Low >= prevHigh-0.08*atr
		boBullish := last.Close > last.Open*1.001
		boVolume := avgVol > 0 && last.Volume >= avgVol*1.1

		breakoutEntry = boCloseOK && boLowOK && boBullish && boVolume

		logger.DebugColorf(
			logger.Magenta,
			"[ST-TREND] breakout check: boClose=%t boLow=%t bull=%t vol=%t (boLvl=%.4f last=%.4f prevHigh=%.4f)",
			boCloseOK, boLowOK, boBullish, boVolume, boLevel, last.Close, prevHigh,
		)
	}

	ridingTrend := ci.MacdLine > ci.SignalLine &&
		ci.HistSlope > 0.00010 &&
		ci.MiddleBandSlope > 0.00020 &&
		pricePos > 0.45 && pricePos < 0.95 &&
		ci.IchimokuRes.Bullish

	macdMomOK := ci.MacdIndicator == 1 && ci.HistSlope > 0.0
	rsiMomOK := ci.RsiSlope > 0.0 && ci.RSIVal > 25 && ci.RSIVal < 75
	momentumOK := macdMomOK || rsiMomOK

	if !momentumOK && !pullbackEntry && !breakoutEntry {
		logger.DebugColorf(
			logger.Magenta,
			"[ST-TREND] reject: momentum weak and no qualified pullback/breakout (MACDmom=%t RSImom=%t)",
			macdMomOK, rsiMomOK,
		)
		return false
	}

	maxPrice := ci.UpperBand + 0.20*atr
	if ci.UpperBand > 0 && currentPrice > maxPrice {
		logger.DebugColorf(
			logger.Magenta,
			"[ST-TREND] reject: hard overextension (cp=%.4f > max=%.4f)",
			currentPrice, maxPrice,
		)
		return false
	}

	stop := currentPrice - 1.7*atr
	target := math.Max(ci.UpperBand*1.02, currentPrice+2.7*atr)
	rr := cs.calcRR(state, currentPrice, stop, target)
	if rr <= 0 {
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] reject: RR<=0 (stop=%.4f target=%.4f)", stop, target)
		return false
	}

	minRR := dynamicRR * 0.85
	if rr < minRR {
		logger.DebugColorf(
			logger.Magenta,
			"[ST-TREND] reject: RR too low (%.2f < %.2f) dynRR=%.2f",
			rr, minRR, dynamicRR,
		)
		return false
	}

	edge := cs.edgeScore(ci, state, currentPrice, stop, target, dynamicRR)

	minEdge := 0.95
	if !volCCIok {
		minEdge += 0.10 // 1.05 when vol/CCI weak
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] vol/CCI weak → minEdge raised to %.2f", minEdge)
	} else {
		edge *= 1.03 // small reward for clean context
	}

	logger.DebugColorf(
		logger.Magenta,
		"[ST-TREND] RR=%.2f dynRR=%.2f edge=%.2f (minEdge=%.2f) pricePos=%.2f pullback=%t breakout=%t riding=%t volCCIok=%t",
		rr, dynamicRR, edge, minEdge, pricePos, pullbackEntry, breakoutEntry, ridingTrend, volCCIok,
	)

	if edge < minEdge {
		logger.DebugColorf(logger.Magenta,
			"[ST-TREND] reject: EdgeScore too low (%.2f < %.2f)", edge, minEdge)
		return false
	}

	hasPattern := pullbackEntry || breakoutEntry || ridingTrend

	strongTrendFallback := !hasPattern &&
		momentumOK &&
		trendAlign >= (minAlign+0.05) &&
		rr >= dynamicRR*1.10 &&
		edge >= (minEdge+0.10) &&
		pricePos > 0.30 && pricePos < 0.90

	if !hasPattern && !strongTrendFallback {
		logger.DebugColorf(
			logger.Magenta,
			"[ST-TREND] reject: no valid entry pattern (pullback=%t breakout=%t riding=%t fallback=%t)",
			pullbackEntry, breakoutEntry, ridingTrend, strongTrendFallback,
		)
		return false
	}

	logger.InfoColorf(
		logger.Green,
		"[ST-TREND BUY SIGNAL ✓] Price=%.4f RR=%.2f Edge=%.2f TrendAlign=%.2f pattern=%s fallback=%t",
		currentPrice,
		rr,
		edge,
		trendAlign,
		func() string {
			switch {
			case pullbackEntry:
				return "pullback"
			case breakoutEntry:
				return "breakout"
			case ridingTrend:
				return "riding"
			default:
				return "none"
			}
		}(),
		strongTrendFallback,
	)

	return true
}

func (cs *CompoundStrategy) checkBullishRangeBound(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	state models.MarketState,
) bool {

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	if atr <= 0 {
		logger.DebugColorf(logger.Yellow, "[RANGE] reject: ATR<=0")
		return false
	}

	if ci.UpperBand <= 0 || ci.LowerBand <= 0 || ci.UpperBand <= ci.LowerBand {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: invalid bands UB=%.4f LB=%.4f", ci.UpperBand, ci.LowerBand)
		return false
	}

	width := ci.UpperBand - ci.LowerBand
	if width <= 0 {
		logger.DebugColorf(logger.Yellow, "[RANGE] reject: non-positive band width")
		return false
	}

	if currentPrice > 0 && width/currentPrice < 0.015 {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: too narrow band (width=%.6f price=%.6f)", width, currentPrice)
		return false
	}

	if ci.MarketRegime != models.RangeBoundRegime {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: regime=%s (expected RangeBound)", ci.MarketRegime.String())
		return false
	}

	if ci.ADX <= 6 {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: ADX too low / no structure (%.1f <= 6)", ci.ADX)
		return false
	}
	if ci.ADX >= 28 {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: ADX too high for range (%.1f >= 28)", ci.ADX)
		return false
	}

	if ci.NoiseLevel > 0.83 {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: noise too high (%.2f > 0.83)", ci.NoiseLevel)
		return false
	}

	minPAQ := 0.30
	minMQ := 0.26
	if ci.PriceActionQuality < minPAQ || ci.MomentumQuality < minMQ {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] reject: PAQ/MQ too low (PAQ=%.2f<%.2f MQ=%.2f<%.2f)",
			ci.PriceActionQuality, minPAQ, ci.MomentumQuality, minMQ)
		return false
	}

	logger.DebugColorf(
		logger.Yellow,
		"[RANGE PRECHECK] price=%.4f ATR=%.5f width=%.5f noise=%.2f regime=%s PAQ=%.2f MQ=%.2f ADX=%.1f",
		currentPrice, atr, width, ci.NoiseLevel, ci.MarketRegime.String(),
		ci.PriceActionQuality, ci.MomentumQuality, ci.ADX,
	)

	var score float64 = 0.0
	requiredScore := 6.5

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
	if lowBounce {
		score += 2.0
		logger.DebugColorf(logger.Yellow,
			"[RANGE] ✓1 LowBounce [+2.0] nearLower=%t oversoldCount=%d => score=%.2f",
			nearLower, oversoldCount, score)
	} else if nearLower && oversoldCount == 1 {
		score += 1.0
		logger.DebugColorf(logger.Yellow,
			"[RANGE] ✓1 LowBounce [+1.0] nearLower=%t oversoldCount=%d => score=%.2f",
			nearLower, oversoldCount, score)
	} else {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] ✗1 LowBounce nearLower=%t oversoldCount=%d => score=%.2f",
			nearLower, oversoldCount, score)
		return false
	}

	adrConfirmation := ci.ADRSignal == 1
	if adrConfirmation {
		score += 1.0
		logger.DebugColorf(logger.Yellow, "[RANGE] ✓2 ADR [+1.0] ADRSignal=%d => score=%.2f", ci.ADRSignal, score)
	} else if ci.ADRSignal >= 0 {
		score += 0.3
		logger.DebugColorf(logger.Yellow, "[RANGE] ✓2 ADR [+0.3 weak] ADRSignal=%d => score=%.2f", ci.ADRSignal, score)
	} else {
		logger.DebugColorf(logger.Yellow, "[RANGE] ✗2 ADR ADRSignal=%d => score=%.2f", ci.ADRSignal, score)
	}

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

			switch {
			case hammer && engulfing:
				score += 2.0
			case hammer || engulfing:
				score += 1.2
			}

			logger.DebugColorf(logger.Yellow,
				"[RANGE] ✓3 Pattern [+%.1f] hammer=%t engulf=%t => rev=%t score=%.2f",
				func() float64 {
					if hammer && engulfing {
						return 2.0
					}
					if hammer || engulfing {
						return 1.2
					}
					return 0.0
				}(),
				hammer, engulfing, reversalPattern, score)
		}
	} else {
		logger.DebugColorf(logger.Yellow,
			"[RANGE] ✗3 Pattern (insufficient candles) => score=%.2f", score)
	}

	entryScore := cs.entryScore(ci, currentPrice)
	if entryScore >= 0.75 {
		score += 1.6
	} else if entryScore >= 0.65 {
		score += 1.0
	} else if entryScore >= 0.55 {
		score += 0.4
	}
	logger.DebugColorf(logger.Yellow,
		"[RANGE] ✓4 EntryScore [+?] entryScore=%.3f => score=%.2f",
		entryScore, score)

	histUp := ci.HistSlope > 0
	rsiSlopeUp := ci.RsiSlope > 0
	switch {
	case histUp && rsiSlopeUp:
		score += 1.2
	case histUp || rsiSlopeUp:
		score += 0.6
	}
	logger.DebugColorf(logger.Yellow,
		"[RANGE] ✓5 Momentum [+?] hist=%.6f>0:%t rsiSlope=%.4f>0:%t => score=%.2f",
		ci.HistSlope, histUp, ci.RsiSlope, rsiSlopeUp, score)

	if n := len(ci.CandleSticks); n >= 2 {
		last := ci.CandleSticks[n-1]
		avgVol := 0.0
		for i := n - 12; i < n-1; i++ {
			if i >= 0 {
				avgVol += ci.CandleSticks[i].Volume
			}
		}
		avgVol /= math.Min(12, float64(n-1))

		var add float64
		if last.Volume >= avgVol*1.1 {
			add = 1.0
		} else if last.Volume >= avgVol*0.9 {
			add = 0.5
		} else if last.Volume >= avgVol*0.75 {
			add = 0.2
		}
		score += add
		logger.DebugColorf(logger.Yellow,
			"[RANGE] ✓6 Volume [+%.1f] last=%.0f avg=%.0f => score=%.2f",
			add, last.Volume, avgVol, score)
	}

	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	var rrAdd float64
	if rr > dynamicRR*1.20 {
		rrAdd = 2.0
	} else if rr > dynamicRR*1.05 {
		rrAdd = 1.0
	} else if rr > dynamicRR {
		rrAdd = 0.4
	}
	score += rrAdd
	logger.DebugColorf(logger.Yellow,
		"[RANGE] ✓7 RR [+%.1f] stop=%.4f target=%.4f RR=%.3f dynRR=%.3f => score=%.2f",
		rrAdd, stop, target, rr, dynamicRR, score)

	edge := cs.edgeScore(ci, state, currentPrice, stop, target, dynamicRR)
	minRR := dynamicRR * 0.95
	minEdge := 0.98

	logger.DebugColorf(
		logger.Yellow,
		"[RANGE] EdgeCheck RR=%.2f dynRR=%.2f edge=%.2f (minRR=%.2f minEdge=%.2f) => score=%.2f",
		rr, dynamicRR, edge, minRR, minEdge, score,
	)

	if rr < minRR || edge < minEdge {
		logger.DebugColorf(
			logger.Yellow,
			"[RANGE] reject: RR/Edge too weak (RR=%.2f<%.2f or Edge=%.2f<%.2f)",
			rr, minRR, edge, minEdge,
		)
		return false
	}

	result := score >= requiredScore
	logger.DebugColorf(logger.Yellow,
		"[RANGE] ✓8 FINAL | Score=%.2f Required=%.2f => %t",
		score, requiredScore, result)

	if result {
		logger.InfoColorf(logger.Green,
			"[RANGE BUY SIGNAL ✓] Price=%.4f RR=%.2f Score=%.2f Pattern=%s",
			currentPrice, rr, score,
			map[bool]string{true: "Hammer/Engulf", false: "None"}[reversalPattern],
		)
	}
	return result
}

func (cs *CompoundStrategy) checkBullishTransitional(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	emaAlignment bool,
	state models.MarketState,
) bool {

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	if atr <= 0 {
		logger.DebugColorf(logger.BrightCyan, "[TRANS] reject: ATR<=0")
		return false
	}

	if ci.UpperBand <= 0 || ci.LowerBand <= 0 || ci.MiddleBand <= 0 {
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] reject: invalid bands UB=%.4f MB=%.4f LB=%.4f",
			ci.UpperBand, ci.MiddleBand, ci.LowerBand)
		return false
	}

	trendAlign := cs.trendAlignmentScore(ci, state)

	minAlign := 0.40
	maxAlign := 0.80 // too high => use Trending/StronglyTrending instead

	if emaAlignment {
		trendAlign += 0.03
		if trendAlign > 1.0 {
			trendAlign = 1.0
		}
	}

	logger.DebugColorf(
		logger.BrightCyan,
		"[TRANS PRECHECK] price=%.4f trendAlign=%.2f (range=%.2f–%.2f) noise=%.2f regime=%s HTF=%s mTf=%.2f PAQ=%.2f MQ=%.2f ADX=%.1f",
		currentPrice,
		trendAlign,
		minAlign,
		maxAlign,
		ci.NoiseLevel,
		ci.MarketRegime.String(),
		ci.MarketStateHTF.String(),
		ci.MultiTFTrendScore,
		ci.PriceActionQuality,
		ci.MomentumQuality,
		ci.ADX,
	)

	if trendAlign < minAlign || trendAlign > maxAlign {
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] reject: trendAlign out of transitional band (%.2f ∉ [%.2f,%.2f])",
			trendAlign, minAlign, maxAlign)
		return false
	}

	if ci.NoiseLevel >= 0.80 {
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] reject: noise too high (%.2f >= 0.80)", ci.NoiseLevel)
		return false
	}

	if ci.ADX < 14 || ci.ADX > 35 {
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] reject: ADX out of range (%.1f ∉ [14,35])", ci.ADX)
		return false
	}

	minPAQ := 0.34
	minMQ := 0.30
	if ci.PriceActionQuality < minPAQ || ci.MomentumQuality < minMQ {
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] reject: PAQ/MQ too low (PAQ=%.2f<%.2f MQ=%.2f<%.2f)",
			ci.PriceActionQuality, minPAQ, ci.MomentumQuality, minMQ)
		return false
	}

	var score float64 = 0.0
	requiredScore := 7.0

	entryScore := cs.entryScore(ci, currentPrice)
	if entryScore >= 0.80 {
		score += 2.0
	} else if entryScore >= 0.72 {
		score += 1.4
	} else if entryScore >= 0.65 {
		score += 0.6
	}
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓1 BaseScore [+?] entryScore=%.3f => score=%.2f",
		entryScore, score)

	histOK := ci.HistSlope > 0
	rsiSlopeOK := ci.RsiSlope > 0
	if histOK {
		score += 1.4 // slightly more weight than before for momentum
	} else if rsiSlopeOK {
		score += 0.7
	}
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓2 Momentum [+?] hist=%.6f>0:%t rsiSlope=%.4f>0:%t => score=%.2f",
		ci.HistSlope, histOK, ci.RsiSlope, rsiSlopeOK, score)

	adrOK := ci.ADRSignal > 0
	if adrOK {
		score += 1.0
	} else if ci.ADRSignal == 0 {
		score += 0.3
	}
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓3 ADR [+?] ADR=%d => score=%.2f",
		ci.ADRSignal, score)

	priceOK := currentPrice >= ci.MiddleBand*0.998 && currentPrice <= ci.UpperBand*1.03
	if priceOK {
		score += 1.0
	} else if currentPrice >= ci.MiddleBand*0.990 {
		score += 0.4
	}
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓4 Price [+?] price=%.4f MB*0.998=%.4f => score=%.2f",
		currentPrice, ci.MiddleBand*0.998, score)

	cciMid := ci.CCIVal > -100 && ci.CCIVal < 100
	mfiMid := ci.MFIVal > 25 && ci.MFIVal < 75
	if cciMid {
		score += 0.4
	}
	if mfiMid {
		score += 0.4
	}
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓5 Oscillators [+?] CCI=%.1f mid=%t MFI=%.1f mid=%t => score=%.2f",
		ci.CCIVal, cciMid, ci.MFIVal, mfiMid, score)

	adxOK := ci.ADX >= 18 && ci.ADX <= 35
	if adxOK {
		score += 1.0
	} else if ci.ADX >= 15 && ci.ADX < 18 {
		score += 0.3
	}
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓6 ADX [+?] ADX=%.1f => score=%.2f",
		ci.ADX, score)

	if emaAlignment {
		score += 1.0
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] ✓7 EMA Align [+1.0] => score=%.2f", score)
	} else {
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] ✗7 EMA Align => score=%.2f", score)
	}

	if n := len(ci.CandleSticks); n >= 2 {
		last := ci.CandleSticks[n-1]
		avgVol := 0.0
		for i := n - 10; i < n-1; i++ {
			if i >= 0 {
				avgVol += ci.CandleSticks[i].Volume
			}
		}
		avgVol /= math.Min(10, float64(n-1))

		var add float64
		if last.Volume >= avgVol*1.1 {
			add = 1.0
		} else if last.Volume >= avgVol*0.9 {
			add = 0.5
		}
		score += add
		logger.DebugColorf(logger.BrightCyan,
			"[TRANS] ✓8 Volume [+%.1f] last=%.0f avg=%.0f => score=%.2f",
			add, last.Volume, avgVol, score)
	}

	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	var rrAdd float64
	if rr > dynamicRR*1.15 {
		rrAdd = 1.5
	} else if rr > dynamicRR*1.05 {
		rrAdd = 0.8
	} else if rr > dynamicRR {
		rrAdd = 0.4
	}
	score += rrAdd
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓9 RR [+%.1f] stop=%.4f target=%.4f RR=%.3f dynRR=%.3f => score=%.2f",
		rrAdd, stop, target, rr, dynamicRR, score)

	edge := cs.edgeScore(ci, state, currentPrice, stop, target, dynamicRR)
	minRR := dynamicRR * 0.92
	minEdge := 1.02

	logger.DebugColorf(
		logger.BrightCyan,
		"[TRANS] EdgeCheck RR=%.2f dynRR=%.2f edge=%.2f (minRR=%.2f minEdge=%.2f) => score=%.2f",
		rr, dynamicRR, edge, minRR, minEdge, score,
	)

	if rr < minRR || edge < minEdge {
		logger.DebugColorf(
			logger.BrightCyan,
			"[TRANS] reject: RR/Edge too weak (RR=%.2f<%.2f or Edge=%.2f<%.2f)",
			rr, minRR, edge, minEdge,
		)
		return false
	}

	result := score >= requiredScore
	logger.DebugColorf(logger.BrightCyan,
		"[TRANS] ✓10 FINAL | Score=%.2f Required=%.2f => %t",
		score, requiredScore, result)

	if result {
		logger.InfoColorf(logger.Green,
			"[TRANS BUY SIGNAL ✓] Price=%.4f Score=%.2f RR=%.2f ADX=%.1f",
			currentPrice, score, rr, ci.ADX)
	}
	return result
}

func (cs *CompoundStrategy) checkBullishChaotic(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	state models.MarketState,
) bool {

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	if atr <= 0 {
		logger.DebugColorf(logger.BrightYellow, "[CHAOTIC] reject: ATR<=0")
		return false
	}

	if ci.UpperBand <= 0 || ci.LowerBand <= 0 {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: invalid bands UB=%.4f LB=%.4f", ci.UpperBand, ci.LowerBand)
		return false
	}

	if ci.VolatilityRegime != models.HighVolatilityRegime {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: volatilityRegime=%s (expected HighVol)", ci.VolatilityRegime.String())
		return false
	}
	if ci.MarketRegime == models.RangeBoundRegime {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: RangeBound regime – use RangeBound logic instead")
		return false
	}

	if ci.NoiseLevel < 0.55 || ci.NoiseLevel > 0.86 {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: noise out of range (%.2f ∉ [0.55,0.86])", ci.NoiseLevel)
		return false
	}

	if ci.ADX < 18 || ci.ADX > 50 {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: ADX out of range (%.1f ∉ [18,50])", ci.ADX)
		return false
	}

	minPAQ := 0.38
	minMQ := 0.35
	if ci.PriceActionQuality < minPAQ || ci.MomentumQuality < minMQ {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: PAQ/MQ too low (PAQ=%.2f<%.2f MQ=%.2f<%.2f)",
			ci.PriceActionQuality, minPAQ, ci.MomentumQuality, minMQ)
		return false
	}

	trendAlign := cs.trendAlignmentScore(ci, state)

	logger.DebugColorf(
		logger.BrightYellow,
		"[CHAOTIC PRECHECK] price=%.4f ATR=%.5f noise=%.2f regime=%s vol=%s HTF=%s mTf=%.2f PAQ=%.2f MQ=%.2f ADX=%.1f trendAlign=%.2f",
		currentPrice,
		atr,
		ci.NoiseLevel,
		ci.MarketRegime.String(),
		ci.VolatilityRegime.String(),
		ci.MarketStateHTF.String(),
		ci.MultiTFTrendScore,
		ci.PriceActionQuality,
		ci.MomentumQuality,
		ci.ADX,
		trendAlign,
	)

	if trendAlign < 0.30 || trendAlign > 0.80 {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: trendAlign out of chaotic band (%.2f ∉ [0.30,0.80])",
			trendAlign)
		return false
	}

	var score float64 = 0.0
	requiredScore := 8.0 // a bit higher than before

	entryScore := cs.entryScore(ci, currentPrice)
	if entryScore >= 0.82 {
		score += 2.2
	} else if entryScore >= 0.78 {
		score += 1.6
	} else if entryScore >= 0.70 {
		score += 0.8
	}
	logger.DebugColorf(logger.BrightYellow,
		"[CHAOTIC] ✓1 BaseScore [+?] entryScore=%.3f => score=%.2f",
		entryScore, score)

	histOK := ci.HistSlope > 0
	rsiSlopeOK := ci.RsiSlope > 0
	rsiInRange := ci.RSIVal < 58 && ci.RSIVal > 35

	if histOK {
		score += 1.0
	}
	if rsiSlopeOK {
		score += 0.8
	}
	if rsiInRange {
		score += 0.7
	}
	logger.DebugColorf(logger.BrightYellow,
		"[CHAOTIC] ✓2 Momentum [+?] hist=%.6f>0:%t rsiSlope=%.4f>0:%t RSI=%.2f in(35,58):%t => score=%.2f",
		ci.HistSlope, histOK, ci.RsiSlope, rsiSlopeOK, ci.RSIVal, rsiInRange, score)

	if !(histOK || rsiSlopeOK) || !rsiInRange {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] reject: momentum / RSI conditions not met (histOK=%t rsiOK=%t rsiInRange=%t)",
			histOK, rsiSlopeOK, rsiInRange)
		return false
	}

	logger.DebugColorf(logger.BrightYellow, "[CHAOTIC] ✓3 ATR | ATR=%.6f", atr)

	nearMB := currentPrice <= ci.MiddleBand*1.005
	lowerQuarter := currentPrice <= ci.LowerBand+0.25*(ci.UpperBand-ci.LowerBand)
	nearSupport := nearMB || lowerQuarter
	if nearSupport {
		score += 1.4
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] ✓4 Support [+1.4] nearMB=%t lowerQuarter=%t => score=%.2f",
			nearMB, lowerQuarter, score)
	} else {
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] ✗4 Support nearMB=%t lowerQuarter=%t => score=%.2f",
			nearMB, lowerQuarter, score)

		return false
	}

	adxOK := ci.ADX >= 20
	if adxOK {
		score += 1.2
	} else if ci.ADX >= 16 {
		score += 0.5
	}
	logger.DebugColorf(logger.BrightYellow,
		"[CHAOTIC] ✓5 ADX [+?] ADX=%.1f => score=%.2f",
		ci.ADX, score)

	if n := len(ci.CandleSticks); n >= 2 {
		last := ci.CandleSticks[n-1]
		avgVol := 0.0
		for i := n - 15; i < n-1; i++ {
			if i >= 0 {
				avgVol += ci.CandleSticks[i].Volume
			}
		}
		avgVol /= math.Min(15, float64(n-1))

		var add float64
		if last.Volume >= avgVol*1.3 {
			add = 1.6
		} else if last.Volume >= avgVol*1.1 {
			add = 1.0
		} else if last.Volume >= avgVol*0.95 {
			add = 0.4
		}
		score += add
		logger.DebugColorf(logger.BrightYellow,
			"[CHAOTIC] ✓6 Volume [+%.1f] last=%.0f avg=%.0f => score=%.2f",
			add, last.Volume, avgVol, score)
	}

	target := ci.UpperBand
	stop := currentPrice - 2.5*atr
	rr := cs.calcRR(state, currentPrice, stop, target)
	var rrAdd float64
	if rr > dynamicRR*1.35 {
		rrAdd = 2.0
	} else if rr > dynamicRR*1.25 {
		rrAdd = 1.2
	} else if rr > dynamicRR*1.10 {
		rrAdd = 0.6
	}
	score += rrAdd
	logger.DebugColorf(logger.BrightYellow,
		"[CHAOTIC] ✓7 RR [+%.1f] stop=%.4f target=%.4f RR=%.3f dynRR=%.3f => score=%.2f",
		rrAdd, stop, target, rr, dynamicRR, score)

	edge := cs.edgeScore(ci, state, currentPrice, stop, target, dynamicRR)
	minRR := dynamicRR * 1.10
	minEdge := 1.08

	logger.DebugColorf(
		logger.BrightYellow,
		"[CHAOTIC] EdgeCheck RR=%.2f dynRR=%.2f edge=%.2f (minRR=%.2f minEdge=%.2f) => score=%.2f",
		rr, dynamicRR, edge, minRR, minEdge, score,
	)

	if rr < minRR || edge < minEdge {
		logger.DebugColorf(
			logger.BrightYellow,
			"[CHAOTIC] reject: RR/Edge too weak (RR=%.2f<%.2f or Edge=%.2f<%.2f)",
			rr, minRR, edge, minEdge,
		)
		return false
	}

	result := score >= requiredScore
	logger.DebugColorf(logger.BrightYellow,
		"[CHAOTIC] ✓8 FINAL | Score=%.2f Required=%.2f => %t",
		score, requiredScore, result)

	if result {
		logger.InfoColorf(logger.Green,
			"[CHAOTIC BUY SIGNAL ✓] Price=%.4f Score=%.2f RR=%.2f RSI=%.1f ADX=%.1f",
			currentPrice, score, rr, ci.RSIVal, ci.ADX)
	}
	return result
}

func (cs *CompoundStrategy) checkBullishConditionsDefault(
	state models.MarketState,
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volumeOk bool,
) bool {

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, ci.MiddleBand)
	if atr <= 0 {
		logger.DebugColorf(logger.BrightBlack, "[DEFAULT] reject: ATR<=0")
		return false
	}

	if ci.UpperBand <= 0 || ci.LowerBand <= 0 || ci.MiddleBand <= 0 {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] reject: invalid bands UB=%.4f MB=%.4f LB=%.4f",
			ci.UpperBand, ci.MiddleBand, ci.LowerBand)
		return false
	}

	if ci.NoiseLevel >= 0.82 {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] reject: noise too high (%.2f >= 0.82)", ci.NoiseLevel)
		return false
	}

	if ci.ADX < 10 {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] reject: ADX too low (%.1f < 10)", ci.ADX)
		return false
	}
	if ci.ADX > 38 {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] reject: ADX too high for default (%.1f > 38)", ci.ADX)
		return false
	}

	minPAQ := 0.32
	minMQ := 0.28
	if ci.PriceActionQuality < minPAQ || ci.MomentumQuality < minMQ {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] reject: PAQ/MQ too low (PAQ=%.2f<%.2f MQ=%.2f<%.2f)",
			ci.PriceActionQuality, minPAQ, ci.MomentumQuality, minMQ)
		return false
	}

	trendAlign := cs.trendAlignmentScore(ci, state)
	if trendAlign < 0.45 {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] reject: trendAlign too low (%.2f < 0.45)", trendAlign)
		return false
	}

	logger.DebugColorf(
		logger.BrightBlack,
		"[DEFAULT PRECHECK] price=%.4f ATR=%.5f noise=%.2f regime=%s HTF=%s mTf=%.2f PAQ=%.2f MQ=%.2f ADX=%.1f trendAlign=%.2f",
		currentPrice,
		atr,
		ci.NoiseLevel,
		ci.MarketRegime.String(),
		ci.MarketStateHTF.String(),
		ci.MultiTFTrendScore,
		ci.PriceActionQuality,
		ci.MomentumQuality,
		ci.ADX,
		trendAlign,
	)

	var score float64 = 0.0
	requiredScore := 6.0

	entryScore := cs.entryScore(ci, currentPrice)
	if entryScore >= 0.78 {
		score += 2.0
	} else if entryScore >= 0.70 {
		score += 1.3
	} else if entryScore >= 0.60 {
		score += 0.6
	}
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓1 BaseScore [+?] entryScore=%.3f => score=%.2f",
		entryScore, score)

	histOK := ci.HistSlope > 0
	rsiSlopeOK := ci.RsiSlope > 0
	if histOK {
		score += 1.0
	}
	if rsiSlopeOK {
		score += 0.6
	}
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓2 Momentum [+?] hist=%.6f>0:%t rsiSlope=%.4f>0:%t => score=%.2f",
		ci.HistSlope, histOK, ci.RsiSlope, rsiSlopeOK, score)

	rsiMid := ci.RSIVal > 40 && ci.RSIVal < 65
	if rsiMid {
		score += 0.8
	}
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓3 RSI [+0.8?] RSI=%.2f mid=%t => score=%.2f",
		ci.RSIVal, rsiMid, score)

	mbSlopeOK := ci.MiddleBandSlope >= 0
	if mbSlopeOK {
		score += 0.7
	}
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓4 MB Slope [+0.7?] mbSlope=%.6f>=0:%t => score=%.2f",
		ci.MiddleBandSlope, mbSlopeOK, score)

	adrOK := ci.ADRSignal > 0
	if adrOK {
		score += 0.7
	} else if ci.ADRSignal == 0 {
		score += 0.2
	}
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓5 ADR [+?] ADR=%d => score=%.2f",
		ci.ADRSignal, score)

	nearMidOrLow := false
	if ci.UpperBand > ci.LowerBand {
		width := ci.UpperBand - ci.LowerBand
		lowerZone := ci.LowerBand + 0.45*width
		upperZone := ci.MiddleBand * 1.02
		nearMidOrLow = currentPrice >= ci.LowerBand*0.99 &&
			currentPrice <= upperZone &&
			currentPrice <= lowerZone
	}
	if nearMidOrLow {
		score += 1.0
	} else if currentPrice <= ci.MiddleBand*1.03 {
		score += 0.5
	}
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓6 Price [+?] price=%.4f nearMidOrLow=%t => score=%.2f",
		currentPrice, nearMidOrLow, score)

	if volumeOk {
		score += 0.7
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] ✓7 VolumeFlag [+0.7] volumeOk=%t => score=%.2f", volumeOk, score)
	} else {
		logger.DebugColorf(logger.BrightBlack,
			"[DEFAULT] ✗7 VolumeFlag volumeOk=%t => score=%.2f", volumeOk, score)
	}

	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	var rrAdd float64
	if rr > dynamicRR*1.15 {
		rrAdd = 1.4
	} else if rr > dynamicRR*1.05 {
		rrAdd = 0.9
	} else if rr > dynamicRR {
		rrAdd = 0.4
	}
	score += rrAdd
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓8 RR [+%.1f] stop=%.4f target=%.4f RR=%.3f dynRR=%.3f => score=%.2f",
		rrAdd, stop, target, rr, dynamicRR, score)

	edge := cs.edgeScore(ci, state, currentPrice, stop, target, dynamicRR)
	minRR := dynamicRR * 0.92
	minEdge := 1.00

	logger.DebugColorf(
		logger.BrightBlack,
		"[DEFAULT] EdgeCheck RR=%.2f dynRR=%.2f edge=%.2f (minRR=%.2f minEdge=%.2f) => score=%.2f",
		rr, dynamicRR, edge, minRR, minEdge, score,
	)

	if rr < minRR || edge < minEdge {
		logger.DebugColorf(
			logger.BrightBlack,
			"[DEFAULT] reject: RR/Edge too weak (RR=%.2f<%.2f or Edge=%.2f<%.2f)",
			rr, minRR, edge, minEdge,
		)
		return false
	}

	result := score >= requiredScore
	logger.DebugColorf(logger.BrightBlack,
		"[DEFAULT] ✓9 FINAL | Score=%.2f Required=%.2f => %t",
		score, requiredScore, result)

	if result {
		logger.InfoColorf(logger.Green,
			"[DEFAULT BUY SIGNAL ✓] Price=%.4f Score=%.2f RR=%.2f",
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
		res := cs.checkBearishStronglyTrending(ci, currentPrice)
		logger.Debugf("[BEARISH RESULT] state=StronglyTrending res=%t rsi=%.1f hist=%.6f macdInd=%d", res, ci.RSIVal, ci.HistSlope, ci.MacdIndicator)
		return res
	case models.Trending:
		res := cs.checkBearishTrending(ci)
		logger.Debugf("[BEARISH RESULT] state=Trending res=%t rsi=%.1f hist=%.6f macdInd=%d", res, ci.RSIVal, ci.HistSlope, ci.MacdIndicator)
		return res
	case models.RangeBound:
		res := cs.checkBearishRangeBound(ci, currentPrice)
		logger.Debugf("[BEARISH RESULT] state=RangeBound res=%t rsi=%.1f upper=%.4f cp=%.4f", res, ci.RSIVal, ci.UpperBand, currentPrice)
		return res
	case models.Chaotic:
		res := cs.checkBearishChaotic(ci, currentPrice)
		logger.Debugf("[BEARISH RESULT] state=Chaotic res=%t rsi=%.1f hist=%.6f upper=%.4f cp=%.4f", res, ci.RSIVal, ci.HistSlope, ci.UpperBand, currentPrice)
		return res
	case models.Transitional:
		res := cs.checkBearishTransitional(ci)
		logger.Debugf("[BEARISH RESULT] state=Transitional res=%t rsi=%.1f hist=%.6f macdInd=%d", res, ci.RSIVal, ci.HistSlope, ci.MacdIndicator)
		return res
	default:
		res := cs.checkBearishDefault(ci)
		logger.Debugf("[BEARISH RESULT] state=Default res=%t rsi=%.1f hist=%.6f macdInd=%d", res, ci.RSIVal, ci.HistSlope, ci.MacdIndicator)
		return res
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

func (cs *CompoundStrategy) trendAlignmentScore(ci CurrentIndicators, state models.MarketState) float64 {
	score := 0.0

	if ci.EMA20 > 0 && ci.EMA50 > 0 && ci.EMA200 > 0 &&
		ci.EMA20 >= ci.EMA50 && ci.EMA50 >= ci.EMA200 {
		score += 0.40
	}
	if ci.EMASlope20 > 0 {
		score += 0.20
	}
	if ci.EMASlope50 > 0 {
		score += 0.20
	}

	if ci.ADX >= 25 {
		score += 0.40
	} else if ci.ADX >= 18 {
		score += 0.25
	} else if ci.ADX >= 14 {
		score += 0.10
	}

	cp := 0.0
	if n := len(ci.CandleSticks); n > 0 {
		cp = ci.CandleSticks[n-1].Close
	}
	cloudTop := math.Max(ci.IchimokuRes.SpanA, ci.IchimokuRes.SpanB)
	if ci.IchimokuRes.Bullish && cp >= cloudTop {
		score += 0.40
	} else if ci.IchimokuRes.Bullish {
		score += 0.25
	}

	switch ci.MarketStateHTF {
	case models.StronglyTrending, models.Trending:
		score += 0.25
	case models.RangeBound:
		score += 0.10
	case models.Chaotic:
		score -= 0.15
	}

	score += (ci.MultiTFTrendScore - 0.5) * 0.8

	switch ci.MarketRegime {
	case models.TrendingRegime:
		score += 0.15
	case models.RangeBoundRegime:
		score -= 0.05
	case models.MarketHighVolatilityRegime:
		score -= 0.05
	}

	score += (ci.PriceActionQuality - 0.5) * 0.6
	score += (ci.MomentumQuality - 0.5) * 0.6

	if state == models.StronglyTrending || state == models.Trending {
		score += 0.05
	}

	nl := ci.NoiseLevel
	if nl < 0 {
		nl = 0
	}
	if nl > 0.9 {
		nl = 0.9
	}
	score *= 1.0 - nl*0.6

	return clamp01(score)
}

func (cs *CompoundStrategy) supertrendBand(ci CurrentIndicators, atrMult float64) (mid, lower, upper float64) {
	mid = ci.MiddleBand
	if mid <= 0 {
		if n := len(ci.CandleSticks); n > 0 {
			mid = ci.CandleSticks[n-1].Close
		}
	}
	if mid <= 0 {
		return 0, 0, 0
	}

	atr := getATRSafe(ci.CandleSticks, ci.ADRVal, mid)
	if atr <= 0 {
		return mid, mid, mid
	}

	lower = mid - atrMult*atr
	upper = mid + atrMult*atr
	return mid, lower, upper
}

func (cs *CompoundStrategy) edgeScore(
	ci CurrentIndicators,
	state models.MarketState,
	currentPrice, stop, target, dynamicRR float64,
) float64 {

	if dynamicRR <= 0 {
		return 0
	}
	rr := cs.calcRR(state, currentPrice, stop, target)
	if rr <= 0 {
		return 0
	}

	trendAlign := cs.trendAlignmentScore(ci, state)

	nl := ci.NoiseLevel
	if nl < 0 {
		nl = 0
	}
	if nl > 0.9 {
		nl = 0.9
	}

	noiseFactor := 1.0 - nl*0.7
	if noiseFactor < 0.3 {
		noiseFactor = 0.3
	}

	score := (rr / dynamicRR) * trendAlign * noiseFactor

	if (state == models.StronglyTrending || state == models.Trending) &&
		ci.MarketRegime == models.TrendingRegime {
		score *= 1.05
	}

	return score
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
	state models.MarketState,
) int {
	repo := cs.ensureRepo()

	repo.UpdateSnapshot(pair, indicators, currentPrice, state)
	bias := repo.GetTrendBias(pair)

	oscScore := cs.oscillationScore(indicators)
	if cs.isOscillating(indicators) {
		logger.DebugColorf(
			logger.BrightBlack,
			"[PENDING] %s => Skip activation/add due to high oscillation (score=%.2f, noise=%.2f, regime=%s)",
			pair, oscScore, indicators.NoiseLevel, indicators.MarketRegime.String(),
		)
		return 0
	}

	allBefore := repo.GetAll(pair)
	logger.DebugColorf(
		logger.BrightBlack,
		"[PENDING] %s => evaluate: price=%.6f state=%s pend=%d bias=%s/%.2f cooldown=%v osc=%.2f",
		pair, currentPrice, state.String(), len(allBefore), bias.Direction, bias.Strength, pendingCoolDown, oscScore,
	)

	if bias.Direction == "UP" && bias.Strength > 0.55 {
		pendingCoolDown = time.Duration(float64(pendingCoolDown) * 0.8)
	}

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

		ok := last.Close >= boLevel &&
			last.Low >= prevHigh-0.06*atr &&
			indicators.MacdLine > indicators.SignalLine*0.99 &&
			indicators.HistSlope > -0.00001 &&
			last.Volume > avgVol*0.95

		logger.DebugColorf(
			logger.BrightBlack,
			"[PENDING MICRO BO] %s => ok=%t lastClose=%.6f boLevel=%.6f prevHigh=%.6f vol=%.0f avgVol=%.0f",
			pair, ok, last.Close, boLevel, prevHigh, last.Volume, avgVol,
		)

		return ok
	}

	ok := func(pb *PendingBuy) bool {

		if cs.isOscillating(indicators) {
			logger.DebugColorf(
				logger.BrightBlack,
				"[PENDING ACTIVATE] %s => Blocked due to oscillation at activation time (score=%.2f)",
				pair, oscScore,
			)
			return false
		}

		if pendingCoolDown > 0 && time.Since(pb.TriggerTime) < pendingCoolDown {
			if !(bias.Direction == "UP" && bias.Strength > 0.60 &&
				(pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending)) {
				logger.DebugColorf(
					logger.BrightBlack,
					"[PENDING ACTIVATE] %s => cooldown not elapsed (age=%.1fm < %v)",
					pair, time.Since(pb.TriggerTime).Minutes(), pendingCoolDown,
				)
				return false
			}
		}

		trendReady := pb.UpdateCount >= 3
		if !trendReady && bias.Direction == "UP" && bias.Strength > 0.65 && pb.UpdateCount >= 1 {
			trendReady = true
		}

		if trendReady {
			if !pb.ShouldBuyNow(currentPrice, indicators) {
				logger.DebugColorf(logger.Yellow,
					"[TREND CHECK] %s => Not ready yet (Quality:%.2f, Dir:%s, Cons:%.2f, Updates:%d, Bias:%.2f %s)",
					pair, pb.GetTrendQuality(), pb.TrendHistory.TrendDirection, pb.TrendHistory.Consistency, pb.UpdateCount, bias.Strength, bias.Direction)
				return false
			}

			logger.InfoColorf(logger.Green,
				"[TREND CHECK ✓] %s => Trend quality passed! Proceeding with entry checks...",
				pair)
		} else {
			logger.DebugColorf(
				logger.BrightBlack,
				"[TREND CHECK] %s => insufficient updates for trend (updates=%d, bias=%.2f %s)",
				pair, pb.UpdateCount, bias.Strength, bias.Direction,
			)
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
		if bias.Direction == "UP" && bias.Strength > 0.65 {
			chaseThreshold += 0.003
		}
		if pb.TriggerPrice > 0 && currentPrice > pb.TriggerPrice*chaseThreshold {
			logger.Debugf("[PENDING] %s => price moved too far: %.4f > %.4f (chaseThreshold=%.3f)",
				pair, currentPrice, pb.TriggerPrice*chaseThreshold, chaseThreshold)
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
		if bias.Direction == "UP" && bias.Strength > 0.70 && indicators.RsiSlope >= 0 && indicators.HistSlope > -0.00005 {
			momentumOK = true
		}
		logger.Debugf("[PENDING MOMO] %s breakoutOK=%t accept=%t momentumOK=%t macdStrong=%t rsiHealthy=%t hist=%.6f rsislp=%.6f bias=%.2f",
			pair, breakoutOK, accept, momentumOK, macdStrong, rsiHealthy, indicators.HistSlope, indicators.RsiSlope, bias.Strength)

		if pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending {
			if microBreakoutOK() || momentumOK {
				breakoutOK = true
				accept = true
			}
		}

		if !((breakoutOK && accept) || momentumOK) {
			logger.DebugColorf(
				logger.BrightBlack,
				"[PENDING ACTIVATE] %s => activation failed (breakoutOK=%t accept=%t momentumOK=%t)",
				pair, breakoutOK, accept, momentumOK,
			)
			return false
		}

		maxExtension := 1.012
		if pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending {
			maxExtension = 1.018
			if bias.Direction == "UP" && bias.Strength > 0.70 {
				maxExtension += 0.002
			}
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
			volFloor := 0.70
			if bias.Direction == "UP" && bias.Strength > 0.65 {
				volFloor = 0.65
			}
			if last.Volume < avgVol*volFloor {
				logger.Debugf("[PENDING] %s => low volume: %.2f < %.2f",
					pair, last.Volume, avgVol*volFloor)
				return false
			}
		}

		ok := cs.checkBullishConditions(pb.MarketState, indicators, currentPrice, pair)
		logger.DebugColorf(
			logger.BrightBlack,
			"[PENDING ACTIVATE] %s => bullishRecipe=%t state=%s",
			pair, ok, pb.MarketState.String(),
		)
		return ok
	}

	if confirmed := repo.Confirm(pair, ok); confirmed != nil {
		logger.InfoColorf(
			logger.Blue,
			"[PENDING BUY CONFIRM CANDIDATE] %s => id=%d age=%.1fm score=%.2f quality=%.2f",
			pair,
			confirmed.ID,
			time.Since(confirmed.TriggerTime).Minutes(),
			confirmed.ConfidenceScore,
			confirmed.GetTrendQuality(),
		)

		if cs.isOscillating(indicators) {
			logger.Warnf("[PENDING BUY ABORTED] %s => oscillation detected right before buy (score=%.2f)",
				pair, oscScore)
			return 0
		}

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

	if len(all) > 0 {
		logger.Debugf("[PENDING STATUS] %s count=%d bias=%s/%.2f latestAge=%.1fm",
			pair, len(all), bias.Direction, bias.Strength, time.Since(all[len(all)-1].TriggerTime).Minutes())
	}

	return 0
}

func (cs *CompoundStrategy) shouldSkipLongDueToChop(
	state models.MarketState,
	ci CurrentIndicators,
	pair string,
) (bool, string) {

	osc := cs.oscillationScore(ci)

	if ci.NoiseLevel >= 0.80 {
		return true, fmt.Sprintf("noise=%.2f >= 0.80 (extreme noise)", ci.NoiseLevel)
	}

	if ci.MarketRegime == models.RangeBoundRegime &&
		ci.VolatilityRegime == models.HighVolatilityRegime &&
		osc >= 1.4 {
		return true, fmt.Sprintf("range+highVol chop: osc=%.2f, noise=%.2f", osc, ci.NoiseLevel)
	}

	if (state == models.Chaotic || state == models.RangeBound) && osc >= 1.5 {
		return true, fmt.Sprintf("state=%s too choppy: osc=%.2f", state.String(), osc)
	}

	if ci.ADX < 12 && ci.NoiseLevel > 0.65 && osc >= 1.3 {
		return true, fmt.Sprintf("weak trend ADX=%.1f, noise=%.2f, osc=%.2f", ci.ADX, ci.NoiseLevel, osc)
	}

	return false, ""
}

func (cs *CompoundStrategy) passesHTFBullishGate(
	state models.MarketState,
	ci CurrentIndicators,
	pair string,
) (bool, string) {

	if cs.Analyzer == nil {
		return true, ""
	}

	htf := ci.MarketStateHTF
	regime := ci.MarketRegime
	multi := ci.MultiTFTrendScore

	if htf == models.Chaotic && multi < 0.40 && ci.NoiseLevel > 0.60 {
		return false, fmt.Sprintf("HTF chaotic, mTf=%.2f, noise=%.2f", multi, ci.NoiseLevel)
	}

	if state == models.StronglyTrending || state == models.Trending {
		if (htf == models.RangeBound || htf == models.Chaotic) && multi < 0.45 {
			return false, fmt.Sprintf("HTF=%s, mTf=%.2f < 0.45 (no trend alignment)", htf.String(), multi)
		}

		if regime == models.RangeBoundRegime && multi < 0.50 {
			return false, fmt.Sprintf("regime=RangeBound, mTf=%.2f < 0.50", multi)
		}

		if !ci.HasBullishStructure && multi < 0.55 {
			return false, fmt.Sprintf("no bullish structure & mTf=%.2f < 0.55", multi)
		}
	}

	if state == models.RangeBound || state == models.Transitional {
		if htf == models.StronglyTrending || htf == models.Trending {
			if multi < 0.30 {
				return false, fmt.Sprintf("HTF trend strong & mTf=%.2f < 0.30", multi)
			}
		}
	}

	if htf == models.Chaotic && ci.VolumeProfile != models.BalancedProfile && multi < 0.40 {
		return false, fmt.Sprintf("HTF chaotic, non-balanced volume profile, mTf=%.2f", multi)
	}

	return true, ""
}

func (cs *CompoundStrategy) finalBuyValidation(
	indicators CurrentIndicators,
	currentPrice float64,
	pb *PendingBuy,
) bool {

	if cs.isOscillating(indicators) {
		logger.Debugf("[FINAL BUY] %s => rejected: oscillation score too high (score=%.2f, noise=%.2f, regime=%s)",
			pb.Pair, cs.oscillationScore(indicators), indicators.NoiseLevel, indicators.MarketRegime.String())
		return false
	}

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
	case models.RangeBound, models.Transitional, models.Default:
	default:
		logger.Debugf("[FINAL BUY] %s => unknown marketState=%v for overbought; using baseline %.1f", pb.Pair, pb.MarketState, overbought)
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
	case models.Default:
	default:
		logger.Debugf("[FINAL BUY] %s => unknown marketState=%v for stops; using baseline", pb.Pair, pb.MarketState)
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
	case models.Default:
	default:
		logger.Debugf("[FINAL BUY] %s => unknown marketState=%v for minScore; using baseline %.2f", pb.Pair, pb.MarketState, minScore)
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

func (cs *CompoundStrategy) recordEquitySample(symbol string, pnlPercent float64, label string) {
	if cs == nil {
		return
	}
	eg := cs.ensureEquityGuard()
	if eg == nil {
		return
	}
	eg.RecordTrade(symbol, pnlPercent, label)
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

	var maxHoldingTime time.Duration
	var minSoftAge time.Duration

	switch state {
	case models.StronglyTrending:
		maxHoldingTime = 5 * time.Hour
		minSoftAge = 5 * time.Minute
	case models.Trending:
		maxHoldingTime = 4 * time.Hour
		minSoftAge = 4 * time.Minute
	case models.Transitional:
		maxHoldingTime = 90 * time.Minute
		minSoftAge = 3 * time.Minute
	case models.RangeBound:
		maxHoldingTime = 45 * time.Minute
		minSoftAge = 2 * time.Minute
	case models.Chaotic:
		maxHoldingTime = 30 * time.Minute
		minSoftAge = 2 * time.Minute
	default:
		maxHoldingTime = 60 * time.Minute
		minSoftAge = 3 * time.Minute
	}

	if tradeDuration < minSoftAge {
		logger.DebugColorf(
			logger.BrightYellow,
			"[EARLY EXIT DISARMED] %s: age=%v < minSoftAge=%v, PM=%.2f%%, state=%s, momo=%s",
			trade.Symbol, tradeDuration, minSoftAge, profitMargin, state.String(), mstate,
		)
		return false
	}

	lossHard := cs.lossThreshold() // negative number, already enforced as HARD STOP earlier

	if state != models.StronglyTrending && state != models.Trending {

		if profitMargin <= lossHard*0.6 && profitMargin > lossHard && tradeDuration >= maxHoldingTime/2 {
			logger.InfoColorf(
				logger.BrightYellow,
				"[EARLY EXIT] %s: non-trend, slow bleed | age=%v PM=%.2f%% lossHard=%.2f%% state=%s",
				trade.Symbol, tradeDuration, profitMargin, lossHard, state.String(),
			)
			return true
		}

		if tradeDuration >= maxHoldingTime && profitMargin <= 0.1 {
			logger.InfoColorf(
				logger.BrightYellow,
				"[EARLY EXIT] %s: non-trend, stale trade | age=%v PM=%.2f%% state=%s",
				trade.Symbol, tradeDuration, profitMargin, state.String(),
			)
			return true
		}

		logger.DebugColorf(
			logger.BrightYellow,
			"[EARLY EXIT WAIT] %s: non-trend, keep holding | age=%v PM=%.2f%% state=%s",
			trade.Symbol, tradeDuration, profitMargin, state.String(),
		)
		return false
	}

	if profitMargin < 0 && profitMargin > lossHard &&
		(tradeDuration >= maxHoldingTime/3) &&
		(mstate == "DOWN") {

		logger.InfoColorf(
			logger.BrightRed,
			"[EARLY EXIT] %s: failed trend | age=%v PM=%.2f%% lossHard=%.2f%% state=%s momo=%s",
			trade.Symbol, tradeDuration, profitMargin, lossHard, state.String(), mstate,
		)
		return true
	}

	smallWin := profitMargin >= 0 && profitMargin <= 0.5
	flattenedMomentum :=
		mstate != "UP" ||
			cs.localIndicators.HistSlope <= 0 ||
			cs.localIndicators.RsiSlope <= 0

	if smallWin && tradeDuration >= maxHoldingTime && flattenedMomentum {
		logger.InfoColorf(
			logger.BrightYellow,
			"[EARLY EXIT] %s: stalled trend | age=%v PM=%.2f%% state=%s momo=%s hist=%.6f rsiSlope=%.6f",
			trade.Symbol, tradeDuration, profitMargin, state.String(), mstate,
			cs.localIndicators.HistSlope, cs.localIndicators.RsiSlope,
		)
		return true
	}

	logger.DebugColorf(
		logger.BrightYellow,
		"[EARLY EXIT WAIT] %s: trending, no early exit | age=%v PM=%.2f%% state=%s momo=%s",
		trade.Symbol, tradeDuration, profitMargin, state.String(), mstate,
	)
	return false
}

func (cs *CompoundStrategy) lossThreshold() float64 {
	h := cs.HighestPriceFallOffMargin

	if h == 0 {
		return -1.5 // default max loss from ENTRY (in %)
	}
	if h > 0 {
		return -math.Abs(h) // positive config → treat as absolute % loss
	}
	return h // already negative
}

func (cs *CompoundStrategy) athFalloffThreshold() float64 {
	h := cs.HighestPriceFallOffMargin

	if h == 0 {
		return -1.0 // default max giveback from local ATH (in %)
	}
	if h > 0 {
		return -math.Abs(h)
	}
	return h
}

func (cs *CompoundStrategy) checkPanicSellCondition(profitMargin float64) bool {
	if !cs.PanicSell {
		return false
	}
	threshold := cs.lossThreshold() // negative
	return profitMargin <= threshold
}

func (cs *CompoundStrategy) checkAthFallOffSellCondition(
	profitMargin float64,
	profitMarginFromATH float64,
) bool {

	if profitMargin <= 0 {
		return false
	}

	threshold := cs.athFalloffThreshold() // negative, e.g. -1.0 (% from ATH)
	return profitMarginFromATH <= threshold
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
		adjustedProfit *= 1.8
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

func (cs *CompoundStrategy) checkActiveTrade(
	trade *models.ActiveTrade,
	currentPrice float64,
	bearishSignal bool,
	state models.MarketState,
) (int, error) {

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

	lossHard := cs.lossThreshold()       // negative
	fallHard := cs.athFalloffThreshold() // negative
	age := time.Since(trade.Timestamp)

	if pmBuy <= lossHard {
		logger.InfoColorf(
			logger.BrightRed,
			"[HARD STOP EXIT] %s: PM=%.2f%% <= %.2f%% (lossThreshold) | state=%s, age=%v",
			trade.Symbol, pmBuy, lossHard, state.String(), age,
		)
		cs.recordEquitySample(trade.Symbol, pmBuy, "HARD_STOP")
		return -1, nil
	}

	if pmPeak > 0 && pmFromATH <= fallHard {
		logger.InfoColorf(
			logger.BrightRed,
			"[HARD TRAIL EXIT] %s: PM=%.2f%% Peak=%.2f%% DropFromATH=%.2f%% <= %.2f%% (athFalloff) | state=%s, age=%v",
			trade.Symbol, pmBuy, pmPeak, pmFromATH, fallHard, state.String(), age,
		)
		cs.recordEquitySample(trade.Symbol, pmBuy, "HARD_TRAIL")
		return -1, nil
	}

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
	default:
		trailMult = 1.5
	}

	trailingActive := pmBuy > 0 && athPrice > trade.BuyPrice && atr > 0
	trailingStop := 0.0
	if trailingActive {
		trailingStop = athPrice - trailMult*atr

		minTrailTrigger := 0.35 // %
		switch state {
		case models.StronglyTrending:
			minTrailTrigger = 0.60
		case models.Trending:
			minTrailTrigger = 0.45
		case models.Transitional:
			minTrailTrigger = 0.35
		case models.RangeBound:
			minTrailTrigger = 0.30
		case models.Chaotic:
			minTrailTrigger = 0.25
		default:
			minTrailTrigger = 0.35
		}

		if pmBuy >= minTrailTrigger {
			minStop := breakevenPrice * 1.0012
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
		logger.InfoColorf(
			logger.BrightRed,
			"[PANIC EXIT] %s: PM=%.2f%% (panic lossThreshold=%.2f%%)",
			trade.Symbol, pmBuy, cs.lossThreshold(),
		)
		cs.recordEquitySample(trade.Symbol, pmBuy, "PANIC")
		return -1, nil
	}

	if cs.checkEarlyExitCondition(trade, currentPrice, state, mstate) {
		cs.recordEquitySample(trade.Symbol, pmBuy, "EARLY_EXIT")
		return -1, nil
	}

	if trailingActive && trailingStop > 0 && currentPrice <= trailingStop {
		logger.InfoColorf(logger.BrightRed, "[TRAILING STOP EXIT] %s: cp=%.4f <= tsl=%.4f (state=%s, PM=%.2f%%, momo=%s)",
			trade.Symbol, currentPrice, trailingStop, state.String(), pmBuy, mstate)

		if (state == models.StronglyTrending || state == models.Trending) && cs.sinceTrailExit(trade.Symbol) > 3*time.Minute {
			cs.enqueueTrendReentry(trade.Symbol, currentPrice, state)
			cs.touchTrailExit(trade.Symbol)
		}
		cs.recordEquitySample(trade.Symbol, pmBuy, "TRAIL_TSL")
		return -1, nil
	}

	if pmBuy > 0 && athPrice > trade.BuyPrice {
		if cs.checkAthFallOffSellCondition(pmBuy, pmFromATH) {
			logger.InfoColorf(
				logger.BrightRed,
				"[SOFT TRAIL EXIT] %s: PM=%.2f%%, dropFromATH=%.2f%% (limit=%.2f%%, state=%s, momo=%s)",
				trade.Symbol,
				pmBuy,
				pmFromATH,
				-cs.athFalloffThreshold(),
				state.String(),
				mstate,
			)
			cs.recordEquitySample(trade.Symbol, pmBuy, "SOFT_TRAIL")
			return -1, nil
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
	default:
		g2rArm = 0.45
	}
	beBuf := 0.0012
	if pmPeak >= g2rArm && currentPrice <= breakevenPrice*(1.0+beBuf) {
		logger.InfoColorf(logger.BrightRed, "[G2R LOCK EXIT] %s: pmPeak=%.2f%%, cp near/under BE", trade.Symbol, pmPeak)
		cs.recordEquitySample(trade.Symbol, pmBuy, "G2R_LOCK")
		return -1, nil
	}

	if cs.checkTimeSinceSellCondition(state, trade.Symbol, pmBuy, lastAthTime) {
		isTrend := state == models.StronglyTrending || state == models.Trending
		if !isTrend || (cs.localIndicators.HistSlope <= 0 || cs.localIndicators.RsiSlope <= 0 || mstate != "UP") {
			cs.recordEquitySample(trade.Symbol, pmBuy, "ATH_STALE")
			return -1, nil
		}
	}

	repo := cs.ensureRepo()
	bias := repo.GetTrendBias(trade.Symbol)

	if cs.checkBearishSignalSellCondition(pmBuy, bearishSignal, state) {
		isTrend := state == models.StronglyTrending || state == models.Trending
		strongUpSupport := isTrend && bias.Direction == "UP" && bias.Strength >= 0.65 && mstate == "UP"

		switch {
		case !isTrend:
			logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: Non-trend state=%s, PM=%.2f%%, momo=%s",
				trade.Symbol, state.String(), pmBuy, mstate)
			cs.recordEquitySample(trade.Symbol, pmBuy, "BEARISH_EXIT_NON_TREND")
			return -1, nil
		case mstate == "DOWN":
			logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: Trend %s, momo=DOWN, PM=%.2f%%",
				trade.Symbol, state.String(), pmBuy)
			cs.recordEquitySample(trade.Symbol, pmBuy, "BEARISH_EXIT_DOWN")
			return -1, nil
		case mstate == "NEUTRAL":
			if cs.localIndicators.MacdLine < cs.localIndicators.SignalLine &&
				cs.localIndicators.HistSlope < 0 &&
				!strongUpSupport {
				logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: Trend %s, momo=NEUTRAL (confirm), PM=%.2f%%",
					trade.Symbol, state.String(), pmBuy)
				cs.recordEquitySample(trade.Symbol, pmBuy, "BEARISH_EXIT_NEUTRAL")
				return -1, nil
			}
		}
	}

	if cs.trendDecayExit(trade, currentPrice, state, mstate, bias, pmBuy) {
		cs.recordEquitySample(trade.Symbol, pmBuy, "TREND_DECAY")
		return -1, nil
	}

	if currentPrice < breakevenPrice {
		logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven, PM=%.2f%%", trade.Symbol, pmBuy)
		return 0, nil
	}

	if cs.DesiredProfit > 0 {
		target := cs.PartialTP1Pct
		nearUpper := cs.localIndicators.UpperBand > 0 && currentPrice >= cs.localIndicators.UpperBand*0.999
		momoCooling := cs.localIndicators.HistSlope <= 0.0 || cs.localIndicators.RsiSlope <= 0.0 ||
			cs.localIndicators.MacdLine <= cs.localIndicators.SignalLine
		if pmBuy >= target && (momoCooling || nearUpper) {
			logger.InfoColorf(logger.BrightBlack, "[SCALP EXIT] %s: PM=%.2f%% (target=%.2f%%), cooling=%t, nearUpper=%t",
				trade.Symbol, pmBuy, target, momoCooling, nearUpper)
			cs.recordEquitySample(trade.Symbol, pmBuy, "SCALP_EXIT")
			return -2, nil
		}
	}

	if met, adjusted := cs.checkDesiredProfitSellCondition(pmBuy, state); met {
		logger.InfoColorf(logger.BrightBlack, "[PROFIT SELL] %s: PM=%.2f%% vs target %.2f%% (momo=%s)",
			trade.Symbol, pmBuy, adjusted, mstate)
		cs.recordEquitySample(trade.Symbol, pmBuy, "PROFIT_EXIT")
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

func (cs *CompoundStrategy) trendDecayExit(
	trade *models.ActiveTrade,
	currentPrice float64,
	state models.MarketState,
	mstate string,
	bias TrendBias,
	pmBuy float64,
) bool {
	age := time.Since(trade.Timestamp)

	if age < 5*time.Minute {
		return false
	}

	ci := cs.localIndicators

	flatHist := math.Abs(ci.HistSlope) < 0.00003
	flatRSI := math.Abs(ci.RsiSlope) < 0.0005
	flatEMA := math.Abs(ci.EMASlope20) < 0.0001 && math.Abs(ci.EMASlope50) < 0.0001
	flatKC := math.Abs(ci.KCSlope) < 0.00005

	flattened := flatHist && flatRSI && (flatEMA || flatKC)

	macdBear := ci.MacdLine <= ci.SignalLine
	histDown := ci.HistSlope < 0
	rsiDown := ci.RsiSlope < 0
	biasDown := bias.Direction == "DOWN" && bias.Strength > 0.55

	turningDown := (macdBear && (histDown || rsiDown)) ||
		biasDown ||
		mstate == "DOWN"

	var posAgeLimit, negAgeLimit time.Duration
	switch state {
	case models.StronglyTrending:
		posAgeLimit = 35 * time.Minute
		negAgeLimit = 20 * time.Minute
	case models.Trending:
		posAgeLimit = 25 * time.Minute
		negAgeLimit = 15 * time.Minute
	case models.Transitional:
		posAgeLimit = 20 * time.Minute
		negAgeLimit = 12 * time.Minute
	case models.RangeBound:
		posAgeLimit = 15 * time.Minute
		negAgeLimit = 10 * time.Minute
	case models.Chaotic:
		posAgeLimit = 10 * time.Minute
		negAgeLimit = 8 * time.Minute
	default:
		posAgeLimit = 20 * time.Minute
		negAgeLimit = 12 * time.Minute
	}

	if pmBuy < 0 && age >= negAgeLimit && (turningDown || flattened) {
		logger.InfoColorf(
			logger.BrightRed,
			"[TREND DECAY EXIT] %s: NEGATIVE & trend gone | Age=%v PM=%.2f%% Bias=%s/%.2f Momo=%s",
			trade.Symbol, age, pmBuy, bias.Direction, bias.Strength, mstate,
		)
		return true
	}

	smallWin := pmBuy >= 0 && pmBuy <= 0.5
	strongUpSupport := bias.Direction == "UP" && bias.Strength >= 0.70 && mstate == "UP"

	if smallWin && age >= posAgeLimit && (flattened || turningDown) && !strongUpSupport {
		logger.InfoColorf(
			logger.BrightYellow,
			"[STALL EXIT] %s: flat small win | Age=%v PM=%.2f%% Bias=%s/%.2f Momo=%s",
			trade.Symbol, age, pmBuy, bias.Direction, bias.Strength, mstate,
		)
		return true
	}

	return false
}

func (cs *CompoundStrategy) oscillationScore(ci CurrentIndicators) float64 {
	score := 0.0

	if ci.NoiseLevel > 0.75 {
		score += 1.0
	} else if ci.NoiseLevel > 0.60 {
		score += 0.6
	} else if ci.NoiseLevel > 0.50 {
		score += 0.3
	}

	if ci.MarketRegime == models.RangeBoundRegime &&
		ci.VolatilityRegime == models.HighVolatilityRegime {
		score += 0.9
	} else if ci.MarketRegime == models.RangeBoundRegime {
		score += 0.5
	}

	if ci.ADX < 12 {
		score += 0.7
	} else if ci.ADX < 16 {
		score += 0.4
	}

	if (ci.HistSlope > 0 && ci.RsiSlope < 0) || (ci.HistSlope < 0 && ci.RsiSlope > 0) {
		score += 0.6
	}

	c := ci.CandleSticks
	n := len(c)
	if n >= 6 {
		const window = 6
		start := n - window
		if start < 0 {
			start = 0
		}

		altCnt := 0
		dojiCnt := 0

		var lastSign int
		for i := start; i < n; i++ {
			body := c[i].Close - c[i].Open
			rng := c[i].High - c[i].Low
			if rng <= 0 {
				continue
			}
			bodyFrac := math.Abs(body) / rng

			sign := 0
			switch {
			case body > 0:
				sign = 1
			case body < 0:
				sign = -1
			}

			if bodyFrac < 0.25 {
				dojiCnt++
			}

			if lastSign != 0 && sign != 0 && sign != lastSign {
				altCnt++
			}
			if sign != 0 {
				lastSign = sign
			}
		}

		if altCnt >= 3 {
			score += 0.8
		} else if altCnt >= 2 {
			score += 0.5
		}

		if dojiCnt >= 3 {
			score += 0.6
		} else if dojiCnt >= 2 {
			score += 0.3
		}
	}

	return score
}

func (cs *CompoundStrategy) isOscillating(ci CurrentIndicators) bool {
	return cs.oscillationScore(ci) >= 1.6
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

	ichiRes, errIchi := cs.Ichimoku.Calculate(candles)
	if errIchi != nil {
		logger.Warnf("[ICHIMOKU MISS] %s err=%v", pair, errIchi)
		return CurrentIndicators{}, fmt.Errorf("ichimoku: %w", errIchi)
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

	var adrVal float64
	var adrSig int
	if cs.ADR != nil {
		var err6 error
		adrVal, adrSig, err6 = cs.ADR.Calculate(candles, pair)
		if err6 != nil {
			logger.Warnf("[ADR MISS] %s err=%v", pair, err6)
			return CurrentIndicators{}, fmt.Errorf("ADR: %w", err6)
		}
	} else {

		adrVal, adrSig = 0, 0
		logger.Warnf("[ADR NIL] %s => ADR strategy is nil, using adrVal=0", pair)
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
		} else {
			logger.Debugf("[KELTNER MISS] %s err=%v", pair, err)
		}
	}

	var (
		marketStateHTF     = models.Default
		marketRegime       = models.UnknownRegime
		volRegime          = models.NormalVolatilityRegime
		multiTFScore       = 0.5
		priceActionQuality = 0.5
		momentumQuality    = 0.5
		noiseLevel         = 0.0
		volumeProfile      = models.BalancedProfile
		hasBullishStruct   bool
		hasBearishDiv      bool
	)

	if cs.Analyzer != nil {
		marketStateHTF, _, _ = cs.Analyzer.AnalyzeMarket(pair, candles)

		marketRegime = cs.Analyzer.DetectMarketRegime(candles)
		volRegime = cs.Analyzer.DetectVolatilityRegime(candles)

		priceActionQuality = cs.Analyzer.AssessPriceActionQuality(candles)
		momentumQuality = cs.Analyzer.AssessMomentumQuality(candles)
		noiseLevel = cs.Analyzer.CalculateNoiseLevel(candles)

		multiTFScore = cs.Analyzer.SimulateMultiTimeframeAnalysis(candles)
		volumeProfile = cs.Analyzer.AnalyzeVolumeProfile(candles)

		structScore := cs.Analyzer.AssessPriceStructure(candles)
		hasBullishStruct = structScore >= 0.60

		hasBearishDiv = cs.Analyzer.IsDivergencePresent(candles)
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

		StochasticK:  stochK,
		StochasticD:  stochD,
		LowerBand:    lowB,
		MiddleBand:   midB,
		UpperBand:    upB,
		BandwidthPct: bwPct,

		CCIVal:    cciVal,
		CCISignal: cciSig,
		MFIVal:    mfiVal,
		MFiSignal: mfiSig,

		ADRVal:    adrVal,
		ADRSignal: adrSig,

		CandleSticks: candles,
		IchimokuRes:  ichiRes,

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

		MarketStateHTF:      marketStateHTF,
		MarketRegime:        marketRegime,
		VolatilityRegime:    volRegime,
		MultiTFTrendScore:   multiTFScore,
		PriceActionQuality:  priceActionQuality,
		MomentumQuality:     momentumQuality,
		NoiseLevel:          noiseLevel,
		VolumeProfile:       volumeProfile,
		HasBullishStructure: hasBullishStruct,
		HasBearishDiv:       hasBearishDiv,
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
		ADR                       *algos.ADRStrategy          `json:"adr"`
		Keltner                   *algos.KeltnerChannel       `json:"keltnerChannel"`
		ADX                       *algos.ADXStrategy          `json:"adx"`
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
	cs.ADR = aux.ADR
	cs.Keltner = aux.Keltner
	cs.ADX = aux.ADX
	cs.MarketState = aux.MarketState
	cs.RiskRewardThreshold = aux.RiskRewardThreshold
	cs.FeeRate = aux.FeeRate
	cs.DesiredProfit = aux.DesiredProfit
	cs.HighestPriceFallOffMargin = aux.HighestPriceFallOffMargin
	cs.CandleInterval = aux.CandleInterval
	cs.PanicSell = aux.PanicSell
	cs.SellOnBearish = aux.SellOnBearish

	if cs.ADR == nil {
		cs.ADR = &algos.ADRStrategy{
			Period:     14,
			Multiplier: 1.0,
		}
	}

	return nil
}

func (cs *CompoundStrategy) SetAnalyzer(ma *analysis.MarketAnalyzer) {
	cs.Analyzer = ma
}

func (cs *CompoundStrategy) Clone() interfaces.Strategy {
	require := func(name string, v any) {
		if v == nil {
			log.Panicf("Clone: %s is nil", name)
		}
	}

	require("ADR", cs.ADR)
	require("RSI", cs.RSI)
	require("MACD", cs.MACD)
	require("Stochastic", cs.Stochastic)
	require("BollingerBands", cs.BollingerBands)
	require("Ichimoku", cs.Ichimoku)
	require("CCI", cs.CCI)
	require("MFI", cs.MFI)

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
		Keltner: func() *algos.KeltnerChannel {
			if cs.Keltner == nil {
				return nil
			}
			return &algos.KeltnerChannel{Period: cs.Keltner.Period, Multiplier: cs.Keltner.Multiplier}
		}(),
		ADX: func() *algos.ADXStrategy {
			if cs.ADX == nil {
				return nil
			}
			return &algos.ADXStrategy{Period: cs.ADX.Period}
		}(),
		MarketState:               cs.MarketState,
		RiskRewardThreshold:       cs.RiskRewardThreshold,
		FeeRate:                   cs.FeeRate,
		DesiredProfit:             cs.DesiredProfit,
		HighestPriceFallOffMargin: cs.HighestPriceFallOffMargin,
		CandleInterval:            cs.CandleInterval,
		PanicSell:                 cs.PanicSell,
		SellOnBearish:             cs.SellOnBearish,
		Analyzer:                  cs.Analyzer,
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
	return emaNow - emaPrev
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

func (cs *CompoundStrategy) logCompactState(pair string, state models.MarketState, price float64, ci CurrentIndicators, bias TrendBias, pendingCount int, trade *models.ActiveTrade, bull, bear bool) {
	width := ci.UpperBand - ci.LowerBand
	pos := 0.5
	if width > 0 {
		pos = clamp01((price - ci.LowerBand) / width)
	}

	logger.Debugf(
		"[COMPACT] pair=%s state=%s price=%.5f rsi=%.1f rsislp=%.5f macd=%.5f/%.5f hslp=%.6f ichiB=%t ichiBr=%t adx=%.1f +di=%.1f -di=%.1f bbPos=%.2f kcPos=%.2f bias=%s/%.2f pend=%d bull=%t bear=%t active=%t",
		pair, state.String(), price, ci.RSIVal, ci.RsiSlope, ci.MacdLine, ci.SignalLine, ci.HistSlope,
		ci.IchimokuRes.Bullish, ci.IchimokuRes.Bearish, ci.ADX, ci.PlusDI, ci.MinusDI, pos, ci.KCPos,
		bias.Direction, bias.Strength, pendingCount, bull, bear, trade != nil,
	)
}

func closesOf(c []models.CandleStick) []float64 {
	out := make([]float64, 0, len(c))
	for _, k := range c {
		out = append(out, k.Close)
	}
	return out
}
