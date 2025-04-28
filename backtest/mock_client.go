package backtest

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/adshao/go-binance/v2"
	"time"
)

// MockExchangeClient implements the interfaces.ExchangeClient interface for backtesting purposes.
// It simulates trading without making actual API calls by using historical data.
type MockExchangeClient struct {
	// Historical candle data for each symbol and interval
	historicalData map[string]map[string][]models.CandleStick
	// Current position in the historical data for each symbol
	currentIndex map[string]int
	// Trading pairs being tracked
	tradingPairs map[string]*models.TradingPair
	// Simulated balances for each asset
	balances map[string]float64
	// Transaction history for performance analysis
	transactions []Transaction
	// Fee rate for simulated trades (e.g., 0.001 for 0.1%)
	feeRate float64
}

// Transaction represents a simulated trade for backtesting
type Transaction struct {
	Symbol    string
	Side      string
	Price     float64
	Quantity  float64
	Timestamp time.Time
	Fee       float64
}

// NewMockExchangeClient creates a new instance of MockExchangeClient
func NewMockExchangeClient(initialBalances map[string]float64, feeRate float64) *MockExchangeClient {
	if initialBalances == nil {
		initialBalances = make(map[string]float64)
	}

	return &MockExchangeClient{
		historicalData: make(map[string]map[string][]models.CandleStick),
		currentIndex:   make(map[string]int),
		tradingPairs:   make(map[string]*models.TradingPair),
		balances:       initialBalances,
		transactions:   []Transaction{},
		feeRate:        feeRate,
	}
}

// LoadHistoricalData loads historical candle data for backtesting
func (m *MockExchangeClient) LoadHistoricalData(symbol, interval string, candles []models.CandleStick) {
	if _, ok := m.historicalData[symbol]; !ok {
		m.historicalData[symbol] = make(map[string][]models.CandleStick)
	}
	m.historicalData[symbol][interval] = candles
	m.currentIndex[symbol] = 0
}

// ResetBacktest resets the backtesting state
func (m *MockExchangeClient) ResetBacktest(initialBalances map[string]float64) {
	m.currentIndex = make(map[string]int)
	m.balances = initialBalances
	m.transactions = []Transaction{}
}

// GetTransactions returns all transactions made during the backtest
func (m *MockExchangeClient) GetTransactions() []Transaction {
	return m.transactions
}

// AddTradingPair implements interfaces.ExchangeClient
func (m *MockExchangeClient) AddTradingPair(pair models.TradingPair) error {
	m.tradingPairs[pair.Symbol] = &pair
	return nil
}

// GetCurrentPrice implements interfaces.ExchangeClient
func (m *MockExchangeClient) GetCurrentPrice(symbol string) (float64, error) {
	if _, ok := m.historicalData[symbol]; !ok {
		return 0, fmt.Errorf("no historical data for symbol %s", symbol)
	}

	// Use the current candle's close price as the current price
	for _, candles := range m.historicalData[symbol] {
		if len(candles) == 0 {
			continue
		}

		idx := m.currentIndex[symbol]
		if idx >= len(candles) {
			return 0, fmt.Errorf("reached end of historical data for %s", symbol)
		}

		return candles[idx].Close, nil
	}

	return 0, fmt.Errorf("no candle data available for %s", symbol)
}

// FetchCandles implements interfaces.ExchangeClient
func (m *MockExchangeClient) FetchCandles(symbol, interval string, limit int, priority bool) ([]models.CandleStick, error) {
	if _, ok := m.historicalData[symbol]; !ok {
		return nil, fmt.Errorf("no historical data for symbol %s", symbol)
	}

	if _, ok := m.historicalData[symbol][interval]; !ok {
		return nil, fmt.Errorf("no historical data for symbol %s with interval %s", symbol, interval)
	}

	candles := m.historicalData[symbol][interval]
	idx := m.currentIndex[symbol]

	if idx >= len(candles) {
		return nil, fmt.Errorf("reached end of historical data for %s", symbol)
	}

	// Return up to 'limit' candles ending at the current index
	start := idx - limit + 1
	if start < 0 {
		start = 0
	}

	return candles[start : idx+1], nil
}

// GetBalance implements interfaces.ExchangeClient
func (m *MockExchangeClient) GetBalance(asset string) (float64, error) {
	balance, ok := m.balances[asset]
	if !ok {
		return 0, nil // Return 0 if the asset is not in the balance map
	}
	return balance, nil
}

// CreateOrder implements interfaces.ExchangeClient
func (m *MockExchangeClient) CreateOrder(symbol, orderType, side string, amount string) (float64, error) {
	price, err := m.GetCurrentPrice(symbol)
	if err != nil {
		return 0, err
	}

	quantity, err := parseAmount(amount)
	if err != nil {
		return 0, err
	}

	// Extract base and quote assets from the symbol (e.g., "BTCUSDT" -> "BTC", "USDT")
	baseAsset, quoteAsset := extractAssets(symbol)

	// Calculate the total value and fee
	totalValue := price * quantity
	fee := totalValue * m.feeRate

	// Update balances based on the trade
	if side == "BUY" {
		// Check if we have enough quote asset (e.g., USDT) to buy
		quoteBalance, _ := m.GetBalance(quoteAsset)
		if quoteBalance < totalValue+fee {
			return 0, fmt.Errorf("insufficient balance of %s for buy order", quoteAsset)
		}

		// Deduct quote asset and add base asset
		m.balances[quoteAsset] -= (totalValue + fee)
		m.balances[baseAsset] += quantity
	} else if side == "SELL" {
		// Check if we have enough base asset (e.g., BTC) to sell
		baseBalance, _ := m.GetBalance(baseAsset)
		if baseBalance < quantity {
			return 0, fmt.Errorf("insufficient balance of %s for sell order", baseAsset)
		}

		// Deduct base asset and add quote asset
		m.balances[baseAsset] -= quantity
		m.balances[quoteAsset] += (totalValue - fee)
	} else {
		return 0, fmt.Errorf("invalid order side: %s", side)
	}

	// Record the transaction
	m.transactions = append(m.transactions, Transaction{
		Symbol:    symbol,
		Side:      side,
		Price:     price,
		Quantity:  quantity,
		Timestamp: time.Now(),
		Fee:       fee,
	})

	return price, nil
}

// CreateMarketOrder implements interfaces.ExchangeClient
func (m *MockExchangeClient) CreateMarketOrder(symbol, side, quantity string) (int64, float64, error) {
	price, err := m.CreateOrder(symbol, "MARKET", side, quantity)
	if err != nil {
		return 0, 0, err
	}

	// Return a dummy order ID and the execution price
	return time.Now().UnixNano(), price, nil
}

// IsTickerTooNew implements interfaces.ExchangeClient
func (m *MockExchangeClient) IsTickerTooNew(symbol string) (bool, error) {
	// In backtesting, we assume all tickers are established
	return false, nil
}

// GetTradingPairs implements interfaces.ExchangeClient
func (m *MockExchangeClient) GetTradingPairs() map[string]*models.TradingPair {
	return m.tradingPairs
}

// FetchMarkets implements interfaces.ExchangeClient
func (m *MockExchangeClient) FetchMarkets(tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) ([]models.TradingPair, error) {
	// In backtesting, we only return the trading pairs that have historical data
	var result []models.TradingPair

	for symbol := range m.historicalData {
		// Check if this symbol should be included based on the filters
		if shouldIncludeSymbol(symbol, tickers, excludedMarkets, excludedTradingPairs) {
			baseAsset, quoteAsset := extractAssets(symbol)
			pair := models.TradingPair{
				Symbol:     symbol,
				BaseAsset:  baseAsset,
				QuoteAsset: quoteAsset,
			}
			result = append(result, pair)
		}
	}

	return result, nil
}

// FetchActiveTrades implements interfaces.ExchangeClient
func (m *MockExchangeClient) FetchActiveTrades(symbol string) ([]*binance.TradeV3, error) {
	// In backtesting, we don't simulate active trades
	return []*binance.TradeV3{}, nil
}

// FetchHistoricalCandles implements interfaces.ExchangeClient
func (m *MockExchangeClient) FetchHistoricalCandles(symbol string, interval string, startTime, endTime time.Time, limit int) ([]models.CandleStick, error) {
	if _, ok := m.historicalData[symbol]; !ok {
		return nil, fmt.Errorf("no historical data for symbol %s", symbol)
	}

	if _, ok := m.historicalData[symbol][interval]; !ok {
		return nil, fmt.Errorf("no historical data for symbol %s with interval %s", symbol, interval)
	}

	candles := m.historicalData[symbol][interval]

	// Filter candles by time range
	var filteredCandles []models.CandleStick
	for _, candle := range candles {
		// Use Timestamp for start time comparison
		// For end time, we can estimate it based on the interval
		if (candle.Timestamp.After(startTime) || candle.Timestamp.Equal(startTime)) && 
		   (candle.Timestamp.Before(endTime) || candle.Timestamp.Equal(endTime)) {
			filteredCandles = append(filteredCandles, candle)
		}
	}

	// Apply limit if needed
	if limit > 0 && len(filteredCandles) > limit {
		return filteredCandles[len(filteredCandles)-limit:], nil
	}

	return filteredCandles, nil
}

// AdvanceTime moves the current index forward to simulate the passage of time
func (m *MockExchangeClient) AdvanceTime(symbol string, steps int) error {
	if _, ok := m.currentIndex[symbol]; !ok {
		return fmt.Errorf("symbol %s not initialized", symbol)
	}

	m.currentIndex[symbol] += steps

	// Check if we've reached the end of the data
	for interval, candles := range m.historicalData[symbol] {
		if m.currentIndex[symbol] >= len(candles) {
			return fmt.Errorf("reached end of historical data for %s with interval %s", symbol, interval)
		}
	}

	return nil
}

// Helper functions

// parseAmount converts a string amount to a float64
func parseAmount(amount string) (float64, error) {
	var quantity float64
	_, err := fmt.Sscanf(amount, "%f", &quantity)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format: %s", amount)
	}
	return quantity, nil
}

// extractAssets extracts base and quote assets from a symbol (e.g., "BTCUSDT" -> "BTC", "USDT")
func extractAssets(symbol string) (string, string) {
	// This is a simplified implementation
	// In a real scenario, you might need more sophisticated logic or a lookup table

	// Common quote assets
	quoteAssets := []string{"USDT", "BTC", "ETH", "BNB", "BUSD"}

	for _, quote := range quoteAssets {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			base := symbol[:len(symbol)-len(quote)]
			return base, quote
		}
	}

	// Default fallback - assume the last 4 characters are the quote asset
	if len(symbol) > 4 {
		return symbol[:len(symbol)-4], symbol[len(symbol)-4:]
	}

	return symbol, ""
}

// shouldIncludeSymbol checks if a symbol should be included based on the filters
func shouldIncludeSymbol(symbol string, tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) bool {
	baseAsset, quoteAsset := extractAssets(symbol)

	// Check if the base asset is in the tickers list
	if len(tickers) > 0 {
		found := false
		for _, ticker := range tickers {
			if baseAsset == ticker {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check if the quote asset is in the excluded markets list
	for _, excluded := range excludedMarkets {
		if quoteAsset == excluded {
			return false
		}
	}

	// Check if the symbol is in the excluded trading pairs list
	for _, excludedPair := range excludedTradingPairs {
		if symbol == excludedPair.Symbol {
			return false
		}
	}

	return true
}

// Ensure MockExchangeClient implements interfaces.ExchangeClient
var _ interfaces.ExchangeClient = (*MockExchangeClient)(nil)
