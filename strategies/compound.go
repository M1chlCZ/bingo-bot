package strategies

import (
	"github.com/M1chlCZ/bingo-bot/algos"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/go-playground/validator/v10"
)

type CompoundStrategy struct {
	StrategyType              StrategyType                `validate:"required"`
	RSI                       *algos.RSIStrategy          `validate:"required"`
	MACD                      *algos.MACDStrategy         `validate:"required"`
	Stochastic                *algos.StochasticOscillator `validate:"required"`
	BollingerBands            *algos.BollingerBands       `validate:"required"`
	Ichimoku                  *algos.IchimokuStrategy     `validate:"required"`
	CCI                       *algos.CCIStrategy          `validate:"required"`
	MFI                       *algos.MFIStrategy          `validate:"required"`
	MarketState               models.MarketState          `validate:"required"`
	RiskRewardThreshold       float64                     `validate:"gte=0"`
	FeeRate                   float64                     `validate:"gte=0"`
	DesiredProfit             float64                     `validate:"gte=0"`
	HighestPriceFallOffMargin float64                     `validate:"gte=0"`
	CandleInterval            string                      `validate:"required"`
	PanicSell                 bool                        `validate:"required"`
}

type LastData struct {
	RSIVal         float64
	Histogram      float64
	SignalLine     float64
	MacdLine       float64
	StochasticK    float64
	StochasticD    float64
	LowerBand      float64
	MiddleBand     float64
	UpperBand      float64
	IchimokuTenkan float64
	IchimokuKijun  float64
	CCIVal         float64
	MFIVal         float64
}

var lastData LastData

func (cs *CompoundStrategy) GetStrategyType() StrategyType {
	return RSIMACDStrategyType
}

func (cs *CompoundStrategy) Calculate(candles []models.CandleStick, pair string, marketState models.MarketState) (int, error) {
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
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating RSI: %v", err.Error())
		return 0, err
	}
	macdHistogram, signalLine, macdLine, macdIndicatorSignal, err := cs.MACD.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating MACD: %v", err)
		return 0, err
	}

	stochasticK, stochasticD, err := cs.Stochastic.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Stochastic: %v", err.Error())
		return 0, err
	}

	lowerBand, middleBand, upperBand, err := cs.BollingerBands.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Bollinger Bands: %v", err.Error())
		return 0, err
	}

	ichimokuRes, err := cs.Ichimoku.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Ichimoku: %v", err.Error())
		return 0, err
	}

	cciVal, cciSignal, err := cs.CCI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating CCI: %v", err)
	}

	mfiVal, mfiSignal, err := cs.MFI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating MFI: %v", err)
	}

	lastData = LastData{
		RSIVal:         rsiVal,
		Histogram:      macdHistogram,
		SignalLine:     signalLine,
		MacdLine:       macdLine,
		StochasticK:    stochasticK,
		StochasticD:    stochasticD,
		LowerBand:      lowerBand,
		MiddleBand:     middleBand,
		UpperBand:      upperBand,
		IchimokuKijun:  ichimokuRes.Kijun,
		IchimokuTenkan: ichimokuRes.Tenkan,
		CCIVal:         cciVal,
		MFIVal:         mfiVal,
	}

	// If a trade exists, handle P/L logic first
	if trade != nil {
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
			pair, trade.BuyPrice, currentPrice, profitMargin, athPrice, profitMarginATH)

		if profitMargin < 0 && currentPrice > atlPrice {
			logger.InfoColorf(logger.BrightYellow, "[ CurrentPrice is above ATL ] %s: Uplift from ATL (%.2f%%)", pair, upliftFromAtl)
		}

		if profitMargin < -cs.HighestPriceFallOffMargin {
			if cs.PanicSell {
				logger.InfoColorf(logger.BrightRed, "[PANIC SELL] %s: Price dropped below margin %.2f", pair, profitMargin)
				return -1, nil
			}
		}

		if profitMarginATH < -cs.HighestPriceFallOffMargin {
			if cs.PanicSell {
				if profitMargin > 0 {
					logger.InfoColorf(logger.BrightBlack, "[ATH FALL OFF SELL] %s: Desired profit dropped below set ATH dropoff margin: (%.2f%%)", pair, profitMarginATH)
					return -1, nil
				}
			}
		}

		if currentPrice < breakevenPrice {
			logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven. Profit=%.2f%%", pair, profitMargin)
			return 0, nil
		}

		if profitMargin > cs.DesiredProfit {
			logger.InfoColorf(logger.BrightBlack, "[SELL] %s: Desired profit reached (%.2f%%)", pair, profitMargin)
			return -1, nil
		}

		logger.InfoColorf(logger.BrightBlack, "[HOLD] %s: PM=%.2f%% < Desired=%.2f%%", pair, profitMargin, cs.DesiredProfit)
		return 0, nil
	}

	bullishConditions := (ichimokuRes.Bullish || !ichimokuRes.Bearish) &&
		(macdIndicatorSignal == 1) &&
		(rsiVal < float64(cs.RSI.Overbought)) &&
		(macdHistogram > 0)

	bearishConditions := (ichimokuRes.Bearish || !ichimokuRes.Bullish) &&
		(macdIndicatorSignal == -1) &&
		(rsiVal > float64(cs.RSI.Oversold))

	// Log
	logger.DebugColorf(logger.BrightBlack,
		"%s | %s | Ichimoku=(B:%t, Br:%t), MACD=%d (hist=%.2f), RSI=%.2f, Stoch=%.2f, CCI=%.2f, MFI=%.2f",
		pair, marketState.String(),
		ichimokuRes.Bullish, ichimokuRes.Bearish,
		macdIndicatorSignal, macdHistogram, rsiVal, stochasticK, cciVal, mfiVal,
	)

	mfiOverbought := float64(cs.MFI.Overbought)
	mfiOversold := float64(cs.MFI.Oversold)
	cciOverbought := cs.CCI.Overbought
	cciOversold := cs.CCI.Oversold
	rsiOverbought := float64(cs.RSI.Overbought)
	//rsiOversold := float64(cs.RSI.Oversold)

	// === Market-state based logic ===
	switch marketState {
	// ------------------ STRONGLY TRENDING ------------------
	case models.StronglyTrending:
		if ichimokuRes.Bullish && macdIndicatorSignal == 1 && macdHistogram > 0 &&
			rsiVal < rsiOverbought &&
			cciVal < cciOverbought && cciVal > cciOversold &&
			mfiVal < mfiOverbought && mfiVal > mfiOversold &&
			currentPrice < middleBand {

			target := upperBand * 1.02
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.Infof("[StrongTrend BUY] cci=%.1f,mfi=%.1f,RR=%.2f", cciVal, mfiVal, rr)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[StrongTrend BUY] Finalizing buy. RR=%.2f > %.2f", rr, cs.RiskRewardThreshold)
				return 1, nil
			}
		}

		// For a SELL logic, you might check cciVal > 100 or mfiVal > 80 => Overbought
		if isActive &&
			cciVal > cciOverbought &&
			mfiVal > mfiOverbought &&
			macdIndicatorSignal == -1 {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[StrongTrend SELL] cci=%.1f,mfi=%.1f => rr=%.2f", cciVal, mfiVal, rr)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.BrightMagenta, "[StronglyTrending HOLD] %s: No trade triggered.", pair)
		return 0, nil

	// ------------------ TRANSITIONAL ------------------
	case models.Transitional:
		// e.g. cciSignal == 1 => CCI < cciOversold threshold, mfiSignal == 1 => MFI < mfiOversold => might catch a big contrarian trade
		if cciSignal == 1 && mfiSignal == 1 {
			target := middleBand
			stop := currentPrice * 0.98
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.Infof("[Transitional BUY] cci=%.1f,mfi=%.1f,RR=%.2f", cciVal, mfiVal, rr)
			if rr > cs.RiskRewardThreshold*0.8 {
				logger.Infof("[Transitional BUY triggered!]")
				return 1, nil
			}
		}
		// etc. for SELL if cciSignal == -1 && mfiSignal == -1
		logger.DebugColorf(logger.BrightYellow, "[Transitional HOLD]%s: no extreme condition found. (CCI=%.1f, MFI=%.1f)", pair, cciVal, mfiVal)
		return 0, nil

	// ------------------ TRENDING ------------------
	case models.Trending:
		if bullishConditions &&
			cciVal < cciOverbought &&
			cciVal > cciOversold &&
			mfiVal < mfiOverbought &&
			mfiVal > mfiOversold {

			target := upperBand
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[Trending BUY] cci=%.1f,mfi=%.1f,RR=%.2f", cciVal, mfiVal, rr)
				return 1, nil
			}
		}

		if bearishConditions && isActive &&
			cciVal > cciOverbought &&
			mfiVal > mfiOverbought {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[Trending SELL] cci=%.1f,mfi=%.1f => rr=%.2f", cciVal, mfiVal, rr)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.Yellow, "%s: No trending trade found. CCI=%.1f, MFI=%.1f", pair, cciVal, mfiVal)
		return 0, nil

	// ------------------ CHAOTIC ------------------
	case models.Chaotic:
		if currentPrice < lowerBand &&
			cciVal < cciOversold &&
			mfiVal < mfiOversold {
			logger.Infof("[Chaotic BUY] cci=%.1f,mfi=%.1f => oversold", cciVal, mfiVal)
			return 1, nil
		}
		logger.DebugColorf(logger.Yellow, "[CHAOTIC HOLD]%s: cci=%.1f, mfi=%.1f => no trade", pair, cciVal, mfiVal)
		return 0, nil

	// ------------------ RANGEBOUND ------------------
	case models.RangeBound:
		relaxedRR := cs.RiskRewardThreshold * 0.5

		// BUY near lower band if cciSignal=1 or mfiSignal=1
		if currentPrice <= lowerBand && (cciSignal == 1 || mfiSignal == 1) {
			target := middleBand
			stop := lowerBand * 0.99
			rr := cs.calcRR(currentPrice, stop, target, pair)
			if rr > relaxedRR {
				logger.Infof("[RangeBound BUY] cci=%.1f,mfi=%.1f,rr=%.2f", cciVal, mfiVal, rr)
				return 1, nil
			}
		}
		// SELL near upper band if cciSignal=-1 or mfiSignal=-1
		if currentPrice >= upperBand && isActive && (cciSignal == -1 || mfiSignal == -1) {
			target := middleBand
			stop := upperBand * 1.01
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > relaxedRR {
				logger.Infof("[RangeBound SELL] cci=%.1f,mfi=%.1f, rr=%.2f", cciVal, mfiVal, rr)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.Yellow, "[RANGE-BOUND HOLD]%s: cci=%.1f, mfi=%.1f => no trade", pair, cciVal, mfiVal)
		return 0, nil

	// ------------------ DEFAULT ------------------
	default:
		if bullishConditions &&
			cciVal < cciOverbought &&
			mfiVal < mfiOverbought {
			target := upperBand
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[Default BUY] cci=%.1f, mfi=%.1f => RR=%.2f", cciVal, mfiVal, rr)
				return 1, nil
			}
		}

		if bearishConditions && isActive &&
			cciVal > cciOverbought &&
			mfiVal > mfiOverbought {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("[Default SELL] cci=%.1f, mfi=%.1f => rr=%.2f", cciVal, mfiVal, rr)
				return -1, nil
			}
		}

		logger.DebugColorf(logger.Yellow, "[DEFAULT HOLD] %s => cci=%.1f, mfi=%.1f => no trade", pair, cciVal, mfiVal)
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

func (cs *CompoundStrategy) GetLatestData() LastData {
	return lastData
}

func (cs *CompoundStrategy) GetCandleInterval() string {
	return cs.CandleInterval
}

func (cs *CompoundStrategy) Validate() error {
	v := validator.New()
	return v.Struct(cs)
}
