package backtest

import (
	"fmt"
	"github.com/M1chlCZ/bingo-bot/logger"
	"github.com/M1chlCZ/bingo-bot/models"
	"github.com/M1chlCZ/bingo-bot/strategies"
	"time"
)

func RunBacktestExample() {

	logger.InitLogger(nil, nil)

	strategy := &strategies.CompoundStrategy{}

	tradingPairs := []models.TradingPair{
		{
			Symbol:     "BTCUSDT",
			BaseAsset:  "BTC",
			QuoteAsset: "USDT",
		},
	}

	initialBalances := map[string]float64{
		"USDT": 10000.0, // Start with 10,000 USDT
		"BTC":  0.0,     // Start with 0 BTC
	}

	historicalData := loadSampleHistoricalData()

	config := BacktestConfig{
		InitialBalances: initialBalances,
		FeeRate:         0.001, // 0.1% trading fee
		TradingPairs:    tradingPairs,
		HistoricalData:  historicalData,
		Strategy:        strategy,
		StartTime:       time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:         time.Date(2023, 2, 1, 0, 0, 0, 0, time.UTC),
		TimeStep:        4 * time.Hour, // Advance time by 4 hours in each iteration
		RiskPercentage:  0.02,          // Risk 2% of account on each trade
	}

	runner := NewRunner(config)
	result, err := runner.Run()
	if err != nil {
		logger.Errorf("Failed to run backtest: %v", err)
		return
	}

	printBacktestResults(result)
}

func loadSampleHistoricalData() map[string]map[string][]models.CandleStick {

	data := make(map[string]map[string][]models.CandleStick)

	btcusdt1h := make(map[string][]models.CandleStick)

	var candles []models.CandleStick
	startTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	price := 16500.0 // Starting price
	for i := 0; i < 744; i++ {

		priceChange := (float64(i%10) - 5.0) * 10.0
		high := price + priceChange
		low := price - priceChange

		candle := models.CandleStick{
			Timestamp: startTime.Add(time.Duration(i) * time.Hour),
			Open:      price,
			High:      high,
			Low:       low,
			Close:     price + priceChange/2,
			Volume:    1000.0 + float64(i%100)*10,
		}

		candles = append(candles, candle)
		price = candle.Close // Next candle opens at previous close
	}

	btcusdt1h["1h"] = candles
	data["BTCUSDT"] = btcusdt1h

	return data
}

func printBacktestResults(result *BacktestResult) {
	fmt.Println("===== BACKTEST RESULTS =====")
	fmt.Printf("Starting Balance: %.2f\n", result.StartingBalance)
	fmt.Printf("Ending Balance: %.2f\n", result.EndingBalance)
	fmt.Printf("Total Return: %.2f%%\n", result.PercentageReturn)
	fmt.Printf("Total Trades: %d\n", result.TotalTrades)
	fmt.Printf("Winning Trades: %d (%.2f%%)\n", result.WinningTrades, result.WinRate)
	fmt.Printf("Losing Trades: %d (%.2f%%)\n", result.LosingTrades, 100-result.WinRate)
	fmt.Printf("Break-Even Trades: %d\n", result.BreakEvenTrades)
	fmt.Printf("Total Profit/Loss: %.2f\n", result.TotalProfitLoss)
	fmt.Printf("Average Profit: %.2f\n", result.AverageProfit)
	fmt.Printf("Average Loss: %.2f\n", result.AverageLoss)
	fmt.Printf("Largest Profit: %.2f\n", result.LargestProfit)
	fmt.Printf("Largest Loss: %.2f\n", result.LargestLoss)
	fmt.Printf("Profit Factor: %.2f\n", result.ProfitFactor)

	fmt.Println("\nFinal Balances:")
	for asset, balance := range result.FinalBalances {
		fmt.Printf("%s: %.8f\n", asset, balance)
	}

	fmt.Println("\nTransaction Summary:")
	buys := 0
	sells := 0
	for _, tx := range result.Transactions {
		if tx.Side == "BUY" {
			buys++
		} else {
			sells++
		}
	}
	fmt.Printf("Total Buys: %d\n", buys)
	fmt.Printf("Total Sells: %d\n", sells)
}
