package backtest

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/analysis"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"time"
)

type BacktestConfig struct {
	InitialBalances map[string]float64

	FeeRate float64

	TradingPairs []models.TradingPair

	HistoricalData map[string]map[string][]models.CandleStick

	Strategy interfaces.Strategy

	StartTime time.Time

	EndTime time.Time

	TimeStep time.Duration

	RiskPercentage float64

	MinNotional float64
}

type BacktestResult struct {
	FinalBalances map[string]float64

	Transactions []Transaction

	TotalTrades      int
	WinningTrades    int
	LosingTrades     int
	BreakEvenTrades  int
	TotalProfitLoss  float64
	WinRate          float64
	AverageProfit    float64
	AverageLoss      float64
	LargestProfit    float64
	LargestLoss      float64
	ProfitFactor     float64
	MaxDrawdown      float64
	SharpeRatio      float64
	StartingBalance  float64
	EndingBalance    float64
	PercentageReturn float64
}

type Runner struct {
	config         BacktestConfig
	client         *MockExchangeClient
	marketAnalyzer *analysis.MarketAnalyzer
}

func NewRunner(config BacktestConfig) *Runner {
	return &Runner{
		config: config,
		marketAnalyzer: analysis.NewMarketAnalyzer(analysis.MarketAnalyzer{
			EMAPeriods:               []int{9, 21, 55},
			ATRPeriod:                14,
			ADXPeriod:                14,
			HighVolatilityThreshold:  0.035,
			StrongTrendThreshold:     25,
			IchimokuConversionPeriod: 9,
			IchimokuBasePeriod:       26,
			IchimokuSpanBPeriod:      52,
			VolumeThreshold:          15000,
			FractalLookback:          20,
			MFIPeriod:                14,
			MFIOverbought:            80,
			MFIOversold:              20,
			CCIPeriod:                20,
			CCIOverbought:            100,
			CCIOversold:              -100,
		}),
	}
}

func (r *Runner) Run() (*BacktestResult, error) {

	r.client = NewMockExchangeClient(r.config.InitialBalances, r.config.FeeRate)

	for symbol, intervalData := range r.config.HistoricalData {
		for interval, candles := range intervalData {
			r.client.LoadHistoricalData(symbol, interval, candles)
		}
	}

	for _, pair := range r.config.TradingPairs {
		err := r.client.AddTradingPair(pair)
		if err != nil {
			return nil, fmt.Errorf("failed to add trading pair %s: %v", pair.Symbol, err)
		}
	}

	result := &BacktestResult{
		FinalBalances: make(map[string]float64),
		Transactions:  []Transaction{},
	}

	var startingBalance float64
	for _, balance := range r.config.InitialBalances {

		startingBalance += balance
	}
	result.StartingBalance = startingBalance

	currentTime := r.config.StartTime
	for currentTime.Before(r.config.EndTime) {

		for _, pair := range r.config.TradingPairs {

			candles, err := r.client.FetchCandles(pair.Symbol, r.config.Strategy.GetCandleInterval(), 100, true)
			if err != nil {
				logger.Warnf("Backtest: Failed to fetch candles for %s: %v", pair.Symbol, err)
				continue
			}

			marketState, atr, adx := r.marketAnalyzer.AnalyzeMarket(candles)
			logger.Debugf("Backtest: Market state for %s: %s (ATR=%.2f, ADX=%.2f)", pair.Symbol, marketState.String(), atr, adx)

			strategy := r.SuggestStrategy(marketState)

			signal, err := strategy.Calculate(candles, pair.Symbol, marketState, 24*time.Hour)
			if err != nil {
				logger.Warnf("Backtest: Failed to calculate signal for %s: %v", pair.Symbol, err)
				continue
			}

			if signal != 0 {
				err = r.executeTrade(pair.Symbol, signal, atr, adx)
				if err != nil {
					logger.Warnf("Backtest: Failed to execute trade for %s: %v", pair.Symbol, err)
				}
			}
		}

		currentTime = currentTime.Add(r.config.TimeStep)
		for _, pair := range r.config.TradingPairs {

			err := r.client.AdvanceTime(pair.Symbol, 1)
			if err != nil {
				logger.Warnf("Backtest: Failed to advance time for %s: %v", pair.Symbol, err)
			}
		}
	}

	result.Transactions = r.client.GetTransactions()
	result.FinalBalances = r.client.balances
	r.calculatePerformanceMetrics(result)

	return result, nil
}

func (r *Runner) executeTrade(symbol string, signal int, atr, adx float64) error {

	baseAsset, quoteAsset := extractAssets(symbol)

	currentPrice, err := r.client.GetCurrentPrice(symbol)
	if err != nil {
		return err
	}

	baseBalance, err := r.client.GetBalance(baseAsset)
	if err != nil {
		return err
	}

	quoteBalance, err := r.client.GetBalance(quoteAsset)
	if err != nil {
		return err
	}

	tradeAmount := r.calculateTradeAmount(signal, r.config.MinNotional, quoteBalance, baseBalance, symbol, atr, adx)
	if tradeAmount == 0 {
		return nil // Skip trade if amount is 0
	}

	if signal > 0 { // Buy

		amountStr := fmt.Sprintf("%.8f", tradeAmount)

		_, err = r.client.CreateOrder(symbol, "MARKET", "BUY", amountStr)
		if err != nil {
			return err
		}

		logger.Infof("Backtest: BUY %s: %.8f at price %.8f", symbol, tradeAmount, currentPrice)
	} else if signal < 0 { // Sell

		if baseBalance > 0 {

			amountStr := fmt.Sprintf("%.8f", tradeAmount)

			_, err = r.client.CreateOrder(symbol, "MARKET", "SELL", amountStr)
			if err != nil {
				return err
			}

			logger.Infof("Backtest: SELL %s: %.8f at price %.8f", symbol, tradeAmount, currentPrice)
		}
	}

	return nil
}

func (r *Runner) SuggestStrategy(marketState models.MarketState) interfaces.Strategy {

	switch marketState {
	case models.Trending:
		if r.config.Strategy != nil {
			return r.config.Strategy
		}
		return r.config.Strategy
	case models.Chaotic:
		if r.config.Strategy != nil {
			return r.config.Strategy
		}
		return r.config.Strategy
	case models.RangeBound:
		if r.config.Strategy != nil {
			return r.config.Strategy
		}
		return r.config.Strategy
	case models.Transitional:
		if r.config.Strategy != nil {
			return r.config.Strategy
		}
		return r.config.Strategy
	case models.StronglyTrending:
		if r.config.Strategy != nil {
			return r.config.Strategy
		}
		return r.config.Strategy
	default:
		return r.config.Strategy
	}
}

func (r *Runner) calculateTradeAmount(signal int, notional, quoteBalance, baseBalance float64, pair string, atr, adx float64) float64 {

	const (
		baseRiskPercentage = 0.1  // Risk 10% of quote balance as baseline risk per trade
		atrReference       = 1.0  // ATR reference level
		adxReference       = 25.0 // ADX reference for "moderate" trend
		minTradePercentage = 0.18 // Minimum 18% of quote balance
		maxTradePercentage = 0.5  // Maximum 50% of quote balance
		extraPercent       = 0.1  // Extra 10% above notional
	)

	basePosition := quoteBalance * baseRiskPercentage

	adxMultiplier := 1.0 + (adx-adxReference)*0.02
	if adxMultiplier < 0.5 {
		adxMultiplier = 0.5
	} else if adxMultiplier > 2.0 {
		adxMultiplier = 2.0
	}
	adjustedForADX := basePosition * adxMultiplier

	atrMultiplier := atrReference / atr
	if atrMultiplier < 0.5 {
		atrMultiplier = 0.5
	} else if atrMultiplier > 1.5 {
		atrMultiplier = 1.5
	}
	adjustedForVolatility := adjustedForADX * atrMultiplier

	tradePercentage := adjustedForVolatility / quoteBalance

	if tradePercentage < minTradePercentage {
		tradePercentage = minTradePercentage
	} else if tradePercentage > maxTradePercentage {
		tradePercentage = maxTradePercentage
	}

	finalAmount := tradePercentage * quoteBalance

	minBuyAbsolute := notional * (1 + extraPercent)

	if signal > 0 { // BUY Signal

		if finalAmount < minBuyAbsolute {
			finalAmount = minBuyAbsolute
		}

		if finalAmount > quoteBalance {
			finalAmount = quoteBalance
		}

		if quoteBalance < minBuyAbsolute {
			logger.Infof("Skipping BUY for %s: Insufficient balance. Need %.4f Have %.4f", pair, minBuyAbsolute, quoteBalance)
			return 0
		}

		logger.Debugf(
			"BUY %.2f %s | ATR=%.2f, ADX=%.2f, BaseRisk=%.2f%%, ADXMultiplier=%.2f, ATRMultiplier=%.2f, TradePercentage=%.2f%%, MinBuy=%.2f",
			finalAmount, pair, atr, adx, baseRiskPercentage*100, adxMultiplier, atrMultiplier, tradePercentage*100, minBuyAbsolute,
		)
		return finalAmount
	}

	logger.Debugf(
		"SELL %s: BaseBalance=%.2f, ATR=%.2f, ADX=%.2f, TradePercentage=%.2f%%",
		pair, baseBalance, atr, adx, tradePercentage*100,
	)
	return baseBalance
}

func (r *Runner) calculatePerformanceMetrics(result *BacktestResult) {

	result.TotalTrades = len(result.Transactions)

	var totalProfit, totalLoss float64
	result.LargestProfit = 0
	result.LargestLoss = 0

	positions := make(map[string]float64)  // symbol -> average entry price
	quantities := make(map[string]float64) // symbol -> quantity held

	for _, tx := range result.Transactions {
		if tx.Side == "BUY" {

			currentQty := quantities[tx.Symbol]
			currentAvgPrice := positions[tx.Symbol]

			newQty := currentQty + tx.Quantity
			newAvgPrice := (currentQty*currentAvgPrice + tx.Quantity*tx.Price) / newQty

			positions[tx.Symbol] = newAvgPrice
			quantities[tx.Symbol] = newQty
		} else if tx.Side == "SELL" {

			entryPrice := positions[tx.Symbol]
			exitPrice := tx.Price
			quantity := tx.Quantity

			pnl := (exitPrice - entryPrice) * quantity

			if pnl > 0 {
				result.WinningTrades++
				totalProfit += pnl
				if pnl > result.LargestProfit {
					result.LargestProfit = pnl
				}
			} else if pnl < 0 {
				result.LosingTrades++
				totalLoss += -pnl // Make loss positive for calculations
				if -pnl > result.LargestLoss {
					result.LargestLoss = -pnl
				}
			} else {
				result.BreakEvenTrades++
			}

			quantities[tx.Symbol] -= quantity
			if quantities[tx.Symbol] <= 0 {

				quantities[tx.Symbol] = 0
				positions[tx.Symbol] = 0
			}
		}
	}

	var endingBalance float64
	for _, balance := range result.FinalBalances {

		endingBalance += balance
	}
	result.EndingBalance = endingBalance

	result.TotalProfitLoss = totalProfit - totalLoss
	result.PercentageReturn = (endingBalance - result.StartingBalance) / result.StartingBalance * 100

	if result.WinningTrades > 0 {
		result.AverageProfit = totalProfit / float64(result.WinningTrades)
	}

	if result.LosingTrades > 0 {
		result.AverageLoss = totalLoss / float64(result.LosingTrades)
	}

	if result.TotalTrades > 0 {
		result.WinRate = float64(result.WinningTrades) / float64(result.TotalTrades) * 100
	}

	if totalLoss > 0 {
		result.ProfitFactor = totalProfit / totalLoss
	} else {
		result.ProfitFactor = totalProfit // Infinite profit factor if no losses
	}

}
