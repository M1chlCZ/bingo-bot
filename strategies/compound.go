package strategies

import (
	"encoding/json"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/algos"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
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
	RsiVal       float64
	MacdLine     float64
}

var currentIndicators CurrentIndicators

var pendingBuys sync.Map

func (cs *CompoundStrategy) GetStrategyType() StrategyType {
	return CompoundStrategyType
}

// nolint:
//
//nolint:gocognit,gocyclo main function suppose to be complex
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
	currentIndicators, err = cs.getIndicators(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating indicators: %v", err.Error())
		return 0, err
	}
	// If a trade exists, handle P/L logic first
	if trade != nil {
		return cs.checkActiveTrade(trade, currentPrice)
	}

	bullishConditions := (currentIndicators.IchimokuRes.Bullish || !currentIndicators.IchimokuRes.Bearish) &&
		(currentIndicators.MacdIndicator == 1) &&
		(currentIndicators.RSIVal < float64(cs.RSI.Overbought)) &&
		(currentIndicators.Histogram > 0)

	bearishConditions := currentIndicators.IchimokuRes.Bearish && (currentIndicators.MacdIndicator == -1) && (currentIndicators.RSIVal > float64(cs.RSI.Overbought))

	bought := cs.confirmOrCancelPendingBuysFor(pair, currentPrice, currentIndicators.MacdIndicator, currentIndicators.RSIVal, pendingCoolDown)
	if bought == 1 {
		return 1, nil
	}

	logger.DebugColorf(logger.BrightBlack,
		"%s | %s | Ichimoku=(B:%t, Br:%t), MACD=%d (hist=%.2f), RSI=%.2f, Stoch=%.2f, CCI=%.2f, MFI=%.2f",
		pair, marketState.String(),
		currentIndicators.IchimokuRes.Bullish, currentIndicators.IchimokuRes.Bearish,
		currentIndicators.MacdIndicator, currentIndicators.Histogram, currentIndicators.RSIVal, currentIndicators.StochasticK, currentIndicators.CCIVal, currentIndicators.MFIVal,
	)

	mfiOverbought := float64(cs.MFI.Overbought)
	mfiOversold := float64(cs.MFI.Oversold)
	cciOverbought := cs.CCI.Overbought
	cciOversold := cs.CCI.Oversold
	//rsiOverbought := float64(cs.RSI.Overbought)
	// rsiOversold := float64(cs.RSI.Oversold)

	// === Market-state based logic ===
	switch marketState {
	// ------------------ STRONGLY TRENDING ------------------
	case models.StronglyTrending:
		if bullishConditions &&
			currentIndicators.CCIVal < cciOverbought && currentIndicators.CCIVal > cciOversold &&
			currentIndicators.MFIVal < mfiOverbought && currentIndicators.MFIVal > mfiOversold &&
			currentPrice < currentIndicators.MiddleBand {

			target := currentIndicators.UpperBand * 1.02
			stop := currentIndicators.LowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.Infof("[StrongTrend BUY] cci=%.1f,mfi=%.1f,RR=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
			if rr > cs.RiskRewardThreshold {
				pbKey := fmt.Sprintf("%s_%d", pair, time.Now().UnixNano())
				newPb := &PendingBuy{
					ID:           pbKey,
					Pair:         pair,
					TriggerPrice: currentPrice,
					TriggerTime:  time.Now(),
					RsiVal:       currentIndicators.RSIVal,
					MacdLine:     currentIndicators.MacdLine,
				}
				pendingBuys.Store(pbKey, newPb)
				logger.Infof("[StrongTrend PENDING BUY CREATED] Pair=%s => Price=%.2f", pair, currentPrice)
				return 0, nil
			}

		}

		// For a SELL logic, you might check cciVal > 100 or currentIndicators.MFIVal > 80 => Overbought
		if isActive &&
			currentIndicators.CCIVal > cciOverbought &&
			currentIndicators.MFIVal > mfiOverbought &&
			currentIndicators.MacdIndicator == -1 {
			target := currentIndicators.LowerBand
			stop := currentIndicators.MiddleBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.InfoColorf(logger.Cyan, "[StrongTrend SELL] cci=%.1f,mfi=%.1f => rr=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.BrightMagenta, "[StronglyTrending HOLD] %s: No trade triggered.", pair)
		return 0, nil

	// ------------------ TRANSITIONAL ------------------
	case models.Transitional:
		// e.g. cciSignal == 1 => CCI < cciOversold threshold, mfiSignal == 1 => MFI < mfiOversold => might catch a big contrarian trade
		if currentIndicators.CCISignal == 1 && currentIndicators.MFiSignal == 1 {
			target := currentIndicators.MiddleBand
			stop := currentPrice * 0.98
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.Infof("[Transitional BUY] cci=%.1f,mfi=%.1f,RR=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
			if rr > cs.RiskRewardThreshold*0.8 {
				logger.InfoColorf(logger.Cyan, "[Transitional BUY triggered!]")
				return 1, nil
			}
		}
		// etc. for SELL if cciSignal == -1 && mfiSignal == -1
		logger.DebugColorf(logger.BrightYellow, "[Transitional HOLD]%s: no extreme condition found. (CCI=%.1f, MFI=%.1f)", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
		return 0, nil

	// ------------------ TRENDING ------------------
	case models.Trending:
		if bullishConditions &&
			currentIndicators.CCIVal < cciOverbought &&
			currentIndicators.CCIVal > cciOversold &&
			currentIndicators.MFIVal < mfiOverbought &&
			currentIndicators.MFIVal > mfiOversold {

			target := currentIndicators.MiddleBand
			stop := currentIndicators.LowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				pbKey := fmt.Sprintf("%s_%d", pair, time.Now().UnixNano())
				newPb := &PendingBuy{
					ID:           pbKey,
					Pair:         pair,
					TriggerPrice: currentPrice,
					TriggerTime:  time.Now(),
					RsiVal:       currentIndicators.RSIVal,
					MacdLine:     currentIndicators.MacdLine,
				}
				pendingBuys.Store(pbKey, newPb)
				logger.InfoColorf(logger.Cyan, "[TrendingState PENDING BUY CREATED] Pair=%s => Price=%.2f", pair, currentPrice)
				return 0, nil
			}
		}

		if bearishConditions && isActive &&
			currentIndicators.CCIVal > cciOverbought &&
			currentIndicators.MFIVal > mfiOverbought {
			target := currentIndicators.LowerBand
			stop := currentIndicators.MiddleBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[Trending SELL] cci=%.1f,mfi=%.1f => rr=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.Yellow, "%s: No trending trade found. CCI=%.1f, MFI=%.1f", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
		return 0, nil

	// ------------------ CHAOTIC ------------------
	case models.Chaotic:
		if currentPrice < currentIndicators.LowerBand &&
			currentIndicators.CCIVal < cciOversold &&
			currentIndicators.MFIVal < mfiOversold {
			pbKey := fmt.Sprintf("%s_%d", pair, time.Now().UnixNano())
			newPb := &PendingBuy{
				ID:           pbKey,
				Pair:         pair,
				TriggerPrice: currentPrice,
				TriggerTime:  time.Now(),
				RsiVal:       currentIndicators.RSIVal,
				MacdLine:     currentIndicators.MacdLine,
			}
			pendingBuys.Store(pbKey, newPb)
			logger.InfoColorf(logger.Cyan, "[Chaotic PENDING BUY CREATED] Pair=%s => Price=%.2f", pair, currentPrice)
			//logger.InfoColorf(logger.Cyan, "[Chaotic BUY] cci=%.1f,mfi=%.1f => oversold", currentIndicators.CCIVal, currentIndicators.MFIVal)
			return 1, nil
		}
		logger.DebugColorf(logger.Yellow, "[CHAOTIC HOLD]%s: cci=%.1f, mfi=%.1f => no trade", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
		return 0, nil

	// ------------------ RANGEBOUND ------------------
	case models.RangeBound:
		relaxedRR := cs.RiskRewardThreshold * 0.5

		// BUY near lower band if cciSignal=1 or mfiSignal=1
		if currentPrice <= currentIndicators.LowerBand && (currentIndicators.MacdLine == 1 || currentIndicators.MFiSignal == 1) {
			target := currentIndicators.MiddleBand
			stop := currentIndicators.LowerBand * 0.99
			rr := cs.calcRR(currentPrice, stop, target, pair)
			if rr > relaxedRR {
				logger.InfoColorf(logger.Cyan, "[RangeBound BUY] cci=%.1f,mfi=%.1f,rr=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
				return 1, nil
			}
		}
		// SELL near upper band if cciSignal=-1 or mfiSignal=-1
		if currentPrice >= currentIndicators.MiddleBand && isActive && (currentIndicators.MacdLine == -1 || currentIndicators.MFiSignal == -1) {
			target := currentIndicators.MiddleBand
			stop := currentIndicators.MiddleBand * 1.01
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > relaxedRR {
				logger.Infof("[RangeBound SELL] cci=%.1f,mfi=%.1f, rr=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.Yellow, "[RANGE-BOUND HOLD]%s: cci=%.1f, mfi=%.1f => no trade", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
		return 0, nil

	// ------------------ DEFAULT ------------------
	default:
		if bullishConditions &&
			currentIndicators.CCIVal < cciOverbought &&
			currentIndicators.MFIVal < mfiOverbought {
			target := currentIndicators.MiddleBand
			stop := currentIndicators.LowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				pbKey := fmt.Sprintf("%s_%d", pair, time.Now().UnixNano())
				newPb := &PendingBuy{
					ID:           pbKey,
					Pair:         pair,
					TriggerPrice: currentPrice,
					TriggerTime:  time.Now(),
					RsiVal:       currentIndicators.RSIVal,
					MacdLine:     currentIndicators.MacdLine,
				}
				pendingBuys.Store(pbKey, newPb)
				logger.InfoColorf(logger.Cyan, "[DefaultState PENDING BUY CREATED] Pair=%s => Price=%.2f", pair, currentPrice)
				return 0, nil
			}
		}

		if bearishConditions && isActive &&
			currentIndicators.CCIVal > cciOverbought &&
			currentIndicators.MFIVal > mfiOverbought {
			target := currentIndicators.LowerBand
			stop := currentIndicators.MiddleBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[Default SELL] cci=%.1f, mfi=%.1f => rr=%.2f", currentIndicators.CCIVal, currentIndicators.MFIVal, rr)
				return -1, nil
			}
		}

		logger.DebugColorf(logger.Yellow, "[DEFAULT HOLD] %s => cci=%.1f, mfi=%.1f => no trade", pair, currentIndicators.CCIVal, currentIndicators.MFIVal)
		return 0, nil
	}
}

func (cs *CompoundStrategy) calcRR(currentPrice, stop, target float64, _ string) float64 {
	risk := (currentPrice - stop) / currentPrice
	reward := (target - currentPrice) / currentPrice
	if risk <= 0 || reward <= 0 {
		return 0
	}
	rr := reward / risk
	return rr
}

func (cs *CompoundStrategy) calcRRForSell(currentPrice, stop, target float64, _ string) float64 {
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

func (cs *CompoundStrategy) confirmOrCancelPendingBuysFor(pair string, currentPrice float64, macdIndicator int, rsiVal float64, pendingCoolDown time.Duration) int {

	var bought int
	pendingBuys.Range(func(key, val interface{}) bool {
		pbKey := key.(string)
		pb := val.(*PendingBuy)
		if pb.Pair != pair {
			return true
		}

		age := time.Since(pb.TriggerTime)
		if age < pendingCoolDown {
			logger.Debugf("[PENDING BUY WAIT] Pair=%s => Age=%.2fsec < %dsec cooldown",
				pb.Pair, age.Seconds(), int(pendingCoolDown.Seconds()))
			return true // keep looping for other pairs/pending
		}

		// --- Condition to confirm the buy ---
		// Example: price is now <= the original trigger price (pullback),
		// and we still have a bullish MACD, and RSI < Overbought(65).
		if currentPrice <= pb.TriggerPrice && macdIndicator == 1 && rsiVal < float64(cs.RSI.Overbought) {
			logger.InfoColorf(logger.Cyan, "[PENDING BUY CONFIRMED] Pair=%s => actual buy now", pb.Pair)
			pendingBuys.Delete(pbKey)
			bought = 1
			return false
		}

		// --- Condition to cancel the buy ---
		// If price has gone 5% above trigger, or MACD turned negative, or RSI is too high => no longer want it
		if currentPrice > pb.TriggerPrice*1.05 || macdIndicator == -1 || rsiVal > float64(cs.RSI.Overbought) {
			logger.Warnf("[PENDING BUY CANCELLED] Pair=%s => conditions reversed", pb.Pair)
			pendingBuys.Delete(pbKey)
		}

		return true
	})

	return bought
}

func (cs *CompoundStrategy) checkActiveTrade(trade *models.ActiveTrade, currentPrice float64) (int, error) {
	breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
	profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

	// HIGH PRICE since trade
	athPrice, err := db2.SQLiteDB.GetAth(trade.Symbol)
	if err != nil || currentPrice > athPrice {
		logger.Infof("Setting new HIGH price for %s: %.8f", trade.Symbol, currentPrice)
		if e := db2.SQLiteDB.SetUpdateAth(trade.Symbol, currentPrice); e != nil {
			logger.Errorf("Error updating ATH price for %s: %v", trade.Symbol, e)
		}
		athPrice = currentPrice
	}

	// LOW PRICE since trade
	atlPrice, err := db2.SQLiteDB.GetAtl(trade.Symbol)
	if err != nil || currentPrice < atlPrice {
		logger.Infof("Setting new LOW price for %s: %.8f", trade.Symbol, currentPrice)
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
	return currentIndicators
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
