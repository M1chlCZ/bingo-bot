package backtest

import (
	"encoding/json"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/client"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"os"
	"path/filepath"
	"time"
)

// DataFetcher is responsible for fetching historical data from Binance
type DataFetcher struct {
	client interfaces.ExchangeClient
}

// NewDataFetcher creates a new DataFetcher instance
func NewDataFetcher(apiKey, apiSecret string) (*DataFetcher, error) {
	binanceClient, err := client.NewBinanceClient(apiKey, apiSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create Binance client: %v", err)
	}

	return &DataFetcher{
		client: binanceClient,
	}, nil
}

// FetchHistoricalData fetches historical data for the given symbol, interval, and time range
func (df *DataFetcher) FetchHistoricalData(symbol, interval string, startTime, endTime time.Time) ([]models.CandleStick, error) {
	logger.Infof("Fetching historical data for %s with interval %s from %s to %s",
		symbol, interval, startTime.Format("2006-01-02"), endTime.Format("2006-01-02"))

	// Fetch historical candles
	candles, err := df.client.FetchHistoricalCandles(symbol, interval, startTime, endTime, 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch historical candles: %v", err)
	}

	logger.Infof("Fetched %d candles for %s", len(candles), symbol)
	return candles, nil
}

// SaveHistoricalData saves the historical data to a file
func (df *DataFetcher) SaveHistoricalData(candles []models.CandleStick, symbol, interval string) error {
	// Create data directory if it doesn't exist
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Create filename based on symbol and interval
	filename := filepath.Join(dataDir, fmt.Sprintf("%s_%s.json", symbol, interval))

	// Marshal candles to JSON
	data, err := json.Marshal(candles)
	if err != nil {
		return fmt.Errorf("failed to marshal candles to JSON: %v", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write data to file: %v", err)
	}

	logger.Infof("Saved %d candles to %s", len(candles), filename)
	return nil
}

// LoadHistoricalData loads historical data from a file
func LoadHistoricalData(symbol, interval string) ([]models.CandleStick, error) {
	// Create filename based on symbol and interval
	dataDir := "data"
	filename := filepath.Join(dataDir, fmt.Sprintf("%s_%s.json", symbol, interval))

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("historical data file does not exist: %s", filename)
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read data from file: %v", err)
	}

	// Unmarshal JSON to candles
	var candles []models.CandleStick
	if err := json.Unmarshal(data, &candles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to candles: %v", err)
	}

	logger.Infof("Loaded %d candles from %s", len(candles), filename)
	return candles, nil
}

// RunBacktest runs a backtest with the given configuration
func RunBacktest(symbol, interval string, strategy interfaces.Strategy, startTime, endTime time.Time, initialBalance float64) (*BacktestResult, error) {
	// Load historical data
	candles, err := LoadHistoricalData(symbol, interval)
	if err != nil {
		return nil, fmt.Errorf("failed to load historical data: %v", err)
	}

	// Filter candles by time range
	var filteredCandles []models.CandleStick
	for _, candle := range candles {
		if (candle.Timestamp.After(startTime) || candle.Timestamp.Equal(startTime)) &&
			(candle.Timestamp.Before(endTime) || candle.Timestamp.Equal(endTime)) {
			filteredCandles = append(filteredCandles, candle)
		}
	}

	if len(filteredCandles) == 0 {
		return nil, fmt.Errorf("no candles found in the specified time range")
	}

	logger.Infof("Running backtest with %d candles from %s to %s",
		len(filteredCandles),
		filteredCandles[0].Timestamp.Format("2006-01-02"),
		filteredCandles[len(filteredCandles)-1].Timestamp.Format("2006-01-02"))

	// Set up initial balances
	baseAsset, quoteAsset := extractAssets(symbol)
	initialBalances := map[string]float64{
		quoteAsset: initialBalance,
		baseAsset:  0.0,
	}

	// Set up historical data for the backtest
	historicalData := make(map[string]map[string][]models.CandleStick)
	intervalData := make(map[string][]models.CandleStick)
	intervalData[interval] = filteredCandles
	historicalData[symbol] = intervalData

	// Set up trading pairs
	tradingPairs := []models.TradingPair{
		{
			Symbol:     symbol,
			BaseAsset:  baseAsset,
			QuoteAsset: quoteAsset,
		},
	}

	// Configure the backtest
	config := BacktestConfig{
		InitialBalances: initialBalances,
		FeeRate:         0.001, // 0.1% trading fee
		TradingPairs:    tradingPairs,
		HistoricalData:  historicalData,
		Strategy:        strategy,
		StartTime:       startTime,
		EndTime:         endTime,
		TimeStep:        getTimeStepFromInterval(interval),
		RiskPercentage:  0.02, // Risk 2% of account on each trade
		MinNotional:     10.0, // Minimum notional value for trades (e.g., $10)
	}

	// Create and run the backtest
	runner := NewRunner(config)
	result, err := runner.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run backtest: %v", err)
	}

	return result, nil
}

// getTimeStepFromInterval converts an interval string to a time.Duration
func getTimeStepFromInterval(interval string) time.Duration {
	switch interval {
	case "1m":
		return 1 * time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return 1 * time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "3d":
		return 72 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	case "1M":
		return 30 * 24 * time.Hour
	default:
		logger.Warnf("Unknown interval: %s, defaulting to 1h", interval)
		return 1 * time.Hour
	}
}
