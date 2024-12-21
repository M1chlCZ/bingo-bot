package strategies

import (
	"binance_bot/algos"
	db2 "binance_bot/db"
	"binance_bot/logger"
	"binance_bot/models"
	"fmt"
	"github.com/go-playground/validator/v10"
)

const (
	Reset   = "\033[0m"
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"
)

// Color helpers
func colorGreen(s string) string  { return Green + s + Reset }
func colorRed(s string) string    { return Red + s + Reset }
func colorYellow(s string) string { return Yellow + s + Reset }
func colorCyan(s string) string   { return Cyan + s + Reset }

func colorGreenF(s string, params ...any) string  { return Green + fmt.Sprintf(s, params) + Reset }
func colorRedF(s string, params ...any) string    { return Red + fmt.Sprintf(s, params) + Reset }
func colorYellowF(s string, params ...any) string { return Yellow + fmt.Sprintf(s, params) + Reset }
func colorCyanF(s string, params ...any) string   { return Cyan + fmt.Sprintf(s, params) + Reset }

// highlightRR returns the RR value as a colored string depending on how close it is
// to the threshold.
func (cs *CompoundStrategy) highlightRR(rr float64) string {
	if rr > cs.RiskRewardThreshold {
		return colorGreen(fmt.Sprintf("%.2f", rr))
	} else if rr > cs.RiskRewardThreshold*0.8 {
		return colorYellow(fmt.Sprintf("%.2f", rr))
	}
	return colorRed(fmt.Sprintf("%.2f", rr))
}

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
	logger.Debugf("%sState: %v, Pair: %s, CurrentPrice: %.2f%s", colorCyan("["), marketState, pair, currentPrice, Reset)

	trade, _ := db2.SQLiteDB.GetActiveTrade(pair)
	isActive, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair)
	if err != nil {
		logger.Errorf("Error checking active trade: %v", err)
		isActive = false
	}

	// Calculate Indicators
	rsiVal, _, err := cs.RSI.Calculate(candles, pair)
	if err != nil {
		logger.Errorf("Error calculating RSI: %v", err)
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
			logger.Infof("Setting new HIGH price for %s: %.2f", trade.Symbol, currentPrice)
			if e := db2.SQLiteDB.SetUpdateAth(trade.Symbol, currentPrice); e != nil {
				logger.Errorf("Error updating ATH price for %s: %v", trade.Symbol, e)
			} else {
				athPrice = currentPrice
			}
		}
		atlPrice, err := db2.SQLiteDB.GetAtl(trade.Symbol)
		if err != nil || currentPrice < atlPrice {
			logger.Infof("Setting new LOW price for %s: %.2f", trade.Symbol, currentPrice)
			if e := db2.SQLiteDB.SetUpdateAtl(trade.Symbol, currentPrice); e != nil {
				logger.Errorf("Error updating ATL price for %s: %v", trade.Symbol, e)
			} else {
				atlPrice = currentPrice
			}
		}

		profitMarginATH := (currentPrice - athPrice) / athPrice * 100
		upliftFromAtl := (currentPrice - atlPrice) / atlPrice * 100

		logger.Infof("[Trade Monitoring] %s | Buy=%.2f | Current=%.2f | PM=%.2f%% | ATH=%.2f | PM ATH=%.2f%%",
			pair, trade.BuyPrice, currentPrice, profitMargin, athPrice, profitMarginATH)

		if profitMargin < 0 && atlPrice < currentPrice {
			logger.InfoColorf(BrightYellow, "[ CurrentPrice is above ATL ] %s: Uplift from ATL (%.4f%%)", pair, upliftFromAtl)
		}

		if profitMargin < -cs.HighestPriceFallOffMargin {
			logger.InfoColorf(BrightRed, "[PANIC SELL] %s: Price dropped below margin %.2f", pair, profitMargin)
			return -1, nil
		}

		if currentPrice < breakevenPrice {
			logger.InfoColorf(BrightYellow, "[HOLD] %s: Below breakeven. Profit=%.2f%%", pair, profitMargin)
			return 0, nil
		}

		if profitMargin > cs.DesiredProfit {
			logger.InfoColorf(BrightGreen, "[SELL] %s: Desired profit reached (%.2f%%)", pair, profitMargin)
			return -1, nil
		}

		logger.InfoColorf(BrightBlack, "[HOLD] %s: PM=%.2f%% < Desired=%.2f%%", pair, profitMargin, cs.DesiredProfit)
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

	logger.DebugColorf(BrightBlack, "%s | %s | Ichimoku=(B:%t, Br:%t), MACD=%d, RSIVal=%.2f, StochK=%.2f", pair, marketState.String(), ichimokuRes.Bullish, ichimokuRes.Bearish, macdIndicatorSignal, rsiVal, stochasticK)

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
					logger.Infof("%s %s: Uptrend confirmed, RR=%s",
						colorGreen("[StronglyTrending BUY]"), pair, cs.highlightRR(rr))
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
					logger.Infof("%s %s: Downtrend confirmed, RR=%s",
						colorRed("[StronglyTrending SELL]"), pair, cs.highlightRR(rr))
					return -1, nil
				}
			}
		}

		logger.Debugf("%s %s: No ideal setup found.",
			colorYellow("[StronglyTrending HOLD]"), pair)
		return 0, nil

	case models.Transitional:
		if rsiVal < 15 && macdIndicatorSignal >= 0 && stochasticK < float64(cs.Stochastic.Oversold) {
			target := middleBand
			stop := currentPrice * 0.98
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.BrightMagenta, "[ %s TRANSITIONAL RR=%.2f ]", pair, rr)
			if rr > (cs.RiskRewardThreshold * 0.8) {
				logger.Infof("%s %s: Uptrend confirmed, RR=%s",
					colorGreen("[Transitional BUY]"), pair, cs.highlightRR(rr))
				return 1, nil
			}
		}

		if rsiVal > 85 && macdIndicatorSignal <= 0 && stochasticK > float64(cs.Stochastic.Overbought) && isActive {
			target := middleBand
			stop := currentPrice * 1.02
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > (cs.RiskRewardThreshold * 0.8) {
				logger.Infof("%s %s: Downtrend confirmed, RR=%s",
					colorRed("[Transitional SELL]"), pair, cs.highlightRR(rr))
				return -1, nil
			}
		}

		logger.Debugf("%s[Transitional HOLD]%s %s: No extreme condition found.", colorYellow("["), Reset, pair)
		return 0, nil

	case models.Trending:
		if bullishConditions && macdVal > macdSignalLine && stochasticK < float64(cs.Stochastic.Overbought) {
			target := upperBand
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.Cyan, "[ %s TRENDING RR=%.2f ]", pair, rr)

			if rr > cs.RiskRewardThreshold {
				logger.Infof("%s %s: Good conditions, RR=%s",
					colorGreen("[Trending BUY]"), pair, cs.highlightRR(rr))
				return 1, nil
			}
		}

		if bearishConditions && isActive && macdVal < macdSignalLine {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("%s %s: Trend down continuation, RR=%s",
					colorRed("[Trending SELL]"), pair, cs.highlightRR(rr))
				return -1, nil
			}
		}

		logger.Debugf("%s %s: No optimal trend trade found.", colorYellow("[Trending HOLD]"), pair)
		return 0, nil

	case models.Chaotic:
		if currentPrice > upperBand && int(stochasticK) > cs.Stochastic.Overbought && rsiVal > float64(cs.RSI.Overbought) {
			logger.Infof("%s %s: Overbought mean-reversion attempt %s",
				colorRed("[Chaotic SELL]"), pair, Reset)
			return -1, nil
		}
		if currentPrice < lowerBand && int(stochasticK) < cs.Stochastic.Oversold && rsiVal < float64(cs.RSI.Oversold) {
			logger.Infof("%s %s: Overbought mean-reversion attempt %s",
				colorGreen("[Chaotic BUY]"), pair, Reset)
			return 1, nil
		}
		logger.Debugf("%s %s: No mean reversion opportunity. %s", colorYellow("[Chaotic HOLD]"), pair, Reset)
		return 0, nil

	case models.RangeBound:
		relaxedRR := cs.RiskRewardThreshold * 0.5
		if currentPrice <= lowerBand && macdVal > macdSignalLine && int(stochasticK) < cs.Stochastic.Oversold && int(rsiVal) < cs.RSI.Oversold {
			target := middleBand
			stop := lowerBand * 0.99
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.InfoColorf(logger.Cyan, "[ %s RANGE BOUND RR=%.2f ]", pair, rr)
			if rr > relaxedRR {
				logger.Infof("%s %s: Range low buy, RR=%s",
					colorGreen("[RangeBound BUY]"), pair, cs.highlightRR(rr))
				return 1, nil
			}
		}

		if currentPrice >= upperBand && macdVal < macdSignalLine && int(stochasticK) > cs.Stochastic.Overbought && int(rsiVal) > cs.RSI.Overbought && isActive {
			target := middleBand
			stop := upperBand * 1.01
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > relaxedRR {
				logger.Infof("%s %s: Range high sell, RR=%s",
					colorRed("[RangeBound SELL]"), pair, cs.highlightRR(rr))
				return -1, nil
			}
		}
		logger.Debugf("%s %s: No clear range trade. %s", colorYellow("[RangeBound HOLD]"), pair, Reset)
		return 0, nil

	default:
		if bullishConditions && macdVal > macdSignalLine && stochasticK < float64(cs.Stochastic.Overbought) {
			target := upperBand
			stop := lowerBand
			rr := cs.calcRR(currentPrice, stop, target, pair)
			logger.Infof(colorCyanF("[ DEFAULT RR=%s ]", rr))
			if rr > cs.RiskRewardThreshold {
				logger.Infof("%s %s: Balanced conditions, RR=%s",
					colorGreen("[Default BUY]"), pair, cs.highlightRR(rr))
				return 1, nil
			}
		}

		if bearishConditions && isActive && macdVal < macdSignalLine && stochasticK > float64(cs.Stochastic.Oversold) {
			target := lowerBand
			stop := upperBand
			rr := cs.calcRRForSell(currentPrice, stop, target, pair)
			if rr > cs.RiskRewardThreshold {
				logger.Infof("%s %s: Balanced conditions, RR=%s",
					colorRed("[Default SELL]"), pair, cs.highlightRR(rr))
				return -1, nil
			}
		}

		logger.Debugf("%s %s: No strong signal met RR criteria. %s", colorYellow("[Default HOLD]"), pair, Reset)
		return 0, nil
	}
}

func (cs *CompoundStrategy) calcRR(currentPrice, stop, target float64, pair string) float64 {
	risk := (currentPrice - stop) / currentPrice
	reward := (target - currentPrice) / currentPrice
	if risk <= 0 || reward <= 0 {
		return 0
	}
	rr := reward / risk
	return rr
}

func (cs *CompoundStrategy) calcRRForSell(currentPrice, stop, target float64, pair string) float64 {
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
