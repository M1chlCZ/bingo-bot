package strategies

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/algos"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"log"
	"math"
	"sync"
	"time"
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
}

type CurrentIndicators struct {
	RSIVal        float64
	Histogram     float64
	SignalLine    float64
	MacdLine      float64
	MacdIndicator int
	StochasticK   float64
	StochasticD   float64
	LowerBand     float64
	MiddleBand    float64
	UpperBand     float64
	IchimokuRes   algos.IchimokuResult
	CCIVal        float64
	CCISignal     int
	MFIVal        float64
	MFiSignal     int
	ADRVal        float64
	ADRSignal     int
	CandleSticks  []models.CandleStick
}

type PendingBuy struct {
	ID           string
	Pair         string
	TriggerPrice float64
	TriggerTime  time.Time
	MarketState  models.MarketState
	RsiVal       float64
	MacdLine     float64
}

var pendingBuys sync.Map

//nolint:funlen, has to be this way
func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, marketState models.MarketState, pendingCoolDown time.Duration) (int, error) {
	if len(candles) == 0 {
		return 0, nil
	}

	currentPrice := candles[len(candles)-1].Close
	logger.DebugColorf(logger.Cyan, "State: %s, Pair: %s, CurrentPrice: %.2f", marketState.String(), pair, currentPrice)

	trade, _ := db2.SQLiteDB.GetActiveTrade(pair)
	isActive, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair)
	if err != nil {
		logger.Errorf("Error checking active trade: %v", err.Error())
		isActive = false
	}

	// Calculate Indicators
	currentIndicators, err := cs.getIndicators(candles, pair)
	if err != nil {
		logger.DebugColorf(logger.BrightRed, "Error calculating indicators: %v", err.Error())
		return 0, err
	}
	cs.localIndicators = currentIndicators
	bullishConditions := cs.checkBullishConditions(marketState, currentIndicators, currentPrice, pair)
	bearishConditions := cs.checkBearishConditions(marketState, currentIndicators, currentPrice)

	// If a trade exists, handle P/L logic first
	if trade != nil {
		return cs.checkActiveTrade(trade, currentPrice, bearishConditions && !bullishConditions, marketState)
	}

	logger.DebugColorf(logger.BrightBlack,
		"%s | %s | Ichimoku=(B:%t, Br:%t), MACD=%d (hist=%.2f), RSI=%.2f, Stoch=%.2f, CCI=%.2f, MFI=%.2f",
		pair, marketState.String(),
		currentIndicators.IchimokuRes.Bullish, currentIndicators.IchimokuRes.Bearish,
		currentIndicators.MacdIndicator, currentIndicators.Histogram, currentIndicators.RSIVal, currentIndicators.StochasticK, currentIndicators.CCIVal, currentIndicators.MFIVal,
	)

	bought := cs.evaluatePendingBuys(pair, currentPrice, currentIndicators, pendingCoolDown)
	if bought == 1 {
		return 1, nil
	}

	if bullishConditions {
		if cs.alreadyInPendingBuys(pair) {
			logger.DebugColorf(logger.Yellow, "[PENDING BUY] %s => already pending", pair)
			return 0, nil
		}
		pbKey := fmt.Sprintf("%s_%d", pair, time.Now().UnixNano())
		newPb := &PendingBuy{
			ID:           pbKey,
			Pair:         pair,
			TriggerPrice: currentPrice,
			TriggerTime:  time.Now(),
			MarketState:  marketState,
			RsiVal:       currentIndicators.RSIVal,
			MacdLine:     currentIndicators.MacdLine,
		}
		pendingBuys.Store(pbKey, newPb)
		logger.InfoColorf(logger.Blue, "[PENDING BUY ADDED] %s => price=%.2f, state=%v", pair, currentPrice, marketState)
		return 0, nil
	}
	if bearishConditions && isActive {
		return -1, nil
	}
	logger.DebugColorf(logger.Yellow, "[DEFAULT HOLD] %s => cci=%.1f, mfi=%.1f => no trade", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
	return 0, nil
}

// checkBullishConditions decides if we have a 'bullish' signal,
// given the current market state and the current indicators.
func (cs *CompoundStrategy) checkBullishConditions(
	state models.MarketState,
	ci CurrentIndicators,
	currentPrice float64,
	pair string,
) bool {
	// Calculate dynamic risk-reward threshold
	dynRR := cs.calculateDynamicRiskReward(ci)

	mfiOK := ci.MFIVal > float64(cs.MFI.Oversold) &&
		ci.MFIVal < float64(cs.MFI.Overbought)
	cciOK := ci.CCIVal < cs.CCI.Overbought &&
		ci.CCIVal > cs.CCI.Oversold
	volumeOk := mfiOK || cciOK // renamed

	emaAlignment := ci.MacdLine > ci.SignalLine &&
		ci.MacdLine >= 0 // allow 0-line crossover

	// Check if we're in a cooldown period after a losing trade
	if cs.isInCooldownPeriod(pair) {
		return false
	}

	// Apply market-state specific logic
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

// calculateDynamicRiskReward calculates the adjusted risk-reward threshold based on volatility
func (cs *CompoundStrategy) calculateDynamicRiskReward(ci CurrentIndicators) float64 {
	atr := ci.ADRVal / ci.MiddleBand // normalised ATR (≈ volatility %)
	dynRR := cs.RiskRewardThreshold
	switch {
	case atr > 0.05:
		dynRR *= 1.25 // very volatile → demand more reward
	case atr < 0.02:
		dynRR *= 0.80 // sleepy → accept a bit less
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
		if time.Since(tm) < 10*time.Minute {
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

// checkBullishStronglyTrending checks bullish conditions for strongly trending markets
func (cs *CompoundStrategy) checkBullishStronglyTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool, emaUp bool,
	state models.MarketState,
) bool {

	adrConfirmation := ci.ADRSignal == 1
	// ---------- adaptive RSI ceiling ----------
	rsiCeil := float64(cs.RSI.Overbought)
	rsiOk := ci.RSIVal < rsiCeil

	atr, err := algos.ATR(ci.CandleSticks, 14)
	if err != nil {
		atr = ci.MiddleBand
	}
	stop := currentPrice - 2.5*atr
	target := math.Max(ci.UpperBand*1.02, currentPrice+2*atr)
	rr := cs.calcRR(state, currentPrice, stop, target)
	indicators := atLeast(2, volCCIok, emaUp, rsiOk)

	logger.Debugf("State %s | Indicators: %t, ADR: %t, RR: %.2f Required RR %.2f",
		state.String(), indicators, adrConfirmation, rr, dynamicRR)

	if indicators && adrConfirmation {
		reward := rr > dynamicRR
		logger.DebugColorf(logger.BrightBlack, "Yes, bullish signal for %s with ADR low volatility confirmation | Reward %t",
			state.String(), reward)
		return reward
	}
	return false
}

// checkBullishTrending checks bullish conditions for trending markets
func (cs *CompoundStrategy) checkBullishTrending(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	volCCIok bool,
	state models.MarketState,
) bool {
	atr, err := algos.ATR(ci.CandleSticks, 14)
	if err != nil {
		atr = ci.MiddleBand
	}
	adrConfirmation := ci.ADRSignal >= 0 // neutral or low volatility is fine
	target := ci.UpperBand
	stop := currentPrice - 2.2*atr
	rr := cs.calcRR(state, currentPrice, stop, target)
	indicators := ci.MacdIndicator == 1 && ci.RSIVal < float64(cs.RSI.Overbought) && volCCIok && currentPrice < ci.UpperBand
	logger.DebugColorf(logger.BrightBlack, "Reward: %.2f Needed %.2f", rr, dynamicRR)

	logger.Debugf("State %s | Indicators: %t, ADR: %t, RR: %.2f Required RR %.2f",
		state.String(), indicators, adrConfirmation, rr, dynamicRR)

	if indicators && adrConfirmation {
		reward := rr > dynamicRR
		logger.DebugColorf(logger.BrightBlack, "Yes, bullish signal for %s with ADR volatility confirmation | Reward %t",
			state.String(), reward)
		return reward
	}
	return false
}

// checkBullishRangeBound checks bullish conditions for range-bound markets
func (cs *CompoundStrategy) checkBullishRangeBound(
	ci CurrentIndicators,
	currentPrice, dynamicRR float64,
	state models.MarketState,
) bool {
	adrConfirmation := ci.ADRSignal == 1 // low volatility
	indicators := (currentPrice <= ci.LowerBand && ci.RSIVal < 30) || (ci.CCIVal <= cs.CCI.Oversold)
	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	logger.Debugf("State %s | Indicators: %t, ADR: %t, RR: %.2f Required RR %.2f", state.String(), indicators, adrConfirmation, rr, dynamicRR)
	if indicators && adrConfirmation {
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)

		reward := rr > dynamicRR
		logger.DebugColorf(logger.BrightBlack, "Yes, bullish for %s with ADR low volatility | Reward %t", state.String(), reward)
		return reward

	}
	return false
}

func (cs *CompoundStrategy) checkBullishTransitional(ci CurrentIndicators, currentPrice, dynamicRR float64, emaAlignment bool, state models.MarketState) bool {
	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	indicators := (ci.RSIVal > 40 && ci.RSIVal < 70) && (ci.MFIVal > 35 && ci.MFIVal < 65) && emaAlignment
	logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
	if indicators {
		reward := rr > dynamicRR
		logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
		logger.DebugColorf(logger.BrightBlack, "Yes, bullish for %s with ADR high volatility spike | Reward %t", state.String(), reward)
		return reward
	}

	return false
}

// checkBullishChaotic checks bullish conditions for chaotic markets
func (cs *CompoundStrategy) checkBullishChaotic(ci CurrentIndicators, currentPrice, dynamicRR float64, state models.MarketState) bool {
	atr, err := algos.ATR(ci.CandleSticks, 14)
	if err != nil {
		atr = ci.MiddleBand
	}
	adrConfirmation := ci.ADRSignal >= 0 // neutral or low volatility is fine
	target := ci.UpperBand
	stop := currentPrice - 2.2*atr
	rr := cs.calcRR(state, currentPrice, stop, target)
	indicators := currentPrice < ci.LowerBand && ci.RSIVal < 40 && ci.MFIVal < 30
	logger.Debugf("State %s | Indicators: %t, ADR: %t, RR: %.2f Required RR %.2f", state.String(), indicators, adrConfirmation, rr, dynamicRR)
	if indicators && adrConfirmation {
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)

		reward := rr > (dynamicRR * 1.2)
		logger.DebugColorf(logger.BrightBlack, "Yes, bullish for %s with ADR high volatility spike | Reward %t", state.String(), reward)
		return reward

	}
	return false
}

func (cs *CompoundStrategy) checkBullishConditionsDefault(state models.MarketState, ci CurrentIndicators, currentPrice, dynamicRR float64, volumeOk bool) bool {
	target := ci.UpperBand
	stop := ci.LowerBand
	rr := cs.calcRR(state, currentPrice, stop, target)
	indicators := ci.MacdIndicator == 1 && ci.MFIVal < float64(cs.MFI.Overbought) && volumeOk
	logger.Debugf("Default logic for %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
	if indicators {
		reward := rr > (dynamicRR * 0.9)
		logger.DebugColorf(logger.BrightBlack, "Yes, default bullish %s | Reward %t", state.String(), reward)
		return reward
	}
	return false
}

// checkBearishStronglyTrending checks bearish conditions for strongly trending markets
func (cs *CompoundStrategy) checkBearishStronglyTrending(ci CurrentIndicators, currentPrice float64) bool {
	// In a strong uptrend, you might rarely short.
	// But maybe you exit if RSI is extremely high, or price is far above upper band, etc.
	// Example: If MACD goes negative or RSI > 80 => strong overbought
	if ci.MacdIndicator == -1 && ci.RSIVal > 80 {
		// also check if price is above upper band => blow-off top scenario
		if currentPrice > ci.MiddleBand {
			return true
		}
	}
	return false
}

// checkBearishTrending checks bearish conditions for trending markets
func (cs *CompoundStrategy) checkBearishTrending(ci CurrentIndicators) bool {
	// Normal trending scenario: short if MACD crosses down and RSI is high.
	if ci.MacdIndicator == -1 && ci.RSIVal > float64(cs.RSI.Overbought) {
		return true
	}
	// Optionally, if CCI or MFI is also overbought => more confirmation
	return false
}

// checkBearishRangeBound checks bearish conditions for range-bound markets
func (cs *CompoundStrategy) checkBearishRangeBound(ci CurrentIndicators, currentPrice float64) bool {
	// Mean reversion: we short near the upper band or if RSI > 70.
	if currentPrice >= ci.MiddleBand && ci.RSIVal > 70 {
		return true
	}
	// or if CCI is above +100 => overbought
	if ci.CCIVal > cs.CCI.Overbought {
		return true
	}
	return false
}

// checkBearishChaotic checks bearish conditions for chaotic markets
func (cs *CompoundStrategy) checkBearishChaotic(ci CurrentIndicators, currentPrice float64) bool {
	// In a chaotic market, you might consider shorting big spikes, e.g. if price is well above upper band
	// and RSI is extreme. But also be careful—chaotic markets can whipsaw quickly.
	if currentPrice > ci.MiddleBand && ci.RSIVal > 75 {
		return true
	}
	return false
}

// checkBearishTransitional checks bearish conditions for transitional markets
func (cs *CompoundStrategy) checkBearishTransitional(ci CurrentIndicators) bool {
	// Possibly short if we suspect a new downtrend forming:
	// e.g. MACD crossing below 0, RSI dropping from a moderate zone
	if ci.MacdIndicator == -1 && ci.RSIVal > 50 {
		// maybe also check MFI > 70 => distribution
		if ci.MFIVal > 70 {
			return true
		}
	}
	return false
}

// checkBearishDefault checks bearish conditions for default market state
func (cs *CompoundStrategy) checkBearishDefault(ci CurrentIndicators) bool {
	// Fallback logic
	// For example, short if MACD < 0 and RSI > Overbought
	if ci.MacdIndicator == -1 && ci.RSIVal > float64(cs.RSI.Overbought) {
		return true
	}
	return false
}

// checkBearishConditions decides if we have a 'bearish' (short/exit) signal,
// given the current market state and the current indicators.
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
	rr := reward / risk
	return rr
}

func (cs *CompoundStrategy) calculateRiskRewardRatioForSell(
	currentPrice, stop, target float64,
) float64 {
	if stop <= currentPrice || target >= currentPrice { // <-- NEW
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
	var bought int
	pendingBuys.Range(func(key, val interface{}) bool {
		pbKey := key.(string)
		pb := val.(*PendingBuy)
		if pb.Pair != pair {
			return true
		}

		// Check cooldown and time decay
		age := time.Since(pb.TriggerTime)
		if age < pendingCoolDown {
			logger.Debugf("Pending buy %s: age=%.1fsec < cooldown => skipping for now", pbKey, age.Seconds())
			return true
		}

		// check if price ran too far away
		if currentPrice > pb.TriggerPrice*1.03 {
			logger.Warnf("[PENDING BUY CANCELLED] %s => Price soared 3%% above trigger (%.2f -> %.2f).",
				pb.Pair, pb.TriggerPrice, currentPrice)
			pendingBuys.Delete(pbKey)
			return true
		}

		// If still bullish from your normal logic
		if cs.checkBullishConditions(pb.MarketState, indicators, currentPrice, pair) {
			logger.InfoColorf(logger.Blue, "[PENDING BUY CONFIRMED] %s => Buying now at %.2f", pb.Pair, currentPrice)
			pendingBuys.Delete(pbKey)
			bought = 1
			return false // break Range loop
		}

		// else conditions are no longer bullish => remove it
		logger.Infof("[PENDING BUY CANCELLED] %s => conditions changed", pb.Pair)
		pendingBuys.Delete(pbKey)
		return true
	})
	return bought
}

func (cs *CompoundStrategy) checkBuyConfirmation(
	indicators CurrentIndicators,
	currentPrice float64,
) bool {
	// Require at least 2 out of 3 confirmations
	confirmations := 0

	// 1. Volume confirmation
	if indicators.MFIVal > 20 && indicators.MFIVal < 80 {
		confirmations++
	}

	// 2. MACD confirmation
	if indicators.MacdLine > indicators.SignalLine && indicators.MacdLine > 0 {
		confirmations++
	}

	// 3. Price action confirmation
	if currentPrice > indicators.LowerBand && currentPrice < indicators.MiddleBand {
		confirmations++
	}

	return confirmations >= 2
}

func (cs *CompoundStrategy) alreadyInPendingBuys(pair string) bool {
	inTheMap := false
	pendingBuys.Range(func(_, val interface{}) bool {
		pb := val.(*PendingBuy)
		if pb.Pair == pair {
			inTheMap = true
			return false
		}
		return true
	})
	return inTheMap
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

	const maxHoldingTime = 60 * time.Minute // příklad: 60 minut
	const minimalAcceptableLoss = -0.2      // -0.2% pod breakeven

	if tradeDuration > maxHoldingTime && profitMargin >= minimalAcceptableLoss {
		logger.InfoColorf(logger.BrightYellow, "[EARLY EXIT] %s: Holding too long (%v), current margin=%.2f%%",
			trade.Symbol, tradeDuration, profitMargin)
		return true
	}
	return false
}

// checkPanicSellCondition checks if we should panic sell based on price drop
func (cs *CompoundStrategy) checkPanicSellCondition(profitMargin float64) bool {
	return profitMargin < -cs.HighestPriceFallOffMargin && (profitMargin > 0 || cs.PanicSell)
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

// checkBearishSignalSellCondition checks if we should sell based on bearish signal
func (cs *CompoundStrategy) checkBearishSignalSellCondition(profitMargin float64, bearishSignal bool) bool {
	return cs.SellOnBearish && bearishSignal && profitMargin > 0
}

// checkDesiredProfitSellCondition checks if we should sell based on desired profit
func (cs *CompoundStrategy) checkDesiredProfitSellCondition(profitMargin float64) bool {
	return profitMargin > cs.DesiredProfit
}

// checkActiveTrade evaluates an active trade to determine if it should be sold
func (cs *CompoundStrategy) checkActiveTrade(trade *models.ActiveTrade, currentPrice float64, bearishSignal bool, state models.MarketState) (int, error) {
	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

	// Track price extremes
	athPrice, atlPrice, lastAthTime, _ := cs.trackPriceExtremes(trade.Symbol, currentPrice)

	// Calculate profit margins
	profitMarginATH := (currentPrice - athPrice) / athPrice * 100
	upliftFromAtl := (currentPrice - atlPrice) / atlPrice * 100

	// Log trade monitoring information
	logger.Infof("[Trade Monitoring] %s | Buy=%.2f | Current=%.2f | PM=%.2f%% | ATH=%.2f | PM ATH=%.2f%%",
		trade.Symbol, trade.BuyPrice, currentPrice, profitMargin, athPrice, profitMarginATH)

	if profitMargin < 0 && currentPrice > atlPrice {
		logger.InfoColorf(logger.BrightYellow, "[ CurrentPrice is above ATL ] %s: Uplift from ATL (%.2f%%)", trade.Symbol, upliftFromAtl)
	}

	// Check sell conditions
	if cs.checkPanicSellCondition(profitMargin) {
		logger.InfoColorf(logger.BrightRed, "[PANIC SELL] %s: Price dropped below margin %.2f", trade.Symbol, profitMargin)
		return -1, nil
	}

	if cs.checkEarlyExitCondition(trade, currentPrice) {
		return -1, nil
	}

	if cs.checkAthFallOffSellCondition(profitMargin, profitMarginATH) {
		logger.InfoColorf(logger.BrightRed, "[ATH FALL OFF SELL] %s: Desired profit dropped below set ATH dropoff margin: (%.2f%%)", trade.Symbol, profitMarginATH)
		return -1, nil
	}

	if cs.checkTimeSinceSellCondition(state, trade.Symbol, profitMargin, lastAthTime) {
		return -1, nil
	}

	if cs.checkBearishSignalSellCondition(profitMargin, bearishSignal) {
		logger.InfoColorf(logger.BrightRed, "[BEARISH SIGNAL] %s", trade.Symbol)
		return -1, nil
	}

	if currentPrice < breakevenPrice {
		logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven. Profit=%.2f%%", trade.Symbol, profitMargin)
		return 0, nil
	}

	if cs.checkDesiredProfitSellCondition(profitMargin) {
		logger.InfoColorf(logger.BrightBlack, "[SELL] %s: Desired profit reached (%.2f%%)", trade.Symbol, profitMargin)
		return -1, nil
	}

	timeToMinutesParse := time.Since(lastAthTime).Minutes()
	logger.InfoColorf(logger.BrightBlack, "[HOLD] %s: PM=%.2f%% < Desired=%.2f%% | Time since ATH %.2f m", trade.Symbol, profitMargin, cs.DesiredProfit, timeToMinutesParse)
	return 0, nil
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

	stochK, stochD, err2 := cs.Stochastic.Calculate(candles)
	if err2 != nil {
		return CurrentIndicators{}, fmt.Errorf("Stochastic: %w", err2)
	}

	lowB, midB, upB, err3 := cs.BollingerBands.Calculate(candles)
	if err3 != nil {
		return CurrentIndicators{}, fmt.Errorf("Bollinger: %w", err3)
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

	return CurrentIndicators{
		RSIVal:        rsiVal,
		Histogram:     macdHist,
		SignalLine:    sigLn,
		MacdLine:      macdLn,
		MacdIndicator: macdInd,
		StochasticK:   stochK,
		StochasticD:   stochD,
		LowerBand:     lowB,
		MiddleBand:    midB,
		UpperBand:     upB,
		CCIVal:        cciVal,
		CCISignal:     cciSig,
		MFIVal:        mfiVal,
		MFiSignal:     mfiSig,
		ADRVal:        adrVal,
		ADRSignal:     adrSig,
		CandleSticks:  candles,
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
