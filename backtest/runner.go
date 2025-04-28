package backtest

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/analysis"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"time"
)

// BacktestConfig holds the configuration for a backtest run
type BacktestConfig struct {
	// Initial balances for each asset
	InitialBalances map[string]float64
	// Trading fee rate (e.g., 0.001 for 0.1%)
	FeeRate float64
	// Trading pairs to include in the backtest
	TradingPairs []models.TradingPair
	// Historical data for each symbol and interval
	HistoricalData map[string]map[string][]models.CandleStick
	// Strategy to evaluate
	Strategy interfaces.Strategy
	// Start time for the backtest
	StartTime time.Time
	// End time for the backtest
	EndTime time.Time
	// Time step for each iteration (e.g., 1h, 4h, 1d)
	TimeStep time.Duration
	// Risk percentage for position sizing (0.01 = 1%)
	RiskPercentage float64
	// Minimum notional value for trades
	MinNotional float64
}

// BacktestResult holds the results of a backtest run
type BacktestResult struct {
	// Final balances for each asset
	FinalBalances map[string]float64
	// All transactions made during the backtest
	Transactions []Transaction
	// Performance metrics
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

// Runner is responsible for executing backtest simulations
type Runner struct {
	config BacktestConfig
	client *MockExchangeClient
	marketAnalyzer *analysis.MarketAnalyzer
}

// NewRunner creates a new backtest runner with the given configuration
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

// Run executes the backtest simulation
func (r *Runner) Run() (*BacktestResult, error) {
	// Initialize the mock client
	r.client = NewMockExchangeClient(r.config.InitialBalances, r.config.FeeRate)

	// Load historical data
	for symbol, intervalData := range r.config.HistoricalData {
		for interval, candles := range intervalData {
			r.client.LoadHistoricalData(symbol, interval, candles)
		}
	}

	// Add trading pairs
	for _, pair := range r.config.TradingPairs {
		err := r.client.AddTradingPair(pair)
		if err != nil {
			return nil, fmt.Errorf("failed to add trading pair %s: %v", pair.Symbol, err)
		}
	}

	// Initialize result
	result := &BacktestResult{
		FinalBalances: make(map[string]float64),
		Transactions:  []Transaction{},
	}

	// Calculate starting balance in quote asset (e.g., USDT)
	var startingBalance float64
	for _, balance := range r.config.InitialBalances {
		// For simplicity, we're just summing up all balances
		// In a real implementation, you might want to convert everything to a common denominator
		startingBalance += balance
	}
	result.StartingBalance = startingBalance

	// Main backtest loop
	currentTime := r.config.StartTime
	for currentTime.Before(r.config.EndTime) {
		// Process each trading pair
		for _, pair := range r.config.TradingPairs {
			// Fetch candles for the current time
			candles, err := r.client.FetchCandles(pair.Symbol, r.config.Strategy.GetCandleInterval(), 100, true)
			if err != nil {
				logger.Warnf("Backtest: Failed to fetch candles for %s: %v", pair.Symbol, err)
				continue
			}

			// Analyze market to determine market state
			marketState, atr, adx := r.marketAnalyzer.AnalyzeMarket(candles)
			logger.Debugf("Backtest: Market state for %s: %s (ATR=%.2f, ADX=%.2f)", pair.Symbol, marketState.String(), atr, adx)

			// Get strategy based on market state
			strategy := r.SuggestStrategy(marketState)

			// Calculate trading signal
			signal, err := strategy.Calculate(candles, pair.Symbol, marketState, 24*time.Hour)
			if err != nil {
				logger.Warnf("Backtest: Failed to calculate signal for %s: %v", pair.Symbol, err)
				continue
			}

			// Execute trade based on signal
			if signal != 0 {
				err = r.executeTrade(pair.Symbol, signal, atr, adx)
				if err != nil {
					logger.Warnf("Backtest: Failed to execute trade for %s: %v", pair.Symbol, err)
				}
			}
		}

		// Advance time
		currentTime = currentTime.Add(r.config.TimeStep)
		for _, pair := range r.config.TradingPairs {
			// Advance the mock client's time for each symbol
			err := r.client.AdvanceTime(pair.Symbol, 1)
			if err != nil {
				logger.Warnf("Backtest: Failed to advance time for %s: %v", pair.Symbol, err)
			}
		}
	}

	// Calculate final results
	result.Transactions = r.client.GetTransactions()
	result.FinalBalances = r.client.balances
	r.calculatePerformanceMetrics(result)

	return result, nil
}

// executeTrade executes a trade based on the signal
func (r *Runner) executeTrade(symbol string, signal int, atr, adx float64) error {
	// Extract base and quote assets
	baseAsset, quoteAsset := extractAssets(symbol)

	// Get current price
	currentPrice, err := r.client.GetCurrentPrice(symbol)
	if err != nil {
		return err
	}

	// Get balances
	baseBalance, err := r.client.GetBalance(baseAsset)
	if err != nil {
		return err
	}

	quoteBalance, err := r.client.GetBalance(quoteAsset)
	if err != nil {
		return err
	}

	// Calculate trade amount using the advanced method
	tradeAmount := r.calculateTradeAmount(signal, r.config.MinNotional, quoteBalance, baseBalance, symbol, atr, adx)
	if tradeAmount == 0 {
		return nil // Skip trade if amount is 0
	}

	if signal > 0 { // Buy
		// Format the amount as a string (as required by the client interface)
		amountStr := fmt.Sprintf("%.8f", tradeAmount)

		// Execute the buy
		_, err = r.client.CreateOrder(symbol, "MARKET", "BUY", amountStr)
		if err != nil {
			return err
		}

		logger.Infof("Backtest: BUY %s: %.8f at price %.8f", symbol, tradeAmount, currentPrice)
	} else if signal < 0 { // Sell
		// If we have a position, sell it
		if baseBalance > 0 {
			// Format the amount as a string (as required by the client interface)
			amountStr := fmt.Sprintf("%.8f", tradeAmount)

			// Execute the sell
			_, err = r.client.CreateOrder(symbol, "MARKET", "SELL", amountStr)
			if err != nil {
				return err
			}

			logger.Infof("Backtest: SELL %s: %.8f at price %.8f", symbol, tradeAmount, currentPrice)
		}
	}

	return nil
}

// SuggestStrategy selects a strategy based on the market state
func (r *Runner) SuggestStrategy(marketState models.MarketState) interfaces.Strategy {
	// Return the strategy based on the market state
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

// calculateTradeAmount calculates the trade amount based on market conditions
func (r *Runner) calculateTradeAmount(signal int, notional, quoteBalance, baseBalance float64, pair string, atr, adx float64) float64 {
	// Define baseline risk parameters
	const (
		baseRiskPercentage = 0.1  // Risk 10% of quote balance as baseline risk per trade
		atrReference       = 1.0  // ATR reference level
		adxReference       = 25.0 // ADX reference for "moderate" trend
		minTradePercentage = 0.18 // Minimum 18% of quote balance
		maxTradePercentage = 0.5  // Maximum 50% of quote balance
		extraPercent       = 0.1  // Extra 10% above notional
	)

	// Start with base position in quote currency = quoteBalance * baseRiskPercentage
	basePosition := quoteBalance * baseRiskPercentage

	// Adjust position size based on ADX
	adxMultiplier := 1.0 + (adx-adxReference)*0.02
	if adxMultiplier < 0.5 {
		adxMultiplier = 0.5
	} else if adxMultiplier > 2.0 {
		adxMultiplier = 2.0
	}
	adjustedForADX := basePosition * adxMultiplier

	// Adjust position size based on ATR (Volatility)
	atrMultiplier := atrReference / atr
	if atrMultiplier < 0.5 {
		atrMultiplier = 0.5
	} else if atrMultiplier > 1.5 {
		atrMultiplier = 1.5
	}
	adjustedForVolatility := adjustedForADX * atrMultiplier

	// Convert the absolute currency amount to fraction of the quote balance
	tradePercentage := adjustedForVolatility / quoteBalance

	// Ensure the tradePercentage is within our min/max
	if tradePercentage < minTradePercentage {
		tradePercentage = minTradePercentage
	} else if tradePercentage > maxTradePercentage {
		tradePercentage = maxTradePercentage
	}

	// Final amount to trade
	finalAmount := tradePercentage * quoteBalance

	// Enforce an absolute minimum buy: (exchangeMinNotional * (1 + extraPercent))
	minBuyAbsolute := notional * (1 + extraPercent)

	// Proceed with BUY or SELL logic
	if signal > 0 { // BUY Signal
		// If finalAmount is below that absolute minBuy, clamp it
		if finalAmount < minBuyAbsolute {
			finalAmount = minBuyAbsolute
		}

		// Still can't exceed the actual quoteBalance
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
	// SELL Signal
	// If we are selling, typically we sell all base balance
	logger.Debugf(
		"SELL %s: BaseBalance=%.2f, ATR=%.2f, ADX=%.2f, TradePercentage=%.2f%%",
		pair, baseBalance, atr, adx, tradePercentage*100,
	)
	return baseBalance
}

// calculatePerformanceMetrics calculates performance metrics for the backtest result
func (r *Runner) calculatePerformanceMetrics(result *BacktestResult) {
	// Calculate total trades
	result.TotalTrades = len(result.Transactions)

	// Initialize metrics
	var totalProfit, totalLoss float64
	result.LargestProfit = 0
	result.LargestLoss = 0

	// Track positions for P&L calculation
	positions := make(map[string]float64) // symbol -> average entry price
	quantities := make(map[string]float64) // symbol -> quantity held

	// Calculate trade metrics
	for _, tx := range result.Transactions {
		if tx.Side == "BUY" {
			// Update position
			currentQty := quantities[tx.Symbol]
			currentAvgPrice := positions[tx.Symbol]

			// Calculate new average price
			newQty := currentQty + tx.Quantity
			newAvgPrice := (currentQty*currentAvgPrice + tx.Quantity*tx.Price) / newQty

			positions[tx.Symbol] = newAvgPrice
			quantities[tx.Symbol] = newQty
		} else if tx.Side == "SELL" {
			// Calculate profit/loss
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

			// Update position
			quantities[tx.Symbol] -= quantity
			if quantities[tx.Symbol] <= 0 {
				// Position closed
				quantities[tx.Symbol] = 0
				positions[tx.Symbol] = 0
			}
		}
	}

	// Calculate ending balance
	var endingBalance float64
	for _, balance := range result.FinalBalances {
		// For simplicity, we're just summing up all balances
		// In a real implementation, you might want to convert everything to a common denominator
		endingBalance += balance
	}
	result.EndingBalance = endingBalance

	// Calculate overall metrics
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

	// Note: More sophisticated metrics like Sharpe ratio and max drawdown would require
	// time series data of equity curve, which we're not tracking in this simple implementation
}
