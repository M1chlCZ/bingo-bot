package backtest

import (
	"flag"
	"fmt"
	"github.com/M1chlCZ/bingo-bot/config"
	"github.com/M1chlCZ/bingo-bot/interfaces"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"os"
	"time"
)

func FetchDataCmd() {

	symbol := flag.String("symbol", config.BacktestConfig.DefaultSymbol, "Trading pair symbol (e.g., BTCUSDT)")
	interval := flag.String("interval", config.BacktestConfig.DefaultInterval, "Candle interval (e.g., 1h, 4h, 1d)")
	startDateStr := flag.String("start", "", "Start date (YYYY-MM-DD)")
	endDateStr := flag.String("end", "", "End date (YYYY-MM-DD)")
	apiKey := flag.String("api-key", "", "Binance API key")
	apiSecret := flag.String("api-secret", "", "Binance API secret")

	flag.Parse()

	if *apiKey == "" || *apiSecret == "" {
		fmt.Println("Error: API key and secret are required")
		fmt.Println("Usage: go run main.go fetch-data -api-key=YOUR_API_KEY -api-secret=YOUR_API_SECRET [options]")
		os.Exit(1)
	}

	if *startDateStr == "" {

		*startDateStr = time.Now().AddDate(0, 0, -config.BacktestConfig.FetchLookbackDays).Format("2006-01-02")
	}
	if *endDateStr == "" {

		*endDateStr = time.Now().Format("2006-01-02")
	}

	startDate, err := time.Parse("2006-01-02", *startDateStr)
	if err != nil {
		fmt.Printf("Error parsing start date: %v\n", err)
		os.Exit(1)
	}
	endDate, err := time.Parse("2006-01-02", *endDateStr)
	if err != nil {
		fmt.Printf("Error parsing end date: %v\n", err)
		os.Exit(1)
	}

	logger.InitLogger(nil, nil)

	fetcher, err := NewDataFetcher(*apiKey, *apiSecret)
	if err != nil {
		fmt.Printf("Error creating data fetcher: %v\n", err)
		os.Exit(1)
	}

	candles, err := fetcher.FetchHistoricalData(*symbol, *interval, startDate, endDate)
	if err != nil {
		fmt.Printf("Error fetching historical data: %v\n", err)
		os.Exit(1)
	}

	err = fetcher.SaveHistoricalData(candles, *symbol, *interval)
	if err != nil {
		fmt.Printf("Error saving historical data: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully fetched and saved %d candles for %s with interval %s from %s to %s\n",
		len(candles), *symbol, *interval, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
}

func RunBacktestCmd() {

	symbol := flag.String("symbol", config.BacktestConfig.DefaultSymbol, "Trading pair symbol (e.g., BTCUSDT)")
	interval := flag.String("interval", config.BacktestConfig.DefaultInterval, "Candle interval (e.g., 1h, 4h, 1d)")
	strategyName := flag.String("strategy", config.BacktestConfig.DefaultStrategy, "Strategy to use (e.g., compound)")
	startDateStr := flag.String("start", "", "Start date (YYYY-MM-DD)")
	endDateStr := flag.String("end", "", "End date (YYYY-MM-DD)")
	initialBalance := flag.Float64("balance", config.BacktestConfig.DefaultBalance, "Initial balance")

	flag.Parse()

	if *startDateStr == "" {

		*startDateStr = time.Now().AddDate(0, 0, -config.BacktestConfig.BacktestLookbackDays).Format("2006-01-02")
	}
	if *endDateStr == "" {

		*endDateStr = time.Now().Format("2006-01-02")
	}

	startDate, err := time.Parse("2006-01-02", *startDateStr)
	if err != nil {
		fmt.Printf("Error parsing start date: %v\n", err)
		os.Exit(1)
	}
	endDate, err := time.Parse("2006-01-02", *endDateStr)
	if err != nil {
		fmt.Printf("Error parsing end date: %v\n", err)
		os.Exit(1)
	}

	logger.InitLogger(nil, nil)

	var strategy interface{}
	switch *strategyName {
	case "compound":
		strategy = &strategies.CompoundStrategy{}
	default:
		fmt.Printf("Unknown strategy: %s\n", *strategyName)
		os.Exit(1)
	}

	result, err := RunBacktest(*symbol, *interval, strategy.(interfaces.Strategy), startDate, endDate, *initialBalance)
	if err != nil {
		fmt.Printf("Error running backtest: %v\n", err)
		os.Exit(1)
	}

	printBacktestResults(result)
}
