package bot

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/M1chlCZ/bingo-bot/algos"
	"github.com/M1chlCZ/bingo-bot/analysis"
	"github.com/M1chlCZ/bingo-bot/config"
	db2 "github.com/M1chlCZ/bingo-bot/db"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"github.com/M1chlCZ/bingo-bot/utils"
)

type MultiPairTradingBot struct {
	exchange        interfaces.ExchangeClient
	marketAnalyzer  *analysis.MarketAnalyzer
	analysisData    sync.Map
	interval        string
	pairs           sync.Map
	pairStrategies  sync.Map
	candles         sync.Map
	wg              sync.WaitGroup
	config          config.MultiTrading
	stopCh          chan struct{}
	stopOnce        sync.Once
	analysisRunning atomic.Bool
	stopAllBuys     atomic.Bool
}

// NewMultiPairTradingBot creates a new instance of MultiPairTradingBot
func NewMultiPairTradingBot(exchange interfaces.ExchangeClient, config *config.MultiTrading) *MultiPairTradingBot {
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
		interval:       "1d", // only for initial analysis
		marketAnalyzer: config.AnalyzerConfig,
		stopCh:         make(chan struct{}),
	}
}

func (bot *MultiPairTradingBot) AddPair(pair *models.TradingPair, analysisData *models.AnalysisData) {
	bot.pairs.Store(pair.Symbol, pair)

	// Fetch candles and analyze the market
	strategy := bot.SuggestStrategy(analysisData.MarketState)
	bot.analysisData.Store(pair.Symbol, &analysis.PairAnalysis{
		Pair:        pair,
		MarketState: analysisData.MarketState,
		LastUpdated: time.Now().Unix(),
		ATR:         analysisData.ATR,
		ADX:         analysisData.ADX,
	})

	// Suggest a strategy based on the market state
	bot.pairStrategies.Store(pair.Symbol, strategy)
	logger.Infof("xStrategy assigned for %s:| Market State: %s", pair.Symbol, analysisData.MarketState.String())
}

func (bot *MultiPairTradingBot) StartTrading() {
	if bot.stopCh == nil {
		bot.stopCh = make(chan struct{})
	}

	bot.resyncTrades()

	// Only do these if AutoTrading is on and analyzer is present
	if bot.config.AutoTrading && bot.marketAnalyzer != nil {
		bot.performPairAdjustment()
		if bot.config.ThresholdStartTrading != 0 && bot.config.ThresholdStopTrading != 0 {
			bot.checkMarketMeltdown()
		}
		go bot.updateMarketAnalysis()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	utils.PrintMemStats()
	logger.Infof("Trading pairs initialized. Starting trading loops...")

	// launch loops
	bot.pairs.Range(func(_, v interface{}) bool {
		pair := v.(*models.TradingPair)
		bot.wg.Add(1)
		go func() {
			defer bot.wg.Done()
			bot.tradePair(pair)
		}()
		return true
	})

	<-stop
	logger.Infof("Shutdown signal received. Stopping all trading...")

	// close stopCh so loops exit
	bot.stopOnce.Do(func() { close(bot.stopCh) })
	bot.wg.Wait()
	logger.Infof("All trading loops stopped.")
}

func (bot *MultiPairTradingBot) Stop() {
	bot.stopOnce.Do(func() { close(bot.stopCh) })
	bot.wg.Wait()
	logger.Warn("Trading bot stopped.")
}

//nolint:funlen, has to be long like that
func (bot *MultiPairTradingBot) calculateTradeAmountAdvance(signal int, notional, quoteBalance, baseBalance float64, pair string, atr, adx float64) float64 {
	// Define baseline risk parameters
	const (
		baseRiskPercentage = 0.10 // Risk 10%
		minTradePercentage = 0.18 // 18%
		atrReference       = 1.0  // ATR reference level
		adxReference       = 25.0 // ADX reference for "moderate" trend
		maxTradePercentage = 0.5  // Maximum 50% of quote balance

		// want at least 10% above that notional
		extraPercent = 0.1
	)

	//  Start with base position in quote currency = quoteBalance * baseRiskPercentage
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
	atrMultiplier := 1.0
	if atr > 0 {
		atrMultiplier = atrReference / atr
	}
	if atrMultiplier < 0.5 {
		atrMultiplier = 0.5
	} else if atrMultiplier > 1.5 {
		atrMultiplier = 1.5
	}
	adjustedForVolatility := adjustedForADX * atrMultiplier

	//  Convert the absolute currency amount to fraction of the quote balance
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
	// so we don't get a NOTIONAL error. E.g., if min notional is $10, we want $10.50 as minimum.
	minBuyAbsolute := notional
	if minBuyAbsolute <= 0 {
		minBuyAbsolute = 10 // or from config
	}
	minBuyAbsolute *= 1 + extraPercent

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
			logger.Infof("Skipping BUY for %s: Insufficient USDC balance. Need %.4f Have %.4f", pair, minBuyAbsolute, quoteBalance)
			return 0
		}

		logger.Debugf(
			"BUY %.2f %s | ATR=%.2f, ADX=%.2f, BaseRisk=%.2f%%, ADXMultiplier=%.2f, ATRMultiplier=%.2f, TradePercentage=%.2f%%, MinBuy=%.2f",
			finalAmount, pair, atr, adx, baseRiskPercentage*100, adxMultiplier, atrMultiplier, tradePercentage*100, minBuyAbsolute,
		)
		return finalAmount
	}
	// SELL Signal
	// If we are selling, typically we sell all base balance or handle partial logic
	logger.Debugf(
		"SELL %s: BaseBalance=%.2f, ATR=%.2f, ADX=%.2f, TradePercentage=%.2f%%",
		pair, baseBalance, atr, adx, tradePercentage*100,
	)
	return baseBalance
}

//nolint:gocognit, core function
func (bot *MultiPairTradingBot) tradePair(pair *models.TradingPair) {

	// Tickers for trading and resetting daily counters
	tradeTicker := time.NewTicker(bot.config.TradingLoopInterval)
	defer tradeTicker.Stop()

	logger.Infof("Started trading loop for %s", pair.Symbol)

	for {
		select {
		case <-bot.stopCh:
			return

		case <-tradeTicker.C:
			// Fetch strategy and market analysis for the pair
			active, err := db2.SQLiteDB.IsCurrentlyActiveTrade(pair.Symbol)
			if err != nil {
				logger.Errorf("Error checking active trade for %s: %v", pair.Symbol, err)
				continue
			}
			logger.Debugf("Active trade status for %s: %t", pair.Symbol, active)
			if bot.analysisRunning.Load() && !active {
				// slows down the trading loop
				// so there is place for analysis to finish
				logger.Debugf("Analysis is running, skipping trade cycle for %s", pair.Symbol)
				continue
			}
			strg, hasStrategy := bot.pairStrategies.Load(pair.Symbol)
			anls, hasAnalysis := bot.analysisData.Load(pair.Symbol)

			if !hasStrategy || !hasAnalysis {
				logger.Warnf("Missing strategy or analysis for %s. Skipping trade cycle.", pair.Symbol)
				continue
			}

			analys, ok := anls.(*analysis.PairAnalysis)
			if !ok {
				logger.Warnf("Missing analysis for %s. Skipping trade cycle.", pair.Symbol)
				continue
			}

			strategy, ok := strg.(interfaces.Strategy)
			if !ok {
				logger.Warnf("Missing strategy for %s. Skipping trade cycle.", pair.Symbol)
				continue
			}

			// Fetch candles
			candles, err := bot.exchange.FetchCandles(pair.Symbol, strategy.GetCandleInterval(), 1000, false)
			if err != nil {
				logger.Errorf("Error fetching candles for %s ", pair.Symbol)
				continue
			}

			// Update shared candle storage
			bot.candles.Store(pair.Symbol, candles)

			// Detect trend
			isUptrend := false
			if bot.config.AutoTrading && bot.marketAnalyzer != nil {
				isUptrend = bot.marketAnalyzer.IsUptrend(candles)
			}

			// Calculate signal
			tradeSignal, err := strategy.Calculate(candles, pair.Symbol, analys.MarketState, bot.config.PendingBuyCoolDown)
			if err != nil {
				logger.Errorf("Error calculating signal for %s: %v", pair.Symbol, err)
				continue
			}

			// If stopAllBuys is set, force a SELL all to QUOTE BALANCE
			if bot.stopAllBuys.Load() {
				tradeSignal = -2
			}

			if tradeSignal == 0 {
				// No action required for HOLD signal
				continue
			}
			logger.Infof("Signal calculated for %s (%d)", pair.Symbol, tradeSignal)

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
			tradeAmount := bot.calculateTradeAmountAdvance(tradeSignal, pair.MinNotional, quoteBalance, baseBalance, pair.Symbol, analys.ATR, analys.ADX)
			if tradeAmount == 0 {
				continue
			}

			// Execute trade based on signal
			if tradeSignal > 0 { // BUY
				if !bot.isMarketStateEnabled(analys.MarketState) {
					logger.Infof("Skipping BUY for %s: %s market detected.", pair.Symbol, analys.MarketState.String())
					continue
				}

				//Prevent overtrading
				activeTradesTotal, err := db2.SQLiteDB.GetActiveTradesCount()
				if err != nil {
					logger.Errorf("Error fetching active trades count: %v", err)
					continue
				}
				todayTradeCount, err := db2.SQLiteDB.GetTodaysTradeCount()
				if err != nil {
					logger.Errorf("Error fetching today's active trades count: %v", err)
					continue
				}
				if analys.MarketState != models.StronglyTrending && analys.MarketState != models.Trending {
					logger.Infof("Today's trade count: %d | Total active trade count %d", todayTradeCount, activeTradesTotal)
					if activeTradesTotal >= bot.config.MaxTotalTrades || todayTradeCount >= bot.config.MaxDailyTrades {
						logger.InfoColorf(logger.Yellow, "Maximum daily trades reached. Skipping trade.")
						continue
					}
				}
				if active {
					logger.Debugf("Skipping BUY for %s: Active trade exists.", pair.Symbol)
					continue
				}
				logger.Infof("BUY signal for %s | Amount: %.4f | Price: %.4f | Quote Balance: %.4f",
					pair.Symbol, tradeAmount/currentPrice, currentPrice, quoteBalance)

				if !bot.handleBuy(pair, strategy, tradeAmount/currentPrice, currentPrice, quoteBalance) {
					return
				}
			}
			if tradeSignal < 0 { // SELL
				// Check if the market is in an uptrend before selling
				if isUptrend && tradeSignal != -2 { // tradeSignal == -2 > panic sell
					logger.InfoColorf(logger.BrightBlack, "UPTREND signal for %s, cancelling sell", pair.Symbol)
					continue
				}
				logger.Infof("SELL signal for %s | Amount: %.4f | Price: %.4f | Base Balance: %.4f",
					pair.Symbol, tradeAmount, currentPrice, baseBalance)
				if !bot.handleSell(pair, tradeAmount, currentPrice, baseBalance) {
					logger.Errorf("Error handling SELL for %s", pair.Symbol)
					continue
				}
			}
		}
	}
}

func (bot *MultiPairTradingBot) handleBuy(pair *models.TradingPair, strategy interfaces.Strategy, tradeAmount, currentPrice, quoteBalance float64) bool {
	if tradeAmount*currentPrice < pair.MinNotional {
		tradeAmount = pair.MinNotional

		if tradeAmount > quoteBalance {
			logger.Infof("Skipping BUY for %s: Insufficient USDC balance. Need %.4f Have %.4f", pair.Symbol, tradeAmount, quoteBalance)
			return false
		}
	}

	// Place Limit BUY Order
	limitPrice := currentPrice * 1.000

	executedVolume := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)

	logger.Infof("Placing MARKET BUY order for %s: Quantity=%.2f, Limit Price=%.2f, Base amount %.4f", pair.Symbol, tradeAmount, limitPrice, tradeAmount*currentPrice)
	orderID, price, err := bot.exchange.CreateMarketOrder(pair.Symbol, "BUY", executedVolume)
	if err != nil {
		logger.Infof("Error placing LIMIT BUY order for %s: %v", pair.Symbol, err)
		return false
	}
	var rsiVal, macdVal, stochasticStr, stochasticSignal, lowerBound, middleBound, upperBound, ichimokuKijun, ichimokuTenkan, mfi, cci float64 = 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0
	if rsiMacdStrategy, ok := strategy.(*strategies.CompoundStrategy); ok {
		lastData := rsiMacdStrategy.GetLatestData()
		rsiVal = lastData.RSIVal
		macdVal = lastData.MacdLine
		stochasticStr = lastData.StochasticK
		stochasticSignal = lastData.StochasticD
		lowerBound = lastData.LowerBand
		middleBound = lastData.MiddleBand
		upperBound = lastData.UpperBand
		mfi = lastData.MFIVal
		cci = lastData.CCIVal
		ichimokuKijun = lastData.IchimokuRes.Kijun
		ichimokuTenkan = lastData.IchimokuRes.Tenkan
	} else {
		logger.Errorf("Error getting strategy for %s", pair.Symbol)
	}

	logger.Debugf("Trade Indicators: RSI=%.2f, MACD=%.2f, Stochastic=%.2f (Signal=%.2f) OrderID %d", rsiVal, macdVal, stochasticStr, stochasticSignal, orderID)

	// Log trade in database
	err = db2.SQLiteDB.LogActiveTrade(pair.Symbol, price, tradeAmount, orderID, rsiVal, macdVal, stochasticSignal, lowerBound, middleBound, upperBound, mfi, cci, ichimokuTenkan, ichimokuKijun)
	if err != nil {
		logger.Infof("Error logging BUY trade for %s: %v", pair.Symbol, err)
	}
	logger.Infof("Successfully placed LIMIT BUY order for %s. Order ID: %d", pair.Symbol, orderID)
	return true
}

// handleSell processes a SELL order
//
//nolint:funlen, its okay
func (bot *MultiPairTradingBot) handleSell(pair *models.TradingPair, tradeAmount, currentPrice, baseBalance float64) bool {
	if tradeAmount*currentPrice < pair.MinNotional {
		tradeAmount = pair.MinNotional / currentPrice

		if tradeAmount > baseBalance {
			logger.Infof("Skipping SELL for %s: Insufficient balance. Need %.8f Have %.8f", pair.Symbol, tradeAmount, baseBalance)
			return false
		}
	}

	// Place Limit SELL Order
	limitPrice := currentPrice
	executedVolume := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)

	logger.Infof("Placing LIMIT SELL order for %s: Quantity=%.4f, Limit Price=%.4f, Quote Amount %.4f, MinNotional %.4f", pair.Symbol, tradeAmount, limitPrice, tradeAmount*currentPrice, pair.MinNotional)
	orderID, price, err := bot.exchange.CreateMarketOrder(pair.Symbol, "SELL", executedVolume)
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
	profitLoss := (price - activeTrade.BuyPrice) * activeTrade.Quantity

	var ichimoku algos.IchimokuResult

	rsiVal := activeTrade.RSI
	macdVal := activeTrade.Macd
	stochasticSignal := activeTrade.Stochastic
	lowerBound := activeTrade.LowerBound
	middleBound := activeTrade.MiddleBound
	upperBound := activeTrade.UpperBound
	mfi := activeTrade.MFI
	cci := activeTrade.CCI
	ichimoku.Tenkan = activeTrade.IchimokuTenkan
	ichimoku.Kijun = activeTrade.IchimokuKijun

	// Fetch indicator values
	logger.Debugf("Trade Indicators: RSI=%.2f, MACD=%.2f, Stochastic=%.2f (Signal=%.2f) OrderID %d", rsiVal, macdVal, stochasticSignal, stochasticSignal, orderID)

	// Log the trade
	err = db2.SQLiteDB.LogCompletedTrade(pair.Symbol, activeTrade.BuyPrice, price, activeTrade.Quantity, profitLoss, rsiVal, macdVal, stochasticSignal, lowerBound, middleBound, upperBound, mfi, cci, ichimoku.Tenkan, ichimoku.Kijun, activeTrade.Timestamp.Unix(), activeTrade.GetOrderID())
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

	// Remove atl trade
	err = db2.SQLiteDB.RemoveAtl(pair.Symbol)
	if err != nil {
		logger.Errorf("Error removing ath for %s: %v", pair.Symbol, err)
	} else {
		logger.Infof("Successfully removed atg for %s.", pair.Symbol)
	}

	return true
}

func (bot *MultiPairTradingBot) updateMarketAnalysis() {
	ticker := time.NewTicker(bot.config.AnalysisLoopInterval)
	defer ticker.Stop()

	for {
		select {
		case <-bot.stopCh:
			logger.Infof("Stopping market analysis updater.")
			return
		case <-ticker.C:
			if bot.analysisRunning.Load() {
				logger.Debugf("Market analysis is already running, skipping this cycle.")
				continue
			}
			bot.analysisRunning.Store(true)
			logger.Infof("Starting market analysis update...")

			bot.performPairAdjustment()

			pairsToAnalyze := make([]*models.TradingPair, 0)
			bot.pairs.Range(func(_, v interface{}) bool {
				pairsToAnalyze = append(pairsToAnalyze, v.(*models.TradingPair))
				return true
			})

			logger.Debugf("/// Pairs to analyze: %d", len(pairsToAnalyze))

			localCandles := make(map[string][]models.CandleStick)
			for _, pair := range pairsToAnalyze {
				strg, _ := bot.pairStrategies.Load(pair.Symbol)
				strategy, ok := strg.(interfaces.Strategy)
				if !ok {
					logger.Warnf("Missing strategy for %s. Skipping trade cycle.", pair.Symbol)
					continue
				}
				candles, err := bot.exchange.FetchCandles(pair.Symbol, strategy.GetCandleInterval(), 100, true)
				if err != nil {
					logger.Errorf("Error fetching candles for %s: %v", pair.Symbol, err)
					continue
				}
				marketState, atr, adx := bot.marketAnalyzer.AnalyzeMarket(candles)
				logger.Debugf("/// Market analysis for %s: State=%v, ATR=%.4f, ADX=%.4f", pair.Symbol, marketState, atr, adx)
				analysisData := analysis.PairAnalysis{
					Pair:        pair,
					MarketState: marketState,
					LastUpdated: time.Now().Unix(),
					ATR:         atr,
					ADX:         adx,
				}
				newStrategy := bot.SuggestStrategy(analysisData.MarketState)
				localCandles[pair.Symbol] = candles

				if newStrategy.GetMarketState() == strategy.GetMarketState() {
					logger.Infof("No change in strategy for %s: Market State=%v", pair.Symbol, marketState)
					continue
				}

				bot.analysisData.Store(pair.Symbol, &analysisData)
				bot.pairStrategies.Store(pair.Symbol, newStrategy)

				logger.Infof("Market analysis updated for %s: State=%v", pair.Symbol, marketState)
			}
			bot.checkEarlyWarning(localCandles)
			logger.Infof("Market analysis update completed.")
			bot.analysisRunning.Store(false)
		}
	}
}

//nolint:funlen, gocognit, gocyclo shh, all okay
func (bot *MultiPairTradingBot) performPairAdjustment() {
	logger.Infof("Adjusting trading pairs based on market analysis...")

	// Fetch all USDT markets from the exchange
	allMarkets, err := bot.exchange.FetchMarkets(bot.config.IncludedBaseMarkets, bot.config.ExcludedQuoteMarkets, bot.config.ExcludedMarkets)
	if err != nil {
		logger.Errorf("Failed to fetch USDT markets: %v", err)
		return
	}

	// Analyze markets and categorize them
	enabledMarkets := make([]models.TradingPair, 0)
	analysisData := make(map[string]models.AnalysisData)
	var wgm sync.WaitGroup
	wgm.Add(len(allMarkets))
	for _, market := range allMarkets {
		go func(market models.TradingPair) {
			defer wgm.Done()
			candles, err := bot.exchange.FetchCandles(market.Symbol, bot.interval, 100, false)
			if err != nil {
				logger.Errorf("Error fetching candles for %s | Sleeping for 3 minutes", market.Symbol)
				return // Skip this market
			}
			isOkay, err := bot.exchange.IsTickerTooNew(market.Symbol)
			if err != nil {
				logger.Errorf("Error checking if ticker is too new for %s: %v", market.Symbol, err)
				return
			}
			if !isOkay {
				logger.Infof("Skipping trading pair: %s Too new for trading", market.Symbol)
				return
			}
			// Analyze the market using ATR and ADX
			marketState, atr, adx := bot.marketAnalyzer.AnalyzeMarket(candles)
			analysisData[market.Symbol] = models.AnalysisData{
				MarketState: marketState,
				ATR:         atr,
				ADX:         adx,
			}
			if bot.isMarketStateEnabled(marketState) {
				enabledMarkets = append(enabledMarkets, market)
			}
			logger.Debugf("||| Market analysis for %s: State=%v, ATR=%.4f, ADX=%.4f", market.Symbol, marketState, atr, adx)
		}(market)
	}

	wgm.Wait()

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

	// Add active trading pairs on program start, regardless of their market state
	if utils.LenSyncMap(&bot.pairs) == 0 {
		for _, market := range enabledMarkets {
			pairsToAdd = append(pairsToAdd, market)
		}
		for _, market := range allMarkets {
			if activeTradePairs[market.Symbol] && !containsPairs(pairsToAdd, market.Symbol) {
				logger.Infof("Adding pair %s as it has active trades (even if not enabled).", market.Symbol)
				pairsToAdd = append(pairsToAdd, market)
			}
		}
	}

	// On subsequent runs, remove pairs that are no longer trending and have no active trades
	if utils.LenSyncMap(&bot.pairs) != 0 {
		bot.pairs.Range(func(k, _ any) bool {
			symbol := k.(string)
			if !containsPairs(enabledMarkets, symbol) && !activeTradePairs[symbol] {
				pairsToRemove = append(pairsToRemove, symbol)
			}
			return true
		})
		// On subsequent runs, add new enabled markets
		for _, market := range enabledMarkets {
			if _, exists := bot.pairs.Load(market.Symbol); !exists {
				pairsToAdd = append(pairsToAdd, market)
			}
		}

		// Remove pairs no longer trending unless they have active trades
		for _, symbol := range pairsToRemove {
			logger.Warnf("Removing pair %s as it is no longer in enabled market state and has no active trades.", symbol)
			bot.pairs.Delete(symbol)
		}
	}

	logger.Infof("------")
	logger.InfoColorf(logger.BrightBlack, "Removed %d active trades", len(pairsToRemove))
	numPairs := 0
	// Add markets to the bot for trading
	var wg sync.WaitGroup
	for _, pair := range pairsToAdd {
		data, ok := analysisData[pair.Symbol]
		if !ok {
			logger.Warnf("Missing analysis data for %s. Skipping pair addition.", pair.Symbol)
			continue
		}
		numPairs++
		wg.Add(1)
		go func(pair models.TradingPair) {
			defer wg.Done()
			logger.Debugf("Adding pair %s", pair.Symbol)
			bot.AddPair(&pair, &data)
		}(pair)
	}
	wg.Wait()

	logger.InfoColorf(logger.BrightBlack, "Added %d new active trades", numPairs)
	logger.Infof("------")

	logger.Infof("Trading pairs adjusted successfully.")
}

func (bot *MultiPairTradingBot) isMarketStateEnabled(state models.MarketState) bool {
	switch state {
	case models.Trending:
		return bot.config.Trending.Enabled
	case models.Chaotic:
		return bot.config.Chaotic.Enabled
	case models.RangeBound:
		return bot.config.RangeBound.Enabled
	case models.Default:
		return bot.config.Default.Enabled
	case models.Transitional:
		return bot.config.Transitional.Enabled
	case models.StronglyTrending:
		return bot.config.StronglyTrending.Enabled
	default:
		return false
	}
}

func (bot *MultiPairTradingBot) resyncTrades() {
	tradesActiveDB, err := db2.SQLiteDB.GetAllActiveTrades()
	if err != nil {
		logger.Errorf("Failed to fetch active trades: %v", err)
		return
	}

	for _, trade := range tradesActiveDB {
		trades, err := bot.exchange.FetchActiveTrades(trade.Symbol)
		if err != nil {
			logger.Errorf("Failed to fetch active trades for %s: %v", trade.Symbol, err)
			continue
		}

		if len(trades) == 0 {
			log.Printf("No active trades found on Binance for symbol: %s", trade.Symbol)
			continue
		}

		lastTrade := trades[len(trades)-1]

		// Check if the trade is a sell and needs resyncing
		if lastTrade.OrderListId == -1 && !lastTrade.IsBuyer {
			log.Printf("Resyncing trade for symbol: %s", trade.Symbol)

			sellPrice, err := strconv.ParseFloat(lastTrade.Price, 64)
			if err != nil {
				logger.Errorf("Failed to parse sell price for %s: %v", trade.Symbol, err)
				continue
			}

			quantity := trade.Quantity
			profitLoss := (sellPrice - trade.BuyPrice) * quantity

			err = db2.SQLiteDB.LogCompletedTrade(trade.Symbol, trade.BuyPrice, sellPrice, quantity, profitLoss, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, lastTrade.Time, lastTrade.OrderID)
			if err != nil {
				logger.Errorf("Error logging completed trade for %s: %v", trade.Symbol, err)
				continue
			}

			err = db2.SQLiteDB.RemoveActiveTrade(trade.ID)
			if err != nil {
				logger.Errorf("Error removing active trade for %s: %v", trade.Symbol, err)
				continue
			}

			log.Printf("Successfully resynced trade for %s. Completed trade ID: %d", trade.Symbol, lastTrade.ID)
		}
	}
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
	// Return the strategy based on the market state
	// Clone() is called because we need new instances of the strategy for each pair
	// This prevents wrong data being shared between pairs

	switch marketState {
	case models.Trending:
		return bot.config.Trending.Strategy.Clone()

	case models.Chaotic:
		return bot.config.Chaotic.Strategy.Clone()

	case models.RangeBound:
		return bot.config.RangeBound.Strategy.Clone()

	case models.Transitional:
		return bot.config.Transitional.Strategy.Clone()

	case models.StronglyTrending:
		return bot.config.StronglyTrending.Strategy.Clone()

	default:
		return bot.config.Default.Strategy.Clone()
	}
}

// checkEarlyWarning checks for early warning signs of a market meltdown
func (bot *MultiPairTradingBot) checkEarlyWarning(candlesMap map[string][]models.CandleStick) {
	if (bot.config.ThresholdStartTrading == 0) || (bot.config.ThresholdStopTrading == 0) {
		return
	}
	var meltdownCount int
	total := len(candlesMap)

	for symbol, cset := range candlesMap {
		if len(cset) < 10 {
			// skip
			continue
		}
		// measure performance over last 5 candles vs. prev 5
		perf := bot.measureShortTermPerformance(cset, 5)
		if perf < -5.0 { // if dropped >2% in last N bars
			logger.InfoColorf(logger.BrightRed, "[%s] Meltdown detected: %.2f%% drop in last 5 bars", symbol, perf)
			meltdownCount++
		}
	}

	if total == 0 {
		logger.Warn("Market meltdown check skipped: no pairs")
		return
	}
	meltdownRatio := float64(meltdownCount) / float64(total)
	if meltdownRatio >= bot.config.ThresholdStopTrading {
		logger.Errorf("Market meltdown detected: %.2f%% of pairs are in meltdown state", meltdownRatio*100)
		bot.stopAllBuys.Store(true)
	}

	if meltdownRatio < bot.config.ThresholdStartTrading {
		logger.Infof("Market meltdown resolved: %.2f%% of pairs are in meltdown state", meltdownRatio*100)
		bot.stopAllBuys.Store(false)
	}
	logger.Infof("Market meltdown check completed. %.2f%% of pairs are in meltdown state", meltdownRatio*100)
}

// checkMarketMeltdown checks for a market meltdown, before trading starts
func (bot *MultiPairTradingBot) checkMarketMeltdown() {
	if (bot.config.ThresholdStartTrading == 0) || (bot.config.ThresholdStopTrading == 0) {
		return
	}
	logger.Infof("Checking for market meltdown...")
	localCandles := make(map[string][]models.CandleStick)
	bot.pairs.Range(func(_, v interface{}) bool {
		pair := v.(*models.TradingPair)
		cset, errC := bot.exchange.FetchCandles(pair.Symbol, bot.interval, 100, false)
		if errC == nil {
			localCandles[pair.Symbol] = cset
		}
		return true
	})

	bot.checkEarlyWarning(localCandles)
	logger.Infof("Market meltdown check completed.")
}

func (bot *MultiPairTradingBot) measureShortTermPerformance(candles []models.CandleStick, shortPeriod int) float64 {
	n := len(candles)
	if n < shortPeriod*2 {
		// Not enough candles to compare last X vs. previous X
		return 0
	}

	// Sum of the last 'shortPeriod' closes
	var lastSum float64
	for i := n - shortPeriod; i < n; i++ {
		lastSum += candles[i].Close
	}
	lastAvg := lastSum / float64(shortPeriod)

	// Sum of the previous 'shortPeriod' closes
	var prevSum float64
	for i := n - (2 * shortPeriod); i < n-shortPeriod; i++ {
		prevSum += candles[i].Close
	}
	prevAvg := prevSum / float64(shortPeriod)

	// Percentage difference: ((lastAvg - prevAvg) / prevAvg) * 100
	perfPct := 0.0
	if prevAvg != 0 {
		perfPct = (lastAvg - prevAvg) / prevAvg * 100.0
	}

	return perfPct
}
