package client

import (
	"binance_bot/models"
	"context"
	"fmt"
	"github.com/adshao/go-binance/v2"
	"log"
	"math"
	"strconv"
	"sync"
	"time"
)

// BinanceClient implements the Exchange interface
type BinanceClient struct {
	client      *binance.Client
	Pairs       map[string]*models.TradingPair
	PairsMutex  sync.RWMutex
	candleCache map[string][]models.CandleStick
	cacheMutex  sync.RWMutex
}

// NewBinanceClient creates a new Binance client instance
func NewBinanceClient(apiKey, apiSecret string) (*BinanceClient, error) {
	client := binance.NewClient(apiKey, apiSecret)
	return &BinanceClient{
		client:      client,
		Pairs:       make(map[string]*models.TradingPair),
		candleCache: make(map[string][]models.CandleStick),
	}, nil
}

// AddTradingPair adds a new trading pair to monitor
func (b *BinanceClient) AddTradingPair(pair models.TradingPair) error {
	info, err := b.client.NewExchangeInfoService().Symbol(pair.Symbol).Do(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get exchange info for %s: %v", pair.Symbol, err)
	}

	// Find and set symbol precision and filters
	for _, symbol := range info.Symbols {
		if symbol.Symbol == pair.Symbol {
			pair.PricePrecision = symbol.QuotePrecision
			pair.QtyPrecision = symbol.BaseAssetPrecision

			for _, filter := range symbol.Filters {
				if filter["filterType"] == "MIN_NOTIONAL" {
					minNotionalStr := filter["minNotional"].(string)
					pair.MinNotional, err = strconv.ParseFloat(minNotionalStr, 64)
					if err != nil {
						return fmt.Errorf("failed to parse minNotional for %s: %v", pair.Symbol, err)
					}
				}
			}

			b.PairsMutex.Lock()
			b.Pairs[pair.Symbol] = &pair
			b.PairsMutex.Unlock()
			return nil
		}
	}

	return fmt.Errorf("symbol %s not found", pair.Symbol)
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
	b.PairsMutex.RLock()
	_, exists := b.Pairs[symbol]
	b.PairsMutex.RUnlock()

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
		log.Printf("Invalid stepSize for %s: %.8f", symbol, stepSize)
		return 0, fmt.Errorf("invalid stepSize for %s: %.8f", symbol, stepSize)
	}

	// Align with StepSize
	adjustedQty := math.Floor(quantityFloat/stepSize) * stepSize
	if math.IsNaN(adjustedQty) || adjustedQty <= 0 {
		log.Printf("Adjusted Quantity for %s is invalid: Original=%s, Adjusted=NaN or <= 0", symbol, quantity)
		return 0, fmt.Errorf("adjusted quantity is invalid for %s: Original=%s", symbol, quantity)
	}

	formattedQty := strconv.FormatFloat(adjustedQty, 'f', -1, 64)
	log.Printf("Adjusted Quantity for %s: Original=%s, Adjusted=%s", symbol, quantity, formattedQty)

	// Adjust price using PRICE_FILTER (already implemented in previous steps)
	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}
	pricePrecision := info.Symbols[0].QuotePrecision
	adjustedPrice := math.Floor(priceFloat/tickSize) * tickSize
	formattedPrice := strconv.FormatFloat(adjustedPrice, 'f', pricePrecision, 64)

	fmt.Println("Price Precision: ", pricePrecision)
	fmt.Println("Adjusted Price: ", adjustedPrice)
	fmt.Println("Formatted Price: ", formattedPrice)

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

	log.Printf("Successfully placed LIMIT %s order for %s: OrderID=%d", side, symbol, order.OrderID)
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
		log.Printf("Invalid stepSize for %s: %.8f", symbol, stepSize)
		return 0, fmt.Errorf("invalid stepSize for %s: %.8f", symbol, stepSize)
	}

	// Align with StepSize
	adjustedQty := math.Floor(quantityFloat/stepSize) * stepSize
	if math.IsNaN(adjustedQty) || adjustedQty <= 0 {
		log.Printf("Adjusted Quantity for %s is invalid: Original=%s, Adjusted=NaN or <= 0", symbol, quantity)
		return 0, fmt.Errorf("adjusted quantity is invalid for %s: Original=%s", symbol, quantity)
	}

	formattedQty := strconv.FormatFloat(adjustedQty, 'f', -1, 64)
	log.Printf("Adjusted Quantity for %s: Original=%s, Adjusted=%s", symbol, quantity, formattedQty)

	// Adjust price using PRICE_FILTER (already implemented in previous steps)
	priceFloat, err := strconv.ParseFloat(price, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}
	pricePrecision := info.Symbols[0].QuotePrecision
	adjustedPrice := math.Floor(priceFloat/tickSize) * tickSize
	formattedPrice := strconv.FormatFloat(adjustedPrice, 'f', pricePrecision, 64)

	fmt.Println("Price Precision: ", pricePrecision)
	fmt.Println("Adjusted Price: ", adjustedPrice)
	fmt.Println("Formatted Price: ", formattedPrice)

	// Adjust price using PRICE_FILTER (already implemented in previous steps)
	priceStopLossFloat, err := strconv.ParseFloat(stopLoss, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid price format: %v", err)
	}
	adjustedStopLossPrice := math.Floor(priceStopLossFloat/tickSize) * tickSize
	formattedStopLossPrice := strconv.FormatFloat(adjustedPrice, 'f', pricePrecision, 64)

	fmt.Println("Stop Loss Price Precision: ", pricePrecision)
	fmt.Println("Stop Loss Adjusted Price: ", adjustedStopLossPrice)
	fmt.Println("Stop Loss Formatted Price: ", formattedStopLossPrice)

	// Place the limit order
	order, err := b.client.NewCreateOrderService().
		Symbol(symbol).
		Side(binance.SideType(side)).
		Type(binance.OrderTypeLimit).
		TimeInForce(binance.TimeInForceTypeGTC).
		Quantity(formattedQty).
		Price(formattedPrice).
		StopPrice(formattedStopLossPrice).
		Do(context.Background())

	if err != nil {
		return 0, fmt.Errorf("failed to place LIMIT %s order for %s: %v", side, symbol, err)
	}

	log.Printf("Successfully placed LIMIT %s order for %s: OrderID=%d", side, symbol, order.OrderID)
	return order.OrderID, nil
}

func (b *BinanceClient) MonitorOrder(symbol string, orderID int64) (bool, error) {
	log.Printf("Monitoring order %d for %s", orderID, symbol)

	for {
		// Fetch order status
		order, err := b.client.NewGetOrderService().
			Symbol(symbol).
			OrderID(orderID).
			Do(context.Background())
		if err != nil {
			return false, fmt.Errorf("failed to fetch order status for %s: %v", symbol, err)
		}

		log.Printf("Order %d status: %s (Filled Quantity: %s)", orderID, order.Status, order.ExecutedQuantity)

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
	log.Printf("Canceling order %d for %s", orderID, symbol)

	_, err := b.client.NewCancelOrderService().
		Symbol(symbol).
		OrderID(orderID).
		Do(context.Background())

	if err != nil {
		return fmt.Errorf("failed to cancel order %d for %s: %v", orderID, symbol, err)
	}

	log.Printf("Successfully canceled order %d for %s", orderID, symbol)
	return nil
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
