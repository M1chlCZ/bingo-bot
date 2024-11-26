package bot

import (
	"binance_bot/client"
	db2 "binance_bot/db"
	"binance_bot/interfaces"
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
	exchange *client.BinanceClient
	strategy interfaces.Strategy
	interval string
	wg       sync.WaitGroup
	stopCh   chan struct{}
}

func NewMultiPairTradingBot(exchange *client.BinanceClient, strategy interfaces.Strategy, interval string) *MultiPairTradingBot {
	return &MultiPairTradingBot{
		exchange: exchange,
		strategy: strategy,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (bot *MultiPairTradingBot) StartTrading() {
	bot.exchange.PairsMutex.RLock()
	pairs := make([]*models.TradingPair, 0, len(bot.exchange.Pairs))
	for _, pair := range bot.exchange.Pairs {
		pairs = append(pairs, pair)
	}
	bot.exchange.PairsMutex.RUnlock()

	if !bot.strategy.GetStrategyType().IsValid() {
		log.Fatalf("Invalid strategy type: %s", bot.strategy.GetStrategyType())
	}

	fmt.Println()
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
	fmt.Println("Started trading for", len(pairs), "pairs", "using", bot.strategy.GetStrategyType().String())
}

func (bot *MultiPairTradingBot) Stop() {
	close(bot.stopCh)
	bot.wg.Wait()
}

func (bot *MultiPairTradingBot) isUptrend(candles []models.CandleStick) bool {
	if len(candles) < 50 { // Ensure enough candles for SMA calculation
		fmt.Printf("Insufficient candles for trend detection. Expected 50, got %d\n", len(candles))
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
		fmt.Printf("BUY %.2f %s \n", amount, pair)
		return amount // Use 25% of quote balance
	} else if signal < 0 { // SELL Signal
		fmt.Printf("SELL %s %.2f \n", pair, baseBalance)
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

	log.Printf("Started trading %s", pair.Symbol)

	tradesToday := 0                 // Track number of trades per day
	lastResetDay := time.Now().Day() // Track the day of the last reset

	for {
		select {
		case <-bot.stopCh:
			return
		case <-resetTicker.C:
			// Check if the day has changed
			currentDay := time.Now().Day()
			if currentDay != lastResetDay {
				log.Printf("Resetting daily trade counter for %s. Previous trades: %d", pair.Symbol, tradesToday)
				tradesToday = 0
				lastResetDay = currentDay
			}
		case <-ticker.C:
			// Fetch candles
			candles, err := bot.exchange.FetchCandles(pair.Symbol, bot.interval, 100)
			if err != nil {
				log.Printf("Error fetching candles for %s: %v", pair.Symbol, err)
				continue
			}

			// Trend Detection
			isUptrend := bot.isUptrend(candles)

			// Calculate signal using Compound Strategy (RSI + MACD)
			sngl, err := bot.strategy.Calculate(candles, pair.Symbol, isUptrend)
			if err != nil {
				log.Printf("Error calculating strategy for %s: %v", pair.Symbol, err)
				continue
			}

			// Avoid overtrading
			if tradesToday >= 25 {
				log.Printf("Max trades reached for %s today. Skipping further trades.", pair.Symbol)
				continue
			}

			// Skip trades against the trend
			//if (sngl > 0 && !isUptrend) || (sngl < 0 && isUptrend) {
			//	log.Printf("Skipping trade for %s due to trend mismatch.", pair.Symbol)
			//	continue
			//}

			// Fetch balances
			quoteBalance, err := bot.exchange.GetBalance(pair.QuoteAsset)
			if err != nil {
				log.Printf("Error fetching %s balance: %v", pair.QuoteAsset, err)
				continue
			}

			baseBalance, err := bot.exchange.GetBalance(pair.BaseAsset)
			if err != nil {
				log.Printf("Error fetching %s balance: %v", pair.BaseAsset, err)
				continue
			}

			// Current price
			currentPrice := candles[len(candles)-1].Close

			//log.Printf("Trend for %s: %s Price: %.2f", pair.Symbol, map[bool]string{true: "Uptrend", false: "Downtrend"}[isUptrend], currentPrice)

			// Determine trade size based on signal strength
			tradeAmount := bot.calculateTradeAmount(sngl, quoteBalance, baseBalance, pair.Symbol)
			if tradeAmount == 0 {
				if sngl != 0 {
					log.Printf("Insufficient balance for %s trade. Skipping trade.", pair.Symbol)
				}
				continue
			}

			if sngl > 0 { // BUY signal
				// Check if trade amount is less than the minimum tradeable amount or minNotional
				if tradeAmount*currentPrice < pair.MinNotional {
					// Calculate the minimum trade amount required to meet minNotional
					if quoteBalance >= pair.MinNotional {
						tradeAmount = pair.MinNotional / currentPrice // Adjust to meet minNotional
						log.Printf("Adjusted BUY trade amount for %s to meet minNotional: %.2f (Min Notional: %.2f)", pair.Symbol, tradeAmount, pair.MinNotional)
					} else {
						log.Printf("Insufficient USDT balance for minimum BUY trade for %s. Required: %.2f, Available: %.2f", pair.Symbol, pair.MinNotional, quoteBalance)
						continue
					}
				}
			} else if sngl < 0 { // SELL signal
				// Check if trade amount is less than the minimum tradeable amount or minNotional
				if tradeAmount*currentPrice < pair.MinNotional {
					// Calculate the minimum trade amount required to meet minNotional
					requiredTradeAmount := pair.MinNotional * currentPrice
					if baseBalance >= requiredTradeAmount {
						tradeAmount = requiredTradeAmount // Adjust to meet minNotional
						log.Printf("Adjusted SELL trade amount for %s to meet minNotional: %.2f (Min Notional: %.2f)", pair.Symbol, tradeAmount, pair.MinNotional)
					} else {
						log.Printf("Insufficient %s balance for minimum SELL trade for %s. Required: %.2f, Available: %.2f", pair.BaseAsset, pair.Symbol, requiredTradeAmount, baseBalance)
						continue
					}
				}
			}

			// Set stop-loss and take-profit levels
			var limitPrice, stopLossPrice float64
			if sngl > 0 {
				limitPrice = currentPrice * 1.02    // 2% above for take-profit
				stopLossPrice = currentPrice * 0.98 // 2% below for stop-loss
			} else if sngl < 0 {
				limitPrice = currentPrice * 0.98    // 2% below for take-profit
				stopLossPrice = currentPrice * 1.02 // 2% above for stop-loss
			}

			// Execute the trade as a limit order with a stop-loss
			side := map[int]string{1: "BUY", -1: "SELL"}[sngl]
			executedVolume := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)

			// Limit Order
			limitOrderPrice := strconv.FormatFloat(limitPrice, 'f', pair.PricePrecision, 64)
			log.Printf("Attempting to %s %.2f %s at limit price %.2f with stop-loss %.2f",
				side, tradeAmount, pair.BaseAsset, limitPrice, stopLossPrice)

			orderID, err := bot.exchange.CreateLimitOrder(pair.Symbol, side, executedVolume, limitOrderPrice)
			if err != nil {
				log.Printf("Error executing LIMIT %s trade for %s: %v", side, pair.Symbol, err)
				continue
			}

			// Place Stop-Limit Order for Stop-Loss
			stopPrice := strconv.FormatFloat(stopLossPrice, 'f', pair.PricePrecision, 64)
			stopLimitPrice := strconv.FormatFloat(stopLossPrice*0.99, 'f', pair.PricePrecision, 64) // Slightly below the stop price

			stopOrderID, err := bot.exchange.CreateStopLossLimitOrder(pair.Symbol, side, executedVolume, stopPrice, stopLimitPrice)
			if err != nil {
				log.Printf("Error placing STOP-LOSS order for %s: %v", pair.Symbol, err)
				// Optionally cancel the limit order if stop-loss fails
				cancelErr := bot.exchange.CancelOrder(pair.Symbol, orderID)
				if cancelErr != nil {
					log.Printf("Error canceling limit order for %s: %v", pair.Symbol, cancelErr)
				}
				continue
			}

			log.Printf("Stop-Loss Order placed: %d", stopOrderID)

			// Monitor the orders
			filled, err := bot.exchange.MonitorOrder(pair.Symbol, orderID)
			if err != nil {
				log.Printf("Error monitoring LIMIT order for %s: %v", pair.Symbol, err)
				// Cancel stop-loss order if limit order fails
				cancelErr := bot.exchange.CancelOrder(pair.Symbol, stopOrderID)
				if cancelErr != nil {
					log.Printf("Error canceling stop-loss order for %s: %v", pair.Symbol, cancelErr)
				}
				continue
			}

			if filled {
				log.Printf("Successfully filled LIMIT %s order for %s.", side, pair.Symbol)

				if side == "BUY" {
					// Log the BUY trade as active
					err = db2.SQLiteDB.LogActiveTrade(pair.Symbol, limitPrice, tradeAmount)
					if err != nil {
						log.Printf("Error logging BUY trade for %s: %v", pair.Symbol, err)
					}
				} else {
					// Calculate profit/loss for SELL
					activeTrade, err := db2.SQLiteDB.GetActiveTrade(pair.Symbol)
					if err != nil {
						log.Printf("Error fetching active trade for %s: %v", pair.Symbol, err)
					} else {
						profitLoss := (limitPrice - activeTrade.BuyPrice) * tradeAmount
						err = db2.SQLiteDB.LogCompletedTrade(pair.Symbol, activeTrade.BuyPrice, limitPrice, tradeAmount, profitLoss)
						if err != nil {
							log.Printf("Error logging SELL trade for %s: %v", pair.Symbol, err)
						}

						// Remove the trade from active trades
						err = db2.SQLiteDB.RemoveActiveTrade(activeTrade.ID)
						if err != nil {
							log.Printf("Error removing active trade for %s: %v", pair.Symbol, err)
						}
					}
				}

				tradesToday++
			} else {
				log.Printf("LIMIT %s order for %s was not filled. Canceling stop-loss order.", side, pair.Symbol)
				cancelErr := bot.exchange.CancelOrder(pair.Symbol, stopOrderID)
				if cancelErr != nil {
					log.Printf("Error canceling stop-loss order for %s: %v", pair.Symbol, cancelErr)
				}
			}
		}
	}
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
				log.Printf("Error fetching current price for %s: %v", pair.Symbol, err)
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
				log.Printf("Detected upward spike for %s: Price Change = %.2f%%", pair.Symbol, priceChange)

				// Fetch available USDT balance
				quoteBalance, err := bot.exchange.GetBalance(pair.QuoteAsset)
				if err != nil {
					log.Printf("Error fetching USDT balance for %s: %v", pair.Symbol, err)
					continue
				}

				// Calculate the trade amount
				tradeAmount := pair.MinNotional / currentPrice
				if tradeAmount*currentPrice < pair.MinNotional || tradeAmount > quoteBalance {
					log.Printf("Skipping BUY for %s: Insufficient USDT balance or below minNotional", pair.Symbol)
					continue
				}

				// Place a BUY order
				quantity := strconv.FormatFloat(tradeAmount, 'f', pair.QtyPrecision, 64)
				orderID, err := bot.exchange.CreateMarketOrder(pair.Symbol, "BUY", quantity)
				if err != nil {
					log.Printf("Error executing BUY order for %s: %v", pair.Symbol, err)
					continue
				}

				// Log the trade in the database
				log.Printf("Executed BUY order for %s. Order ID: %s", pair.Symbol, orderID)
				err = db2.SQLiteDB.LogActiveTrade(pair.Symbol, currentPrice, tradeAmount)
				if err != nil {
					log.Printf("Error logging BUY trade for %s: %v", pair.Symbol, err)
				}
			}

			activeTrades, err := db2.SQLiteDB.GetActiveTrades(pair.Symbol)
			if err != nil {
				log.Printf("Error fetching active trades for %s: %v", pair.Symbol, err)
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
					log.Printf("Detected price drop below breakeven or spike reversal for %s. Reversing trade.", pair.Symbol)
					log.Printf("Current Price: %.8f, Max Price: %.8f, Price Drop: %.2f%%", currentPrice, maxPrice, priceDrop)

					// Sell immediately to avoid losses or secure profit
					quantity := strconv.FormatFloat(trade.Quantity, 'f', pair.QtyPrecision, 64)
					orderID, err := bot.exchange.CreateMarketOrder(pair.Symbol, "SELL", quantity)
					if err != nil {
						log.Printf("Error executing SELL order for %s: %v", pair.Symbol, err)
						continue
					}

					// Log and remove the trade
					log.Printf("Executed SELL order for %s. Order ID: %f", pair.Symbol, orderID)
					err = db2.SQLiteDB.RemoveActiveTrade(trade.ID)
					if err != nil {
						log.Printf("Error removing active trade for %s: %v", pair.Symbol, err)
					}
				}
			}
		}
	}
}
