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
		logger.Errorf("Error calculating indicators: %v", err.Error())
		return 0, err
	}
	cs.localIndicators = currentIndicators
	bullishConditions := cs.checkBullishConditions(marketState, currentIndicators, currentPrice)
	bearishConditions := cs.checkBearishConditions(marketState, currentIndicators, currentPrice)

	// If a trade exists, handle P/L logic first
	if trade != nil {
		return cs.checkActiveTrade(trade, currentPrice, bearishConditions && !bullishConditions)
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
) bool {
	// Dynamic Risk/Reward based on ATR
	atr := (ci.UpperBand - ci.LowerBand) / ci.MiddleBand
	dynamicRR := cs.RiskRewardThreshold
	if atr > 0.05 { // High volatility
		dynamicRR *= 1.2
	} else if atr < 0.02 { // Low volatility
		dynamicRR *= 0.8
	}

	// Volume confirmation
	volumeOk := ci.MFIVal > float64(cs.MFI.Oversold) && ci.MFIVal < float64(cs.MFI.Overbought)

	// EMA alignment check
	emaAlignment := ci.MacdLine > ci.SignalLine && ci.MacdLine > 0

	switch state {
	case models.StronglyTrending:
		target := ci.UpperBand * 1.02
		stop := ci.LowerBand
		rr := cs.calcRR(currentPrice, stop, target)
		indicators := ci.MacdIndicator == 1 && ci.RSIVal < float64(cs.RSI.Overbought) && volumeOk && emaAlignment
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)

		if indicators {
			reward := rr > dynamicRR
			logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
			return reward
		}

	case models.Trending:
		target := ci.UpperBand
		stop := ci.LowerBand
		rr := cs.calcRR(currentPrice, stop, target)
		indicators := ci.MacdIndicator == 1 && ci.RSIVal < float64(cs.RSI.Overbought) && volumeOk && currentPrice < ci.UpperBand
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
		if indicators {
			reward := rr > dynamicRR
			logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
			return reward
		}

	case models.RangeBound:
		indicators := (currentPrice <= ci.LowerBand && ci.RSIVal < 30) || (ci.CCIVal <= cs.CCI.Oversold)
		target := ci.UpperBand
		stop := ci.LowerBand
		rr := cs.calcRR(currentPrice, stop, target)
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
		if indicators {
			reward := rr > (dynamicRR * 0.8)
			logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
			return reward
		}

	case models.Chaotic:
		target := ci.MiddleBand
		stop := ci.LowerBand * 0.98
		rr := cs.calcRR(currentPrice, stop, target)
		indicators := currentPrice < ci.LowerBand && ci.RSIVal < 40 && ci.MFIVal < 30
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
		if indicators {
			reward := rr > (dynamicRR * 1.2)
			logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
			return reward
		}

	case models.Transitional:
		target := ci.UpperBand
		stop := ci.LowerBand
		rr := cs.calcRR(currentPrice, stop, target)
		indicators := ci.MacdIndicator == 1 && (ci.RSIVal > 40 && ci.RSIVal < 70) && (ci.MFIVal > 35 && ci.MFIVal < 65) && emaAlignment
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
		if indicators {
			reward := rr > dynamicRR
			logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
			return reward
		}

	default:
		indicators := ci.MacdIndicator == 1 && ci.MFIVal < float64(cs.MFI.Overbought) && volumeOk
		target := ci.UpperBand
		stop := ci.LowerBand
		rr := cs.calcRR(currentPrice, stop, target)
		logger.Debugf("State %s | Indicators: %t, RR: %.2f Required RR %.2f", state.String(), indicators, rr, dynamicRR)
		if indicators {
			reward := rr > (dynamicRR * 0.9)
			logger.DebugColorf(logger.BrightBlack, "Yes, we have a strong bullish signal for %s | Reward %t", state.String(), reward)
			return reward
		}
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
		// In a strong uptrend, you might rarely short.
		// But maybe you exit if RSI is extremely high, or price is far above upper band, etc.
		// Example: If MACD goes negative or RSI > 80 => strong overbought
		if ci.MacdIndicator == -1 && ci.RSIVal > 80 {
			// also check if price is above upper band => blow-off top scenario
			if currentPrice > ci.UpperBand {
				return true
			}
		}
		return false

	case models.Trending:
		// Normal trending scenario: short if MACD crosses down and RSI is high.
		if ci.MacdIndicator == -1 && ci.RSIVal > float64(cs.RSI.Overbought) {
			return true
		}
		// Optionally, if CCI or MFI is also overbought => more confirmation
		return false

	case models.RangeBound:
		// Mean reversion: we short near the upper band or if RSI > 70.
		if currentPrice >= ci.UpperBand && ci.RSIVal > 70 {
			return true
		}
		// or if CCI is above +100 => overbought
		if ci.CCIVal > cs.CCI.Overbought {
			return true
		}
		return false

	case models.Chaotic:
		// In a chaotic market, you might consider shorting big spikes, e.g. if price is well above upper band
		// and RSI is extreme. But also be careful—chaotic markets can whipsaw quickly.
		if currentPrice > ci.UpperBand && ci.RSIVal > 75 {
			return true
		}
		return false

	case models.Transitional:
		// Possibly short if we suspect a new downtrend forming:
		// e.g. MACD crossing below 0, RSI dropping from a moderate zone
		if ci.MacdIndicator == -1 && ci.RSIVal > 50 {
			// maybe also check MFI > 70 => distribution
			if ci.MFIVal > 70 {
				return true
			}
		}
		return false

	default:
		// Fallback logic
		// For example, short if MACD < 0 and RSI > Overbought
		if ci.MacdIndicator == -1 && ci.RSIVal > float64(cs.RSI.Overbought) {
			return true
		}
		return false
	}
}

func (cs *CompoundStrategy) calcRR(currentPrice, stop, target float64) float64 {
	risk := (currentPrice - stop) / currentPrice
	reward := (target - currentPrice) / currentPrice
	if risk <= 0 || reward <= 0 {
		return 0
	}
	rr := reward / risk
	return rr
}

func (cs *CompoundStrategy) calcRRForSell(currentPrice, stop, target float64) float64 {
	risk := (stop - currentPrice) / currentPrice
	reward := (currentPrice - target) / currentPrice
	if risk <= 0 || reward <= 0 {
		return 0
	}
	rr := reward / risk
	return rr
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

func (cs *CompoundStrategy) Validate() error {
	v := validator.New()
	return v.Struct(cs)
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

		// Skip if it's for another pair
		if pb.Pair != pair {
			return true
		}

		// Check cooldown and time decay
		age := time.Since(pb.TriggerTime)
		if age < pendingCoolDown {
			logger.Debugf("Pending buy %s: age=%.1fsec < cooldown => skipping for now", pbKey, age.Seconds())
			return true
		}

		// Additional confirmation for pending buys
		confirmation := cs.checkBuyConfirmation(indicators, currentPrice)
		if !confirmation {
			logger.InfoColorf(logger.Red, "Pending buy %s: confirmation failed => cancelling", pbKey)
			pendingBuys.Delete(pbKey)
			return true
		}

		// Execute buy if conditions are met
		if cs.checkBullishConditions(pb.MarketState, indicators, currentPrice) &&
			currentPrice <= pb.TriggerPrice {
			logger.InfoColorf(logger.Green, "[PENDING BUY CONFIRMED] %s => actual buy now (State=%v)", pb.Pair, pb.MarketState)
			pendingBuys.Delete(pbKey)
			bought = 1
			return false
		}

		// Dynamic cancellation conditions
		if (currentPrice > pb.TriggerPrice*1.05) || // Price ran away
			(indicators.MacdIndicator == -1) || // MACD turned bearish
			(indicators.RSIVal > float64(cs.RSI.Overbought)) || // Overbought
			(indicators.MFIVal > float64(cs.MFI.Overbought)) { // Volume exhaustion
			logger.Warnf("[PENDING BUY CANCELLED] %s => conditions reversed. (State=%v)", pb.Pair, pb.MarketState)
			pendingBuys.Delete(pbKey)
		}

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

func (cs *CompoundStrategy) checkActiveTrade(trade *models.ActiveTrade, currentPrice float64, bearishSignal bool) (int, error) {
	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

	// HIGH PRICE since trade
	athPrice, err := db2.SQLiteDB.GetAth(trade.Symbol)
	if err != nil || currentPrice > athPrice {
		logger.InfoColorf(logger.Green, "New HIGH price for %s: %.8f", trade.Symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAth(trade.Symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATH price for %s: %v", trade.Symbol, e)
		}
		athPrice = currentPrice
	}

	// LOW PRICE since trade
	atlPrice, err := db2.SQLiteDB.GetAtl(trade.Symbol)
	if err != nil || currentPrice < atlPrice {
		logger.InfoColorf(logger.BrightRed, "New LOW price for %s: %.8f", trade.Symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAtl(trade.Symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATL price for %s: %v", trade.Symbol, e)
		}
		atlPrice = currentPrice
	}

	// Percentage change since trade
	profitMarginATH := (currentPrice - athPrice) / athPrice * 100
	upliftFromAtl := (currentPrice - atlPrice) / atlPrice * 100

	logger.Infof("[Trade Monitoring] %s | Buy=%.2f | Current=%.2f | PM=%.2f%% | ATH=%.2f | PM ATH=%.2f%%",
		trade.Symbol, trade.BuyPrice, currentPrice, profitMargin, athPrice, profitMarginATH)

	if profitMargin < 0 && currentPrice > atlPrice {
		logger.InfoColorf(logger.BrightYellow, "[ CurrentPrice is above ATL ] %s: Uplift from ATL (%.2f%%)", trade.Symbol, upliftFromAtl)
	}

	if profitMargin < -cs.HighestPriceFallOffMargin {
		if cs.PanicSell {
			logger.InfoColorf(logger.BrightRed, "[PANIC SELL] %s: Price dropped below margin %.2f", trade.Symbol, profitMargin)
			return -1, nil
		}
	}

	if cs.SellOnBearish && bearishSignal {
		if profitMargin > 0 {
			logger.InfoColorf(logger.BrightRed, "[BEARISH SIGNAL] %s", trade.Symbol)
			return -1, nil
		}
	}

	if profitMarginATH < -cs.HighestPriceFallOffMargin {
		if profitMargin > 0 {
			logger.InfoColorf(logger.BrightBlack, "[ATH FALL OFF SELL] %s: Desired profit dropped below set ATH dropoff margin: (%.2f%%)", trade.Symbol, profitMarginATH)
			return -1, nil
		}
	}

	if currentPrice < breakevenPrice {
		logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven. Profit=%.2f%%", trade.Symbol, profitMargin)
		return 0, nil
	}

	if profitMargin > cs.DesiredProfit {
		logger.InfoColorf(logger.BrightBlack, "[SELL] %s: Desired profit reached (%.2f%%)", trade.Symbol, profitMargin)
		return -1, nil
	}

	logger.InfoColorf(logger.BrightBlack, "[HOLD] %s: PM=%.2f%% < Desired=%.2f%%", trade.Symbol, profitMargin, cs.DesiredProfit)
	return 0, nil

}

func (cs *CompoundStrategy) getIndicators(candles []models.CandleStick, pair string) (CurrentIndicators, error) {
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating RSI: %v", err.Error())
		return CurrentIndicators{}, err
	}
	macdHistogram, signalLine, macdLine, macdIndicatorSignal, err := cs.MACD.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating MACD: %v", err)
		return CurrentIndicators{}, err
	}

	stochasticK, stochasticD, err := cs.Stochastic.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Stochastic: %v", err.Error())
		return CurrentIndicators{}, err
	}

	lowerBand, middleBand, upperBand, err := cs.BollingerBands.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Bollinger Bands: %v", err.Error())
		return CurrentIndicators{}, err
	}

	ichimokuRes, err := cs.Ichimoku.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Ichimoku: %v", err.Error())
		return CurrentIndicators{}, err
	}

	cciVal, cciSignal, err := cs.CCI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating CCI: %v", err)
	}

	mfiVal, mfiSignal, err := cs.MFI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating MFI: %v", err)
	}

	return CurrentIndicators{
		RSIVal:        rsiVal,
		Histogram:     macdHistogram,
		SignalLine:    signalLine,
		MacdLine:      macdLine,
		MacdIndicator: macdIndicatorSignal,
		StochasticK:   stochasticK,
		StochasticD:   stochasticD,
		LowerBand:     lowerBand,
		MiddleBand:    middleBand,
		UpperBand:     upperBand,
		IchimokuRes:   ichimokuRes,
		CCIVal:        cciVal,
		CCISignal:     cciSignal,
		MFIVal:        mfiVal,
		MFiSignal:     mfiSignal,
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
		// lastIndicators stays zero or copy if needed
	}
	return newCS
}
func (cs *CompoundStrategy) GetMarketState() models.MarketState {
	return cs.MarketState
}
