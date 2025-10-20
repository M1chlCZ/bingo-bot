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
	RSIVal        float64
	Histogram     float64
	PrevHistogram float64
	HistSlope     float64
	PrevRSI       float64
	RsiSlope      float64
	SignalLine    float64
	MacdLine      float64
	MacdIndicator int
	StochasticK   float64
	StochasticD   float64
	LowerBand     float64
	MiddleBand    float64
	UpperBand     float64
	BandwidthPct  float64
	IchimokuRes   algos.IchimokuResult
	CCIVal        float64
	CCISignal     int
	MFIVal        float64
	MFiSignal     int
	ADRVal        float64
	ADRSignal     int
	CandleSticks  []models.CandleStick
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

		// Derive extra fields for scoring
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

		// ATR: primary from algos.ATR, fallback to ADRVal
		atr := 0.0
		if v, err := algos.ATR(currentIndicators.CandleSticks, 14); err == nil {
			atr = v
		} else {
			atr = currentIndicators.ADRVal
		}

		// Lightweight confidence estimator (0..1)
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

// calculateDynamicRiskReward calculates adjusted RR based on volatility (ATR%)
func (cs *CompoundStrategy) calculateDynamicRiskReward(ci CurrentIndicators) float64 {
	cp := 0.0
	if n := len(ci.CandleSticks); n > 0 {
		cp = ci.CandleSticks[n-1].Close
	}
	atrAbs, err := algos.ATR(ci.CandleSticks, 14)
	var atrPct float64
	if err == nil && cp > 0 {
		atrPct = atrAbs / cp
	} else if ci.MiddleBand > 0 {
		atrPct = ci.ADRVal / ci.MiddleBand
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

var pairCooldownMu sync.Map

// isInCooldownPeriod checks if we're in a cooldown period after a losing trade
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

// ---------- Entry scoring & helpers ----------

func (cs *CompoundStrategy) entryScore(ci CurrentIndicators, currentPrice float64) float64 {
	score := 0.0
	// Trend/confluence
	if ci.ADRSignal > 0 {
		score += 0.5
	}
	if ci.MacdIndicator == 1 {
		score += 0.35
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
	// Not extended/overbought on entry
	if currentPrice <= ci.MiddleBand*1.01 {
		score += 0.10
	}
	// Healthy momentum/volume proxies
	if ci.RSIVal > 35 && ci.RSIVal < 65 {
		score += 0.10
	}
	if ci.MFIVal > 30 && ci.MFIVal < 70 {
		score += 0.03
	}
	if ci.CCIVal > -100 && ci.CCIVal < 100 {
		score += 0.02
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

// scalePendingCooldown: make confirmation wait shorter in strong trends and longer in chaos/range
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

// ---------- Bullish checks ----------
func (cs *CompoundStrategy) checkBullishConditions(
	state models.MarketState,
	ci CurrentIndicators,
	currentPrice float64,
	pair string,
) bool {
	// Dynamic RR
	dynRR := cs.calculateDynamicRiskReward(ci)

	mfiOK := ci.MFIVal > float64(cs.MFI.Oversold) &&
		ci.MFIVal < float64(cs.MFI.Overbought)
	cciOK := ci.CCIVal < cs.CCI.Overbought &&
		ci.CCIVal > cs.CCI.Oversold
	volumeOk := mfiOK || cciOK

	emaAlignment := ci.MacdLine > ci.SignalLine

	// cooldown after loss?
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

func (cs *CompoundStrategy) checkBullishStronglyTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool, emaUp bool,
	state models.MarketState,
) bool {
	score := cs.entryScore(ci, currentPrice)
	if !(emaUp && score >= 0.60 && ci.HistSlope > 0) {
		return false
	}

	atr, err := algos.ATR(ci.CandleSticks, 14)
	if err != nil || atr <= 0 {
		atr = math.Max(1e-9, ci.MiddleBand*0.01)
	}

	pullbackEntry := currentPrice <= ci.MiddleBand*1.02 && currentPrice >= ci.MiddleBand*0.98
	momentumEntry := ci.MacdLine > ci.SignalLine*1.05 && ci.RSIVal > 45

	if currentPrice > ci.UpperBand+0.5*atr {
		return false
	}

	if !pullbackEntry && !momentumEntry {
		return false
	}

	stop := currentPrice - 1.8*atr
	target := math.Max(ci.UpperBand*1.02, currentPrice+2.2*atr)
	rr := cs.calcRR(state, currentPrice, stop, target)

	return volCCIok && rr > (dynamicRR*0.85)
}

func (cs *CompoundStrategy) checkBullishTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool,
	state models.MarketState,
) bool {
	score := cs.entryScore(ci, currentPrice)
	if !(score >= 0.62 && ci.HistSlope > 0) {
		return false
	}

	atr, err := algos.ATR(ci.CandleSticks, 14)
	if err != nil || atr <= 0 {
		atr = math.Max(1e-9, ci.MiddleBand*0.01)
	}
	target := ci.UpperBand
	stop := currentPrice - 2.1*atr
	rr := cs.calcRR(state, currentPrice, stop, target)

	return volCCIok && rr > dynamicRR
}

func (cs *CompoundStrategy) checkBullishRangeBound(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	state models.MarketState,
) bool {
	// Enter near lower band with oversold conditions; require low-ish volatility confirmation (ADR green)
	lowBounce := cs.nearLowerBand(ci, currentPrice) &&
		(ci.RSIVal < 38 || ci.CCIVal <= cs.CCI.Oversold || ci.StochasticK < 25)
	adrConfirmation := ci.ADRSignal == 1
	if !(lowBounce && adrConfirmation) {
		return false
	}
	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	return rr > dynamicRR
}

func (cs *CompoundStrategy) checkBullishTransitional(ci CurrentIndicators, currentPrice, dynamicRR float64, emaAlignment bool, state models.MarketState) bool {
	score := cs.entryScore(ci, currentPrice)
	// Require reclaim of middle band and positive momentum
	if !(emaAlignment && score >= 0.65 && ci.HistSlope > 0 && currentPrice >= ci.MiddleBand*0.995) {
		return false
	}
	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	return rr > dynamicRR
}

func (cs *CompoundStrategy) checkBullishChaotic(ci CurrentIndicators, currentPrice, dynamicRR float64, state models.MarketState) bool {
	// In chaos, be very picky and take quicker RR
	score := cs.entryScore(ci, currentPrice)
	if !(score >= 0.72 && ci.HistSlope > 0 && ci.RSIVal < 62) {
		return false
	}

	atr, err := algos.ATR(ci.CandleSticks, 14)
	if err != nil || atr <= 0 {
		atr = math.Max(1e-9, ci.MiddleBand*0.015)
	}
	target := ci.UpperBand
	stop := currentPrice - 2.3*atr
	rr := cs.calcRR(state, currentPrice, stop, target)
	return rr > (dynamicRR * 1.10) // stricter
}

func (cs *CompoundStrategy) checkBullishConditionsDefault(state models.MarketState, ci CurrentIndicators, currentPrice, dynamicRR float64, volumeOk bool) bool {
	score := cs.entryScore(ci, currentPrice)
	if !(score >= 0.60 && ci.HistSlope > 0) {
		return false
	}
	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	return volumeOk && rr > (dynamicRR*0.95)
}

// ---------- Bearish checks (exits/shorts) ----------

func (cs *CompoundStrategy) checkBearishStronglyTrending(ci CurrentIndicators, currentPrice float64) bool {
	// Exit/short only on strong reversal hints
	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > 80 && currentPrice > ci.MiddleBand
}

func (cs *CompoundStrategy) checkBearishTrending(ci CurrentIndicators) bool {
	// Require down cross + decreasing momentum
	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > float64(cs.RSI.Overbought)
}

func (cs *CompoundStrategy) checkBearishRangeBound(ci CurrentIndicators, currentPrice float64) bool {
	// Fade tops in a range
	return currentPrice >= ci.UpperBand || ci.RSIVal > 70 || ci.CCIVal > cs.CCI.Overbought
}

func (cs *CompoundStrategy) checkBearishChaotic(ci CurrentIndicators, currentPrice float64) bool {
	// Only short sharp spikes
	return currentPrice >= ci.UpperBand && ci.RSIVal > 72 && ci.HistSlope < 0
}

func (cs *CompoundStrategy) checkBearishTransitional(ci CurrentIndicators) bool {
	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > 55 && ci.MFIVal > 70
}

func (cs *CompoundStrategy) checkBearishDefault(ci CurrentIndicators) bool {
	return ci.MacdIndicator == -1 && ci.HistSlope < 0 && ci.RSIVal > float64(cs.RSI.Overbought)
}

// checkBearishConditions decides if we have a 'bearish' (short/exit) signal
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

	// --- NOVÉ: pomocná mikro-breakout logika ---
	microBreakoutOK := func() bool {
		c := indicators.CandleSticks
		n := len(c)
		if n < 3 {
			return false
		}
		// vyšší low + proražení krátkého high o malý buffer dle ATR
		atr := 0.0
		if v, err := algos.ATR(c, 14); err == nil {
			atr = v
		} else {
			atr = indicators.ADRVal
		}
		h1 := c[n-1].High
		h2 := c[n-2].High
		l1 := c[n-1].Low
		l2 := c[n-2].Low
		hh := math.Max(h1, h2)
		hl := l1 > l2
		minBO := hh + 0.15*atr
		return hl && currentPrice >= minBO && indicators.MacdLine > indicators.SignalLine && indicators.HistSlope > 0
	}

	ok := func(pb *PendingBuy) bool {
		// Dynamic cooldown
		if pendingCoolDown > 0 && time.Since(pb.TriggerTime) < pendingCoolDown {
			return false
		}

		// Adaptive chase prevention based on market state
		chaseThreshold := 1.03
		if pb.MarketState == models.StronglyTrending {
			chaseThreshold = 1.04 // Allow more chase in strong trends
		} else if pb.MarketState == models.Chaotic {
			chaseThreshold = 1.02 // Stricter in chaos
		}

		if pb.TriggerPrice > 0 && currentPrice > pb.TriggerPrice*chaseThreshold {
			return false
		}

		// Improved breakout detection
		atr := pb.ATR
		if atr <= 0 {
			if v, err := algos.ATR(indicators.CandleSticks, 14); err == nil {
				atr = v
			} else {
				atr = indicators.ADRVal
			}
		}

		// More lenient breakout requirement
		minBreakout := math.Max(pb.TriggerPrice*1.0003, pb.TriggerPrice+0.05*atr)
		breakoutOK := currentPrice >= minBreakout

		// Enhanced momentum check
		macdStrength := indicators.MacdLine > indicators.SignalLine*1.02
		rsiHealthy := indicators.RSIVal > 35 && indicators.RSIVal < 68
		momentumOK := (macdStrength && indicators.HistSlope > 0) ||
			(indicators.RsiSlope > 0 && rsiHealthy)

		if pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending {
			if microBreakoutOK() {
				breakoutOK = true
			}
		}

		// More lenient confirmation - either condition can pass
		if !breakoutOK && !momentumOK {
			return false
		}

		// Relaxed overextension check for trending states
		maxExtension := 1.01
		if pb.MarketState == models.StronglyTrending || pb.MarketState == models.Trending {
			maxExtension = 1.015
		}
		if currentPrice >= indicators.UpperBand*maxExtension {
			return false
		}

		return cs.checkBullishConditions(pb.MarketState, indicators, currentPrice, pair)
	}

	if confirmed := repo.Confirm(pair, ok); confirmed != nil {
		if !cs.finalEntrySanity(indicators) {
			logger.Warnf("[PENDING BUY ABORTED] %s => final sanity check failed", pair)
			return 0
		}
		logger.InfoColorf(
			logger.Blue,
			"[PENDING BUY CONFIRMED] %s => Buying at %.4f (trigger %.4f, state %s, age %.1fm)",
			confirmed.Pair, currentPrice, confirmed.TriggerPrice, confirmed.MarketState.String(),
			time.Since(confirmed.TriggerTime).Minutes(),
		)
		return 1
	}

	// Improved cancellation logic
	all := repo.GetAll(pair)
	removed := 0
	for _, pb := range all {
		shouldCancel := false
		reason := ""

		// State-specific cancellation
		switch pb.MarketState {
		case models.StronglyTrending:
			if currentPrice > pb.TriggerPrice*1.045 {
				shouldCancel = true
				reason = "price moved too far in strong trend"
			}
		case models.Chaotic:
			if currentPrice > pb.TriggerPrice*1.025 || time.Since(pb.TriggerTime) > 5*time.Minute {
				shouldCancel = true
				reason = "chaos opportunity expired"
			}
		default:
			if currentPrice > pb.TriggerPrice*1.03 {
				shouldCancel = true
				reason = "standard chase limit exceeded"
			}
		}

		if shouldCancel && repo.Remove(pair, pb.ID) {
			logger.Warnf("[PENDING BUY CANCELLED] %s => %s (%.4f -> %.4f)",
				pair, reason, pb.TriggerPrice, currentPrice)
			removed++
			continue
		}

		// Regime change cancellation
		if !cs.checkBullishConditions(pb.MarketState, indicators, currentPrice, pair) {
			if repo.Remove(pair, pb.ID) {
				logger.Infof("[PENDING BUY CANCELLED] %s => conditions changed", pair)
				removed++
			}
		}
	}

	if removed > 0 {
		logger.Debugf("[PENDING BUY] %s => pruned %d entries", pair, removed)
	}

	return 0
}

// Enhanced finalEntrySanity with more nuanced pattern detection
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

		// Reject obvious shooting stars
		if upperFrac >= 0.58 && bodyFrac <= 0.25 && current.Close < current.Open {
			return false
		}

		// Reject strong bearish engulfing without support
		isPrevBullish := previous.Close > previous.Open
		isCurrentBearish := current.Close < current.Open
		if isPrevBullish && isCurrentBearish && bodyFrac > 0.55 && lowerFrac < 0.15 {
			prevBody := math.Abs(previous.Close - previous.Open)
			if body > prevBody*1.2 {
				return false
			}
		}

		// Accept bullish patterns
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
	// kept for potential further use; main confirmation moved into evaluatePendingBuys.ok + finalEntrySanity
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
	// Volume confirmation
	if indicators.MFIVal > 20 && indicators.MFIVal < 80 {
		confirmations++
	}
	// MACD confirmation
	if indicators.MacdLine > indicators.SignalLine && indicators.HistSlope > 0 {
		confirmations++
	}
	// Price action confirmation (not overextended)
	if currentPrice > indicators.LowerBand && currentPrice <= indicators.MiddleBand*1.02 {
		confirmations++
	}
	// Micro-structure: higher low and small breakout over recent highs
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

// trackPriceExtremes updates and returns the all-time high and low prices for a trade
func (cs *CompoundStrategy) trackPriceExtremes(symbol string, currentPrice float64) (athPrice, atlPrice float64, lastAthTime time.Time, err error) {
	// HIGH PRICE since trade
	athPrice, err = db2.SQLiteDB.GetAth(symbol)
	if err != nil || currentPrice > athPrice {
		logger.InfoColorf(logger.Green, "New HIGH price for %s: %.8f", symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAth(symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATH price for %s: %v", symbol, e)
		}
		athPrice = currentPrice
	}

	// LOW PRICE since trade
	atlPrice, err = db2.SQLiteDB.GetAtl(symbol)
	if err != nil || currentPrice < atlPrice {
		logger.InfoColorf(logger.BrightRed, "New LOW price for %s: %.8f", symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAtl(symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATL price for %s: %v", symbol, e)
		}
		atlPrice = currentPrice
	}

	// Check last time it reached ATH
	lastAthTime, err = db2.SQLiteDB.GetLastATHTimestamp(symbol)
	if err != nil {
		logger.Errorf("Error getting last ATH time for %s: %v", symbol, err)
	}

	return athPrice, atlPrice, lastAthTime, nil
}

func (cs *CompoundStrategy) checkEarlyExitCondition(trade *models.ActiveTrade, currentPrice float64) bool {
	tradeDuration := time.Since(trade.Timestamp)
	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	profitMargin := (currentPrice - breakevenPrice) / breakevenPrice * 100

	// tighten time stop & allow tiny red/flat exits
	const maxHoldingTime = 45 * time.Minute
	const minimalAcceptableLoss = -0.35 // -0.35% under breakeven

	if tradeDuration > maxHoldingTime && profitMargin >= minimalAcceptableLoss {
		logger.InfoColorf(logger.BrightYellow, "[EARLY EXIT] %s: Holding too long (%v), current margin=%.2f%%",
			trade.Symbol, tradeDuration, profitMargin)
		return true
	}
	return false
}

// checkPanicSellCondition: if price dumps from entry too far, bail
func (cs *CompoundStrategy) checkPanicSellCondition(profitMargin float64) bool {
	if !cs.PanicSell {
		return false
	}
	return profitMargin < -cs.HighestPriceFallOffMargin
}

// checkAthFallOffSellCondition checks if we should sell based on drop from ATH
func (cs *CompoundStrategy) checkAthFallOffSellCondition(profitMargin, profitMarginATH float64) bool {
	return profitMarginATH < -cs.HighestPriceFallOffMargin && profitMargin > 0
}

// checkTimeSinceSellCondition checks if we should sell based on time since last ATH
func (cs *CompoundStrategy) checkTimeSinceSellCondition(state models.MarketState, symbol string, profitMargin float64, lastAthTime time.Time) bool {
	if profitMargin < 0 {
		return false
	}
	return cs.getTimeSinceATHSell(symbol, lastAthTime, state) && profitMargin >= 0
}

// Enhanced sell logic
func (cs *CompoundStrategy) checkDesiredProfitSellCondition(profitMargin float64, state models.MarketState) (bool, float64) {
	adjustedProfit := cs.DesiredProfit

	// More aggressive profit scaling
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

// checkActiveTrade evaluates an active trade to determine if it should be sold
func (cs *CompoundStrategy) checkActiveTrade(trade *models.ActiveTrade, currentPrice float64, bearishSignal bool, state models.MarketState) (int, error) {
	if trade == nil || trade.Symbol == "" {
		return 0, nil
	}
	if currentPrice <= 0 || trade.BuyPrice <= 0 {
		logger.InfoColorf(logger.BrightYellow, "[HOLD SAFE] %s: Waiting for valid prices (buy=%.4f, curr=%.4f)", trade.Symbol, trade.BuyPrice, currentPrice)
		return 0, nil
	}
	if time.Since(trade.Timestamp) < 2*time.Minute {
		logger.InfoColorf(logger.BrightBlack, "[GRACE] %s: %.0fs since entry, skipping exits", trade.Symbol, time.Since(trade.Timestamp).Seconds())
		_, _, _, _ = cs.trackPriceExtremes(trade.Symbol, currentPrice)
		return 0, nil
	}

	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

	athPrice, atlPrice, lastAthTime, _ := cs.trackPriceExtremes(trade.Symbol, currentPrice)

	if athPrice <= 0 {
		athPrice = currentPrice
	}
	if atlPrice <= 0 {
		atlPrice = currentPrice
	}

	profitMarginATH := (currentPrice - athPrice) / athPrice * 100
	upliftFromAtl := (currentPrice - atlPrice) / atlPrice * 100

	logger.Infof("[Trade Monitor] %s | State=%s | Buy=%.4f | Curr=%.4f | PM=%.2f%% | ATH=%.4f | PM_ATH=%.2f%%",
		trade.Symbol, state.String(), trade.BuyPrice, currentPrice, profitMargin, athPrice, profitMarginATH)

	atr := 0.0
	if v, err := algos.ATR(cs.localIndicators.CandleSticks, 14); err == nil {
		atr = v
	} else {
		atr = math.Max(1e-9, cs.localIndicators.ADRVal)
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
		trailMult = 1.6
	}

	trailingActive := profitMargin > 0 && athPrice > trade.BuyPrice && atr > 0
	trailingStop := 0.0
	if trailingActive {
		trailingStop = athPrice - trailMult*atr
		minStop := breakevenPrice * 1.001
		if trailingStop < minStop {
			trailingStop = minStop
		}

		ageMin := time.Since(lastAthTime).Minutes()
		switch {
		case ageMin > 60:
			trailingStop = math.Max(trailingStop, currentPrice-1.1*atr)
		case ageMin > 30:
			trailingStop = math.Max(trailingStop, currentPrice-1.4*atr)
		case ageMin > 15:
			trailingStop = math.Max(trailingStop, currentPrice-1.6*atr)
		}

		if (state == models.StronglyTrending || state == models.Trending) && profitMargin >= 1.2 {
			lock := trade.BuyPrice * 1.0035
			if trailingStop < lock {
				trailingStop = lock
			}
		}
	}

	if profitMargin > 0 {
		ageMin := time.Since(lastAthTime).Minutes()
		switch {
		case ageMin > 60:
			trailingStop = math.Max(trailingStop, currentPrice-1.1*atr)
		case ageMin > 30:
			trailingStop = math.Max(trailingStop, currentPrice-1.4*atr)
		case ageMin > 15:
			trailingStop = math.Max(trailingStop, currentPrice-1.6*atr)
		}
	}

	if (state == models.StronglyTrending || state == models.Trending) && profitMargin >= 1.2 {
		lock := trade.BuyPrice * 1.0035
		if trailingStop < lock {
			trailingStop = lock
		}
	}

	if profitMargin < 0 && currentPrice > atlPrice {
		logger.InfoColorf(logger.BrightYellow, "[Above ATL] %s: Uplift %.2f%%", trade.Symbol, upliftFromAtl)
	}

	if cs.checkPanicSellCondition(profitMargin) {
		logger.InfoColorf(logger.BrightRed, "[PANIC SELL] %s: Drop %.2f%%", trade.Symbol, profitMargin)
		return -1, nil
	}
	if cs.checkEarlyExitCondition(trade, currentPrice) {
		return -1, nil
	}

	if trailingActive && trailingStop > 0 && currentPrice <= trailingStop {
		logger.InfoColorf(logger.BrightRed, "[TRAIL STOP] %s: cp=%.4f <= tsl=%.4f (state=%s, PM=%.2f%%)",
			trade.Symbol, currentPrice, trailingStop, state.String(), profitMargin)
		if state == models.StronglyTrending || state == models.Trending {
			if cs.sinceTrailExit(trade.Symbol) > 3*time.Minute {
				cs.enqueueTrendReentry(trade.Symbol, currentPrice, state)
				cs.touchTrailExit(trade.Symbol)
			}
		}
		return -1, nil
	}

	if profitMargin > 0 && athPrice > trade.BuyPrice {
		if cs.checkAthFallOffSellCondition(profitMargin, profitMarginATH) {
			logger.InfoColorf(logger.BrightRed, "[ATH FALLOFF] %s: Drop %.2f%% from ATH", trade.Symbol, profitMarginATH)
			return -1, nil
		}
	}

	if cs.checkTimeSinceSellCondition(state, trade.Symbol, profitMargin, lastAthTime) {
		return -1, nil
	}

	if cs.checkBearishSignalSellCondition(profitMargin, bearishSignal, state) {
		logger.InfoColorf(logger.BrightRed, "[BEARISH EXIT] %s: State=%s, PM=%.2f%%",
			trade.Symbol, state.String(), profitMargin)
		return -1, nil
	}

	ptp1 := cs.PartialTP1Pct
	if ptp1 <= 0 {
		ptp1 = 0.9 // %
	}
	ptpSize := cs.PartialTP1Size
	if ptpSize <= 0 || ptpSize > 1 {
		ptpSize = 0.33
	}

	adjPTP1 := ptp1
	switch state {
	case models.Chaotic:
		adjPTP1 *= 0.85
	case models.RangeBound:
		adjPTP1 *= 0.9
	case models.StronglyTrending:
		adjPTP1 *= 1.05
	default:
		adjPTP1 *= 1.0
	}

	if profitMargin >= adjPTP1 {
		logger.InfoColorf(logger.BrightBlack, "[PARTIAL TP1] %s: PM=%.2f%% >= %.2f%% (size=%.0f%%)",
			trade.Symbol, profitMargin, adjPTP1, ptpSize*100)

		return -2, nil
	}
	// --- Konec Partial TP1 ---

	// Hold below breakeven
	if currentPrice < breakevenPrice {
		logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven, PM=%.2f%%", trade.Symbol, profitMargin)
		return 0, nil
	}

	// Profit taking (plný)
	if met, adjustedProfit := cs.checkDesiredProfitSellCondition(profitMargin, state); met {
		logger.InfoColorf(logger.BrightBlack, "[PROFIT SELL] %s: PM=%.2f%% vs target %.2f%%",
			trade.Symbol, profitMargin, adjustedProfit)
		return -2, nil
	}

	timeSinceATH := time.Since(lastAthTime).Minutes()
	logger.InfoColorf(logger.BrightBlack, "[HOLD] %s: PM=%.2f%% < Target | ATH age %.1fm",
		trade.Symbol, profitMargin, timeSinceATH)
	return 0, nil
}
func (cs *CompoundStrategy) enqueueTrendReentry(pair string, currentPrice float64, state models.MarketState) {
	repo := cs.ensureRepo()
	ci := cs.localIndicators

	// Nepřidávej duplicity v krátkém čase a blízko ceny
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

	// Potvrzení re-entry jen pokud není přestřeleno nad horní pásmo a momentum není negativní
	atr := 0.0
	if v, err := algos.ATR(ci.CandleSticks, 14); err == nil {
		atr = v
	} else {
		atr = math.Max(1e-9, ci.ADRVal)
	}

	// Cílíme návrat k průměru: cena poblíž middle band, histogram se zlepšuje
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
		Priority:        5, // priorita re-entry v trendu
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

//nolint:funlen, a lot of indicators to calculate
func (cs *CompoundStrategy) getIndicators(candles []models.CandleStick, pair string) (CurrentIndicators, error) {
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		return CurrentIndicators{}, fmt.Errorf("RSI: %w", err)
	}

	macdHist, sigLn, macdLn, macdInd, err1 := cs.MACD.Calculate(candles)
	if err1 != nil {
		return CurrentIndicators{}, fmt.Errorf("MACD: %w", err1)
	}

	// previous values for slope checks (best effort)
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
