package bot

import (
	db2 "binance_bot/db"
	"binance_bot/interfaces"
	"binance_bot/logger"
	"binance_bot/models"
	"binance_bot/strategies"
	"fmt"
	"log"
	"math"
	"strconv"
	"sync"
	"time"
)

// MultiPairTradingBot manages multiple trading pairs
type MultiPairTradingBot struct {
	exchange interfaces.ExchangeClient
	strategy interfaces.Strategy
	interval string
	pairs    map[string]*models.TradingPair
	pairsMu  sync.RWMutex
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

// NewMultiPairTradingBot creates a new instance of MultiPairTradingBot
func NewMultiPairTradingBot(exchange interfaces.ExchangeClient, strategy interfaces.Strategy, interval string) *MultiPairTradingBot {
	return &MultiPairTradingBot{
		exchange: exchange,
		strategy: strategy,
		interval: interval,
		pairs:    make(map[string]*models.TradingPair),
		stopCh:   make(chan struct{}),
	}
}

func (bot *MultiPairTradingBot) StartTrading() {
	pairsExchange := bot.exchange.GetTradingPairs()
	bot.pairsMu.RLock()
	pairs := make([]*models.TradingPair, 0, len(pairsExchange))
	for _, pair := range pairsExchange {
		pairs = append(pairs, pair)
	}
	bot.pairsMu.RUnlock()

	if !bot.strategy.GetStrategyType().IsValid() {
		log.Fatalf("Invalid strategy type: %s", bot.strategy.GetStrategyType())
	}

	for _, pair := range pairs {
		bot.wg.Add(1)
		switch bot.strategy.GetStrategyType() {
		case strategies.RSIMACDStrategyType:
			fmt.Println("Starting trading for", pair.Symbol, "using RSI-MACD strategy")
			go bot.tradePair(pair) // Use RSI-MACD strategy
		case strategies.SpikeDetectionStrategyType:
			fmt.Println("Starting trading for", pair.Symbol, "using Spike Detection strategy")
			go bot.monitorCurrentCandle(pair) // Use spike detection strategy
		default:
			log.Printf("Unknown strategy type: %s. Skipping trading for %s", bot.strategy.GetStrategyType(), pair.Symbol)
			bot.wg.Done()
		}
	}
	logger.Debug("Started trading for", len(pairs), "pairs", "using", bot.strategy.GetStrategyType().String())
}

// Stop stops the trading bot
func (bot *MultiPairTradingBot) Stop() {
	close(bot.stopCh)
	bot.wg.Wait()
	fmt.Println("Trading bot stopped.")
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

func (bot *MultiPairTradingBot) tradePair(pair *models.TradingPair) {
	defer bot.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	resetTicker := time.NewTicker(1 * time.Minute) // Check every minute for day change
	defer resetTicker.Stop()

	logger.Infof("Started trading %s", pair.Symbol)

	tradesToday := 0                 // Track number of trades per day
	lastResetDay := time.Now().Day() // Track the day of the last reset

	for {
		select {
		case <-bot.stopCh:
			return
		case <-resetTicker.C:
			// Reset daily trade counter at midnight
			currentDay := time.Now().Day()
			if currentDay != lastResetDay {
				logger.Infof("Resetting daily trade counter for %s. Previous trades: %d", pair.Symbol, tradesToday)
				tradesToday = 0
				lastResetDay = currentDay
			}
		case <-ticker.C:
			// Fetch candles
			candles, err := bot.exchange.FetchCandles(pair.Symbol, bot.interval, 100)
			if err != nil {
				logger.Infof("Error fetching candles for %s: %v", pair.Symbol, err)
				continue
			}

			// Detect trend and calculate signal
			isUptrend := bot.isUptrend(candles)
			sngl, err := bot.strategy.Calculate(candles, pair.Symbol, isUptrend)
			if err != nil {
				logger.Infof("Error calculating strategy for %s: %v", pair.Symbol, err)
				continue
			}

			if sngl == 0 {
				// HOLD signal
				continue
			}

			// Avoid overtrading
			if tradesToday >= 25 {
				logger.Infof("Max trades reached for %s today. Skipping further trades.", pair.Symbol)
				continue
			}

			// Fetch balances
			quoteBalance, err := bot.exchange.GetBalance(pair.QuoteAsset)
			if err != nil {
				logger.Infof("Error fetching %s balance: %v", pair.QuoteAsset, err)
				continue
			}

			baseBalance, err := bot.exchange.GetBalance(pair.BaseAsset)
			if err != nil {
				logger.Infof("Error fetching %s balance: %v", pair.BaseAsset, err)
				continue
			}

			// Current price
			currentPrice := candles[len(candles)-1].Close

			// Determine trade size
			tradeAmount := bot.calculateTradeAmount(sngl, quoteBalance, baseBalance, pair.Symbol)
			if tradeAmount == 0 {
				logger.Infof("Insufficient balance for %s trade. Skipping trade.", pair.Symbol)
				continue
			}

			// Handle BUY or SELL
			if sngl > 0 { // BUY signal
				trAmount := tradeAmount / currentPrice
				fmt.Println("BUY signal", pair.Symbol, "Trade amount", trAmount, "Current price", currentPrice, "Base balance", baseBalance)
				if !bot.handleBuy(pair, trAmount, currentPrice, quoteBalance) {
					logger.Infof("Error handling BUY for %s\n", pair.Symbol)
					continue
				}
			} else if sngl < 0 { // SELL signal
				fmt.Println("SELL signal", pair.Symbol, "Trade amount", tradeAmount, "Current price", currentPrice, "Quote balance", quoteBalance)
				if !bot.handleSell(pair, tradeAmount, currentPrice, baseBalance) {
					logger.Infof("Error handling SELL for %s\n", pair.Symbol)
					continue
				}
			}

			tradesToday++
		}
	}
}

func (bot *MultiPairTradingBot) handleBuy(pair *models.TradingPair, tradeAmount, currentPrice, quoteBalance float64) bool {
	if tradeAmount*currentPrice < pair.MinNotional {
		logger.Infof("BUY amount too small for %s. Adjusting to minimum notional.", pair.Symbol)
		tradeAmount = pair.MinNotional / currentPrice

		if tradeAmount > quoteBalance {
			logger.Infof("Skipping BUY for %s: Insufficient USDT balance. Need %.2f Have %.2f", pair.Symbol, tradeAmount, quoteBalance)
			return false
		}
	}

	// Place Limit BUY Order
	limitPrice := currentPrice * 1.001 // 0.1% higher than current price
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
	logger.Infof("Successfully placed LIMIT BUY order for %s. Order ID: %s", pair.Symbol, orderID)
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

	// Place Limit SELL Order
	limitPrice := currentPrice * 0.999 // 0.1% lower than current price
	limitOrderPrice := strconv.FormatFloat(limitPrice, 'f', pair.PricePrecision, 64)
	executedVolume := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)

	logger.Infof("Placing LIMIT SELL order for %s: Quantity=%.2f, Limit Price=%.2f", pair.Symbol, tradeAmount, limitPrice)
	orderID, err := bot.exchange.CreateLimitOrder(pair.Symbol, "SELL", executedVolume, limitOrderPrice)
	if err != nil {
		logger.Infof("Error placing LIMIT SELL order for %s: %v", pair.Symbol, err)
		return false
	}

	// Fetch active trade and log completed trade
	activeTrades, err := db2.SQLiteDB.GetActiveTrades(pair.Symbol)
	if err != nil {
		logger.Infof("Error fetching active trade for %s: %v", pair.Symbol, err)
		return false
	}
	for _, activeTrade := range activeTrades {
		profitLoss := (limitPrice - activeTrade.BuyPrice) * tradeAmount
		err = db2.SQLiteDB.LogCompletedTrade(pair.Symbol, activeTrade.BuyPrice, limitPrice, tradeAmount, profitLoss)
		if err != nil {
			logger.Infof("Error logging SELL trade for %s: %v", pair.Symbol, err)
			return false
		}

		// Remove the active trade
		err = db2.SQLiteDB.RemoveActiveTrade(activeTrade.ID)
		if err != nil {
			logger.Infof("Error removing active trade for %s: %v", pair.Symbol, err)
			return false
		}
		logger.Infof("Successfully completed SELL order for %s. Order ID: %s", pair.Symbol, orderID)

	}

	return true
}

func (bot *MultiPairTradingBot) monitorCurrentCandle(pair *models.TradingPair) {
	ticker := time.NewTicker(1 * time.Second) // Monitor every second
	defer ticker.Stop()

	lastPrice := 0.0 // Track the last price for detecting spikes

	for {
		select {
		case <-bot.stopCh:
			return
		case <-ticker.C:
			// Fetch current price
			currentPrice, err := bot.exchange.GetCurrentPrice(pair.Symbol)
			if err != nil {
				logger.Infof("Error fetching current price for %s: %v", pair.Symbol, err)
				continue
			}

			// Calculate the price change percentage
			priceChange := 0.0
			if lastPrice > 0 {
				priceChange = (currentPrice - lastPrice) / lastPrice * 100
			}

			lastPrice = currentPrice // Update the last price

			log.Println(pair.Symbol, "Current Price:", currentPrice, "Price Change:", priceChange)

			// Spike detection logic
			if priceChange > 1.0 { // Spike up (e.g., >2%)
				logger.Infof("Detected upward spike for %s: Price Change = %.2f%%", pair.Symbol, priceChange)

				// Fetch available USDT balance
				quoteBalance, err := bot.exchange.GetBalance(pair.QuoteAsset)
				if err != nil {
					logger.Infof("Error fetching USDT balance for %s: %v", pair.Symbol, err)
					continue
				}

				// Calculate the trade amount
				tradeAmount := pair.MinNotional / currentPrice
				if tradeAmount*currentPrice < pair.MinNotional || tradeAmount > quoteBalance {
					logger.Infof("Skipping BUY for %s: Insufficient USDT balance or below minNotional", pair.Symbol)
					continue
				}

				// Place a BUY order
				quantity := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)
				orderID, err := bot.exchange.CreateMarketOrder(pair.Symbol, "BUY", quantity)
				if err != nil {
					logger.Infof("Error executing BUY order for %s: %v", pair.Symbol, err)
					continue
				}

				// Log the trade in the database
				logger.Infof("Executed BUY order for %s. Order ID: %s", pair.Symbol, orderID)
				err = db2.SQLiteDB.LogActiveTrade(pair.Symbol, currentPrice, tradeAmount)
				if err != nil {
					logger.Infof("Error logging BUY trade for %s: %v", pair.Symbol, err)
				}
			}

			activeTrades, err := db2.SQLiteDB.GetActiveTrades(pair.Symbol)
			if err != nil {
				logger.Infof("Error fetching active trades for %s: %v", pair.Symbol, err)
				continue
			}

			dropThreshold := 0.5 // Percentage drop to trigger a sell (e.g., 2%)

			for _, trade := range activeTrades {
				// Initialize maxPrice with the price at which the trade was executed (PriceOnBuy)
				maxPrice := trade.BuyPrice

				// Calculate breakeven price (include fees)
				feeRate := 0.001 // Default fee rate
				requiredPrice := trade.BuyPrice * (1 + feeRate)

				// Update the maximum price if current price exceeds it
				if currentPrice > maxPrice {
					maxPrice = currentPrice
				}

				// Calculate the percentage drop from the maximum price
				priceDrop := (maxPrice - currentPrice) / maxPrice * 100

				// Check if the price is below the breakeven price or if the drop exceeds the threshold
				if currentPrice <= requiredPrice || priceDrop >= dropThreshold {
					logger.Infof("Detected price drop below breakeven or spike reversal for %s. Reversing trade.", pair.Symbol)
					logger.Infof("Current Price: %.8f, Max Price: %.8f, Price Drop: %.2f%%", currentPrice, maxPrice, priceDrop)

					// Sell immediately to avoid losses or secure profit
					quantity := strconv.FormatFloat(trade.Quantity, 'f', pair.QtyPrecision, 64)
					orderID, err := bot.exchange.CreateMarketOrder(pair.Symbol, "SELL", quantity)
					if err != nil {
						logger.Infof("Error executing SELL order for %s: %v", pair.Symbol, err)
						continue
					}

					// Log and remove the trade
					logger.Infof("Executed SELL order for %s. Order ID: %f", pair.Symbol, orderID)
					err = db2.SQLiteDB.RemoveActiveTrade(trade.ID)
					if err != nil {
						logger.Infof("Error removing active trade for %s: %v", pair.Symbol, err)
					}
				}
			}
		}
	}
}
