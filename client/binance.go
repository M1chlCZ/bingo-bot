package client

import (
	"binance_bot/interfaces"
	"binance_bot/logger"
	"binance_bot/models"
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BinanceClient implements the Exchange interface
type BinanceClient struct {
	client      *binance.Client
	pairs       map[string]*models.TradingPair
	pairsMutex  sync.RWMutex
	candleCache map[string][]models.CandleStick
	cacheMutex  sync.RWMutex
}

// NewBinanceClient creates a new Binance client instance
func NewBinanceClient(apiKey, apiSecret string) (interfaces.ExchangeClient, error) {
	client := binance.NewClient(apiKey, apiSecret)
	logger.Info("Started trading using Binance")
	return &BinanceClient{
		client:      client,
		pairs:       make(map[string]*models.TradingPair),
		candleCache: make(map[string][]models.CandleStick),
	}, nil
}

func (b *BinanceClient) GetTradingPairs() map[string]*models.TradingPair {
	return b.pairs
}

// AddTradingPair adds a new trading pair to monitor
func (b *BinanceClient) AddTradingPair(pair models.TradingPair) error {
	// Fetch exchange info for the trading pair
	info, err := b.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get exchange info for %s: %v", pair.Symbol, err)
	}

	// Loop through symbols to find the matching one
	var symbolFound bool
	for _, symbol := range info.Symbols {
		if symbol.Symbol == pair.Symbol {
			symbolFound = true
			pair.BaseAsset = symbol.BaseAsset
			pair.QuoteAsset = symbol.QuoteAsset
			pair.PricePrecision = symbol.QuotePrecision
			pair.QtyPrecision = symbol.BaseAssetPrecision

			// Parse filters to extract trading rules
			for _, filter := range symbol.Filters {
				switch filter["filterType"] {
				case "NOTIONAL":
					// Extract `minNotional`
					minNotionalStr, ok := filter["minNotional"].(string)
					if !ok {
						return fmt.Errorf("invalid minNotional format for %s", pair.Symbol)
					}
					pair.MinNotional, err = strconv.ParseFloat(minNotionalStr, 64)
					if err != nil {
						return fmt.Errorf("failed to parse minNotional for %s: %v", pair.Symbol, err)
					}
				}
			}

			// Safely add the pair to the map
			b.pairsMutex.Lock()
			b.pairs[pair.Symbol] = &pair
			b.pairsMutex.Unlock()

			logger.Infof("Successfully added trading pair: %s", pair.Symbol)
			break
		}
	}

	if !symbolFound {
		return fmt.Errorf("symbol %s not found in exchange info", pair.Symbol)
	}

	return nil
}

// GetCurrentPrice fetches the current price for a given symbol
func (b *BinanceClient) GetCurrentPrice(symbol string) (float64, error) {
	// Fetch the price from the Binance API
	prices, err := b.client.NewListPricesService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to fetch current price for %s: %v", symbol, err)
	}

	if len(prices) == 0 {
		return 0, fmt.Errorf("no price data returned for symbol %s", symbol)
	}

	// Parse the price as a float
	price, err := strconv.ParseFloat(prices[0].Price, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse price for %s: %v", symbol, err)
	}

	logger.Infof("Current price for %s: %.8f", symbol, price)
	return price, nil
}

// FetchCandles implements the Exchange interface
func (b *BinanceClient) FetchCandles(symbol, interval string, limit int) ([]models.CandleStick, error) {
	var klines []*binance.Kline
	err := retry(func() error {
		var err error
		klines, err = b.client.NewKlinesService().
			Symbol(symbol).
			Interval(interval).
			Limit(limit).
			Do(context.Background())
		return err
	}, 3, time.Second)

	if err != nil {
		return nil, fmt.Errorf("failed to fetch candles: %v", err)
	}

	candles := make([]models.CandleStick, len(klines))
	for i, k := range klines {
		open, _ := strconv.ParseFloat(k.Open, 64)
		high, _ := strconv.ParseFloat(k.High, 64)
		low, _ := strconv.ParseFloat(k.Low, 64)
		cls, _ := strconv.ParseFloat(k.Close, 64)
		volume, _ := strconv.ParseFloat(k.Volume, 64)
		candles[i] = models.CandleStick{
			Timestamp: time.Unix(k.OpenTime/1000, 0),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     cls,
			Volume:    volume,
		}
	}

	b.cacheMutex.Lock()
	b.candleCache[symbol] = candles
	b.cacheMutex.Unlock()

	return candles, nil
}

// GetBalance implements the Exchange interface
func (b *BinanceClient) GetBalance(asset string) (float64, error) {
	account, err := b.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to get account info: %v", err)
	}

	for _, balance := range account.Balances {
		if balance.Asset == asset {
			free, _ := strconv.ParseFloat(balance.Free, 64)
			return free, nil
		}
	}

	return 0, fmt.Errorf("asset %s not found", asset)
}

// CreateOrder implements the Exchange interface
func (b *BinanceClient) CreateOrder(symbol, orderType, side string, amount string) (float64, error) {
	b.pairsMutex.RLock()
	_, exists := b.pairs[symbol]
	b.pairsMutex.RUnlock()

	if !exists {
		return 0, fmt.Errorf("trading pair %s not configured", symbol)
	}

	// Place market order
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderType(orderType)).
		QuoteOrderQty(amount). // Specify the amount in quote asset
		Do(context.Background())

	if err != nil {
		return 0, fmt.Errorf("failed to place %s order for %s: %v", side, symbol, err)
	}

	// Calculate the executed price (for market orders, it's filled)
	var executedPrice float64
	for _, fill := range order.Fills {
		price, err := strconv.ParseFloat(fill.Price, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse fill price: %v", err)
		}
		quantity, err := strconv.ParseFloat(fill.Quantity, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse fill quantity: %v", err)
		}
		executedPrice += price * quantity
	}

	// Average executed price
	if executedPrice > 0 {
		cumQuoteQty, _ := strconv.ParseFloat(order.CummulativeQuoteQuantity, 64)
		executedPrice /= cumQuoteQty
	}

	return executedPrice, nil
}

func (b *BinanceClient) CreateMarketOrder(symbol, side, quantity string) (float64, error) {
	// Check if the trading pair is configured
	b.pairsMutex.RLock()
	_, exists := b.pairs[symbol]
	b.pairsMutex.RUnlock()

	if !exists {
		return 0, fmt.Errorf("trading pair %s not configured", symbol)
	}

	// Place the market order
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderTypeMarket). // Market order
		Quantity(quantity).            // Base asset quantity
		Do(context.Background())

	if err != nil {
		return 0, fmt.Errorf("failed to place MARKET %s order for %s: %v", side, symbol, err)
	}

	// Calculate the executed price based on fills
	var totalQuoteQty float64
	var totalBaseQty float64

	for _, fill := range order.Fills {
		price, err := strconv.ParseFloat(fill.Price, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse fill price: %v", err)
		}
		quantity, err := strconv.ParseFloat(fill.Quantity, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse fill quantity: %v", err)
		}
		totalQuoteQty += price * quantity
		totalBaseQty += quantity
	}

	// Calculate the average executed price
	if totalBaseQty == 0 {
		return 0, fmt.Errorf("no fills returned for the market order")
	}
	averagePrice := totalQuoteQty / totalBaseQty

	return averagePrice, nil
}

func (b *BinanceClient) CreateLimitOrder(symbol, side, quantity, price string) (int64, error) {
	// Fetch symbol filters to comply with LOT_SIZE and PRICE_FILTER
	info, err := b.client.NewExchangeInfoService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to fetch exchange info for %s: %v", symbol, err)
	}

	var minQty, maxQty, stepSize, tickSize float64
	for _, filter := range info.Symbols[0].Filters {
		if filter["filterType"] == "LOT_SIZE" {
			minQty, err = strconv.ParseFloat(filter["minQty"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse minQty for %s: %v", symbol, err)
			}
			maxQty, err = strconv.ParseFloat(filter["maxQty"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse maxQty for %s: %v", symbol, err)
			}
			stepSize, err = strconv.ParseFloat(filter["stepSize"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse stepSize for %s: %v", symbol, err)
			}
			fmt.Println(minQty, maxQty, stepSize)
		}
		if filter["filterType"] == "PRICE_FILTER" {
			tickSize, err = strconv.ParseFloat(filter["tickSize"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse tickSize for %s: %v", symbol, err)
			}
		}
	}

	// Adjust quantity to comply with LOT_SIZE
	quantityFloat, err := strconv.ParseFloat(quantity, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity format: %v", err)
	}

	if quantityFloat < minQty {
		return 0, fmt.Errorf("quantity %.8f is below the minimum allowed %.8f for %s", quantityFloat, minQty, symbol)
	}

	if quantityFloat > maxQty {
		quantityFloat = maxQty // Cap at maxQty
	}

	// Ensure stepSize is valid
	if stepSize <= 0 {
		logger.Infof("Invalid stepSize for %s: %.8f", symbol, stepSize)
		return 0, fmt.Errorf("invalid stepSize for %s: %.8f", symbol, stepSize)
	}

	// Align with StepSize
	adjustedQty := math.Floor(quantityFloat/stepSize) * stepSize
	if math.IsNaN(adjustedQty) || adjustedQty <= 0 {
		logger.Infof("Adjusted Quantity for %s is invalid: Original=%s, Adjusted=NaN or <= 0", symbol, quantity)
		return 0, fmt.Errorf("adjusted quantity is invalid for %s: Original=%s", symbol, quantity)
	}

	formattedQty := strconv.FormatFloat(adjustedQty, 'f', -1, 64)
	logger.Infof("Adjusted Quantity for %s: Original=%s, Adjusted=%s", symbol, quantity, formattedQty)

	// Adjust price using PRICE_FILTER (already implemented in previous steps)
	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}
	pricePrecision := 0
	quantityPrecision := 0

	if side == "BUY" {
		pricePrecision = info.Symbols[0].QuoteAssetPrecision
		quantityPrecision = info.Symbols[0].BaseAssetPrecision
	} else {
		pricePrecision = info.Symbols[0].QuotePrecision
		quantityPrecision = info.Symbols[0].BaseAssetPrecision
	}

	adjustedPrice := math.Floor(priceFloat/tickSize) * tickSize
	formattedPrice := fmt.Sprintf("%.*f", pricePrecision, adjustedPrice)
	formattedQty = strconv.FormatFloat(adjustedQty, 'f', quantityPrecision, 64)

	logger.Debug("Price Precision: ", pricePrecision)
	logger.Debug("Adjusted Price: ", adjustedPrice)
	logger.Debug("Formatted Price: ", formattedPrice)
	logger.Debug("Formatted Qty: ", formattedQty)
	logger.Debug("Amount in USDT: ", adjustedPrice*adjustedQty)

	// Place the limit order
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(formattedQty).
		Price(formattedPrice).
		Do(context.Background())

	if err != nil {
		return 0, fmt.Errorf("failed to place LIMIT %s order for %s: %v", side, symbol, err)
	}

	logger.Infof("Successfully placed LIMIT %s order for %s: OrderID=%d", side, symbol, order.OrderID)
	return order.OrderID, nil
}

func (b *BinanceClient) CreateStopLossLimitOrder(symbol, side, quantity, price, stopLoss string) (int64, error) {
	// Fetch symbol filters to comply with LOT_SIZE and PRICE_FILTER
	info, err := b.client.NewExchangeInfoService().Symbol(symbol).Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to fetch exchange info for %s: %v", symbol, err)
	}

	var minQty, maxQty, stepSize, tickSize float64
	for _, filter := range info.Symbols[0].Filters {
		if filter["filterType"] == "LOT_SIZE" {
			minQty, err = strconv.ParseFloat(filter["minQty"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse minQty for %s: %v", symbol, err)
			}
			maxQty, err = strconv.ParseFloat(filter["maxQty"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse maxQty for %s: %v", symbol, err)
			}
			stepSize, err = strconv.ParseFloat(filter["stepSize"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse stepSize for %s: %v", symbol, err)
			}
			fmt.Println(minQty, maxQty, stepSize)
		}
		if filter["filterType"] == "PRICE_FILTER" {
			tickSize, err = strconv.ParseFloat(filter["tickSize"].(string), 64)
			if err != nil {
				return 0, fmt.Errorf("failed to parse tickSize for %s: %v", symbol, err)
			}
		}
	}

	// Adjust quantity to comply with LOT_SIZE
	quantityFloat, err := strconv.ParseFloat(quantity, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid quantity format: %v", err)
	}

	if quantityFloat < minQty {
		return 0, fmt.Errorf("quantity %.8f is below the minimum allowed %.8f for %s", quantityFloat, minQty, symbol)
	}
	if quantityFloat > maxQty {
		quantityFloat = maxQty // Cap at maxQty
	}

	// Ensure stepSize is valid
	if stepSize <= 0 {
		logger.Infof("Invalid stepSize for %s: %.8f", symbol, stepSize)
		return 0, fmt.Errorf("invalid stepSize for %s: %.8f", symbol, stepSize)
	}

	// Align with StepSize
	adjustedQty := math.Floor(quantityFloat/stepSize) * stepSize
	if math.IsNaN(adjustedQty) || adjustedQty <= 0 {
		logger.Infof("Adjusted Quantity for %s is invalid: Original=%s, Adjusted=NaN or <= 0", symbol, quantity)
		return 0, fmt.Errorf("adjusted quantity is invalid for %s: Original=%s", symbol, quantity)
	}

	formattedQty := strconv.FormatFloat(adjustedQty, 'f', -1, 64)
	logger.Infof("Adjusted Quantity for %s: Original=%s, Adjusted=%s", symbol, quantity, formattedQty)

	// Adjust price using PRICE_FILTER (already implemented in previous steps)
	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}
	pricePrecision := info.Symbols[0].QuotePrecision
	adjustedPrice := math.Floor(priceFloat/tickSize) * tickSize
	formattedPrice := strconv.FormatFloat(adjustedPrice, 'f', pricePrecision, 64)

	logger.Debug("Price Precision: ", pricePrecision)
	logger.Debug("Adjusted Price: ", adjustedPrice)
	logger.Debug("Formatted Price: ", formattedPrice)

	// Adjust price using PRICE_FILTER (already implemented in previous steps)
	priceStopLossFloat, err := strconv.ParseFloat(stopLoss, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}
	adjustedStopLossPrice := math.Floor(priceStopLossFloat/tickSize) * tickSize
	formattedStopLossPrice := strconv.FormatFloat(adjustedStopLossPrice, 'f', pricePrecision, 64)

	logger.Debug("Stop Loss Price Precision: ", pricePrecision)
	logger.Debug("Stop Loss Adjusted Price: ", adjustedStopLossPrice)
	logger.Debug("Stop Loss Formatted Price: ", formattedStopLossPrice)

	// Place the limit order
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderTypeStopLossLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(formattedQty).
		Price(formattedPrice).
		StopPrice(formattedStopLossPrice).
		Do(context.Background())

	if err != nil {
		return 0, fmt.Errorf("failed to place LIMIT %s order for %s: %v", side, symbol, err)
	}

	logger.Infof("Successfully placed LIMIT %s order for %s: OrderID=%d", side, symbol, order.OrderID)
	return order.OrderID, nil
}

// MonitorOrder TODO: USE THIS FUNCTION
func (b *BinanceClient) MonitorOrder(symbol string, orderID int64) (bool, error) {
	logger.Infof("Monitoring order %d for %s", orderID, symbol)

	for {
		// Fetch order status
		order, err := b.client.NewGetOrderService().
			Symbol(symbol).
			OrderID(orderID).
			Do(context.Background())
		if err != nil {
			return false, fmt.Errorf("failed to fetch order status for %s: %v", symbol, err)
		}

		logger.Infof("Order %d status: %s (Filled Quantity: %s)", orderID, order.Status, order.ExecutedQuantity)

		// Check if the order is fully filled
		if order.Status == binance.OrderStatusTypeFilled {
			return true, nil
		}

		// Break the loop if the order is canceled or rejected
		if order.Status == binance.OrderStatusTypeCanceled || order.Status == binance.OrderStatusTypeRejected {
			return false, nil
		}

		// Wait before the next status check
		time.Sleep(5 * time.Second)
	}
}

func (b *BinanceClient) CancelOrder(symbol string, orderID int64) error {
	logger.Infof("Canceling order %d for %s", orderID, symbol)

	_, err := b.client.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel order %d for %s: %v", orderID, symbol, err)
	}

	logger.Infof("Successfully canceled order %d for %s", orderID, symbol)
	return nil
}

func (b *BinanceClient) GetFeeRate() (float64, error) {
	accountInfo, err := b.client.NewGetAccountService().Do(context.Background())
	if err != nil {
		return 0, fmt.Errorf("failed to fetch account info: %v", err)
	}

	// Default spot trading fee (if using BNB discounts, adjust this value)
	feeRate := 0.001 // Default 0.1%
	if accountInfo.MakerCommission > 0 {
		feeRate = float64(accountInfo.MakerCommission) / 10000 // Convert commission to decimal
	}
	return feeRate, nil
}

// FetchMarkets retrieves all trading pairs matching the provided ticker (e.g., "USDT").
func (b *BinanceClient) FetchMarkets(tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) ([]models.TradingPair, error) {
	log.Printf("[INFO] Fetching all markets matching ticker: %v", tickers)

	// Fetch exchange information
	exchangeInfo, err := b.client.NewExchangeInfoService().Do(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch exchange info: %v", err)
	}

	// Filter symbols that end with the specified ticker
	tradingPairs := make([]models.TradingPair, 0)

	for _, symbol := range exchangeInfo.Symbols {
		// Check if the symbol is excluded
		if isExcludedMarket(symbol.Symbol, excludedMarkets) {
			logger.Infof("Excluded quote market: %s", symbol.Symbol)
			continue
		}

		if isExcludedTradingPair(models.TradingPair{Symbol: symbol.Symbol}, excludedTradingPairs) {
			logger.Infof("Excluded trading pair: %s", symbol.Symbol)
			continue
		}

		if symbol.Status != "TRADING" {
			logger.Debugf("Trading is not allowed for %s", symbol.Symbol)
			continue
		}
		if isIncludedMarket(symbol.Symbol, tickers) {
			if !symbol.IsSpotTradingAllowed {
				logger.Warnf("Spot trading is not allowed for %s", symbol.Symbol)
				continue
			}
			var minNotional float64
			for _, filter := range symbol.Filters {
				switch filter["filterType"] {
				case "NOTIONAL":
					// Extract `minNotional`
					minNotionalStr, ok := filter["minNotional"].(string)
					if !ok {
						logger.Errorf("Invalid minNotional format for %s", symbol.Symbol)
						continue
					}
					minNotional, err = strconv.ParseFloat(minNotionalStr, 64)
					if err != nil {
						logger.Errorf("Failed to parse minNotional for %s: %v", symbol.Symbol, err)
						continue
					}
				}
			}
			tp := models.TradingPair{
				Symbol:         symbol.Symbol,
				BaseAsset:      symbol.BaseAsset,
				QuoteAsset:     symbol.QuoteAsset,
				PricePrecision: symbol.QuotePrecision,
				QtyPrecision:   symbol.BaseAssetPrecision,
				MinNotional:    minNotional,
			}
			logger.Infof("Found trading pair: %s", tp.Symbol)
			logger.Debugf("Found trading pair: %s Base Assets %s Quote Asset %s MinNotional %.2f", tp.Symbol, tp.BaseAsset, tp.QuoteAsset, tp.MinNotional)
			tradingPairs = append(tradingPairs, tp)
		}
	}
	log.Printf("[INFO] Found %d markets matching tickers: %s", len(tradingPairs), tickers)

	return tradingPairs, nil
}

func isIncludedMarket(symbol string, includedMarkets []string) bool {
	for _, market := range includedMarkets {
		if strings.HasSuffix(symbol, market) {
			return true
		}
	}
	return false
}

func isExcludedMarket(symbol string, excludedMarkets []string) bool {
	for _, market := range excludedMarkets {
		if strings.HasPrefix(symbol, market) {
			return true
		}
	}
	return false
}

func isExcludedTradingPair(tradingPair models.TradingPair, excludedTradingPairs []models.TradingPair) bool {
	for _, tp := range excludedTradingPairs {
		if tp.Symbol == tradingPair.Symbol {
			return true
		}
	}
	return false
}

// toFloat safely converts string to float64
func toFloat(value string) float64 {
	if result, err := strconv.ParseFloat(value, 64); err == nil {
		return result
	}
	return 0
}

// Retry helper for API calls
func retry(fn func() error, retries int, delay time.Duration) error {
	for i := 0; i < retries; i++ {
		if err := fn(); err == nil {
			return nil
		}
		time.Sleep(delay)
	}
	return fmt.Errorf("operation failed after %d retries", retries)
}
