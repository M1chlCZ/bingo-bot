package strategies

import (
	"binance_bot/algos"
	db2 "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/models"
	"github.com/go-playground/validator/v10"
)

type CompoundStrategy struct {
	StrategyType              StrategyType                `validate:"required"`
	RSI                       *algos.RSIStrategy          `validate:"required"`
	MACD                      *algos.MACDStrategy         `validate:"required"`
	Stochastic                *algos.StochasticOscillator `validate:"required"`
	BollingerBands            *algos.BollingerBands       `validate:"required"`
	Ichimoku                  *algos.IchimokuStrategy     `validate:"required"`
	MarketState               models.MarketState          `validate:"required"`
	RiskRewardThreshold       float64                     `validate:"gte=0"`
	FeeRate                   float64                     `validate:"gte=0"`
	DesiredProfit             float64                     `validate:"gte=0"`
	HighestPriceFallOffMargin float64                     `validate:"gte=0"`
	CandleInterval            string                      `validate:"required"`
}

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
		logger.Errorf("Error checking active trade: %v", err)
		isActive = false
	}

	// Calculate Indicators
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating RSI: %w", err)
		return 0, err
	}

	_, macdSignalLine, macdVal, macdIndicatorSignal, err := cs.MACD.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating MACD: %v", err)
		return 0, err
	}

	stochasticK, _, err := cs.Stochastic.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Stochastic: %v", err)
		return 0, err
	}

	lowerBand, middleBand, upperBand, err := cs.BollingerBands.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Bollinger Bands: %v", err)
		return 0, err
	}

	ichimokuRes, err := cs.Ichimoku.Calculate(candles)
	if err != nil {
		logger.Errorf("Error calculating Ichimoku: %v", err)
		return 0, err
	}

	// If a trade exists, handle P/L logic first
	if trade != nil {
		breakevenPrice := trade.BuyPrice * (1 + cs.FeeRate)
		profitMargin := (currentPrice - trade.BuyPrice) / trade.BuyPrice * 100

		athPrice, err := db2.SQLiteDB.GetAth(trade.Symbol)
		if err != nil || currentPrice > athPrice {
			logger.Infof("Setting new HIGH price for %s: %.8f", trade.Symbol, currentPrice)
			if e := db2.SQLiteDB.SetUpdateAth(trade.Symbol, currentPrice); e != nil {
				logger.Errorf("Error updating ATH price for %s: %v", trade.Symbol, e)
			}
			athPrice = currentPrice
		}
		atlPrice, err := db2.SQLiteDB.GetAtl(trade.Symbol)
		if err != nil || currentPrice < atlPrice {
			logger.Infof("Setting new LOW price for %s: %.8f", trade.Symbol, currentPrice)
			if e := db2.SQLiteDB.SetUpdateAtl(trade.Symbol, currentPrice); e != nil {
				logger.Errorf("Error updating ATL price for %s: %v", trade.Symbol, e)
			}
			atlPrice = currentPrice
		}

		profitMarginATH := (currentPrice - athPrice) / athPrice * 100
		upliftFromAtl := (currentPrice - atlPrice) / atlPrice * 100

		logger.Infof("[Trade Monitoring] %s | Buy=%.2f | Current=%.2f | PM=%.2f%% | ATH=%.2f | PM ATH=%.2f%%",
			pair, trade.BuyPrice, currentPrice, profitMargin, athPrice, profitMarginATH)

		if profitMargin < 0 && currentPrice > atlPrice {
			logger.InfoColorf(logger.BrightYellow, "[ CurrentPrice is above ATL ] %s: Uplift from ATL (%.2f%%)", pair, upliftFromAtl)
		}

		if profitMargin < -cs.HighestPriceFallOffMargin {
			logger.InfoColorf(logger.BrightRed, "[PANIC SELL] %s: Price dropped below margin %.2f", pair, profitMargin)
			return -1, nil
		}

		if currentPrice < breakevenPrice {
			logger.InfoColorf(logger.BrightYellow, "[HOLD] %s: Below breakeven. Profit=%.2f%%", pair, profitMargin)
			return 0, nil
		}

		if profitMargin > cs.DesiredProfit {
			logger.InfoColorf(logger.BrightGreen, "[SELL] %s: Desired profit reached (%.2f%%)", pair, profitMargin)
			return -1, nil
		}

		logger.InfoColorf(logger.BrightBlack, "[HOLD] %s: PM=%.2f%% < Desired=%.2f%%", pair, profitMargin, cs.DesiredProfit)
		return 0, nil
	}

	// Determine signals commonly used across states
	bullishConditions := (ichimokuRes.Bullish || !ichimokuRes.Bearish) && macdIndicatorSignal == 1 && rsiVal < float64(cs.RSI.Overbought)
	bearishConditions := (ichimokuRes.Bearish || !ichimokuRes.Bullish) && macdIndicatorSignal == -1 && rsiVal > float64(cs.RSI.Oversold)

	//signalColor := Cyan
	//if bullishConditions {
	//	signalColor = Green
	//} else if bearishConditions {
	//	signalColor = Red
	//} else if ichimokuRes.Bullish || ichimokuRes.Bearish {
	//	signalColor = Yellow
	//}

	logger.DebugColorf(logger.BrightBlack, "%s | %s | Ichimoku=(B:%t, Br:%t), MACD=%d, RSIVal=%.2f, StochK=%.2f", pair, marketState.String(), ichimokuRes.Bullish, ichimokuRes.Bearish, macdIndicatorSignal, rsiVal, stochasticK)

	// Market-state based logic
	switch marketState {
	case models.StronglyTrending:
		if ichimokuRes.Bullish && macdIndicatorSignal == 1 && rsiVal < float64(cs.RSI.Overbought) && stochasticK < float64(cs.Stochastic.Overbought) {
			if currentPrice < middleBand {
				target := upperBand * 1.02
				stop := lowerBand
				rr := cs.calcRR(currentPrice, stop, target, pair)
				logger.InfoColorf(logger.BrightBlue, "[ %s STRONGLY TREDING RR=%.2f ]", pair, rr)
				if rr > cs.RiskRewardThreshold {
					logger.InfoColorf(logger.BrightGreen, "[ %s STRONGLY TREDING | UPTREND CONFIRMED RR=%.2f ]", pair, rr)
					return 1, nil
				}
			}
		}

		if ichimokuRes.Bearish && macdIndicatorSignal == -1 && rsiVal > float64(cs.RSI.Oversold) && stochasticK > float64(cs.Stochastic.Oversold) && isActive {
			if currentPrice > middleBand {
				target := lowerBand * 0.98
				stop := upperBand * 1.02
				rr := cs.calcRRForSell(currentPrice, stop, target, pair)
				if rr > cs.RiskRewardThreshold {
					logger.InfoColorf(logger.BrightRed, "[ %s Strongly Trending SELL| DOWNTREND CONFIRMED RR=%.2f ]", pair, rr)
					return -1, nil
				}
			}
		}
		logger.DebugColorf(logger.BrightMagenta, "[ Strongly Trending HOLD ] %s", pair)

		return 0, nil

	case models.Transitional:
		if rsiVal < 15 && macdIndicatorSignal >= 0 && stochasticK < float64(cs.Stochastic.Oversold) {
			target := middleBand
			stop := currentPrice * 0.98
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.BrightMagenta, "[ %s TRANSITIONAL RR=%.2f ]", pair, rr)
			if rr > (cs.RiskRewardThreshold * 0.8) {
				logger.InfoColorf(logger.BrightMagenta, "[ %s TRANSITIONAL BUY RR=%.2f ]", pair, rr)
				return 1, nil
			}
		}

		if rsiVal > 85 && macdIndicatorSignal <= 0 && stochasticK > float64(cs.Stochastic.Overbought) && isActive {
			target := middleBand
			stop := currentPrice * 1.02
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > (cs.RiskRewardThreshold * 0.8) {
				logger.InfoColorf(logger.Red, "[ TRANSITIONAL SELL ] Downtrend confirmed, RR=%.2f %s ", rr, pair)
				return -1, nil
			}
		}

		logger.DebugColorf(logger.BrightYellow, "[Transitional HOLD]%s: No extreme condition found.", pair)
		return 0, nil

	case models.Trending:
		if bullishConditions && macdVal > macdSignalLine && stochasticK < float64(cs.Stochastic.Overbought) {
			target := upperBand
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.Cyan, "[ %s TRENDING RR=%.2f ]", pair, rr)
			if rr > cs.RiskRewardThreshold {
				logger.InfoColorf(logger.BrightMagenta, "[ %s TRENDING BUY RR=%.2f ]", pair, rr)
				return 1, nil
			}
		}

		if bearishConditions && isActive && macdVal < macdSignalLine {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.InfoColorf(logger.Red, "[ TRENDING SELL ] Downtrend confirmed, RR=%.2f %s ", rr, pair)
				return -1, nil
			}
		}

		logger.DebugColorf(logger.Yellow, "%s: No optimal trend trade found.", pair)
		return 0, nil

	case models.Chaotic:
		if currentPrice > upperBand && int(stochasticK) > cs.Stochastic.Overbought && rsiVal > float64(cs.RSI.Overbought) {
			logger.InfoColorf(logger.Red, "[ CHAOTIC SELL ] Downtrend confirmed, RR=%.2f %s ", 0.0, pair)

			return -1, nil
		}
		if currentPrice < lowerBand && int(stochasticK) < cs.Stochastic.Oversold && rsiVal < float64(cs.RSI.Oversold) {
			logger.InfoColorf(logger.BrightMagenta, "[ %s CHAOTIC BUY ]", pair)

			return 1, nil
		}
		logger.DebugColorf(logger.Yellow, "[ CHAOTIC HOLD ]%s: No optimal trend trade found.", pair)
		return 0, nil

	case models.RangeBound:
		relaxedRR := cs.RiskRewardThreshold * 0.5
		if currentPrice <= lowerBand && macdVal > macdSignalLine && int(stochasticK) < cs.Stochastic.Oversold && int(rsiVal) < cs.RSI.Oversold {
			target := middleBand
			stop := lowerBand * 0.99
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.Cyan, "[ %s RANGE BOUND RR=%.2f ]", pair, rr)
			if rr > relaxedRR {
				logger.InfoColorf(logger.BrightMagenta, "[ %s RANGE-BOUND BUY RR=%.2f ]", pair, rr)
				return 1, nil
			}
		}

		if currentPrice >= upperBand && macdVal < macdSignalLine && int(stochasticK) > cs.Stochastic.Overbought && int(rsiVal) > cs.RSI.Overbought && isActive {
			target := middleBand
			stop := upperBand * 1.01
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > relaxedRR {
				logger.InfoColorf(logger.Red, "[ RANGE-BOUND SELL ] Downtrend confirmed, RR=%.2f %s ", 0.0, pair)
				return -1, nil
			}
		}
		logger.DebugColorf(logger.Yellow, "[ RANGE-BOUND HOLD ]%s: No optimal trend trade found.", pair)
		return 0, nil

	default:
		if bullishConditions && macdVal > macdSignalLine && stochasticK < float64(cs.Stochastic.Overbought) {
			target := upperBand
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.Cyan, "[ %s DEFAULT RR=%.2f ]", pair, rr)

			if rr > cs.RiskRewardThreshold {
				logger.InfoColorf(logger.BrightMagenta, "[ %s DEFAULT BUY RR=%.2f ]", pair, rr)

				return 1, nil
			}
		}

		if bearishConditions && isActive && macdVal < macdSignalLine && stochasticK > float64(cs.Stochastic.Oversold) {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.InfoColorf(logger.Red, "[ DEFAULT SELL ] Downtrend confirmed, RR=%.2f %s ", 0.0, pair)

				return -1, nil
			}
		}

		logger.DebugColorf(logger.Yellow, "[ DEFAULT HOLD ]%s: No optimal trend trade found.", pair)
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

func (cs *CompoundStrategy) GetCandleInterval() string {
	return cs.CandleInterval
}

func (cs *CompoundStrategy) Validate() error {
	v := validator.New()
	return v.Struct(cs)
}
