package backtest

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/adshao/go-binance/v2"
	"time"
)

type MockExchangeClient struct {
	historicalData map[string]map[string][]models.CandleStick

	currentIndex map[string]int

	tradingPairs map[string]*models.TradingPair

	balances map[string]float64

	transactions []Transaction

	feeRate float64
}

type Transaction struct {
	Symbol    string
	Side      string
	Price     float64
	Quantity  float64
	Timestamp time.Time
	Fee       float64
}

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

func (m *MockExchangeClient) LoadHistoricalData(symbol, interval string, candles []models.CandleStick) {
	if _, ok := m.historicalData[symbol]; !ok {
		m.historicalData[symbol] = make(map[string][]models.CandleStick)
	}
	m.historicalData[symbol][interval] = candles
	m.currentIndex[symbol] = 0
}

func (m *MockExchangeClient) SetIndex(symbol string, idx int) error {
	if _, ok := m.historicalData[symbol]; !ok {
		return fmt.Errorf("no historical data for symbol %s", symbol)
	}
	if idx < 0 {
		idx = 0
	}

	for _, candles := range m.historicalData[symbol] {
		if len(candles) == 0 {
			continue
		}
		if idx >= len(candles) {
			idx = len(candles) - 1
		}
		break
	}
	m.currentIndex[symbol] = idx
	return nil
}

func (m *MockExchangeClient) ResetBacktest(initialBalances map[string]float64) {
	m.currentIndex = make(map[string]int)
	m.balances = initialBalances
	m.transactions = []Transaction{}
}

func (m *MockExchangeClient) GetTransactions() []Transaction {
	return m.transactions
}

func (m *MockExchangeClient) AddTradingPair(pair models.TradingPair) error {
	m.tradingPairs[pair.Symbol] = &pair
	return nil
}

func (m *MockExchangeClient) GetCurrentPrice(symbol string) (float64, error) {
	if _, ok := m.historicalData[symbol]; !ok {
		return 0, fmt.Errorf("no historical data for symbol %s", symbol)
	}

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

	start := idx - limit + 1
	if start < 0 {
		start = 0
	}

	return candles[start : idx+1], nil
}

func (m *MockExchangeClient) GetBalance(asset string) (float64, error) {
	balance, ok := m.balances[asset]
	if !ok {
		return 0, nil // Return 0 if the asset is not in the balance map
	}
	return balance, nil
}

func (m *MockExchangeClient) CreateOrder(symbol, orderType, side string, amount string) (float64, error) {
	price, err := m.GetCurrentPrice(symbol)
	if err != nil {
		return 0, err
	}

	quantity, err := parseAmount(amount)
	if err != nil {
		return 0, err
	}

	baseAsset, quoteAsset := extractAssets(symbol)

	totalValue := price * quantity
	fee := totalValue * m.feeRate

	if side == "BUY" {

		quoteBalance, _ := m.GetBalance(quoteAsset)
		if quoteBalance < totalValue+fee {
			return 0, fmt.Errorf("insufficient balance of %s for buy order", quoteAsset)
		}

		m.balances[quoteAsset] -= (totalValue + fee)
		m.balances[baseAsset] += quantity
	} else if side == "SELL" {

		baseBalance, _ := m.GetBalance(baseAsset)
		if baseBalance < quantity {
			return 0, fmt.Errorf("insufficient balance of %s for sell order", baseAsset)
		}

		m.balances[baseAsset] -= quantity
		m.balances[quoteAsset] += (totalValue - fee)
	} else {
		return 0, fmt.Errorf("invalid order side: %s", side)
	}

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

func (m *MockExchangeClient) CreateMarketOrder(symbol, side, quantity string) (int64, float64, error) {
	price, err := m.CreateOrder(symbol, "MARKET", side, quantity)
	if err != nil {
		return 0, 0, err
	}

	return time.Now().UnixNano(), price, nil
}

func (m *MockExchangeClient) IsTickerTooNew(symbol string) (bool, error) {

	return false, nil
}

func (m *MockExchangeClient) GetTradingPairs() map[string]*models.TradingPair {
	return m.tradingPairs
}

func (m *MockExchangeClient) FetchMarkets(tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) ([]models.TradingPair, error) {

	var result []models.TradingPair

	for symbol := range m.historicalData {

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

func (m *MockExchangeClient) FetchActiveTrades(symbol string) ([]*binance.TradeV3, error) {

	return []*binance.TradeV3{}, nil
}

func (m *MockExchangeClient) FetchHistoricalCandles(symbol string, interval string, startTime, endTime time.Time, limit int) ([]models.CandleStick, error) {
	if _, ok := m.historicalData[symbol]; !ok {
		return nil, fmt.Errorf("no historical data for symbol %s", symbol)
	}

	if _, ok := m.historicalData[symbol][interval]; !ok {
		return nil, fmt.Errorf("no historical data for symbol %s with interval %s", symbol, interval)
	}

	candles := m.historicalData[symbol][interval]

	var filteredCandles []models.CandleStick
	for _, candle := range candles {

		if (candle.Timestamp.After(startTime) || candle.Timestamp.Equal(startTime)) &&
			(candle.Timestamp.Before(endTime) || candle.Timestamp.Equal(endTime)) {
			filteredCandles = append(filteredCandles, candle)
		}
	}

	if limit > 0 && len(filteredCandles) > limit {
		return filteredCandles[len(filteredCandles)-limit:], nil
	}

	return filteredCandles, nil
}

func (m *MockExchangeClient) AdvanceTime(symbol string, steps int) error {
	if _, ok := m.currentIndex[symbol]; !ok {
		return fmt.Errorf("symbol %s not initialized", symbol)
	}

	m.currentIndex[symbol] += steps

	for interval, candles := range m.historicalData[symbol] {
		if m.currentIndex[symbol] >= len(candles) {
			return fmt.Errorf("reached end of historical data for %s with interval %s", symbol, interval)
		}
	}

	return nil
}

func parseAmount(amount string) (float64, error) {
	var quantity float64
	_, err := fmt.Sscanf(amount, "%f", &quantity)
	if err != nil {
		return 0, fmt.Errorf("invalid amount format: %s", amount)
	}
	return quantity, nil
}

func extractAssets(symbol string) (string, string) {

	quoteAssets := []string{"USDT", "BTC", "ETH", "BNB", "BUSD"}

	for _, quote := range quoteAssets {
		if len(symbol) > len(quote) && symbol[len(symbol)-len(quote):] == quote {
			base := symbol[:len(symbol)-len(quote)]
			return base, quote
		}
	}

	if len(symbol) > 4 {
		return symbol[:len(symbol)-4], symbol[len(symbol)-4:]
	}

	return symbol, ""
}

func shouldIncludeSymbol(symbol string, tickers []string, excludedMarkets []string, excludedTradingPairs []models.TradingPair) bool {
	baseAsset, quoteAsset := extractAssets(symbol)

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

	for _, excluded := range excludedMarkets {
		if quoteAsset == excluded {
			return false
		}
	}

	for _, excludedPair := range excludedTradingPairs {
		if symbol == excludedPair.Symbol {
			return false
		}
	}

	return true
}

var _ interfaces.ExchangeClient = (*MockExchangeClient)(nil)
