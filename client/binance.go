package client

import (
	"context"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
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

	// Fields for rate limiting
	requestsMu   sync.Mutex
	requestTimes []time.Time
	maxRequests  int
	interval     time.Duration

	// Fields for priority queue approach
	highPriorityQueue chan BinanceRequest
	normalQueue       chan BinanceRequest

	// Worker fields
	workerStarted bool
	workerDone    chan struct{}
	workerOnce    sync.Once
}

func (b *BinanceClient) CreateOrder(symbol, orderType, side string, amount string) (float64, error) {
	//TODO implement me
	panic("implement me")
}

// BinanceRequest is our encapsulation of a single Binance operation
type BinanceRequest struct {
	// The function that actually does the work (place order, fetch data, etc.)
	Do func() (any, error)
	// Priority = true => goes to highPriorityQueue
	Priority bool
	// The channel to send back the result
	resultChan chan resultWrapper
}

// resultWrapper carries the returned any plus error
type resultWrapper struct {
	data any
	err  error
}

// NewBinanceClient creates a new Binance client instance
func NewBinanceClient(apiKey, apiSecret string) (interfaces.ExchangeClient, error) {
	client := binance.NewClient(apiKey, apiSecret)
	logger.Info("Started trading using Binance")
	b := &BinanceClient{
		client:            client,
		pairs:             make(map[string]*models.TradingPair),
		candleCache:       make(map[string][]models.CandleStick),
		maxRequests:       10,              // max requests
		interval:          1 * time.Second, // time window
		highPriorityQueue: make(chan BinanceRequest, 100),
		normalQueue:       make(chan BinanceRequest, 500),
		workerDone:        make(chan struct{}),
	}
	// Start the worker goroutine
	b.startWorker()

	return b, nil
}

func (b *BinanceClient) startWorker() {
	b.workerOnce.Do(func() {
		b.workerStarted = true
		go b.requestWorker()
	})
}

func (b *BinanceClient) requestWorker() {
	defer close(b.workerDone)
	for {
		var req BinanceRequest
		select {
		case req = <-b.highPriorityQueue:
		default:
			req = <-b.normalQueue
		}

		// If the request is nil, channels might be closed -> exit worker
		if req.Do == nil && req.resultChan == nil {
			logger.Infof("[Worker] Received nil request; shutting down?")
			return
		}
		b.rateLimitWait()
		data, err := req.Do()
		req.resultChan <- resultWrapper{data: data, err: err}
		close(req.resultChan)
	}
}

// enqueueRequest is used internally to place a request into one of the queues
func (b *BinanceClient) enqueueRequest(doFn func() (interface{}, error), priority bool) (interface{}, error) {
	// Prepare the request object
	req := BinanceRequest{
		Do:         doFn,
		Priority:   priority,
		resultChan: make(chan resultWrapper, 1),
	}

	// push into queue
	if priority {
		b.highPriorityQueue <- req
	} else {
		b.normalQueue <- req
	}

	// Wait for result
	r := <-req.resultChan
	return r.data, r.err
}

// ==================== Public Methods Implementation ==================== //

func (b *BinanceClient) GetTradingPairs() map[string]*models.TradingPair {
	return b.pairs
}

// AddTradingPair adds a new trading pair to monitor
func (b *BinanceClient) AddTradingPair(pair models.TradingPair) error {
	doFunc := func() (any, error) {
		// Fetch exchange info for the trading pair
		info, err := b.client.NewExchangeInfoService().Do(context.Background())
		if err != nil {
			return nil, fmt.Errorf("failed to get exchange info for %s: %w", pair.Symbol, err)
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
							return nil, fmt.Errorf("invalid minNotional format for %s", pair.Symbol)
						}
						pair.MinNotional, err = strconv.ParseFloat(minNotionalStr, 64)
						if err != nil {
							return nil, fmt.Errorf("failed to parse minNotional for %s: %w", pair.Symbol, err)
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
			return nil, fmt.Errorf("symbol %s not found in exchange info", pair.Symbol)
		}

		return nil, nil
	}

	data, err := b.enqueueRequest(doFunc, false)
	if err != nil {
		return err
	}

	// Not used
	_ = data
	return nil
}

// GetCurrentPrice fetches the current price for a given symbol
func (b *BinanceClient) GetCurrentPrice(symbol string) (float64, error) {
	// Fetch the price from the Binance API
	type Price struct {
		Value float64 `json:"price"`
	}
	doFunc := func() (any, error) {
		prices, err := b.client.NewListPricesService().Symbol(symbol).Do(context.Background())
		if err != nil {
			return Price{Value: 0}, fmt.Errorf("failed to fetch current price for %s: %w", symbol, err)
		}

		if len(prices) == 0 {
			return Price{Value: 0}, fmt.Errorf("no price data returned for symbol %s", symbol)
		}

		// Parse the price as a float
		price, err := strconv.ParseFloat(prices[0].Price, 64)
		if err != nil {
			return Price{Value: 0}, fmt.Errorf("failed to parse price for %s: %w", symbol, err)
		}

		logger.Infof("Current price for %s: %.8f", symbol, price)
		return Price{Value: price}, nil
	}

	data, err := b.enqueueRequest(doFunc, false)
	if err != nil {
		return 0, err
	}
	pv, ok := data.(Price)
	if !ok {
		return 0, fmt.Errorf("unexpected data type returned from queue")
	}
	return pv.Value, nil
}

// FetchCandles implements the Exchange interface
func (b *BinanceClient) FetchCandles(symbol, interval string, limit int) ([]models.CandleStick, error) {
	doFunc := func() (any, error) {
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
			return nil, fmt.Errorf("failed to fetch candles: %w", err)
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

	data, err := b.enqueueRequest(doFunc, false)
	if err != nil {
		return nil, err
	}
	candles, ok := data.([]models.CandleStick)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from queue")
	}
	return candles, nil
}

// GetBalance implements the Exchange interface
func (b *BinanceClient) GetBalance(asset string) (float64, error) {
	type Price struct {
		Value float64 `json:"price"`
	}
	doFunc := func() (any, error) {
		account, err := b.client.NewGetAccountService().Do(context.Background())
		if err != nil {
			return Price{Value: 0}, fmt.Errorf("failed to get account info: %w", err)
		}

		for _, balance := range account.Balances {
			if balance.Asset == asset {
				free, _ := strconv.ParseFloat(balance.Free, 64)
				return Price{Value: free}, nil
			}
		}

		return Price{Value: 0}, fmt.Errorf("asset %s not found", asset)
	}

	data, err := b.enqueueRequest(doFunc, true)
	if err != nil {
		return 0, err
	}
	pv, ok := data.(Price)
	if !ok {
		return 0, fmt.Errorf("unexpected data type returned from queue")
	}
	return pv.Value, nil
}

func (b *BinanceClient) CreateMarketOrder(symbol, side, quantity string) (int64, float64, error) {
	// Check if the trading pair is configured
	type Result struct {
		OrderID      int64
		AveragePrice float64
	}
	do := func() (any, error) {
		info, err := b.client.NewExchangeInfoService().Symbol(symbol).Do(context.Background())
		if err != nil {
			return Result{}, fmt.Errorf("failed to fetch exchange info for %s: %w", symbol, err)
		}

		var minQty, maxQty, stepSize float64
		for _, filter := range info.Symbols[0].Filters {
			if filter["filterType"] == "LOT_SIZE" {
				minQty, err = strconv.ParseFloat(filter["minQty"].(string), 64)
				if err != nil {
					return Result{}, fmt.Errorf("failed to parse minQty for %s: %w", symbol, err)
				}
				maxQty, err = strconv.ParseFloat(filter["maxQty"].(string), 64)
				if err != nil {
					return Result{}, fmt.Errorf("failed to parse maxQty for %s: %w", symbol, err)
				}
				stepSize, err = strconv.ParseFloat(filter["stepSize"].(string), 64)
				if err != nil {
					return Result{}, fmt.Errorf("failed to parse stepSize for %s: %w", symbol, err)
				}
			}
		}

		// Adjust quantity to comply with LOT_SIZE
		quantityFloat, err := strconv.ParseFloat(quantity, 64)
		if err != nil {
			return Result{}, fmt.Errorf("invalid quantity format: %w", err)
		}

		if quantityFloat < minQty {
			return Result{}, fmt.Errorf("quantity %.8f is below the minimum allowed %.8f for %s", quantityFloat, minQty, symbol)
		}

		if quantityFloat > maxQty {
			quantityFloat = maxQty // Cap at maxQty
		}

		// Ensure stepSize is valid
		if stepSize <= 0 {
			logger.Infof("Invalid stepSize for %s: %.8f", symbol, stepSize)
			return Result{}, fmt.Errorf("invalid stepSize for %s: %.8f", symbol, stepSize)
		}

		// Align with StepSize
		adjustedQty := math.Floor(quantityFloat/stepSize) * stepSize
		if math.IsNaN(adjustedQty) || adjustedQty <= 0 {
			logger.Infof("Adjusted Quantity for %s is invalid: Original=%s, Adjusted=NaN or <= 0", symbol, quantity)
			return Result{}, fmt.Errorf("adjusted quantity is invalid for %s: Original=%s", symbol, quantity)
		}

		formattedQty := strconv.FormatFloat(adjustedQty, 'f', -1, 64)
		logger.Infof("Adjusted Quantity for %s: Original=%s, Adjusted=%s", symbol, quantity, formattedQty)

		quantityPrecision := 0

		if side == "SELL" {
			quantityPrecision = info.Symbols[0].BaseAssetPrecision
		} else {
			quantityPrecision = info.Symbols[0].QuoteAssetPrecision
		}

		formattedQty = strconv.FormatFloat(adjustedQty, 'f', quantityPrecision, 64)

		logger.Debug("Formatted Qty: ", formattedQty)

		// Place the market order
		order, err := b.client.NewCreateOrderService().
			Symbol(symbol).
			Side(binance.SideType(side)).
			Type(binance.OrderTypeMarket).
			Quantity(formattedQty).
			Do(context.Background())

		if err != nil {
			return Result{}, fmt.Errorf("failed to place MARKET %s order for %s: %w", side, symbol, err)
		}

		// Calculate the executed price based on fills
		var totalQuoteQty float64
		var totalBaseQty float64

		for _, fill := range order.Fills {
			price, err := strconv.ParseFloat(fill.Price, 64)
			if err != nil {
				return Result{}, fmt.Errorf("failed to parse fill price: %w", err)
			}
			quantity, err := strconv.ParseFloat(fill.Quantity, 64)
			if err != nil {
				return Result{}, fmt.Errorf("failed to parse fill quantity: %w", err)
			}
			totalQuoteQty += price * quantity
			totalBaseQty += quantity
		}

		// Calculate the average executed price
		if totalBaseQty == 0 {
			return Result{}, fmt.Errorf("no fills returned for the market order")
		}
		averagePrice := totalQuoteQty / totalBaseQty

		return Result{OrderID: order.OrderID, AveragePrice: averagePrice}, nil
	}

	data, err := b.enqueueRequest(do, true)
	if err != nil {
		return 0, 0, err
	}
	res, ok := data.(Result)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected data type returned from queue")
	}
	return res.OrderID, res.AveragePrice, nil
}

// FetchMarkets retrieves all trading pairs matching the provided ticker (e.g., "USDT").
func (b *BinanceClient) FetchMarkets(tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) ([]models.TradingPair, error) {
	logger.Infof("Fetching all markets matching ticker: %v", tickers)

	doFunc := func() (any, error) {
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

	data, err := b.enqueueRequest(doFunc, false)
	if err != nil {
		return nil, err
	}
	tps, ok := data.([]models.TradingPair)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from queue")
	}
	return tps, nil
}

// FetchActiveTrades from the Binance client
func (b *BinanceClient) FetchActiveTrades(symbol string) ([]*binance.TradeV3, error) {

	doFunc := func() (any, error) {

		trades, err := b.client.NewListTradesService().Symbol(symbol).Do(context.Background())
		if err != nil {
			return nil, err
		}

		return trades, nil
	}

	data, err := b.enqueueRequest(doFunc, false)
	if err != nil {
		return nil, err
	}
	trades, ok := data.([]*binance.TradeV3)
	if !ok {
		return nil, fmt.Errorf("unexpected data type returned from queue")
	}
	return trades, nil
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
		err := fn()
		if err == nil {
			return nil
		}
		logger.Error(err.Error())
		time.Sleep(delay)
	}
	return fmt.Errorf("operation failed after %d retries", retries)
}

// rateLimitWait enforces request/s rate limiting
func (b *BinanceClient) rateLimitWait() {
	b.requestsMu.Lock()
	defer b.requestsMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-b.interval)
	filtered := b.requestTimes[:0]
	for _, t := range b.requestTimes {
		if t.After(cutoff) {
			filtered = append(filtered, t)
		}
	}
	b.requestTimes = filtered

	if len(b.requestTimes) >= b.maxRequests {
		oldest := b.requestTimes[0]
		waitDuration := oldest.Add(b.interval).Sub(now)
		if waitDuration > 0 {
			logger.Debugf("Rate limit reached, waiting for %s", waitDuration)
			time.Sleep(waitDuration)
		}
		newNow := time.Now()

		newCutoff := newNow.Add(-b.interval)
		newFiltered := b.requestTimes[:0]
		for _, t := range b.requestTimes {
			if t.After(newCutoff) {
				newFiltered = append(newFiltered, t)
			}
		}
		b.requestTimes = newFiltered
	}

	b.requestTimes = append(b.requestTimes, time.Now())
}
