package bot

import (
	"binance_bot/analysis"
	db2 "binance_bot/db"
	"binance_bot/interfaces"
	"binance_bot/logger"
	"binance_bot/models"
	"binance_bot/strategies"
	"binance_bot/types"
	"log"
	"math"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// MultiPairTradingBot manages multiple trading pairs
type MultiPairTradingBot struct {
	exchange       interfaces.ExchangeClient
	marketAnalyzer *analysis.MarketAnalyzer
	analysisData   map[string]*analysis.PairAnalysis
	analysisLock   sync.Mutex
	interval       string
	pairs          map[string]*models.TradingPair
	pairStrategies map[string]interfaces.Strategy // Pair-specific strategies
	pairsMu        sync.RWMutex
	candles        map[string][]models.CandleStick
	candlesMu      sync.RWMutex
	wg             sync.WaitGroup
	stopCh         chan struct{}
	config         types.ConfigMultiTrading
}

// NewMultiPairTradingBot creates a new instance of MultiPairTradingBot
func NewMultiPairTradingBot(exchange interfaces.ExchangeClient, config *types.ConfigMultiTrading) *MultiPairTradingBot {
	if exchange == nil {
		log.Fatal("Exchange must be provided")
	}

	if config == nil {
		log.Fatal("Exchange and strategy must be provided")
	}

	if config.Default.IsZero() {
		log.Fatal("Default market state must be provided")
	}

	if config.Default.Enabled && config.Default.Strategy == nil {
		log.Fatal("Default strategy is enabled, strategy must be provided")
	}

	if config.Chaotic.Enabled && config.Chaotic.Strategy == nil {
		log.Fatal("Chaotic strategy is enabled, strategy must be provided")
	}

	if config.Trending.Enabled && config.Trending.Strategy == nil {
		log.Fatal("Trending strategy is enable, strategy must be provided")
	}

	if config.RangeBound.Enabled && config.RangeBound.Strategy == nil {
		log.Fatal("RangeBound strategy is enabled, strategy must be provided")
	}

	if config.AutoTrading && config.AnalyzerConfig == nil {
		log.Fatal("AutoTrading is enabled, AnalyzerConfig must be provided")
	}
	// TODO: Validate the strategy type, values, and other fields
	return &MultiPairTradingBot{
		exchange:       exchange,
		config:         *config,
		interval:       "1h", // only default for initial setup, not used in actual trading
		marketAnalyzer: config.AnalyzerConfig,
		pairs:          make(map[string]*models.TradingPair),
		analysisData:   make(map[string]*analysis.PairAnalysis),
		pairStrategies: make(map[string]interfaces.Strategy),
		stopCh:         make(chan struct{}),
	}
}

func (bot *MultiPairTradingBot) AddPair(pair *models.TradingPair) {
	bot.pairsMu.Lock()
	defer bot.pairsMu.Unlock()
	bot.analysisLock.Lock()
	defer bot.analysisLock.Unlock()

	bot.pairs[pair.Symbol] = pair

	// Fetch candles and analyze the market
	candles, err := bot.exchange.FetchCandles(pair.Symbol, bot.interval, 69)
	if err != nil {
		log.Printf("[WARN] Error fetching candles for %s: %v", pair.Symbol, err)
		return
	}
	anls, atr, adx := bot.marketAnalyzer.AnalyzeMarket(candles)
	strategy := bot.SuggestStrategy(anls)
	bot.analysisData[pair.Symbol] = &analysis.PairAnalysis{
		Pair:        pair,
		MarketState: anls,
		LastUpdated: time.Now().Unix(),
		ATR:         atr,
		ADX:         adx,
	}

	// Suggest a strategy based on the market state

	bot.pairStrategies[pair.Symbol] = strategy
	logger.Infof("Strategy assigned for %s:| Market State: %s", pair.Symbol, anls.String())
}

func (bot *MultiPairTradingBot) StartTrading() {
	if bot.stopCh == nil {
		bot.stopCh = make(chan struct{})
	}

	// Perform initial pair adjustment
	bot.performPairAdjustment()

	// Signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Launch market analysis updater as a separate goroutine
	go bot.updateMarketAnalysis()
	// Launch strategy updater as a separate goroutine
	go bot.updateStrategies()
	// Launch trading pair adjustment routine
	go bot.adjustTradingPairs()

	// Fetch all trading pairs from the exchange and add them (only for manually added pairs)
	//var wg sync.WaitGroup
	//tradingPairs := bot.exchange.GetTradingPairs()
	//for _, pair := range tradingPairs {
	//	wg.Add(1)
	//	go func(p types.TradingPair) {
	//		defer wg.Done()
	//		bot.AddPair(pair)
	//		logger.Infof("Successfully added trading pair: %s", pair.Symbol)
	//	}(*pair)
	//}
	//wg.Wait()

	logger.Infof("Trading pairs initialized. Starting trading loops...")

	// Launch trading routines for each pair
	bot.pairsMu.RLock()
	for _, pair := range bot.pairs {
		bot.wg.Add(1)
		go func(pair *models.TradingPair) {
			defer bot.wg.Done()
			bot.tradePair(pair)
		}(pair)
	}
	bot.pairsMu.RUnlock()

	// Wait for stop signal
	<-stop
	logger.Infof("Shutdown signal received. Stopping all trading...")

	// Signal all goroutines to stop
	close(bot.stopCh)

	// Wait for all goroutines to finish
	bot.wg.Wait()

	logger.Infof("Trading loops launched for all pairs.")
}

// Stop stops the trading bot
func (bot *MultiPairTradingBot) Stop() {
	close(bot.stopCh)
	bot.wg.Wait()
	logger.Warn("Trading bot stopped.")
}

func (bot *MultiPairTradingBot) isUptrend(candles []models.CandleStick) bool {
	if len(candles) < 50 { // Ensure enough candles for SMA calculation
		logger.Infof("Insufficient candles for trend detection. Expected 50, got %d\n", len(candles))
		return false
	}

	// Calculate the short-term and long-term SMAs
	shortSMA := bot.calculateSMA(candles, 20) // 20-period SMA
	longSMA := bot.calculateSMA(candles, 50)  // 50-period SMA

	// Compare the latest short-term SMA with the long-term SMA
	return shortSMA[len(shortSMA)-1] > longSMA[len(longSMA)-1]
}

// Helper function to calculate SMA
func (bot *MultiPairTradingBot) calculateSMA(candles []models.CandleStick, period int) []float64 {
	if len(candles) < period {
		return nil
	}

	sma := make([]float64, len(candles)-period+1)
	for i := 0; i <= len(candles)-period; i++ {
		sum := 0.0
		for j := 0; j < period; j++ {
			sum += candles[i+j].Close
		}
		sma[i] = sum / float64(period)
	}
	return sma
}

func (bot *MultiPairTradingBot) calculateTradeAmount(signal int, quoteBalance, baseBalance float64, pair string) float64 {
	if signal > 0 { // BUY Signal
		amount := math.Min(quoteBalance*0.25, quoteBalance)
		logger.Infof("BUY %.2f %s \n", amount, pair)
		return amount // Use 25% of quote balance
	} else if signal < 0 { // SELL Signal
		logger.Infof("SELL %s %.2f \n", pair, baseBalance)
		return baseBalance // Sell all base balance
	}
	return 0
}

func (bot *MultiPairTradingBot) calculateTradeAmountAdvance(signal int, quoteBalance, baseBalance float64, pair string, atr, adx float64) float64 {
	// Define thresholds and scaling factors
	const (
		atrScalingFactor     = 1.5  // Adjust trade size based on ATR
		adxStrengthThreshold = 25.0 // Threshold to consider a strong trend
		minTradePercentage   = 0.1  // Minimum 10% of quote balance
		maxTradePercentage   = 0.5  // Maximum 50% of quote balance
	)

	// Calculate base trade percentage based on ADX
	tradePercentage := minTradePercentage
	if adx > adxStrengthThreshold {
		tradePercentage = minTradePercentage + (adx-adxStrengthThreshold)*0.01
		if tradePercentage > maxTradePercentage {
			tradePercentage = maxTradePercentage
		}
	}

	// Adjust trade size further based on ATR
	volatilityAdjustedAmount := tradePercentage * quoteBalance * (atr / atrScalingFactor)

	if signal > 0 { // BUY Signal
		amount := math.Min(volatilityAdjustedAmount, quoteBalance)
		logger.Infof("BUY %.2f %s | ATR=%.2f, ADX=%.2f, TradePercentage=%.2f%% \n", amount, pair, atr, adx, tradePercentage*100)
		return amount
	} else if signal < 0 { // SELL Signal
		logger.Infof("SELL %s %.2f | ATR=%.2f, ADX=%.2f \n", pair, baseBalance, atr, adx)
		return baseBalance // Sell all base balance
	}

	return 0
}

func (bot *MultiPairTradingBot) tradePair(pair *models.TradingPair) {
	defer bot.wg.Done()

	// Tickers for trading and resetting daily counters
	tradeTicker := time.NewTicker(bot.config.TradingLoopInterval)
	defer tradeTicker.Stop()

	resetTicker := time.NewTicker(1 * time.Minute)
	defer resetTicker.Stop()

	logger.Infof("Started trading loop for %s", pair.Symbol)

	tradesToday := 0
	lastResetDay := time.Now().Day()

	for {
		select {
		case <-bot.stopCh:
			return

		case <-resetTicker.C:
			// Reset the daily trade counter at midnight
			currentDay := time.Now().Day()
			if currentDay != lastResetDay {
				logger.Infof("Resetting daily trade counter for %s. Previous trades: %d", pair.Symbol, tradesToday)
				tradesToday = 0
				lastResetDay = currentDay
			}

		case <-tradeTicker.C:
			// Fetch strategy and market analysis for the pair
			bot.pairsMu.RLock()
			strategy, hasStrategy := bot.pairStrategies[pair.Symbol]
			anls, hasAnalysis := bot.analysisData[pair.Symbol]
			bot.pairsMu.RUnlock()

			if !hasStrategy || !hasAnalysis {
				logger.Warnf("Missing strategy or analysis for %s. Skipping trade cycle.", pair.Symbol)
				continue
			}

			// Fetch candles
			candles, err := bot.exchange.FetchCandles(pair.Symbol, strategy.GetCandleInterval(), 69)
			if err != nil {
				logger.Errorf("Error fetching candles for %s | Sleeping for 3 minutes", pair.Symbol)
				continue
			}

			if bot.candles == nil {
				bot.candles = make(map[string][]models.CandleStick)
			}

			// Update shared candle storage
			bot.candlesMu.Lock()
			bot.candles[pair.Symbol] = candles
			bot.candlesMu.Unlock()

			// Detect trend
			isUptrend := bot.marketAnalyzer.IsUptrend(candles)

			// Calculate signal
			sngl, err := strategy.Calculate(candles, pair.Symbol, anls.MarketState)
			if err != nil {
				logger.Errorf("Error calculating signal for %s: %v", pair.Symbol, err)
				continue
			}

			if sngl == 0 {
				// No action required for HOLD signal
				continue
			}

			// Prevent overtrading
			//if tradesToday >= 25 {
			//	logger.Infof("Maximum daily trades reached for %s. Skipping further trades.", pair.Symbol)
			//	continue
			//}

			// Fetch balances
			quoteBalance, err := bot.exchange.GetBalance(pair.QuoteAsset)
			if err != nil {
				if pair.QuoteAsset != "" {
					logger.Errorf("Error fetching %s balance: %v", pair.QuoteAsset, err)

				}

				continue
			}

			baseBalance, err := bot.exchange.GetBalance(pair.BaseAsset)
			if err != nil {
				if pair.BaseAsset != "" {
					logger.Errorf("Error fetching %s balance: %v", pair.BaseAsset, err)
				}

				continue
			}

			// Get current price
			currentPrice := candles[len(candles)-1].Close

			// Determine trade size
			tradeAmount := bot.calculateTradeAmount(sngl, quoteBalance, baseBalance, pair.Symbol)
			if tradeAmount == 0 {
				logger.Warnf("Insufficient balance for %s trade. Skipping trade.", pair.Symbol)
				continue
			}

			tr := bot.calculateTradeAmountAdvance(sngl, quoteBalance, baseBalance, pair.Symbol, anls.ATR, anls.ADX)
			logger.Infof("Debug: Advance trade amount: %.8f", tr)

			if !bot.isMarketStateEnabled(anls.MarketState) {
				logger.Infof("Skipping trade for %s: %s market detected.", pair.Symbol, anls.MarketState.String())
				continue
			}

			// Execute trade based on signal
			if sngl > 0 { // BUY
				active, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair.Symbol)
				if err != nil {
					logger.Errorf("Error checking active trade for %s: %v", pair.Symbol, err)
					continue
				}
				if active {
					logger.Infof("Skipping BUY for %s: Active trade exists.", pair.Symbol)
					continue
				}
				logger.Infof("BUY signal for %s | Amount: %.4f | Price: %.4f | Quote Balance: %.4f",
					pair.Symbol, tradeAmount/currentPrice, currentPrice, quoteBalance)

				if !bot.handleBuy(pair, tradeAmount/currentPrice, currentPrice, quoteBalance) {
					//logger.Errorf("Error handling BUY for %s", pair.Symbol)
					continue
				}
			}
			if sngl < 0 { // SELL
				// Check if the market is in an uptrend before selling
				if isUptrend && sngl != -2 { // panic sell
					logger.Debugf("UPTREND signal for %s, cancelling sell", pair.Symbol)
					continue
				}
				logger.Infof("SELL signal for %s | Amount: %.4f | Price: %.4f | Base Balance: %.4f",
					pair.Symbol, tradeAmount, currentPrice, baseBalance)

				if !bot.handleSell(pair, tradeAmount, currentPrice, baseBalance) {
					logger.Errorf("Error handling SELL for %s", pair.Symbol)
					continue
				}
			} else {
				logger.Debugf("UPTREND signal for %s, cancelling sell", pair.Symbol)
			}

			tradesToday++
		}
	}
}

func (bot *MultiPairTradingBot) handleBuy(pair *models.TradingPair, tradeAmount, currentPrice, quoteBalance float64) bool {
	if tradeAmount*currentPrice < pair.MinNotional {
		//logger.Infof("BUY amount too small for %s. Adjusting to minimum notional.", pair.Symbol)
		tradeAmount = pair.MinNotional / currentPrice

		if tradeAmount*currentPrice > quoteBalance {
			//logger.Infof("Skipping BUY for %s: Insufficient USDT balance. Need %.2f Have %.2f", pair.Symbol, tradeAmount, quoteBalance)
			return false
		}
	}

	// Place Limit BUY Order
	limitPrice := currentPrice * 1.001 // 0.01% higher than current price
	limitOrderPrice := strconv.FormatFloat(limitPrice, 'f', pair.PricePrecision, 64)
	executedVolume := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)

	logger.Infof("Placing LIMIT BUY order for %s: Quantity=%.2f, Limit Price=%.2f", pair.Symbol, tradeAmount, limitPrice)
	orderID, err := bot.exchange.CreateLimitOrder(pair.Symbol, "BUY", executedVolume, limitOrderPrice)
	if err != nil {
		logger.Infof("Error placing LIMIT BUY order for %s: %v", pair.Symbol, err)
		return false
	}

	// Log trade in database
	err = db2.SQLiteDB.LogActiveTrade(pair.Symbol, limitPrice, tradeAmount)
	if err != nil {
		logger.Infof("Error logging BUY trade for %s: %v", pair.Symbol, err)
	}
	logger.Infof("Successfully placed LIMIT BUY order for %s. Order ID: %d", pair.Symbol, orderID)
	return true
}

// handleSell processes a SELL order
func (bot *MultiPairTradingBot) handleSell(pair *models.TradingPair, tradeAmount, currentPrice, baseBalance float64) bool {
	if tradeAmount*currentPrice < pair.MinNotional {
		logger.Infof("SELL amount too small for %s. Adjusting to minimum notional.", pair.Symbol)
		tradeAmount = pair.MinNotional / currentPrice

		if tradeAmount > baseBalance {
			logger.Infof("Skipping SELL for %s: Insufficient balance. Need %.2f Have %.2f", pair.Symbol, tradeAmount, baseBalance)
			return false
		}
	}

	bot.candlesMu.RLock()
	candles, exists := bot.candles[pair.Symbol]
	bot.candlesMu.RUnlock()

	bot.pairsMu.RLock()
	strategy, hasStrategy := bot.pairStrategies[pair.Symbol]
	bot.pairsMu.RUnlock()

	// Place Limit SELL Order
	limitPrice := currentPrice * 0.99 // 0.1% lower than current price
	limitOrderPrice := strconv.FormatFloat(limitPrice, 'f', pair.PricePrecision, 64)
	executedVolume := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)

	logger.Infof("Placing LIMIT SELL order for %s: Quantity=%.2f, Limit Price=%.2f", pair.Symbol, tradeAmount, limitPrice)
	orderID, err := bot.exchange.CreateLimitOrder(pair.Symbol, "SELL", executedVolume, limitOrderPrice)
	if err != nil {
		logger.Infof("Error placing LIMIT SELL order for %s: %v", pair.Symbol, err)
		return false
	}

	// Fetch active trade and log completed trade
	activeTrade, err := db2.SQLiteDB.GetActiveTrade(pair.Symbol)
	if err != nil {
		logger.Infof("Error fetching active trade for %s: %v", pair.Symbol, err)
		return false
	}
	profitLoss := (limitPrice - activeTrade.BuyPrice) * activeTrade.Quantity

	var rsiVal, macdVal, stochasticStr, stochasticSignal, lowerBound, middleBound, upperBound float64 = 0, 0, 0, 0, 0, 0, 0
	if hasStrategy {
		// Load the strategy and fetch indicators and save it to the database
		if rsiMacdStrategy, ok := strategy.(*strategies.CompoundStrategy); ok {
			if exists || len(candles) != 0 {
				rsiVal, _, _ = rsiMacdStrategy.GetRSI(candles, pair.Symbol)
				macdVal, _, _, _, _ = rsiMacdStrategy.GetMACD(candles)
				stochasticStr, stochasticSignal, _ = rsiMacdStrategy.GetStochastic(candles)
				lowerBound, middleBound, upperBound, _ = rsiMacdStrategy.GetBollingerBands(candles)
			}
		}
	}

	// Fetch indicator values
	logger.Debugf("Trade Indicators: RSI=%.2f, MACD=%.2f, Stochastic=%.2f (Signal=%.2f) OrderID %d", rsiVal, macdVal, stochasticStr, stochasticSignal, orderID)

	// Log the trade
	err = db2.SQLiteDB.LogCompletedTrade(pair.Symbol, activeTrade.BuyPrice, limitPrice, activeTrade.Quantity, profitLoss, rsiVal, macdVal, stochasticSignal, lowerBound, middleBound, upperBound)
	if err != nil {
		logger.Errorf("Error logging completed trade (SELL) for %s: %v", pair.Symbol, err)
	} else {
		logger.Infof("Successfully logged completed trade (SELL) for %s.", pair.Symbol)
	}

	// Remove active trade
	err = db2.SQLiteDB.RemoveActiveTrade(activeTrade.ID)
	if err != nil {
		logger.Errorf("Error removing active trade for %s: %v", pair.Symbol, err)
	} else {
		logger.Infof("Successfully removed active trade for %s.", pair.Symbol)
	}

	// Remove ath trade
	err = db2.SQLiteDB.RemoveAth(pair.Symbol)
	if err != nil {
		logger.Errorf("Error removing ath for %s: %v", pair.Symbol, err)
	} else {
		logger.Infof("Successfully removed atg for %s.", pair.Symbol)
	}

	return true
}

func (bot *MultiPairTradingBot) analyzeMarket(candles []models.CandleStick) (marketState models.MarketState, atr float64, adx float64) {
	return bot.marketAnalyzer.AnalyzeMarket(candles)
}

func (bot *MultiPairTradingBot) updateMarketAnalysis() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-bot.stopCh:
			logger.Infof("Stopping market analysis updater.")
			return
		case <-ticker.C:
			logger.Infof("Starting market analysis update...")

			bot.pairsMu.RLock()
			pairsToAnalyze := make([]*models.TradingPair, 0, len(bot.pairs))
			for _, pair := range bot.pairs {
				pairsToAnalyze = append(pairsToAnalyze, pair)
			}
			bot.pairsMu.RUnlock()

			for _, pair := range pairsToAnalyze {
				candles, err := bot.exchange.FetchCandles(pair.Symbol, bot.interval, 69)
				if err != nil {
					logger.Errorf("Error fetching candles for %s | Sleeping for 3 minutes", pair.Symbol)
					continue
				}

				marketState, atr, adx := bot.marketAnalyzer.AnalyzeMarket(candles)

				bot.pairsMu.Lock()
				bot.analysisData[pair.Symbol] = &analysis.PairAnalysis{
					Pair:        pair,
					MarketState: marketState,
					LastUpdated: time.Now().Unix(),
					ATR:         atr,
					ADX:         adx,
				}
				bot.pairsMu.Unlock()

				logger.Infof("Market analysis updated for %s: State=%v", pair.Symbol, marketState)
			}
			logger.Infof("Market analysis update completed.")
		}
	}
}

func (bot *MultiPairTradingBot) updateStrategies() {
	ticker := time.NewTicker(bot.config.AnalysisLoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bot.stopCh:
			logger.Infof("Stopping strategy updates.")
			return
		case <-ticker.C:
			logger.Infof("Updating strategies...")

			bot.pairsMu.RLock()
			pairsToUpdate := make([]string, 0, len(bot.pairs))
			for symbol := range bot.pairs {
				pairsToUpdate = append(pairsToUpdate, symbol)
			}
			bot.pairsMu.RUnlock()

			for _, symbol := range pairsToUpdate {
				bot.pairsMu.RLock()
				analysisData := bot.analysisData[symbol]
				bot.pairsMu.RUnlock()

				if analysisData == nil {
					logger.Warnf("Missing analysis data for %s. Skipping strategy update.", symbol)
					continue
				}
				bot.candlesMu.RLock()
				newStrategy := bot.SuggestStrategy(analysisData.MarketState)
				bot.candlesMu.RUnlock()

				logger.Infof("New market state for %s: %s", symbol, analysisData.MarketState.String())

				bot.pairsMu.Lock()
				bot.pairStrategies[symbol] = newStrategy
				bot.pairsMu.Unlock()
			}

			logger.Infof("Strategy updates completed.")
		}
	}
}

func (bot *MultiPairTradingBot) adjustTradingPairs() {

	ticker := time.NewTicker(bot.config.AnalysisLoopInterval) // Adjust pairs every 10 minutes
	defer ticker.Stop()

	for {
		select {
		case <-bot.stopCh:
			log.Println("[INFO] Stopping AdjustTradingPairs loop.")
			return
		case <-ticker.C:
			log.Println("[INFO] Periodic adjustment of trading pairs based on market analysis...")
			bot.performPairAdjustment()
		}
	}
}

func (bot *MultiPairTradingBot) performPairAdjustment() {
	logger.Infof("Adjusting trading pairs based on market analysis...")

	logger.Infof("%v", bot.config)

	// Fetch all USDT markets from the exchange
	allMarkets, err := bot.exchange.FetchMarkets(bot.config.IncludedBaseMarkets, bot.config.ExcludedQuoteMarkets, bot.config.ExcludedMarkets)
	if err != nil {
		logger.Errorf("Failed to fetch USDT markets: %v", err)
		return
	}

	// Analyze markets and categorize them
	trendingMarkets := make([]models.TradingPair, 0)

	for _, market := range allMarkets {
		candles, err := bot.exchange.FetchCandles(market.Symbol, bot.interval, 69)
		if err != nil {
			logger.Errorf("Error fetching candles for %s | Sleeping for 3 minutes", market.Symbol)
			return // Skip this market
		}

		// Analyze the market using ATR and ADX
		marketState, _, _ := bot.marketAnalyzer.AnalyzeMarket(candles)
		if bot.isMarketStateEnabled(marketState) {
			trendingMarkets = append(trendingMarkets, market)
		}
	}

	// Keep active trades to prevent removal
	activeTrades, err := db2.SQLiteDB.GetAllActiveTrades()
	if err != nil {
		logger.Errorf("Failed to fetch active trades: %v", err)
		return
	}

	activeTradePairs := make(map[string]bool)
	for _, trade := range activeTrades {
		activeTradePairs[trade.Symbol] = true
	}

	// Prepare lists for safe updates outside of the locked section
	pairsToRemove := make([]string, 0)
	pairsToAdd := make([]models.TradingPair, 0)

	bot.pairsMu.RLock()
	for symbol := range bot.pairs {
		if !containsPairs(trendingMarkets, symbol) && !activeTradePairs[symbol] {
			pairsToRemove = append(pairsToRemove, symbol)
		}
	}
	for _, market := range trendingMarkets {
		if _, exists := bot.pairs[market.Symbol]; !exists {
			pairsToAdd = append(pairsToAdd, market)
		}
	}
	bot.pairsMu.RUnlock()

	// Remove pairs no longer trending unless they have active trades
	bot.pairsMu.Lock()
	for _, symbol := range pairsToRemove {
		logger.Warnf("Removing pair %s as it is no longer trending and has no active trades.", symbol)
		delete(bot.pairs, symbol)
	}
	bot.pairsMu.Unlock()

	// Add new trending markets
	var wg sync.WaitGroup
	for _, pair := range pairsToAdd {
		wg.Add(1)
		go func(pair models.TradingPair) {
			defer wg.Done()
			logger.Infof("Adding pair %s", pair.Symbol)
			bot.AddPair(&pair)
		}(pair)
	}
	wg.Wait()

	logger.Infof("Trading pairs adjusted successfully.")

}

func (bot *MultiPairTradingBot) isMarketStateEnabled(state models.MarketState) bool {
	if bot.config.Trending.Enabled && state == models.Trending {
		return true
	}
	if bot.config.Chaotic.Enabled && state == models.Chaotic {
		return true
	}
	if bot.config.RangeBound.Enabled && state == models.RangeBound {
		return true
	}
	if bot.config.Default.Enabled && state == models.Default {
		return true
	}
	return false
}

// Helper function to check if a slice of TradingPair contains a symbol
func containsPairs(pairs []models.TradingPair, symbol string) bool {
	for _, pair := range pairs {
		if pair.Symbol == symbol {
			return true
		}
	}
	return false
}

func (bot *MultiPairTradingBot) SuggestStrategy(marketState models.MarketState) interfaces.Strategy {
	switch marketState {
	case models.Trending:
		return bot.config.Trending.Strategy
	case models.Chaotic:
		return bot.config.Chaotic.Strategy
	case models.RangeBound:
		return bot.config.RangeBound.Strategy
	default:
		return bot.config.Default.Strategy
	}
}
